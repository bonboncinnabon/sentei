package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/layouts"
)

// Any path into a repository whose shared git directory is bare is an entry
// point, whatever that directory is called and wherever the worktrees live.
// Worktrees of a bare directory not named .bare used to be taken for regular
// repositories, and every command refused them.
func TestDetectContext_BareLayoutFromEveryEntryPoint(t *testing.T) {
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		if got := DetectContext(realGit, entry); got != ContextBareRepo {
			t.Errorf("DetectContext(%q) = %v, want %v", entry, got, ContextBareRepo)
		}
		if got := ResolveBareRoot(realGit, entry); got != layout.Root {
			t.Errorf("ResolveBareRoot(%q) = %q, want %q", entry, got, layout.Root)
		}
	})
}

// A regular clone is what migrate converts. Linked worktrees do not make it a
// bare layout: its shared git directory has a working tree of its own.
func TestDetectContext_RegularRepositoryAndNoRepository(t *testing.T) {
	_, regular := newCheckout(t)
	linked := filepath.Join(t.TempDir(), "linked")
	mustGit(t, regular, "worktree", "add", "--quiet", "-b", "linked", linked)

	tests := []struct {
		name string
		path string
		want RepoContext
	}{
		{"main working tree", regular, ContextNonBareRepo},
		{"directory inside the working tree", filepath.Join(regular, "src", "deep"), ContextNonBareRepo},
		{"its .git directory", filepath.Join(regular, ".git"), ContextNonBareRepo},
		{"linked worktree", linked, ContextNonBareRepo},
		{"directory outside any repository", t.TempDir(), ContextNoRepo},
		{"path that does not exist", filepath.Join(t.TempDir(), "missing"), ContextNoRepo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectContext(realGit, tt.path); got != tt.want {
				t.Errorf("DetectContext(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// git sparse-checkout moves core.bare of a bare repository into
// config.worktree. Git still answers "bare" there; the config file alone does
// not say so.
func TestDetectContext_BareSettingInWorktreeConfig(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "project.git")
	worktree := filepath.Join(t.TempDir(), "main")
	mustGit(t, filepath.Dir(bare), "init", "--quiet", "--bare", bare)
	mustGit(t, bare, "worktree", "add", "--quiet", "--orphan", "-b", "main", worktree)
	mustGit(t, bare, "config", "extensions.worktreeConfig", "true")
	mustGit(t, bare, "config", "--unset", "core.bare")
	mustGit(t, bare, "config", "--worktree", "core.bare", "true")

	for _, entry := range []string{bare, worktree} {
		if got := DetectContext(realGit, entry); got != ContextBareRepo {
			t.Errorf("DetectContext(%q) = %v, want %v", entry, got, ContextBareRepo)
		}
	}
}

// With safe.bareRepository=explicit git refuses a bare directory it has to
// discover, so the shared directory is named to git, never entered. A layout
// reached through a worktree or through the container's .git pointer is still
// a bare layout; asked inside the directory, git refused and every command
// took the layout for a regular repository.
func TestDetectContext_WhenBareRepositoriesMustBeExplicit(t *testing.T) {
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		if entry == layout.Bare {
			return // git itself finds no repository when started in the bare directory
		}
		gitConfigEnv(t, "safe.bareRepository", "explicit")
		if got := DetectContext(realGit, entry); got != ContextBareRepo {
			t.Errorf("DetectContext(%q) = %v, want %v", entry, got, ContextBareRepo)
		}
	})
}

// A regular clone whose config has no core.bare is still a regular clone.
// Started inside such a .git directory git guesses "bare", and remove and
// create would then run against an ordinary checkout.
func TestDetectContext_RegularRepositoryWithoutCoreBare(t *testing.T) {
	_, regular := newCheckout(t)
	linked := filepath.Join(t.TempDir(), "linked")
	mustGit(t, regular, "worktree", "add", "--quiet", "-b", "linked", linked)
	mustGit(t, regular, "config", "--unset", "core.bare")

	for _, entry := range []string{regular, linked, filepath.Join(regular, ".git")} {
		if got := DetectContext(realGit, entry); got != ContextNonBareRepo {
			t.Errorf("DetectContext(%q) = %v, want %v", entry, got, ContextNonBareRepo)
		}
	}
}

// A layout git will not enter is not "no repository": an error says why and
// how to fix it, and following the fix makes the layout an entry point again.
// sentei never enters it itself: git passes over these on purpose.
func TestUnreachable_SaysHowToReachALayoutGitPassesOver(t *testing.T) {
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		if err := Unreachable(realGit, entry); err != nil {
			t.Fatalf("Unreachable(%q) = %v for a layout git enters", entry, err)
		}

		switch entry {
		case layout.Root:
			if layout.Root == layout.Bare || filepath.Base(layout.Bare) != ".bare" {
				return
			}
			pointer := filepath.Join(layout.Root, ".git")
			if err := os.Remove(pointer); err != nil {
				t.Fatal(err)
			}
			err := Unreachable(realGit, entry)
			if err == nil || !strings.Contains(err.Error(), "no .git file") {
				t.Fatalf("Unreachable without the pointer = %v, want it to name the missing .git file", err)
			}
			// The fix the message prints, run as printed.
			_, fix, _ := strings.Cut(err.Error(), "recreate the file with: ")
			if out, err := exec.Command("sh", "-c", fix).CombinedOutput(); err != nil {
				t.Fatalf("running the printed fix %q: %v\n%s", fix, err, out)
			}
			if got := DetectContext(realGit, entry); got != ContextBareRepo {
				t.Errorf("after the printed fix DetectContext = %v, want %v", got, ContextBareRepo)
			}

		case layout.Bare:
			gitConfigEnv(t, "safe.bareRepository", "explicit")
			err := Unreachable(realGit, entry)
			if err == nil || !strings.Contains(err.Error(), "safe.bareRepository") || !strings.Contains(err.Error(), "worktrees") {
				t.Errorf("Unreachable in the bare directory under explicit = %v, want the setting and the way in", err)
			}
		}
	})

	if err := Unreachable(realGit, t.TempDir()); err != nil {
		t.Errorf("an empty directory holds no layout, got %v", err)
	}
}
