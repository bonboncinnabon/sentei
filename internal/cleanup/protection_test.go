package cleanup

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

// The repo's real default branch may be non-standard. When no worktree holds
// it, the worktree set alone would let aggressive cleanup delete it.
func TestListNonWorktreeCandidates_ProtectsDetectedDefaultBranch(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[worktree list --porcelain]":           {Output: "worktree /repo/.bare\nbare\n\nworktree /repo/feature\nHEAD abc123\nbranch refs/heads/feature"},
		"/repo:[branch --format=%(refname:lstrip=2)]": {Output: "feature\ntrunk\nstale"},
		"/repo:[symbolic-ref --short HEAD]":           {Output: "trunk"},
	}}

	got, err := listNonWorktreeCandidates(runner, "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"stale"}; !slices.Equal(got, want) {
		t.Errorf("candidates = %v, want %v (trunk is the default branch)", got, want)
	}
}

func TestPurgeOrphanedBranchConfigs_CRLFKeepsLiveBranches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	content := "[core]\r\n\tbare = true\r\n[branch \"main\"]\r\n\tremote = origin\r\n[branch \"gone\"]\r\n\tremote = origin\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[branch --format=%(refname:lstrip=2)]": {Output: "main"},
	}}

	result, err := PurgeOrphanedBranchConfigs(runner, "/repo", path, Options{}, func(Event) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Removed != 1 {
		t.Errorf("Removed = %d, want 1 (only the section for the deleted branch)", result.Removed)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), `[branch "main"]`) {
		t.Errorf("the live branch's section was purged from a CRLF config:\n%q", after)
	}
}

// With a tag and a branch of the same name, %(refname:short) abbreviates the
// branch to "heads/<name>", so its config section looked orphaned.
func TestPurgeOrphanedBranchConfigs_TagNamedLikeBranch(t *testing.T) {
	dir := t.TempDir()
	gitIn := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = testtmp.HermeticGitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitIn("init", "--initial-branch=main")
	gitIn("-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "--allow-empty", "-m", "initial commit")
	gitIn("branch", "rel")
	gitIn("tag", "rel")
	gitIn("config", "branch.rel.remote", "origin")
	gitIn("config", "branch.rel.merge", "refs/heads/rel")
	configPath := filepath.Join(dir, ".git", "config")

	result, err := PurgeOrphanedBranchConfigs(&git.GitRunner{}, dir, configPath, Options{}, func(Event) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Removed != 0 {
		t.Errorf("Removed = %d, want 0: branch rel exists", result.Removed)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), `[branch "rel"]`) {
		t.Errorf("tracking config for the existing branch rel was purged:\n%s", after)
	}
}
