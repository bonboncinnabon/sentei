package creator

import (
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktreefile"
)

// Step identities are plain strings that say what the step is. They are
// shown in the TUI's progress details, so they hold no control characters.
func TestPrepareCreation_StepIDsArePlainStrings(t *testing.T) {
	prepared, err := prepareCreation(Options{
		BranchName: "feature/x", BaseBranch: "main", RepoPath: "/repo",
		MergeBase:     true,
		WorktreeFiles: []worktreefile.Rule{{Source: ".sentei/a.toml", Destination: ".tool/a.toml"}},
		CopyEnvFiles:  true,
		EnvFiles:      []string{".env", ".env.local"},
		Ecosystems: []config.EcosystemConfig{
			{Name: "pnpm", Install: config.InstallConfig{Command: "pnpm install"}},
			{Name: "go", Install: config.InstallConfig{Command: "go mod download"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got []progress.StepID
	for _, phase := range prepared.plan.Phases {
		for _, step := range phase.Steps {
			got = append(got, step.ID)
			if strings.ContainsFunc(step.ID, unicode.IsControl) {
				t.Errorf("step ID %q holds a control character", step.ID)
			}
		}
	}
	want := []progress.StepID{
		"create-worktree:feature/x", "merge-base:main", "copy-worktree-files", "copy-env",
		"install-dependencies:pnpm", "install-dependencies:go",
	}
	if !slices.Equal(got, want) {
		t.Errorf("step IDs = %q, want %q", got, want)
	}
}
