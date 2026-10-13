## Purpose
Covers the --stale, --merged, and --all filter flags for the remove command, including duration parsing, composability, and command scoping.

## Requirements

### Requirement: --stale flag filters worktrees by last commit age
The `remove` command SHALL accept a `--stale <duration>` flag that pre-selects worktrees whose last commit is older than the specified duration.

#### Scenario: Stale filter with TUI
- **WHEN** the user runs `sentei remove --stale 30d`
- **THEN** the TUI SHALL display the worktree list with worktrees whose last commit is older than 30 days pre-selected

#### Scenario: Stale filter duration formats
- **WHEN** the user provides `--stale 7d`, `--stale 2w`, or `--stale 3m`
- **THEN** the system SHALL parse the duration as days, weeks, or months respectively

#### Scenario: Invalid stale duration
- **WHEN** the user provides `--stale abc`
- **THEN** the system SHALL print an error: "invalid duration for --stale: expected format like '30d', '2w', or '3m'" and exit with code 1

#### Scenario: Stale filter with --non-interactive
- **WHEN** the user runs `sentei remove --stale 30d --force --non-interactive`
- **THEN** the system SHALL delete all worktrees whose last commit is older than 30 days without TUI interaction

### Requirement: --merged flag filters worktrees by merge status
The `remove` command SHALL accept a `--merged` flag that pre-selects worktrees whose branches the default branch has absorbed. A branch is merged when the default branch contains its tip and one of these holds: (a) the tip is not on the default branch's first-parent chain, so it arrived through a merge commit; (b) the tip is on that chain and the branch's reflog shows that the branch once pointed at another commit, so commits made on it were fast-forwarded in. A branch on the first-parent chain whose reflog shows no such move, or that has no reflog, SHALL NOT be selected, however far the default branch has moved. The default branch itself SHALL never be selected. The first-parent chain SHALL be read once per run, not once per branch.

#### Scenario: Merged filter with TUI
- **WHEN** the user runs `sentei remove --merged`
- **THEN** the TUI SHALL display the worktree list with merged-branch worktrees pre-selected

#### Scenario: Merged filter with --non-interactive
- **WHEN** the user runs `sentei remove --merged --force --non-interactive`
- **THEN** the system SHALL delete all worktrees with fully merged branches without TUI interaction

#### Scenario: No merged worktrees
- **WHEN** the user runs `sentei remove --merged` and no worktrees have merged branches
- **THEN** the TUI SHALL display the list with nothing pre-selected and a message: "No merged worktrees found"

#### Scenario: Branch merged through a merge commit
- **WHEN** a branch's commits reached the default branch through a merge commit
- **THEN** `--merged` SHALL select its worktree

#### Scenario: Branch fast-forwarded into the default branch
- **WHEN** commits were made on a branch, the default branch was fast-forwarded to them, and the default branch has not moved since
- **THEN** `--merged` SHALL select its worktree

#### Scenario: Branch nobody has worked on
- **WHEN** a branch was created from the default branch and has not moved since, and the default branch has gained commits
- **THEN** `--merged` SHALL NOT select its worktree

#### Scenario: Branch brought up to date without commits of its own
- **WHEN** a branch was created from the default branch, never committed to, and later fast-forwarded to the default branch
- **THEN** `--merged` SHALL select its worktree, because its reflog shows a move and nothing tells that move from merged work

#### Scenario: Fast-forwarded branch without a reflog
- **WHEN** a branch's tip is on the default branch's first-parent chain and the branch has no reflog
- **THEN** `--merged` SHALL NOT select its worktree

#### Scenario: Squash merge
- **WHEN** a branch was squash-merged, so its tip is not in the default branch
- **THEN** `--merged` SHALL NOT select its worktree

### Requirement: --all flag selects all non-protected worktrees
The `remove` command SHALL accept an `--all` flag that pre-selects all non-protected worktrees.

#### Scenario: All filter with TUI
- **WHEN** the user runs `sentei remove --all`
- **THEN** the TUI SHALL display the worktree list with all non-protected worktrees pre-selected

#### Scenario: All filter with --non-interactive requires --force
- **WHEN** the user runs `sentei remove --all --non-interactive` without `--force`
- **THEN** the system SHALL print: "destructive operation requires --force with --non-interactive" and exit with code 1

#### Scenario: All filter excludes protected
- **WHEN** the user runs `sentei remove --all` and branch "main" is protected
- **THEN** the "main" worktree SHALL NOT be pre-selected

### Requirement: Filter flags are composable
Multiple filter flags on the `remove` command SHALL be combined with OR logic — a worktree is pre-selected if it matches any provided filter.

#### Scenario: Combined stale and merged filters
- **WHEN** the user runs `sentei remove --stale 30d --merged`
- **THEN** worktrees that are stale OR merged SHALL be pre-selected

#### Scenario: All overrides other filters
- **WHEN** the user runs `sentei remove --all --stale 30d`
- **THEN** all non-protected worktrees SHALL be pre-selected (--all supersedes other filters)

### Requirement: Filter flags only apply to the remove command
Filter flags (`--stale`, `--merged`, `--all`) SHALL only be accepted by the `remove` command. Other commands SHALL reject these flags with an error.

#### Scenario: Filter flag on wrong command
- **WHEN** the user runs `sentei create --stale 30d`
- **THEN** the system SHALL print: "unknown flag: --stale" and exit with code 1
