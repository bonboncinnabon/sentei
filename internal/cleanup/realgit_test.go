package cleanup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// repoFixture is the layout sentei manages, built with real git under
// t.TempDir(): a remote, and a repository root holding .bare, the .git
// pointer and a worktree for main.
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
