# Repo Operations

**Sub-project 3 of 3** for the sentei expansion from worktree cleanup tool to full worktree lifecycle manager.

**Scope:** Context detection (bare/non-bare/no repo), adaptive menu, create repo (with optional GitHub publish), clone repo as bare, migrate existing repo to bare, re-launch sentei at new repo after operations.

**Depends on:** Sub-project 1 (config, ecosystem, integration) and sub-project 2 (TUI restructure, creator pipeline, menu system).

---

## Architecture Overview

### Context Detection

On startup, sentei detects the environment before launching the TUI:

```go
type RepoContext int
const (
    ContextBareRepo    RepoContext = iota // bare repo with worktrees — full menu
    ContextNonBareRepo                    // regular git repo — offer migrate
    ContextNoRepo                         // not in a git repo — offer create/clone
)
```

Detection logic:
1. `git rev-parse --is-bare-repository` → if `"true"`, `ContextBareRepo`
2. `git rev-parse --git-dir` → if succeeds, `ContextNonBareRepo`
3. Otherwise, `ContextNoRepo`

Note: sentei's bare repo structure uses a `.bare/` directory with a `.git` pointer file. `--is-bare-repository` returns `false` in a worktree of such a setup, but `--git-common-dir` resolves to `.bare`. For detection purposes, check if `.bare` directory exists at repo root as an additional signal for `ContextBareRepo` when invoked from inside a worktree.

### Adaptive Menu

The menu items change based on detected context:

**ContextBareRepo** (existing SP2 menu, unchanged):
```
  > Create new worktree
    Remove worktrees              5 available
    Cleanup                       safe mode
```

**ContextNoRepo:**
```
  > Create new repository
    Clone repository as bare
```

**ContextNonBareRepo:**
```
  > Migrate to bare repository
    Clone repository as bare
    Create new repository
```

Only relevant items shown — no grayed-out options.

### New Package

`internal/repo/` — three operation pipelines (create, clone, migrate). Event-driven with `emit func(Event)` callback, same pattern as `internal/creator/` and `internal/cleanup/`.

### Re-launch After Operations

After create, clone, or migrate completes, the summary view offers "open in sentei" which uses `tea.ExecProcess` to replace the current sentei process with a new instance pointed at the new/migrated repo. This provides a seamless transition to the bare repo menu without internal state management complexity.

---

## Create Repository

### Input View (repoNameView)

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

- Location defaults to CWD
- Validation: name can't contain spaces, path must exist, `location/name` can't already exist
- Inline error on validation failure

### Options View (repoOptionsView)

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

**Progressive disclosure:** GitHub sub-options (Visibility, Description) only appear when "Publish to GitHub" is checked. When unchecked, they collapse.

**GitHub status detection:** On view entry, check `gh auth status`. Show:
- `authenticated ●` (green) — gh installed and authenticated
- `not authenticated ✗` (red) — gh installed but not logged in. Toggle disabled.
- `gh not found ✗` (red) — gh not installed. Toggle disabled.

**Visibility** cycles between `private` / `public` on space when focused. Default: `private`.

**Description** is optional text input.

### Pipeline

```go
type CreateOptions struct {
    Name          string
    Location      string
    PublishGitHub bool
    Visibility    string // "private" or "public"
    Description   string
}

func Create(shell git.ShellRunner, opts CreateOptions, emit func(Event)) CreateResult
```

**Phase 1: Setup (sequential)**
1. Create directory at `location/name`
2. `git init --bare .bare` inside the new directory
3. Create `.git` pointer file containing `gitdir: .bare`
4. `git config remote.origin.fetch "+refs/heads/*:refs/remotes/origin/*"` (from `.bare`)
5. Create main worktree: `git worktree add main -b main`
6. Create `README.md` in main worktree with repo name as heading
7. `git -C main add -A && git -C main commit -m "Initial commit"`

**Phase 2: GitHub (sequential, skipped if `PublishGitHub` is false)**
1. `gh repo create <name> --<visibility> --description "<desc>" --source main --push`
2. Configure SSH remote: `git -C .bare remote set-url origin git@github.com:<user>/<name>.git`
   - `<user>` from `gh api user --jq .login`
3. `git -C main push -u origin main`
4. `git -C .bare remote set-head origin main`

### Summary View

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

`enter` → `tea.ExecProcess("sentei", []string{newRepoPath})` to re-launch sentei at the new repo.

---

## Clone Repository

### Input View (cloneInputView)

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

- Directory name auto-derives from URL: strip `.git` suffix, take last path segment
- Updates in real-time as user types the URL
- Tab to override directory name
- No options screen — straight to progress after input

**URL-to-name derivation:**
- `git@github.com:user/repo.git` → `repo`
- `https://github.com/user/repo.git` → `repo`
- `https://github.com/user/repo` → `repo`

### Pipeline

```go
type CloneOptions struct {
    URL      string
    Location string
    Name     string
}

func Clone(shell git.ShellRunner, opts CloneOptions, emit func(Event)) CloneResult
```

**Phase 1: Clone (sequential)**
1. `git clone --bare <url> <location>/<name>/.bare`

**Phase 2: Structure (sequential)**
1. Create `.git` pointer file containing `gitdir: .bare`
2. `git config remote.origin.fetch "+refs/heads/*:refs/remotes/origin/*"` (from `.bare`)

**Phase 3: Worktree (sequential)**
1. Detect default branch: `git symbolic-ref refs/remotes/origin/HEAD` → strip `refs/remotes/origin/`
   - Fallback: try `main`, then `master`
2. `git worktree add <branch> <branch>` (creates worktree named after the branch)
3. `git -C <branch> branch --set-upstream-to=origin/<branch>`

### Summary View

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

---

## Migrate Repository

### Confirm View (migrateConfirmView)

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

If uncommitted changes:
```
  Status            ⚠ uncommitted changes

  ⚠ Uncommitted changes will be preserved in the backup
    but not in the new worktree
```

### Pipeline

```go
type MigrateOptions struct {
    RepoPath string
}

type MigrateResult struct {
    BareRoot     string
    WorktreePath string
    BackupPath   string
    Phases       []Phase
}

func Migrate(shell git.ShellRunner, runner git.CommandRunner, opts MigrateOptions, emit func(Event)) MigrateResult
```

**Phase 1: Validate (sequential)**
1. Verify git repository
2. Check `git status --porcelain` for uncommitted changes → emit warning event if dirty

**Phase 2: Backup (sequential)**
1. Determine backup path: `<repo>_backup_<YYYYMMDD_HHMMSS>`
2. Copy entire repo directory to backup path
3. Emit event with backup path and size

**Phase 3: Migrate (sequential)**
1. `git clone --bare .git .bare` (clones the repo's git data as a proper bare repo)
2. Remove original `.git/` directory
3. Create `.git` pointer file containing `gitdir: .bare`
4. `git config remote.origin.fetch "+refs/heads/*:refs/remotes/origin/*"` (from `.bare`)
5. Detect current branch: `git branch --show-current`
6. Create worktree for current branch: `git worktree add <branch>`

**Phase 4: Copy (sequential, best-effort)**
1. Copy from backup to new worktree (skip missing, log warnings):
   - `.env*` files
   - `node_modules/`, `vendor/`, `build/`, `dist/`
   - `.vscode/`, `.idea/`

### Summary View with Backup Cleanup

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

- `y` → delete backup dir, then re-launch sentei at the migrated repo
- `n` → keep backup, re-launch sentei at the migrated repo
- `q` → quit (show `cd` path)

Backup size calculated via `du -sh` or directory walk.

---

## Re-launch Mechanism

After any repo operation (create, clone, migrate), the summary view offers re-launch:

```go
// In the summary update handler:
case key.Matches(msg, keys.Confirm):
    senteiPath, err := os.Executable()
    if err != nil {
        senteiPath = "sentei" // fallback to PATH lookup
    }
    return m, tea.ExecProcess(senteiPath, []string{newRepoPath}, tea.WithEnv(os.Environ()))
```

`tea.ExecProcess` replaces the current process — no parent process left running. Sentei starts fresh with context detection finding `ContextBareRepo`, showing the full menu.

For migrate: the repo path doesn't change (same directory, restructured internals), so the re-launch just works at the same path.

---

## Shared Types

```go
// internal/repo/repo.go

type Event struct {
    Phase   string
    Step    string
    Status  StepStatus
    Message string
    Error   error
}

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
```

Note: These types mirror `internal/creator/` types. If the duplication feels wrong, extract a shared `internal/pipeline/` package. But per YAGNI, keep them separate until a third consumer appears. The types are small and stable.

---

## Error Handling

**Create repo:**
- Directory already exists → abort with inline validation error
- `git init --bare` fails → abort (Phase 1 failure)
- `gh repo create` fails → Phase 2 failure, local repo still usable. Show error, summary still shows local path.
- Push fails → Phase 2 failure, same treatment

**Clone repo:**
- Invalid URL / network error → Phase 1 failure, abort
- Default branch detection fails → fallback to `main`, warn

**Migrate repo:**
- Not a git repo → abort before pipeline starts (context detection handles this)
- Backup fails (disk space) → abort before migration
- Migration fails mid-way → backup exists, show rollback instructions:
  ```
  ✗ Migration failed: <error>

  Your original repo is backed up at:
    /path/to/backup

  To restore: rm -rf /path/to/repo && mv /path/to/backup /path/to/repo
  ```
- Copy phase failures → warnings only, non-critical

---

## Testing Strategy

### Unit Tests

**Repo package:**
- Create: successful local-only, successful with GitHub, gh not authenticated (skip GitHub), dir already exists (abort), gh user lookup
- Clone: successful, invalid URL, default branch detection (main, master, fallback), URL-to-name derivation
- Migrate: successful, uncommitted changes (warning + continue), backup creation verified, backup cleanup, mid-migration failure (rollback instructions)

**TUI views:**
- Context detection: bare repo, non-bare repo, no repo, worktree-inside-bare-repo
- Menu item rendering per context
- GitHub status detection (authenticated, not authenticated, not found)
- Progressive disclosure toggle (GitHub sub-options show/hide)
- URL-to-name derivation updates in real-time
- Backup cleanup prompt (y/n/q)

### E2E Tests

- Create: temp dir, run create pipeline, verify bare structure + `.git` pointer + main worktree + README
- Create with GitHub: mock gh commands, verify correct command sequence
- Clone: create temp origin repo, run clone, verify bare structure + worktree + tracking branch
- Migrate: create temp regular repo, run migrate, verify bare structure + backup exists + files copied
- Migrate cleanup: verify backup deleted after confirmation

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/repo/repo.go` | Create | Shared types (Event, StepStatus, StepResult, Phase), context detection |
| `internal/repo/create.go` | Create | Create repo pipeline |
| `internal/repo/clone.go` | Create | Clone repo pipeline |
| `internal/repo/migrate.go` | Create | Migrate repo pipeline |
| `internal/repo/create_test.go` | Create | Create pipeline tests |
| `internal/repo/clone_test.go` | Create | Clone pipeline tests |
| `internal/repo/migrate_test.go` | Create | Migrate pipeline tests |
| `internal/tui/model.go` | Modify | Add repo view states, repoState struct, context field |
| `internal/tui/menu.go` | Modify | Adapt menu items based on RepoContext |
| `internal/tui/repo_name.go` | Create | Create repo name input view |
| `internal/tui/repo_options.go` | Create | Create repo options with GitHub disclosure |
| `internal/tui/repo_progress.go` | Create | Repo operation progress (shared by create/clone/migrate) |
| `internal/tui/repo_summary.go` | Create | Repo operation summary with re-launch |
| `internal/tui/clone_input.go` | Create | Clone URL + name input view |
| `internal/tui/migrate_confirm.go` | Create | Migration confirmation view |
| `internal/tui/migrate_summary.go` | Create | Migration summary with backup cleanup |
| `main.go` | Modify | Context detection, pass context to TUI |

---

## View States (added)

```
repoNameView         → create repo name/location input
repoOptionsView      → create repo options with GitHub toggle
repoProgressView     → shared progress for create/clone/migrate
repoSummaryView      → shared summary for create/clone with re-launch
cloneInputView       → clone URL/name input
migrateConfirmView   → migration confirmation
migrateProgressView  → migration progress (reuses phased pattern)
migrateSummaryView   → migration summary with backup cleanup + re-launch
```
