package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

// Every migrate step uses the repo path both as git's -C directory and inside
// arguments resolved against it. A relative path was applied twice: the bare
// clone landed in <path>/<path>/.bare and the original .git was then removed.
func TestPrepareMigrate_ResolvesRelativeRepoPath(t *testing.T) {
	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, "myrepo"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(parent)

	for _, relative := range []string{"myrepo", "myrepo/", "./myrepo", "."} {
		prepared := prepareMigrate(&mock.Runner{}, nil, MigrateOptions{RepoPath: relative})

		root := prepared.result.BareRoot
		if !filepath.IsAbs(root) {
			t.Errorf("RepoPath %q: bare root %q is not absolute", relative, root)
		}
		if root != filepath.Clean(root) {
			t.Errorf("RepoPath %q: bare root %q is not clean (a trailing slash puts the backup inside the repo)", relative, root)
		}
	}
}

func TestMigrate_RefusesLinkedWorktree(t *testing.T) {
	dir := t.TempDir()
	pointer := filepath.Join(dir, ".git")
	if err := os.WriteFile(pointer, []byte("gitdir: /elsewhere/proj.git/worktrees/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &mock.Runner{}

	result := Migrate(runner, nil, MigrateOptions{RepoPath: dir}, nil)

	if result.Err == nil || !strings.Contains(result.Err.Error(), "linked worktree") {
		t.Fatalf("Err = %v, want a refusal naming the linked worktree", result.Err)
	}
	for _, call := range runner.Calls {
		if strings.Contains(call, "clone") {
			t.Errorf("no migration step may run for a linked worktree, got call %q", call)
		}
	}
	if info, err := os.Lstat(pointer); err != nil || info.IsDir() {
		t.Errorf("the worktree's .git pointer must be left untouched (err=%v)", err)
	}
}
