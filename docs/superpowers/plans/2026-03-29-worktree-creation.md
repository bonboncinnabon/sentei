# Worktree Creation Flow — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add worktree creation flow with TUI menu, creator pipeline, integration teardown, and phased parallel progress reporting.

**Architecture:** New `internal/creator/` package for creation pipeline (parallel to `internal/worktree/` for deletion). Model restructured into grouped state (`removeState`, `createState`, `menuState`). Menu becomes the TUI entry point. Four new views for the create flow. Enhanced confirm and progress views for teardown-aware removal. Event-driven pipeline with emit callback pattern matching `cleanup.Run()`.

**Tech Stack:** Go, Bubble Tea (Elm architecture), Bubbles (text input, spinner), Lip Gloss (styling), existing `git.CommandRunner` interface, `internal/config` and `internal/ecosystem` and `internal/integration` packages from sub-project 1.

**Spec:** `docs/superpowers/specs/2026-03-29-worktree-creation-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/creator/creator.go` | Create | `Options`, `Result`, `StepResult`, `StepStatus`, `Phase`, `Event` types; `Run()` orchestrator |
| `internal/creator/setup.go` | Create | `SanitizeBranchPath()`, create worktree step, merge base step, copy env files step |
| `internal/creator/setup_test.go` | Create | Unit tests for setup phase steps |
| `internal/creator/deps.go` | Create | Ecosystem dependency installation with parallel workspace support |
| `internal/creator/deps_test.go` | Create | Unit tests for dependency installation |
| `internal/creator/integrations.go` | Create | Integration dependency resolution, install, setup command execution, gitignore append |
| `internal/creator/integrations_test.go` | Create | Unit tests for integration setup |
| `internal/creator/teardown.go` | Create | `ScanArtifacts()`, `Teardown()` — scan worktrees for integration artifacts, run teardown commands, fallback to dir deletion |
| `internal/creator/teardown_test.go` | Create | Unit tests for teardown scanning and execution |
| `internal/creator/creator_test.go` | Create | Full pipeline orchestrator tests |
| `internal/tui/styles.go` | Modify | Add phase/indicator/separator styles |
| `internal/tui/model.go` | Modify | Restructure into grouped state (`removeState`, `createState`), add new `viewState` values, update `NewModel`, `Update`, `View` routing |
| `internal/tui/list.go` | Modify | Access fields via `m.remove.*` |
| `internal/tui/confirm.go` | Modify | Access fields via `m.remove.*`, add integration teardown info section |
| `internal/tui/progress.go` | Modify | Access fields via `m.remove.*`, adopt phased reporting |
| `internal/tui/summary.go` | Modify | Access fields via `m.remove.*` |
| `internal/tui/keys.go` | Modify | Add menu navigation, create flow, and toggle key bindings |
| `internal/tui/menu.go` | Create | Menu view rendering and update logic |
| `internal/tui/create_branch.go` | Create | Branch input view with validation |
| `internal/tui/create_options.go` | Create | Setup/integration toggles view |
| `internal/tui/create_progress.go` | Create | Phased parallel progress view for creation |
| `internal/tui/create_summary.go` | Create | Creation summary view |
| `main.go` | Modify | Lazy loading, start at menu, pass config to TUI |

---

## Task 1: Creator pipeline types and setup phase

**Files:**
- Create: `internal/creator/creator.go`
- Create: `internal/creator/setup.go`
- Create: `internal/creator/setup_test.go`

- [ ] **Step 1: Write tests for path sanitization and setup steps**

Create `internal/creator/setup_test.go`:

```go
package creator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type mockRunner struct {
	responses map[string]mockResponse
	calls     []string
}

type mockResponse struct {
	output string
	err    error
}

func (m *mockRunner) Run(dir string, args ...string) (string, error) {
	key := fmt.Sprintf("%s:%v", dir, args)
	m.calls = append(m.calls, key)
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	return "", fmt.Errorf("unexpected call: %s", key)
}

type eventCollector struct {
	events []Event
}

func (c *eventCollector) emit(e Event) {
	c.events = append(c.events, e)
}

func TestSanitizeBranchPath(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{
			name:   "slash replaced with dash",
			branch: "feature/auth",
			want:   "feature-auth",
		},
		{
			name:   "multiple slashes",
			branch: "bugfix/login/redirect",
			want:   "bugfix-login-redirect",
		},
		{
			name:   "no slash unchanged",
			branch: "hotfix",
			want:   "hotfix",
		},
		{
			name:   "trailing slash stripped",
			branch: "feature/",
			want:   "feature-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeBranchPath(tt.branch)
			if got != tt.want {
				t.Errorf("SanitizeBranchPath(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}

func TestCreateWorktreeStep(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		baseBranch string
		repoPath  string
		runnerErr error
		wantStatus StepStatus
		wantPath  string
	}{
		{
			name:       "successful creation",
			branch:     "feature/auth",
			baseBranch: "main",
			repoPath:   "/repo",
			wantStatus: StepDone,
			wantPath:   "/repo/feature-auth",
		},
		{
			name:       "branch already exists",
			branch:     "feature/dup",
			baseBranch: "main",
			repoPath:   "/repo",
			runnerErr:  fmt.Errorf("fatal: branch already exists"),
			wantStatus: StepFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := SanitizeBranchPath(tt.branch)
			wtPath := filepath.Join(tt.repoPath, sanitized)

			runner := &mockRunner{responses: map[string]mockResponse{
				fmt.Sprintf("%s:[worktree add %s -b %s %s]", tt.repoPath, wtPath, tt.branch, tt.baseBranch): {
					output: "",
					err:    tt.runnerErr,
				},
			}}

			ec := &eventCollector{}
			result, path := createWorktreeStep(runner, tt.repoPath, tt.branch, tt.baseBranch, ec.emit)

			if result.Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", result.Status, tt.wantStatus)
			}
			if tt.wantStatus == StepDone && path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if len(ec.events) == 0 {
				t.Error("expected at least one event emitted")
			}
		})
	}
}

func TestMergeBaseStep(t *testing.T) {
	tests := []struct {
		name       string
		mergeBase  bool
		baseBranch string
		runnerErr  error
		wantStatus StepStatus
	}{
		{
			name:       "successful merge",
			mergeBase:  true,
			baseBranch: "main",
			wantStatus: StepDone,
		},
		{
			name:       "merge conflict continues",
			mergeBase:  true,
			baseBranch: "main",
			runnerErr:  fmt.Errorf("merge conflict"),
			wantStatus: StepFailed,
		},
		{
			name:       "merge disabled",
			mergeBase:  false,
			baseBranch: "main",
			wantStatus: StepSkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{responses: map[string]mockResponse{
				"/repo/feature-auth:[merge main --no-edit]": {
					output: "",
					err:    tt.runnerErr,
				},
			}}

			ec := &eventCollector{}
			result := mergeBaseStep(runner, "/repo/feature-auth", tt.baseBranch, tt.mergeBase, ec.emit)

			if result.Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", result.Status, tt.wantStatus)
			}
		})
	}
}

func TestCopyEnvFilesStep(t *testing.T) {
	tests := []struct {
		name       string
		envFiles   []string
		srcFiles   []string
		wantStatus StepStatus
	}{
		{
			name:       "copies existing files",
			envFiles:   []string{".env", ".env.local"},
			srcFiles:   []string{".env"},
			wantStatus: StepDone,
		},
		{
			name:       "no env files configured",
			envFiles:   nil,
			wantStatus: StepSkipped,
		},
		{
			name:       "no source files exist",
			envFiles:   []string{".env"},
			srcFiles:   nil,
			wantStatus: StepDone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir := t.TempDir()
			dstDir := t.TempDir()

			for _, f := range tt.srcFiles {
				os.WriteFile(filepath.Join(srcDir, f), []byte("SECRET=val"), 0644)
			}

			ec := &eventCollector{}
			result := copyEnvFilesStep(srcDir, dstDir, tt.envFiles, ec.emit)

			if result.Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", result.Status, tt.wantStatus)
			}

			for _, f := range tt.srcFiles {
				dstPath := filepath.Join(dstDir, f)
				if _, err := os.Stat(dstPath); os.IsNotExist(err) {
					t.Errorf("expected %s to be copied to dest", f)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/creator/ -v -run 'TestSanitize|TestCreate|TestMerge|TestCopyEnv'`

Expected: Compilation error — `creator` package doesn't exist yet.

- [ ] **Step 3: Write creator types**

Create `internal/creator/creator.go`:

```go
package creator

import (
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/integration"
)

type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepDone
	StepFailed
	StepSkipped
)

type StepResult struct {
	Name    string
	Status  StepStatus
	Message string
	Error   error
}

type Phase struct {
	Name  string
	Steps []StepResult
}

type Event struct {
	Phase   string
	Step    string
	Status  StepStatus
	Message string
	Error   error
}

type Options struct {
	BranchName     string
	BaseBranch     string
	RepoPath       string
	SourceWorktree string
	MergeBase      bool
	CopyEnvFiles   bool
	Ecosystems     []config.EcosystemConfig
	Integrations   []integration.Integration
}

type Result struct {
	WorktreePath string
	Phases       []Phase
}

func (r *Result) HasFailures() bool {
	for _, p := range r.Phases {
		for _, s := range p.Steps {
			if s.Status == StepFailed {
				return true
			}
		}
	}
	return false
}

func Run(runner git.CommandRunner, opts Options, emit func(Event)) Result {
	result := Result{}

	setupPhase := runSetup(runner, opts, emit)
	result.Phases = append(result.Phases, setupPhase)

	if setupPhase.Steps[0].Status == StepFailed {
		return result
	}
	result.WorktreePath = worktreePath(opts.RepoPath, opts.BranchName)

	depsPhase := runDeps(runner, result.WorktreePath, opts, emit)
	result.Phases = append(result.Phases, depsPhase)

	intPhase := runIntegrations(runner, result.WorktreePath, opts, emit)
	result.Phases = append(result.Phases, intPhase)

	return result
}

func worktreePath(repoPath, branch string) string {
	return repoPath + "/" + SanitizeBranchPath(branch)
}
```

- [ ] **Step 4: Write setup phase implementation**

Create `internal/creator/setup.go`:

```go
package creator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func SanitizeBranchPath(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

func runSetup(runner git.CommandRunner, opts Options, emit func(Event)) Phase {
	phase := Phase{Name: "Setup"}

	wtResult, wtPath := createWorktreeStep(runner, opts.RepoPath, opts.BranchName, opts.BaseBranch, emit)
	phase.Steps = append(phase.Steps, wtResult)

	if wtResult.Status == StepFailed {
		return phase
	}

	mergeResult := mergeBaseStep(runner, wtPath, opts.BaseBranch, opts.MergeBase, emit)
	phase.Steps = append(phase.Steps, mergeResult)

	var envFiles []string
	for _, eco := range opts.Ecosystems {
		envFiles = append(envFiles, eco.EnvFiles...)
	}
	if opts.CopyEnvFiles {
		envResult := copyEnvFilesStep(opts.SourceWorktree, wtPath, envFiles, emit)
		phase.Steps = append(phase.Steps, envResult)
	} else {
		phase.Steps = append(phase.Steps, StepResult{
			Name:   "Copy env files",
			Status: StepSkipped,
		})
	}

	return phase
}

func createWorktreeStep(runner git.CommandRunner, repoPath, branch, baseBranch string, emit func(Event)) (StepResult, string) {
	stepName := "Create worktree"
	emit(Event{Phase: "Setup", Step: stepName, Status: StepRunning})

	sanitized := SanitizeBranchPath(branch)
	wtPath := filepath.Join(repoPath, sanitized)

	_, err := runner.Run(repoPath, "worktree", "add", wtPath, "-b", branch, baseBranch)
	if err != nil {
		emit(Event{Phase: "Setup", Step: stepName, Status: StepFailed, Error: err})
		return StepResult{
			Name:   stepName,
			Status: StepFailed,
			Error:  fmt.Errorf("creating worktree: %w", err),
		}, ""
	}

	emit(Event{Phase: "Setup", Step: stepName, Status: StepDone, Message: wtPath})
	return StepResult{
		Name:    stepName,
		Status:  StepDone,
		Message: wtPath,
	}, wtPath
}

func mergeBaseStep(runner git.CommandRunner, wtPath, baseBranch string, enabled bool, emit func(Event)) StepResult {
	stepName := "Merge base branch"

	if !enabled {
		return StepResult{Name: stepName, Status: StepSkipped}
	}

	emit(Event{Phase: "Setup", Step: stepName, Status: StepRunning})

	_, err := runner.Run(wtPath, "merge", baseBranch, "--no-edit")
	if err != nil {
		emit(Event{Phase: "Setup", Step: stepName, Status: StepFailed, Error: err, Message: "merge conflict — resolve manually"})
		return StepResult{
			Name:    stepName,
			Status:  StepFailed,
			Message: "merge conflict — resolve manually",
			Error:   err,
		}
	}

	emit(Event{Phase: "Setup", Step: stepName, Status: StepDone})
	return StepResult{Name: stepName, Status: StepDone}
}

func copyEnvFilesStep(srcDir, dstDir string, envFiles []string, emit func(Event)) StepResult {
	stepName := "Copy env files"

	if len(envFiles) == 0 {
		return StepResult{Name: stepName, Status: StepSkipped}
	}

	emit(Event{Phase: "Setup", Step: stepName, Status: StepRunning})

	var copied []string
	for _, name := range envFiles {
		src := filepath.Join(srcDir, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		dst := filepath.Join(dstDir, name)
		if err := copyFile(src, dst); err != nil {
			emit(Event{Phase: "Setup", Step: stepName, Status: StepFailed, Error: err})
			return StepResult{
				Name:   stepName,
				Status: StepFailed,
				Error:  fmt.Errorf("copying %s: %w", name, err),
			}
		}
		copied = append(copied, name)
	}

	msg := strings.Join(copied, ", ")
	if msg == "" {
		msg = "no source files found"
	}
	emit(Event{Phase: "Setup", Step: stepName, Status: StepDone, Message: msg})
	return StepResult{
		Name:    stepName,
		Status:  StepDone,
		Message: msg,
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
```

- [ ] **Step 5: Run tests — verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/creator/ -v -run 'TestSanitize|TestCreate|TestMerge|TestCopyEnv'`

Expected: All tests pass.

- [ ] **Step 6: Run full test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./...`

Expected: All existing tests still pass, new tests pass.

**Commit message:** `feat(creator): add pipeline types and setup phase (create worktree, merge, copy env)`

---

## Task 2: Creator dependency installation

**Files:**
- Create: `internal/creator/deps.go`
- Create: `internal/creator/deps_test.go`

- [ ] **Step 1: Write tests for dependency installation**

Create `internal/creator/deps_test.go`:

```go
package creator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestRunDeps_SingleEcosystem(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		"/repo/feature-auth:[pnpm install]": {output: ""},
	}}

	opts := Options{
		Ecosystems: []config.EcosystemConfig{
			{
				Name:    "pnpm",
				Install: config.InstallConfig{Command: "pnpm install"},
			},
		},
	}

	ec := &eventCollector{}
	phase := runDeps(runner, "/repo/feature-auth", opts, ec.emit)

	if phase.Name != "Dependencies" {
		t.Errorf("phase name = %q, want %q", phase.Name, "Dependencies")
	}
	if len(phase.Steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(phase.Steps))
	}
	if phase.Steps[0].Status != StepDone {
		t.Errorf("step status = %v, want StepDone", phase.Steps[0].Status)
	}
}

func TestRunDeps_NoEcosystems(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{}}

	opts := Options{Ecosystems: nil}
	ec := &eventCollector{}
	phase := runDeps(runner, "/repo/feature-auth", opts, ec.emit)

	if len(phase.Steps) != 0 {
		t.Errorf("step count = %d, want 0", len(phase.Steps))
	}
}

func TestRunDeps_InstallFailure(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		"/repo/feature-auth:[pnpm install]": {err: fmt.Errorf("ENOENT")},
	}}

	opts := Options{
		Ecosystems: []config.EcosystemConfig{
			{
				Name:    "pnpm",
				Install: config.InstallConfig{Command: "pnpm install"},
			},
		},
	}

	ec := &eventCollector{}
	phase := runDeps(runner, "/repo/feature-auth", opts, ec.emit)

	if phase.Steps[0].Status != StepFailed {
		t.Errorf("step status = %v, want StepFailed", phase.Steps[0].Status)
	}
}

func TestRunDeps_ParallelWorkspaces(t *testing.T) {
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "feature-auth")
	os.MkdirAll(wtPath, 0755)

	pkgsUI := filepath.Join(wtPath, "packages", "ui")
	pkgsCore := filepath.Join(wtPath, "packages", "core")
	os.MkdirAll(pkgsUI, 0755)
	os.MkdirAll(pkgsCore, 0755)

	wsYaml := "packages:\n  - packages/*\n"
	os.WriteFile(filepath.Join(wtPath, "pnpm-workspace.yaml"), []byte(wsYaml), 0644)

	runner := &mockRunner{responses: map[string]mockResponse{
		wtPath + ":[pnpm install]": {output: ""},
		fmt.Sprintf("%s:[pnpm install --filter packages/ui]", wtPath):   {output: ""},
		fmt.Sprintf("%s:[pnpm install --filter packages/core]", wtPath): {output: ""},
	}}

	opts := Options{
		Ecosystems: []config.EcosystemConfig{
			{
				Name: "pnpm",
				Install: config.InstallConfig{
					Command:          "pnpm install",
					WorkspaceDetect:  "pnpm-workspace.yaml",
					WorkspaceInstall: "pnpm install --filter {dir}",
					Parallel:         boolPtr(true),
				},
			},
		},
	}

	ec := &eventCollector{}
	phase := runDeps(runner, wtPath, opts, ec.emit)

	// Root install + 2 workspace installs = 3 steps
	if len(phase.Steps) != 3 {
		t.Fatalf("step count = %d, want 3", len(phase.Steps))
	}

	// Verify all steps completed
	for i, step := range phase.Steps {
		if step.Status != StepDone {
			t.Errorf("step[%d] %q status = %v, want StepDone", i, step.Name, step.Status)
		}
	}

	// Verify events contain "running" and "done" for each
	runningCount := 0
	for _, e := range ec.events {
		if e.Status == StepRunning {
			runningCount++
		}
	}
	if runningCount < 3 {
		t.Errorf("expected at least 3 running events, got %d", runningCount)
	}
}

func TestRunDeps_CommandParsing(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		"/wt:[go mod download]": {output: ""},
	}}

	opts := Options{
		Ecosystems: []config.EcosystemConfig{
			{
				Name:    "go",
				Install: config.InstallConfig{Command: "go mod download"},
			},
		},
	}

	ec := &eventCollector{}
	runDeps(runner, "/wt", opts, ec.emit)

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.calls))
	}
	// The call key includes parsed args
	if !strings.Contains(runner.calls[0], "go mod download") {
		t.Errorf("call = %q, expected to contain 'go mod download'", runner.calls[0])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/creator/ -v -run TestRunDeps`

Expected: Compilation error — `runDeps` function doesn't exist yet.

- [ ] **Step 3: Write deps implementation**

Create `internal/creator/deps.go`:

```go
package creator

import (
	"fmt"
	"strings"
	"sync"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/ecosystem"
	"github.com/bonboncinnabon/sentei/internal/git"
)

const maxDepsConcurrency = 5

func runDeps(runner git.CommandRunner, wtPath string, opts Options, emit func(Event)) Phase {
	phase := Phase{Name: "Dependencies"}

	if len(opts.Ecosystems) == 0 {
		return phase
	}

	for _, eco := range opts.Ecosystems {
		steps := installEcosystem(runner, wtPath, eco, emit)
		phase.Steps = append(phase.Steps, steps...)
	}

	return phase
}

func installEcosystem(runner git.CommandRunner, wtPath string, eco config.EcosystemConfig, emit func(Event)) []StepResult {
	rootStep := runInstallCommand(runner, wtPath, eco.Name, eco.Install.Command, emit)
	steps := []StepResult{rootStep}

	if rootStep.Status == StepFailed {
		return steps
	}

	if eco.Install.WorkspaceDetect == "" || eco.Install.WorkspaceInstall == "" {
		return steps
	}

	workspaces, err := ecosystem.DetectWorkspaces(wtPath, eco.Install.WorkspaceDetect)
	if err != nil || len(workspaces) == 0 {
		return steps
	}

	if eco.Install.IsParallel() {
		wsSteps := installWorkspacesParallel(runner, wtPath, eco, workspaces, emit)
		steps = append(steps, wsSteps...)
	} else {
		for _, ws := range workspaces {
			cmd := strings.ReplaceAll(eco.Install.WorkspaceInstall, "{dir}", ws)
			step := runInstallCommand(runner, wtPath, fmt.Sprintf("%s (%s)", eco.Name, ws), cmd, emit)
			steps = append(steps, step)
		}
	}

	return steps
}

func installWorkspacesParallel(runner git.CommandRunner, wtPath string, eco config.EcosystemConfig, workspaces []string, emit func(Event)) []StepResult {
	results := make([]StepResult, len(workspaces))
	sem := make(chan struct{}, maxDepsConcurrency)
	var wg sync.WaitGroup

	for i, ws := range workspaces {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, workspace string) {
			defer wg.Done()
			defer func() { <-sem }()

			cmd := strings.ReplaceAll(eco.Install.WorkspaceInstall, "{dir}", workspace)
			stepName := fmt.Sprintf("%s (%s)", eco.Name, workspace)
			results[idx] = runInstallCommand(runner, wtPath, stepName, cmd, emit)
		}(i, ws)
	}

	wg.Wait()
	return results
}

func runInstallCommand(runner git.CommandRunner, wtPath, stepName, command string, emit func(Event)) StepResult {
	emit(Event{Phase: "Dependencies", Step: stepName, Status: StepRunning})

	args := strings.Fields(command)
	if len(args) == 0 {
		emit(Event{Phase: "Dependencies", Step: stepName, Status: StepFailed, Error: fmt.Errorf("empty install command")})
		return StepResult{
			Name:   stepName,
			Status: StepFailed,
			Error:  fmt.Errorf("empty install command for %s", stepName),
		}
	}

	_, err := runner.Run(wtPath, args...)
	if err != nil {
		emit(Event{Phase: "Dependencies", Step: stepName, Status: StepFailed, Error: err})
		return StepResult{
			Name:   stepName,
			Status: StepFailed,
			Error:  fmt.Errorf("installing %s: %w", stepName, err),
		}
	}

	emit(Event{Phase: "Dependencies", Step: stepName, Status: StepDone})
	return StepResult{Name: stepName, Status: StepDone}
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/creator/ -v -run TestRunDeps`

Expected: All tests pass.

- [ ] **Step 5: Run full test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./...`

Expected: All tests pass.

**Commit message:** `feat(creator): add dependency installation with parallel workspace support`

---

## Task 3: Creator integration setup

**Files:**
- Create: `internal/creator/integrations.go`
- Create: `internal/creator/integrations_test.go`

- [ ] **Step 1: Write tests for integration setup**

Create `internal/creator/integrations_test.go`:

```go
package creator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/integration"
)

func TestRunIntegrations_NoIntegrations(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{}}
	opts := Options{Integrations: nil}
	ec := &eventCollector{}

	phase := runIntegrations(runner, "/wt", opts, ec.emit)

	if len(phase.Steps) != 0 {
		t.Errorf("step count = %d, want 0", len(phase.Steps))
	}
}

func TestRunIntegrations_AlreadyInstalled(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		"/wt:[code-review-graph --version]": {output: "1.0.0"},
		"/repo:[code-review-graph build --repo /wt]": {output: "built"},
	}}

	opts := Options{
		RepoPath: "/repo",
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
				GitignoreEntries: []string{".code-review-graph/"},
			},
		},
	}

	ec := &eventCollector{}
	wtPath := t.TempDir()

	phase := runIntegrations(runner, wtPath, opts, ec.emit)

	// Should have steps: detect + setup
	hasSetup := false
	for _, s := range phase.Steps {
		if strings.Contains(s.Name, "setup") || strings.Contains(s.Name, "Setup") {
			hasSetup = true
		}
	}
	if !hasSetup {
		t.Error("expected setup step to be present")
	}
}

func TestRunIntegrations_InstallRequired(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		// Detect fails first time
		"/wt:[code-review-graph --version]": {err: fmt.Errorf("not found")},
		// Dependency checks
		`/wt:[python3 -c "import sys; assert sys.version_info >= (3,10)"]`: {output: ""},
		"/wt:[pipx --version]": {output: "1.0"},
		// Install
		"/wt:[pipx install code-review-graph]": {output: "installed"},
		// Setup (working dir = repo, so runs from opts.RepoPath)
		"/repo:[code-review-graph build --repo /wt]": {output: "built"},
	}}

	opts := Options{
		RepoPath: "/repo",
		Integrations: []integration.Integration{
			{
				Name: "code-review-graph",
				Dependencies: []integration.Dependency{
					{
						Name:   "python3.10+",
						Detect: `python3 -c "import sys; assert sys.version_info >= (3,10)"`,
					},
					{
						Name:    "pipx",
						Detect:  "pipx --version",
						Install: "brew install pipx",
					},
				},
				Detect: integration.DetectSpec{
					Command: "code-review-graph --version",
				},
				Install: integration.InstallSpec{
					Command: "pipx install code-review-graph",
				},
				Setup: integration.SetupSpec{
					Command:    "code-review-graph build --repo {path}",
					WorkingDir: "repo",
				},
				GitignoreEntries: []string{".code-review-graph/"},
			},
		},
	}

	ec := &eventCollector{}
	wtPath := t.TempDir()

	phase := runIntegrations(runner, wtPath, opts, ec.emit)

	hasFailed := false
	for _, s := range phase.Steps {
		if s.Status == StepFailed {
			hasFailed = true
		}
	}
	if hasFailed {
		t.Error("expected no failures when install + setup succeed")
	}
}

func TestRunIntegrations_SetupFailure(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		"/wt:[ccc --version]":    {output: "1.0"},
		"/wt:[ccc init]":         {err: fmt.Errorf("init failed")},
	}}

	opts := Options{
		RepoPath: "/repo",
		Integrations: []integration.Integration{
			{
				Name: "cocoindex-code",
				Detect: integration.DetectSpec{
					Command: "ccc --version",
				},
				Setup: integration.SetupSpec{
					Command:    "ccc init",
					WorkingDir: "worktree",
				},
			},
		},
	}

	ec := &eventCollector{}
	phase := runIntegrations(runner, "/wt", opts, ec.emit)

	hasFailed := false
	for _, s := range phase.Steps {
		if s.Status == StepFailed {
			hasFailed = true
		}
	}
	if !hasFailed {
		t.Error("expected a failure when setup command fails")
	}
}

func TestAppendGitignore(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		entries  []string
		want     string
	}{
		{
			name:     "adds new entries",
			existing: "node_modules/\n",
			entries:  []string{".code-review-graph/"},
			want:     "node_modules/\n.code-review-graph/\n",
		},
		{
			name:     "skips existing entries",
			existing: ".code-review-graph/\n",
			entries:  []string{".code-review-graph/"},
			want:     ".code-review-graph/\n",
		},
		{
			name:     "creates file if absent",
			existing: "",
			entries:  []string{".code-review-graph/", ".cocoindex_code/"},
			want:     ".code-review-graph/\n.cocoindex_code/\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			gitignorePath := filepath.Join(dir, ".gitignore")

			if tt.existing != "" {
				os.WriteFile(gitignorePath, []byte(tt.existing), 0644)
			}

			appendGitignore(dir, tt.entries)

			got, _ := os.ReadFile(gitignorePath)
			if string(got) != tt.want {
				t.Errorf("gitignore content:\ngot:  %q\nwant: %q", string(got), tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/creator/ -v -run 'TestRunIntegrations|TestAppendGitignore'`

Expected: Compilation error — functions don't exist yet.

- [ ] **Step 3: Write integrations implementation**

Create `internal/creator/integrations.go`:

```go
package creator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/integration"
)

func runIntegrations(runner git.CommandRunner, wtPath string, opts Options, emit func(Event)) Phase {
	phase := Phase{Name: "Integrations"}

	if len(opts.Integrations) == 0 {
		return phase
	}

	for _, integ := range opts.Integrations {
		steps := setupIntegration(runner, wtPath, opts.RepoPath, integ, emit)
		phase.Steps = append(phase.Steps, steps...)
	}

	return phase
}

func setupIntegration(runner git.CommandRunner, wtPath, repoPath string, integ integration.Integration, emit func(Event)) []StepResult {
	var steps []StepResult

	installed := detectIntegration(runner, wtPath, integ)

	if !installed {
		depSteps := checkAndInstallDeps(runner, wtPath, integ, emit)
		steps = append(steps, depSteps...)

		for _, s := range depSteps {
			if s.Status == StepFailed {
				return steps
			}
		}

		installStep := installIntegration(runner, wtPath, integ, emit)
		steps = append(steps, installStep)
		if installStep.Status == StepFailed {
			return steps
		}
	}

	setupStep := runSetupCommand(runner, wtPath, repoPath, integ, emit)
	steps = append(steps, setupStep)

	if setupStep.Status != StepFailed && len(integ.GitignoreEntries) > 0 {
		appendGitignore(wtPath, integ.GitignoreEntries)
	}

	return steps
}

func detectIntegration(runner git.CommandRunner, wtPath string, integ integration.Integration) bool {
	if integ.Detect.Command != "" {
		args := strings.Fields(integ.Detect.Command)
		_, err := runner.Run(wtPath, args...)
		return err == nil
	}
	if integ.Detect.BinaryName != "" {
		_, err := runner.Run(wtPath, integ.Detect.BinaryName, "--version")
		return err == nil
	}
	return false
}

func checkAndInstallDeps(runner git.CommandRunner, wtPath string, integ integration.Integration, emit func(Event)) []StepResult {
	var steps []StepResult

	for _, dep := range integ.Dependencies {
		stepName := fmt.Sprintf("Check %s", dep.Name)
		emit(Event{Phase: "Integrations", Step: stepName, Status: StepRunning})

		args := strings.Fields(dep.Detect)
		_, err := runner.Run(wtPath, args...)
		if err == nil {
			emit(Event{Phase: "Integrations", Step: stepName, Status: StepDone})
			steps = append(steps, StepResult{Name: stepName, Status: StepDone})
			continue
		}

		if dep.Install == "" {
			emit(Event{Phase: "Integrations", Step: stepName, Status: StepFailed, Error: fmt.Errorf("%s not found and no install command available", dep.Name)})
			steps = append(steps, StepResult{
				Name:   stepName,
				Status: StepFailed,
				Error:  fmt.Errorf("%s not found and no install command available", dep.Name),
			})
			return steps
		}

		installName := fmt.Sprintf("Install %s", dep.Name)
		emit(Event{Phase: "Integrations", Step: installName, Status: StepRunning})
		installArgs := strings.Fields(dep.Install)
		_, installErr := runner.Run(wtPath, installArgs...)
		if installErr != nil {
			emit(Event{Phase: "Integrations", Step: installName, Status: StepFailed, Error: installErr})
			steps = append(steps, StepResult{
				Name:   installName,
				Status: StepFailed,
				Error:  fmt.Errorf("installing dependency %s: %w", dep.Name, installErr),
			})
			return steps
		}

		emit(Event{Phase: "Integrations", Step: installName, Status: StepDone})
		steps = append(steps, StepResult{Name: installName, Status: StepDone})
	}

	return steps
}

func installIntegration(runner git.CommandRunner, wtPath string, integ integration.Integration, emit func(Event)) StepResult {
	stepName := fmt.Sprintf("Install %s", integ.Name)
	emit(Event{Phase: "Integrations", Step: stepName, Status: StepRunning})

	args := strings.Fields(integ.Install.Command)
	_, err := runner.Run(wtPath, args...)
	if err != nil {
		emit(Event{Phase: "Integrations", Step: stepName, Status: StepFailed, Error: err})
		return StepResult{
			Name:   stepName,
			Status: StepFailed,
			Error:  fmt.Errorf("installing %s: %w", integ.Name, err),
		}
	}

	emit(Event{Phase: "Integrations", Step: stepName, Status: StepDone})
	return StepResult{Name: stepName, Status: StepDone}
}

func runSetupCommand(runner git.CommandRunner, wtPath, repoPath string, integ integration.Integration, emit func(Event)) StepResult {
	stepName := fmt.Sprintf("Setup %s", integ.Name)

	if integ.Setup.Command == "" {
		return StepResult{Name: stepName, Status: StepSkipped}
	}

	emit(Event{Phase: "Integrations", Step: stepName, Status: StepRunning})

	command := strings.ReplaceAll(integ.Setup.Command, "{path}", wtPath)
	args := strings.Fields(command)

	var runDir string
	switch integ.Setup.WorkingDir {
	case "repo":
		runDir = repoPath
	default:
		runDir = wtPath
	}

	_, err := runner.Run(runDir, args...)
	if err != nil {
		emit(Event{Phase: "Integrations", Step: stepName, Status: StepFailed, Error: err})
		return StepResult{
			Name:   stepName,
			Status: StepFailed,
			Error:  fmt.Errorf("setting up %s: %w", integ.Name, err),
		}
	}

	emit(Event{Phase: "Integrations", Step: stepName, Status: StepDone})
	return StepResult{Name: stepName, Status: StepDone}
}

func appendGitignore(dir string, entries []string) {
	gitignorePath := filepath.Join(dir, ".gitignore")

	existing, _ := os.ReadFile(gitignorePath)
	content := string(existing)

	var toAdd []string
	for _, entry := range entries {
		if !strings.Contains(content, entry) {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	for _, entry := range toAdd {
		fmt.Fprintln(f, entry)
	}
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/creator/ -v -run 'TestRunIntegrations|TestAppendGitignore'`

Expected: All tests pass.

- [ ] **Step 5: Run full test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./...`

Expected: All tests pass.

**Commit message:** `feat(creator): add integration dependency resolution, install, and setup`

---

## Task 4: Integration teardown

**Files:**
- Create: `internal/creator/teardown.go`
- Create: `internal/creator/teardown_test.go`

- [ ] **Step 1: Write tests for teardown scanning and execution**

Create `internal/creator/teardown_test.go`:

```go
package creator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/integration"
)

func TestScanArtifacts(t *testing.T) {
	tests := []struct {
		name       string
		dirs       []string
		integs     []integration.Integration
		wantCount  int
	}{
		{
			name: "finds code-review-graph artifacts",
			dirs: []string{".code-review-graph"},
			integs: []integration.Integration{
				{
					Name: "code-review-graph",
					Teardown: integration.TeardownSpec{
						Dirs: []string{".code-review-graph/"},
					},
				},
			},
			wantCount: 1,
		},
		{
			name:   "no artifacts present",
			dirs:   nil,
			integs: []integration.Integration{
				{
					Name: "code-review-graph",
					Teardown: integration.TeardownSpec{
						Dirs: []string{".code-review-graph/"},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "multiple integration artifacts",
			dirs: []string{".code-review-graph", ".cocoindex_code"},
			integs: []integration.Integration{
				{
					Name:     "code-review-graph",
					Teardown: integration.TeardownSpec{Dirs: []string{".code-review-graph/"}},
				},
				{
					Name:     "cocoindex-code",
					Teardown: integration.TeardownSpec{Dirs: []string{".cocoindex_code/"}},
				},
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wtDir := t.TempDir()
			for _, d := range tt.dirs {
				os.MkdirAll(filepath.Join(wtDir, d), 0755)
			}

			artifacts := ScanArtifacts(wtDir, tt.integs)
			if len(artifacts) != tt.wantCount {
				t.Errorf("artifact count = %d, want %d", len(artifacts), tt.wantCount)
			}
		})
	}
}

func TestTeardown_WithCommand(t *testing.T) {
	wtDir := t.TempDir()
	os.MkdirAll(filepath.Join(wtDir, ".cocoindex_code"), 0755)

	runner := &mockRunner{responses: map[string]mockResponse{
		wtDir + ":[ccc reset --all --force]": {output: "reset"},
	}}

	integs := []integration.Integration{
		{
			Name: "cocoindex-code",
			Teardown: integration.TeardownSpec{
				Command: "ccc reset --all --force",
				Dirs:    []string{".cocoindex_code/"},
			},
		},
	}

	ec := &eventCollector{}
	results := Teardown(runner, wtDir, integs, ec.emit)

	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].Status != StepDone {
		t.Errorf("status = %v, want StepDone", results[0].Status)
	}
}

func TestTeardown_CommandFailsFallsBackToDirDelete(t *testing.T) {
	wtDir := t.TempDir()
	artifactDir := filepath.Join(wtDir, ".cocoindex_code")
	os.MkdirAll(artifactDir, 0755)
	os.WriteFile(filepath.Join(artifactDir, "index.db"), []byte("data"), 0644)

	runner := &mockRunner{responses: map[string]mockResponse{
		wtDir + ":[ccc reset --all --force]": {err: fmt.Errorf("command not found")},
	}}

	integs := []integration.Integration{
		{
			Name: "cocoindex-code",
			Teardown: integration.TeardownSpec{
				Command: "ccc reset --all --force",
				Dirs:    []string{".cocoindex_code/"},
			},
		},
	}

	ec := &eventCollector{}
	results := Teardown(runner, wtDir, integs, ec.emit)

	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].Status != StepDone {
		t.Errorf("status = %v, want StepDone (fallback should succeed)", results[0].Status)
	}

	if _, err := os.Stat(artifactDir); !os.IsNotExist(err) {
		t.Error("expected artifact directory to be deleted")
	}
}

func TestTeardown_NoArtifacts(t *testing.T) {
	wtDir := t.TempDir()
	runner := &mockRunner{responses: map[string]mockResponse{}}

	integs := []integration.Integration{
		{
			Name:     "code-review-graph",
			Teardown: integration.TeardownSpec{Dirs: []string{".code-review-graph/"}},
		},
	}

	ec := &eventCollector{}
	results := Teardown(runner, wtDir, integs, ec.emit)

	if len(results) != 0 {
		t.Errorf("result count = %d, want 0 (no artifacts)", len(results))
	}
	if len(ec.events) != 0 {
		t.Errorf("event count = %d, want 0", len(ec.events))
	}
}

func TestTeardown_DirOnlyNoCommand(t *testing.T) {
	wtDir := t.TempDir()
	artifactDir := filepath.Join(wtDir, ".code-review-graph")
	os.MkdirAll(artifactDir, 0755)

	runner := &mockRunner{responses: map[string]mockResponse{}}

	integs := []integration.Integration{
		{
			Name:     "code-review-graph",
			Teardown: integration.TeardownSpec{Dirs: []string{".code-review-graph/"}},
		},
	}

	ec := &eventCollector{}
	results := Teardown(runner, wtDir, integs, ec.emit)

	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].Status != StepDone {
		t.Errorf("status = %v, want StepDone", results[0].Status)
	}

	if _, err := os.Stat(artifactDir); !os.IsNotExist(err) {
		t.Error("expected artifact directory to be deleted")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/creator/ -v -run 'TestScan|TestTeardown'`

Expected: Compilation error — `ScanArtifacts`, `Teardown` don't exist yet.

- [ ] **Step 3: Write teardown implementation**

Create `internal/creator/teardown.go`:

```go
package creator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/integration"
)

type ArtifactInfo struct {
	IntegrationName string
	Dirs            []string
}

func ScanArtifacts(wtPath string, integrations []integration.Integration) []ArtifactInfo {
	var found []ArtifactInfo

	for _, integ := range integrations {
		var presentDirs []string
		for _, dir := range integ.Teardown.Dirs {
			cleanDir := strings.TrimSuffix(dir, "/")
			fullPath := filepath.Join(wtPath, cleanDir)
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				presentDirs = append(presentDirs, dir)
			}
		}
		if len(presentDirs) > 0 {
			found = append(found, ArtifactInfo{
				IntegrationName: integ.Name,
				Dirs:            presentDirs,
			})
		}
	}

	return found
}

func Teardown(runner git.CommandRunner, wtPath string, integrations []integration.Integration, emit func(Event)) []StepResult {
	artifacts := ScanArtifacts(wtPath, integrations)
	if len(artifacts) == 0 {
		return nil
	}

	var results []StepResult

	for _, artifact := range artifacts {
		integ := findIntegration(integrations, artifact.IntegrationName)
		if integ == nil {
			continue
		}

		stepName := fmt.Sprintf("Teardown %s", integ.Name)
		emit(Event{Phase: "Teardown", Step: stepName, Status: StepRunning})

		if integ.Teardown.Command != "" {
			args := strings.Fields(integ.Teardown.Command)
			_, err := runner.Run(wtPath, args...)
			if err == nil {
				emit(Event{Phase: "Teardown", Step: stepName, Status: StepDone})
				results = append(results, StepResult{Name: stepName, Status: StepDone})
				continue
			}
			// Command failed — fall back to directory deletion
		}

		allRemoved := true
		for _, dir := range artifact.Dirs {
			cleanDir := strings.TrimSuffix(dir, "/")
			fullPath := filepath.Join(wtPath, cleanDir)
			if err := os.RemoveAll(fullPath); err != nil {
				allRemoved = false
			}
		}

		if allRemoved {
			emit(Event{Phase: "Teardown", Step: stepName, Status: StepDone, Message: "removed artifact dirs"})
			results = append(results, StepResult{Name: stepName, Status: StepDone, Message: "removed artifact dirs"})
		} else {
			emit(Event{Phase: "Teardown", Step: stepName, Status: StepFailed, Error: fmt.Errorf("failed to remove some artifact dirs")})
			results = append(results, StepResult{
				Name:   stepName,
				Status: StepFailed,
				Error:  fmt.Errorf("failed to remove some artifact dirs for %s", integ.Name),
			})
		}
	}

	return results
}

func findIntegration(integrations []integration.Integration, name string) *integration.Integration {
	for i := range integrations {
		if integrations[i].Name == name {
			return &integrations[i]
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/creator/ -v -run 'TestScan|TestTeardown'`

Expected: All tests pass.

- [ ] **Step 5: Run full test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./...`

Expected: All tests pass.

**Commit message:** `feat(creator): add integration teardown with scan and fallback dir deletion`

---

## Task 5: Creator orchestrator

**Files:**
- Create: `internal/creator/creator_test.go`
- Modify: `internal/creator/creator.go` (wire `Run()`)

- [ ] **Step 1: Write orchestrator tests**

Create `internal/creator/creator_test.go`:

```go
package creator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/integration"
)

func TestRun_FullPipeline(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		"/repo:[worktree add /repo/feature-auth -b feature/auth main]": {output: ""},
		"/repo/feature-auth:[merge main --no-edit]":                    {output: ""},
		"/repo/feature-auth:[go mod download]":                         {output: ""},
		"/repo/feature-auth:[code-review-graph --version]":             {output: "1.0"},
		"/repo:[code-review-graph build --repo /repo/feature-auth]":    {output: ""},
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
				GitignoreEntries: []string{".code-review-graph/"},
			},
		},
	}

	ec := &eventCollector{}
	result := Run(runner, opts, ec.emit)

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
	if len(ec.events) == 0 {
		t.Error("expected events to be emitted")
	}
}

func TestRun_CreateWorktreeFails_AbortsEarly(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		"/repo:[worktree add /repo/feature-dup -b feature/dup main]": {
			err: fmt.Errorf("fatal: branch already exists"),
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

	ec := &eventCollector{}
	result := Run(runner, opts, ec.emit)

	if len(result.Phases) != 1 {
		t.Fatalf("phase count = %d, want 1 (abort after setup)", len(result.Phases))
	}
	if result.WorktreePath != "" {
		t.Errorf("WorktreePath = %q, want empty on failure", result.WorktreePath)
	}
}

func TestRun_MergeFailsContinues(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		"/repo:[worktree add /repo/feature-conflict -b feature/conflict main]": {output: ""},
		"/repo/feature-conflict:[merge main --no-edit]": {err: fmt.Errorf("conflict")},
	}}

	opts := Options{
		BranchName:     "feature/conflict",
		BaseBranch:     "main",
		RepoPath:       "/repo",
		SourceWorktree: "/repo/main",
		MergeBase:      true,
		CopyEnvFiles:   false,
	}

	ec := &eventCollector{}
	result := Run(runner, opts, ec.emit)

	if len(result.Phases) != 3 {
		t.Fatalf("phase count = %d, want 3 (continues despite merge failure)", len(result.Phases))
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

	runner := &mockRunner{responses: map[string]mockResponse{
		fmt.Sprintf("%s:[worktree add %s -b feature/env main]", repoDir, wtPath): {output: ""},
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

	ec := &eventCollector{}
	result := Run(runner, opts, ec.emit)

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
			name: "no failures",
			result: Result{
				Phases: []Phase{
					{Steps: []StepResult{{Status: StepDone}, {Status: StepSkipped}}},
				},
			},
			want: false,
		},
		{
			name: "has failure",
			result: Result{
				Phases: []Phase{
					{Steps: []StepResult{{Status: StepDone}, {Status: StepFailed}}},
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
```

- [ ] **Step 2: Run tests — verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/creator/ -v`

Expected: All creator tests pass. The `Run()` function already exists from Step 3 of Task 1.

- [ ] **Step 3: Run full test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./...`

Expected: All tests pass.

**Commit message:** `feat(creator): wire Run() orchestrator with full pipeline tests`

---

## Task 6: Visual language — styles and indicators

**Files:**
- Modify: `internal/tui/styles.go`

- [ ] **Step 1: Add phase, indicator, and separator styles**

Add to the end of `internal/tui/styles.go`, after the existing `colWidth*` declarations:

```go
// Phase header styles
var (
	stylePhaseDone = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Bold(true)

	stylePhaseActive = lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true)

	stylePhasePending = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
)

// Progress indicators
var (
	styleIndicatorDone = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	styleIndicatorActive = lipgloss.NewStyle().
		Foreground(lipgloss.Color("62"))

	styleIndicatorPending = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	styleIndicatorFailed = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	styleIndicatorWarning = lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))
)

// Layout elements
var (
	styleSeparator = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	styleTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15"))

	styleAccent = lipgloss.NewStyle().
		Foreground(lipgloss.Color("62"))

	styleCheckboxOn = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	styleCheckboxOff = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	styleHint = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
)

// Indicator characters
const (
	indicatorDone    = "●"
	indicatorActive  = "◐"
	indicatorPending = "·"
	indicatorFailed  = "✗"
	indicatorWarning = "⚠"
)

// Separator renders a dotted separator line at the given width.
func separator(width int) string {
	if width <= 4 {
		width = 40
	}
	line := strings.Repeat("┄", width-4)
	return styleSeparator.Render("  " + line)
}
```

Also add the `"strings"` import to `styles.go` since `separator()` uses `strings.Repeat`.

- [ ] **Step 2: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./...`

Expected: Build succeeds.

- [ ] **Step 3: Run full test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./...`

Expected: All existing tests still pass.

**Commit message:** `feat(tui): add phase, indicator, and separator styles for creation flow`

---

## Task 7: Model restructure

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/list.go`
- Modify: `internal/tui/confirm.go`
- Modify: `internal/tui/progress.go`
- Modify: `internal/tui/summary.go`

This is the highest-risk task. Every existing view file accesses flat `Model` fields that move into `removeState`. All changes must be mechanical — rename field accesses from `m.fieldName` to `m.remove.fieldName`.

- [ ] **Step 1: Restructure model.go**

Replace the contents of `internal/tui/model.go` with:

```go
package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

type viewState int

const (
	menuView viewState = iota
	listView
	confirmView
	progressView
	summaryView
	createBranchView
	createOptionsView
	createProgressView
	createSummaryView
)

type SortField int

const (
	SortByAge SortField = iota
	SortByBranch
)

// removeState holds all state for the worktree removal flow.
type removeState struct {
	worktrees      []git.Worktree
	selected       map[string]bool
	visibleIndices []int
	cursor         int
	offset         int

	sortField     SortField
	sortAscending bool

	filterText   string
	filterActive bool
	filterInput  textinput.Model

	deletionStatuses map[string]string
	deletionResult   worktree.DeletionResult
	deletionTotal    int
	progressCh       <-chan worktree.DeletionEvent

	pruneErr      *error
	cleanupResult *cleanup.Result

	teardownResults []creator.StepResult
}

// menuItem represents a selectable menu entry.
type menuItem struct {
	label   string
	hint    string
	enabled bool
}

// createState holds all state for the worktree creation flow.
type createState struct {
	branchInput    textinput.Model
	baseInput      textinput.Model
	focusedField   int // 0 = branch, 1 = base

	ecosystems   []config.EcosystemConfig
	integrations []integration.Integration
	ecoEnabled   map[string]bool
	intEnabled   map[string]bool
	mergeBase    bool
	copyEnvFiles bool
	optionsCursor int

	result    *creator.Result
	events    []creator.Event
	eventCh   <-chan creator.Event
}

type Model struct {
	view   viewState
	runner git.CommandRunner
	repoPath string
	cfg      *config.Config
	width    int
	height   int

	menuCursor int
	menuItems  []menuItem

	remove removeState
	create createState
}

func NewModel(worktrees []git.Worktree, runner git.CommandRunner, repoPath string) Model {
	ti := textinput.New()
	ti.Prompt = "filter: "

	m := Model{
		view:     listView,
		runner:   runner,
		repoPath: repoPath,
		remove: removeState{
			worktrees:        worktrees,
			selected:         make(map[string]bool),
			sortField:        SortByAge,
			sortAscending:    true,
			filterInput:      ti,
			deletionStatuses: make(map[string]string),
		},
		height: 20,
	}
	m.reindex()
	return m
}

func NewMenuModel(runner git.CommandRunner, repoPath string, cfg *config.Config) Model {
	branchInput := textinput.New()
	branchInput.Placeholder = "feature/my-branch"
	branchInput.Focus()

	baseInput := textinput.New()
	baseInput.Placeholder = "main"
	baseInput.SetValue("main")

	m := Model{
		view:     menuView,
		runner:   runner,
		repoPath: repoPath,
		cfg:      cfg,
		height:   20,
		menuItems: []menuItem{
			{label: "Create new worktree", enabled: true},
			{label: "Remove worktrees", enabled: true},
			{label: "Cleanup", hint: "safe mode", enabled: true},
		},
		remove: removeState{
			selected:         make(map[string]bool),
			sortField:        SortByAge,
			sortAscending:    true,
			filterInput:      textinput.New(),
			deletionStatuses: make(map[string]string),
		},
		create: createState{
			branchInput:  branchInput,
			baseInput:    baseInput,
			ecoEnabled:   make(map[string]bool),
			intEnabled:   make(map[string]bool),
			mergeBase:    true,
			copyEnvFiles: true,
		},
	}
	m.remove.filterInput.Prompt = "filter: "

	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.view {
	case menuView:
		return m.updateMenu(msg)
	case listView:
		return m.updateList(msg)
	case confirmView:
		return m.updateConfirm(msg)
	case progressView:
		return m.updateProgress(msg)
	case summaryView:
		return m.updateSummary(msg)
	case createBranchView:
		return m.updateCreateBranch(msg)
	case createOptionsView:
		return m.updateCreateOptions(msg)
	case createProgressView:
		return m.updateCreateProgress(msg)
	case createSummaryView:
		return m.updateCreateSummary(msg)
	}
	return m, nil
}

func (m Model) View() string {
	switch m.view {
	case menuView:
		return m.viewMenu()
	case listView:
		return m.viewList()
	case confirmView:
		return m.viewConfirm()
	case progressView:
		return m.viewProgress()
	case summaryView:
		return m.viewSummary()
	case createBranchView:
		return m.viewCreateBranch()
	case createOptionsView:
		return m.viewCreateOptions()
	case createProgressView:
		return m.viewCreateProgress()
	case createSummaryView:
		return m.viewCreateSummary()
	}
	return ""
}

func (m Model) selectedWorktrees() []git.Worktree {
	var result []git.Worktree
	for _, wt := range m.remove.worktrees {
		if m.remove.selected[wt.Path] {
			result = append(result, wt)
		}
	}
	return result
}

func (m *Model) reindex() {
	filterLower := strings.ToLower(m.remove.filterText)

	var indices []int
	for i, wt := range m.remove.worktrees {
		if filterLower != "" {
			branch := strings.ToLower(stripBranchPrefix(wt.Branch))
			if !strings.Contains(branch, filterLower) {
				continue
			}
		}
		indices = append(indices, i)
	}

	sortAsc := m.remove.sortAscending
	sortField := m.remove.sortField
	wts := m.remove.worktrees

	sort.SliceStable(indices, func(a, b int) bool {
		wa, wb := wts[indices[a]], wts[indices[b]]

		switch sortField {
		case SortByAge:
			aZero := wa.LastCommitDate.IsZero()
			bZero := wb.LastCommitDate.IsZero()
			if aZero != bZero {
				return !aZero
			}
			if aZero && bZero {
				return false
			}
			if sortAsc {
				return wa.LastCommitDate.Before(wb.LastCommitDate)
			}
			return wa.LastCommitDate.After(wb.LastCommitDate)

		case SortByBranch:
			ba := strings.ToLower(stripBranchPrefix(wa.Branch))
			bb := strings.ToLower(stripBranchPrefix(wb.Branch))
			if sortAsc {
				return ba < bb
			}
			return ba > bb

		default:
			return false
		}
	})

	m.remove.visibleIndices = indices

	if m.remove.cursor >= len(m.remove.visibleIndices) {
		m.remove.cursor = max(len(m.remove.visibleIndices)-1, 0)
	}
	if m.remove.offset > m.remove.cursor {
		m.remove.offset = m.remove.cursor
	}
	if m.remove.cursor >= m.remove.offset+m.height && m.height > 0 {
		m.remove.offset = m.remove.cursor - m.height + 1
	}
}
```

- [ ] **Step 2: Update list.go to use m.remove**

In `internal/tui/list.go`, perform these mechanical replacements:
- `m.worktrees` → `m.remove.worktrees`
- `m.selected` → `m.remove.selected`
- `m.visibleIndices` → `m.remove.visibleIndices`
- `m.cursor` → `m.remove.cursor` (only in list context — be careful with this)
- `m.offset` → `m.remove.offset`
- `m.sortField` → `m.remove.sortField`
- `m.sortAscending` → `m.remove.sortAscending`
- `m.filterText` → `m.remove.filterText`
- `m.filterActive` → `m.remove.filterActive`
- `m.filterInput` → `m.remove.filterInput`

The `updateList` method references:
- `m.width` / `m.height` → remain on Model (shared)
- `m.view` → remains on Model (shared)

Full replacement for `internal/tui/list.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/bonboncinnabon/sentei/internal/git"
)

const (
	colCursor   = 0
	colCheckbox = 1
	colStatus   = 2
	colBranch   = 3
	colAge      = 4
	colSubject  = 5
)

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func stripBranchPrefix(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

func statusIndicator(wt git.Worktree) string {
	switch {
	case wt.IsLocked:
		return styleStatusLocked.Render("[L]")
	case wt.HasUncommittedChanges:
		return styleStatusDirty.Render("[~]")
	case wt.HasUntrackedFiles:
		return styleStatusUntracked.Render("[!]")
	default:
		return styleStatusClean.Render("[ok]")
	}
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(msg.Height-6, 5)
		return m, nil

	case tea.KeyMsg:
		if m.remove.filterActive {
			return m.updateFilterInput(msg)
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Back):
			if m.remove.filterText != "" {
				m.remove.filterText = ""
				m.reindex()
				return m, nil
			}
			if m.view == listView && m.menuItems != nil {
				m.view = menuView
				return m, nil
			}
			return m, tea.Quit

		case key.Matches(msg, keys.Filter):
			m.remove.filterActive = true
			m.remove.filterInput.SetValue(m.remove.filterText)
			m.remove.filterInput.Focus()
			return m, m.remove.filterInput.Cursor.BlinkCmd()

		case key.Matches(msg, keys.Sort):
			m.remove.sortField = (m.remove.sortField + 1) % 2
			m.remove.cursor = 0
			m.remove.offset = 0
			m.reindex()

		case key.Matches(msg, keys.ReverseSort):
			m.remove.sortAscending = !m.remove.sortAscending
			m.remove.cursor = 0
			m.remove.offset = 0
			m.reindex()

		case key.Matches(msg, keys.Down):
			if m.remove.cursor < len(m.remove.visibleIndices)-1 {
				m.remove.cursor++
				if m.remove.cursor >= m.remove.offset+m.height {
					m.remove.offset = m.remove.cursor - m.height + 1
				}
			}

		case key.Matches(msg, keys.Up):
			if m.remove.cursor > 0 {
				m.remove.cursor--
				if m.remove.cursor < m.remove.offset {
					m.remove.offset = m.remove.cursor
				}
			}

		case key.Matches(msg, keys.PageDown):
			m.remove.cursor += m.height
			if m.remove.cursor >= len(m.remove.visibleIndices) {
				m.remove.cursor = len(m.remove.visibleIndices) - 1
			}
			if m.remove.cursor < 0 {
				m.remove.cursor = 0
			}
			if m.remove.cursor >= m.remove.offset+m.height {
				m.remove.offset = m.remove.cursor - m.height + 1
			}

		case key.Matches(msg, keys.PageUp):
			m.remove.cursor -= m.height
			if m.remove.cursor < 0 {
				m.remove.cursor = 0
			}
			if m.remove.cursor < m.remove.offset {
				m.remove.offset = m.remove.cursor
			}

		case key.Matches(msg, keys.Toggle):
			if len(m.remove.visibleIndices) > 0 {
				wt := m.remove.worktrees[m.remove.visibleIndices[m.remove.cursor]]
				if git.IsProtectedBranch(wt.Branch) {
					break
				}
				if m.remove.selected[wt.Path] {
					delete(m.remove.selected, wt.Path)
				} else {
					m.remove.selected[wt.Path] = true
				}
			}

		case key.Matches(msg, keys.All):
			allSelected := true
			for _, idx := range m.remove.visibleIndices {
				wt := m.remove.worktrees[idx]
				if git.IsProtectedBranch(wt.Branch) {
					continue
				}
				if !m.remove.selected[wt.Path] {
					allSelected = false
					break
				}
			}
			if allSelected {
				for _, idx := range m.remove.visibleIndices {
					wt := m.remove.worktrees[idx]
					if git.IsProtectedBranch(wt.Branch) {
						continue
					}
					delete(m.remove.selected, wt.Path)
				}
			} else {
				for _, idx := range m.remove.visibleIndices {
					wt := m.remove.worktrees[idx]
					if git.IsProtectedBranch(wt.Branch) {
						continue
					}
					m.remove.selected[wt.Path] = true
				}
			}

		case key.Matches(msg, keys.Confirm):
			if len(m.remove.selected) > 0 {
				m.view = confirmView
			}
		}
	}
	return m, nil
}

func (m Model) updateFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.remove.filterActive = false
		m.remove.filterText = ""
		m.remove.filterInput.SetValue("")
		m.remove.filterInput.Blur()
		m.reindex()
		return m, nil

	case key.Matches(msg, keys.Confirm):
		m.remove.filterActive = false
		m.remove.filterText = m.remove.filterInput.Value()
		m.remove.filterInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.remove.filterInput, cmd = m.remove.filterInput.Update(msg)
	m.remove.filterText = m.remove.filterInput.Value()
	m.reindex()
	return m, cmd
}

func (m Model) viewList() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("sentei - Git Worktree Cleanup"))
	b.WriteString("\n\n")

	if len(m.remove.worktrees) == 0 {
		b.WriteString(styleDim.Render("  No worktrees found."))
		b.WriteString("\n")
		return b.String()
	}

	if len(m.remove.visibleIndices) == 0 {
		b.WriteString(styleDim.Render("  No matches."))
		b.WriteString("\n\n")
		b.WriteString(m.viewStatusOrFilter())
		b.WriteString("\n")
		b.WriteString(m.viewBottomLine())
		return b.String()
	}

	end := min(m.remove.offset+m.height, len(m.remove.visibleIndices))

	arrow := " ▲"
	if !m.remove.sortAscending {
		arrow = " ▼"
	}
	hdrBranch := "Branch"
	hdrAge := "Age"
	hdrSubject := "Subject"
	switch m.remove.sortField {
	case SortByBranch:
		hdrBranch += arrow
	case SortByAge:
		hdrAge += arrow
	}

	t := table.New().
		BorderTop(false).BorderBottom(false).
		BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderHeader(false).BorderRow(false).
		Headers("", "", "", hdrBranch, hdrAge, hdrSubject).
		Wrap(true)

	if m.width > 0 {
		t.Width(m.width)
	}

	fixedWidth := colWidthCursor + colWidthCheckbox + colWidthStatus + colWidthAge
	colPadding := 3
	remaining := max(m.width-fixedWidth-colPadding, 20)
	branchWidth := remaining / 2
	subjectWidth := remaining - branchWidth

	for i := m.remove.offset; i < end; i++ {
		wt := m.remove.worktrees[m.remove.visibleIndices[i]]

		cursor := "  "
		if i == m.remove.cursor {
			cursor = "> "
		}

		var checkbox string
		if git.IsProtectedBranch(wt.Branch) {
			checkbox = styleStatusProtected.Render("[P]")
		} else if m.remove.selected[wt.Path] {
			checkbox = "[x]"
		} else {
			checkbox = "[ ]"
		}

		status := statusIndicator(wt)

		branch := stripBranchPrefix(wt.Branch)
		if branch == "" {
			switch {
			case wt.IsDetached:
				branch = wt.HEAD
				if len(branch) >= 7 {
					branch = branch[:7]
				}
			case wt.IsPrunable:
				branch = "(prunable)"
			}
		}

		age := relativeTime(wt.LastCommitDate)
		subject := wt.LastCommitSubject
		if wt.EnrichmentError != "" {
			age = "error"
			subject = wt.EnrichmentError
		}

		maxSubject := subjectWidth - 2
		if maxSubject > 3 && lipgloss.Width(subject) > maxSubject {
			runes := []rune(subject)
			if len(runes) > maxSubject-3 {
				subject = string(runes[:maxSubject-3]) + "..."
			}
		}

		t.Row(cursor, checkbox, status, branch, age, subject)
	}

	sortedCol := colAge
	if m.remove.sortField == SortByBranch {
		sortedCol = colBranch
	}

	t.StyleFunc(func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			base := styleColumnHeader
			if col == sortedCol {
				base = styleColumnHeaderSorted
			}
			return columnStyle(base, col, branchWidth, subjectWidth)
		}

		idx := m.remove.offset + row

		var base lipgloss.Style
		switch {
		case idx == m.remove.cursor:
			base = styleCursorRow
		case m.remove.selected[m.remove.worktrees[m.remove.visibleIndices[idx]].Path]:
			base = styleSelectedRow
		default:
			base = styleNormalRow
		}

		return columnStyle(base, col, branchWidth, subjectWidth)
	})

	b.WriteString(t.Render())
	b.WriteString("\n")

	b.WriteString(m.viewStatusOrFilter())
	b.WriteString("\n")
	b.WriteString(m.viewBottomLine())
	return b.String()
}

func (m Model) viewStatusOrFilter() string {
	if m.remove.filterActive {
		return m.remove.filterInput.View()
	}
	return m.viewStatusBar()
}

func (m Model) viewBottomLine() string {
	if m.remove.filterActive {
		return styleDim.Render("  enter: apply | esc: cancel")
	}
	return m.viewLegend()
}

func (m Model) viewStatusBar() string {
	count := len(m.remove.selected)

	var filterInfo string
	if m.remove.filterText != "" {
		filterInfo = fmt.Sprintf(" | filter: %q (%d/%d)", m.remove.filterText, len(m.remove.visibleIndices), len(m.remove.worktrees))
	}

	return styleStatusBar.Render(
		fmt.Sprintf("  %d selected%s | space: toggle | a: all | enter: delete | /: filter | s: sort | q: quit", count, filterInfo),
	)
}

func (m Model) viewLegend() string {
	return styleDim.Render("  ") +
		styleStatusClean.Render("[ok]") + styleDim.Render(" clean  ") +
		styleStatusDirty.Render("[~]") + styleDim.Render(" dirty  ") +
		styleStatusUntracked.Render("[!]") + styleDim.Render(" untracked  ") +
		styleStatusLocked.Render("[L]") + styleDim.Render(" locked  ") +
		styleStatusProtected.Render("[P]") + styleDim.Render(" protected")
}

func columnStyle(base lipgloss.Style, col, branchWidth, subjectWidth int) lipgloss.Style {
	switch col {
	case colCursor:
		return base.Width(colWidthCursor)
	case colCheckbox:
		return base.Width(colWidthCheckbox)
	case colStatus:
		return base.Width(colWidthStatus)
	case colBranch:
		return base.Width(branchWidth).Padding(0, 1)
	case colAge:
		return base.Width(colWidthAge).Padding(0, 1)
	case colSubject:
		return base.Width(subjectWidth).Padding(0, 1)
	default:
		return base
	}
}
```

- [ ] **Step 3: Update confirm.go to use m.remove**

Replace `internal/tui/confirm.go` with:

```go
package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Yes):
			m.view = progressView
			selected := m.selectedWorktrees()
			m.remove.deletionTotal = len(selected)
			for _, wt := range selected {
				m.remove.deletionStatuses[wt.Path] = statusPending
			}
			ch := make(chan worktree.DeletionEvent, len(selected)*2)
			m.remove.progressCh = ch
			go worktree.DeleteWorktrees(os.RemoveAll, selected, 5, ch)
			return m, waitForDeletionEvent(m.remove.progressCh)

		case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
			m.view = listView
		}
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	var b strings.Builder

	selected := m.selectedWorktrees()

	b.WriteString(styleHeader.Render("  Confirm Deletion  "))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "  You are about to delete %d worktree(s):\n\n", len(selected))

	var dirtyCount, untrackedCount, lockedCount int
	for _, wt := range selected {
		branch := stripBranchPrefix(wt.Branch)

		var label string
		switch {
		case wt.IsLocked:
			label = styleWarning.Render("[L] LOCKED - will force-remove")
			lockedCount++
		case wt.HasUncommittedChanges:
			label = styleWarning.Render("[~] HAS UNCOMMITTED CHANGES")
			dirtyCount++
		case wt.HasUntrackedFiles:
			label = styleWarning.Render("[!] has untracked files")
			untrackedCount++
		default:
			label = styleSuccess.Render("(clean)")
		}

		fmt.Fprintf(&b, "    * %s %s\n", branch, label)
	}

	b.WriteString("\n")

	if dirtyCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  WARNING: %d worktree(s) have uncommitted changes that will be LOST", dirtyCount),
		))
		b.WriteString("\n")
	}
	if untrackedCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  WARNING: %d worktree(s) have untracked files that will be LOST", untrackedCount),
		))
		b.WriteString("\n")
	}
	if lockedCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  WARNING: %d worktree(s) are locked and will be force-removed", lockedCount),
		))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString("  [y] Yes, delete  |  [n] No, go back\n")

	return styleDialogBox.Render(b.String())
}
```

- [ ] **Step 4: Update progress.go to use m.remove**

Replace `internal/tui/progress.go` with:

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

const (
	statusPending  = "pending"
	statusRemoving = "removing"
	statusRemoved  = "removed"
	statusFailed   = "failed"

	progressBarWidth = 40
)

type cleanupCompleteMsg struct {
	Result cleanup.Result
}

type worktreeDeleteStartedMsg struct{ Path string }
type worktreeDeletedMsg struct{ Path string }
type worktreeDeleteFailedMsg struct {
	Path string
	Err  error
}
type allDeletionsCompleteMsg struct{}
type pruneCompleteMsg struct{ Err error }

func waitForDeletionEvent(ch <-chan worktree.DeletionEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return allDeletionsCompleteMsg{}
		}
		switch ev.Type {
		case worktree.DeletionStarted:
			return worktreeDeleteStartedMsg{Path: ev.Path}
		case worktree.DeletionCompleted:
			return worktreeDeletedMsg{Path: ev.Path}
		case worktree.DeletionFailed:
			return worktreeDeleteFailedMsg{Path: ev.Path, Err: ev.Error}
		default:
			return waitForDeletionEvent(ch)()
		}
	}
}

func runPrune(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		err := worktree.PruneWorktrees(runner, repoPath)
		return pruneCompleteMsg{Err: err}
	}
}

func runCleanup(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		result := cleanup.Run(runner, repoPath, cleanup.Options{Mode: cleanup.ModeSafe}, func(cleanup.Event) {})
		return cleanupCompleteMsg{Result: result}
	}
}

func (m Model) updateProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreeDeleteStartedMsg:
		m.remove.deletionStatuses[msg.Path] = statusRemoving
		return m, waitForDeletionEvent(m.remove.progressCh)

	case worktreeDeletedMsg:
		m.remove.deletionStatuses[msg.Path] = statusRemoved
		m.remove.deletionResult.SuccessCount++
		m.remove.deletionResult.Outcomes = append(m.remove.deletionResult.Outcomes, worktree.WorktreeOutcome{
			Path:    msg.Path,
			Success: true,
		})
		return m, waitForDeletionEvent(m.remove.progressCh)

	case worktreeDeleteFailedMsg:
		m.remove.deletionStatuses[msg.Path] = statusFailed
		m.remove.deletionResult.FailureCount++
		m.remove.deletionResult.Outcomes = append(m.remove.deletionResult.Outcomes, worktree.WorktreeOutcome{
			Path:    msg.Path,
			Success: false,
			Error:   fmt.Errorf("removing %s: %w", msg.Path, msg.Err),
		})
		return m, waitForDeletionEvent(m.remove.progressCh)

	case allDeletionsCompleteMsg:
		return m, runPrune(m.runner, m.repoPath)

	case pruneCompleteMsg:
		pruneErr := msg.Err
		m.remove.pruneErr = &pruneErr
		return m, runCleanup(m.runner, m.repoPath)

	case cleanupCompleteMsg:
		m.remove.cleanupResult = &msg.Result
		m.view = summaryView
	}
	return m, nil
}

func (m Model) viewProgress() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("  Removing Worktrees  "))
	b.WriteString("\n\n")

	done := len(m.remove.deletionResult.Outcomes)
	pct := 0
	if m.remove.deletionTotal > 0 {
		pct = done * 100 / m.remove.deletionTotal
	}

	filled := progressBarWidth * pct / 100
	empty := progressBarWidth - filled

	bar := strings.Repeat("#", filled) + strings.Repeat("-", empty)
	fmt.Fprintf(&b, "  [%s] %d/%d (%d%%)\n\n", bar, done, m.remove.deletionTotal, pct)

	selected := m.selectedWorktrees()
	for _, wt := range selected {
		branch := stripBranchPrefix(wt.Branch)
		status := m.remove.deletionStatuses[wt.Path]

		var indicator string
		switch status {
		case statusRemoved:
			indicator = styleSuccess.Render("v") + " " + branch + "  " + styleDim.Render("removed")
		case statusFailed:
			indicator = styleError.Render("x") + " " + branch + "  " + styleError.Render("failed")
		case statusRemoving:
			indicator = styleWarning.Render("~") + " " + branch + "  " + styleDim.Render("removing...")
		default:
			indicator = styleDim.Render(".") + " " + branch + "  " + styleDim.Render("pending")
		}

		b.WriteString("  " + indicator + "\n")
	}

	return b.String()
}
```

- [ ] **Step 5: Update summary.go to use m.remove**

Replace `internal/tui/summary.go` with:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit), key.Matches(msg, keys.Confirm):
			if m.menuItems != nil {
				m.view = menuView
				return m, nil
			}
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			if m.menuItems != nil {
				m.view = menuView
				return m, nil
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewSummary() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("  Summary  "))
	b.WriteString("\n\n")

	r := m.remove.deletionResult
	if r.FailureCount == 0 {
		b.WriteString(styleSuccess.Render(
			fmt.Sprintf("  %d worktree(s) removed successfully", r.SuccessCount),
		))
		b.WriteString("\n")
	} else {
		fmt.Fprintf(&b, "  %s, %s\n",
			styleSuccess.Render(fmt.Sprintf("%d removed", r.SuccessCount)),
			styleError.Render(fmt.Sprintf("%d failed", r.FailureCount)),
		)
		b.WriteString("\n")
		b.WriteString(styleError.Render("  Failures:\n"))
		for _, o := range r.Outcomes {
			if !o.Success {
				fmt.Fprintf(&b, "    x %s: %s\n", o.Path, o.Error)
			}
		}
	}

	b.WriteString("\n")
	if m.remove.pruneErr != nil && *m.remove.pruneErr != nil {
		b.WriteString(styleWarning.Render(fmt.Sprintf("  Warning: failed to prune worktree metadata: %s", *m.remove.pruneErr)))
		b.WriteString("\n")
	} else {
		b.WriteString(styleDim.Render("  Pruned orphaned worktree metadata"))
		b.WriteString("\n")
	}
	if m.remove.cleanupResult != nil {
		r := m.remove.cleanupResult
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  Cleanup:"))
		b.WriteString("\n")
		if r.StaleRefsRemoved > 0 {
			fmt.Fprintf(&b, "    %s Pruned %d remote ref(s)\n", styleSuccess.Render("v"), r.StaleRefsRemoved)
		}
		if r.ConfigDedupResult.Removed > 0 {
			fmt.Fprintf(&b, "    %s Removed %d config duplicates\n", styleSuccess.Render("v"), r.ConfigDedupResult.Removed)
		}
		if r.GoneBranchesDeleted > 0 {
			fmt.Fprintf(&b, "    %s Deleted %d branch(es) with gone upstream\n", styleSuccess.Render("v"), r.GoneBranchesDeleted)
		}
		if r.ConfigOrphanResult.Removed > 0 {
			fmt.Fprintf(&b, "    %s Removed %d orphaned config section(s)\n", styleSuccess.Render("v"), r.ConfigOrphanResult.Removed)
		}
		if r.NonWtBranchesRemaining > 0 {
			b.WriteString("\n")
			b.WriteString(styleDim.Render(fmt.Sprintf("  Tip: %d local branch(es) not in any worktree.", r.NonWtBranchesRemaining)))
			b.WriteString("\n")
			b.WriteString(styleDim.Render("       Run `sentei cleanup --mode=aggressive` to remove them."))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.menuItems != nil {
		b.WriteString(styleDim.Render("  Press enter to return to menu, or q to quit"))
	} else {
		b.WriteString(styleDim.Render("  Press q, enter, or esc to exit"))
	}
	b.WriteString("\n")

	return b.String()
}
```

- [ ] **Step 6: Add stub methods for new views**

Create placeholder methods so the code compiles. These will be fully implemented in later tasks. Add to `internal/tui/menu.go`:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) viewMenu() string {
	return ""
}
```

Create `internal/tui/create_branch.go`:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateCreateBranch(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) viewCreateBranch() string {
	return ""
}
```

Create `internal/tui/create_options.go`:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateCreateOptions(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) viewCreateOptions() string {
	return ""
}
```

Create `internal/tui/create_progress.go`:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateCreateProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) viewCreateProgress() string {
	return ""
}
```

Create `internal/tui/create_summary.go`:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateCreateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) viewCreateSummary() string {
	return ""
}
```

- [ ] **Step 7: Verify build and existing tests pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./... && go test ./...`

Expected: Build succeeds. All existing tests pass. This is the critical verification — the model restructure must be transparent to existing behavior.

**Commit message:** `refactor(tui): restructure Model into grouped state (removeState, createState, menuState)`

---

## Task 8: Menu view

**Files:**
- Modify: `internal/tui/menu.go` (replace stub)
- Modify: `internal/tui/keys.go`

- [ ] **Step 1: Add Tab key binding to keys.go**

Add to the `keyMap` struct in `internal/tui/keys.go`:

```go
Tab key.Binding
```

Add to the `keys` initializer:

```go
Tab: key.NewBinding(
	key.WithKeys("tab"),
	key.WithHelp("tab", "switch field"),
),
```

- [ ] **Step 2: Implement menu view**

Replace `internal/tui/menu.go` with:

```go
package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

type worktreeContextMsg struct {
	worktrees []git.Worktree
	err       error
}

func loadWorktreeContext(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		wts, err := git.ListWorktrees(runner, repoPath)
		if err != nil {
			return worktreeContextMsg{err: err}
		}
		wts = worktree.EnrichWorktrees(runner, wts, 10)
		var filtered []git.Worktree
		for _, wt := range wts {
			if !wt.IsBare {
				filtered = append(filtered, wt)
			}
		}
		return worktreeContextMsg{worktrees: filtered}
	}
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(msg.Height-6, 5)
		return m, nil

	case worktreeContextMsg:
		if msg.err == nil {
			m.remove.worktrees = msg.worktrees
			m.reindex()
			m.updateMenuHints()
		}
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Down):
			for {
				m.menuCursor++
				if m.menuCursor >= len(m.menuItems) {
					m.menuCursor = len(m.menuItems) - 1
					break
				}
				if m.menuItems[m.menuCursor].enabled {
					break
				}
			}

		case key.Matches(msg, keys.Up):
			for {
				m.menuCursor--
				if m.menuCursor < 0 {
					m.menuCursor = 0
					break
				}
				if m.menuItems[m.menuCursor].enabled {
					break
				}
			}

		case key.Matches(msg, keys.Confirm):
			if m.menuCursor >= 0 && m.menuCursor < len(m.menuItems) && m.menuItems[m.menuCursor].enabled {
				switch m.menuCursor {
				case 0: // Create
					m.view = createBranchView
					return m, m.create.branchInput.Cursor.BlinkCmd()
				case 1: // Remove
					m.view = listView
					if len(m.remove.worktrees) == 0 {
						return m, loadWorktreeContext(m.runner, m.repoPath)
					}
				case 2: // Cleanup
					return m, tea.Quit
				}
			}
		}
	}
	return m, nil
}

func (m *Model) updateMenuHints() {
	if len(m.menuItems) < 2 {
		return
	}
	count := len(m.remove.worktrees)
	if count > 0 {
		m.menuItems[1].hint = fmt.Sprintf("%d available", count)
		m.menuItems[1].enabled = true
	} else {
		m.menuItems[1].hint = "none"
		m.menuItems[1].enabled = false
	}
}

func (m Model) viewMenu() string {
	var b strings.Builder

	repoName := filepath.Base(m.repoPath)
	b.WriteString(styleTitle.Render(fmt.Sprintf("  sentei %s Git Worktree Manager", "\u2500")))
	b.WriteString("\n\n")

	b.WriteString(styleDim.Render(fmt.Sprintf("  %s (bare) %s %s", repoName, "\u00b7", m.repoPath)))
	b.WriteString("\n")

	if len(m.remove.worktrees) > 0 {
		clean, dirty, locked := 0, 0, 0
		for _, wt := range m.remove.worktrees {
			switch {
			case wt.IsLocked:
				locked++
			case wt.HasUncommittedChanges || wt.HasUntrackedFiles:
				dirty++
			default:
				clean++
			}
		}
		b.WriteString(styleDim.Render(fmt.Sprintf("  %d worktrees %s %d clean, %d dirty, %d locked",
			len(m.remove.worktrees), "\u00b7", clean, dirty, locked)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	for i, item := range m.menuItems {
		cursor := "  "
		if i == m.menuCursor {
			cursor = "> "
		}

		label := item.label
		if !item.enabled {
			label = styleDim.Render(label)
		}

		hint := ""
		if item.hint != "" {
			hint = "  " + styleDim.Render(item.hint)
		}

		if i == m.menuCursor {
			b.WriteString(styleAccent.Render(cursor) + label + hint)
		} else {
			b.WriteString("  " + label + hint)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  j/k navigate \u00b7 enter select \u00b7 q quit"))
	b.WriteString("\n")

	return b.String()
}
```

- [ ] **Step 3: Verify build and tests pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./... && go test ./...`

Expected: Build succeeds, all tests pass.

**Commit message:** `feat(tui): implement menu view as new entry point with lazy worktree loading`

---

## Task 9: Create branch view

**Files:**
- Modify: `internal/tui/create_branch.go` (replace stub)

- [ ] **Step 1: Implement branch input view**

Replace `internal/tui/create_branch.go` with:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/ecosystem"
	"github.com/bonboncinnabon/sentei/internal/integration"
)

type branchValidationError struct {
	message string
}

func validateBranchName(name string, existingWorktrees []string) *branchValidationError {
	if name == "" {
		return &branchValidationError{message: "branch name is required"}
	}
	if strings.Contains(name, " ") {
		return &branchValidationError{message: "branch name cannot contain spaces"}
	}
	if strings.Contains(name, "..") {
		return &branchValidationError{message: "branch name cannot contain '..'"}
	}
	sanitized := creator.SanitizeBranchPath(name)
	for _, wt := range existingWorktrees {
		if strings.HasSuffix(wt, "/"+sanitized) || wt == sanitized {
			return &branchValidationError{message: fmt.Sprintf("worktree %q already exists", sanitized)}
		}
	}
	return nil
}

func (m Model) updateCreateBranch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(msg.Height-6, 5)
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Back):
			m.view = menuView
			return m, nil

		case key.Matches(msg, keys.Tab):
			if m.create.focusedField == 0 {
				m.create.focusedField = 1
				m.create.branchInput.Blur()
				m.create.baseInput.Focus()
				return m, m.create.baseInput.Cursor.BlinkCmd()
			}
			m.create.focusedField = 0
			m.create.baseInput.Blur()
			m.create.branchInput.Focus()
			return m, m.create.branchInput.Cursor.BlinkCmd()

		case key.Matches(msg, keys.Confirm):
			branch := m.create.branchInput.Value()
			var existingPaths []string
			for _, wt := range m.remove.worktrees {
				existingPaths = append(existingPaths, wt.Path)
			}
			if err := validateBranchName(branch, existingPaths); err != nil {
				return m, nil
			}

			m.prepareCreateOptions()
			m.view = createOptionsView
			return m, nil
		}

		// Forward key to focused input
		var cmd tea.Cmd
		if m.create.focusedField == 0 {
			m.create.branchInput, cmd = m.create.branchInput.Update(msg)
		} else {
			m.create.baseInput, cmd = m.create.baseInput.Update(msg)
		}
		return m, cmd
	}
	return m, nil
}

func (m *Model) prepareCreateOptions() {
	if m.cfg == nil {
		return
	}

	// Detect ecosystems from a source worktree
	sourceWT := m.findSourceWorktree()
	if sourceWT != "" {
		registry := ecosystem.NewRegistry(m.cfg.Ecosystems)
		detected, _ := registry.Detect(sourceWT)
		m.create.ecosystems = nil
		for _, eco := range detected {
			m.create.ecosystems = append(m.create.ecosystems, eco.Config)
			m.create.ecoEnabled[eco.Name] = true
		}
	}

	// Load integrations
	m.create.integrations = nil
	enabledSet := make(map[string]bool)
	for _, name := range m.cfg.IntegrationsEnabled {
		enabledSet[name] = true
	}
	for _, integ := range integration.All() {
		m.create.integrations = append(m.create.integrations, integ)
		m.create.intEnabled[integ.Name] = enabledSet[integ.Name]
	}
}

func (m Model) findSourceWorktree() string {
	for _, wt := range m.remove.worktrees {
		branch := stripBranchPrefix(wt.Branch)
		if branch == "main" || branch == "master" {
			return wt.Path
		}
	}
	if len(m.remove.worktrees) > 0 {
		return m.remove.worktrees[0].Path
	}
	return ""
}

func (m Model) viewCreateBranch() string {
	var b strings.Builder

	b.WriteString(styleTitle.Render(fmt.Sprintf("  sentei %s Create Worktree", "\u2500")))
	b.WriteString("\n\n")

	b.WriteString(styleDim.Render(fmt.Sprintf("  %s %s %s", strings.TrimPrefix(m.repoPath, ""), "\u00b7", m.repoPath)))
	b.WriteString("\n\n")

	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	b.WriteString("  Branch name\n")
	if m.create.focusedField == 0 {
		b.WriteString("  " + m.create.branchInput.View())
	} else {
		val := m.create.branchInput.Value()
		if val == "" {
			val = styleDim.Render("(empty)")
		}
		b.WriteString("    " + val)
	}
	b.WriteString("\n\n")

	b.WriteString("  Base branch\n")
	if m.create.focusedField == 1 {
		b.WriteString("  " + m.create.baseInput.View())
	} else {
		val := m.create.baseInput.Value()
		if val == "" {
			val = "main"
		}
		b.WriteString("    " + val)
	}
	b.WriteString("\n\n")

	b.WriteString(separator(m.width))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  enter continue \u00b7 tab switch field \u00b7 esc back"))
	b.WriteString("\n")

	return b.String()
}
```

- [ ] **Step 2: Verify build and tests pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./... && go test ./...`

Expected: Build succeeds, all tests pass.

**Commit message:** `feat(tui): implement create branch view with input validation`

---

## Task 10: Create options view

**Files:**
- Modify: `internal/tui/create_options.go` (replace stub)

- [ ] **Step 1: Implement options view**

Replace `internal/tui/create_options.go` with:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/integration"
)

type optionItem struct {
	label       string
	description string
	hint        string
	key         string
	section     string // "setup" or "integration"
}

func (m Model) buildOptionItems() []optionItem {
	var items []optionItem

	for _, eco := range m.create.ecosystems {
		items = append(items, optionItem{
			label:   fmt.Sprintf("Install dependencies (%s)", eco.Name),
			hint:    eco.Name + " detected",
			key:     "eco:" + eco.Name,
			section: "setup",
		})
	}

	items = append(items, optionItem{
		label:   "Merge default branch",
		hint:    fmt.Sprintf("%s \u2192 %s", m.create.baseInput.Value(), m.create.branchInput.Value()),
		key:     "merge",
		section: "setup",
	})

	hasEnvFiles := false
	var envFileNames []string
	for _, eco := range m.create.ecosystems {
		if len(eco.EnvFiles) > 0 {
			hasEnvFiles = true
			envFileNames = append(envFileNames, eco.EnvFiles...)
		}
	}
	if hasEnvFiles {
		items = append(items, optionItem{
			label:   "Copy environment files",
			hint:    strings.Join(envFileNames, ", "),
			key:     "envfiles",
			section: "setup",
		})
	}

	for _, integ := range m.create.integrations {
		items = append(items, optionItem{
			label:       integ.Name,
			description: integ.Description,
			hint:        integ.URL,
			key:         "int:" + integ.Name,
			section:     "integration",
		})
	}

	return items
}

func (m Model) isOptionEnabled(item optionItem) bool {
	switch {
	case strings.HasPrefix(item.key, "eco:"):
		name := strings.TrimPrefix(item.key, "eco:")
		return m.create.ecoEnabled[name]
	case item.key == "merge":
		return m.create.mergeBase
	case item.key == "envfiles":
		return m.create.copyEnvFiles
	case strings.HasPrefix(item.key, "int:"):
		name := strings.TrimPrefix(item.key, "int:")
		return m.create.intEnabled[name]
	}
	return false
}

func (m *Model) toggleOption(item optionItem) {
	switch {
	case strings.HasPrefix(item.key, "eco:"):
		name := strings.TrimPrefix(item.key, "eco:")
		m.create.ecoEnabled[name] = !m.create.ecoEnabled[name]
	case item.key == "merge":
		m.create.mergeBase = !m.create.mergeBase
	case item.key == "envfiles":
		m.create.copyEnvFiles = !m.create.copyEnvFiles
	case strings.HasPrefix(item.key, "int:"):
		name := strings.TrimPrefix(item.key, "int:")
		m.create.intEnabled[name] = !m.create.intEnabled[name]
	}
}

func (m Model) updateCreateOptions(msg tea.Msg) (tea.Model, tea.Cmd) {
	items := m.buildOptionItems()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(msg.Height-6, 5)
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Back):
			m.view = createBranchView
			m.create.branchInput.Focus()
			return m, m.create.branchInput.Cursor.BlinkCmd()

		case key.Matches(msg, keys.Down):
			if m.create.optionsCursor < len(items)-1 {
				m.create.optionsCursor++
			}

		case key.Matches(msg, keys.Up):
			if m.create.optionsCursor > 0 {
				m.create.optionsCursor--
			}

		case key.Matches(msg, keys.Toggle):
			if m.create.optionsCursor < len(items) {
				m.toggleOption(items[m.create.optionsCursor])
			}

		case key.Matches(msg, keys.Confirm):
			m.startCreation()
			m.view = createProgressView
			return m, m.waitForCreateEvent()
		}
	}
	return m, nil
}

func (m *Model) startCreation() {
	var enabledEcos []config.EcosystemConfig
	for _, eco := range m.create.ecosystems {
		if m.create.ecoEnabled[eco.Name] {
			enabledEcos = append(enabledEcos, eco)
		}
	}

	var enabledInts []integration.Integration
	for _, integ := range m.create.integrations {
		if m.create.intEnabled[integ.Name] {
			enabledInts = append(enabledInts, integ)
		}
	}

	opts := creator.Options{
		BranchName:     m.create.branchInput.Value(),
		BaseBranch:     m.create.baseInput.Value(),
		RepoPath:       m.repoPath,
		SourceWorktree: m.findSourceWorktree(),
		MergeBase:      m.create.mergeBase,
		CopyEnvFiles:   m.create.copyEnvFiles,
		Ecosystems:     enabledEcos,
		Integrations:   enabledInts,
	}

	ch := make(chan creator.Event, 50)
	m.create.eventCh = ch

	go func() {
		result := creator.Run(m.runner, opts, func(e creator.Event) {
			ch <- e
		})
		close(ch)
		_ = result
	}()
}

func (m Model) waitForCreateEvent() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.create.eventCh
		if !ok {
			return createCompleteMsg{}
		}
		return createEventMsg{Event: ev}
	}
}

type createEventMsg struct {
	Event creator.Event
}
type createCompleteMsg struct{}

func (m Model) viewCreateOptions() string {
	var b strings.Builder
	items := m.buildOptionItems()

	branch := m.create.branchInput.Value()
	base := m.create.baseInput.Value()

	b.WriteString(styleTitle.Render(fmt.Sprintf("  sentei %s Create Worktree", "\u2500")))
	b.WriteString("\n\n")
	b.WriteString(styleAccent.Render(fmt.Sprintf("  %s \u2192 from %s", branch, base)))
	b.WriteString("\n\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	currentSection := ""
	for i, item := range items {
		if item.section != currentSection {
			currentSection = item.section
			sectionLabel := "Setup"
			if currentSection == "integration" {
				sectionLabel = "Integrations"
			}
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("  " + styleTitle.Render(sectionLabel))
			b.WriteString("\n\n")
		}

		cursor := "  "
		if i == m.create.optionsCursor {
			cursor = "> "
		}

		var checkbox string
		if m.isOptionEnabled(item) {
			checkbox = styleCheckboxOn.Render("[x]")
		} else {
			checkbox = styleCheckboxOff.Render("[ ]")
		}

		hint := ""
		if item.hint != "" {
			hint = "  " + styleDim.Render(item.hint)
		}

		if i == m.create.optionsCursor {
			b.WriteString(styleAccent.Render(cursor) + checkbox + " " + item.label + hint)
		} else {
			b.WriteString("  " + checkbox + " " + item.label + hint)
		}
		b.WriteString("\n")

		if item.description != "" {
			b.WriteString("        " + styleDim.Render(item.description))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  space toggle \u00b7 enter create \u00b7 esc back"))
	b.WriteString("\n")

	return b.String()
}
```

- [ ] **Step 2: Verify build and tests pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./... && go test ./...`

Expected: Build succeeds, all tests pass.

**Commit message:** `feat(tui): implement create options view with ecosystem and integration toggles`

---

## Task 11: Create progress view

**Files:**
- Modify: `internal/tui/create_progress.go` (replace stub)

- [ ] **Step 1: Implement create progress view**

Replace `internal/tui/create_progress.go` with:

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/creator"
)

type phaseDisplay struct {
	name   string
	steps  []stepDisplay
	done   int
	total  int
	failed int
}

type stepDisplay struct {
	name   string
	status creator.StepStatus
}

func (m Model) updateCreateProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(msg.Height-6, 5)
		return m, nil

	case createEventMsg:
		m.create.events = append(m.create.events, msg.Event)
		return m, m.waitForCreateEvent()

	case createCompleteMsg:
		m.view = createSummaryView
		return m, nil
	}
	return m, nil
}

func (m Model) buildPhaseDisplays() []phaseDisplay {
	phases := map[string]*phaseDisplay{}
	var order []string

	for _, ev := range m.create.events {
		pd, exists := phases[ev.Phase]
		if !exists {
			pd = &phaseDisplay{name: ev.Phase}
			phases[ev.Phase] = pd
			order = append(order, ev.Phase)
		}

		found := false
		for i := range pd.steps {
			if pd.steps[i].name == ev.Step {
				pd.steps[i].status = ev.Status
				found = true
				break
			}
		}
		if !found {
			pd.steps = append(pd.steps, stepDisplay{name: ev.Step, status: ev.Status})
		}
	}

	var result []phaseDisplay
	for _, name := range order {
		pd := phases[name]
		pd.total = len(pd.steps)
		for _, s := range pd.steps {
			switch s.status {
			case creator.StepDone:
				pd.done++
			case creator.StepFailed:
				pd.failed++
				pd.done++
			}
		}
		result = append(result, *pd)
	}

	return result
}

func (m Model) viewCreateProgress() string {
	var b strings.Builder

	branch := m.create.branchInput.Value()
	base := m.create.baseInput.Value()

	b.WriteString(styleTitle.Render(fmt.Sprintf("  sentei %s Creating Worktree", "\u2500")))
	b.WriteString("\n\n")
	b.WriteString(styleAccent.Render(fmt.Sprintf("  %s \u2192 from %s", branch, base)))
	b.WriteString("\n\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	displays := m.buildPhaseDisplays()

	for i, pd := range displays {
		isComplete := pd.done == pd.total && pd.total > 0
		hasFailure := pd.failed > 0
		isActive := !isComplete && pd.total > 0

		var headerStyle func(string) string
		var statusText string

		switch {
		case isComplete && !hasFailure:
			headerStyle = stylePhaseDone.Render
			statusText = fmt.Sprintf("%d/%d %s", pd.done, pd.total, styleIndicatorDone.Render(indicatorDone))
		case isComplete && hasFailure:
			headerStyle = stylePhaseActive.Render
			statusText = fmt.Sprintf("%d/%d %s", pd.done, pd.total, styleIndicatorWarning.Render(indicatorWarning))
		case isActive:
			headerStyle = stylePhaseActive.Render
			statusText = fmt.Sprintf("%d/%d", pd.done, pd.total)
		default:
			headerStyle = stylePhasePending.Render
			statusText = "pending"
		}

		headerLine := fmt.Sprintf("  %-30s %s", headerStyle(pd.name), styleDim.Render(statusText))
		b.WriteString(headerLine)
		b.WriteString("\n")

		if !isComplete || hasFailure {
			for _, step := range pd.steps {
				var ind string
				switch step.status {
				case creator.StepDone:
					ind = styleIndicatorDone.Render(indicatorDone)
				case creator.StepRunning:
					ind = styleIndicatorActive.Render(indicatorActive)
				case creator.StepFailed:
					ind = styleIndicatorFailed.Render(indicatorFailed)
				default:
					ind = styleIndicatorPending.Render(indicatorPending)
				}
				b.WriteString(fmt.Sprintf("  %s %s\n", ind, step.name))
			}
		}

		if i < len(displays)-1 {
			b.WriteString("\n")
		}
	}

	// Show pending phases that haven't started
	knownPhases := make(map[string]bool)
	for _, pd := range displays {
		knownPhases[pd.name] = true
	}
	pendingNames := []string{"Setup", "Dependencies", "Integrations"}
	for _, name := range pendingNames {
		if !knownPhases[name] {
			b.WriteString(fmt.Sprintf("\n  %-30s %s\n", stylePhasePending.Render(name), styleDim.Render("pending")))
		}
	}

	b.WriteString("\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n")

	return b.String()
}
```

- [ ] **Step 2: Verify build and tests pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./... && go test ./...`

Expected: Build succeeds, all tests pass.

**Commit message:** `feat(tui): implement create progress view with phased parallel reporting`

---

## Task 12: Create summary view

**Files:**
- Modify: `internal/tui/create_summary.go` (replace stub)

- [ ] **Step 1: Implement create summary view**

Replace `internal/tui/create_summary.go` with:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/creator"
)

func (m Model) updateCreateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Confirm):
			if m.menuItems != nil {
				m.view = menuView
				return m, nil
			}
			return m, tea.Quit
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewCreateSummary() string {
	var b strings.Builder

	branch := m.create.branchInput.Value()
	base := m.create.baseInput.Value()
	wtPath := m.repoPath + "/" + creator.SanitizeBranchPath(branch)

	hasFailures := false
	for _, ev := range m.create.events {
		if ev.Status == creator.StepFailed {
			hasFailures = true
			break
		}
	}

	b.WriteString(styleTitle.Render(fmt.Sprintf("  sentei %s Worktree Created", "\u2500")))
	b.WriteString("\n\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	if hasFailures {
		b.WriteString(fmt.Sprintf("  %s %s created with issues\n\n",
			styleIndicatorWarning.Render(indicatorWarning), branch))
	} else {
		b.WriteString(fmt.Sprintf("  %s %s ready\n\n",
			styleIndicatorDone.Render(indicatorDone), branch))
	}

	b.WriteString(fmt.Sprintf("    %-10s %s\n", styleDim.Render("Path"), wtPath))
	b.WriteString(fmt.Sprintf("    %-10s %s (from %s)\n", styleDim.Render("Branch"), branch, base))

	// Summarize ecosystems
	for _, eco := range m.create.ecosystems {
		if !m.create.ecoEnabled[eco.Name] {
			continue
		}
		status := styleIndicatorDone.Render(indicatorDone)
		for _, ev := range m.create.events {
			if ev.Phase == "Dependencies" && strings.Contains(ev.Step, eco.Name) && ev.Status == creator.StepFailed {
				status = styleIndicatorFailed.Render(indicatorFailed)
				if ev.Error != nil {
					status += "  " + styleError.Render(ev.Error.Error())
				}
				break
			}
		}
		b.WriteString(fmt.Sprintf("    %-10s %s %s\n", styleDim.Render("Deps"), eco.Name, status))
	}

	// Summarize integrations
	for _, integ := range m.create.integrations {
		if !m.create.intEnabled[integ.Name] {
			continue
		}
		status := styleIndicatorDone.Render(indicatorDone)
		for _, ev := range m.create.events {
			if ev.Phase == "Integrations" && strings.Contains(ev.Step, integ.Name) && ev.Status == creator.StepFailed {
				status = styleIndicatorFailed.Render(indicatorFailed)
				if ev.Error != nil {
					status += "  " + styleError.Render(ev.Error.Error())
				}
				break
			}
		}
		b.WriteString(fmt.Sprintf("    %-10s %s %s\n", styleDim.Render("Index"), integ.Name, status))
	}

	b.WriteString("\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("    cd %s\n", wtPath))
	b.WriteString("\n")

	if m.menuItems != nil {
		b.WriteString(styleDim.Render("  enter menu \u00b7 q quit"))
	} else {
		b.WriteString(styleDim.Render("  enter quit \u00b7 q quit"))
	}
	b.WriteString("\n")

	return b.String()
}
```

- [ ] **Step 2: Verify build and tests pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./... && go test ./...`

Expected: Build succeeds, all tests pass.

**Commit message:** `feat(tui): implement create summary view with success/failure display`

---

## Task 13: Enhanced confirm view

**Files:**
- Modify: `internal/tui/confirm.go`

- [ ] **Step 1: Add teardown info to confirm view**

In `internal/tui/confirm.go`, update `viewConfirm()` to scan for integration artifacts and display a "Cleaning up" section. Add the import for `integration` package and update `updateConfirm` to run teardown before deletion.

Replace `internal/tui/confirm.go`:

```go
package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

type teardownCompleteMsg struct {
	results []creator.StepResult
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Yes):
			m.view = progressView
			selected := m.selectedWorktrees()
			m.remove.deletionTotal = len(selected)
			for _, wt := range selected {
				m.remove.deletionStatuses[wt.Path] = statusPending
			}

			integrations := integration.All()
			hasTeardown := false
			for _, wt := range selected {
				if len(creator.ScanArtifacts(wt.Path, integrations)) > 0 {
					hasTeardown = true
					break
				}
			}

			if hasTeardown {
				return m, m.runTeardownPhase(selected, integrations)
			}

			ch := make(chan worktree.DeletionEvent, len(selected)*2)
			m.remove.progressCh = ch
			go worktree.DeleteWorktrees(os.RemoveAll, selected, 5, ch)
			return m, waitForDeletionEvent(m.remove.progressCh)

		case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
			m.view = listView
		}

	case teardownCompleteMsg:
		m.remove.teardownResults = msg.results
		selected := m.selectedWorktrees()
		ch := make(chan worktree.DeletionEvent, len(selected)*2)
		m.remove.progressCh = ch
		go worktree.DeleteWorktrees(os.RemoveAll, selected, 5, ch)
		return m, waitForDeletionEvent(m.remove.progressCh)
	}
	return m, nil
}

func (m Model) runTeardownPhase(worktrees []git.Worktree, integrations []integration.Integration) tea.Cmd {
	runner := m.runner
	return func() tea.Msg {
		var allResults []creator.StepResult
		for _, wt := range worktrees {
			results := creator.Teardown(runner, wt.Path, integrations, func(creator.Event) {})
			allResults = append(allResults, results...)
		}
		return teardownCompleteMsg{results: allResults}
	}
}

func (m Model) viewConfirm() string {
	var b strings.Builder

	selected := m.selectedWorktrees()

	b.WriteString(styleHeader.Render("  Confirm Deletion  "))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "  You are about to delete %d worktree(s):\n\n", len(selected))

	var dirtyCount, untrackedCount, lockedCount int
	for _, wt := range selected {
		branch := stripBranchPrefix(wt.Branch)

		var label string
		switch {
		case wt.IsLocked:
			label = styleWarning.Render("[L] LOCKED - will force-remove")
			lockedCount++
		case wt.HasUncommittedChanges:
			label = styleWarning.Render("[~] HAS UNCOMMITTED CHANGES")
			dirtyCount++
		case wt.HasUntrackedFiles:
			label = styleWarning.Render("[!] has untracked files")
			untrackedCount++
		default:
			label = styleSuccess.Render("(clean)")
		}

		fmt.Fprintf(&b, "    * %s %s\n", branch, label)
	}

	b.WriteString("\n")

	// Integration teardown info
	integrations := integration.All()
	type artifactSummary struct {
		dirName string
		count   int
	}
	dirCounts := make(map[string]int)
	for _, wt := range selected {
		artifacts := creator.ScanArtifacts(wt.Path, integrations)
		for _, a := range artifacts {
			for _, d := range a.Dirs {
				dirCounts[d]++
			}
		}
	}
	if len(dirCounts) > 0 {
		b.WriteString("  Cleaning up:\n\n")
		for dir, count := range dirCounts {
			noun := "worktree"
			if count > 1 {
				noun = "worktrees"
			}
			fmt.Fprintf(&b, "    %-28s in %d %s\n", dir, count, noun)
		}
		b.WriteString("\n")
	}

	if dirtyCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  WARNING: %d worktree(s) have uncommitted changes that will be LOST", dirtyCount),
		))
		b.WriteString("\n")
	}
	if untrackedCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  WARNING: %d worktree(s) have untracked files that will be LOST", untrackedCount),
		))
		b.WriteString("\n")
	}
	if lockedCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  WARNING: %d worktree(s) are locked and will be force-removed", lockedCount),
		))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString("  [y] Yes, delete  |  [n] No, go back\n")

	return styleDialogBox.Render(b.String())
}
```

Note: We need to add the `git` import since `runTeardownPhase` uses `git.Worktree`. Add to the import block:
```go
"github.com/bonboncinnabon/sentei/internal/git"
```

- [ ] **Step 2: Verify build and tests pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./... && go test ./...`

Expected: Build succeeds, all tests pass.

**Commit message:** `feat(tui): enhance confirm view with integration teardown info and execution`

---

## Task 14: Enhanced removal progress

**Files:**
- Modify: `internal/tui/progress.go`

- [ ] **Step 1: Update progress view to use phased reporting**

This is an optional enhancement — the progress view already works with the flat reporting from Task 7. The phased reporting (teardown → remove → prune/cleanup) can be layered in by updating `viewProgress()` to show phase headers.

Update `viewProgress()` in `internal/tui/progress.go`:

Replace the `viewProgress()` method body with:

```go
func (m Model) viewProgress() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("  Removing Worktrees  "))
	b.WriteString("\n\n")

	// Teardown phase (if any)
	if len(m.remove.teardownResults) > 0 {
		allDone := true
		hasFailed := false
		for _, r := range m.remove.teardownResults {
			if r.Status == creator.StepFailed {
				hasFailed = true
			}
			if r.Status != creator.StepDone && r.Status != creator.StepFailed {
				allDone = false
			}
		}
		_ = allDone

		statusText := fmt.Sprintf("%d/%d", len(m.remove.teardownResults), len(m.remove.teardownResults))
		if hasFailed {
			statusText += " " + styleIndicatorWarning.Render(indicatorWarning)
		} else {
			statusText += " " + styleIndicatorDone.Render(indicatorDone)
		}
		b.WriteString(fmt.Sprintf("  %-30s %s\n", stylePhaseDone.Render("Teardown"), styleDim.Render(statusText)))
		b.WriteString("\n")
	}

	// Remove phase
	done := len(m.remove.deletionResult.Outcomes)
	pct := 0
	if m.remove.deletionTotal > 0 {
		pct = done * 100 / m.remove.deletionTotal
	}

	phaseStatus := fmt.Sprintf("%d/%d", done, m.remove.deletionTotal)
	if done == m.remove.deletionTotal && m.remove.deletionTotal > 0 {
		phaseStatus += " " + styleIndicatorDone.Render(indicatorDone)
		b.WriteString(fmt.Sprintf("  %-30s %s\n", stylePhaseDone.Render("Removing worktrees"), styleDim.Render(phaseStatus)))
	} else {
		b.WriteString(fmt.Sprintf("  %-30s %s\n", stylePhaseActive.Render("Removing worktrees"), styleDim.Render(phaseStatus)))

		selected := m.selectedWorktrees()
		for _, wt := range selected {
			branch := stripBranchPrefix(wt.Branch)
			status := m.remove.deletionStatuses[wt.Path]

			var ind string
			switch status {
			case statusRemoved:
				ind = styleIndicatorDone.Render(indicatorDone)
			case statusFailed:
				ind = styleIndicatorFailed.Render(indicatorFailed)
			case statusRemoving:
				ind = styleIndicatorActive.Render(indicatorActive)
			default:
				ind = styleIndicatorPending.Render(indicatorPending)
			}

			b.WriteString(fmt.Sprintf("  %s %s\n", ind, branch))
		}
	}

	b.WriteString("\n")

	// Prune & cleanup phase
	if m.remove.pruneErr != nil {
		b.WriteString(fmt.Sprintf("  %-30s %s\n", stylePhaseDone.Render("Prune & cleanup"), styleDim.Render(styleIndicatorDone.Render(indicatorDone))))
	} else if done == m.remove.deletionTotal && m.remove.deletionTotal > 0 {
		b.WriteString(fmt.Sprintf("  %-30s %s\n", stylePhaseActive.Render("Prune & cleanup"), styleDim.Render(styleIndicatorActive.Render(indicatorActive))))
	} else {
		b.WriteString(fmt.Sprintf("  %-30s %s\n", stylePhasePending.Render("Prune & cleanup"), styleDim.Render("pending")))
	}

	_ = pct
	return b.String()
}
```

Also add the `creator` import to the import block in `progress.go`:

```go
"github.com/bonboncinnabon/sentei/internal/creator"
```

- [ ] **Step 2: Verify build and tests pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./... && go test ./...`

Expected: Build succeeds, all tests pass.

**Commit message:** `feat(tui): enhance removal progress with phased teardown/remove/prune reporting`

---

## Task 15: Main.go update

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Update main.go for lazy loading and menu entry**

Replace `main.go` with:

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/cmd"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/dryrun"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/playground"
	"github.com/bonboncinnabon/sentei/internal/tui"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	enrichConcurrency = 10
	playgroundDelay   = 800 * time.Millisecond
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "cleanup" {
		cmd.RunCleanup(os.Args[2:])
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "ecosystems" {
		cmd.RunEcosystems(os.Args[2:])
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "integrations" {
		cmd.RunIntegrations()
		return
	}

	versionFlag := flag.Bool("version", false, "Print version and exit")
	playgroundFlag := flag.Bool("playground", false, "Launch with a temporary test repo")
	dryRunFlag := flag.Bool("dry-run", false, "Print worktree summary and exit (no interactive TUI)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("sentei %s (%s, %s)\n", version, commit, date)
		os.Exit(0)
	}

	repoPath := "."
	if flag.NArg() > 0 {
		repoPath = flag.Arg(0)
	}

	if *playgroundFlag {
		var cleanup func()
		var err error
		repoPath, cleanup, err = playground.Setup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting up playground: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Playground repo: %s\n", repoPath)
		defer cleanup()
	}

	runner := &git.GitRunner{}

	// Validate this is a git repo before doing anything
	if err := git.ValidateRepository(runner, repoPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Dry-run mode: eager load worktrees and print
	if *dryRunFlag {
		worktrees, err := git.ListWorktrees(runner, repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		worktrees = worktree.EnrichWorktrees(runner, worktrees, enrichConcurrency)

		var filtered []git.Worktree
		for _, wt := range worktrees {
			if !wt.IsBare {
				filtered = append(filtered, wt)
			}
		}

		if len(filtered) == 0 {
			fmt.Println("No worktrees found (only the main working tree exists).")
			os.Exit(0)
		}

		if err := dryrun.Print(filtered, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Load config (best-effort — nil config is safe)
	cfg, err := config.LoadConfig(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
	}

	var tuiRunner git.CommandRunner = runner
	if *playgroundFlag {
		tuiRunner = &git.DelayRunner{Inner: runner, Delay: playgroundDelay}
	}

	// Start at menu — worktrees loaded lazily
	model := tui.NewMenuModel(tuiRunner, repoPath, cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build ./...`

Expected: Build succeeds.

- [ ] **Step 3: Run full test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./...`

Expected: All tests pass.

**Commit message:** `feat: update main.go to start at menu with lazy worktree loading and config`

---

## Task 16: E2E tests and final verification

**Files:**
- Create: `internal/creator/e2e_test.go`

- [ ] **Step 1: Write creator pipeline E2E test**

Create `internal/creator/e2e_test.go`:

```go
//go:build e2e

package creator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestE2E_CreateWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Create a bare repo with an initial commit on main
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test-repo.git")

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("command failed: %s %v: %v", args[0], args[1:], err)
		}
	}

	// Init bare repo
	os.MkdirAll(bareRepo, 0755)
	run(bareRepo, "git", "init", "--bare")

	// Create a temporary clone to make initial commit
	cloneDir := filepath.Join(tmpDir, "clone")
	run(tmpDir, "git", "clone", bareRepo, cloneDir)
	run(cloneDir, "git", "config", "user.email", "test@test.com")
	run(cloneDir, "git", "config", "user.name", "Test")

	// Create initial commit with go.mod
	os.WriteFile(filepath.Join(cloneDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)
	os.WriteFile(filepath.Join(cloneDir, ".env"), []byte("SECRET=test\n"), 0644)
	run(cloneDir, "git", "add", ".")
	run(cloneDir, "git", "commit", "-m", "initial commit")
	run(cloneDir, "git", "push", "origin", "main")

	// Create main worktree from bare repo
	mainWT := filepath.Join(tmpDir, "main")
	run(bareRepo, "git", "worktree", "add", mainWT, "main")

	// Run creator pipeline
	runner := &git.GitRunner{}

	opts := Options{
		BranchName:     "feature/test-create",
		BaseBranch:     "main",
		RepoPath:       bareRepo,
		SourceWorktree: mainWT,
		MergeBase:      false,
		CopyEnvFiles:   true,
		Ecosystems: []config.EcosystemConfig{
			{
				Name:     "go",
				EnvFiles: []string{".env"},
			},
		},
	}

	var events []Event
	result := Run(runner, opts, func(e Event) {
		events = append(events, e)
	})

	// Verify worktree was created
	expectedPath := filepath.Join(bareRepo, "feature-test-create")
	if result.WorktreePath != expectedPath {
		t.Errorf("WorktreePath = %q, want %q", result.WorktreePath, expectedPath)
	}

	// Verify directory exists
	if _, err := os.Stat(result.WorktreePath); os.IsNotExist(err) {
		t.Fatal("worktree directory was not created")
	}

	// Verify branch exists
	out, err := runner.Run(result.WorktreePath, "branch", "--show-current")
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if out != "feature/test-create" {
		t.Errorf("branch = %q, want %q", out, "feature/test-create")
	}

	// Verify env file was copied
	envPath := filepath.Join(result.WorktreePath, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("env file not copied: %v", err)
	}
	if string(data) != "SECRET=test\n" {
		t.Errorf("env file content = %q, want %q", string(data), "SECRET=test\n")
	}

	// Verify events were emitted
	if len(events) == 0 {
		t.Error("expected events to be emitted")
	}

	// Verify no failures (except possibly empty install command)
	for _, phase := range result.Phases {
		if phase.Name == "Setup" {
			for _, step := range phase.Steps {
				if step.Status == StepFailed && step.Name == "Create worktree" {
					t.Errorf("create worktree step failed: %v", step.Error)
				}
			}
		}
	}
}

func TestE2E_Teardown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tmpDir := t.TempDir()

	// Create fake integration artifacts
	crgDir := filepath.Join(tmpDir, ".code-review-graph")
	os.MkdirAll(crgDir, 0755)
	os.WriteFile(filepath.Join(crgDir, "graph.json"), []byte("{}"), 0644)

	cocDir := filepath.Join(tmpDir, ".cocoindex_code")
	os.MkdirAll(cocDir, 0755)
	os.WriteFile(filepath.Join(cocDir, "index.db"), []byte("data"), 0644)

	runner := &git.GitRunner{}
	integrations := []integration.Integration{
		{
			Name:     "code-review-graph",
			Teardown: integration.TeardownSpec{Dirs: []string{".code-review-graph/"}},
		},
		{
			Name:     "cocoindex-code",
			Teardown: integration.TeardownSpec{Dirs: []string{".cocoindex_code/"}},
		},
	}

	var events []Event
	results := Teardown(runner, tmpDir, integrations, func(e Event) {
		events = append(events, e)
	})

	// Both should succeed
	for _, r := range results {
		if r.Status != StepDone {
			t.Errorf("teardown %q: status = %v, want StepDone", r.Name, r.Status)
		}
	}

	// Verify directories removed
	if _, err := os.Stat(crgDir); !os.IsNotExist(err) {
		t.Error(".code-review-graph/ should be deleted")
	}
	if _, err := os.Stat(cocDir); !os.IsNotExist(err) {
		t.Error(".cocoindex_code/ should be deleted")
	}
}
```

Note: The E2E teardown test needs the `integration` import. Add to the import block:

```go
"github.com/bonboncinnabon/sentei/internal/integration"
```

- [ ] **Step 2: Run unit tests (not E2E)**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./...`

Expected: All unit tests pass. E2E tests are skipped (build tag).

- [ ] **Step 3: Run E2E tests**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test -tags=e2e ./internal/creator/ -v -run TestE2E`

Expected: E2E tests pass (creates real git repos in temp dirs).

- [ ] **Step 4: Full build verification**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build -o /tmp/sentei . && echo "Build OK"`

Expected: Binary builds successfully.

- [ ] **Step 5: Verify vet and format**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go vet ./... && gofmt -l .`

Expected: No vet errors, no unformatted files.

**Commit message:** `test(creator): add E2E tests for creation pipeline and teardown`

---

## Self-Review Checklist

1. **Every spec requirement has a task:**
   - [x] Creator pipeline types — Task 1
   - [x] Setup phase (create, merge, copy env) — Task 1
   - [x] Dependency installation with parallel workspaces — Task 2
   - [x] Integration setup (detect, deps, install, setup, gitignore) — Task 3
   - [x] Integration teardown (scan, command, fallback) — Task 4
   - [x] Pipeline orchestrator (`Run()`) — Task 5
   - [x] Visual language (phase/indicator/separator styles) — Task 6
   - [x] Model restructure (grouped state) — Task 7
   - [x] Menu view — Task 8
   - [x] Create branch view — Task 9
   - [x] Create options view — Task 10
   - [x] Create progress view — Task 11
   - [x] Create summary view — Task 12
   - [x] Enhanced confirm (teardown info) — Task 13
   - [x] Enhanced removal progress (phased) — Task 14
   - [x] main.go lazy loading + menu entry — Task 15
   - [x] E2E tests — Task 16

2. **No placeholders:** Every step contains complete code.

3. **Type names consistent across tasks:**
   - `StepStatus`, `StepResult`, `Phase`, `Event`, `Options`, `Result` — used consistently from Task 1 through Task 16
   - `removeState`, `createState` — defined in Task 7, used in Tasks 8-14
   - `viewState` constants — defined in Task 7, used throughout

4. **All existing tests pass after Task 7 restructure:** Task 7 Step 7 explicitly verifies this. The restructure is purely mechanical — moving fields from `Model` into `Model.remove` and updating all access sites.
