# Integration Selection & Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add repo-level integration management to sentei — a view to see/toggle integrations, migration onboarding with integration detection, ccc copy optimization, and simplified create-worktree flow.

**Architecture:** Integrations are enabled per bare repo (stored in `.bare/sentei.json`), not per branch. Main's worktree is the source of truth. New worktrees inherit active integrations. A new `internal/state` package handles JSON persistence. A new `internal/integration/manager.go` handles enable/disable across worktrees. Three new TUI views: integration list, integration progress, and migration integration selection.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, encoding/json

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `internal/state/state.go` | Read/write `.bare/sentei.json` |
| Create | `internal/state/state_test.go` | Unit tests for state persistence |
| Create | `internal/integration/detect.go` | Scan worktree dirs for integration artifacts |
| Create | `internal/integration/detect_test.go` | Unit tests for detection |
| Create | `internal/fileutil/copy.go` | Shared `CopyDir` helper for cross-package use |
| Create | `internal/fileutil/copy_test.go` | Unit tests for CopyDir |
| Create | `internal/integration/manager.go` | Enable/disable integrations across worktrees |
| Create | `internal/integration/manager_test.go` | Unit tests for manager |
| Create | `internal/tui/integration_list.go` | Management list view (update + view + info dialog) |
| Create | `internal/tui/integration_progress.go` | Apply changes progress view |
| Create | `internal/tui/migrate_integrations.go` | Migration onboarding integration selection |
| Modify | `internal/tui/model.go` | Add view states, integration state struct |
| Modify | `internal/tui/menu.go` | Add "Manage integrations" menu item |
| Modify | `internal/tui/keys.go` | Add Left/Right bindings, Info binding |
| Modify | `internal/tui/styles.go` | Add staged-add/staged-remove styles |
| Modify | `internal/tui/migrate_summary.go` | Update what-next text, wire integration step |
| Modify | `internal/tui/create_options.go` | Remove integration toggles, add info line |
| Modify | `internal/tui/create_branch.go` | Use state for integration list instead of config toggles |
| Modify | `internal/creator/integrations.go` | Add ccc copy optimization |
| Modify | `internal/creator/integrations_test.go` | Tests for index copy |
| Modify | `internal/integration/integration.go` | Add IndexCopyDir field |
| Modify | `internal/integration/ccc.go` | Set IndexCopyDir for ccc |

---

### Task 1: State Persistence

**Files:**
- Create: `internal/state/state.go`
- Create: `internal/state/state_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/state/state_test.go
package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FileNotExist_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(filepath.Join(dir, ".bare"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Integrations) != 0 {
		t.Errorf("integrations = %v, want empty", s.Integrations)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, ".bare")
	os.MkdirAll(bare, 0755)
	os.WriteFile(filepath.Join(bare, "sentei.json"), []byte(`{"integrations":["code-review-graph"]}`), 0644)

	s, err := Load(bare)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Integrations) != 1 || s.Integrations[0] != "code-review-graph" {
		t.Errorf("integrations = %v, want [code-review-graph]", s.Integrations)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, ".bare")
	os.MkdirAll(bare, 0755)
	os.WriteFile(filepath.Join(bare, "sentei.json"), []byte(`{broken`), 0644)

	_, err := Load(bare)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSave_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, ".bare")
	os.MkdirAll(bare, 0755)

	s := &State{Integrations: []string{"code-review-graph", "cocoindex-code"}}
	if err := Save(bare, s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := Load(bare)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Integrations) != 2 {
		t.Errorf("integrations count = %d, want 2", len(loaded.Integrations))
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, ".bare")
	os.MkdirAll(bare, 0755)

	Save(bare, &State{Integrations: []string{"code-review-graph"}})
	Save(bare, &State{Integrations: []string{"cocoindex-code"}})

	loaded, _ := Load(bare)
	if len(loaded.Integrations) != 1 || loaded.Integrations[0] != "cocoindex-code" {
		t.Errorf("integrations = %v, want [cocoindex-code]", loaded.Integrations)
	}
}

func TestHasIntegration(t *testing.T) {
	s := &State{Integrations: []string{"code-review-graph"}}
	if !s.HasIntegration("code-review-graph") {
		t.Error("expected true for code-review-graph")
	}
	if s.HasIntegration("cocoindex-code") {
		t.Error("expected false for cocoindex-code")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./internal/state/ -v`
Expected: Compilation error — package does not exist yet.

- [ ] **Step 3: Write the implementation**

```go
// internal/state/state.go
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const stateFile = "sentei.json"

// State holds persistent sentei state for a bare repository.
type State struct {
	Integrations []string `json:"integrations"`
}

// HasIntegration reports whether the named integration is enabled.
func (s *State) HasIntegration(name string) bool {
	for _, n := range s.Integrations {
		if n == name {
			return true
		}
	}
	return false
}

// Load reads the state from bareDir/sentei.json. Returns an empty state if the
// file does not exist.
func Load(bareDir string) (*State, error) {
	data, err := os.ReadFile(filepath.Join(bareDir, stateFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{}, nil
		}
		return nil, fmt.Errorf("reading state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	return &s, nil
}

// Save writes the state to bareDir/sentei.json atomically (write to temp, then rename).
func Save(bareDir string, s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding state: %w", err)
	}
	data = append(data, '\n')

	target := filepath.Join(bareDir, stateFile)
	tmp, err := os.CreateTemp(bareDir, ".sentei-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming state file: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./internal/state/ -v`
Expected: All 6 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat: add state persistence for bare repo integration config"
```

---

### Task 1b: Add IndexCopyDir to Integration Struct

**Files:**
- Modify: `internal/integration/integration.go`
- Modify: `internal/integration/ccc.go`

The ccc copy optimization needs to know which directory to copy between worktrees. Rather than hardcoding `"cocoindex-code"` name checks, add a config-driven field to the `Integration` struct.

- [ ] **Step 1: Add IndexCopyDir field to Integration struct**

In `internal/integration/integration.go`, add to the `Integration` struct:

```go
// IndexCopyDir is the directory name (relative to worktree root) that can be
// copied from one worktree to another to seed an incremental index. Empty means
// the integration's index cannot be shared across worktrees (e.g., absolute paths).
IndexCopyDir string
```

- [ ] **Step 2: Set IndexCopyDir for ccc**

In `internal/integration/ccc.go`, add to the `cocoindexCode()` return:

```go
IndexCopyDir: ".cocoindex_code",
```

crg's `codeReviewGraph()` gets no `IndexCopyDir` (empty string = not copyable).

- [ ] **Step 3: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/integration/integration.go internal/integration/ccc.go
git commit -m "feat: add IndexCopyDir field for config-driven index copy optimization"
```

---

### Task 2: Integration Detection

**Files:**
- Create: `internal/integration/detect.go`
- Create: `internal/integration/detect_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/integration/detect_test.go
package integration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPresent_DirExists(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".code-review-graph"), 0755)

	integ := codeReviewGraph()
	if !DetectPresent(dir, integ) {
		t.Error("expected true when artifact dir exists")
	}
}

func TestDetectPresent_DirMissing(t *testing.T) {
	dir := t.TempDir()

	integ := codeReviewGraph()
	if DetectPresent(dir, integ) {
		t.Error("expected false when artifact dir missing")
	}
}

func TestDetectPresent_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".cocoindex_code"), 0755)

	integ := cocoindexCode()
	if !DetectPresent(dir, integ) {
		t.Error("expected true when at least one artifact dir exists")
	}
}

func TestDetectAllPresent(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".code-review-graph"), 0755)

	result := DetectAllPresent(dir, All())
	if !result["code-review-graph"] {
		t.Error("expected code-review-graph to be detected")
	}
	if result["cocoindex-code"] {
		t.Error("expected cocoindex-code to not be detected")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./internal/integration/ -run TestDetect -v`
Expected: Compilation error — `DetectPresent` and `DetectAllPresent` not defined.

- [ ] **Step 3: Write the implementation**

```go
// internal/integration/detect.go
package integration

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectPresent checks whether an integration's artifacts exist in the given directory.
// It checks for the presence of any directory listed in GitignoreEntries.
func DetectPresent(dir string, integ Integration) bool {
	for _, entry := range integ.GitignoreEntries {
		name := strings.TrimSuffix(entry, "/")
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// DetectAllPresent checks all integrations for artifact presence in the given
// directory. Returns a map of integration name → present.
func DetectAllPresent(dir string, integrations []Integration) map[string]bool {
	result := make(map[string]bool, len(integrations))
	for _, integ := range integrations {
		result[integ.Name] = DetectPresent(dir, integ)
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./internal/integration/ -run TestDetect -v`
Expected: All 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/integration/detect.go internal/integration/detect_test.go
git commit -m "feat: add on-disk integration artifact detection"
```

---

### Task 3: Key Bindings and Styles

**Files:**
- Modify: `internal/tui/keys.go`
- Modify: `internal/tui/styles.go`

- [ ] **Step 1: Add Left, Right, and Info key bindings to keys.go**

Add to the `keyMap` struct:

```go
Left  key.Binding
Right key.Binding
Info  key.Binding
```

Add to the `keys` var:

```go
Left: key.NewBinding(
    key.WithKeys("left", "a"),
    key.WithHelp("a/left", "left"),
),
Right: key.NewBinding(
    key.WithKeys("right", "d"),
    key.WithHelp("d/right", "right"),
),
Info: key.NewBinding(
    key.WithKeys("?"),
    key.WithHelp("?", "info"),
),
```

Also update `Up` and `Down` to include `w` and `s`:

```go
Up: key.NewBinding(
    key.WithKeys("up", "k", "w"),
    key.WithHelp("w/up", "up"),
),
Down: key.NewBinding(
    key.WithKeys("down", "j", "s"),
    key.WithHelp("s/down", "down"),
),
```

**Important**: Adding `w` and `s` to Up/Down globally will conflict with views that use text input (branch name, filter, repo name, etc.). These bindings must be scoped — only active in views that don't have text inputs. The update handlers for text-input views already forward keys to the text input first and only check navigation bindings in specific switch cases, so `w`/`s` keystrokes will be captured by the text input before reaching the navigation cases. However, in the **list view**, `s` is currently bound to `Sort`. This creates a conflict.

**Resolution**: Do NOT add `w`/`s` to the global Up/Down bindings. Instead, create separate bindings for the integration views only:

```go
// Navigation bindings for integration views (no text inputs, no sort conflicts)
IntUp: key.NewBinding(
    key.WithKeys("up", "w"),
    key.WithHelp("w/up", "up"),
),
IntDown: key.NewBinding(
    key.WithKeys("down", "s"),
    key.WithHelp("s/down", "down"),
),
```

- [ ] **Step 2: Add staged styles to styles.go**

Add after the existing checkbox styles:

```go
styleStagedAdd = lipgloss.NewStyle().
    Foreground(lipgloss.Color("42")) // green — same as clean/success

styleStagedRemove = lipgloss.NewStyle().
    Foreground(lipgloss.Color("214")) // orange — same as dirty/warning
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/keys.go internal/tui/styles.go
git commit -m "feat: add navigation keys and staged styles for integration views"
```

---

### Task 4: Integration Manager

**Files:**
- Create: `internal/integration/manager.go`
- Create: `internal/integration/manager_test.go`

This is the business logic for enabling/disabling integrations across all worktrees.

- [ ] **Step 1: Write the failing tests**

```go
// internal/integration/manager_test.go
package integration

import (
	"fmt"
	"strings"
	"testing"
)

type managerMockShell struct {
	responses map[string]mockShellResponse
	calls     []string
}

type mockShellResponse struct {
	output string
	err    error
}

func (m *managerMockShell) RunShell(dir string, command string) (string, error) {
	key := fmt.Sprintf("%s:shell[%s]", dir, command)
	m.calls = append(m.calls, key)
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	return "", fmt.Errorf("unexpected shell call: %s", key)
}

func TestEnableIntegration_RunsSetupOnEachWorktree(t *testing.T) {
	shell := &managerMockShell{responses: map[string]mockShellResponse{
		"/repo/main:shell[code-review-graph --version]":          {output: "1.0"},
		"/repo:shell[code-review-graph build --repo /repo/main]": {output: "built"},
		"/repo/feat:shell[code-review-graph --version]":          {output: "1.0"},
		"/repo:shell[code-review-graph build --repo /repo/feat]": {output: "built"},
	}}

	integ := codeReviewGraph()
	wtPaths := []string{"/repo/main", "/repo/feat"}
	var events []ManagerEvent

	EnableIntegration(shell, "/repo", "/repo/main", wtPaths, integ, func(e ManagerEvent) {
		events = append(events, e)
	})

	if len(events) == 0 {
		t.Fatal("expected events to be emitted")
	}

	// Assert exact event sequence: Setup for main, then Setup for feat
	type wantEvent struct {
		worktree string
		step     string
		status   ManagerStatus
	}
	want := []wantEvent{
		{"/repo/main", "Setup code-review-graph", StatusRunning},
		{"/repo/main", "Setup code-review-graph", StatusDone},
		{"/repo/feat", "Setup code-review-graph", StatusRunning},
		{"/repo/feat", "Setup code-review-graph", StatusDone},
	}
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d\nevents: %+v", len(events), len(want), events)
	}
	for i, w := range want {
		got := events[i]
		if got.Worktree != w.worktree || got.Step != w.step || got.Status != w.status {
			t.Errorf("event[%d] = {%s, %s, %d}, want {%s, %s, %d}",
				i, got.Worktree, got.Step, got.Status, w.worktree, w.step, w.status)
		}
	}
}

func TestDisableIntegration_RemovesArtifacts(t *testing.T) {
	shell := &managerMockShell{responses: map[string]mockShellResponse{
		"/repo/main:shell[ccc reset --all --force]": {output: ""},
		"/repo/feat:shell[ccc reset --all --force]": {output: ""},
	}}

	integ := cocoindexCode()
	wtPaths := []string{"/repo/main", "/repo/feat"}
	var events []ManagerEvent

	DisableIntegration(shell, wtPaths, integ, func(e ManagerEvent) {
		events = append(events, e)
	})

	if len(events) == 0 {
		t.Fatal("expected events to be emitted")
	}

	var teardownSteps int
	for _, e := range events {
		if strings.Contains(e.Step, "Teardown") || strings.Contains(e.Step, "Remove") {
			teardownSteps++
		}
	}
	if teardownSteps < 2 {
		t.Errorf("teardown steps = %d, want at least 2", teardownSteps)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./internal/integration/ -run TestEnable -v && go test ./internal/integration/ -run TestDisable -v`
Expected: Compilation error — `EnableIntegration`, `DisableIntegration`, `ManagerEvent` not defined.

- [ ] **Step 3: Write the implementation**

```go
// internal/integration/manager.go
package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// ManagerEvent is emitted during enable/disable operations.
type ManagerEvent struct {
	Worktree string
	Step     string
	Status   ManagerStatus
	Error    error
}

type ManagerStatus int

const (
	StatusRunning ManagerStatus = iota
	StatusDone
	StatusFailed
)

// EnableIntegration sets up an integration on all given worktrees.
// It runs detect → deps → install → setup → gitignore for each.
func EnableIntegration(shell git.ShellRunner, repoPath string, wtPaths []string, integ Integration, emit func(ManagerEvent)) {
	for _, wtPath := range wtPaths {
		enableOnWorktree(shell, repoPath, wtPath, integ, emit)
	}
}

func enableOnWorktree(shell git.ShellRunner, repoPath, wtPath string, integ Integration, emit func(ManagerEvent)) {
	// Detect if already installed globally
	installed := detectTool(shell, wtPath, integ)

	if !installed {
		// Check and install dependencies
		for _, dep := range integ.Dependencies {
			stepName := fmt.Sprintf("Check %s", dep.Name)
			emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusRunning})

			_, err := shell.RunShell(wtPath, dep.Detect)
			if err == nil {
				emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusDone})
				continue
			}

			if dep.Install == "" {
				emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusFailed,
					Error: fmt.Errorf("%s not found", dep.Name)})
				return
			}

			installName := fmt.Sprintf("Install %s", dep.Name)
			emit(ManagerEvent{Worktree: wtPath, Step: installName, Status: StatusRunning})
			if _, err := shell.RunShell(wtPath, dep.Install); err != nil {
				emit(ManagerEvent{Worktree: wtPath, Step: installName, Status: StatusFailed, Error: err})
				return
			}
			emit(ManagerEvent{Worktree: wtPath, Step: installName, Status: StatusDone})
		}

		// Install tool
		if integ.Install.Command != "" {
			stepName := fmt.Sprintf("Install %s", integ.Name)
			emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusRunning})
			if _, err := shell.RunShell(wtPath, integ.Install.Command); err != nil {
				emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusFailed, Error: err})
				return
			}
			emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusDone})
		}
	}

	// Run setup
	if integ.Setup.Command != "" {
		stepName := fmt.Sprintf("Setup %s", integ.Name)
		emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusRunning})

		command := strings.ReplaceAll(integ.Setup.Command, "{path}", wtPath)
		var runDir string
		switch integ.Setup.WorkingDir {
		case "repo":
			runDir = repoPath
		default:
			runDir = wtPath
		}

		if _, err := shell.RunShell(runDir, command); err != nil {
			emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusFailed, Error: err})
			return
		}
		emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusDone})
	}

	// Update gitignore
	if len(integ.GitignoreEntries) > 0 {
		if err := appendGitignoreEntries(wtPath, integ.GitignoreEntries); err != nil {
			emit(ManagerEvent{Worktree: wtPath, Step: fmt.Sprintf("Gitignore %s", integ.Name), Status: StatusFailed, Error: err})
		}
	}
}

// DisableIntegration tears down an integration from all given worktrees.
func DisableIntegration(shell git.ShellRunner, wtPaths []string, integ Integration, emit func(ManagerEvent)) {
	for _, wtPath := range wtPaths {
		disableOnWorktree(shell, wtPath, integ, emit)
	}
}

func disableOnWorktree(shell git.ShellRunner, wtPath string, integ Integration, emit func(ManagerEvent)) {
	// Run teardown command if defined
	if integ.Teardown.Command != "" {
		stepName := fmt.Sprintf("Teardown %s", integ.Name)
		emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusRunning})
		if _, err := shell.RunShell(wtPath, integ.Teardown.Command); err != nil {
			emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusFailed, Error: err})
		} else {
			emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusDone})
		}
	}

	// Remove artifact directories
	for _, dir := range integ.Teardown.Dirs {
		name := strings.TrimSuffix(dir, "/")
		stepName := fmt.Sprintf("Remove %s", name)
		emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusRunning})
		target := filepath.Join(wtPath, name)
		if err := os.RemoveAll(target); err != nil {
			emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusFailed, Error: err})
		} else {
			emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusDone})
		}
	}
}

func detectTool(shell git.ShellRunner, wtPath string, integ Integration) bool {
	if integ.Detect.Command != "" {
		_, err := shell.RunShell(wtPath, integ.Detect.Command)
		return err == nil
	}
	if integ.Detect.BinaryName != "" {
		_, err := shell.RunShell(wtPath, fmt.Sprintf("which %s", integ.Detect.BinaryName))
		return err == nil
	}
	return false
}

func appendGitignoreEntries(dir string, entries []string) error {
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
		return nil
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening .gitignore: %w", err)
	}
	defer f.Close()
	for _, entry := range toAdd {
		if _, err := fmt.Fprintln(f, entry); err != nil {
			return fmt.Errorf("writing to .gitignore: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./internal/integration/ -run "TestEnable|TestDisable" -v`
Expected: All tests pass.

- [ ] **Step 5: Run full test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./...`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/integration/manager.go internal/integration/manager_test.go
git commit -m "feat: add integration manager for enable/disable across worktrees"
```

---

### Task 5: Model & View State Scaffolding

**Files:**
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Add new view states to the viewState enum**

In `internal/tui/model.go`, after `migrateNextView`, add:

```go
integrationListView
integrationProgressView
migrateIntegrationsView
```

- [ ] **Step 2: Add integrationState struct**

Add after the `repoState` struct:

```go
// integrationState holds all state for the integration management flow.
type integrationState struct {
	integrations []integration.Integration
	current      map[string]bool // what's on disk right now
	staged       map[string]bool // desired state after apply (nil = no change staged)
	detected     map[string]bool // what was detected on disk at load time (for "detected" hints)
	cursor       int
	colCursor    int // 0-based column index for future expansion

	// Info dialog
	showInfo  bool
	infoCursor int // which integration is shown in the carousel

	// Progress
	events   []integration.ManagerEvent
	eventCh  chan integration.ManagerEvent
	doneCh   chan struct{}

	// Context: where to return after progress completes
	returnView viewState
}
```

- [ ] **Step 3: Add integrationState field to Model**

Add to the `Model` struct:

```go
integ integrationState
```

- [ ] **Step 4: Initialize integrationState in NewMenuModel**

In `NewMenuModel`, add to the model initialization:

```go
integ: integrationState{
    current:  make(map[string]bool),
    staged:   make(map[string]bool),
    detected: make(map[string]bool),
},
```

- [ ] **Step 5: Add dispatch cases to Update and View**

In `Update()`, add cases:

```go
case integrationListView:
    return m.updateIntegrationList(msg)
case integrationProgressView:
    return m.updateIntegrationProgress(msg)
case migrateIntegrationsView:
    return m.updateMigrateIntegrations(msg)
```

In `View()`, add cases:

```go
case integrationListView:
    return m.viewIntegrationList()
case integrationProgressView:
    return m.viewIntegrationProgress()
case migrateIntegrationsView:
    return m.viewMigrateIntegrations()
```

- [ ] **Step 6: Verify build compiles**

The build will fail because the update/view functions don't exist yet. Add placeholder stubs to unblock compilation:

Create `internal/tui/integration_list.go` with:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateIntegrationList(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) viewIntegrationList() string {
	return "  Integration list (todo)\n"
}
```

Create `internal/tui/integration_progress.go` with:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateIntegrationProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) viewIntegrationProgress() string {
	return "  Integration progress (todo)\n"
}
```

Create `internal/tui/migrate_integrations.go` with:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateMigrateIntegrations(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) viewMigrateIntegrations() string {
	return "  Migrate integrations (todo)\n"
}
```

- [ ] **Step 7: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build ./...`
Expected: Build succeeds.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/model.go internal/tui/integration_list.go internal/tui/integration_progress.go internal/tui/migrate_integrations.go
git commit -m "feat: scaffold integration view states and model"
```

---

### Task 6: Menu Item & Navigation Entry Point

**Files:**
- Modify: `internal/tui/menu.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Add "Manage integrations" to bare repo menu**

In `internal/tui/model.go` `NewMenuModel`, change the `ContextBareRepo` case:

```go
case repo.ContextBareRepo:
    items = []menuItem{
        {label: "Create new worktree", enabled: true},
        {label: "Manage integrations", enabled: true},
        {label: "Remove worktrees", hint: "loading\u2026", enabled: false},
        {label: "Cleanup", hint: "safe mode", enabled: true},
    }
```

- [ ] **Step 2: Add menu handler in menu.go**

In `updateMenu`, add a case in the `Confirm` key handler switch on `label`:

```go
case "Manage integrations":
    m.view = integrationListView
    return m, m.loadIntegrationState()
```

- [ ] **Step 3: Add loadIntegrationState command**

In `internal/tui/integration_list.go`, replace the stub and add the load command:

```go
package tui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/state"
)

type integrationStateLoadedMsg struct {
	integrations []integration.Integration
	current      map[string]bool
	enabled      []string
	err          error
}

func (m Model) loadIntegrationState() tea.Cmd {
	return func() tea.Msg {
		all := integration.All()
		mainWT := m.findSourceWorktree()
		current := make(map[string]bool)
		if mainWT != "" {
			current = integration.DetectAllPresent(mainWT, all)
		}

		bareDir := filepath.Join(m.repoPath, ".bare")
		st, err := state.Load(bareDir)
		if err != nil {
			return integrationStateLoadedMsg{
				integrations: all,
				current:      current,
				err:          err,
			}
		}

		return integrationStateLoadedMsg{
			integrations: all,
			current:      current,
			enabled:      st.Integrations,
		}
	}
}

func (m Model) updateIntegrationList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case integrationStateLoadedMsg:
		m.integ.integrations = msg.integrations
		m.integ.current = msg.current
		// Initialize staged from current disk state, overlaid with saved state
		m.integ.staged = make(map[string]bool)
		for _, integ := range msg.integrations {
			m.integ.staged[integ.Name] = msg.current[integ.Name]
		}
		// If state file had entries, use those as truth
		if len(msg.enabled) > 0 {
			for _, integ := range msg.integrations {
				m.integ.staged[integ.Name] = false
			}
			for _, name := range msg.enabled {
				m.integ.staged[name] = true
			}
		}
		return m, nil
	}
	return m, nil
}

func (m Model) viewIntegrationList() string {
	return "  Integration list (loading...)\n"
}
```

- [ ] **Step 4: Update updateMenuHints for new menu item index**

In `menu.go`, `updateMenuHints` references `m.menuItems[1]` for "Remove worktrees". Since we inserted "Manage integrations" at index 1, "Remove worktrees" moved to index 2. Update:

```go
func (m *Model) updateMenuHints() {
	if m.context != repo.ContextBareRepo {
		return
	}
	if len(m.menuItems) < 3 {
		return
	}
	count := len(m.remove.worktrees)
	if count > 0 {
		m.menuItems[2].hint = fmt.Sprintf("%d available", count)
		m.menuItems[2].enabled = true
	} else {
		m.menuItems[2].hint = "none"
		m.menuItems[2].enabled = false
	}
}
```

- [ ] **Step 5: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build ./...`
Expected: Build succeeds.

- [ ] **Step 6: Run tests**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./...`
Expected: All tests pass (some TUI tests may need adjusting if they assert on menu item indices).

- [ ] **Step 7: Commit**

```bash
git add internal/tui/model.go internal/tui/menu.go internal/tui/integration_list.go
git commit -m "feat: add Manage integrations menu item with state loading"
```

---

### Task 7: Integration List View (Full Implementation)

**Files:**
- Modify: `internal/tui/integration_list.go`

- [ ] **Step 1: Implement the full update handler**

Replace the `updateIntegrationList` function with the full implementation handling navigation, toggling, info dialog, and apply:

```go
func (m Model) updateIntegrationList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(msg.Height-6, 5)
		return m, nil

	case integrationStateLoadedMsg:
		m.integ.integrations = msg.integrations
		m.integ.current = msg.current
		// Initialize staged from disk state
		m.integ.staged = make(map[string]bool)
		for _, integ := range msg.integrations {
			m.integ.staged[integ.Name] = msg.current[integ.Name]
		}
		// If state file loaded successfully and had entries, overlay those as source of truth.
		// If state file was corrupt (msg.err != nil), fall back to disk detection only.
		if msg.err == nil && len(msg.enabled) > 0 {
			for _, integ := range msg.integrations {
				m.integ.staged[integ.Name] = false
			}
			for _, name := range msg.enabled {
				m.integ.staged[name] = true
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.integ.showInfo {
			return m.updateIntegrationInfo(msg)
		}

		switch {
		case key.Matches(msg, keys.IntDown):
			if m.integ.cursor < len(m.integ.integrations)-1 {
				m.integ.cursor++
			}

		case key.Matches(msg, keys.IntUp):
			if m.integ.cursor > 0 {
				m.integ.cursor--
			}

		case key.Matches(msg, keys.Toggle):
			if m.integ.cursor < len(m.integ.integrations) {
				name := m.integ.integrations[m.integ.cursor].Name
				m.integ.staged[name] = !m.integ.staged[name]
			}

		case key.Matches(msg, keys.Info):
			m.integ.showInfo = true
			m.integ.infoCursor = m.integ.cursor
			return m, nil

		case key.Matches(msg, keys.Confirm):
			if m.integrationHasPendingChanges() {
				return m, m.applyIntegrationChanges()
			}

		case key.Matches(msg, keys.Back):
			// Discard any pending changes and return to menu in one press
			for _, integ := range m.integ.integrations {
				m.integ.staged[integ.Name] = m.integ.current[integ.Name]
			}
			m.view = menuView
			return m, nil

		case key.Matches(msg, keys.Quit):
			m.view = menuView
			return m, nil
		}
	}
	return m, nil
}

func (m Model) integrationHasPendingChanges() bool {
	for _, integ := range m.integ.integrations {
		if m.integ.staged[integ.Name] != m.integ.current[integ.Name] {
			return true
		}
	}
	return false
}

func (m Model) pendingChangeCount() int {
	count := 0
	for _, integ := range m.integ.integrations {
		if m.integ.staged[integ.Name] != m.integ.current[integ.Name] {
			count++
		}
	}
	return count
}
```

Note: `keys.IntUp`, `keys.IntDown`, and `keys.Info` reference the bindings added in Task 3. `m.updateIntegrationInfo` will be implemented in Step 3. `m.applyIntegrationChanges` will be implemented in Task 8.

- [ ] **Step 2: Implement the view renderer**

```go
func (m Model) viewIntegrationList() string {
	var b strings.Builder

	b.WriteString(styleTitle.Render(fmt.Sprintf("  sentei %s Integrations", "\u2500")))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render(fmt.Sprintf("  %s (bare)", filepath.Base(m.repoPath))))
	b.WriteString("\n\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	if len(m.integ.integrations) == 0 {
		b.WriteString(styleDim.Render("  No integrations available."))
		b.WriteString("\n")
	}

	for i, integ := range m.integ.integrations {
		cursor := "  "
		if i == m.integ.cursor {
			cursor = "> "
		}

		staged := m.integ.staged[integ.Name]
		current := m.integ.current[integ.Name]

		var checkbox string
		switch {
		case staged && !current:
			checkbox = styleStagedAdd.Render("[+]")
		case !staged && current:
			checkbox = styleStagedRemove.Render("[-]")
		case staged:
			checkbox = styleCheckboxOn.Render("[x]")
		default:
			checkbox = styleCheckboxOff.Render("[ ]")
		}

		if i == m.integ.cursor {
			b.WriteString(styleAccent.Render(cursor) + checkbox + " " + integ.Name)
		} else {
			b.WriteString("  " + checkbox + " " + integ.Name)
		}
		b.WriteString("\n")
		b.WriteString("        " + styleDim.Render(integ.Description))
		b.WriteString("\n")

		if i < len(m.integ.integrations)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	pending := m.pendingChangeCount()
	if pending > 0 {
		b.WriteString(fmt.Sprintf("  %d %s pending\n\n",
			pending, pluralize(pending, "change", "changes")))
	}

	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	// Legend
	b.WriteString("  ")
	b.WriteString(styleCheckboxOn.Render("[x]"))
	b.WriteString(styleDim.Render(" active  "))
	b.WriteString(styleCheckboxOff.Render("[ ]"))
	b.WriteString(styleDim.Render(" inactive  "))
	b.WriteString(styleStagedAdd.Render("[+]"))
	b.WriteString(styleDim.Render(" adding  "))
	b.WriteString(styleStagedRemove.Render("[-]"))
	b.WriteString(styleDim.Render(" removing"))
	b.WriteString("\n\n")

	// Key hints
	hints := "  w/s navigate \u00b7 space toggle \u00b7 ? info"
	if pending > 0 {
		hints += " \u00b7 enter apply"
	}
	hints += " \u00b7 esc back"
	b.WriteString(styleDim.Render(hints))
	b.WriteString("\n")

	// Info dialog overlay
	if m.integ.showInfo {
		return m.renderIntegrationInfo(b.String())
	}

	return b.String()
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
```

- [ ] **Step 3: Implement the info dialog (carousel)**

Add to `integration_list.go`:

```go
func (m Model) updateIntegrationInfo(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.integ.showInfo = false
	case key.Matches(msg, keys.Left):
		if m.integ.infoCursor > 0 {
			m.integ.infoCursor--
		} else {
			m.integ.infoCursor = len(m.integ.integrations) - 1
		}
	case key.Matches(msg, keys.Right):
		if m.integ.infoCursor < len(m.integ.integrations)-1 {
			m.integ.infoCursor++
		} else {
			m.integ.infoCursor = 0
		}
	}
	return m, nil
}

func (m Model) renderIntegrationInfo(background string) string {
	if m.integ.infoCursor >= len(m.integ.integrations) {
		return background
	}

	integ := m.integ.integrations[m.integ.infoCursor]
	page := fmt.Sprintf("%d / %d", m.integ.infoCursor+1, len(m.integ.integrations))

	var content strings.Builder
	content.WriteString(fmt.Sprintf("%-30s %s", styleTitle.Render(integ.Name), styleDim.Render(page)))
	content.WriteString("\n\n")
	content.WriteString(integ.Description)
	content.WriteString("\n\n")
	content.WriteString(styleDim.Render(integ.URL))
	content.WriteString("\n")

	if len(integ.Dependencies) > 0 {
		var depNames []string
		for _, dep := range integ.Dependencies {
			depNames = append(depNames, dep.Name)
		}
		content.WriteString("\n")
		content.WriteString(fmt.Sprintf("%-10s %s", styleDim.Render("Requires"), strings.Join(depNames, ", ")))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	navHint := styleDim.Render(fmt.Sprintf("%s a/\u2190 prev \u00b7 d/\u2192 next %s", "\u25c0", "\u25b6"))
	content.WriteString(fmt.Sprintf("%40s", navHint))
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf("%40s", styleDim.Render("esc to close")))

	dialog := styleDialogBox.Render(content.String())

	// Center the dialog on the background
	return lipgloss.Place(
		m.width, m.height+6,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
	)
}
```

Add the `lipgloss` import and `key` import to the file's import block.

- [ ] **Step 4: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build ./...`
Expected: Build succeeds (with `applyIntegrationChanges` not yet implemented — add a placeholder returning nil for now).

Add a temporary placeholder at the bottom of integration_list.go:

```go
func (m Model) applyIntegrationChanges() tea.Cmd {
	return nil // implemented in Task 8
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/integration_list.go
git commit -m "feat: implement integration list view with toggle and info carousel"
```

---

### Task 8: Integration Progress View

**Files:**
- Modify: `internal/tui/integration_progress.go`
- Modify: `internal/tui/integration_list.go` (replace `applyIntegrationChanges` placeholder)

- [ ] **Step 1: Implement applyIntegrationChanges in integration_list.go**

Replace the placeholder `applyIntegrationChanges`:

```go
type integrationEventMsg struct {
	Event integration.ManagerEvent
}

type integrationApplyDoneMsg struct{}

func (m Model) applyIntegrationChanges() tea.Cmd {
	ch := make(chan integration.ManagerEvent, 50)
	doneCh := make(chan struct{}, 1)
	m.integ.eventCh = ch
	m.integ.doneCh = doneCh
	m.integ.events = nil
	m.integ.returnView = integrationListView
	m.view = integrationProgressView

	// Collect what needs to change
	var toEnable, toDisable []integration.Integration
	for _, integ := range m.integ.integrations {
		staged := m.integ.staged[integ.Name]
		current := m.integ.current[integ.Name]
		if staged && !current {
			toEnable = append(toEnable, integ)
		} else if !staged && current {
			toDisable = append(toDisable, integ)
		}
	}

	// Collect worktree paths
	var wtPaths []string
	for _, wt := range m.remove.worktrees {
		wtPaths = append(wtPaths, wt.Path)
	}

	go func() {
		emit := func(e integration.ManagerEvent) { ch <- e }

		for _, integ := range toEnable {
			integration.EnableIntegration(m.shell, m.repoPath, wtPaths, integ, emit)
		}
		for _, integ := range toDisable {
			integration.DisableIntegration(m.shell, wtPaths, integ, emit)
		}

		close(ch)
		doneCh <- struct{}{}
	}()

	return m.waitForIntegrationEvent()
}

func (m Model) waitForIntegrationEvent() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.integ.eventCh
		if !ok {
			<-m.integ.doneCh
			return integrationApplyDoneMsg{}
		}
		return integrationEventMsg{Event: ev}
	}
}
```

**Important**: `applyIntegrationChanges` needs to set `m.view` but since it returns a Cmd and the model is a value type, the view transition and channel setup must happen in the update handler, not in the Cmd factory. Refactor: move the setup into the `updateIntegrationList` Confirm handler:

In `updateIntegrationList`, replace the Confirm case:

```go
case key.Matches(msg, keys.Confirm):
    if m.integrationHasPendingChanges() {
        m.integ.events = nil
        m.integ.returnView = integrationListView
        m.view = integrationProgressView
        return m, m.startIntegrationApply()
    }
```

And rename/refactor. **Important**: This must be a value receiver that captures channels locally
and passes them to `waitForIntegrationEvent` as parameters — NOT stored on the model — to
avoid a data race between the goroutine and Bubble Tea's value-copy update loop.

```go
func (m Model) startIntegrationApply() (Model, tea.Cmd) {
    var toEnable, toDisable []integration.Integration
    for _, integ := range m.integ.integrations {
        staged := m.integ.staged[integ.Name]
        current := m.integ.current[integ.Name]
        if staged && !current {
            toEnable = append(toEnable, integ)
        } else if !staged && current {
            toDisable = append(toDisable, integ)
        }
    }

    var wtPaths []string
    for _, wt := range m.remove.worktrees {
        wtPaths = append(wtPaths, wt.Path)
    }

    ch := make(chan integration.ManagerEvent, 50)
    doneCh := make(chan struct{}, 1)
    m.integ.eventCh = ch
    m.integ.doneCh = doneCh

    repoPath := m.repoPath
    shell := m.shell
    mainWT := m.findSourceWorktree()

    go func() {
        emit := func(e integration.ManagerEvent) { ch <- e }
        for _, integ := range toEnable {
            integration.EnableIntegration(shell, repoPath, mainWT, wtPaths, integ, emit)
        }
        for _, integ := range toDisable {
            integration.DisableIntegration(shell, wtPaths, integ, emit)
        }
        close(ch)
        doneCh <- struct{}{}
    }()

    return m, waitForIntegrationEvent(ch, doneCh)
}
```

Update the Confirm handler in `updateIntegrationList` to use the returned model:

```go
case key.Matches(msg, keys.Confirm):
    if m.integrationHasPendingChanges() {
        m.integ.events = nil
        m.integ.returnView = integrationListView
        m.view = integrationProgressView
        m, cmd := m.startIntegrationApply()
        return m, cmd
    }
```

Update `waitForIntegrationEvent` to be a standalone function that takes channel params:

```go
func waitForIntegrationEvent(ch <-chan integration.ManagerEvent, doneCh <-chan struct{}) tea.Cmd {
    return func() tea.Msg {
        ev, ok := <-ch
        if !ok {
            <-doneCh
            return integrationApplyDoneMsg{}
        }
        return integrationEventMsg{Event: ev}
    }
}
```

Remove the standalone `applyIntegrationChanges` placeholder.

- [ ] **Step 2: Implement the progress update handler**

Replace the stub in `integration_progress.go`:

```go
package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/state"
)

func (m Model) updateIntegrationProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(msg.Height-6, 5)
		return m, nil

	case integrationEventMsg:
		m.integ.events = append(m.integ.events, msg.Event)
		return m, waitForIntegrationEvent(m.integ.eventCh, m.integ.doneCh)

	case integrationApplyDoneMsg:
		// Save state and refresh disk detection in a Cmd (avoid I/O on UI goroutine)
		return m, m.finalizeIntegrationApply()

	case integrationFinalizedMsg:
		if m.integ.returnView != migrateNextView {
			m.integ.current = msg.current
			for _, integ := range m.integ.integrations {
				m.integ.staged[integ.Name] = m.integ.current[integ.Name]
			}
		}
		m.view = m.integ.returnView
		return m, nil
	}
	return m, nil
}

type integrationFinalizedMsg struct {
	current map[string]bool
	err     error
}

func (m Model) finalizeIntegrationApply() tea.Cmd {
	repoPath := m.repoPath
	returnView := m.integ.returnView
	integrations := m.integ.integrations
	staged := make(map[string]bool)
	for k, v := range m.integ.staged {
		staged[k] = v
	}
	repoResult := m.repo.result

	return func() tea.Msg {
		bareDir := filepath.Join(repoPath, ".bare")
		if returnView == migrateNextView {
			if result, ok := repoResult.(repo.MigrateResult); ok {
				bareDir = filepath.Join(result.BareRoot, ".bare")
			}
		}

		var enabled []string
		for _, integ := range integrations {
			if staged[integ.Name] {
				enabled = append(enabled, integ.Name)
			}
		}
		err := state.Save(bareDir, &state.State{Integrations: enabled})

		current := make(map[string]bool)
		for _, integ := range integrations {
			current[integ.Name] = staged[integ.Name]
		}

		return integrationFinalizedMsg{current: current, err: err}
	}
}

func (m Model) viewIntegrationProgress() string {
	var b strings.Builder

	b.WriteString(styleTitle.Render(fmt.Sprintf("  sentei %s Applying Integration Changes", "\u2500")))
	b.WriteString("\n\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	// Group events by worktree
	type wtGroup struct {
		path   string
		events []integration.ManagerEvent
	}
	var groups []wtGroup
	groupIdx := make(map[string]int)

	for _, ev := range m.integ.events {
		idx, exists := groupIdx[ev.Worktree]
		if !exists {
			idx = len(groups)
			groupIdx[ev.Worktree] = idx
			groups = append(groups, wtGroup{path: ev.Worktree})
		}
		groups[idx].events = append(groups[idx].events, ev)
	}

	done, total := 0, 0
	for _, g := range groups {
		name := filepath.Base(g.path)
		b.WriteString("  " + styleTitle.Render(name))
		b.WriteString("\n")

		for _, ev := range g.events {
			var ind string
			switch ev.Status {
			case integration.StatusDone:
				ind = styleIndicatorDone.Render(indicatorDone)
				done++
				total++
			case integration.StatusRunning:
				ind = styleIndicatorActive.Render(indicatorActive)
				total++
			case integration.StatusFailed:
				ind = styleIndicatorFailed.Render(indicatorFailed)
				done++
				total++
			}
			b.WriteString(fmt.Sprintf("    %s %s", ind, ev.Step))
			if ev.Error != nil {
				b.WriteString("  " + styleError.Render(ev.Error.Error()))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if total > 0 {
		barWidth := max(m.width-8, 20)
		filled := (done * barWidth) / total
		empty := barWidth - filled
		bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", empty)
		b.WriteString(fmt.Sprintf("  %s %d/%d\n", styleDim.Render(bar), done, total))
	}

	b.WriteString("\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n")

	return b.String()
}
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/integration_list.go internal/tui/integration_progress.go
git commit -m "feat: implement integration apply progress view"
```

---

### Task 9: Migration Onboarding Integration Selection

**Files:**
- Modify: `internal/tui/migrate_integrations.go`
- Modify: `internal/tui/migrate_summary.go`

- [ ] **Step 1: Implement migrate integration selection view**

Replace the stub in `migrate_integrations.go`:

```go
package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

type migrateIntegrationDetectedMsg struct {
	integrations []integration.Integration
	detected     map[string]bool
}

func loadMigrateIntegrations(worktreePath string) tea.Cmd {
	return func() tea.Msg {
		all := integration.All()
		detected := integration.DetectAllPresent(worktreePath, all)
		return migrateIntegrationDetectedMsg{
			integrations: all,
			detected:     detected,
		}
	}
}

func (m Model) updateMigrateIntegrations(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(msg.Height-6, 5)
		return m, nil

	case migrateIntegrationDetectedMsg:
		m.integ.integrations = msg.integrations
		m.integ.current = make(map[string]bool)
		m.integ.staged = make(map[string]bool)
		m.integ.detected = msg.detected
		for _, integ := range msg.integrations {
			m.integ.staged[integ.Name] = msg.detected[integ.Name]
			m.integ.current[integ.Name] = false // nothing is "currently" enabled — this is onboarding
		}
		return m, nil

	case tea.KeyMsg:
		if m.integ.showInfo {
			return m.updateIntegrationInfo(msg)
		}

		switch {
		case key.Matches(msg, keys.IntDown):
			if m.integ.cursor < len(m.integ.integrations)-1 {
				m.integ.cursor++
			}

		case key.Matches(msg, keys.IntUp):
			if m.integ.cursor > 0 {
				m.integ.cursor--
			}

		case key.Matches(msg, keys.Toggle):
			if m.integ.cursor < len(m.integ.integrations) {
				name := m.integ.integrations[m.integ.cursor].Name
				m.integ.staged[name] = !m.integ.staged[name]
			}

		case key.Matches(msg, keys.Info):
			m.integ.showInfo = true
			m.integ.infoCursor = m.integ.cursor
			return m, nil

		case key.Matches(msg, keys.Confirm):
			// Apply selected integrations, then proceed to what-next
			hasSelections := false
			for _, integ := range m.integ.integrations {
				if m.integ.staged[integ.Name] {
					hasSelections = true
					break
				}
			}
			if hasSelections {
				m.integ.events = nil
				m.integ.returnView = migrateNextView
				m.view = integrationProgressView
				m, cmd := m.startMigrateIntegrationApply()
				return m, cmd
			}
			m.view = migrateNextView
			return m, nil

		case key.Matches(msg, keys.Back):
			// Skip integrations
			m.view = migrateNextView
			return m, nil
		}
	}
	return m, nil
}

func (m Model) startMigrateIntegrationApply() (Model, tea.Cmd) {
	result, ok := m.repo.result.(repo.MigrateResult)
	if !ok {
		return m, nil
	}

	var toEnable []integration.Integration
	for _, integ := range m.integ.integrations {
		if m.integ.staged[integ.Name] {
			toEnable = append(toEnable, integ)
		}
	}

	wtPath := result.WorktreePath
	if wtPath == "" {
		wtPath = filepath.Join(result.BareRoot, result.Branch)
	}

	ch := make(chan integration.ManagerEvent, 50)
	doneCh := make(chan struct{}, 1)
	m.integ.eventCh = ch
	m.integ.doneCh = doneCh

	repoPath := result.BareRoot
	shell := m.shell

	go func() {
		emit := func(e integration.ManagerEvent) { ch <- e }
		for _, integ := range toEnable {
			// mainWTPath == wtPath for migration (main is the only worktree),
			// so ccc copy optimization is correctly skipped — full index runs.
			integration.EnableIntegration(shell, repoPath, wtPath, []string{wtPath}, integ, emit)
		}
		close(ch)
		doneCh <- struct{}{}
	}()

	return m, waitForIntegrationEvent(ch, doneCh)
}

func (m Model) viewMigrateIntegrations() string {
	var b strings.Builder

	b.WriteString(styleTitle.Render(fmt.Sprintf("  sentei %s Set Up Integrations", "\u2500")))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  We detected your repo may benefit from"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  these dev tools. Select any to enable."))
	b.WriteString("\n\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	for i, integ := range m.integ.integrations {
		cursor := "  "
		if i == m.integ.cursor {
			cursor = "> "
		}

		staged := m.integ.staged[integ.Name]
		var checkbox string
		if staged {
			checkbox = styleCheckboxOn.Render("[x]")
		} else {
			checkbox = styleCheckboxOff.Render("[ ]")
		}

		hint := ""
		if m.integ.detected[integ.Name] {
			hint = "  " + styleDim.Render("detected")
		}

		if i == m.integ.cursor {
			b.WriteString(styleAccent.Render(cursor) + checkbox + " " + integ.Name + hint)
		} else {
			b.WriteString("  " + checkbox + " " + integ.Name + hint)
		}
		b.WriteString("\n")
		b.WriteString("        " + styleDim.Render(integ.Description))
		b.WriteString("\n")

		if i < len(m.integ.integrations)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(separator(m.width))
	b.WriteString("\n\n")

	b.WriteString("  ")
	b.WriteString(styleCheckboxOn.Render("[x]"))
	b.WriteString(styleDim.Render(" active  "))
	b.WriteString(styleCheckboxOff.Render("[ ]"))
	b.WriteString(styleDim.Render(" inactive"))
	b.WriteString("\n\n")

	b.WriteString(styleDim.Render("  w/s navigate \u00b7 space toggle \u00b7 enter continue \u00b7 ? info \u00b7 esc skip"))
	b.WriteString("\n")

	if m.integ.showInfo {
		return m.renderIntegrationInfo(b.String())
	}

	return b.String()
}
```

- [ ] **Step 2: Wire migration flow to show integration screen**

In `internal/tui/migrate_summary.go`, modify `updateMigrateSummary` to transition to `migrateIntegrationsView` instead of `migrateNextView`:

Change both the `Yes` and `No` key handlers:

```go
case key.Matches(msg, keys.Yes):
    if result.BackupPath != "" {
        _ = repo.DeleteBackup(result.BackupPath)
    }
    m.view = migrateIntegrationsView
    return m, loadMigrateIntegrations(m.migrateWorktreePath(result))

case key.Matches(msg, keys.No):
    m.view = migrateIntegrationsView
    return m, loadMigrateIntegrations(m.migrateWorktreePath(result))
```

Add a helper method:

```go
func (m Model) migrateWorktreePath(result repo.MigrateResult) string {
	if result.WorktreePath != "" {
		return result.WorktreePath
	}
	return filepath.Join(result.BareRoot, result.Branch)
}
```

- [ ] **Step 3: Verify migration path in finalizeIntegrationApply**

The `finalizeIntegrationApply` function in `integration_progress.go` (Task 8) already handles the migration case by checking `returnView == migrateNextView` and using `result.BareRoot` for the bare dir. No additional changes needed here — just verify the migration path works during E2E testing.

Note: during migration onboarding, `ccc index` runs as a full (slow) index because `mainWTPath == wtPath` (main is the only worktree), so the copy optimization is correctly skipped.

- [ ] **Step 4: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build ./...`
Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/migrate_integrations.go internal/tui/migrate_summary.go internal/tui/integration_progress.go
git commit -m "feat: add integration selection during migration onboarding"
```

---

### Task 10: Update Post-Migration Text

**Files:**
- Modify: `internal/tui/migrate_summary.go`

- [ ] **Step 1: Update viewMigrateNext text**

In `viewMigrateNext()`, replace the integration-specific text:

```go
// Replace these lines:
b.WriteString(styleDim.Render("  Continue in sentei to create worktrees"))
b.WriteString("\n")
b.WriteString(styleDim.Render("  and set up integrations (code-review-graph,"))
b.WriteString("\n")
b.WriteString(styleDim.Render("  cocoindex-code), or exit to your shell."))
b.WriteString("\n")

// With:
b.WriteString(styleDim.Render("  Your repo is ready for worktrees."))
b.WriteString("\n")
b.WriteString(styleDim.Render("  Continue in sentei to create worktrees"))
b.WriteString("\n")
b.WriteString(styleDim.Render("  and set up your workspace, or exit"))
b.WriteString("\n")
b.WriteString(styleDim.Render("  to your shell."))
b.WriteString("\n")
```

- [ ] **Step 2: Verify build and run any existing migration tests**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build ./... && go test ./internal/tui/ -v`
Expected: Build and tests pass.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/migrate_summary.go
git commit -m "fix: update post-migration text to remove specific integration names"
```

---

### Task 11: Update Create Worktree Flow

**Files:**
- Modify: `internal/tui/create_options.go`
- Modify: `internal/tui/create_branch.go`

- [ ] **Step 1: Remove integration toggles from create_options.go**

In `buildOptionItems()`, remove the integration section at the end:

```go
// Remove this block:
for _, integ := range m.create.integrations {
    items = append(items, optionItem{
        label:       integ.Name,
        description: integ.Description,
        hint:        integ.URL,
        key:         "int:" + integ.Name,
        section:     "integration",
    })
}
```

In `isOptionEnabled()`, remove the `int:` case:

```go
// Remove:
case strings.HasPrefix(item.key, "int:"):
    name := strings.TrimPrefix(item.key, "int:")
    return m.create.intEnabled[name]
```

In `toggleOption()`, remove the `int:` case:

```go
// Remove:
case strings.HasPrefix(item.key, "int:"):
    name := strings.TrimPrefix(item.key, "int:")
    m.create.intEnabled[name] = !m.create.intEnabled[name]
```

- [ ] **Step 2: Load active integrations in prepareCreateOptions, render in view**

Add an `activeIntegrationNames []string` field to `createState` in `model.go`:

```go
activeIntegrationNames []string // loaded from state, displayed as info line
```

In `prepareCreateOptions()` in `create_branch.go`, load the integration names (replacing the removed integration loading section):

```go
// Load active integration names from state for display
bareDir := filepath.Join(m.repoPath, ".bare")
st, _ := state.Load(bareDir)
m.create.activeIntegrationNames = nil
for _, name := range st.Integrations {
    switch name {
    case "code-review-graph":
        m.create.activeIntegrationNames = append(m.create.activeIntegrationNames, "crg")
    case "cocoindex-code":
        m.create.activeIntegrationNames = append(m.create.activeIntegrationNames, "ccc")
    default:
        m.create.activeIntegrationNames = append(m.create.activeIntegrationNames, name)
    }
}
```

In `viewCreateOptions()`, after rendering the option items and before the separator, add:

```go
b.WriteString("\n")

if len(m.create.activeIntegrationNames) > 0 {
    b.WriteString(styleDim.Render(fmt.Sprintf("  Integrations from main: %s",
        strings.Join(m.create.activeIntegrationNames, ", "))))
    b.WriteString("\n")
}
```

Add import for `state` and `filepath` to `create_branch.go` (not `create_options.go` — the load happens in the Update path).

- [ ] **Step 3: Update startCreation to read integrations from state**

In `startCreation()` in `create_options.go`, replace the integration collection logic:

```go
// Replace:
var enabledInts []integration.Integration
for _, integ := range m.create.integrations {
    if m.create.intEnabled[integ.Name] {
        enabledInts = append(enabledInts, integ)
    }
}

// With (reads state — this runs in the Update path, triggered by enter keypress, so I/O is acceptable):
bareDir := filepath.Join(m.repoPath, ".bare")
st, err := state.Load(bareDir)
if err != nil {
    st = &state.State{}
}
var enabledInts []integration.Integration
enabledSet := make(map[string]bool)
for _, name := range st.Integrations {
    enabledSet[name] = true
}
for _, integ := range integration.All() {
    if enabledSet[integ.Name] {
        enabledInts = append(enabledInts, integ)
    }
}
```

- [ ] **Step 4: Simplify prepareCreateOptions in create_branch.go**

In `prepareCreateOptions()`, remove the integration loading section:

```go
// Remove:
m.create.integrations = nil
enabledSet := make(map[string]bool)
for _, name := range m.cfg.IntegrationsEnabled {
    enabledSet[name] = true
}
for _, integ := range integration.All() {
    m.create.integrations = append(m.create.integrations, integ)
    m.create.intEnabled[integ.Name] = enabledSet[integ.Name]
}
```

The `create.integrations` and `create.intEnabled` fields in `createState` can also be removed since they're no longer used. Remove from the struct in `model.go`:

```go
// Remove from createState:
integrations  []integration.Integration
intEnabled    map[string]bool
```

And remove `intEnabled: make(map[string]bool)` from `NewMenuModel`.

- [ ] **Step 5: Remove section header rendering for "Integrations" in viewCreateOptions**

The `sectionLabel` logic that renders "Integrations" when `currentSection == "integration"` will no longer trigger since we removed integration items. But clean it up — the only section is now "setup", so simplify:

In `viewCreateOptions()`, replace the section header logic:

```go
// Replace the section header block with just "Setup":
b.WriteString("  " + styleTitle.Render("Setup"))
b.WriteString("\n\n")
```

And remove the `currentSection` tracking variable and the per-item section check.

- [ ] **Step 6: Verify build and tests**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build ./... && go test ./...`
Expected: All pass. Some tests in `creator_test.go` may still pass integration options — those are fine since `creator.Options.Integrations` still exists and is populated from state.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/create_options.go internal/tui/create_branch.go internal/tui/model.go
git commit -m "feat: replace integration toggles with repo-level state in create flow"
```

---

### Task 12: ccc Copy Optimization

**Files:**
- Modify: `internal/creator/integrations.go`
- Modify: `internal/creator/integrations_test.go`
- Modify: `internal/integration/manager.go`

- [ ] **Step 1: Write failing test for ccc copy**

Add to `internal/creator/integrations_test.go`:

```go
func TestCopyIntegrationIndex_CopiesFromSource(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source index
	srcIndex := filepath.Join(sourceDir, ".cocoindex_code")
	os.MkdirAll(srcIndex, 0755)
	os.WriteFile(filepath.Join(srcIndex, "settings.yml"), []byte("test"), 0644)

	err := copyIntegrationIndex(sourceDir, targetDir, ".cocoindex_code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify copy
	targetIndex := filepath.Join(targetDir, ".cocoindex_code", "settings.yml")
	if _, err := os.Stat(targetIndex); os.IsNotExist(err) {
		t.Error("expected settings.yml to be copied")
	}
}

func TestCopyIntegrationIndex_NoSourceIndex_ReturnsError(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	err := copyIntegrationIndex(sourceDir, targetDir, ".cocoindex_code")
	if err == nil {
		t.Error("expected error when source index doesn't exist")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./internal/creator/ -run TestCopyIntegration -v`
Expected: Compilation error — `copyCCCIndex` not defined.

- [ ] **Step 3a: Create shared fileutil.CopyDir**

Create `internal/fileutil/copy.go`:

```go
package fileutil

import (
	"os"
	"path/filepath"
)

// CopyDir recursively copies a directory tree from src to dst.
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
```

Create `internal/fileutil/copy_test.go`:

```go
package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "sub"), 0755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0644)

	dst := filepath.Join(t.TempDir(), "copy")
	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dst, "a.txt"))
	if string(got) != "hello" {
		t.Errorf("a.txt = %q, want hello", got)
	}
	got, _ = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if string(got) != "world" {
		t.Errorf("sub/b.txt = %q, want world", got)
	}
}
```

- [ ] **Step 3b: Implement copyIntegrationIndex**

Add to `internal/creator/integrations.go`:

```go
// copyIntegrationIndex copies the IndexCopyDir from source to target worktree.
// Returns an error if the source index doesn't exist. Uses the Integration's
// IndexCopyDir field — config-driven, no hardcoded integration names.
func copyIntegrationIndex(sourceWT, targetWT, indexDir string) error {
	srcDir := filepath.Join(sourceWT, indexDir)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("no index at %s", srcDir)
	}

	dstDir := filepath.Join(targetWT, indexDir)
	os.RemoveAll(dstDir)

	return fileutil.CopyDir(srcDir, dstDir)
}
```

Add `"github.com/bonboncinnabon/sentei/internal/fileutil"` to the import block.
```

- [ ] **Step 4: Integrate copy into setupIntegration**

In `setupIntegration()` in `internal/creator/integrations.go`, add a ccc copy step before running setup. Add a `sourceWorktree` parameter:

Update `runIntegrations` to pass the source worktree path:

```go
func runIntegrations(shell git.ShellRunner, wtPath string, opts Options, emit func(Event)) Phase {
	phase := Phase{Name: "Integrations"}
	if len(opts.Integrations) == 0 {
		return phase
	}
	for _, integ := range opts.Integrations {
		steps := setupIntegration(shell, wtPath, opts.RepoPath, opts.SourceWorktree, integ, emit)
		phase.Steps = append(phase.Steps, steps...)
	}
	return phase
}
```

Update `setupIntegration` signature to accept `sourceWorktree` and add the copy step:

```go
func setupIntegration(shell git.ShellRunner, wtPath, repoPath, sourceWorktree string, integ integration.Integration, emit func(Event)) []StepResult {
	var steps []StepResult

	// Copy optimization: if the integration has a copyable index dir, seed from source worktree
	if integ.IndexCopyDir != "" && sourceWorktree != "" {
		stepName := "Copy index from main"
		emit(Event{Phase: "Integrations", Step: stepName, Status: StepRunning})
		if err := copyIntegrationIndex(sourceWorktree, wtPath, integ.IndexCopyDir); err != nil {
			// Not fatal — just means we'll do a full index
			emit(Event{Phase: "Integrations", Step: stepName, Status: StepSkipped, Message: err.Error()})
		} else {
			emit(Event{Phase: "Integrations", Step: stepName, Status: StepDone})
			steps = append(steps, StepResult{Name: stepName, Status: StepDone})
		}
	}

	installed := detectIntegration(shell, wtPath, integ)
	// ... rest of existing logic unchanged
```

- [ ] **Step 5: Add same optimization to integration manager**

In `internal/integration/manager.go`, update `enableOnWorktree` to accept a `mainWTPath` parameter and add the copy step for ccc:

Update `EnableIntegration` signature:

```go
func EnableIntegration(shell git.ShellRunner, repoPath string, mainWTPath string, wtPaths []string, integ Integration, emit func(ManagerEvent)) {
	for _, wtPath := range wtPaths {
		enableOnWorktree(shell, repoPath, mainWTPath, wtPath, integ, emit)
	}
}
```

In `enableOnWorktree`, add before the detect/install/setup steps:

```go
func enableOnWorktree(shell git.ShellRunner, repoPath, mainWTPath, wtPath string, integ Integration, emit func(ManagerEvent)) {
	// Copy optimization: if the integration has a copyable index dir and this isn't main, seed from main
	if integ.IndexCopyDir != "" && mainWTPath != "" && wtPath != mainWTPath {
		srcDir := filepath.Join(mainWTPath, integ.IndexCopyDir)
		if _, err := os.Stat(srcDir); err == nil {
			stepName := "Copy index from main"
			emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusRunning})
			dstDir := filepath.Join(wtPath, integ.IndexCopyDir)
			os.RemoveAll(dstDir)
			if err := copyDirManager(srcDir, dstDir); err != nil {
				emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusFailed, Error: err})
			} else {
				emit(ManagerEvent{Worktree: wtPath, Step: stepName, Status: StatusDone})
			}
		}
	}

	// ... rest unchanged
```

Use `fileutil.CopyDir` from `internal/fileutil/copy.go` (shared helper, created in Task 12 Step 3a).
Replace `copyDirManager(srcDir, dstDir)` with `fileutil.CopyDir(srcDir, dstDir)`.
Add import: `"github.com/bonboncinnabon/sentei/internal/fileutil"`

- [ ] **Step 6: Update all callers of EnableIntegration to pass mainWTPath**

In `integration_list.go` `startIntegrationApply`:

```go
mainWT := m.findSourceWorktree()
// ...
integration.EnableIntegration(shell, repoPath, mainWT, wtPaths, integ, emit)
```

In `migrate_integrations.go` `startMigrateIntegrationApply`:

```go
integration.EnableIntegration(shell, repoPath, wtPath, []string{wtPath}, integ, emit)
// mainWTPath == wtPath for migration (main is the only worktree)
```

- [ ] **Step 7: Update tests**

Update `TestEnableIntegration_RunsSetupOnEachWorktree` in `manager_test.go` to pass `mainWTPath`:

```go
EnableIntegration(shell, "/repo", "/repo/main", wtPaths, integ, func(e ManagerEvent) {
    events = append(events, e)
})
```

- [ ] **Step 8: Run all tests**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./...`
Expected: All tests pass.

- [ ] **Step 9: Commit**

```bash
git add internal/creator/integrations.go internal/creator/integrations_test.go internal/integration/manager.go internal/integration/manager_test.go internal/tui/integration_list.go internal/tui/migrate_integrations.go
git commit -m "feat: add ccc copy optimization for cross-worktree index reuse"
```

---

### Task 13: End-to-End Verification

**Files:** None new — verification only.

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go test ./... -v`
Expected: All tests pass.

- [ ] **Step 2: Run go vet**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go vet ./...`
Expected: No issues.

- [ ] **Step 3: Build binary**

Run: `cd /Users/abiswas/code/personal/sentei/feature-integration-selection && go build -o sentei .`
Expected: Binary builds successfully.

- [ ] **Step 4: Manual smoke test**

Test in a real bare repo:
1. Run `./sentei` — verify "Manage integrations" appears in menu
2. Select "Manage integrations" — verify list view shows integrations with correct detection
3. Toggle an integration, press `?` — verify info carousel
4. Press `esc` to discard, verify changes reverted
5. Toggle and press `enter` — verify progress view

- [ ] **Step 5: Clean up binary**

Run: `rm sentei`

- [ ] **Step 6: Final commit if any fixes were needed**

```bash
git add -A
git commit -m "fix: address issues found during E2E verification"
```
