package creator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestRun_FullPipeline(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[show-ref --verify refs/heads/feature/auth]":                {Err: fmt.Errorf("not found")},
		"/repo:[worktree add /repo/feature-auth -b feature/auth main]":     {Output: ""},
		"/repo/feature-auth:[merge main --no-edit]":                        {Output: ""},
		"/repo/feature-auth:shell[go mod download]":                        {Output: ""},
		"/repo/main:shell[code-review-graph --version]":                    {Output: "1.0"},
		"/repo:shell[code-review-graph build --repo '/repo/feature-auth']": {Output: ""},
	}}

	opts := Options{
		BranchName:     "feature/auth",
		BaseBranch:     "main",
		RepoPath:       "/repo",
		SourceWorktree: "/repo/main",
		MergeBase:      true,
		CopyEnvFiles:   false,
		Ecosystems: []config.EcosystemConfig{
			{
				Name:    "go",
				Install: config.InstallConfig{Command: "go mod download"},
			},
		},
		Integrations: []integration.Integration{
			{
				Name: "code-review-graph",
				Detect: integration.DetectSpec{
					Command: "code-review-graph --version",
				},
				Setup: integration.SetupSpec{
					Command:    "code-review-graph build --repo {path}",
					WorkingDir: "repo",
				},
			},
		},
	}

	ec := &mock.EventCollector[progress.Event]{}
	result := Run(runner, runner, opts, ec.Emit)

	if result.WorktreePath != "/repo/feature-auth" {
		t.Errorf("WorktreePath = %q, want %q", result.WorktreePath, "/repo/feature-auth")
	}
	if len(result.Phases) != 3 {
		t.Fatalf("phase count = %d, want 3", len(result.Phases))
	}
	if result.Phases[0].Name != "Setup" {
		t.Errorf("phase[0] = %q, want Setup", result.Phases[0].Name)
	}
	if result.Phases[1].Name != "Dependencies" {
		t.Errorf("phase[1] = %q, want Dependencies", result.Phases[1].Name)
	}
	if result.Phases[2].Name != "Integrations" {
		t.Errorf("phase[2] = %q, want Integrations", result.Phases[2].Name)
	}
	if result.HasFailures() {
		t.Error("expected no failures in full pipeline")
	}
	if len(ec.Events) == 0 {
		t.Error("expected events to be emitted")
	}
}

func TestRun_CreateWorktreeFails_AbortsEarly(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[show-ref --verify refs/heads/feature/dup]": {Err: fmt.Errorf("not found")},
		"/repo:[worktree add /repo/feature-dup -b feature/dup main]": {
			Err: fmt.Errorf("fatal: something broke"),
		},
	}}

	opts := Options{
		BranchName:     "feature/dup",
		BaseBranch:     "main",
		RepoPath:       "/repo",
		SourceWorktree: "/repo/main",
		MergeBase:      true,
		CopyEnvFiles:   false,
		Ecosystems: []config.EcosystemConfig{
			{Name: "go", Install: config.InstallConfig{Command: "go mod download"}},
		},
	}

	ec := &mock.EventCollector[progress.Event]{}
	result := Run(runner, runner, opts, ec.Emit)

	if len(result.Phases) != 2 {
		t.Fatalf("phase count = %d, want full prepared projection", len(result.Phases))
	}
	if result.WorktreePath != "" {
		t.Errorf("WorktreePath = %q, want empty on failure", result.WorktreePath)
	}
}

func TestRun_MergeFailsContinues(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[show-ref --verify refs/heads/feature/conflict]":                {Err: fmt.Errorf("not found")},
		"/repo:[worktree add /repo/feature-conflict -b feature/conflict main]": {Output: ""},
		"/repo/feature-conflict:[merge main --no-edit]":                        {Err: fmt.Errorf("conflict")},
	}}

	opts := Options{
		BranchName:     "feature/conflict",
		BaseBranch:     "main",
		RepoPath:       "/repo",
		SourceWorktree: "/repo/main",
		MergeBase:      true,
		CopyEnvFiles:   false,
	}

	ec := &mock.EventCollector[progress.Event]{}
	result := Run(runner, runner, opts, ec.Emit)

	if len(result.Phases) != 1 {
		t.Fatalf("phase count = %d, want only the non-empty setup phase", len(result.Phases))
	}
	if result.WorktreePath == "" {
		t.Error("WorktreePath should be set even with merge failure")
	}
	if !result.HasFailures() {
		t.Error("expected HasFailures to return true")
	}
}

func TestRun_CopyEnvFiles(t *testing.T) {
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, ".env"), []byte("KEY=val"), 0644)

	repoDir := t.TempDir()
	wtPath := filepath.Join(repoDir, "feature-env")

	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s:[show-ref --verify refs/heads/feature/env]", repoDir):    {Err: fmt.Errorf("not found")},
		fmt.Sprintf("%s:[worktree add %s -b feature/env main]", repoDir, wtPath): {Output: ""},
	}}

	// Create the worktree dir since the mock doesn't actually create it
	os.MkdirAll(wtPath, 0755)

	opts := Options{
		BranchName:     "feature/env",
		BaseBranch:     "main",
		RepoPath:       repoDir,
		SourceWorktree: srcDir,
		MergeBase:      false,
		CopyEnvFiles:   true,
		Ecosystems: []config.EcosystemConfig{
			{
				Name:     "node",
				EnvFiles: []string{".env"},
				Install:  config.InstallConfig{Command: ""},
			},
		},
	}

	ec := &mock.EventCollector[progress.Event]{}
	result := Run(runner, runner, opts, ec.Emit)

	// Verify env file was copied
	envDst := filepath.Join(wtPath, ".env")
	data, err := os.ReadFile(envDst)
	if err != nil {
		t.Fatalf("failed to read copied env file: %v", err)
	}
	if string(data) != "KEY=val" {
		t.Errorf("env file content = %q, want %q", string(data), "KEY=val")
	}

	_ = result
}

func TestResult_HasFailures(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   bool
	}{
		{
			name:   "contract error",
			result: Result{Err: fmt.Errorf("progress delivery failed")},
			want:   true,
		},
		{
			name: "no failures",
			result: Result{
				Phases: []progress.Phase{
					{Steps: []progress.StepResult{{Status: progress.StepDone}, {Status: progress.StepSkipped}}},
				},
			},
			want: false,
		},
		{
			name: "has failure",
			result: Result{
				Phases: []progress.Phase{
					{Steps: []progress.StepResult{{Status: progress.StepDone}, {Status: progress.StepFailed}}},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasFailures(); got != tt.want {
				t.Errorf("HasFailures() = %v, want %v", got, tt.want)
			}
		})
	}
}
