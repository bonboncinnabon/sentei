# Repo Operations — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add repo operations (create, clone, migrate) with context-aware menu, GitHub publish, and sentei re-launch.

**Architecture:** New `internal/repo/` package with three event-driven pipelines (create, clone, migrate) matching the `internal/creator/` pattern. TUI menu adapts based on context detection (bare/non-bare/no repo). After operations, sentei re-launches at the new repo via `tea.ExecProcess`.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, `gh` CLI for GitHub, existing `git.ShellRunner` + `git.CommandRunner`, `os/exec` for re-launch.

**Spec:** `docs/superpowers/specs/2026-03-30-repo-operations-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/repo/repo.go` | Create | Shared types (Event, StepStatus, StepResult, Phase), RepoContext, `DetectContext()` |
| `internal/repo/repo_test.go` | Create | Context detection tests |
| `internal/repo/create.go` | Create | Create repo pipeline (setup + GitHub phases) |
| `internal/repo/create_test.go` | Create | Create pipeline tests |
| `internal/repo/clone.go` | Create | Clone repo pipeline (clone + structure + worktree phases) |
| `internal/repo/clone_test.go` | Create | Clone pipeline tests |
| `internal/repo/migrate.go` | Create | Migrate repo pipeline (validate + backup + migrate + copy phases) |
| `internal/repo/migrate_test.go` | Create | Migrate pipeline tests |
| `internal/tui/model.go` | Modify | Add repo view states, repoState struct, RepoContext field |
| `internal/tui/menu.go` | Modify | Adapt menu items and header based on RepoContext |
| `internal/tui/repo_name.go` | Create | Create repo name/location input view |
| `internal/tui/repo_options.go` | Create | Create repo options with progressive GitHub disclosure |
| `internal/tui/repo_progress.go` | Create | Shared phased progress view for create/clone/migrate |
| `internal/tui/repo_summary.go` | Create | Shared summary view for create/clone with re-launch |
| `internal/tui/clone_input.go` | Create | Clone URL + derived name input view |
| `internal/tui/migrate_confirm.go` | Create | Migration confirmation view with dirty-repo warning |
| `internal/tui/migrate_summary.go` | Create | Migration summary with backup cleanup prompt + re-launch |
| `main.go` | Modify | Context detection before TUI, pass context to `NewMenuModel` |

---

## Task 1: Repo package types and context detection

**Files:**
- Create: `internal/repo/repo.go`
- Create: `internal/repo/repo_test.go`

- [ ] **Step 1: Write tests for DetectContext**

Create `internal/repo/repo_test.go`:

```go
package repo

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

func (m *mockRunner) RunShell(dir string, command string) (string, error) {
	key := fmt.Sprintf("%s:shell[%s]", dir, command)
	m.calls = append(m.calls, key)
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	return "", fmt.Errorf("unexpected shell call: %s", key)
}

type eventCollector struct {
	events []Event
}

func (c *eventCollector) emit(e Event) {
	c.events = append(c.events, e)
}

func TestDetectContext(t *testing.T) {
	tests := []struct {
		name      string
		responses map[string]mockResponse
		setupDir  func(t *testing.T, dir string)
		want      RepoContext
	}{
		{
			name: "bare repo detected via git",
			responses: map[string]mockResponse{
				"{dir}:[rev-parse --is-bare-repository]": {output: "true"},
			},
			want: ContextBareRepo,
		},
		{
			name: "worktree inside bare repo with .bare directory",
			responses: map[string]mockResponse{
				"{dir}:[rev-parse --is-bare-repository]": {output: "false"},
				"{dir}:[rev-parse --git-dir]":            {output: "/repo/.bare"},
				"{dir}:[rev-parse --show-toplevel]":      {output: "{dir}"},
			},
			setupDir: func(t *testing.T, dir string) {
				os.MkdirAll(filepath.Join(dir, ".bare"), 0755)
			},
			want: ContextBareRepo,
		},
		{
			name: "non-bare regular repo",
			responses: map[string]mockResponse{
				"{dir}:[rev-parse --is-bare-repository]": {output: "false"},
				"{dir}:[rev-parse --git-dir]":            {output: ".git"},
				"{dir}:[rev-parse --show-toplevel]":      {output: "{dir}"},
			},
			want: ContextNonBareRepo,
		},
		{
			name: "no repo at all",
			responses: map[string]mockResponse{
				"{dir}:[rev-parse --is-bare-repository]": {output: "", err: fmt.Errorf("not a git repository")},
				"{dir}:[rev-parse --git-dir]":            {output: "", err: fmt.Errorf("not a git repository")},
			},
			want: ContextNoRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Replace {dir} placeholders in response keys
			resolved := make(map[string]mockResponse)
			for k, v := range tt.responses {
				resolvedKey := strings.ReplaceAll(k, "{dir}", dir)
				resolved[resolvedKey] = v
			}

			if tt.setupDir != nil {
				tt.setupDir(t, dir)
			}

			runner := &mockRunner{responses: resolved}
			got := DetectContext(runner, dir)
			if got != tt.want {
				t.Errorf("DetectContext() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

Note: The test uses `strings.ReplaceAll` — add `"strings"` to imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/repo/ -v -run TestDetectContext`

Expected: Compilation error — `repo` package doesn't exist yet.

- [ ] **Step 3: Write shared types and DetectContext**

Create `internal/repo/repo.go`:

```go
package repo

import (
	"os"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/git"
)

type RepoContext int

const (
	ContextBareRepo    RepoContext = iota // bare repo with worktrees — full menu
	ContextNonBareRepo                    // regular git repo — offer migrate
	ContextNoRepo                         // not in a git repo — offer create/clone
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

// DetectContext determines the repo context at the given path.
//
// Detection logic:
//  1. git rev-parse --is-bare-repository → "true" means ContextBareRepo
//  2. Check for .bare directory at repo root (sentei's bare structure from a worktree)
//  3. git rev-parse --git-dir succeeds → ContextNonBareRepo
//  4. Otherwise → ContextNoRepo
func DetectContext(runner git.CommandRunner, path string) RepoContext {
	output, err := runner.Run(path, "rev-parse", "--is-bare-repository")
	if err == nil && output == "true" {
		return ContextBareRepo
	}

	// Check if git repo at all
	_, err = runner.Run(path, "rev-parse", "--git-dir")
	if err != nil {
		return ContextNoRepo
	}

	// Inside a git repo — check for sentei's .bare directory at repo root
	toplevel, err := runner.Run(path, "rev-parse", "--show-toplevel")
	if err == nil {
		bareDir := filepath.Join(toplevel, ".bare")
		if info, statErr := os.Stat(bareDir); statErr == nil && info.IsDir() {
			return ContextBareRepo
		}
	}

	return ContextNonBareRepo
}

func (r *Phase) HasFailures() bool {
	for _, s := range r.Steps {
		if s.Status == StepFailed {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/repo/ -v -run TestDetectContext`

Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo/
git commit -m "feat(repo): add shared types and context detection (bare/non-bare/no-repo)"
```

---

## Task 2: Create repo pipeline

**Files:**
- Create: `internal/repo/create.go`
- Create: `internal/repo/create_test.go`

- [ ] **Step 1: Write tests for Create pipeline**

Create `internal/repo/create_test.go`:

```go
package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCreate_LocalOnly(t *testing.T) {
	dir := t.TempDir()
	repoName := "my-project"
	repoPath := filepath.Join(dir, repoName)

	runner := &mockRunner{responses: map[string]mockResponse{
		fmt.Sprintf("%s/.bare:[init --bare]", repoPath):                     {output: ""},
		fmt.Sprintf("%s/.bare:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", repoPath): {output: ""},
		fmt.Sprintf("%s:[worktree add main -b main]", repoPath):            {output: ""},
		fmt.Sprintf("%s/main:[add -A]", repoPath):                          {output: ""},
		fmt.Sprintf("%s/main:[commit -m Initial commit]", repoPath):        {output: ""},
	}}

	ec := &eventCollector{}
	opts := CreateOptions{
		Name:          repoName,
		Location:      dir,
		PublishGitHub: false,
	}
	result := Create(runner, runner, opts, ec.emit)

	if result.RepoPath != repoPath {
		t.Errorf("RepoPath = %q, want %q", result.RepoPath, repoPath)
	}
	if len(result.Phases) != 1 {
		t.Errorf("want 1 phase (setup only), got %d", len(result.Phases))
	}
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
	if len(ec.events) == 0 {
		t.Error("expected events to be emitted")
	}
}

func TestCreate_WithGitHub(t *testing.T) {
	dir := t.TempDir()
	repoName := "my-project"
	repoPath := filepath.Join(dir, repoName)

	runner := &mockRunner{responses: map[string]mockResponse{
		// Setup phase
		fmt.Sprintf("%s/.bare:[init --bare]", repoPath):                     {output: ""},
		fmt.Sprintf("%s/.bare:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", repoPath): {output: ""},
		fmt.Sprintf("%s:[worktree add main -b main]", repoPath):            {output: ""},
		fmt.Sprintf("%s/main:[add -A]", repoPath):                          {output: ""},
		fmt.Sprintf("%s/main:[commit -m Initial commit]", repoPath):        {output: ""},
		// GitHub phase
		fmt.Sprintf("%s:shell[gh api user --jq .login]", repoPath):         {output: "bonboncinnabon"},
		fmt.Sprintf("%s/main:shell[gh repo create my-project --private --description \"\" --source . --push]", repoPath): {output: ""},
		fmt.Sprintf("%s/.bare:[remote set-url origin git@github.com:bonboncinnabon/my-project.git]", repoPath): {output: ""},
		fmt.Sprintf("%s/main:[push -u origin main]", repoPath):             {output: ""},
		fmt.Sprintf("%s/.bare:[remote set-head origin main]", repoPath):    {output: ""},
	}}

	ec := &eventCollector{}
	opts := CreateOptions{
		Name:          repoName,
		Location:      dir,
		PublishGitHub: true,
		Visibility:    "private",
		Description:   "",
	}
	result := Create(runner, runner, opts, ec.emit)

	if len(result.Phases) != 2 {
		t.Errorf("want 2 phases (setup + github), got %d", len(result.Phases))
	}
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
}

func TestCreate_DirAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	repoName := "existing"
	os.MkdirAll(filepath.Join(dir, repoName), 0755)

	runner := &mockRunner{responses: map[string]mockResponse{}}
	ec := &eventCollector{}
	opts := CreateOptions{
		Name:     repoName,
		Location: dir,
	}
	result := Create(runner, runner, opts, ec.emit)

	if len(result.Phases) == 0 {
		t.Fatal("expected at least one phase")
	}
	if !result.Phases[0].HasFailures() {
		t.Error("expected setup phase to have failures when dir already exists")
	}
}

func TestCreate_GitHubPhaseFailure_LocalStillUsable(t *testing.T) {
	dir := t.TempDir()
	repoName := "my-project"
	repoPath := filepath.Join(dir, repoName)

	runner := &mockRunner{responses: map[string]mockResponse{
		// Setup phase succeeds
		fmt.Sprintf("%s/.bare:[init --bare]", repoPath):                     {output: ""},
		fmt.Sprintf("%s/.bare:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", repoPath): {output: ""},
		fmt.Sprintf("%s:[worktree add main -b main]", repoPath):            {output: ""},
		fmt.Sprintf("%s/main:[add -A]", repoPath):                          {output: ""},
		fmt.Sprintf("%s/main:[commit -m Initial commit]", repoPath):        {output: ""},
		// GitHub phase fails at user lookup
		fmt.Sprintf("%s:shell[gh api user --jq .login]", repoPath):         {output: "", err: fmt.Errorf("gh: not authenticated")},
	}}

	ec := &eventCollector{}
	opts := CreateOptions{
		Name:          repoName,
		Location:      dir,
		PublishGitHub: true,
		Visibility:    "private",
	}
	result := Create(runner, runner, opts, ec.emit)

	// Local repo still usable despite GitHub failure
	if result.RepoPath != repoPath {
		t.Errorf("RepoPath should still be set: got %q", result.RepoPath)
	}
	if len(result.Phases) != 2 {
		t.Fatalf("want 2 phases, got %d", len(result.Phases))
	}
	if result.Phases[0].HasFailures() {
		t.Error("setup phase should not have failures")
	}
	if !result.Phases[1].HasFailures() {
		t.Error("github phase should have failures")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/repo/ -v -run TestCreate`

Expected: Compilation error — `Create`, `CreateOptions` not defined.

- [ ] **Step 3: Write Create pipeline**

Create `internal/repo/create.go`:

```go
package repo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/git"
)

type CreateOptions struct {
	Name          string
	Location      string
	PublishGitHub bool
	Visibility    string // "private" or "public"
	Description   string
}

type CreateResult struct {
	RepoPath     string
	WorktreePath string
	GitHubURL    string
	Phases       []Phase
}

func Create(runner git.CommandRunner, shell git.ShellRunner, opts CreateOptions, emit func(Event)) CreateResult {
	result := CreateResult{}
	repoPath := filepath.Join(opts.Location, opts.Name)
	result.RepoPath = repoPath

	setupPhase := runCreateSetup(runner, repoPath, opts, emit)
	result.Phases = append(result.Phases, setupPhase)
	if setupPhase.HasFailures() {
		return result
	}
	result.WorktreePath = filepath.Join(repoPath, "main")

	if opts.PublishGitHub {
		ghPhase := runCreateGitHub(runner, shell, repoPath, opts, emit)
		result.Phases = append(result.Phases, ghPhase)
		if !ghPhase.HasFailures() {
			// Extract GitHub URL from user lookup
			for _, step := range ghPhase.Steps {
				if step.Name == "Look up GitHub user" && step.Status == StepDone {
					result.GitHubURL = fmt.Sprintf("github.com/%s/%s", step.Message, opts.Name)
				}
			}
		}
	}

	return result
}

func runCreateSetup(runner git.CommandRunner, repoPath string, opts CreateOptions, emit func(Event)) Phase {
	phase := Phase{Name: "Setup"}
	phaseName := "Setup"

	// Create directory
	emit(Event{Phase: phaseName, Step: "Create directory", Status: StepRunning})
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		step := StepResult{Name: "Create directory", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}

	// Check directory is empty (abort if it already had content)
	entries, _ := os.ReadDir(repoPath)
	if len(entries) > 0 {
		err := fmt.Errorf("directory already exists and is not empty: %s", repoPath)
		step := StepResult{Name: "Create directory", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Create directory", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Create directory", Status: StepDone})

	// Init bare repo
	emit(Event{Phase: phaseName, Step: "Init bare repository", Status: StepRunning})
	barePath := filepath.Join(repoPath, ".bare")
	_, err := runner.Run(barePath, "init", "--bare")
	if err != nil {
		step := StepResult{Name: "Init bare repository", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Init bare repository", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Init bare repository", Status: StepDone})

	// Create .git pointer file
	emit(Event{Phase: phaseName, Step: "Create .git pointer", Status: StepRunning})
	gitPointerPath := filepath.Join(repoPath, ".git")
	if err := os.WriteFile(gitPointerPath, []byte("gitdir: .bare\n"), 0644); err != nil {
		step := StepResult{Name: "Create .git pointer", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Create .git pointer", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Create .git pointer", Status: StepDone})

	// Configure refspec
	emit(Event{Phase: phaseName, Step: "Configure refspec", Status: StepRunning})
	_, err = runner.Run(barePath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	if err != nil {
		step := StepResult{Name: "Configure refspec", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Configure refspec", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Configure refspec", Status: StepDone})

	// Create main worktree
	emit(Event{Phase: phaseName, Step: "Create main worktree", Status: StepRunning})
	_, err = runner.Run(repoPath, "worktree", "add", "main", "-b", "main")
	if err != nil {
		step := StepResult{Name: "Create main worktree", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Create main worktree", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Create main worktree", Status: StepDone})

	// Create README and initial commit
	emit(Event{Phase: phaseName, Step: "Initial commit", Status: StepRunning})
	mainPath := filepath.Join(repoPath, "main")
	readmePath := filepath.Join(mainPath, "README.md")
	if err := os.WriteFile(readmePath, []byte(fmt.Sprintf("# %s\n", opts.Name)), 0644); err != nil {
		step := StepResult{Name: "Initial commit", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	if _, err := runner.Run(mainPath, "add", "-A"); err != nil {
		step := StepResult{Name: "Initial commit", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	if _, err := runner.Run(mainPath, "commit", "-m", "Initial commit"); err != nil {
		step := StepResult{Name: "Initial commit", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Initial commit", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Initial commit", Status: StepDone})

	return phase
}

func runCreateGitHub(runner git.CommandRunner, shell git.ShellRunner, repoPath string, opts CreateOptions, emit func(Event)) Phase {
	phase := Phase{Name: "GitHub"}
	phaseName := "GitHub"

	// Look up GitHub user
	emit(Event{Phase: phaseName, Step: "Look up GitHub user", Status: StepRunning})
	ghUser, err := shell.RunShell(repoPath, "gh api user --jq .login")
	if err != nil {
		step := StepResult{Name: "Look up GitHub user", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Look up GitHub user", Status: StepDone, Message: ghUser})
	emit(Event{Phase: phaseName, Step: "Look up GitHub user", Status: StepDone, Message: ghUser})

	// Create GitHub repo
	emit(Event{Phase: phaseName, Step: "Create GitHub repository", Status: StepRunning})
	mainPath := filepath.Join(repoPath, "main")
	desc := opts.Description
	ghCmd := fmt.Sprintf("gh repo create %s --%s --description %q --source . --push", opts.Name, opts.Visibility, desc)
	_, err = shell.RunShell(mainPath, ghCmd)
	if err != nil {
		step := StepResult{Name: "Create GitHub repository", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Create GitHub repository", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Create GitHub repository", Status: StepDone})

	// Configure SSH remote
	emit(Event{Phase: phaseName, Step: "Configure SSH remote", Status: StepRunning})
	barePath := filepath.Join(repoPath, ".bare")
	sshURL := fmt.Sprintf("git@github.com:%s/%s.git", ghUser, opts.Name)
	_, err = runner.Run(barePath, "remote", "set-url", "origin", sshURL)
	if err != nil {
		step := StepResult{Name: "Configure SSH remote", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Configure SSH remote", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Configure SSH remote", Status: StepDone})

	// Push
	emit(Event{Phase: phaseName, Step: "Push to GitHub", Status: StepRunning})
	_, err = runner.Run(mainPath, "push", "-u", "origin", "main")
	if err != nil {
		step := StepResult{Name: "Push to GitHub", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Push to GitHub", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Push to GitHub", Status: StepDone})

	// Set remote HEAD
	emit(Event{Phase: phaseName, Step: "Set remote HEAD", Status: StepRunning})
	_, err = runner.Run(barePath, "remote", "set-head", "origin", "main")
	if err != nil {
		step := StepResult{Name: "Set remote HEAD", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Set remote HEAD", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Set remote HEAD", Status: StepDone})

	return phase
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/repo/ -v -run TestCreate`

Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo/create.go internal/repo/create_test.go
git commit -m "feat(repo): add create repo pipeline with setup and GitHub phases"
```

---

## Task 3: Clone repo pipeline

**Files:**
- Create: `internal/repo/clone.go`
- Create: `internal/repo/clone_test.go`

- [ ] **Step 1: Write tests for Clone pipeline and URL-to-name derivation**

Create `internal/repo/clone_test.go`:

```go
package repo

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestDeriveRepoName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{url: "git@github.com:user/repo.git", want: "repo"},
		{url: "https://github.com/user/repo.git", want: "repo"},
		{url: "https://github.com/user/repo", want: "repo"},
		{url: "git@gitlab.com:org/sub/project.git", want: "project"},
		{url: "https://github.com/user/my-repo.git", want: "my-repo"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := DeriveRepoName(tt.url)
			if got != tt.want {
				t.Errorf("DeriveRepoName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestClone_Successful(t *testing.T) {
	dir := t.TempDir()
	repoName := "repo"
	repoPath := filepath.Join(dir, repoName)
	barePath := filepath.Join(repoPath, ".bare")

	runner := &mockRunner{responses: map[string]mockResponse{
		// Clone phase
		fmt.Sprintf("%s:[clone --bare git@github.com:user/repo.git %s]", dir, barePath): {output: ""},
		// Structure phase
		fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath): {output: ""},
		// Worktree phase
		fmt.Sprintf("%s:[symbolic-ref refs/remotes/origin/HEAD]", barePath): {output: "refs/remotes/origin/main"},
		fmt.Sprintf("%s:[worktree add main main]", repoPath):               {output: ""},
		fmt.Sprintf("%s/main:[branch --set-upstream-to=origin/main]", repoPath): {output: ""},
	}}

	ec := &eventCollector{}
	opts := CloneOptions{
		URL:      "git@github.com:user/repo.git",
		Location: dir,
		Name:     repoName,
	}
	result := Clone(runner, opts, ec.emit)

	if result.RepoPath != repoPath {
		t.Errorf("RepoPath = %q, want %q", result.RepoPath, repoPath)
	}
	if result.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", result.DefaultBranch, "main")
	}
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
}

func TestClone_DefaultBranchFallback(t *testing.T) {
	dir := t.TempDir()
	repoName := "repo"
	repoPath := filepath.Join(dir, repoName)
	barePath := filepath.Join(repoPath, ".bare")

	runner := &mockRunner{responses: map[string]mockResponse{
		fmt.Sprintf("%s:[clone --bare git@github.com:user/repo.git %s]", dir, barePath): {output: ""},
		fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath): {output: ""},
		// symbolic-ref fails — fallback to main
		fmt.Sprintf("%s:[symbolic-ref refs/remotes/origin/HEAD]", barePath): {output: "", err: fmt.Errorf("not found")},
		fmt.Sprintf("%s:[show-ref --verify refs/heads/main]", barePath):    {output: "abc123"},
		fmt.Sprintf("%s:[worktree add main main]", repoPath):               {output: ""},
		fmt.Sprintf("%s/main:[branch --set-upstream-to=origin/main]", repoPath): {output: ""},
	}}

	ec := &eventCollector{}
	opts := CloneOptions{URL: "git@github.com:user/repo.git", Location: dir, Name: repoName}
	result := Clone(runner, opts, ec.emit)

	if result.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q (fallback)", result.DefaultBranch, "main")
	}
}

func TestClone_NetworkError(t *testing.T) {
	dir := t.TempDir()
	barePath := filepath.Join(dir, "repo", ".bare")

	runner := &mockRunner{responses: map[string]mockResponse{
		fmt.Sprintf("%s:[clone --bare git@github.com:user/repo.git %s]", dir, barePath): {
			output: "", err: fmt.Errorf("fatal: Could not read from remote repository"),
		},
	}}

	ec := &eventCollector{}
	opts := CloneOptions{URL: "git@github.com:user/repo.git", Location: dir, Name: "repo"}
	result := Clone(runner, opts, ec.emit)

	if len(result.Phases) == 0 || !result.Phases[0].HasFailures() {
		t.Error("expected clone phase to fail on network error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/repo/ -v -run "TestClone|TestDeriveRepoName"`

Expected: Compilation error — `Clone`, `CloneOptions`, `DeriveRepoName` not defined.

- [ ] **Step 3: Write Clone pipeline and DeriveRepoName**

Create `internal/repo/clone.go`:

```go
package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

type CloneOptions struct {
	URL      string
	Location string
	Name     string
}

type CloneResult struct {
	RepoPath      string
	WorktreePath  string
	DefaultBranch string
	OriginURL     string
	Phases        []Phase
}

// DeriveRepoName extracts a repository name from a git URL.
// "git@github.com:user/repo.git" → "repo"
// "https://github.com/user/repo.git" → "repo"
// "https://github.com/user/repo" → "repo"
func DeriveRepoName(url string) string {
	// Handle SSH-style URLs: git@host:path
	if idx := strings.LastIndex(url, ":"); idx != -1 && !strings.Contains(url, "://") {
		url = url[idx+1:]
	}

	// Take last path segment
	name := url
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	// Strip .git suffix
	name = strings.TrimSuffix(name, ".git")

	return name
}

func Clone(runner git.CommandRunner, opts CloneOptions, emit func(Event)) CloneResult {
	result := CloneResult{OriginURL: opts.URL}
	repoPath := filepath.Join(opts.Location, opts.Name)
	result.RepoPath = repoPath
	barePath := filepath.Join(repoPath, ".bare")

	// Phase 1: Clone
	clonePhase := runClonePhase(runner, opts.Location, opts.URL, barePath, emit)
	result.Phases = append(result.Phases, clonePhase)
	if clonePhase.HasFailures() {
		return result
	}

	// Phase 2: Structure
	structPhase := runCloneStructure(runner, repoPath, barePath, emit)
	result.Phases = append(result.Phases, structPhase)
	if structPhase.HasFailures() {
		return result
	}

	// Phase 3: Worktree
	wtPhase, branch := runCloneWorktree(runner, repoPath, barePath, emit)
	result.Phases = append(result.Phases, wtPhase)
	result.DefaultBranch = branch
	result.WorktreePath = filepath.Join(repoPath, branch)

	return result
}

func runClonePhase(runner git.CommandRunner, location, url, barePath string, emit func(Event)) Phase {
	phase := Phase{Name: "Clone"}
	phaseName := "Clone"

	emit(Event{Phase: phaseName, Step: "Clone bare repository", Status: StepRunning})
	_, err := runner.Run(location, "clone", "--bare", url, barePath)
	if err != nil {
		step := StepResult{Name: "Clone bare repository", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Clone bare repository", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Clone bare repository", Status: StepDone})

	return phase
}

func runCloneStructure(runner git.CommandRunner, repoPath, barePath string, emit func(Event)) Phase {
	phase := Phase{Name: "Structure"}
	phaseName := "Structure"

	// Create .git pointer
	emit(Event{Phase: phaseName, Step: "Create .git pointer", Status: StepRunning})
	gitPointerPath := filepath.Join(repoPath, ".git")
	if err := os.WriteFile(gitPointerPath, []byte("gitdir: .bare\n"), 0644); err != nil {
		step := StepResult{Name: "Create .git pointer", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Create .git pointer", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Create .git pointer", Status: StepDone})

	// Configure refspec
	emit(Event{Phase: phaseName, Step: "Configure refspec", Status: StepRunning})
	_, err := runner.Run(barePath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	if err != nil {
		step := StepResult{Name: "Configure refspec", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Configure refspec", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Configure refspec", Status: StepDone})

	return phase
}

func runCloneWorktree(runner git.CommandRunner, repoPath, barePath string, emit func(Event)) (Phase, string) {
	phase := Phase{Name: "Worktree"}
	phaseName := "Worktree"

	// Detect default branch
	emit(Event{Phase: phaseName, Step: "Detect default branch", Status: StepRunning})
	branch := detectDefaultBranch(runner, barePath)
	phase.Steps = append(phase.Steps, StepResult{
		Name: "Detect default branch", Status: StepDone, Message: branch,
	})
	emit(Event{Phase: phaseName, Step: "Detect default branch", Status: StepDone, Message: branch})

	// Create worktree
	emit(Event{Phase: phaseName, Step: "Create worktree", Status: StepRunning})
	_, err := runner.Run(repoPath, "worktree", "add", branch, branch)
	if err != nil {
		step := StepResult{Name: "Create worktree", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase, branch
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Create worktree", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Create worktree", Status: StepDone})

	// Set upstream
	emit(Event{Phase: phaseName, Step: "Set upstream tracking", Status: StepRunning})
	wtPath := filepath.Join(repoPath, branch)
	_, err = runner.Run(wtPath, "branch", fmt.Sprintf("--set-upstream-to=origin/%s", branch))
	if err != nil {
		step := StepResult{Name: "Set upstream tracking", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase, branch
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Set upstream tracking", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Set upstream tracking", Status: StepDone})

	return phase, branch
}

func detectDefaultBranch(runner git.CommandRunner, barePath string) string {
	output, err := runner.Run(barePath, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// "refs/remotes/origin/main" → "main"
		branch := strings.TrimPrefix(output, "refs/remotes/origin/")
		if branch != output && branch != "" {
			return branch
		}
	}

	// Fallback: try main, then master
	for _, candidate := range []string{"main", "master"} {
		_, err := runner.Run(barePath, "show-ref", "--verify", fmt.Sprintf("refs/heads/%s", candidate))
		if err == nil {
			return candidate
		}
	}

	return "main" // last resort
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/repo/ -v -run "TestClone|TestDeriveRepoName"`

Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo/clone.go internal/repo/clone_test.go
git commit -m "feat(repo): add clone repo pipeline with URL-to-name derivation and default branch detection"
```

---

## Task 4: Migrate repo pipeline

**Files:**
- Create: `internal/repo/migrate.go`
- Create: `internal/repo/migrate_test.go`

- [ ] **Step 1: Write tests for Migrate pipeline**

Create `internal/repo/migrate_test.go`:

```go
package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrate_Successful(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "my-project")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0755)
	barePath := filepath.Join(repoPath, ".bare")

	runner := &mockRunner{responses: map[string]mockResponse{
		// Validate
		fmt.Sprintf("%s:[status --porcelain]", repoPath):        {output: ""},
		fmt.Sprintf("%s:[branch --show-current]", repoPath):     {output: "main"},
		// Migrate
		fmt.Sprintf("%s:[clone --bare .git %s]", repoPath, barePath): {output: ""},
		fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath): {output: ""},
		fmt.Sprintf("%s:[worktree add main]", repoPath):         {output: ""},
	}}

	ec := &eventCollector{}
	opts := MigrateOptions{RepoPath: repoPath}
	result := Migrate(runner, runner, opts, ec.emit)

	if result.BareRoot != repoPath {
		t.Errorf("BareRoot = %q, want %q", result.BareRoot, repoPath)
	}
	if result.WorktreePath != filepath.Join(repoPath, "main") {
		t.Errorf("WorktreePath = %q, want %q", result.WorktreePath, filepath.Join(repoPath, "main"))
	}
	if result.BackupPath == "" {
		t.Error("expected BackupPath to be set")
	}
	if !strings.Contains(result.BackupPath, "_backup_") {
		t.Errorf("BackupPath should contain _backup_: %q", result.BackupPath)
	}

	// No phase failures
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
}

func TestMigrate_DirtyRepo_WarningContinues(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "dirty-project")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0755)
	barePath := filepath.Join(repoPath, ".bare")

	runner := &mockRunner{responses: map[string]mockResponse{
		fmt.Sprintf("%s:[status --porcelain]", repoPath):        {output: "M file.txt"},
		fmt.Sprintf("%s:[branch --show-current]", repoPath):     {output: "develop"},
		fmt.Sprintf("%s:[clone --bare .git %s]", repoPath, barePath): {output: ""},
		fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath): {output: ""},
		fmt.Sprintf("%s:[worktree add develop]", repoPath):      {output: ""},
	}}

	ec := &eventCollector{}
	opts := MigrateOptions{RepoPath: repoPath}
	result := Migrate(runner, runner, opts, ec.emit)

	// Should still succeed — dirty is a warning, not a failure
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}

	// Check that a warning event was emitted for dirty state
	foundWarning := false
	for _, e := range ec.events {
		if strings.Contains(e.Message, "uncommitted") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected warning event about uncommitted changes")
	}
}

func TestMigrate_CloneFailure_ShowsRollbackInfo(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "fail-project")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0755)
	barePath := filepath.Join(repoPath, ".bare")

	runner := &mockRunner{responses: map[string]mockResponse{
		fmt.Sprintf("%s:[status --porcelain]", repoPath):    {output: ""},
		fmt.Sprintf("%s:[branch --show-current]", repoPath): {output: "main"},
		fmt.Sprintf("%s:[clone --bare .git %s]", repoPath, barePath): {
			output: "", err: fmt.Errorf("fatal: failed to clone"),
		},
	}}

	ec := &eventCollector{}
	opts := MigrateOptions{RepoPath: repoPath}
	result := Migrate(runner, runner, opts, ec.emit)

	migratePhase := findPhase(result.Phases, "Migrate")
	if migratePhase == nil {
		t.Fatal("expected Migrate phase")
	}
	if !migratePhase.HasFailures() {
		t.Error("expected Migrate phase to have failures")
	}
	// Backup should still exist for rollback
	if result.BackupPath == "" {
		t.Error("backup path should be set even on migration failure")
	}
}

func TestDeleteBackup(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "file.txt"), []byte("data"), 0644)

	err := DeleteBackup(backupDir)
	if err != nil {
		t.Fatalf("DeleteBackup() error: %v", err)
	}
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Error("backup directory should be deleted")
	}
}

func findPhase(phases []Phase, name string) *Phase {
	for i := range phases {
		if phases[i].Name == name {
			return &phases[i]
		}
	}
	return nil
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/repo/ -v -run "TestMigrate|TestDeleteBackup"`

Expected: Compilation error — `Migrate`, `MigrateOptions`, `DeleteBackup` not defined.

- [ ] **Step 3: Write Migrate pipeline**

Create `internal/repo/migrate.go`:

```go
package repo

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
)

type MigrateOptions struct {
	RepoPath string
}

type MigrateResult struct {
	BareRoot     string
	WorktreePath string
	BackupPath   string
	BackupSize   string
	Branch       string
	IsDirty      bool
	Phases       []Phase
}

func Migrate(runner git.CommandRunner, shell git.ShellRunner, opts MigrateOptions, emit func(Event)) MigrateResult {
	result := MigrateResult{BareRoot: opts.RepoPath}

	// Phase 1: Validate
	validatePhase, branch, isDirty := runMigrateValidate(runner, opts.RepoPath, emit)
	result.Phases = append(result.Phases, validatePhase)
	result.Branch = branch
	result.IsDirty = isDirty
	if validatePhase.HasFailures() {
		return result
	}

	// Phase 2: Backup
	backupPhase, backupPath, backupSize := runMigrateBackup(shell, opts.RepoPath, emit)
	result.Phases = append(result.Phases, backupPhase)
	result.BackupPath = backupPath
	result.BackupSize = backupSize
	if backupPhase.HasFailures() {
		return result
	}

	// Phase 3: Migrate
	migratePhase := runMigrateBare(runner, opts.RepoPath, branch, emit)
	result.Phases = append(result.Phases, migratePhase)
	if migratePhase.HasFailures() {
		return result
	}
	result.WorktreePath = filepath.Join(opts.RepoPath, branch)

	// Phase 4: Copy (best-effort)
	copyPhase := runMigrateCopy(backupPath, result.WorktreePath, emit)
	result.Phases = append(result.Phases, copyPhase)

	return result
}

func runMigrateValidate(runner git.CommandRunner, repoPath string, emit func(Event)) (Phase, string, bool) {
	phase := Phase{Name: "Validate"}
	phaseName := "Validate"

	// Check for uncommitted changes
	emit(Event{Phase: phaseName, Step: "Check repository status", Status: StepRunning})
	statusOutput, err := runner.Run(repoPath, "status", "--porcelain")
	if err != nil {
		step := StepResult{Name: "Check repository status", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase, "", false
	}
	isDirty := strings.TrimSpace(statusOutput) != ""
	if isDirty {
		emit(Event{Phase: phaseName, Step: "Check repository status", Status: StepDone, Message: "uncommitted changes detected"})
	} else {
		emit(Event{Phase: phaseName, Step: "Check repository status", Status: StepDone, Message: "clean"})
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Check repository status", Status: StepDone})

	// Detect current branch
	emit(Event{Phase: phaseName, Step: "Detect current branch", Status: StepRunning})
	branch, err := runner.Run(repoPath, "branch", "--show-current")
	if err != nil {
		step := StepResult{Name: "Detect current branch", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase, "", isDirty
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Detect current branch", Status: StepDone, Message: branch})
	emit(Event{Phase: phaseName, Step: "Detect current branch", Status: StepDone, Message: branch})

	return phase, branch, isDirty
}

func runMigrateBackup(shell git.ShellRunner, repoPath string, emit func(Event)) (Phase, string, string) {
	phase := Phase{Name: "Backup"}
	phaseName := "Backup"

	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s_backup_%s", repoPath, timestamp)

	emit(Event{Phase: phaseName, Step: "Copy repository to backup", Status: StepRunning})
	parentDir := filepath.Dir(repoPath)
	cpCmd := fmt.Sprintf("cp -a %q %q", repoPath, backupPath)
	_, err := shell.RunShell(parentDir, cpCmd)
	if err != nil {
		step := StepResult{Name: "Copy repository to backup", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase, backupPath, ""
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Copy repository to backup", Status: StepDone, Message: backupPath})
	emit(Event{Phase: phaseName, Step: "Copy repository to backup", Status: StepDone, Message: backupPath})

	// Calculate backup size
	emit(Event{Phase: phaseName, Step: "Calculate backup size", Status: StepRunning})
	size := calculateDirSize(backupPath)
	phase.Steps = append(phase.Steps, StepResult{Name: "Calculate backup size", Status: StepDone, Message: size})
	emit(Event{Phase: phaseName, Step: "Calculate backup size", Status: StepDone, Message: size})

	return phase, backupPath, size
}

func runMigrateBare(runner git.CommandRunner, repoPath, branch string, emit func(Event)) Phase {
	phase := Phase{Name: "Migrate"}
	phaseName := "Migrate"
	barePath := filepath.Join(repoPath, ".bare")

	// Clone bare
	emit(Event{Phase: phaseName, Step: "Create bare repository", Status: StepRunning})
	_, err := runner.Run(repoPath, "clone", "--bare", ".git", barePath)
	if err != nil {
		step := StepResult{Name: "Create bare repository", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Create bare repository", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Create bare repository", Status: StepDone})

	// Remove original .git
	emit(Event{Phase: phaseName, Step: "Remove original .git", Status: StepRunning})
	gitDir := filepath.Join(repoPath, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		step := StepResult{Name: "Remove original .git", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Remove original .git", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Remove original .git", Status: StepDone})

	// Create .git pointer
	emit(Event{Phase: phaseName, Step: "Create .git pointer", Status: StepRunning})
	gitPointerPath := filepath.Join(repoPath, ".git")
	if err := os.WriteFile(gitPointerPath, []byte("gitdir: .bare\n"), 0644); err != nil {
		step := StepResult{Name: "Create .git pointer", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Create .git pointer", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Create .git pointer", Status: StepDone})

	// Configure refspec
	emit(Event{Phase: phaseName, Step: "Configure refspec", Status: StepRunning})
	_, err = runner.Run(barePath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	if err != nil {
		step := StepResult{Name: "Configure refspec", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Configure refspec", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Configure refspec", Status: StepDone})

	// Create worktree for current branch
	emit(Event{Phase: phaseName, Step: "Create worktree", Status: StepRunning})
	_, err = runner.Run(repoPath, "worktree", "add", branch)
	if err != nil {
		step := StepResult{Name: "Create worktree", Status: StepFailed, Error: err}
		phase.Steps = append(phase.Steps, step)
		emit(Event{Phase: phaseName, Step: step.Name, Status: StepFailed, Error: err})
		return phase
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Create worktree", Status: StepDone})
	emit(Event{Phase: phaseName, Step: "Create worktree", Status: StepDone})

	return phase
}

// copyPatterns defines files/directories to copy from backup to new worktree.
var copyPatterns = []string{
	".env*",
	"node_modules",
	"vendor",
	"build",
	"dist",
	".vscode",
	".idea",
}

func runMigrateCopy(backupPath, worktreePath string, emit func(Event)) Phase {
	phase := Phase{Name: "Copy"}
	phaseName := "Copy"

	emit(Event{Phase: phaseName, Step: "Copy development files", Status: StepRunning})
	copied := 0
	for _, pattern := range copyPatterns {
		matches, err := filepath.Glob(filepath.Join(backupPath, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			name := filepath.Base(match)
			dst := filepath.Join(worktreePath, name)
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			if info.IsDir() {
				if err := copyDir(match, dst); err != nil {
					emit(Event{Phase: phaseName, Step: "Copy development files", Status: StepRunning,
						Message: fmt.Sprintf("warning: could not copy %s: %v", name, err)})
					continue
				}
			} else {
				if err := copyFile(match, dst); err != nil {
					emit(Event{Phase: phaseName, Step: "Copy development files", Status: StepRunning,
						Message: fmt.Sprintf("warning: could not copy %s: %v", name, err)})
					continue
				}
			}
			copied++
		}
	}

	msg := fmt.Sprintf("%d items copied", copied)
	if copied == 0 {
		msg = "no development files found to copy"
	}
	phase.Steps = append(phase.Steps, StepResult{Name: "Copy development files", Status: StepDone, Message: msg})
	emit(Event{Phase: phaseName, Step: "Copy development files", Status: StepDone, Message: msg})

	return phase
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}
		return copyFile(path, dstPath)
	})
}

func calculateDirSize(path string) string {
	var totalSize int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
		return nil
	})
	return formatSize(totalSize)
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// DeleteBackup removes the backup directory.
func DeleteBackup(backupPath string) error {
	return os.RemoveAll(backupPath)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/repo/ -v -run "TestMigrate|TestDeleteBackup"`

Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo/migrate.go internal/repo/migrate_test.go
git commit -m "feat(repo): add migrate repo pipeline with backup, bare conversion, and file copy"
```

---

## Task 5: Menu adaptation for repo context

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/menu.go`

**Depends on:** Task 1 (RepoContext type)

- [ ] **Step 1: Add RepoContext field to Model and new view states**

Modify `internal/tui/model.go`:

Add new view states after `createSummaryView`:

```go
const (
	// ... existing states ...
	repoNameView
	repoOptionsView
	repoProgressView
	repoSummaryView
	cloneInputView
	migrateConfirmView
	migrateProgressView
	migrateSummaryView
)
```

Add `repoState` struct:

```go
type repoState struct {
	// Create repo fields
	nameInput     textinput.Model
	locationInput textinput.Model
	focusedField  int // 0 = name, 1 = location
	validationErr string

	// Options
	publishGitHub bool
	visibility    string // "private" or "public"
	descInput     textinput.Model
	ghStatus      string // "authenticated", "not authenticated", "gh not found"
	optionsCursor int

	// Clone fields
	urlInput      textinput.Model
	cloneNameInput textinput.Model
	cloneFocusedField int // 0 = url, 1 = name

	// Migrate fields
	migrateInfo   MigrateInfo
	backupCleanup string // "y", "n", or "" (pending)

	// Shared progress/summary
	eventCh  chan repo.Event
	resultCh chan interface{} // receives CreateResult, CloneResult, or MigrateResult
	events   []repo.Event
	result   interface{}
	opType   string // "create", "clone", "migrate"
}

type MigrateInfo struct {
	Branch  string
	IsDirty bool
}
```

Add `context` field to `Model`:

```go
type Model struct {
	// ... existing fields ...
	context repo.RepoContext
	repo    repoState
}
```

Update `NewMenuModel` to accept `context repo.RepoContext` parameter and build menu items accordingly.

- [ ] **Step 2: Update NewMenuModel to build context-aware menu items**

Modify `NewMenuModel` signature in `internal/tui/model.go`:

```go
func NewMenuModel(runner git.CommandRunner, repoPath string, cfg *config.Config, context repo.RepoContext) Model {
```

Build menu items based on context:

```go
var items []menuItem
switch context {
case repo.ContextBareRepo:
	items = []menuItem{
		{label: "Create new worktree", enabled: true},
		{label: "Remove worktrees", hint: "loading...", enabled: false},
		{label: "Cleanup", hint: "safe mode", enabled: true},
	}
case repo.ContextNoRepo:
	items = []menuItem{
		{label: "Create new repository", enabled: true},
		{label: "Clone repository as bare", enabled: true},
	}
case repo.ContextNonBareRepo:
	items = []menuItem{
		{label: "Migrate to bare repository", enabled: true},
		{label: "Clone repository as bare", enabled: true},
		{label: "Create new repository", enabled: true},
	}
}
```

- [ ] **Step 3: Update menu.go header rendering for non-bare contexts**

In `viewMenu()`, update the header to adapt based on context:

```go
switch m.context {
case repo.ContextBareRepo:
	b.WriteString(styleDim.Render(fmt.Sprintf("  %s (bare) %s %s", repoName, "\u00b7", m.repoPath)))
	// ... existing worktree stats ...
case repo.ContextNonBareRepo:
	b.WriteString(styleDim.Render(fmt.Sprintf("  %s %s %s", repoName, "\u00b7", m.repoPath)))
case repo.ContextNoRepo:
	b.WriteString(styleDim.Render(fmt.Sprintf("  %s", m.repoPath)))
}
```

- [ ] **Step 4: Update menu item selection in updateMenu**

In `updateMenu()`, replace the hardcoded `switch m.menuCursor` with a label-based dispatch:

```go
case key.Matches(msg, keys.Confirm):
	if m.menuCursor >= 0 && m.menuCursor < len(m.menuItems) && m.menuItems[m.menuCursor].enabled {
		label := m.menuItems[m.menuCursor].label
		switch label {
		case "Create new worktree":
			m.view = createBranchView
			return m, m.create.branchInput.Cursor.BlinkCmd()
		case "Remove worktrees":
			m.view = listView
			if len(m.remove.worktrees) == 0 {
				return m, loadWorktreeContext(m.runner, m.repoPath)
			}
		case "Cleanup":
			return m, tea.Quit
		case "Create new repository":
			m.view = repoNameView
			return m, m.repo.nameInput.Focus()
		case "Clone repository as bare":
			m.view = cloneInputView
			return m, m.repo.urlInput.Focus()
		case "Migrate to bare repository":
			m.view = migrateConfirmView
			return m, loadMigrateInfo(m.runner, m.repoPath)
		}
	}
```

- [ ] **Step 5: Update Init() to only load worktree context for bare repos**

In `model.go`, update `Init()`:

```go
func (m Model) Init() tea.Cmd {
	if m.view == menuView && m.context == repo.ContextBareRepo {
		return loadWorktreeContext(m.runner, m.repoPath)
	}
	return nil
}
```

- [ ] **Step 6: Add Update/View dispatch for new view states**

In `model.go`, add cases to `Update()` and `View()` switch statements for the 8 new view states. Each dispatches to the handler implemented in Tasks 6-9.

- [ ] **Step 7: Verify compilation**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build`

Expected: Builds successfully. (TUI view handlers will be stubs until Tasks 6-9.)

- [ ] **Step 8: Commit**

```bash
git add internal/tui/model.go internal/tui/menu.go
git commit -m "feat(tui): adapt menu based on repo context (bare/non-bare/no-repo)"
```

---

## Task 6: Create repo TUI views

**Files:**
- Create: `internal/tui/repo_name.go`
- Create: `internal/tui/repo_options.go`

**Depends on:** Task 2 (CreateOptions), Task 5 (repoState, view states)

- [ ] **Step 1: Write repo name input view**

Create `internal/tui/repo_name.go`:

Handles `repoNameView` — two text inputs (name, location) with tab switching.

Update handler: `updateRepoName(msg)`:
- `tea.KeyMsg` with `keys.Tab` → switch focus between name and location
- `tea.KeyMsg` with `keys.Confirm` → validate: name has no spaces, location exists, `location/name` doesn't exist. On valid, transition to `repoOptionsView`. On invalid, set `m.repo.validationErr`.
- `tea.KeyMsg` with `keys.Back` → return to `menuView`
- Forward other key msgs to focused text input

View: `viewRepoName()`:
```
  sentei ─ Create Repository

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  Repository name
  > my-project█

  Location
    /Users/dev/code/personal

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  enter continue · tab switch field · esc back
```

If `validationErr` is set, render it in `styleError` below the inputs.

- [ ] **Step 2: Write GitHub auth status check command**

Add a `tea.Cmd` function `checkGitHubAuth` that runs `gh auth status` via `ShellRunner` and returns a message:

```go
type ghAuthStatusMsg struct {
	status string // "authenticated", "not authenticated", "gh not found"
}

func checkGitHubAuth(shell git.ShellRunner) tea.Cmd {
	return func() tea.Msg {
		_, err := shell.RunShell(".", "gh auth status")
		if err != nil {
			if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "executable file not found") {
				return ghAuthStatusMsg{status: "gh not found"}
			}
			return ghAuthStatusMsg{status: "not authenticated"}
		}
		return ghAuthStatusMsg{status: "authenticated"}
	}
}
```

- [ ] **Step 3: Write repo options view with progressive GitHub disclosure**

Create `internal/tui/repo_options.go`:

Handles `repoOptionsView` — checkbox list with progressive disclosure.

Focus items (tracked by `m.repo.optionsCursor`):
- 0: Create initial worktree (always checked, display-only)
- 1: Publish to GitHub (toggle with space, disabled if gh not authenticated)
- 2: Visibility (only visible when Publish checked, cycles private/public on space)
- 3: Description (only visible when Publish checked, text input)

Update handler: `updateRepoOptions(msg)`:
- Handle `ghAuthStatusMsg` → set `m.repo.ghStatus`
- `keys.Toggle` (space) on Publish → toggle `m.repo.publishGitHub` (only if ghStatus is "authenticated")
- `keys.Toggle` on Visibility → cycle between "private" and "public"
- `keys.Down`/`keys.Up` → navigate options (skip hidden items)
- `keys.Confirm` → build `CreateOptions`, start pipeline (transition to `repoProgressView`)
- `keys.Back` → return to `repoNameView`
- Forward key msgs to description input when focused on it

View: `viewRepoOptions()`:
```
  sentei ─ Create Repository

  my-project · /Users/dev/code/personal/my-project

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  Setup

  [x] Create initial worktree              main

  GitHub                          authenticated ●

  [x] Publish to GitHub
        Visibility     private
        Description    █

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  space toggle · enter create · esc back
```

GitHub status rendering:
- "authenticated" → `styleSuccess.Render("authenticated ●")`
- "not authenticated" → `styleError.Render("not authenticated ✗")`
- "gh not found" → `styleError.Render("gh not found ✗")`

When Publish is unchecked, Visibility and Description lines are hidden entirely (progressive disclosure).

- [ ] **Step 4: Wire up transition from repoNameView to repoOptionsView**

In `updateRepoName`, on successful validation, transition to `repoOptionsView` and fire `checkGitHubAuth`:

```go
m.view = repoOptionsView
return m, checkGitHubAuth(shell)
```

Note: The `shell` runner needs to be accessible. Add a `shell git.ShellRunner` field to `Model` (set in `NewMenuModel`). This matches the existing pattern where `runner` is passed in.

- [ ] **Step 5: Verify compilation**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build`

Expected: Builds successfully.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/repo_name.go internal/tui/repo_options.go
git commit -m "feat(tui): add create repo name input and options views with GitHub disclosure"
```

---

## Task 7: Clone TUI view

**Files:**
- Create: `internal/tui/clone_input.go`

**Depends on:** Task 3 (CloneOptions, DeriveRepoName), Task 5 (view states)

- [ ] **Step 1: Write clone input view**

Create `internal/tui/clone_input.go`:

Handles `cloneInputView` — URL input with auto-derived name.

Two text inputs: URL and clone-to path. As user types URL, the derived name updates in real-time. Tab to override the name. The "Clone to" line shows `location/derivedName`.

Update handler: `updateCloneInput(msg)`:
- Forward key msgs to focused text input
- After each URL keypress, re-derive name via `repo.DeriveRepoName(url)` and update the display path (but only if user hasn't manually edited the name field)
- `keys.Tab` → switch focus between URL and name
- `keys.Confirm` → validate: URL not empty, destination doesn't exist. On valid, build `CloneOptions`, start pipeline (transition to `repoProgressView` with `opType = "clone"`).
- `keys.Back` → return to `menuView`

Track whether user has manually edited the name with a `nameManuallyEdited bool` field. Set to `true` on any keypress in the name field. When `true`, stop auto-deriving.

View: `viewCloneInput()`:
```
  sentei ─ Clone Repository

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  Repository URL
  > git@github.com:user/repo.git█

  Clone to
    /Users/dev/code/personal/repo

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  enter clone · tab switch field · esc back
```

- [ ] **Step 2: Verify compilation**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build`

- [ ] **Step 3: Commit**

```bash
git add internal/tui/clone_input.go
git commit -m "feat(tui): add clone URL input view with real-time name derivation"
```

---

## Task 8: Migrate TUI views

**Files:**
- Create: `internal/tui/migrate_confirm.go`
- Create: `internal/tui/migrate_summary.go`

**Depends on:** Task 4 (MigrateOptions, MigrateResult), Task 5 (view states)

- [ ] **Step 1: Write migrate info loader command**

Add to `internal/tui/migrate_confirm.go`:

```go
type migrateInfoMsg struct {
	branch  string
	isDirty bool
	err     error
}

func loadMigrateInfo(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		branch, err := runner.Run(repoPath, "branch", "--show-current")
		if err != nil {
			return migrateInfoMsg{err: err}
		}
		status, err := runner.Run(repoPath, "status", "--porcelain")
		if err != nil {
			return migrateInfoMsg{err: err}
		}
		isDirty := strings.TrimSpace(status) != ""
		return migrateInfoMsg{branch: branch, isDirty: isDirty}
	}
}
```

- [ ] **Step 2: Write migrate confirmation view**

Handles `migrateConfirmView`:

Update handler: `updateMigrateConfirm(msg)`:
- Handle `migrateInfoMsg` → store branch and dirty status in `m.repo.migrateInfo`
- `keys.Confirm` → start migrate pipeline (transition to `migrateProgressView` which reuses `repoProgressView` with `opType = "migrate"`)
- `keys.Back` → return to `menuView`

View: `viewMigrateConfirm()`:
```
  sentei ─ Migrate to Bare Repository

  /Users/dev/code/personal/old-project

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  Current branch    main
  Status            clean

  This will:
    ● Back up current repo
    ● Convert to bare repository structure
    ● Create worktree for main

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  enter migrate · esc back
```

When `isDirty`:
```
  Status            ⚠ uncommitted changes

  ⚠ Uncommitted changes will be preserved in the backup
    but not in the new worktree
```

- [ ] **Step 3: Write migrate summary view with backup cleanup**

Create `internal/tui/migrate_summary.go`:

Handles `migrateSummaryView`:

Update handler: `updateMigrateSummary(msg)`:
- `keys.Yes` → delete backup (`repo.DeleteBackup`), then re-launch sentei
- `keys.No` → keep backup, re-launch sentei
- `keys.Quit` → `tea.Quit`

View: `viewMigrateSummary()`:
```
  sentei ─ Migration Complete

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  ● old-project migrated

    Path     /Users/dev/code/personal/old-project
    Branch   main
    Backup   /Users/dev/code/personal/old-project_backup_20260330_142500

  Delete backup? (saves 245 MB)

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  y delete backup · n keep and open in sentei · q quit
```

If migration failed, show rollback instructions instead:
```
  ✗ Migration failed: <error>

  Your original repo is backed up at:
    /path/to/backup

  To restore: rm -rf /path/to/repo && mv /path/to/backup /path/to/repo
```

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build`

- [ ] **Step 5: Commit**

```bash
git add internal/tui/migrate_confirm.go internal/tui/migrate_summary.go
git commit -m "feat(tui): add migrate confirmation and summary views with backup cleanup"
```

---

## Task 9: Shared progress and summary views with re-launch

**Files:**
- Create: `internal/tui/repo_progress.go`
- Create: `internal/tui/repo_summary.go`

**Depends on:** Task 1 (Event, Phase types), Task 5 (repoState, view states)

- [ ] **Step 1: Write shared repo progress view**

Create `internal/tui/repo_progress.go`:

Handles `repoProgressView` and `migrateProgressView` — displays phased progress using the same visual language as SP2's create worktree progress (indicators: `●` done, `◐` active, `·` pending, `✗` failed).

The view receives events from `m.repo.eventCh` and aggregates them into phases.

Add pipeline launch commands for each operation type:

```go
type repoEventMsg repo.Event
type repoDoneMsg struct {
	result interface{} // CreateResult, CloneResult, or MigrateResult
}

func runCreatePipeline(runner git.CommandRunner, shell git.ShellRunner, opts repo.CreateOptions) tea.Cmd {
	return func() tea.Msg {
		// Events streamed via channel, result returned as msg
		// ... (channel-based event streaming matching creator pattern)
	}
}
```

Use channel-based event streaming:
- Pipeline runs in a goroutine, emitting events to `m.repo.eventCh`
- `tea.Cmd` listens on `eventCh`, returns each event as a `repoEventMsg`
- When pipeline completes, return `repoDoneMsg` with the result
- A `waitForRepoEvent` cmd re-subscribes for the next event

Update handler: `updateRepoProgress(msg)`:
- Handle `repoEventMsg` → append to `m.repo.events`, return `waitForRepoEvent`
- Handle `repoDoneMsg` → store result, transition to summary view (`repoSummaryView` or `migrateSummaryView` based on `m.repo.opType`)

View: `viewRepoProgress()`:
```
  sentei ─ Creating Repository

  my-project

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  ● Setup
    ● Create directory
    ● Init bare repository
    ● Create .git pointer
    ◐ Configure refspec
    · Create main worktree
    · Initial commit

  · GitHub
```

Phase header uses `stylePhaseDone`/`stylePhaseActive`/`stylePhasePending`. Step indicators use `styleIndicatorDone`/`styleIndicatorActive`/`styleIndicatorPending`/`styleIndicatorFailed`.

- [ ] **Step 2: Write shared repo summary view with re-launch**

Create `internal/tui/repo_summary.go`:

Handles `repoSummaryView` — displays result for create and clone operations.

Update handler: `updateRepoSummary(msg)`:
- `keys.Confirm` → re-launch sentei at new repo path:
  ```go
  senteiPath, err := os.Executable()
  if err != nil {
      senteiPath = "sentei"
  }
  return m, tea.ExecProcess(senteiPath, []string{newRepoPath}, tea.WithEnv(os.Environ()))
  ```
- `keys.Quit` → `tea.Quit`

View: `viewRepoSummary()`:
For create:
```
  sentei ─ Repository Created

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  ● my-project ready

    Path     /Users/dev/code/personal/my-project
    Branch   main
    GitHub   github.com/bonboncinnabon/my-project ●

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

    cd /Users/dev/code/personal/my-project/main

  enter open in sentei · q quit
```

For clone:
```
  sentei ─ Repository Cloned

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

  ● repo ready

    Path     /Users/dev/code/personal/repo
    Branch   main
    Origin   git@github.com:user/repo.git

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄

    cd /Users/dev/code/personal/repo/main

  enter open in sentei · q quit
```

If the operation had failures (e.g., GitHub phase failed but local succeeded), show the error but still offer re-launch:
```
  ● my-project ready (local only)

    Path     /Users/dev/code/personal/my-project
    Branch   main
    GitHub   ✗ failed to publish

  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
```

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build`

- [ ] **Step 4: Commit**

```bash
git add internal/tui/repo_progress.go internal/tui/repo_summary.go
git commit -m "feat(tui): add shared progress and summary views with sentei re-launch"
```

---

## Task 10: Main.go update

**Files:**
- Modify: `main.go`

**Depends on:** Task 1 (DetectContext), Task 5 (NewMenuModel signature change)

- [ ] **Step 1: Add context detection before TUI launch**

In `main.go`, replace the `ValidateRepository` call with `DetectContext`:

```go
// Detect repo context
context := repo.DetectContext(runner, repoPath)

// Dry-run mode only works in bare repos
if *dryRunFlag {
	if context != repo.ContextBareRepo {
		fmt.Fprintf(os.Stderr, "Error: --dry-run requires a bare repository\n")
		os.Exit(1)
	}
	// ... existing dry-run logic ...
}

// Load config only for bare repos (config lives in repo)
var cfg *config.Config
if context == repo.ContextBareRepo {
	cfg, err = config.LoadConfig(repoPath,
		config.WithRunner(runner),
		config.WithKnownIntegrations(integration.Names()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
	}
}
```

- [ ] **Step 2: Pass context to NewMenuModel**

Update the `NewMenuModel` call:

```go
model := tui.NewMenuModel(tuiRunner, repoPath, cfg, context)
```

- [ ] **Step 3: Add ShellRunner to Model**

The TUI needs a `ShellRunner` for GitHub auth checks and repo operations. Pass `&git.DefaultShellRunner{}` to `NewMenuModel`:

```go
func NewMenuModel(runner git.CommandRunner, shell git.ShellRunner, repoPath string, cfg *config.Config, context repo.RepoContext) Model {
```

In `main.go`:

```go
shell := &git.DefaultShellRunner{}
model := tui.NewMenuModel(tuiRunner, shell, repoPath, cfg, context)
```

- [ ] **Step 4: Add repo import**

Add `"github.com/bonboncinnabon/sentei/internal/repo"` to `main.go` imports.

- [ ] **Step 5: Verify build and basic startup**

Run:
```bash
cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build
# Test in a non-repo directory
cd /tmp && /path/to/sentei  # Should show create/clone menu
# Test in a regular repo
cd /path/to/regular-repo && /path/to/sentei  # Should show migrate/clone/create menu
```

- [ ] **Step 6: Commit**

```bash
git add main.go internal/tui/model.go
git commit -m "feat: wire context detection into main.go and pass to TUI"
```

---

## Task 11: E2E tests and final verification

**Files:**
- Create: `internal/repo/e2e_test.go`

**Depends on:** All previous tasks

- [ ] **Step 1: Write E2E test for create pipeline**

Create `internal/repo/e2e_test.go` with build tag `//go:build e2e`:

```go
//go:build e2e

package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestE2E_CreateRepo(t *testing.T) {
	dir := t.TempDir()
	runner := &git.GitRunner{}
	shell := &git.DefaultShellRunner{}

	ec := &eventCollector{}
	opts := CreateOptions{
		Name:          "test-repo",
		Location:      dir,
		PublishGitHub: false,
	}
	result := Create(runner, shell, opts, ec.emit)

	repoPath := filepath.Join(dir, "test-repo")

	// Verify bare structure
	if _, err := os.Stat(filepath.Join(repoPath, ".bare")); os.IsNotExist(err) {
		t.Error(".bare directory should exist")
	}

	// Verify .git pointer
	content, err := os.ReadFile(filepath.Join(repoPath, ".git"))
	if err != nil {
		t.Fatalf("reading .git pointer: %v", err)
	}
	if string(content) != "gitdir: .bare\n" {
		t.Errorf(".git pointer content = %q, want %q", string(content), "gitdir: .bare\n")
	}

	// Verify main worktree exists
	if _, err := os.Stat(filepath.Join(repoPath, "main")); os.IsNotExist(err) {
		t.Error("main worktree should exist")
	}

	// Verify README
	if _, err := os.Stat(filepath.Join(repoPath, "main", "README.md")); os.IsNotExist(err) {
		t.Error("README.md should exist in main worktree")
	}

	// No failures
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
}

func TestE2E_CloneRepo(t *testing.T) {
	// Create a source repo to clone from
	sourceDir := t.TempDir()
	runner := &git.GitRunner{}
	shell := &git.DefaultShellRunner{}

	// Set up a minimal git repo as origin
	runner.Run(sourceDir, "init")
	runner.Run(sourceDir, "checkout", "-b", "main")
	os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("hello"), 0644)
	runner.Run(sourceDir, "add", "-A")
	runner.Run(sourceDir, "commit", "-m", "init")

	// Clone it
	cloneDir := t.TempDir()
	ec := &eventCollector{}
	opts := CloneOptions{
		URL:      sourceDir, // local path works as URL for git clone
		Location: cloneDir,
		Name:     "cloned",
	}
	result := Clone(runner, opts, ec.emit)

	repoPath := filepath.Join(cloneDir, "cloned")

	// Verify bare structure
	if _, err := os.Stat(filepath.Join(repoPath, ".bare")); os.IsNotExist(err) {
		t.Error(".bare directory should exist")
	}

	// Verify .git pointer
	content, err := os.ReadFile(filepath.Join(repoPath, ".git"))
	if err != nil {
		t.Fatalf("reading .git pointer: %v", err)
	}
	if string(content) != "gitdir: .bare\n" {
		t.Errorf(".git pointer content = %q", string(content))
	}

	// Verify worktree for default branch
	if result.DefaultBranch == "" {
		t.Error("DefaultBranch should be detected")
	}
	wtPath := filepath.Join(repoPath, result.DefaultBranch)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree %q should exist", wtPath)
	}
}

func TestE2E_MigrateRepo(t *testing.T) {
	// Create a regular git repo
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "to-migrate")
	os.MkdirAll(repoPath, 0755)

	runner := &git.GitRunner{}
	shell := &git.DefaultShellRunner{}

	runner.Run(repoPath, "init")
	runner.Run(repoPath, "checkout", "-b", "main")
	os.WriteFile(filepath.Join(repoPath, "file.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(repoPath, ".env"), []byte("SECRET=val"), 0644)
	runner.Run(repoPath, "add", "file.txt")
	runner.Run(repoPath, "commit", "-m", "init")

	ec := &eventCollector{}
	opts := MigrateOptions{RepoPath: repoPath}
	result := Migrate(runner, shell, opts, ec.emit)

	// Verify bare structure
	if _, err := os.Stat(filepath.Join(repoPath, ".bare")); os.IsNotExist(err) {
		t.Error(".bare directory should exist")
	}

	// Verify .git is now a pointer file (not a directory)
	info, err := os.Stat(filepath.Join(repoPath, ".git"))
	if err != nil {
		t.Fatalf("stat .git: %v", err)
	}
	if info.IsDir() {
		t.Error(".git should be a file (pointer), not a directory")
	}

	// Verify backup exists
	if result.BackupPath == "" {
		t.Error("BackupPath should be set")
	}
	if _, err := os.Stat(result.BackupPath); os.IsNotExist(err) {
		t.Error("backup directory should exist")
	}

	// Verify .env was copied to new worktree
	wtEnvPath := filepath.Join(result.WorktreePath, ".env")
	if _, err := os.Stat(wtEnvPath); os.IsNotExist(err) {
		t.Error(".env should be copied to new worktree")
	}

	// Test backup cleanup
	err = DeleteBackup(result.BackupPath)
	if err != nil {
		t.Fatalf("DeleteBackup() error: %v", err)
	}
	if _, err := os.Stat(result.BackupPath); !os.IsNotExist(err) {
		t.Error("backup should be deleted after cleanup")
	}
}
```

- [ ] **Step 2: Run E2E tests**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./internal/repo/ -v -tags e2e -run TestE2E`

Expected: All 3 E2E tests PASS.

- [ ] **Step 3: Run full unit test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go test ./...`

Expected: All tests PASS.

- [ ] **Step 4: Run build and vet**

Run:
```bash
cd /Users/abiswas/code/personal/sentei/feat/config-ecosystem && go build && go vet ./...
```

Expected: Clean build, no vet warnings.

- [ ] **Step 5: Manual smoke test**

Run sentei in three contexts and verify correct menu appears:

```bash
# No repo context
cd /tmp && ./sentei
# Expected: Create new repository / Clone repository as bare

# Non-bare repo
cd /path/to/regular-repo && ./sentei
# Expected: Migrate to bare repository / Clone repository as bare / Create new repository

# Bare repo (existing behavior)
cd /path/to/bare-repo && ./sentei
# Expected: Create new worktree / Remove worktrees / Cleanup
```

- [ ] **Step 6: Commit**

```bash
git add internal/repo/e2e_test.go
git commit -m "test(repo): add E2E tests for create, clone, and migrate pipelines"
```
