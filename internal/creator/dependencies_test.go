package creator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

// Dependencies used to be installed once per workspace member, with the root
// install skipped. The default per-member commands selected nothing for pnpm
// and bun and failed for yarn, so a monorepo worktree came up with no
// node_modules. A root install covers every member.
func TestRun_WorkspaceRepoGetsOneRootInstall(t *testing.T) {
	const install = "pnpm install --frozen-lockfile --prefer-offline"
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[show-ref --verify refs/heads/feature/deps]":               {Err: errors.New("not found")},
		"/repo:[worktree add -b feature/deps -- /repo/feature-deps main]": {},
		"/repo/feature-deps:shell[" + install + "]":                       {},
	}}
	var labels []string

	result := Run(runner, runner, Options{
		BranchName: "feature/deps", BaseBranch: "main", RepoPath: "/repo",
		Ecosystems: []config.EcosystemConfig{{Name: "pnpm", Install: config.InstallConfig{Command: install}}},
	}, func(event progress.Event) {
		if event.PhaseLabel == "Dependencies" && event.Status == progress.StepPending && !event.Close {
			labels = append(labels, event.StepLabel)
		}
	})

	if result.Err != nil || result.HasFailures() {
		t.Fatalf("result = %#v, want a successful root install", result)
	}
	if len(labels) != 1 || labels[0] != "pnpm" {
		t.Errorf("dependency steps = %v, want exactly one step named pnpm", labels)
	}
	installs := 0
	for _, call := range runner.Calls {
		switch {
		case call == "/repo/feature-deps:shell["+install+"]":
			installs++
		case strings.Contains(call, "ls-tree"), strings.Contains(call, "--filter"):
			t.Errorf("unexpected per-workspace planning or install call: %s", call)
		}
	}
	if installs != 1 {
		t.Errorf("root install ran %d times, want 1 (calls: %v)", installs, runner.Calls)
	}
}

// Env files used to come only from the ecosystems selected for install, so
// asking to copy them without selecting an install copied nothing.
func TestRun_CopiesExplicitEnvFilesWithoutEcosystems(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, ".env"), []byte("TOKEN=local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	worktree := filepath.Join(repo, "feature-env")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s:[show-ref --verify refs/heads/feature/env]", repo):         {Err: errors.New("not found")},
		fmt.Sprintf("%s:[worktree add -b feature/env -- %s main]", repo, worktree): {},
	}}

	result := Run(runner, runner, Options{
		BranchName: "feature/env", BaseBranch: "main", RepoPath: repo,
		SourceWorktree: source, CopyEnvFiles: true, EnvFiles: []string{".env"},
	}, func(progress.Event) {})

	if result.Err != nil || result.HasFailures() {
		t.Fatalf("result = %#v, want success", result)
	}
	got, err := os.ReadFile(filepath.Join(worktree, ".env"))
	if err != nil {
		t.Fatalf("the env file was not copied: %v", err)
	}
	if string(got) != "TOKEN=local\n" {
		t.Errorf(".env = %q, want the source contents", got)
	}
}

func TestRun_EcosystemWithoutInstallCommandPlansNoDependencyStep(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[show-ref --verify refs/heads/feature/deps]":               {Err: errors.New("not found")},
		"/repo:[worktree add -b feature/deps -- /repo/feature-deps main]": {},
	}}

	result := Run(runner, runner, Options{
		BranchName: "feature/deps", BaseBranch: "main", RepoPath: "/repo",
		Ecosystems: []config.EcosystemConfig{{Name: "docs-only"}},
	}, func(progress.Event) {})

	if result.Err != nil || result.HasFailures() {
		t.Fatalf("result = %#v, want success with nothing to install", result)
	}
	for _, phase := range result.Phases {
		if phase.Name == "Dependencies" {
			t.Errorf("no Dependencies phase expected, got %#v", phase)
		}
	}
}
