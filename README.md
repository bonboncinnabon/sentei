# sentei

[![codecov](https://codecov.io/gh/bonboncinnabon/sentei/branch/main/graph/badge.svg)](https://codecov.io/gh/bonboncinnabon/sentei)

A TUI tool for cleaning up stale git worktrees. Scan, select, and bulk-delete worktrees with parallel execution and clear progress feedback.

## Features

- Interactive list with metadata (last commit date, branch, status)
- Multi-select with keyboard navigation
- Parallel deletion with real-time progress
- Safety: confirmation dialogs, warnings for dirty worktrees, branch protection
- Sorting, filtering, and dry-run mode

## Installation

### go install

```bash
go install github.com/bonboncinnabon/sentei@latest
```

### Build from source

```bash
git clone https://github.com/bonboncinnabon/sentei.git
cd sentei
go build -ldflags "-X main.version=$(git describe --tags --always)" -o sentei .
```

## Usage

Run inside a git repository with worktrees:

```bash
sentei                          # current directory
sentei /path/to/bare/repo       # specify repo path
sentei --dry-run                # print summary, no interactive TUI
sentei --version                # print version and exit
sentei --playground             # launch with a temporary test repo
```

### Create a worktree

Use the interactive create flow, or provide the branch and base for a
non-interactive run:

```bash
sentei create
sentei create --branch feature/my-change --base main --non-interactive
```

Repositories can declare private files that belong in every new worktree.
Keep the source file in the bare-repository container, outside committed
worktrees, and add rules to the container's `.sentei.yaml`:

```yaml
worktree_files:
  - source: .sentei/private/codex-config.toml
    destination: .codex/config.toml
  - source: .sentei/private/tool.json
    destination: .tool/config.json
    overwrite: true
```

`source` is relative to the bare-repository container. `destination` is
relative to each newly created worktree. Missing destination directories are
created automatically. Existing destinations are preserved unless
`overwrite: true` is explicit.

Both paths must be non-empty relative paths without `..` components. Sentei
rejects source symlinks that escape the container and any destination symlink.
A missing source or failed copy is reported as a failed Setup step; Sentei
keeps the created worktree and continues the independent dependency steps. File contents are never included in diagnostics.

`worktree_files` is automatic repository setup. Ecosystem `env_files` remains
an independent, opt-in create option controlled by `--copy-env`.

### Scripts and agents

Every command runs without the TUI when given `--non-interactive` (or `--yes`,
`-y`):

```bash
sentei create  --non-interactive --branch feature/x --base main [--copy-env] [--ecosystems pnpm] [repo]
sentei remove  --non-interactive --merged [--stale 30d] [--all] [--dry-run] [--force] [repo]
sentei cleanup --non-interactive --mode safe [--dry-run] [--force] [repo]
sentei clone   --non-interactive --url <url> [--name <dir>]
sentei migrate --non-interactive [--delete-backup] <repo>
```

What a caller can rely on:

- **Flags come before the repository path.** An argument after the path is an
  error; it is never silently ignored.
- **The repository path defaults to the current directory** (`migrate` needs it
  spelled out). It may be the repository container or any worktree inside it.
  `create` always places the new worktree beside the existing ones.
- **The exit status is the result.** Non-zero means something failed: a bad
  flag, a worktree that could not be removed, a cleanup step that errored, a
  failed setup or install step.
- **`--non-interactive` does not override a safety.** `remove` refuses the
  whole run, and names the worktrees, when a match has uncommitted, untracked
  or unpushed work, or a status that could not be read. `--force` removes
  those too. Protected branches and the worktree holding the current
  directory are never removed, with or without `--force`.
- **`--dry-run` changes nothing.** It prints what `remove` or `cleanup` would
  do and leaves locks in place.
- **`--copy-env` is independent of `--ecosystems`.** The files come from the
  ecosystems detected in the source worktree; `--ecosystems` only selects
  which installs run.

### CLI Flags

| Flag | Description |
|------|-------------|
| `--version` | Print version and exit |
| `--dry-run` | Print worktree summary to stdout and exit |
| `--playground` | Create a temporary test repo with sample worktrees |

### Key Bindings

| Key | Action |
|-----|--------|
| `j` / `k` / arrows | Navigate up/down |
| `PgUp` / `PgDn` | Page up/down |
| `Space` | Toggle selection |
| `a` | Select/deselect all |
| `s` | Cycle sort (age, branch) |
| `S` | Reverse sort direction |
| `/` | Filter by branch name |
| `Enter` | Confirm deletion of selected |
| `y` / `n` | Yes/no in confirmation dialog |
| `Esc` | Go back / clear filter |
| `q` / `Ctrl+C` | Quit |

### Status Indicators

| Indicator | Meaning |
|-----------|---------|
| `[ok]` | Clean — no uncommitted changes |
| `[~]` | Dirty — has uncommitted changes |
| `[!]` | Has untracked files |
| `[?]` | Status could not be read (treated as at risk) |
| `[L]` | Locked |
| `[P]` | Protected branch (cannot be deleted) |

Protected branches: `main`, `master`, `develop`, `dev`, and the repository's
default branch. The worktree sentei was launched from is protected too.

## License

MIT
