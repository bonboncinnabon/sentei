## Purpose
Covers the CLI-to-TUI handoff architecture: command taxonomy (output vs decision), the --non-interactive and --force flags, partial flag resolution, typed options structs, and the command registry that routes commands.
## Requirements
### Requirement: Command taxonomy classifies commands as output or decision
The system SHALL classify each command as either "output" (read-only, always non-interactive) or "decision" (requires user choices, defaults to TUI). Output commands SHALL always produce stdout and exit. Decision commands SHALL launch the TUI unless `--non-interactive` is provided.

#### Scenario: Output command runs without TUI
- **WHEN** the user runs `sentei ecosystems`
- **THEN** the system SHALL print output to stdout and exit without launching Bubble Tea

#### Scenario: Decision command launches TUI by default
- **WHEN** the user runs `sentei create` without `--non-interactive`
- **THEN** the system SHALL launch the TUI for the create worktree flow

#### Scenario: Decision command with --non-interactive skips TUI
- **WHEN** the user runs `sentei create --branch foo --base main --non-interactive`
- **THEN** the system SHALL execute the create operation without launching Bubble Tea and print results to stdout

### Requirement: --non-interactive flag forces headless execution
The system SHALL accept a `--non-interactive` flag on all decision commands. When provided, the command SHALL execute without launching the TUI. If required flags are missing, the system SHALL print an error listing the missing flags and exit with a non-zero exit code.

#### Scenario: All required flags provided
- **WHEN** the user runs `sentei cleanup --mode safe --non-interactive`
- **THEN** the system SHALL execute cleanup in safe mode and print results to stdout

#### Scenario: Missing required flags
- **WHEN** the user runs `sentei cleanup --non-interactive` without `--mode`
- **THEN** the system SHALL print an error: "missing required flag: --mode (safe|aggressive)" and exit with code 1

#### Scenario: --non-interactive on output command
- **WHEN** the user runs `sentei ecosystems --non-interactive`
- **THEN** the flag SHALL be accepted but have no effect (output commands are already non-interactive)

### Requirement: --force overrides a command's own safeties and nothing else
`--non-interactive` SHALL NOT require `--force`, and SHALL NOT override any safety. `--force` SHALL be the per-command override: for `remove` it allows removing worktrees that are at risk (locked, uncommitted, untracked or unpushed work, or a status that could not be read); for `cleanup` it allows deleting branches that are not fully merged. Protected branches and the worktree holding the current directory SHALL never be removed, with or without `--force`.

#### Scenario: Removal with nothing at risk
- **WHEN** the user runs `sentei remove --merged --non-interactive` and no matched worktree is at risk
- **THEN** the system SHALL execute the removal without TUI interaction and without `--force`

#### Scenario: Removal with a worktree at risk
- **WHEN** the user runs `sentei remove --merged --non-interactive` and a matched worktree is at risk
- **THEN** the system SHALL remove nothing, name the worktrees at risk, and exit with a non-zero exit code

#### Scenario: Removal with --force
- **WHEN** the user runs `sentei remove --merged --force --non-interactive`
- **THEN** the system SHALL also remove the matched worktrees that are at risk

#### Scenario: Migration
- **WHEN** the user runs `sentei migrate --non-interactive <repo>`
- **THEN** the system SHALL run the migration without `--force`: it checks everything before the first change and rolls back on failure

### Requirement: Flags build options struct for decision commands
Each decision command SHALL define a typed options struct. Flags SHALL populate this struct via a `ParseFlags` function. Both TUI and `--non-interactive` paths SHALL consume the same options struct for execution.

#### Scenario: Flags parsed into options struct
- **WHEN** the user runs `sentei create --branch feature/foo --base main`
- **THEN** the system SHALL parse flags into a CreateOptions struct with Branch="feature/foo" and Base="main"

#### Scenario: Invalid flag value
- **WHEN** the user runs `sentei cleanup --mode invalid`
- **THEN** the system SHALL print an error: "invalid value for --mode: must be 'safe' or 'aggressive'" and exit with code 1

### Requirement: Partial flags enter TUI at first missing required field
When a decision command receives some but not all required flags (without `--non-interactive`), the TUI SHALL launch at the first screen whose corresponding flag was not provided. Screens for provided flags SHALL be skipped.

#### Scenario: Branch provided, base missing
- **WHEN** the user runs `sentei create --branch feature/foo`
- **THEN** the TUI SHALL open at the base branch selection step, skipping the branch name input

#### Scenario: No flags provided
- **WHEN** the user runs `sentei create`
- **THEN** the TUI SHALL open at the first step of the create flow (branch name input)

#### Scenario: All required flags provided
- **WHEN** the user runs `sentei create --branch feature/foo --base main`
- **THEN** the TUI SHALL open at the confirmation view, skipping all input steps

#### Scenario: Branch whose worktree exists
- **WHEN** the user runs `sentei create --branch feature/foo`, with or without `--base`, and a worktree for `feature/foo` exists
- **THEN** the TUI SHALL open at the branch name input with the name filled in and the error the input shows for a typed name

### Requirement: Command registry routes commands
The system SHALL use a command registry to dispatch commands instead of an ad-hoc if/else chain. Each registered command SHALL declare its name, type (output/decision), flag parser, CLI executor, and TUI builder.

#### Scenario: Known command dispatched
- **WHEN** the user runs `sentei cleanup --mode safe`
- **THEN** the registry SHALL look up "cleanup", classify it as a decision command, parse its flags, and route to the appropriate handler

#### Scenario: Unknown command
- **WHEN** the user runs `sentei foobar`
- **THEN** the system SHALL print "unknown command: foobar" with usage help and exit with code 1

#### Scenario: No command (root)
- **WHEN** the user runs `sentei` with no arguments
- **THEN** the system SHALL launch the TUI at the main menu

### Requirement: --yes flag skips CLI confirmation
CLI subcommands that show confirmation dialogs SHALL accept a `--yes` / `-y` flag to skip the confirmation and proceed immediately.

#### Scenario: Cleanup with --yes
- **WHEN** the user runs `sentei cleanup --mode safe --yes`
- **THEN** the system SHALL skip the confirmation dialog and run cleanup immediately

#### Scenario: Remove with --yes
- **WHEN** the user runs `sentei remove --merged --yes`
- **THEN** the system SHALL skip the standard confirmation and proceed to removal

#### Scenario: --yes does NOT skip dirty worktree confirmation
- **WHEN** the user runs `sentei remove --all --yes` and the selection includes dirty worktrees
- **THEN** the system SHALL still show the dirty/unpushed confirmation gate (data loss protection cannot be bypassed with --yes)

#### Scenario: --yes with aggressive cleanup
- **WHEN** the user runs `sentei cleanup --mode aggressive --yes`
- **THEN** the system SHALL skip confirmation and run aggressive cleanup immediately (the user explicitly chose aggressive mode via flag)

#### Scenario: --yes without required flags
- **WHEN** the user runs `sentei cleanup --yes` (no --mode flag, defaults to safe)
- **THEN** the system SHALL proceed with safe mode cleanup without confirmation

