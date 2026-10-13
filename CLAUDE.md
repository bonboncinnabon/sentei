# sentei: Git Worktree Cleanup Tool

## Project Overview

sentei manages git worktrees in bare-repository layouts (a container directory holding `.bare`, a `.git` pointer file and one directory per worktree). It has an interactive TUI and a non-interactive CLI with the same commands: remove worktrees, create a worktree, clean up branches and config, clone into the layout, migrate an ordinary clone into it. README.md, section "Scripts and agents", is the contract for the non-interactive CLI.

## Tech Stack

- **Language**: Go 1.25 (see `go.mod`)
- **TUI Framework**: Bubble Tea v2, `charm.land/bubbletea/v2` (Elm architecture)
- **UI Components**: Bubbles v2, `charm.land/bubbles/v2`
- **Styling**: Lip Gloss v2, `charm.land/lipgloss/v2`
- **CLI**: the standard `flag` package; no command framework

## Development Guidelines

### Code Style

- Follow standard Go conventions and idioms
- Use `gofmt` for formatting (should be automatic)
- Keep functions focused and single-purpose
- Prefer explicit error handling over panic
- Use meaningful variable names (no single-letter except for standard Go idioms like `i` in loops)

### Error Handling

- All errors must be handled explicitly
- Error messages should:
  - Clearly state what went wrong
  - Suggest a fix or next step
  - Not expose internal implementation details
- Use `fmt.Errorf` with `%w` for error wrapping to maintain error chains

### Testing

- Write tests for all git parsing logic
- Test edge cases documented in PRD section 9.1
- Use table-driven tests for multiple scenarios
- **Never use `time.Sleep` in tests** — always use condition-based waiting (teatest.WaitFor, channels, sync primitives). A test that passes by timing luck is not a passing test.
- For TUI E2E tests with teatest, use `teatest.WaitFor(t, tm.Output(), condition)` to poll for expected output before sending the next action

### Git Commands

All git calls go through `git.GitRunner` (`internal/git/commands.go`). It runs `git -C <dir>` with `LC_ALL=C`, `GIT_TERMINAL_PROMPT=0` and `GIT_OPTIONAL_LOCKS=0`, ssh in batch mode unless the user configured their own ssh command, no stdin, without an inherited `GIT_DIR`, and with a timeout. Other child processes get the same cleaned environment from `git.Environ`. Under `safe.bareRepository=explicit` git refuses a bare directory it is only pointed into, so name it with `--git-dir=` when asking about the shared directory. Never call `exec.Command("git", ...)` directly, and never parse translated message text when a plumbing command gives the same answer.

The commands the core flows rely on:

```bash
# List worktrees
git worktree list --porcelain

# Last commit date (committer date) and title
git -C <path> log -1 --format=%ci
git -C <path> log -1 --format=%s

# Status, independent of the user's status.showUntrackedFiles
git -C <path> status --porcelain --untracked-files=normal

# Remove a worktree; a locked one takes --force twice and is never unlocked first
git worktree remove --force <path>
git worktree remove --force --force <path>

# Clean metadata
git worktree prune
```

A worktree is "at risk" when it has uncommitted changes, untracked files, commits on no remote, a lock, or a status that could not be read (`worktree.AtRisk`). Removing an at-risk worktree needs the confirmation screen in the TUI or `--force` in the CLI.

### Project Structure

```
sentei/
├── main.go                # Entry point: dispatch, flag parsing for interactive launches
├── cmd/                   # Non-interactive commands (remove, create, cleanup, clone, migrate, ecosystems)
├── internal/
│   ├── cli/              # Command registry and global flags
│   ├── git/              # GitRunner, worktree list parsing, branch protection, path helpers
│   ├── worktree/         # Enrichment, at-risk rules, parallel deletion, display helpers
│   ├── cleanup/          # Stale refs, gone branches, orphaned config sections
│   ├── creator/          # Create a worktree: add, env files, worktree files, installs
│   ├── repo/             # Create, clone and migrate repositories into the layout
│   ├── config/           # Embedded, global and per-repo config (.sentei.yaml)
│   ├── ecosystem/        # Ecosystem detection for installs and env files
│   ├── worktreefile/     # Private files copied into new worktrees
│   ├── progress/         # Plan, step and event model shared by CLI and TUI
│   ├── tui/              # Bubble Tea model, views and flows
│   ├── dryrun/           # `sentei --dry-run` table
│   └── playground/       # Temporary demo repository
├── openspec/              # Behaviour specs (openspec/specs) and change proposals
└── docs/                  # PRD and design notes
```

Keep internal packages truly internal - they are implementation details.

## Key Design Principles

### Safety First

- Never delete without explicit confirmation in interactive mode
- Clearly warn about uncommitted changes and untracked files
- Prevent deletion of current working directory
- Status badges are text: `[ok]` clean, `[~]` dirty, `[!]` untracked files, `[^]` unpushed commits, `[?]` status unreadable, `[L]` locked, `[P]` protected
- Never run a built `sentei` binary or `go run .` from inside this repository without naming a throwaway repository: with no path it acts on the repository that holds the working directory. Tests that run sentei or git build their own repository under `t.TempDir()` and pass it explicitly

### Performance

- Parallelize metadata enrichment (target: <5s for 50 worktrees)
- Parallelize deletion operations
- Keep initial scan fast (<2s for 50 worktrees)

### User Experience

- Zero configuration required for basic usage
- Vim-style keyboard shortcuts (j/k) plus arrow keys
- Clear, helpful error messages
- Show progress for long operations

## Common Tasks

### Adding New Git Metadata

1. Add field to `Worktree` struct
2. Add git command to fetch the data
3. Update enrichment logic to populate it (ensure parallelization)
4. Update TUI to display it

### Modifying TUI

1. Update model state in `internal/tui/model.go`
2. Handle messages/events in Update method
3. Render changes in View method
4. Update key bindings if needed

### Testing Git Parsing

Create test fixtures with sample `git worktree list --porcelain` output and verify parsing.

## Edge Cases to Handle

Refer to PRD section 9.1 for complete list. Key ones:

- Not in a git repository
- No worktrees found
- Worktree directory missing (prunable)
- Locked worktrees
- Worktree is current directory
- Permission denied

## Before Committing

- `golangci-lint fmt ./...`
- `make lint` (golangci-lint, must report 0 issues; the pre-commit hook runs it too)
- `make test` (`go test -race ./...`)
- `go mod tidy -diff` prints nothing
- Commit messages follow Conventional Commits; the commit-msg hook and CI enforce it
- Try behaviour changes with a built binary against a throwaway repository

## Dependencies

Use `go get` to add dependencies. Keep dependencies minimal and well-maintained.

Current major dependencies:
- charm.land/bubbletea/v2 (TUI framework)
- charm.land/bubbles/v2 (UI components)
- charm.land/lipgloss/v2 (styling)
- github.com/charmbracelet/x/ansi (width-aware truncation and stripping)

## Design

When working on TUI design, styling, or visual changes, reference `.impeccable.md` for design context, brand personality, aesthetic direction, and design principles.

## Documentation

- Keep PRD.md as source of truth for requirements
- Update README.md, including "Scripts and agents", when the CLI interface changes
- When a change alters behaviour that a spec under `openspec/specs` states, update that spec
- Document all exported functions and types with Go doc comments
