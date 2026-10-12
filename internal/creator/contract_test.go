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
)

func TestRun_ProgressCallbackPanicIsReturned(t *testing.T) {
	emitErr := errors.New("delivery failed")
	result := Run(&mock.Runner{}, &mock.Runner{}, Options{
		BranchName: "feature/callback", BaseBranch: "main", RepoPath: "/repo",
	}, func(progress.Event) { panic(emitErr) })

	if !errors.Is(result.Err, emitErr) {
		t.Fatalf("Err = %v, want wrapped callback error %v", result.Err, emitErr)
	}
	if len(result.Phases) != 0 {
		t.Fatalf("Phases = %#v, want no projection when Start cannot deliver", result.Phases)
	}
}

func TestRun_RejectsDuplicateEcosystemIdentityBeforeExecution(t *testing.T) {
	runner := &mock.Runner{}
	result := Run(runner, runner, Options{
		BranchName: "feature/duplicate", BaseBranch: "main", RepoPath: "/repo",
		Ecosystems: []config.EcosystemConfig{
			{Name: "node", Install: config.InstallConfig{Command: "npm install"}},
			{Name: "node", Install: config.InstallConfig{Command: "pnpm install"}},
		},
	}, func(progress.Event) {})

	if result.Err == nil || !strings.Contains(result.Err.Error(), "duplicate ecosystem identity") {
		t.Fatalf("Err = %v", result.Err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("calls = %v, want none before rejecting identity conflict", runner.Calls)
	}
}

func TestRun_ResultPhasesMatchCompletedEventStream(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[show-ref --verify refs/heads/feature/parity]":              {Err: errors.New("missing")},
		"/repo:[worktree add /repo/feature-parity -b feature/parity main]": {},
		"/repo/feature-parity:shell[go mod download]":                      {},
	}}
	var events []progress.Event
	result := Run(runner, runner, Options{
		BranchName: "feature/parity", BaseBranch: "main", RepoPath: "/repo",
		Ecosystems: []config.EcosystemConfig{{Name: "go", Install: config.InstallConfig{Command: "go mod download"}}},
	}, func(event progress.Event) { events = append(events, event) })
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if err := progress.ValidateStream(events); err != nil {
		t.Fatalf("invalid stream: %v", err)
	}
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			found := false
			for i := len(events) - 1; i >= 0; i-- {
				if events[i].Phase == phase.ID && events[i].Step == step.ID && !events[i].Close {
					found = true
					if events[i].Status != step.Status {
						t.Fatalf("%s/%s result status %v != stream status %v", phase.ID, step.ID, step.Status, events[i].Status)
					}
					break
				}
			}
			if !found {
				t.Fatalf("result step %s/%s missing from stream", phase.ID, step.ID)
			}
		}
	}
}

func TestRun_MergeFailureDoesNotBlockEnvCopy(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, ".env"), []byte("SAFE=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	worktree := filepath.Join(repo, "feature-independent")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s:[show-ref --verify refs/heads/feature/independent]", repo):      {Err: errors.New("missing")},
		fmt.Sprintf("%s:[worktree add %s -b feature/independent main]", repo, worktree): {},
		fmt.Sprintf("%s:[merge main --no-edit]", worktree):                              {Err: errors.New("conflict")},
	}}
	result := Run(runner, runner, Options{
		BranchName: "feature/independent", BaseBranch: "main", RepoPath: repo,
		SourceWorktree: source, MergeBase: true, CopyEnvFiles: true,
		Ecosystems: []config.EcosystemConfig{{Name: "node", EnvFiles: []string{".env"}}},
	}, func(progress.Event) {})
	if _, err := os.Stat(filepath.Join(worktree, ".env")); err != nil {
		t.Fatalf("env copy was blocked by merge failure: %v", err)
	}
	if !result.HasFailures() {
		t.Fatal("merge failure was not recorded")
	}
}

func TestRun_IndependentSetupFailureDoesNotBlockDependenciesOrIntegrations(t *testing.T) {
	tests := []struct {
		name           string
		mergeErr       error
		makeSourceEnv  func(*testing.T, string)
		failedStepName string
	}{
		{
			name: "merge failure", mergeErr: errors.New("conflict"), failedStepName: "Merge base branch",
			makeSourceEnv: func(t *testing.T, source string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(source, ".env"), []byte("SAFE=1"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "env copy failure", failedStepName: "Copy env files",
			makeSourceEnv: func(t *testing.T, source string) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(source, ".env"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := t.TempDir()
			tc.makeSourceEnv(t, source)
			repo := t.TempDir()
			worktree := filepath.Join(repo, "feature-fallbacks")
			if err := os.MkdirAll(worktree, 0o755); err != nil {
				t.Fatal(err)
			}
			responses := map[string]mock.Response{
				fmt.Sprintf("%s:shell[tool detect]", source):                                  {},
				fmt.Sprintf("%s:[show-ref --verify refs/heads/feature/fallbacks]", repo):      {Err: errors.New("missing")},
				fmt.Sprintf("%s:[worktree add %s -b feature/fallbacks main]", repo, worktree): {},
				fmt.Sprintf("%s:[merge main --no-edit]", worktree):                            {Err: tc.mergeErr},
				fmt.Sprintf("%s:shell[go mod download]", worktree):                            {},
				fmt.Sprintf("%s:shell[tool setup]", worktree):                                 {},
			}
			runner := &mock.Runner{Responses: responses}
			result := Run(runner, runner, Options{
				BranchName: "feature/fallbacks", BaseBranch: "main", RepoPath: repo,
				SourceWorktree: source, MergeBase: true, CopyEnvFiles: true,
				Ecosystems: []config.EcosystemConfig{{
					Name: "go", EnvFiles: []string{".env"}, Install: config.InstallConfig{Command: "go mod download"},
				}},
				Integrations: []integration.Integration{{
					Name: "tool", Detect: integration.DetectSpec{Command: "tool detect"},
					Setup: integration.SetupSpec{Command: "tool setup", WorkingDir: "worktree"},
				}},
			}, func(progress.Event) {})

			if result.Err != nil {
				t.Fatalf("Err = %v", result.Err)
			}
			if step := creatorResultStep(t, result, tc.failedStepName); step.Status != progress.StepFailed {
				t.Fatalf("failed setup result = %#v", step)
			}
			for _, name := range []string{"go", "Setup tool"} {
				if step := creatorResultStep(t, result, name); step.Status != progress.StepDone {
					t.Fatalf("independent step %q = %#v", name, step)
				}
			}
		})
	}
}

func creatorResultStep(t *testing.T, result Result, name string) progress.StepResult {
	t.Helper()
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Name == name {
				return step
			}
		}
	}
	t.Fatalf("step %q not found in %#v", name, result.Phases)
	return progress.StepResult{}
}

func TestRun_WorktreeFailureBlocksAllPreparedExecution(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:shell[tool detect]":                                           {},
		"/repo:[show-ref --verify refs/heads/feature/blocked]":               {Err: errors.New("missing")},
		"/repo:[worktree add /repo/feature-blocked -b feature/blocked main]": {Err: errors.New("cannot create")},
	}}
	result := Run(runner, runner, Options{
		BranchName: "feature/blocked", BaseBranch: "main", RepoPath: "/repo", MergeBase: true,
		Ecosystems: []config.EcosystemConfig{{Name: "go", Install: config.InstallConfig{Command: "go mod download"}}},
		Integrations: []integration.Integration{{
			Name: "tool", Detect: integration.DetectSpec{Command: "tool detect"},
			Setup: integration.SetupSpec{Command: "tool setup", WorkingDir: "worktree"},
		}},
	}, func(progress.Event) {})
	for _, call := range runner.Calls {
		if strings.Contains(call, "merge main") || strings.Contains(call, "go mod download") || strings.Contains(call, "tool setup") {
			t.Fatalf("blocked execution call occurred: %v", runner.Calls)
		}
	}
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Name != "Create worktree" && step.Status != progress.StepSkipped {
				t.Fatalf("blocked step did not settle skipped: %#v", step)
			}
		}
	}
}
