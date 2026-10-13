package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/testutil/layouts"
)

// The tests in this file hold each command to the same result whichever path
// names the repository: the bare directory, the container of a .bare layout,
// a worktree or a directory inside one. Worktrees of a bare directory not
// named .bare used to be refused as regular repositories.

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestRunRemove_SameFromEveryEntryPoint(t *testing.T) {
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		// The shell stands in the feature worktree whichever path is given.
		t.Chdir(layout.Feature)

		var err error
		preview := captureStdout(t, func() {
			captureStderr(t, func() {
				err = RunRemove([]string{"--all", "--dry-run", entry})
			})
		})
		if err != nil {
			t.Fatalf("remove --dry-run: %v", err)
		}
		if !strings.Contains(preview, "Would remove 1 worktree(s)") || !strings.Contains(preview, "scratch") {
			t.Errorf("the preview must name scratch and nothing else, got:\n%s", preview)
		}

		captureStdout(t, func() {
			captureStderr(t, func() {
				err = RunRemove([]string{"--all", entry})
			})
		})
		if err != nil {
			t.Fatalf("remove: %v", err)
		}
		if exists(layout.Scratch) {
			t.Error("scratch matches --all and must be removed")
		}
		if !exists(layout.Default) {
			t.Error("the default branch's worktree must never be removed")
		}
		if !exists(layout.Feature) {
			t.Error("the worktree holding the current directory must never be removed")
		}
	})
}

func TestRunCreate_PlacesTheWorktreeAtTheRootFromEveryEntryPoint(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		// The config and the file it names sit at the root. The file reaching
		// the new worktree shows both were found where the worktree was put.
		mustWriteFile(t, filepath.Join(layout.Root, "private.toml"), "private = true\n")
		mustWriteFile(t, filepath.Join(layout.Root, ".sentei.yaml"),
			"worktree_files:\n  - source: private.toml\n    destination: private.toml\n")

		var err error
		captureStdout(t, func() {
			err = RunCreate([]string{"--branch", "topic/new", "--base", layouts.DefaultBranch, entry})
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		created := filepath.Join(layout.Root, "topic-new")
		if !exists(filepath.Join(created, ".git")) {
			t.Fatalf("the new worktree must be created at the root, as %s", created)
		}
		if !exists(filepath.Join(created, "private.toml")) {
			t.Error("the root's .sentei.yaml was not applied to the new worktree")
		}
	})
}

// Cleanup acts on the whole repository. Run inside a worktree, git judges
// "merged" against that worktree's branch, not the default branch.
func TestRunCleanup_JudgesMergedAgainstTheDefaultBranchFromEveryEntryPoint(t *testing.T) {
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		mustGit(t, layout.Feature, "commit", "--allow-empty", "-m", "feature work")
		// Neither is checked out anywhere. wip is merged into feature but not
		// into the default branch; landed is merged into the default branch.
		mustGit(t, layout.Bare, "branch", "wip", "feature")
		mustGit(t, layout.Bare, "branch", "landed", layouts.DefaultBranch)

		var err error
		captureStdout(t, func() {
			err = RunCleanup([]string{"--mode", "aggressive", entry})
		})
		if err != nil {
			t.Fatalf("cleanup: %v", err)
		}

		runner := &git.GitRunner{}
		if !git.BranchExists(runner, layout.Bare, "wip") {
			t.Error("wip is not merged into the default branch and must be kept")
		}
		if git.BranchExists(runner, layout.Bare, "landed") {
			t.Error("landed is merged into the default branch and must be deleted")
		}
	})
}

// ecosystems shows the configuration create applies, so it reads .sentei.yaml
// from the same root. It is given no path here: started in the .bare
// directory it used to read .bare/.sentei.yaml.
func TestRunEcosystems_ReadsTheRootConfigFromEveryEntryPoint(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		mustWriteFile(t, filepath.Join(layout.Root, ".sentei.yaml"),
			"ecosystems:\n  - name: from-the-root\n    detect:\n      files: [root.lock]\n")
		t.Chdir(entry)

		var err error
		out := captureStdout(t, func() {
			err = RunEcosystems(nil)
		})
		if err != nil {
			t.Fatalf("ecosystems: %v", err)
		}
		if !strings.Contains(out, "from-the-root") {
			t.Errorf("the root's .sentei.yaml was not read:\n%s", out)
		}
	})
}
