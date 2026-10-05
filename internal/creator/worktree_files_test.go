package creator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
	"github.com/bonboncinnabon/sentei/internal/worktreefile"
)

func TestPrepareCreationPlansWorktreeFilesInSetup(t *testing.T) {
	prepared, err := prepareCreation(&mock.Runner{}, &mock.Runner{}, Options{
		BranchName: "feature/files",
		BaseBranch: "main",
		RepoPath:   "/repo",
		MergeBase:  true,
		WorktreeFiles: []worktreefile.Rule{{
			Source: "private/config", Destination: ".codex/config.toml",
		}},
	})
	if err != nil {
		t.Fatalf("prepareCreation() error: %v", err)
	}

	steps := prepared.plan.Phases[0].Steps
	var labels []string
	for _, step := range steps {
		labels = append(labels, step.Label)
	}
	want := []string{"Create worktree", "Merge base branch", "Copy worktree files"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Fatalf("Setup steps = %v, want %v", labels, want)
	}
}

func TestPrepareCreationWithNoWorktreeFilesPreservesSetupPlan(t *testing.T) {
	prepared, err := prepareCreation(&mock.Runner{}, &mock.Runner{}, Options{
		BranchName: "feature/plain",
		BaseBranch: "main",
		RepoPath:   "/repo",
	})
	if err != nil {
		t.Fatalf("prepareCreation() error: %v", err)
	}
	if got := len(prepared.plan.Phases[0].Steps); got != 1 {
		t.Fatalf("Setup step count = %d, want 1", got)
	}
}

func TestRunCopiesConfiguredWorktreeFiles(t *testing.T) {
	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, "feature-files")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "private"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "private", "config"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s:[show-ref --verify refs/heads/feature/files]", repoRoot):          {Err: errors.New("missing")},
		fmt.Sprintf("%s:[worktree add %s -b feature/files main]", repoRoot, worktreePath): {},
	}}

	result := Run(runner, runner, Options{
		BranchName: "feature/files",
		BaseBranch: "main",
		RepoPath:   repoRoot,
		WorktreeFiles: []worktreefile.Rule{{
			Source: "private/config", Destination: ".codex/config.toml",
		}},
	}, func(progress.Event) {})

	if result.HasFailures() {
		t.Fatalf("Run() failures: %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(worktreePath, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "private" {
		t.Errorf("copied content = %q", data)
	}
	step := creatorResultStep(t, result, "Copy worktree files")
	if step.Status != progress.StepDone || !strings.Contains(step.Message, ".codex/config.toml") {
		t.Fatalf("copy step = %#v", step)
	}
}

func TestRunWorktreeFileFailureDoesNotBlockIndependentSetup(t *testing.T) {
	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, "feature-files")
	sourceWorktree := t.TempDir()
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s:shell[tool detect]", sourceWorktree):                              {},
		fmt.Sprintf("%s:[show-ref --verify refs/heads/feature/files]", repoRoot):          {Err: errors.New("missing")},
		fmt.Sprintf("%s:[worktree add %s -b feature/files main]", repoRoot, worktreePath): {},
		fmt.Sprintf("%s:shell[go mod download]", worktreePath):                            {},
		fmt.Sprintf("%s:shell[tool setup]", worktreePath):                                 {},
	}}

	result := Run(runner, runner, Options{
		BranchName:     "feature/files",
		BaseBranch:     "main",
		RepoPath:       repoRoot,
		SourceWorktree: sourceWorktree,
		WorktreeFiles: []worktreefile.Rule{{
			Source: "missing", Destination: ".codex/config.toml",
		}},
		Ecosystems: []config.EcosystemConfig{{
			Name: "go", Install: config.InstallConfig{Command: "go mod download"},
		}},
		Integrations: []integration.Integration{{
			Name:   "tool",
			Detect: integration.DetectSpec{Command: "tool detect"},
			Setup:  integration.SetupSpec{Command: "tool setup", WorkingDir: "worktree"},
		}},
	}, func(progress.Event) {})

	if !result.HasFailures() {
		t.Fatal("missing worktree file did not fail creation result")
	}
	copyStep := creatorResultStep(t, result, "Copy worktree files")
	if copyStep.Status != progress.StepFailed || !strings.Contains(copyStep.Error.Error(), `source "missing" does not exist`) {
		t.Fatalf("copy step = %#v", copyStep)
	}
	for _, name := range []string{"go", "Setup tool"} {
		if step := creatorResultStep(t, result, name); step.Status != progress.StepDone {
			t.Fatalf("independent step %q = %#v", name, step)
		}
	}
}
