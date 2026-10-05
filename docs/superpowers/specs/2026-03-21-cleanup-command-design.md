# sentei cleanup — Design Spec

## Problem

Git repos using the bare-repo + worktree pattern accumulate cruft that degrades IDE performance:

1. **Config bloat**: Cursor/VS Code's GitHub PR extension appends duplicate `github-pr-owner-number` entries to `.git/config` (or `.bare/config`). The `vscode-merge-base` key also duplicates. A real-world repo had 920 lines in config with 403 duplicates.
2. **Stale remote refs**: Remote branches deleted on GitHub remain as local remote-tracking refs until explicitly pruned.
3. **Orphaned local branches**: Branches whose upstream is gone, or branches from old worktrees that were never cleaned up. A real-world repo had 920 local branches with only 5 active worktrees.
4. **Orphaned config sections**: `[branch "..."]` sections in git config for branches that no longer exist locally.

These compound — Cursor parses the bloated config on every git operation, enumerates thousands of branch refs, and the git view becomes unusable.

## CLI Interface

```
sentei cleanup [--mode=safe|aggressive] [--force] [--dry-run] [repo-path]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--mode` | `safe` | Cleanup mode (see Modes below) |
| `--force` | `false` | In aggressive mode, force-delete unmerged branches |
| `--dry-run` | `false` | Show what would happen without making changes |
| `repo-path` | `.` | Path to the git repository |

### Modes

**`safe`** — Conservative cleanup. Only removes things that are clearly stale:
- Deduplicate git config entries (remove exact key+value duplicates within each section, preserving legitimate multi-valued keys)
- Prune stale remote-tracking refs (`git fetch --prune`)
- Delete local branches whose upstream is gone (`git branch -d`)
- Purge `[branch "..."]` config sections for branches that no longer exist

**`aggressive`** — Everything in safe, plus:
- Delete all local branches not checked out in any worktree
- Skips unmerged branches unless `--force` is also set
- Protected branches (`main`, `master`, `develop`, `dev`) are never deleted

### Output

Colored terminal output matching sentei's existing style:
```
→ Pruning stale remote refs...
✓ Pruned 9 stale remote ref(s)
→ Deduplicating git config...
✓ Deduplicated config: removed 321 lines (920 → 599)
→ Deleting branches with gone upstream...
✓ Deleted 48 branch(es)
⚠ 2 branch(es) skipped (not fully merged)
→ Purging orphaned config sections...
✓ Removed 87 orphaned config sections (599 → 30 lines)
```

## Package Architecture

```
internal/cleanup/
├── cleanup.go       # Orchestrator: Options, Result, Run()
├── config.go        # DedupConfig(), PurgeOrphanedBranchConfigs()
├── branches.go      # DeleteGoneBranches(), DeleteNonWorktreeBranches()
├── refs.go          # PruneRemoteRefs()
├── cleanup_test.go  # Orchestrator integration tests
├── config_test.go   # Config operation unit tests
├── branches_test.go # Branch operation unit tests
├── refs_test.go     # Ref operation unit tests
└── testdata/        # Git config fixtures for tests
    ├── bloated.gitconfig
    ├── clean.gitconfig
    └── special-chars.gitconfig
```

### Key Types

```go
package cleanup

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
    OrphanedConfigsRemoved int
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

// Event is emitted during cleanup for progress reporting.
type Event struct {
    Step    string // e.g., "prune-refs", "dedup-config"
    Message string
    Level   EventLevel // Info, Warn, Detail
}

type EventLevel int

const (
    LevelStep EventLevel = iota
    LevelInfo
    LevelWarn
    LevelDetail
)
```

### Design Principles

- Each operation is a standalone function: `(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) -> (partial result, error)`
- Reuses the existing `git.CommandRunner` interface — all git calls are mockable
- Config file operations take a file path, read with `os.ReadFile`, write to a temp file, then atomically rename via `os.Rename` — prevents corruption on crash. A `.bak` copy is created before the first modification.
- No Bubble Tea dependency in the cleanup package — progress is reported via `func(Event)` callback. Since the pipeline is sequential, the callback is always called from a single goroutine — no concurrency safety requirement on the caller.
- Protected branches (from `git.IsProtectedBranch`) are never deleted regardless of mode

## Operation Pipeline

Operations have dependencies:

```
    ┌──────────────────────┐
    │ 1. Prune remote refs │
    └──────────┬───────────┘
    ┌──────────▼───────────┐
    │ 2. Dedup config      │
    └──────────┬───────────┘
    ┌──────────▼───────────┐
    │ 3. Gone branches     │
    └──────────┬───────────┘
    ┌──────────▼───────────┐
    │ 4. Non-wt branches   │
    │    (aggressive only) │
    └──────────┬───────────┘
    ┌──────────▼───────────┐
    │ 5. Purge orphaned    │
    │    config sections   │
    └──────────────────────┘
```

- All steps run sequentially to avoid concurrent config file access (`git fetch --prune` can modify the config file, so running config dedup in parallel would risk corruption)
- Step 2 runs after 1 because `fetch --prune` may modify the config file
- Step 3 depends on 1 (pruning reveals gone upstreams)
- Step 4 depends on 3 (avoid double-processing)
- Step 5 depends on 3+4 and 2 (needs final branch state and clean config base)

### Config Path Resolution

Uses `git rev-parse --git-common-dir` to find the shared git directory (works for both normal repos and bare+worktree setups), then appends `/config`. For a bare repo at `repo/.bare`, this resolves to `repo/.bare/config`. For a normal repo, it resolves to `repo/.git/config`.

### Error Handling

Each operation returns its own error. The orchestrator collects errors into `Result.Errors` but continues to the next operation — one failure should not prevent other independent cleanup steps. The caller decides how to present errors (the CLI prints warnings, the TUI shows them in the summary).

If a config file write fails, the original file is preserved (write-to-temp-then-rename pattern).

### Dry-Run Semantics

Dry-run applies uniformly to all operations:
- Git commands that would mutate state are replaced with their read-only equivalents (e.g., `git remote prune origin --dry-run` instead of `git fetch --prune`). Note: the dry-run prune only checks against already-fetched refs, so it may report fewer stale refs than a real run which fetches first. This is intentional — dry-run should not contact the network.
- Config file operations read and compute the diff but skip the `os.Rename` step
- Branch deletion commands are skipped entirely; the branch list is computed and reported but no `git branch -d/-D` is issued

### Config Dedup Strategy

Deduplication targets **exact key+value duplicates** within a section, not just key duplicates. This preserves legitimate multi-valued keys like `remote.origin.fetch` which can have multiple distinct values. Only lines where both the key and value are identical to a previous line in the same section are removed.

### Branch Deletion Strategy

For gone-upstream branches and non-worktree branches, the approach is try-and-record:
1. Attempt `git branch -d <branch>` (or `-D` if `--force`)
2. On success, record as deleted
3. On failure, record as skipped with the appropriate `SkipReason`

This avoids pre-checking merge status (which has ambiguous semantics — merged into what?) and handles all edge cases git itself would catch.

### Orchestrator

```go
func Run(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) Result {
    configPath, err := resolveConfigPath(runner, repoPath)
    // ...

    var result Result

    // Step 1: Prune stale remote refs
    if r, err := PruneRemoteRefs(runner, repoPath, opts, emit); err != nil {
        result.Errors = append(result.Errors, OperationError{Step: "prune-refs", Err: err})
    } else {
        result.StaleRefsRemoved = r
    }

    // Step 2: Dedup config (runs after prune — fetch may modify config file)
    if r, err := DedupConfig(configPath, opts, emit); err != nil {
        result.Errors = append(result.Errors, OperationError{Step: "dedup-config", Err: err})
    } else {
        result.ConfigDedupResult = r
    }

    // Step 3: Delete branches with gone upstream
    if r, err := DeleteGoneBranches(runner, repoPath, opts, emit); err != nil {
        result.Errors = append(result.Errors, OperationError{Step: "gone-branches", Err: err})
    } else {
        result.GoneBranchesDeleted = r.Deleted
        result.BranchesSkipped = append(result.BranchesSkipped, r.Skipped...)
    }

    // Step 4: Delete non-worktree branches (aggressive only)
    // Always count remaining non-worktree branches for the tip message
    if r, err := CleanNonWorktreeBranches(runner, repoPath, opts, emit); err != nil {
        result.Errors = append(result.Errors, OperationError{Step: "non-wt-branches", Err: err})
    } else {
        result.NonWtBranchesDeleted = r.Deleted
        result.NonWtBranchesRemaining = r.Remaining
        result.BranchesSkipped = append(result.BranchesSkipped, r.Skipped...)
    }

    // Step 5: Purge orphaned config sections (depends on final branch state)
    if r, err := PurgeOrphanedBranchConfigs(runner, repoPath, configPath, opts, emit); err != nil {
        result.Errors = append(result.Errors, OperationError{Step: "orphaned-configs", Err: err})
    } else {
        result.ConfigOrphanResult = r
    }

    return result
}
```

Note: `CleanNonWorktreeBranches` always counts non-worktree branches. In safe mode, it only counts (sets `Remaining`) without deleting. In aggressive mode, it deletes and sets both `Deleted` and `Remaining` (remaining = unmerged branches that were skipped without `--force`).

## Sentei TUI Integration

### Post-deletion auto-run (safe mode)

After worktree deletion and `git worktree prune`, the TUI automatically runs cleanup in safe mode. New message types in `tui/progress.go`:

```go
type cleanupCompleteMsg struct {
    Result cleanup.Result
    Err    error
}
```

Flow: `allDeletionsCompleteMsg` → `runPrune()` → `pruneCompleteMsg` → `runCleanup(safe)` → `cleanupCompleteMsg` → `summaryView`

### Summary view enhancement

The summary view gains a "Cleanup" section showing what was cleaned:

```
Cleanup:
  ✓ Pruned 3 remote refs
  ✓ Removed 12 orphaned config sections
  ✓ Deleted 5 branches with gone upstream

Tip: 47 local branches are not checked out in any worktree.
     Run `sentei cleanup --mode=aggressive` to remove them.
```

The tip only appears when there are non-worktree branches remaining after safe cleanup.

### Counting non-worktree branches

To display the tip, the cleanup orchestrator always counts non-worktree branches even in safe mode — it just doesn't delete them. This count is returned in `Result` as `NonWtBranchesRemaining int`.

## CLI Subcommand

`sentei cleanup` is handled via manual subcommand dispatch in `main.go`: before calling `flag.Parse()` for the default TUI command, check if `len(os.Args) > 1 && os.Args[1] == "cleanup"`. If so, strip `os.Args[0:1]` and dispatch to `cmd/cleanup.go` which uses its own `flag.FlagSet` for `--mode`, `--force`, `--dry-run`, and the positional repo path.

```go
// main.go — subcommand dispatch (before existing flag.Parse)
if len(os.Args) > 1 && os.Args[1] == "cleanup" {
    cmd.RunCleanup(os.Args[2:])
    return
}
```

```go
// cmd/cleanup.go
func RunCleanup(args []string) {
    fs := flag.NewFlagSet("cleanup", flag.ExitOnError)
    mode := fs.String("mode", "safe", "Cleanup mode: safe or aggressive")
    force := fs.Bool("force", false, "Force-delete unmerged branches")
    dryRun := fs.Bool("dry-run", false, "Show what would be done")
    fs.Parse(args)
    // ... resolve repo path from fs.Arg(0), build Options, call cleanup.Run()
}
```

This avoids adding a dependency on cobra/urfave while keeping the TUI's existing flag parsing intact. The cleanup package is decoupled from both the TUI and the CLI — `cmd/cleanup.go` is a thin adapter that formats `Event` callbacks to terminal output.

## Testing Strategy

### Unit tests

All tests use sentei's existing `CommandRunner` interface for mocking git commands. Config tests use temp files.

**`config_test.go`:** (table-driven)
- Dedup: clean config (no-op), exact key+value duplicates removed, distinct values for same key preserved (e.g., multi-valued `fetch` refspecs), multiple sections with mixed duplicates, empty file
- Orphan purge: branches exist (kept), branches gone (purged), special characters in branch names, empty sections
- Atomic write: verify temp-file-then-rename pattern, verify `.bak` creation
- Edge case: `[branch "refs/with/slashes"]`

**`branches_test.go`:**
- Gone-upstream: parse `git branch -vv` output with `gone` marker, handle `+` prefix for worktree-checkout branches, empty output
- Non-worktree: mock worktree list and branch list, verify only non-worktree branches deleted, protected branches skipped
- Force vs non-force: verify `-d` vs `-D` flag usage
- Protected branches: main/master/develop/dev never deleted

**`refs_test.go`:**
- Parse `git remote prune origin --dry-run` output: zero stale, some stale, error cases

**`cleanup_test.go`:**
- Orchestrator integration: mock runner returns canned output, verify `Result` aggregation
- Mode propagation: safe skips non-worktree deletion, aggressive includes it
- Dry-run: verify no mutating git commands or file writes are issued
- Error handling: individual operation failure recorded in `Result.Errors`, subsequent steps still execute

### Test fixtures

`testdata/` directory with sample git config files:
- `bloated.gitconfig`: realistic config with duplicate keys across multiple branch sections
- `clean.gitconfig`: already-clean config
- `special-chars.gitconfig`: branch names with slashes, dots, numbers

## Shell Wrapper

`scripts/git-repo-cleanup.sh` becomes a thin wrapper:

```bash
#!/bin/bash
if command -v sentei >/dev/null 2>&1; then
    exec sentei cleanup "$@"
else
    echo "sentei not found. Install: go install github.com/bonboncinnabon/sentei@latest" >&2
    exit 1
fi
```

All logic lives in Go. The script exists for ad-hoc use or aliasing (`alias git-cleanup='sentei cleanup'`).

## Known Limitations

- **Single remote**: Only prunes refs for `origin`. Repos with multiple remotes (e.g., `origin` + `upstream` in fork workflows) get partial cleanup.
- **Network dependency**: `git fetch --prune` contacts the remote. On slow connections or offline, this step will fail (gracefully — the error is recorded and subsequent steps continue).
- **Hardcoded protected branches**: `main`, `master`, `develop`, `dev`. No user extension mechanism in v1.

## Future Work

- User-configurable protected branches (via flag or config file)
- Multi-remote support
- `git maintenance` / fsmonitor configuration as part of repo optimization

## Out of Scope

- Cursor/VS Code settings optimization
- Preventing the config bloat at the source (that's a Cursor/VS Code extension bug)
- Remote branch deletion (only local refs and branches)
