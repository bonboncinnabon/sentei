// Package layouts builds, with real git, each repository layout sentei treats
// as a bare layout, and names every path sentei can be started from in it.
// Tests use it to hold a behaviour to the same result from every entry point.
// It imports no production package, so any package's tests can use it.
package layouts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

// DefaultBranch is every layout's default branch. No built-in protection
// covers the name, so only detection of the real default branch protects it.
const DefaultBranch = "trunk"

// Layout is one bare layout on disk. Three clean worktrees hold the branches
// trunk, feature and scratch. There is no remote, so none of them is at risk.
type Layout struct {
	// Root is where new worktrees are created and .sentei.yaml is read.
	Root string
	// Bare is the shared git directory.
	Bare string
	// Default, Feature and Scratch are the worktree directories.
	Default, Feature, Scratch string
}

type entry struct {
	name string
	path func(Layout) string
}

var kinds = []struct {
	name string
	// bare and worktrees are relative to the layout's temp dir.
	bare, worktrees string
	// container layouts have a .git pointer beside the bare directory, and
	// the directory holding both is the root.
	container bool
}{
	{name: ".bare in a container", bare: "project/.bare", worktrees: "project", container: true},
	{name: "project.git with worktrees inside", bare: "project.git", worktrees: "project.git"},
	{name: "project.git with worktrees beside", bare: "project.git", worktrees: "."},
}

// EachEntry runs fn as a subtest once per layout and entry point, each time
// with a freshly built layout, so fn may change the repository.
func EachEntry(t *testing.T, fn func(t *testing.T, layout Layout, entry string)) {
	t.Helper()
	for _, kind := range kinds {
		entries := []entry{
			{"the bare directory", func(l Layout) string { return l.Bare }},
			{"a worktree", func(l Layout) string { return l.Feature }},
			{"a directory inside a worktree", func(l Layout) string { return filepath.Join(l.Feature, "nested") }},
		}
		if kind.container {
			entries = append(entries, entry{"the container", func(l Layout) string { return l.Root }})
		}
		for _, e := range entries {
			t.Run(kind.name+"/from "+e.name, func(t *testing.T) {
				layout := build(t, kind.bare, kind.worktrees, kind.container)
				fn(t, layout, e.path(layout))
			})
		}
	}
}

func build(t *testing.T, bare, worktrees string, container bool) Layout {
	t.Helper()
	// Git reports worktree paths with symlinks resolved, and on macOS the temp
	// dir is reached through one. Resolving it here lets tests compare paths.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	layout := Layout{
		Bare:    filepath.Join(dir, bare),
		Default: filepath.Join(dir, worktrees, DefaultBranch),
		Feature: filepath.Join(dir, worktrees, "feature"),
		Scratch: filepath.Join(dir, worktrees, "scratch"),
	}
	layout.Root = layout.Bare
	git(t, dir, "init", "--quiet", "--bare", "--initial-branch="+DefaultBranch, layout.Bare)
	if container {
		layout.Root = filepath.Dir(layout.Bare)
		if err := os.WriteFile(filepath.Join(layout.Root, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, layout.Bare, "worktree", "add", "--quiet", "--orphan", "-b", DefaultBranch, layout.Default)
	git(t, layout.Default, "commit", "--quiet", "--allow-empty", "-m", "initial commit")
	git(t, layout.Bare, "worktree", "add", "--quiet", "-b", "feature", layout.Feature, DefaultBranch)
	git(t, layout.Bare, "worktree", "add", "--quiet", "-b", "scratch", layout.Scratch, DefaultBranch)
	// An empty directory is invisible to git status: the worktree stays clean.
	if err := os.Mkdir(filepath.Join(layout.Feature, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	return layout
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = testtmp.HermeticGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, out)
	}
}
