package cleanup

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
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

// git reads a config written with CRLF line endings; the section of a live
// branch must survive there too.
func TestPurgeOrphanedBranchConfigs_CRLFKeepsLiveBranches(t *testing.T) {
	f := newRepoFixture(t)
	f.git(f.root, "config", "branch.main.remote", "origin")
	f.git(f.root, "config", "branch.gone.remote", "origin")
	crlf := strings.ReplaceAll(f.readConfig(), "\n", "\r\n")
	if err := os.WriteFile(f.configPath(), []byte(crlf), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := PurgeOrphanedBranchConfigs(&git.GitRunner{}, f.root, Options{}, func(Event) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Removed != 1 {
		t.Errorf("Removed = %d, want 1 (only the section for the deleted branch)", result.Removed)
	}
	after := f.readConfig()
	if !strings.Contains(after, `[branch "main"]`) {
		t.Errorf("the live branch's section was purged from a CRLF config:\n%q", after)
	}
	if strings.Contains(after, `[branch "gone"]`) {
		t.Errorf("the orphaned section survived in a CRLF config:\n%q", after)
	}
}

// With a tag and a branch of the same name, %(refname:short) abbreviates the
// branch to "heads/<name>", so its config section looked orphaned.
func TestPurgeOrphanedBranchConfigs_TagNamedLikeBranch(t *testing.T) {
	f := newRepoFixture(t)
	f.git(f.root, "branch", "rel")
	f.git(f.root, "tag", "rel")
	f.git(f.root, "config", "branch.rel.remote", "origin")
	f.git(f.root, "config", "branch.rel.merge", "refs/heads/rel")

	result, err := PurgeOrphanedBranchConfigs(&git.GitRunner{}, f.root, Options{}, func(Event) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Removed != 0 {
		t.Errorf("Removed = %d, want 0: branch rel exists", result.Removed)
	}
	if after := f.readConfig(); !strings.Contains(after, `[branch "rel"]`) {
		t.Errorf("tracking config for the existing branch rel was purged:\n%s", after)
	}
}
