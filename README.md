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

Which branch the worktree gets:

- a name that does not exist yet becomes a new branch from `--base`;
- an existing local branch is checked out as it is;
- a branch that only `origin` has is checked out as a local branch tracking it.

In the last two cases `--base` is not applied and the output says so.
`--merge-base` merges the base into the branch afterwards; in the interactive
flow that option starts unchecked for a branch that already exists. Branch and
base names that start with `-` are refused.

A run that could not create the worktree ends with `create failed: no worktree
was created`. A run that created it but failed a later step (a hook, a file
copy, an install) ends with `create completed with errors: the worktree exists
at <path>`. Both exit non-zero.

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

### Remove worktrees

```bash
sentei                                         # menu, then pick from the list
sentei remove --merged                         # open the list with matches selected
sentei remove --non-interactive --merged --stale 30d --dry-run
sentei remove --non-interactive --branch feature/x
```

Filters combine with OR:

- `--branch <name>`: the worktree of that branch. Repeat the flag for more.
  A named branch that has no worktree, is protected, or whose worktree holds
  the current directory stops the run before anything is removed.
- `--merged`: the default branch contains the branch's tip, and the branch
  either came in through a merge commit or was fast-forwarded in. A
  fast-forward is recognised from the branch's reflog, which must show that
  the branch moved after it was created: a branch still sitting where it was
  created from the default branch is not selected, however far the default
  branch has moved since, and neither is one whose reflog has expired or was
  never written. A pull or a reset moves a branch as a commit does, so a
  branch without commits of its own that was brought up to date with the
  default branch is selected. Squash and rebase merges are not recognised:
  they leave the branch's own commits outside the default branch.
- `--stale 30d` (also `w` and `m`): the last commit, by committer date, is
  older than that.
- `--all`: every worktree that is not protected.

A worktree is at risk when it is locked, has uncommitted changes, untracked
files or commits on no remote, or when its status could not be read. The TUI
shows those on a confirmation screen before anything is deleted and re-reads
the status of the others at the moment of deletion. The command line refuses
them without `--force`.

### Clean up a repository

```bash
sentei cleanup                                          # confirm, then run safe mode
sentei cleanup --non-interactive --mode safe --dry-run  # print what would happen
sentei cleanup --non-interactive --mode safe
```

Safe mode:

- prunes remote-tracking refs whose branch is gone from the remote;
- deletes local branches whose upstream is gone, when they are fully merged;
- removes config sections left behind by deleted branches;
- collapses duplicate config lines, only for a key whose values are all
  identical, so the effective value cannot change;
- prunes stale worktree metadata.

`--mode aggressive` also deletes local branches that have no worktree and are
fully merged. `--force` extends both deletions to branches that are not fully
merged. Protected branches and the default branch are never deleted.

Removing worktrees does not run any of this: a removal ends with a local
`git worktree prune` and nothing else.

### Clone or migrate a repository

sentei works on a container directory that holds the bare repository as
`.bare`, a `.git` pointer file, and one directory per worktree:

```
project/
├── .bare/        # the repository
├── .git          # "gitdir: ./.bare"
├── main/         # one worktree per branch
└── feature-x/
```

```bash
sentei clone --url git@github.com:you/project.git     # clone straight into the layout
sentei migrate /path/to/existing/clone                 # convert an ordinary clone in place
```

`clone` keeps only the default branch as a local branch, as `git clone` does;
the others are available as `origin/<branch>` and `sentei create --branch
<name>` checks one out. `migrate` renames `.git` to `.bare` and moves the
working tree into a worktree named after the current branch. It copies and
deletes nothing, checks everything before the first change, and rolls back if
a step fails; see "Scripts and agents" for what it refuses.

### Configuration

Configuration is optional. sentei merges three layers, each overriding the
one before it:

1. built-in defaults (run `sentei ecosystems` to see them);
2. a global file, `$XDG_CONFIG_HOME/sentei/config.yaml` (`~/.config` when the
   variable is unset);
3. a per-repository `.sentei.yaml`. With the `.bare` layout it sits in the
   container, next to `.bare`. A plain bare repository keeps it inside
   itself.

```yaml
ecosystems:
  - name: pnpm                 # override a built-in by name, or add your own
    install:
      command: pnpm install --frozen-lockfile --prefer-offline
    env_files: [.env, .env.local]
  - name: docs
    detect:
      files: [mkdocs.yml]
    env_files: []              # an explicit empty list clears the inherited one
  - name: go
    install:
      command: ""              # an explicit empty command clears the inherited one
  - name: cargo
    enabled: false
```

An install command runs once, at the root of the new worktree. `command: ""`
turns an inherited install off and keeps the rest of the ecosystem, such as
its env files; leaving the key out inherits the command. A key sentei does
not know is an error that names the file, the line and the key, so a typo
cannot silently switch a setting off.

### Git behaviour

sentei runs git non-interactively. git never prompts on the terminal, so an
HTTPS remote needs a credential helper and an SSH remote needs a loaded key;
otherwise the command fails with a message saying so. git output is read with
`LC_ALL=C`, an inherited `GIT_DIR` is ignored, and a git command that has not
finished after an hour is stopped. Hooks that git starts inherit that
environment.

ssh runs in batch mode (`GIT_SSH_COMMAND="ssh -oBatchMode=yes"`), so it fails
instead of asking you to confirm a host key or type a key's passphrase. When
that happens, connect to the host once by hand (`ssh -T git@github.com`) to
accept its key, or load the key into an agent with `ssh-add`, and run the
command again. If you have configured your own ssh command with `GIT_SSH`,
`GIT_SSH_COMMAND` or `core.sshCommand`, sentei runs it unchanged: add
`-oBatchMode=yes` to it if it should never prompt. `sentei clone` also leaves
ssh alone when an `includeIf` section in your git config brings in a file that
sets `core.sshCommand`: the include may apply to the new repository.

### Layouts and platforms

- sentei works on any repository whose shared git directory is bare, whatever
  that directory is called: the `.bare` layout above, or a plain bare
  repository such as `project.git` with its worktrees inside it or beside it.
  Start it in the bare directory, in the container, or in any worktree, or
  pass one of them as the path: every command behaves the same from each. New
  worktrees are created where `.sentei.yaml` is read. That is the container
  when the bare directory is named `.bare` or a `.git` pointer file beside it
  leads to it, and otherwise the bare directory itself, also when the existing
  worktrees sit beside it. A container needs its `.git` pointer file; without
  it sentei stops and prints the command that recreates it. With git's
  `safe.bareRepository=explicit` only a layout with that pointer file works,
  started in the container or in a worktree: git refuses a bare directory it
  is started in, and sentei says so instead of entering it. An ordinary clone is not such a layout, even
  when it has linked worktrees; see `migrate` above.
- macOS and Linux are tested on every change. Windows binaries are published
  but untested, and dependency installs run through `sh -c`, so they need a
  POSIX shell on the `PATH`.

### Scripts and agents

Every command runs without the TUI when given `--non-interactive` (or `--yes`,
`-y`):

```bash
sentei create  --non-interactive --branch feature/x --base main [--copy-env] [--ecosystems pnpm] [repo]
sentei remove  --non-interactive [--branch <name>] [--merged] [--stale 30d] [--all] [--dry-run] [--force] [repo]
sentei cleanup --non-interactive --mode safe [--dry-run] [--force] [repo]
sentei clone   --non-interactive --url <url> [--name <dir>]
sentei migrate --non-interactive <repo>
```

What a caller can rely on:

- **Flags come before the repository path.** An argument after the path is an
  error; it is never silently ignored.
- **The repository path defaults to the current directory** (`migrate` needs it
  spelled out). It may be the repository container or any worktree inside it.
  `create` always places the new worktree at the layout's root, whichever
  worktree it is run from (see "Layouts and platforms").
- **The exit status is the result.** Non-zero means something failed: a bad
  flag, a config file that does not load, a worktree that could not be
  removed, a cleanup step that errored, a failed setup or install step.
- **Nothing waits for input.** git never prompts for credentials, and a
  config file that fails to load stops the run before anything is created.
- **`--non-interactive` does not override a safety.** `remove` refuses the
  whole run, and names the worktrees, when a match is locked, has uncommitted,
  untracked or unpushed work, or has a status that could not be read. `--force`
  removes those too. Protected branches and the worktree holding the current
  directory are never removed, with or without `--force`.
- **`--dry-run` changes nothing.** It prints what `remove` or `cleanup` would
  do and leaves locks in place. It exits 0 and marks the matches the real run
  would refuse without `--force`.
- **`migrate` converts in place and never copies or deletes.** `.git` becomes
  `.bare` and the working tree moves, with its index, into a worktree named
  after the current branch, so staged, unstaged, untracked and ignored files,
  stashes, remotes, config and hooks all stay. Linked worktrees stay where
  they are and are relinked, submodules keep their commits and local changes,
  a sparse checkout stays sparse, and a remote given as a relative path is
  rewritten to the absolute path it meant. It refuses before touching
  anything when the conversion cannot be done safely (detached or unborn
  `HEAD`, a merge or rebase in progress, `core.worktree`, a config include
  that points into the working tree, a directory already named like the
  worktree, a linked worktree whose directory is missing, a submodule that
  has linked worktrees of its own), and a step that fails midway is rolled
  back to the original layout. Two things follow from the tree sitting one
  directory further down, and need fixing by hand: a relative symlink that
  points outside the repository now points somewhere else, and files that
  record absolute paths into the tree, such as a Python virtual environment,
  have to be recreated.
- **`--copy-env` is independent of `--ecosystems`.** The files come from the
  ecosystems detected in the source worktree; `--ecosystems` only selects
  which installs run.

### Upgrading from 1.x

- **Integrations are gone.** `sentei integrations`, the menu entry and the
  `integrations_enabled` config key are removed. sentei no longer builds or
  removes code indexes; existing `.code-review-graph/` and `.cocoindex_code/`
  folders are left where they are.
- **`migrate` converts in place.** It makes no backup because it copies and
  deletes nothing, so `--delete-backup` is removed. Submodules, linked
  worktrees and sparse checkouts are converted with the rest; "Scripts and
  agents" lists what is still refused. The working tree ends up one directory
  further down, so a relative symlink that points outside the repository, or
  a Python virtual environment and other files that record absolute paths,
  need fixing afterwards.
- **Config is strict.** An unknown key is an error, and a config file that
  fails to load stops the run. Installs run once at the root of the new
  worktree; the per-workspace install keys are accepted and ignored.
- **`remove` on the command line.** A locked worktree needs `--force`.
  `--non-interactive` no longer needs `--force` to run. An argument after the
  repository path is an error. Worktree age is the committer date.
- **Removing worktrees no longer runs cleanup.** The TUI ends a removal with
  `git worktree prune` only; run `sentei cleanup` for branches and config.
- **`clone` keeps only the default branch locally.** Other branches are
  `origin/<branch>` until `sentei create --branch <name>` checks one out.
- **git never prompts.** Use a credential helper or an SSH key.
- **`sentei.json` is no longer written.** An old one in the repository can be
  deleted.

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
| `q` / `Ctrl+C` | Quit (press again to quit while an operation is running) |

### Status Indicators

| Indicator | Meaning |
|-----------|---------|
| `[ok]` | Clean: no uncommitted changes |
| `[~]` | Dirty: has uncommitted changes |
| `[!]` | Has untracked files |
| `[^]` | Commits on no remote (shown on the confirmation screen) |
| `[?]` | Status could not be read (treated as at risk) |
| `[L]` | Locked |
| `[P]` | Protected branch (cannot be deleted) |

Protected branches: `main`, `master`, `develop`, `dev`, and the repository's
default branch. The worktree sentei was launched from is protected too.

## License

MIT
