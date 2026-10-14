package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseRemoveFlags_Branch(t *testing.T) {
	opts, err := ParseRemoveFlags([]string{"--branch", "feat/a", "--branch=feat/b", "/repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(opts.Branches, []string{"feat/a", "feat/b"}) {
		t.Errorf("Branches = %q, want both names in order", opts.Branches)
	}
	if !opts.HasFilter() || ValidateRemoveForNonInteractive(opts) != nil {
		t.Error("--branch alone must count as a filter")
	}
	if got, want := RemoveCLICommand(opts), "sentei remove --branch feat/a --branch feat/b /repo"; got != want {
		t.Errorf("RemoveCLICommand = %q, want %q", got, want)
	}
	if got, want := FormatFilterLabel(opts), "branch feat/a, feat/b"; got != want {
		t.Errorf("FormatFilterLabel = %q, want %q", got, want)
	}

	if _, err := ParseRemoveFlags([]string{"--branch", ""}); err == nil {
		t.Error("an empty --branch must be refused, not match nothing")
	}
}

// setupTwoFeatureWorktrees extends setupBareRepoWithMergedBranch with a second
// feature worktree and a main worktree, and returns the repository and the
// paths of the three worktrees.
func setupTwoFeatureWorktrees(t *testing.T) (repo, merged, other, main string) {
	t.Helper()
	repo = setupBareRepoWithMergedBranch(t)
	merged = filepath.Join(repo, "feature-merged-branch")
	other = filepath.Join(repo, "feature-other")
	main = filepath.Join(repo, "main")
	mustGit(t, repo, "worktree", "add", "-b", "feature/other", other, "main")
	mustGit(t, repo, "worktree", "add", main, "main")
	return repo, merged, other, main
}

func TestRunRemove_BranchRemovesOnlyThatWorktree(t *testing.T) {
	repo, merged, other, main := setupTwoFeatureWorktrees(t)

	var err error
	out := captureStdout(t, func() {
		err = RunRemove([]string{"--branch", "feature/other", repo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(other); !os.IsNotExist(statErr) {
		t.Errorf("the named worktree %s must be removed", other)
	}
	for _, kept := range []string{merged, main} {
		if _, statErr := os.Stat(kept); statErr != nil {
			t.Errorf("%s is not named and must survive: %v", kept, statErr)
		}
	}
}

func TestRunRemove_BranchKeepsTheAtRiskGate(t *testing.T) {
	repo, _, other, _ := setupTwoFeatureWorktrees(t)
	mustWriteFile(t, filepath.Join(other, "notes.txt"), "precious\n")

	err := RunRemove([]string{"--branch", "feature/other", repo})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("want the at-risk gate to refuse an untracked file without --force, got %v", err)
	}
	if _, statErr := os.Stat(other); statErr != nil {
		t.Fatalf("the gate must leave the worktree in place: %v", statErr)
	}

	captureStdout(t, func() {
		err = RunRemove([]string{"--branch", "feature/other", "--force", repo})
	})
	if err != nil {
		t.Fatalf("--force must remove it: %v", err)
	}
	if _, statErr := os.Stat(other); !os.IsNotExist(statErr) {
		t.Error("the worktree should be removed with --force")
	}
}

// A filter passes over what it may not remove. A branch named on the command
// line is either removed or reported, so the exit status says which.
func TestRunRemove_BranchRefusesWhatItCannotRemove(t *testing.T) {
	tests := []struct {
		name   string
		branch []string
		cwd    func(merged string) string
		want   []string
	}{
		{name: "no such worktree", branch: []string{"feature/missing"}, want: []string{`"feature/missing"`, "no worktree"}},
		{name: "protected", branch: []string{"main"}, want: []string{`"main"`, "protected"}},
		{name: "current directory", branch: []string{"feature/merged-branch"},
			cwd: func(merged string) string { return merged }, want: []string{"current directory"}},
		{name: "every name at once", branch: []string{"feature/missing", "main", "feature/other"},
			want: []string{"feature/missing", "protected"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, merged, other, main := setupTwoFeatureWorktrees(t)
			if tt.cwd != nil {
				t.Chdir(tt.cwd(merged))
			}
			args := []string{"--dry-run"}
			for _, b := range tt.branch {
				args = append(args, "--branch", b)
			}
			for _, dryRun := range []bool{true, false} {
				run := args
				if !dryRun {
					run = args[1:]
				}
				var err error
				captureStdout(t, func() { err = RunRemove(append(run, repo)) })
				if err == nil {
					t.Fatalf("dry run %v: want a refusal naming %q", dryRun, tt.want)
				}
				for _, want := range tt.want {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("dry run %v: error %q missing %q", dryRun, err, want)
					}
				}
			}
			for _, kept := range []string{merged, other, main} {
				if _, statErr := os.Stat(kept); statErr != nil {
					t.Errorf("a refused run must remove nothing; %s: %v", kept, statErr)
				}
			}
		})
	}
}
