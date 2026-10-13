package cleanup

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// repoFixture is the layout sentei manages, built with real git under
// t.TempDir(): a remote to branch and delete in, and a repository root holding
// .bare, the .git pointer and a worktree for main.
type repoFixture struct {
	t      *testing.T
	origin string
	root   string
}

func newRepoFixture(t *testing.T) repoFixture {
	t.Helper()
	dir := t.TempDir()
	f := repoFixture{t: t, origin: filepath.Join(dir, "origin"), root: filepath.Join(dir, "repo")}

	f.git(dir, "init", "--quiet", "--initial-branch=main", f.origin)
	f.git(f.origin, "commit", "--quiet", "--allow-empty", "-m", "initial commit")
	f.git(dir, "clone", "--quiet", "--bare", f.origin, filepath.Join(f.root, ".bare"))
	if err := os.WriteFile(filepath.Join(f.root, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.git(f.root, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	f.git(f.root, "fetch", "--quiet", "origin")
	f.git(f.root, "worktree", "add", "--quiet", filepath.Join(f.root, "main"), "main")
	return f
}

func (f repoFixture) git(dir string, args ...string) string {
	f.t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// trackRemoteBranch creates name on the remote and a local branch tracking it.
func (f repoFixture) trackRemoteBranch(name string) {
	f.t.Helper()
	f.git(f.origin, "branch", name)
	f.git(f.root, "fetch", "--quiet", "origin")
	f.git(f.root, "branch", "--quiet", "--track", name, "origin/"+name)
}

// commitOn adds an empty commit to branch through a temporary worktree and,
// when push is set, pushes it to the remote.
func (f repoFixture) commitOn(branch string, push bool) {
	f.t.Helper()
	worktree := filepath.Join(f.root, "commit-worktree")
	f.git(f.root, "worktree", "add", "--quiet", worktree, branch)
	f.git(worktree, "commit", "--quiet", "--allow-empty", "-m", "work on "+branch)
	if push {
		f.git(worktree, "push", "--quiet", "origin", branch)
	}
	f.git(f.root, "worktree", "remove", worktree)
}

func (f repoFixture) branches() []string {
	f.t.Helper()
	return strings.Fields(f.git(f.root, "for-each-ref", "--format=%(refname:lstrip=2)", "refs/heads"))
}

func (f repoFixture) configPath() string {
	return filepath.Join(f.root, ".bare", "config")
}

func (f repoFixture) readConfig() string {
	f.t.Helper()
	data, err := os.ReadFile(f.configPath())
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f repoFixture) appendConfig(text string) {
	f.t.Helper()
	if err := os.WriteFile(f.configPath(), []byte(f.readConfig()+text), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func discard(Event) {}

func stepError(errs []OperationError, step string) error {
	for _, e := range errs {
		if e.Step == step {
			return e.Err
		}
	}
	return nil
}

// A branch deleted on the remote is not reported as gone until its
// remote-tracking ref is pruned, and a dry run does not prune. The preview
// must still name the branch the real run goes on to delete.
func TestDryRun_ListsBranchesThatPruningWillMakeGone(t *testing.T) {
	f := newRepoFixture(t)
	f.trackRemoteBranch("feature/old")
	f.git(f.origin, "branch", "--quiet", "-D", "feature/old")
	runner := &git.GitRunner{}

	scan, err := DryRun(runner, f.root)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if want := []string{"feature/old"}; !slices.Equal(scan.GoneBranches, want) {
		t.Errorf("preview GoneBranches = %v, want %v", scan.GoneBranches, want)
	}

	dry := Run(runner, f.root, Options{Mode: ModeSafe, DryRun: true}, discard)
	if dry.GoneBranchesDeleted != 1 {
		t.Errorf("dry run reports %d gone branch(es), want 1", dry.GoneBranchesDeleted)
	}
	if !slices.Contains(f.branches(), "feature/old") {
		t.Fatal("the dry run deleted the branch")
	}
	f.git(f.root, "show-ref", "--verify", "--quiet", "refs/remotes/origin/feature/old")

	result := Run(runner, f.root, Options{Mode: ModeSafe}, discard)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.GoneBranchesDeleted != dry.GoneBranchesDeleted {
		t.Errorf("real run deleted %d branch(es), the dry run announced %d", result.GoneBranchesDeleted, dry.GoneBranchesDeleted)
	}
	if slices.Contains(f.branches(), "feature/old") {
		t.Error("the real run kept the gone branch")
	}
}

// The upstream state must come from git's ref data, never from text that can
// also appear in a commit subject.
func TestRun_CommitSubjectIsNotUpstreamState(t *testing.T) {
	f := newRepoFixture(t)
	f.trackRemoteBranch("feature/live")
	worktree := filepath.Join(f.root, "live")
	f.git(f.root, "worktree", "add", "--quiet", worktree, "feature/live")
	f.git(worktree, "commit", "--quiet", "--allow-empty", "-m", "handle [origin/x: gone] in the parser")
	f.git(worktree, "push", "--quiet", "origin", "feature/live")
	f.git(f.root, "worktree", "remove", worktree)
	runner := &git.GitRunner{}

	scan, err := DryRun(runner, f.root)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(scan.GoneBranches) != 0 {
		t.Errorf("preview GoneBranches = %v, want none: the upstream of feature/live exists", scan.GoneBranches)
	}

	result := Run(runner, f.root, Options{Mode: ModeSafe}, discard)
	if result.GoneBranchesDeleted != 0 {
		t.Errorf("GoneBranchesDeleted = %d, want 0", result.GoneBranchesDeleted)
	}
	if !slices.Contains(f.branches(), "feature/live") {
		t.Error("a branch with a live upstream was deleted because of its commit subject")
	}
}

func TestRun_KeepsProtectedBranchWithGoneUpstream(t *testing.T) {
	f := newRepoFixture(t)
	f.trackRemoteBranch("develop")
	f.git(f.origin, "branch", "--quiet", "-D", "develop")
	runner := &git.GitRunner{}

	scan, err := DryRun(runner, f.root)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(scan.GoneBranches) != 0 {
		t.Errorf("preview GoneBranches = %v, want none: develop is protected", scan.GoneBranches)
	}

	result := Run(runner, f.root, Options{Mode: ModeSafe}, discard)
	if !slices.Contains(f.branches(), "develop") {
		t.Fatal("protected branch develop was deleted")
	}
	if want := (SkippedBranch{Name: "develop", Reason: SkipProtected}); !slices.Contains(result.BranchesSkipped, want) {
		t.Errorf("BranchesSkipped = %v, want it to contain %v", result.BranchesSkipped, want)
	}
}

// A deletion git refuses for any reason other than unmerged work is a failed
// step, not a skip.
func TestRun_DeleteFailureIsNotReportedAsUnmerged(t *testing.T) {
	f := newRepoFixture(t)
	f.trackRemoteBranch("feature/old")
	f.git(f.origin, "branch", "--quiet", "-D", "feature/old")
	// Another process holding the ref lock makes `git branch -d` fail although
	// the branch is fully merged.
	lock := filepath.Join(f.root, ".bare", "refs", "heads", "feature", "old.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	result := Run(&git.GitRunner{}, f.root, Options{Mode: ModeSafe}, discard)

	if stepError(result.Errors, "gone-branches") == nil {
		t.Errorf("expected a gone-branches error, got errors %v", result.Errors)
	}
	for _, s := range result.BranchesSkipped {
		if s.Name == "feature/old" && s.Reason == SkipUnmerged {
			t.Errorf("feature/old is fully merged but was reported as %q", s.Reason)
		}
	}
}

// Repeated lines are not always redundant: git reads the last value of a
// single-valued key, so dropping a later repeat changes the effective value.
func TestRun_LeavesRepeatedConfigLinesAlone(t *testing.T) {
	f := newRepoFixture(t)
	f.appendConfig("[sentei-test]\n\tflag = false\n\tflag = true\n\tflag = false\n")
	before := f.readConfig()

	result := Run(&git.GitRunner{}, f.root, Options{Mode: ModeSafe}, discard)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if got := f.git(f.root, "config", "--get", "sentei-test.flag"); got != "false" {
		t.Errorf("effective sentei-test.flag = %q after cleanup, want false", got)
	}
	if after := f.readConfig(); after != before {
		t.Errorf("config changed although nothing was orphaned:\n%s", after)
	}
}

func TestRun_RemovesOnlyOrphanedBranchSections(t *testing.T) {
	f := newRepoFixture(t)
	f.git(f.root, "config", "branch.main.remote", "origin")
	f.git(f.root, "config", "branch.main.merge", "refs/heads/main")
	f.git(f.root, "config", "branch.feature/deep/nested.v2.remote", "origin")
	f.git(f.root, "config", "branch.feature/deep/nested.v2.merge", "refs/heads/feature/deep/nested.v2")
	runner := &git.GitRunner{}

	dry := Run(runner, f.root, Options{Mode: ModeSafe, DryRun: true}, discard)
	if dry.ConfigOrphanResult.Removed != 1 {
		t.Errorf("dry run reports %d orphaned section(s), want 1", dry.ConfigOrphanResult.Removed)
	}
	if !strings.Contains(f.readConfig(), `[branch "feature/deep/nested.v2"]`) {
		t.Fatal("the dry run removed the orphaned section")
	}

	result := Run(runner, f.root, Options{Mode: ModeSafe}, discard)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.ConfigOrphanResult.Removed != 1 {
		t.Errorf("Removed = %d, want 1", result.ConfigOrphanResult.Removed)
	}
	config := f.readConfig()
	if strings.Contains(config, "nested.v2") {
		t.Errorf("the orphaned section survived:\n%s", config)
	}
	if got := f.git(f.root, "config", "--get", "branch.main.merge"); got != "refs/heads/main" {
		t.Errorf("tracking config of the live branch main was changed: branch.main.merge = %q", got)
	}
	if _, err := os.Stat(f.configPath() + ".bak"); !os.IsNotExist(err) {
		t.Errorf("cleanup left a config.bak beside the config (stat err: %v)", err)
	}
}

// git takes config.lock before it writes the config. Writing around a held
// lock would discard the other writer's change.
func TestRun_HonoursConfigLock(t *testing.T) {
	f := newRepoFixture(t)
	f.git(f.root, "config", "branch.ghost.remote", "origin")
	before := f.readConfig()
	if err := os.WriteFile(f.configPath()+".lock", nil, 0o644); err != nil {
		t.Fatal(err)
	}

	result := Run(&git.GitRunner{}, f.root, Options{Mode: ModeSafe}, discard)

	if stepError(result.Errors, "orphaned-configs") == nil {
		t.Errorf("expected an orphaned-configs error while config.lock is held, got %v", result.Errors)
	}
	if after := f.readConfig(); after != before {
		t.Errorf("config was rewritten while another writer held config.lock:\n%s", after)
	}
}

// The dry run is a promise about the real run. On a repository that mixes
// every case the steps tell apart, both must report the same result.
func TestRun_DryRunReportsWhatTheRealRunDoes(t *testing.T) {
	modes := []struct {
		name string
		opts Options
	}{
		{"safe", Options{Mode: ModeSafe}},
		{"aggressive", Options{Mode: ModeAggressive}},
		{"aggressive with force", Options{Mode: ModeAggressive, Force: true}},
	}
	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			f := newRepoFixture(t)
			// Deleted on the remote, each still waiting for the prune:
			// merged into main; pushed but never merged; holding a local commit.
			for _, name := range []string{"gone/merged", "gone/pushed", "gone/local-work"} {
				f.trackRemoteBranch(name)
			}
			f.commitOn("gone/pushed", true)
			f.commitOn("gone/local-work", false)
			for _, name := range []string{"gone/merged", "gone/pushed", "gone/local-work"} {
				f.git(f.origin, "branch", "--quiet", "-D", name)
			}
			// Never on the remote: one merged, one with its own commit.
			f.git(f.root, "branch", "--quiet", "local/merged", "main")
			f.git(f.root, "branch", "--quiet", "local/work", "main")
			f.commitOn("local/work", false)
			f.git(f.root, "config", "branch.ghost.remote", "origin")
			runner := &git.GitRunner{}

			dryOpts := mode.opts
			dryOpts.DryRun = true
			dry := Run(runner, f.root, dryOpts, discard)
			result := Run(runner, f.root, mode.opts, discard)

			if len(dry.Errors) > 0 || len(result.Errors) > 0 {
				t.Fatalf("unexpected errors: dry %v, real %v", dry.Errors, result.Errors)
			}
			if dry.StaleRefsRemoved != 3 {
				t.Fatalf("fixture is off: %d stale refs, want 3", dry.StaleRefsRemoved)
			}
			if !reflect.DeepEqual(dry, result) {
				t.Errorf("the dry run announced\n  %+v\nbut the real run did\n  %+v", dry, result)
			}
		})
	}
}

// HEAD is per worktree: inside one it names that worktree's branch. A cleanup
// given a worktree path must still protect the repository's default branch,
// which only the HEAD of the bare repository records.
func TestRun_WorktreePathKeepsTheDefaultBranch(t *testing.T) {
	f := newRepoFixture(t)
	// A default branch with a non-standard name, in no worktree, whose
	// upstream is gone: both branch steps have to recognise it.
	f.trackRemoteBranch("trunk")
	f.git(f.origin, "branch", "--quiet", "-D", "trunk")
	f.git(f.root, "symbolic-ref", "HEAD", "refs/heads/trunk")
	worktree := filepath.Join(f.root, "main")
	runner := &git.GitRunner{}

	scan, err := DryRun(runner, worktree)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if slices.Contains(scan.GoneBranches, "trunk") {
		t.Errorf("preview GoneBranches = %v: the default branch must not be listed", scan.GoneBranches)
	}
	for _, candidate := range scan.AggressiveBranches {
		if candidate.Name == "trunk" {
			t.Error("the preview lists the default branch as an aggressive candidate")
		}
	}

	result := Run(runner, worktree, Options{Mode: ModeAggressive, Force: true}, discard)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if !slices.Contains(f.branches(), "trunk") {
		t.Fatal("cleanup run from a worktree path deleted the default branch")
	}
	if want := (SkippedBranch{Name: "trunk", Reason: SkipProtected}); !slices.Contains(result.BranchesSkipped, want) {
		t.Errorf("BranchesSkipped = %v, want it to contain %v", result.BranchesSkipped, want)
	}
}

// With a tag and a branch of the same name, %(refname:short) abbreviates the
// branch to "heads/<name>", which no candidate is called.
func TestDryRun_MetadataOfBranchNamedLikeATag(t *testing.T) {
	f := newRepoFixture(t)
	f.git(f.root, "branch", "--quiet", "rel", "main")
	f.git(f.root, "tag", "rel")

	scan, err := DryRun(&git.GitRunner{}, f.root)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(scan.AggressiveBranches) != 1 || scan.AggressiveBranches[0].Name != "rel" {
		t.Fatalf("AggressiveBranches = %+v, want only rel", scan.AggressiveBranches)
	}
	if got := scan.AggressiveBranches[0]; got.LastCommitSubject != "initial commit" || got.LastCommitDate.IsZero() {
		t.Errorf("candidate rel has no commit metadata: %+v", got)
	}
}

// In a worktree with a detached HEAD, `git branch` lists a "(no branch)"
// entry beside the branches. It is not a branch: cleanup must neither count
// it nor try to delete it.
func TestRun_DetachedWorktreePathHasNoPhantomBranch(t *testing.T) {
	f := newRepoFixture(t)
	detached := filepath.Join(f.root, "detached")
	f.git(f.root, "worktree", "add", "--quiet", "--detach", detached, "main")
	f.git(f.root, "branch", "--quiet", "extra", "main")
	runner := &git.GitRunner{}

	safe := Run(runner, detached, Options{Mode: ModeSafe, DryRun: true}, discard)
	if safe.NonWtBranchesRemaining != 1 {
		t.Errorf("NonWtBranchesRemaining = %d, want 1 (extra)", safe.NonWtBranchesRemaining)
	}

	result := Run(runner, detached, Options{Mode: ModeAggressive, Force: true}, discard)
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if want := []string{"extra"}; !slices.Equal(result.NonWtBranchesDeleted, want) {
		t.Errorf("NonWtBranchesDeleted = %v, want %v", result.NonWtBranchesDeleted, want)
	}
	if len(result.BranchesSkipped) > 0 {
		t.Errorf("BranchesSkipped = %v, want none", result.BranchesSkipped)
	}
}
