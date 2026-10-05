# Cleanup Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `sentei cleanup` subcommand that removes git repo cruft (config duplicates, stale refs, orphaned branches/config sections) to restore IDE performance.

**Architecture:** New `internal/cleanup/` package with standalone operations orchestrated sequentially. A `cmd/cleanup.go` adapter handles CLI flag parsing. TUI integration chains cleanup after worktree deletion via Bubble Tea messages.

**Tech Stack:** Go, existing `git.CommandRunner` interface for mockable git ops, `os.ReadFile`/`os.Rename` for atomic config file writes, standard `flag.FlagSet` for subcommand parsing.

**Spec:** `docs/superpowers/specs/2026-03-21-cleanup-command-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/cleanup/cleanup.go` | Create | Types (`Options`, `Result`, `Event`, etc.), orchestrator `Run()`, `resolveConfigPath()` |
| `internal/cleanup/config.go` | Create | `DedupConfig()`, `PurgeOrphanedBranchConfigs()`, atomic file write helper |
| `internal/cleanup/refs.go` | Create | `PruneRemoteRefs()` |
| `internal/cleanup/branches.go` | Create | `DeleteGoneBranches()`, `CleanNonWorktreeBranches()`, branch-vv parsing |
| `internal/cleanup/config_test.go` | Create | Table-driven tests for config dedup and orphan purge |
| `internal/cleanup/refs_test.go` | Create | Table-driven tests for ref pruning |
| `internal/cleanup/branches_test.go` | Create | Table-driven tests for branch cleanup |
| `internal/cleanup/cleanup_test.go` | Create | Orchestrator integration tests |
| `internal/cleanup/testdata/bloated.gitconfig` | Create | Fixture: config with duplicate key+value entries |
| `internal/cleanup/testdata/clean.gitconfig` | Create | Fixture: already-clean config |
| `internal/cleanup/testdata/multi-value.gitconfig` | Create | Fixture: legitimate multi-valued keys |
| `internal/cleanup/testdata/special-chars.gitconfig` | Create | Fixture: branch names with slashes, dots, numbers |
| `cmd/cleanup.go` | Create | CLI adapter: `RunCleanup()` with `flag.FlagSet`, event printer |
| `main.go` | Modify | Add subcommand dispatch before existing `flag.Parse()` |
| `internal/tui/model.go` | Modify | Add `cleanupResult` field to Model |
| `internal/tui/progress.go` | Modify | Chain cleanup after prune, add `cleanupCompleteMsg` |
| `internal/tui/summary.go` | Modify | Display cleanup results and aggressive-mode tip |
| `scripts/git-repo-cleanup.sh` | Replace | Thin wrapper delegating to `sentei cleanup` |

---

### Task 1: Types and Test Fixtures

**Files:**
- Create: `internal/cleanup/cleanup.go`
- Create: `internal/cleanup/testdata/bloated.gitconfig`
- Create: `internal/cleanup/testdata/clean.gitconfig`
- Create: `internal/cleanup/testdata/multi-value.gitconfig`
- Create: `internal/cleanup/testdata/special-chars.gitconfig`

- [ ] **Step 1: Create the cleanup package with all shared types**

Create `internal/cleanup/cleanup.go` with the types from the spec: `Mode`, `Options`, `Result`, `ConfigResult`, `OperationError`, `SkipReason`, `SkippedBranch`, `Event`, `EventLevel`, and the `resolveConfigPath` function. Include a stub `Run()` that returns an empty `Result`.

```go
package cleanup

import (
	"fmt"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/git"
)

type Mode string

const (
	ModeSafe       Mode = "safe"
	ModeAggressive Mode = "aggressive"
)

type Options struct {
	Mode   Mode
	Force  bool
	DryRun bool
}

type Result struct {
	ConfigDedupResult      ConfigResult
	ConfigOrphanResult     ConfigResult
	StaleRefsRemoved       int
	GoneBranchesDeleted    int
	NonWtBranchesDeleted   int
	NonWtBranchesRemaining int
	BranchesSkipped        []SkippedBranch
	Errors                 []OperationError
}

type ConfigResult struct {
	Before  int
	After   int
	Removed int
}

type OperationError struct {
	Step string
	Err  error
}

type SkipReason string

const (
	SkipUnmerged   SkipReason = "not fully merged"
	SkipInWorktree SkipReason = "checked out in worktree"
	SkipProtected  SkipReason = "protected branch"
)

type SkippedBranch struct {
	Name   string
	Reason SkipReason
}

type Event struct {
	Step    string
	Message string
	Level   EventLevel
}

type EventLevel int

const (
	LevelStep EventLevel = iota
	LevelInfo
	LevelWarn
	LevelDetail
)

func resolveConfigPath(runner git.CommandRunner, repoPath string) (string, error) {
	commonDir, err := runner.Run(repoPath, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("resolving config path: %w", err)
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repoPath, commonDir)
	}
	return filepath.Join(commonDir, "config"), nil
}

func Run(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) Result {
	return Result{}
}
```

- [ ] **Step 2: Create test fixtures**

Create `internal/cleanup/testdata/bloated.gitconfig`:
```ini
[core]
	repositoryformatversion = 0
	bare = true
	ignorecase = true
[branch "main"]
	vscode-merge-base = origin/main
	remote = origin
	merge = refs/heads/main
	github-pr-owner-number = "Org#repo#71"
	github-pr-owner-number = "Org#repo#71"
	github-pr-owner-number = "Org#repo#71"
	vscode-merge-base = origin/main
[branch "feature/old-work"]
	remote = origin
	merge = refs/heads/feature/old-work
	github-pr-owner-number = "Org#repo#42"
	github-pr-owner-number = "Org#repo#42"
[branch "fix/stale-branch"]
	remote = origin
	merge = refs/heads/fix/stale-branch
[remote "origin"]
	url = git@github.com:Org/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
```

Create `internal/cleanup/testdata/clean.gitconfig`:
```ini
[core]
	repositoryformatversion = 0
	bare = true
[branch "main"]
	remote = origin
	merge = refs/heads/main
[remote "origin"]
	url = git@github.com:Org/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
```

Create `internal/cleanup/testdata/multi-value.gitconfig`:
```ini
[core]
	repositoryformatversion = 0
[remote "origin"]
	url = git@github.com:Org/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
	fetch = +refs/tags/*:refs/tags/*
[branch "main"]
	remote = origin
	merge = refs/heads/main
```

Create `internal/cleanup/testdata/special-chars.gitconfig`:
```ini
[core]
	repositoryformatversion = 0
[branch "feature/deep/nested.v2"]
	remote = origin
	merge = refs/heads/feature/deep/nested.v2
[branch "fix/JIRA-1234-something"]
	remote = origin
	merge = refs/heads/fix/JIRA-1234-something
[branch "2026-03-01-migration-add-tables"]
	remote = origin
	merge = refs/heads/2026-03-01-migration-add-tables
```

- [ ] **Step 3: Write resolveConfigPath test**

Add to a new file `internal/cleanup/cleanup_test.go`:

```go
package cleanup

import (
	"fmt"
	"testing"
)

func TestResolveConfigPath(t *testing.T) {
	tests := []struct {
		name       string
		commonDir  string
		wantSuffix string
	}{
		{
			name:       "absolute path (bare repo)",
			commonDir:  "/repo/.bare",
			wantSuffix: "/repo/.bare/config",
		},
		{
			name:       "relative path (normal repo)",
			commonDir:  ".git",
			wantSuffix: ".git/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{responses: map[string]mockResponse{
				"/repo:[rev-parse --git-common-dir]": {output: tt.commonDir},
			}}

			path, err := resolveConfigPath(runner, "/repo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(path, tt.wantSuffix) {
				t.Errorf("path = %q, want suffix %q", path, tt.wantSuffix)
			}
		})
	}
}
```

Note: This test file also needs the `mockRunner` type and `strings` import. The `mockRunner` will be defined in a shared test helper file — see Task 3 which introduces it in `refs_test.go`. For Task 1, keep this as a placeholder and the actual mock definition will be consolidated in a `helpers_test.go` file or at the top of `cleanup_test.go`.

- [ ] **Step 4: Verify it compiles**

Run: `cd ~/code/personal/sentei/main && go build ./internal/cleanup/`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add internal/cleanup/cleanup.go internal/cleanup/cleanup_test.go internal/cleanup/testdata/
git commit -m "feat(cleanup): add types, fixtures, resolveConfigPath test, and stub orchestrator"
```

---

### Task 2: Config Dedup

**Files:**
- Create: `internal/cleanup/config.go`
- Create: `internal/cleanup/config_test.go`

- [ ] **Step 1: Write failing tests for DedupConfig**

Create `internal/cleanup/config_test.go` with table-driven tests. Use `os.ReadFile` on testdata fixtures and `os.CreateTemp` for writable copies.

```go
package cleanup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	tmp := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return tmp
}

func TestDedupConfig(t *testing.T) {
	tests := []struct {
		name            string
		fixture         string
		wantRemoved     int
		wantLinesBefore int
		wantLinesAfter  int
	}{
		{
			name:            "already clean",
			fixture:         "clean.gitconfig",
			wantRemoved:     0,
			wantLinesBefore: 7,
			wantLinesAfter:  7,
		},
		{
			name:            "removes exact duplicates",
			fixture:         "bloated.gitconfig",
			wantRemoved:     4,
			wantLinesBefore: 18,
			wantLinesAfter:  14,
		},
		{
			name:            "preserves multi-valued keys",
			fixture:         "multi-value.gitconfig",
			wantRemoved:     0,
			wantLinesBefore: 9,
			wantLinesAfter:  9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := copyFixture(t, tt.fixture)
			opts := Options{DryRun: false}
			events := collectEvents(t)

			result, err := DedupConfig(path, opts, events.emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Removed != tt.wantRemoved {
				t.Errorf("Removed = %d, want %d", result.Removed, tt.wantRemoved)
			}
			if result.Before != tt.wantLinesBefore {
				t.Errorf("Before = %d, want %d", result.Before, tt.wantLinesBefore)
			}
			if result.After != tt.wantLinesAfter {
				t.Errorf("After = %d, want %d", result.After, tt.wantLinesAfter)
			}
		})
	}
}

func TestDedupConfig_DryRun(t *testing.T) {
	path := copyFixture(t, "bloated.gitconfig")
	before, _ := os.ReadFile(path)

	opts := Options{DryRun: true}
	events := collectEvents(t)

	result, err := DedupConfig(path, opts, events.emit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Removed == 0 {
		t.Error("expected non-zero Removed count in dry-run")
	}

	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("dry-run modified the file")
	}
}

func TestDedupConfig_CreatesBackup(t *testing.T) {
	path := copyFixture(t, "bloated.gitconfig")
	opts := Options{DryRun: false}
	events := collectEvents(t)

	_, err := DedupConfig(path, opts, events.emit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bakPath := path + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		t.Error("backup file was not created")
	}
}

type eventCollector struct {
	events []Event
}

func collectEvents(t *testing.T) *eventCollector {
	t.Helper()
	return &eventCollector{}
}

func (c *eventCollector) emit(e Event) {
	c.events = append(c.events, e)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestDedup -v`
Expected: Compilation error — `DedupConfig` not defined

- [ ] **Step 3: Implement DedupConfig**

Create `internal/cleanup/config.go`:

```go
package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func DedupConfig(configPath string, opts Options, emit func(Event)) (ConfigResult, error) {
	emit(Event{Step: "dedup-config", Message: "Deduplicating git config...", Level: LevelStep})

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ConfigResult{}, fmt.Errorf("reading config: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	before := len(lines)

	var out []string
	seen := make(map[string]bool)

	for _, line := range lines {
		if strings.HasPrefix(line, "[") {
			seen = make(map[string]bool)
			out = append(out, line)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, line)
			continue
		}

		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, line)
	}

	after := len(out)
	removed := before - after
	result := ConfigResult{Before: before, After: after, Removed: removed}

	if removed == 0 {
		emit(Event{Step: "dedup-config", Message: fmt.Sprintf("Config already clean (%d lines)", before), Level: LevelInfo})
		return result, nil
	}

	if opts.DryRun {
		emit(Event{Step: "dedup-config", Message: fmt.Sprintf("Would remove %d duplicate lines (%d → %d)", removed, before, after), Level: LevelDetail})
		return result, nil
	}

	if err := atomicWriteConfig(configPath, strings.Join(out, "\n")); err != nil {
		return result, fmt.Errorf("writing deduped config: %w", err)
	}

	emit(Event{Step: "dedup-config", Message: fmt.Sprintf("Deduplicated config: removed %d lines (%d → %d)", removed, before, after), Level: LevelInfo})
	return result, nil
}

func atomicWriteConfig(configPath string, content string) error {
	bakPath := configPath + ".bak"

	info, err := os.Stat(configPath)
	var perm os.FileMode = 0644
	if err == nil {
		perm = info.Mode().Perm()
	}

	original, err := os.ReadFile(configPath)
	if err == nil {
		if err := os.WriteFile(bakPath, original, perm); err != nil {
			return fmt.Errorf("creating backup: %w", err)
		}
	}

	dir := filepath.Dir(configPath)
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestDedup -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cleanup/config.go internal/cleanup/config_test.go
git commit -m "feat(cleanup): implement config dedup with atomic writes"
```

---

### Task 3: Remote Ref Pruning

**Files:**
- Create: `internal/cleanup/refs.go`
- Create: `internal/cleanup/refs_test.go`

- [ ] **Step 1: Write failing tests for PruneRemoteRefs**

Create `internal/cleanup/refs_test.go`. Define a shared `mockRunner` type here — it will be available to all `_test.go` files in the package since they share the `cleanup` package in test scope. Also move the `eventCollector` helper here so all test files share it.

```go
package cleanup

import (
	"fmt"
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

func TestPruneRemoteRefs(t *testing.T) {
	tests := []struct {
		name        string
		dryRun      bool
		pruneOutput string
		wantCount   int
	}{
		{
			name:        "no stale refs",
			pruneOutput: "",
			wantCount:   0,
		},
		{
			name: "some stale refs",
			pruneOutput: `Pruning origin
URL: git@github.com:Org/repo.git
 * [would prune] origin/feature/old
 * [would prune] origin/fix/done`,
			wantCount: 2,
		},
		{
			name: "dry run reports count",
			dryRun: true,
			pruneOutput: `Pruning origin
URL: git@github.com:Org/repo.git
 * [would prune] origin/feature/old`,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{responses: map[string]mockResponse{
				"/repo:[remote prune origin --dry-run]": {output: tt.pruneOutput},
				"/repo:[fetch --prune origin]":          {output: ""},
			}}
			opts := Options{DryRun: tt.dryRun}
			events := collectEvents(t)

			count, err := PruneRemoteRefs(runner, "/repo", opts, events.emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if count != tt.wantCount {
				t.Errorf("count = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestPruneRemoteRefs_DryRunDoesNotFetch(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResponse{
		"/repo:[remote prune origin --dry-run]": {output: " * [would prune] origin/old"},
	}}
	opts := Options{DryRun: true}
	events := collectEvents(t)

	_, err := PruneRemoteRefs(runner, "/repo", opts, events.emit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, call := range runner.calls {
		if call == "/repo:[fetch --prune origin]" {
			t.Error("dry-run should not call fetch --prune")
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestPrune -v`
Expected: Compilation error — `PruneRemoteRefs` not defined

- [ ] **Step 3: Implement PruneRemoteRefs**

Create `internal/cleanup/refs.go`:

```go
package cleanup

import (
	"fmt"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func PruneRemoteRefs(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) (int, error) {
	emit(Event{Step: "prune-refs", Message: "Pruning stale remote refs...", Level: LevelStep})

	output, err := runner.Run(repoPath, "remote", "prune", "origin", "--dry-run")
	if err != nil {
		return 0, fmt.Errorf("checking stale refs: %w", err)
	}

	count := strings.Count(output, "[would prune]")

	if count == 0 {
		emit(Event{Step: "prune-refs", Message: "No stale remote refs", Level: LevelInfo})
		return 0, nil
	}

	if opts.DryRun {
		emit(Event{Step: "prune-refs", Message: fmt.Sprintf("Would prune %d stale remote ref(s)", count), Level: LevelDetail})
		return count, nil
	}

	if _, err := runner.Run(repoPath, "fetch", "--prune", "origin"); err != nil {
		return 0, fmt.Errorf("pruning remote refs: %w", err)
	}

	emit(Event{Step: "prune-refs", Message: fmt.Sprintf("Pruned %d stale remote ref(s)", count), Level: LevelInfo})
	return count, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestPrune -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cleanup/refs.go internal/cleanup/refs_test.go
git commit -m "feat(cleanup): implement remote ref pruning"
```

---

### Task 4: Gone-Upstream Branch Cleanup

**Files:**
- Create: `internal/cleanup/branches.go`
- Create: `internal/cleanup/branches_test.go`

- [ ] **Step 1: Write failing tests for DeleteGoneBranches**

Create `internal/cleanup/branches_test.go`:

```go
package cleanup

import (
	"fmt"
	"testing"
)

func TestDeleteGoneBranches(t *testing.T) {
	tests := []struct {
		name           string
		branchVV       string
		extraResponses map[string]mockResponse
		dryRun         bool
		wantDeleted    int
		wantSkipped    int
	}{
		{
			name:        "no gone branches",
			branchVV:    "  main abc123 [origin/main] latest commit",
			wantDeleted: 0,
		},
		{
			name: "deletes gone branches",
			branchVV: `  feature/old abc123 [origin/feature/old: gone] old commit
  main def456 [origin/main] latest`,
			extraResponses: map[string]mockResponse{
				"/repo:[branch -d feature/old]": {output: "Deleted branch feature/old"},
			},
			wantDeleted: 1,
		},
		{
			name: "skips worktree-checkout branches",
			branchVV: `+ fix/in-wt abc123 (/path/to/wt) [origin/fix/in-wt: gone] commit
  feature/gone def456 [origin/feature/gone: gone] commit`,
			extraResponses: map[string]mockResponse{
				"/repo:[branch -d feature/gone]": {output: "Deleted branch feature/gone"},
			},
			wantDeleted: 1,
			wantSkipped: 1,
		},
		{
			name: "skips unmerged on delete failure",
			branchVV: `  feature/unmerged abc123 [origin/feature/unmerged: gone] commit`,
			extraResponses: map[string]mockResponse{
				"/repo:[branch -d feature/unmerged]": {err: fmt.Errorf("error: branch not fully merged")},
			},
			wantDeleted: 0,
			wantSkipped: 1,
		},
		{
			name: "dry run counts without deleting",
			branchVV: `  feature/gone abc123 [origin/feature/gone: gone] commit`,
			dryRun:      true,
			wantDeleted: 1,
			wantSkipped: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := map[string]mockResponse{
				"/repo:[branch -vv]": {output: tt.branchVV},
			}
			for k, v := range tt.extraResponses {
				responses[k] = v
			}

			runner := &mockRunner{responses: responses}
			opts := Options{DryRun: tt.dryRun}
			events := collectEvents(t)

			result, err := DeleteGoneBranches(runner, "/repo", opts, events.emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Deleted != tt.wantDeleted {
				t.Errorf("Deleted = %d, want %d", result.Deleted, tt.wantDeleted)
			}
			if len(result.Skipped) != tt.wantSkipped {
				t.Errorf("Skipped = %d, want %d", len(result.Skipped), tt.wantSkipped)
			}
		})
	}
}

func TestParseGoneBranches(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantGone        int
		wantWorktreeGone int
	}{
		{
			name:  "empty output",
			input: "",
		},
		{
			name:  "no gone branches",
			input: "  main abc123 [origin/main] latest",
		},
		{
			name:     "standard gone branch",
			input:    "  feature/old abc123 [origin/feature/old: gone] commit",
			wantGone: 1,
		},
		{
			name:             "worktree-checkout gone branch",
			input:            "+ fix/wt abc123 (/path) [origin/fix/wt: gone] commit",
			wantWorktreeGone: 1,
		},
		{
			name:     "current branch with gone upstream",
			input:    "* feature/current abc123 [origin/feature/current: gone] commit",
			wantGone: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gone, wtGone := parseGoneBranches(tt.input)
			if len(gone) != tt.wantGone {
				t.Errorf("gone = %d, want %d", len(gone), tt.wantGone)
			}
			if len(wtGone) != tt.wantWorktreeGone {
				t.Errorf("worktreeGone = %d, want %d", len(wtGone), tt.wantWorktreeGone)
			}
		})
	}
}
```

Note: The test setup for mock runner responses will need refinement during implementation — specifically, the `git branch -d <name>` calls need response entries. Fill these in when writing the implementation to match the exact `runner.Run()` call format.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestDeleteGone -v`
Expected: Compilation error — `DeleteGoneBranches` not defined

- [ ] **Step 3: Implement DeleteGoneBranches**

Add to `internal/cleanup/branches.go`:

```go
package cleanup

import (
	"fmt"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

type BranchCleanResult struct {
	Deleted   int
	Remaining int
	Skipped   []SkippedBranch
}

func DeleteGoneBranches(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) (BranchCleanResult, error) {
	emit(Event{Step: "gone-branches", Message: "Deleting branches with gone upstream...", Level: LevelStep})

	output, err := runner.Run(repoPath, "branch", "-vv")
	if err != nil {
		return BranchCleanResult{}, fmt.Errorf("listing branches: %w", err)
	}

	gone, worktreeGone := parseGoneBranches(output)

	var result BranchCleanResult

	for _, b := range worktreeGone {
		result.Skipped = append(result.Skipped, SkippedBranch{Name: b, Reason: SkipInWorktree})
	}

	if len(gone) == 0 {
		if len(worktreeGone) == 0 {
			emit(Event{Step: "gone-branches", Message: "No branches with gone upstream", Level: LevelInfo})
		}
		return result, nil
	}

	if opts.DryRun {
		result.Deleted = len(gone)
		emit(Event{Step: "gone-branches", Message: fmt.Sprintf("Would delete %d branch(es) with gone upstream", len(gone)), Level: LevelDetail})
		return result, nil
	}

	for _, b := range gone {
		if _, err := runner.Run(repoPath, "branch", "-d", b); err != nil {
			result.Skipped = append(result.Skipped, SkippedBranch{Name: b, Reason: SkipUnmerged})
		} else {
			result.Deleted++
		}
	}

	if result.Deleted > 0 {
		emit(Event{Step: "gone-branches", Message: fmt.Sprintf("Deleted %d branch(es) with gone upstream", result.Deleted), Level: LevelInfo})
	}
	if skipped := len(result.Skipped) - len(worktreeGone); skipped > 0 {
		emit(Event{Step: "gone-branches", Message: fmt.Sprintf("%d branch(es) skipped (not fully merged)", skipped), Level: LevelWarn})
	}

	return result, nil
}

func parseGoneBranches(output string) (gone []string, worktreeGone []string) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, ": gone]") {
			continue
		}

		trimmed := strings.TrimLeft(line, " ")
		inWorktree := strings.HasPrefix(trimmed, "+ ")
		if inWorktree {
			trimmed = strings.TrimPrefix(trimmed, "+ ")
		} else {
			trimmed = strings.TrimPrefix(trimmed, "* ")
		}

		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		branch := fields[0]

		if inWorktree {
			worktreeGone = append(worktreeGone, branch)
		} else {
			gone = append(gone, branch)
		}
	}
	return
}
```

- [ ] **Step 4: Fix up test mock responses and run tests**

Update the test mock responses to include `git branch -d` calls for the specific branches being deleted. Then run:

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestDeleteGone -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cleanup/branches.go internal/cleanup/branches_test.go
git commit -m "feat(cleanup): implement gone-upstream branch deletion"
```

---

### Task 5: Non-Worktree Branch Cleanup

**Files:**
- Modify: `internal/cleanup/branches.go`
- Modify: `internal/cleanup/branches_test.go`

- [ ] **Step 1: Write failing tests for CleanNonWorktreeBranches**

Add to `internal/cleanup/branches_test.go`:

```go
func TestCleanNonWorktreeBranches(t *testing.T) {
	worktreeList := `worktree /repo
bare

worktree /repo/main
HEAD abc123
branch refs/heads/main

worktree /repo/feature-x
HEAD def456
branch refs/heads/feature-x`

	branchList := `main
feature-x
feature/old
fix/stale
develop`

	tests := []struct {
		name          string
		mode          Mode
		force         bool
		dryRun        bool
		deleteErrs    map[string]error
		wantDeleted   int
		wantRemaining int
		wantSkipped   int
	}{
		{
			name:          "safe mode only counts",
			mode:          ModeSafe,
			wantDeleted:   0,
			wantRemaining: 2, // feature/old + fix/stale (develop is protected)
		},
		{
			name:          "aggressive deletes non-worktree branches",
			mode:          ModeAggressive,
			wantDeleted:   2, // feature/old + fix/stale
			wantRemaining: 0,
			wantSkipped:   0,
		},
		{
			name:          "aggressive skips protected branches",
			mode:          ModeAggressive,
			wantDeleted:   2,
			wantSkipped:   0, // develop is protected but not reported as skipped, just silently excluded
		},
		{
			name:          "aggressive dry run",
			mode:          ModeAggressive,
			dryRun:        true,
			wantDeleted:   2,
			wantRemaining: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{responses: map[string]mockResponse{
				"/repo:[worktree list --porcelain]":               {output: worktreeList},
				"/repo:[branch --format=%(refname:short)]":        {output: branchList},
				"/repo:[branch -d feature/old]":                   {output: ""},
				"/repo:[branch -d fix/stale]":                     {output: ""},
				"/repo:[branch -D feature/old]":                   {output: ""},
				"/repo:[branch -D fix/stale]":                     {output: ""},
			}}
			opts := Options{Mode: tt.mode, Force: tt.force, DryRun: tt.dryRun}
			events := collectEvents(t)

			result, err := CleanNonWorktreeBranches(runner, "/repo", opts, events.emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Deleted != tt.wantDeleted {
				t.Errorf("Deleted = %d, want %d", result.Deleted, tt.wantDeleted)
			}
			if result.Remaining != tt.wantRemaining {
				t.Errorf("Remaining = %d, want %d", result.Remaining, tt.wantRemaining)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestCleanNon -v`
Expected: Compilation error — `CleanNonWorktreeBranches` not defined

- [ ] **Step 3: Implement CleanNonWorktreeBranches**

Add to `internal/cleanup/branches.go`:

```go
func CleanNonWorktreeBranches(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) (BranchCleanResult, error) {
	emit(Event{Step: "non-wt-branches", Message: "Checking non-worktree branches...", Level: LevelStep})

	wtOutput, err := runner.Run(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return BranchCleanResult{}, fmt.Errorf("listing worktrees: %w", err)
	}
	wtBranches := parseWorktreeBranches(wtOutput)

	branchOutput, err := runner.Run(repoPath, "branch", "--format=%(refname:short)")
	if err != nil {
		return BranchCleanResult{}, fmt.Errorf("listing branches: %w", err)
	}

	var candidates []string
	for _, line := range strings.Split(branchOutput, "\n") {
		b := strings.TrimSpace(line)
		if b == "" {
			continue
		}
		if wtBranches[b] || git.IsProtectedBranch(b) {
			continue
		}
		candidates = append(candidates, b)
	}

	var result BranchCleanResult

	if opts.Mode != ModeAggressive {
		result.Remaining = len(candidates)
		if len(candidates) > 0 {
			emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("%d branch(es) not in any worktree", len(candidates)), Level: LevelDetail})
		}
		return result, nil
	}

	if opts.DryRun {
		result.Deleted = len(candidates)
		emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("Would delete %d non-worktree branch(es)", len(candidates)), Level: LevelDetail})
		return result, nil
	}

	deleteFlag := "-d"
	if opts.Force {
		deleteFlag = "-D"
	}

	for _, b := range candidates {
		if _, err := runner.Run(repoPath, "branch", deleteFlag, b); err != nil {
			result.Skipped = append(result.Skipped, SkippedBranch{Name: b, Reason: SkipUnmerged})
			result.Remaining++
		} else {
			result.Deleted++
		}
	}

	if result.Deleted > 0 {
		emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("Deleted %d non-worktree branch(es)", result.Deleted), Level: LevelInfo})
	}
	if len(result.Skipped) > 0 {
		emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("%d branch(es) skipped (not fully merged — use --force)", len(result.Skipped)), Level: LevelWarn})
	}

	return result, nil
}

func parseWorktreeBranches(output string) map[string]bool {
	branches := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "branch refs/heads/") {
			b := strings.TrimPrefix(line, "branch refs/heads/")
			branches[b] = true
		}
	}
	return branches
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestCleanNon -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cleanup/branches.go internal/cleanup/branches_test.go
git commit -m "feat(cleanup): implement non-worktree branch cleanup"
```

---

### Task 6: Orphaned Config Section Purge

**Files:**
- Modify: `internal/cleanup/config.go`
- Modify: `internal/cleanup/config_test.go`

- [ ] **Step 1: Write failing tests for PurgeOrphanedBranchConfigs**

Add to `internal/cleanup/config_test.go`:

```go
func TestPurgeOrphanedBranchConfigs(t *testing.T) {
	tests := []struct {
		name           string
		fixture        string
		existBranches  string
		wantRemoved    int
	}{
		{
			name:          "no orphans",
			fixture:       "bloated.gitconfig",
			existBranches: "main\nfeature/old-work\nfix/stale-branch",
			wantRemoved:   0,
		},
		{
			name:          "removes orphaned sections",
			fixture:       "bloated.gitconfig",
			existBranches: "main",
			wantRemoved:   2,
		},
		{
			name:          "all branches gone",
			fixture:       "bloated.gitconfig",
			existBranches: "",
			wantRemoved:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := copyFixture(t, tt.fixture)
			runner := &mockRunner{responses: map[string]mockResponse{
				"/repo:[branch --format=%(refname:short)]": {output: tt.existBranches},
			}}
			opts := Options{DryRun: false}
			events := collectEvents(t)

			result, err := PurgeOrphanedBranchConfigs(runner, "/repo", path, opts, events.emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Removed != tt.wantRemoved {
				t.Errorf("Removed = %d, want %d", result.Removed, tt.wantRemoved)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestPurgeOrphaned -v`
Expected: Compilation error — `PurgeOrphanedBranchConfigs` not defined

- [ ] **Step 3: Implement PurgeOrphanedBranchConfigs**

Add to `internal/cleanup/config.go`:

```go
func PurgeOrphanedBranchConfigs(runner git.CommandRunner, repoPath string, configPath string, opts Options, emit func(Event)) (ConfigResult, error) {
	emit(Event{Step: "orphaned-configs", Message: "Removing config sections for deleted branches...", Level: LevelStep})

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ConfigResult{}, fmt.Errorf("reading config: %w", err)
	}

	branchOutput, err := runner.Run(repoPath, "branch", "--format=%(refname:short)")
	if err != nil {
		return ConfigResult{}, fmt.Errorf("listing branches: %w", err)
	}

	existing := make(map[string]bool)
	for _, line := range strings.Split(branchOutput, "\n") {
		b := strings.TrimSpace(line)
		if b != "" {
			existing[b] = true
		}
	}

	lines := strings.Split(string(data), "\n")
	before := len(lines)
	var out []string
	skip := false
	orphanCount := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "[branch \"") {
			branchName := line
			branchName = strings.TrimPrefix(branchName, "[branch \"")
			branchName = strings.TrimSuffix(branchName, "\"]")
			if !existing[branchName] {
				skip = true
				orphanCount++
				continue
			}
			skip = false
			out = append(out, line)
			continue
		}

		if strings.HasPrefix(line, "[") {
			skip = false
			out = append(out, line)
			continue
		}

		if !skip {
			out = append(out, line)
		}
	}

	after := len(out)
	result := ConfigResult{Before: before, After: after, Removed: orphanCount}

	if orphanCount == 0 {
		emit(Event{Step: "orphaned-configs", Message: "No orphaned branch config sections", Level: LevelInfo})
		return result, nil
	}

	if opts.DryRun {
		emit(Event{Step: "orphaned-configs", Message: fmt.Sprintf("Would remove %d orphaned branch config section(s)", orphanCount), Level: LevelDetail})
		return result, nil
	}

	if err := atomicWriteConfig(configPath, strings.Join(out, "\n")); err != nil {
		return result, fmt.Errorf("writing purged config: %w", err)
	}

	emit(Event{Step: "orphaned-configs", Message: fmt.Sprintf("Removed %d orphaned config sections (%d → %d lines)", orphanCount, before, after), Level: LevelInfo})
	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestPurgeOrphaned -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cleanup/config.go internal/cleanup/config_test.go
git commit -m "feat(cleanup): implement orphaned config section purge"
```

---

### Task 7: Orchestrator

**Files:**
- Modify: `internal/cleanup/cleanup.go`
- Create: `internal/cleanup/cleanup_test.go`

- [ ] **Step 1: Write failing orchestrator tests**

Create `internal/cleanup/cleanup_test.go`:

```go
package cleanup

import (
	"testing"
)

// setupOrchestratorTest creates a temp dir with a config file and returns
// a mock runner whose resolveConfigPath points to it.
func setupOrchestratorTest(t *testing.T) (*mockRunner, string) {
	t.Helper()
	tmpDir := t.TempDir()
	bareDir := filepath.Join(tmpDir, ".bare")
	os.MkdirAll(bareDir, 0755)

	configData, _ := os.ReadFile(filepath.Join("testdata", "bloated.gitconfig"))
	configPath := filepath.Join(bareDir, "config")
	os.WriteFile(configPath, configData, 0644)

	runner := &mockRunner{responses: map[string]mockResponse{
		tmpDir + ":[rev-parse --git-common-dir]":        {output: bareDir},
		tmpDir + ":[remote prune origin --dry-run]":     {output: ""},
		tmpDir + ":[fetch --prune origin]":              {output: ""},
		tmpDir + ":[branch -vv]":                        {output: "  main abc123 [origin/main] latest"},
		tmpDir + ":[worktree list --porcelain]":         {output: "worktree " + tmpDir + "\nbare\n\nworktree " + tmpDir + "/main\nHEAD abc\nbranch refs/heads/main"},
		tmpDir + ":[branch --format=%(refname:short)]":  {output: "main\nfeature/old"},
	}}

	return runner, tmpDir
}

func TestRun_SafeMode(t *testing.T) {
	runner, repoPath := setupOrchestratorTest(t)
	events := collectEvents(t)

	result := Run(runner, repoPath, Options{Mode: ModeSafe}, events.emit)

	if result.NonWtBranchesDeleted != 0 {
		t.Error("safe mode should not delete non-worktree branches")
	}
	if result.NonWtBranchesRemaining == 0 {
		t.Error("safe mode should still count remaining non-worktree branches")
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestRun_AggressiveMode(t *testing.T) {
	runner, repoPath := setupOrchestratorTest(t)
	runner.responses[repoPath+":[branch -d feature/old]"] = mockResponse{output: "Deleted"}
	events := collectEvents(t)

	result := Run(runner, repoPath, Options{Mode: ModeAggressive}, events.emit)

	if result.NonWtBranchesDeleted == 0 {
		t.Error("aggressive mode should delete non-worktree branches")
	}
}

func TestRun_ErrorContinues(t *testing.T) {
	runner, repoPath := setupOrchestratorTest(t)
	runner.responses[repoPath+":[remote prune origin --dry-run]"] = mockResponse{err: fmt.Errorf("network error")}
	events := collectEvents(t)

	result := Run(runner, repoPath, Options{Mode: ModeSafe}, events.emit)

	if len(result.Errors) == 0 {
		t.Error("expected errors to be recorded")
	}
	foundPruneError := false
	for _, e := range result.Errors {
		if e.Step == "prune-refs" {
			foundPruneError = true
		}
	}
	if !foundPruneError {
		t.Error("expected prune-refs error to be recorded")
	}
	// Config dedup should still have run (check config was processed)
	if result.ConfigDedupResult.Before == 0 {
		t.Error("config dedup should have run despite prune failure")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -run TestRun -v`
Expected: FAIL — orchestrator `Run()` is a stub returning empty Result

- [ ] **Step 3: Implement the orchestrator**

Update `Run()` in `internal/cleanup/cleanup.go` per the spec's orchestrator pseudocode. Wire all 5 steps sequentially. Handle config path resolution, config file creation for tests.

- [ ] **Step 4: Run all cleanup tests**

Run: `cd ~/code/personal/sentei/main && go test ./internal/cleanup/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cleanup/cleanup.go internal/cleanup/cleanup_test.go
git commit -m "feat(cleanup): wire orchestrator with all 5 pipeline steps"
```

---

### Task 8: CLI Subcommand

**Files:**
- Create: `cmd/cleanup.go`
- Modify: `main.go`

- [ ] **Step 1: Create cmd/cleanup.go**

```go
package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
)

const (
	green  = "\033[0;32m"
	yellow = "\033[1;33m"
	blue   = "\033[0;34m"
	dim    = "\033[2m"
	nc     = "\033[0m"
)

func RunCleanup(args []string) {
	fs := flag.NewFlagSet("cleanup", flag.ExitOnError)
	mode := fs.String("mode", "safe", "Cleanup mode: safe or aggressive")
	force := fs.Bool("force", false, "Force-delete unmerged branches (aggressive mode)")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: sentei cleanup [options] [repo-path]\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	repoPath := "."
	if fs.NArg() > 0 {
		repoPath = fs.Arg(0)
	}

	m := cleanup.Mode(*mode)
	if m != cleanup.ModeSafe && m != cleanup.ModeAggressive {
		fmt.Fprintf(os.Stderr, "Unknown mode: %s (use safe or aggressive)\n", *mode)
		os.Exit(1)
	}

	opts := cleanup.Options{
		Mode:   m,
		Force:  *force,
		DryRun: *dryRun,
	}

	if opts.DryRun {
		fmt.Printf("%s(dry run)%s\n", dim, nc)
	}
	fmt.Println()

	runner := &git.GitRunner{}
	result := cleanup.Run(runner, repoPath, opts, printEvent)

	fmt.Println()
	for _, e := range result.Errors {
		fmt.Printf("%s⚠%s  %s: %s\n", yellow, nc, e.Step, e.Err)
	}

	if result.NonWtBranchesRemaining > 0 && opts.Mode == cleanup.ModeSafe {
		fmt.Printf("\n%sTip:%s %d local branch(es) are not checked out in any worktree.\n", blue, nc, result.NonWtBranchesRemaining)
		fmt.Printf("     Run %ssentei cleanup --mode=aggressive%s to remove them.\n", dim, nc)
	}
}

func printEvent(e cleanup.Event) {
	switch e.Level {
	case cleanup.LevelStep:
		fmt.Printf("%s→%s %s\n", blue, nc, e.Message)
	case cleanup.LevelInfo:
		fmt.Printf("%s✓%s %s\n", green, nc, e.Message)
	case cleanup.LevelWarn:
		fmt.Printf("%s⚠%s  %s\n", yellow, nc, e.Message)
	case cleanup.LevelDetail:
		fmt.Printf("  %s%s%s\n", dim, e.Message, nc)
	}
}
```

- [ ] **Step 2: Add subcommand dispatch to main.go**

Add before the existing `flag.Parse()` call at line 30 of `main.go`:

```go
if len(os.Args) > 1 && os.Args[1] == "cleanup" {
	cmd.RunCleanup(os.Args[2:])
	return
}
```

Add the import: `"github.com/bonboncinnabon/sentei/cmd"`

- [ ] **Step 3: Verify it compiles and runs**

Run: `cd ~/code/personal/sentei/main && go build -o sentei . && ./sentei cleanup --help`
Expected: Shows usage with --mode, --force, --dry-run flags

Run: `./sentei cleanup --dry-run ~/code/saaf/saaf-app/saaf-monorepo`
Expected: Shows dry-run output of cleanup operations

- [ ] **Step 4: Commit**

```bash
git add cmd/cleanup.go main.go
git commit -m "feat(cleanup): add sentei cleanup CLI subcommand"
```

---

### Task 9: TUI Integration

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/progress.go`
- Modify: `internal/tui/summary.go`

- [ ] **Step 1: Add cleanup fields to Model**

In `internal/tui/model.go`, add to the `Model` struct:

```go
cleanupResult *cleanup.Result
```

Add import: `"github.com/bonboncinnabon/sentei/internal/cleanup"`

- [ ] **Step 2: Add cleanup messages and chain to progress.go**

In `internal/tui/progress.go`, add:

```go
type cleanupCompleteMsg struct {
	Result cleanup.Result
}

func runCleanup(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		result := cleanup.Run(runner, repoPath, cleanup.Options{Mode: cleanup.ModeSafe}, func(cleanup.Event) {})
		return cleanupCompleteMsg{Result: result}
	}
}
```

Update `pruneCompleteMsg` handler to chain cleanup instead of going to summary:

```go
case pruneCompleteMsg:
	pruneErr := msg.Err
	m.pruneErr = &pruneErr
	return m, runCleanup(m.runner, m.repoPath)

case cleanupCompleteMsg:
	m.cleanupResult = &msg.Result
	m.view = summaryView
```

- [ ] **Step 3: Update summary view to show cleanup results**

In `internal/tui/summary.go`, after the prune status section, add cleanup results display:

```go
if m.cleanupResult != nil {
	r := m.cleanupResult
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Cleanup:"))
	b.WriteString("\n")
	if r.StaleRefsRemoved > 0 {
		b.WriteString(fmt.Sprintf("    %s Pruned %d remote ref(s)\n", styleSuccess.Render("v"), r.StaleRefsRemoved))
	}
	if r.ConfigDedupResult.Removed > 0 {
		b.WriteString(fmt.Sprintf("    %s Removed %d config duplicates\n", styleSuccess.Render("v"), r.ConfigDedupResult.Removed))
	}
	if r.GoneBranchesDeleted > 0 {
		b.WriteString(fmt.Sprintf("    %s Deleted %d branch(es) with gone upstream\n", styleSuccess.Render("v"), r.GoneBranchesDeleted))
	}
	if r.ConfigOrphanResult.Removed > 0 {
		b.WriteString(fmt.Sprintf("    %s Removed %d orphaned config section(s)\n", styleSuccess.Render("v"), r.ConfigOrphanResult.Removed))
	}
	if r.NonWtBranchesRemaining > 0 {
		b.WriteString("\n")
		b.WriteString(styleDim.Render(fmt.Sprintf("  Tip: %d local branch(es) not in any worktree.", r.NonWtBranchesRemaining)))
		b.WriteString("\n")
		b.WriteString(styleDim.Render("       Run `sentei cleanup --mode=aggressive` to remove them."))
		b.WriteString("\n")
	}
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd ~/code/personal/sentei/main && go build ./...`
Expected: No errors

- [ ] **Step 5: Manual test with playground**

Run: `cd ~/code/personal/sentei/main && go run . --playground`
Expected: Select and delete a worktree. Summary should show "Cleanup:" section after deletion.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/progress.go internal/tui/summary.go
git commit -m "feat(cleanup): integrate cleanup into TUI post-deletion flow"
```

---

### Task 10: Shell Wrapper and Final Verification

**Files:**
- Replace: `scripts/git-repo-cleanup.sh`

- [ ] **Step 1: Replace shell script with thin wrapper**

```bash
#!/bin/bash
if command -v sentei >/dev/null 2>&1; then
    exec sentei cleanup "$@"
else
    echo "sentei not found. Install: go install github.com/bonboncinnabon/sentei@latest" >&2
    exit 1
fi
```

- [ ] **Step 2: Run full test suite**

Run: `cd ~/code/personal/sentei/main && go test ./... -v`
Expected: All PASS

- [ ] **Step 3: Run go vet and fmt**

Run: `cd ~/code/personal/sentei/main && go vet ./... && gofmt -l .`
Expected: No issues

- [ ] **Step 4: Build and smoke test**

Run: `cd ~/code/personal/sentei/main && go build -o sentei .`

Test safe mode: `./sentei cleanup --dry-run ~/code/saaf/saaf-app/saaf-monorepo`
Test aggressive mode: `./sentei cleanup --mode=aggressive --dry-run ~/code/saaf/saaf-app/saaf-monorepo`
Test help: `./sentei cleanup --help`
Test default (TUI still works): `./sentei --version`

- [ ] **Step 5: Commit**

```bash
git add scripts/git-repo-cleanup.sh
git commit -m "chore: replace shell cleanup script with sentei wrapper"
```
