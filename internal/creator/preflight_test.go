package creator

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestRun_MalformedBaseBranchManifestFallsBackToRootInstall(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[ls-tree -r --name-only main]":                                  {Output: "package.json\npackages/api/package.json"},
		"/repo:[show main:package.json]":                                       {Output: `{"workspaces":[`},
		"/repo:[show-ref --verify refs/heads/feature/manifest]":                {Err: errors.New("not found")},
		"/repo:[worktree add /repo/feature-manifest -b feature/manifest main]": {},
		"/repo/feature-manifest:shell[npm install]":                            {},
	}}

	result := Run(runner, runner, workspaceOptions(), func(progress.Event) {})

	if result.Err != nil || result.HasFailures() {
		t.Fatalf("result = %#v, want successful root fallback", result)
	}
	if !slices.Contains(runner.Calls, "/repo/feature-manifest:shell[npm install]") {
		t.Fatalf("calls = %v, want root install fallback", runner.Calls)
	}
}

func TestRun_UnreadableBaseBranchManifestReturnsErrorBeforeExecution(t *testing.T) {
	readErr := errors.New("object unavailable")
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[ls-tree -r --name-only main]": {Output: "package.json\npackages/api/package.json"},
		"/repo:[show main:package.json]":      {Err: readErr},
	}}

	result := Run(runner, runner, workspaceOptions(), func(progress.Event) {})

	if !errors.Is(result.Err, readErr) {
		t.Fatalf("Err = %v, want wrapped %v", result.Err, readErr)
	}
	assertNoExecutionCalls(t, runner.Calls)
}

func TestRun_MalformedPnpmBaseBranchManifestFallsBackToRootInstall(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[ls-tree -r --name-only main]":                                  {Output: "pnpm-workspace.yaml\npackages/api/package.json"},
		"/repo:[show main:pnpm-workspace.yaml]":                                {Output: "packages: [unterminated"},
		"/repo:[show-ref --verify refs/heads/feature/manifest]":                {Err: errors.New("not found")},
		"/repo:[worktree add /repo/feature-manifest -b feature/manifest main]": {},
		"/repo/feature-manifest:shell[npm install]":                            {},
	}}
	opts := workspaceOptions()
	opts.Ecosystems[0].Install.WorkspaceDetect = "pnpm-workspace.yaml"

	result := Run(runner, runner, opts, func(progress.Event) {})

	if result.Err != nil || result.HasFailures() {
		t.Fatalf("result = %#v, want successful root fallback", result)
	}
}

func TestRun_UnresolvableWorkspacePatternFallsBackToRootInstall(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[ls-tree -r --name-only main]":                                  {Output: "package.json\npackages/api/package.json"},
		"/repo:[show main:package.json]":                                       {Output: `{"workspaces":["packages/["]}`},
		"/repo:[show-ref --verify refs/heads/feature/manifest]":                {Err: errors.New("not found")},
		"/repo:[worktree add /repo/feature-manifest -b feature/manifest main]": {},
		"/repo/feature-manifest:shell[npm install]":                            {},
	}}

	result := Run(runner, runner, workspaceOptions(), func(progress.Event) {})

	if result.Err != nil || result.HasFailures() {
		t.Fatalf("result = %#v, want successful root fallback", result)
	}
	if !slices.Contains(runner.Calls, "/repo/feature-manifest:shell[npm install]") {
		t.Fatalf("calls = %v, want root install fallback", runner.Calls)
	}
}

func TestWorkspacePatterns_ParsesPnpmManifestStrictly(t *testing.T) {
	got, err := workspacePatterns("pnpm-workspace.yaml", []byte("packages:\n  - packages/*\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"packages/*"}) {
		t.Fatalf("patterns = %v", got)
	}
}

func TestRun_FreezesWorkspaceInstallsFromBaseBranchBeforeStart(t *testing.T) {
	parallel := true
	opts := workspaceOptions()
	opts.Ecosystems[0].Install.Parallel = &parallel
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[ls-tree -r --name-only main]":                                  {Output: "package.json\npackages/api/package.json\npackages/web/package.json"},
		"/repo:[show main:package.json]":                                       {Output: `{"workspaces":["packages/*"]}`},
		"/repo:[show-ref --verify refs/heads/feature/manifest]":                {Err: errors.New("not found")},
		"/repo:[worktree add /repo/feature-manifest -b feature/manifest main]": {},
		"/repo/feature-manifest:shell[npm --prefix packages/api install]":      {},
		"/repo/feature-manifest:shell[npm --prefix packages/web install]":      {},
	}}
	var events []progress.Event
	result := Run(runner, runner, opts, func(event progress.Event) { events = append(events, event) })

	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if slices.Contains(runner.Calls, "/repo/feature-manifest:shell[npm install]") {
		t.Fatalf("root install ran despite frozen workspace members: %v", runner.Calls)
	}
	wantLabels := []string{"node (packages/api)", "node (packages/web)"}
	var dependencyLabels []string
	for _, event := range events {
		if event.PhaseLabel == "Dependencies" && event.Status == progress.StepPending && !event.Close {
			dependencyLabels = append(dependencyLabels, event.StepLabel)
		}
	}
	if !slices.Equal(dependencyLabels, wantLabels) {
		t.Fatalf("dependency declaration labels = %v, want %v", dependencyLabels, wantLabels)
	}
	firstRunning := slices.IndexFunc(events, func(event progress.Event) bool { return event.Status == progress.StepRunning })
	lastClose := slices.IndexFunc(events, func(event progress.Event) bool { return event.Close && event.PhaseLabel == "Dependencies" })
	if firstRunning < 0 || lastClose < 0 || lastClose > firstRunning {
		t.Fatalf("declaration prefix was not closed before execution: %#v", events)
	}
}

func TestRun_UsesRootInstallWhenBaseBranchHasNoWorkspaceManifest(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[ls-tree -r --name-only main]":                                  {Output: "go.mod"},
		"/repo:[show-ref --verify refs/heads/feature/manifest]":                {Err: errors.New("not found")},
		"/repo:[worktree add /repo/feature-manifest -b feature/manifest main]": {},
		"/repo/feature-manifest:shell[npm install]":                            {},
	}}

	result := Run(runner, runner, workspaceOptions(), func(progress.Event) {})

	if result.Err != nil || result.HasFailures() {
		t.Fatalf("result = %#v, want successful root install", result)
	}
}

func workspaceOptions() Options {
	return Options{
		BranchName: "feature/manifest", BaseBranch: "main", RepoPath: "/repo",
		Ecosystems: []config.EcosystemConfig{{
			Name: "node",
			Install: config.InstallConfig{
				Command: "npm install", WorkspaceDetect: "package.json",
				WorkspaceInstall: "npm --prefix {dir} install",
			},
		}},
	}
}

func assertNoExecutionCalls(t *testing.T, calls []string) {
	t.Helper()
	for _, call := range calls {
		if strings.Contains(call, "worktree add") || strings.Contains(call, ":shell[") {
			t.Fatalf("execution call occurred during failed preflight: %v", calls)
		}
	}
}
