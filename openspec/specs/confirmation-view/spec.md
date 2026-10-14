## Purpose
Covers the reusable TUI confirmation view component that displays resolved command options as a key-value summary, shows the equivalent CLI command, and handles confirm/back/quit navigation.
## Requirements
### Requirement: Confirmation view displays resolved options as key-value summary
The TUI SHALL provide a reusable confirmation view component that renders using standard view chrome (`viewTitle`, `viewSeparator`, `viewKeyHints`) instead of a bordered dialog box. The view SHALL display a title and a list of key-value pairs representing the resolved options for a command.

#### Scenario: Create worktree confirmation
- **WHEN** the confirmation view is shown for a create operation with Branch="feature/foo", Base="main"
- **THEN** the view SHALL display `viewTitle("Create Worktree")` followed by a separator, then each option as a labeled row, without any border wrapping

#### Scenario: Create worktree confirmation for a branch that already exists
- **WHEN** the confirmation view is shown for a create operation whose branch exists locally or only on origin
- **THEN** the Base row SHALL say that the base is not used and why (`main (not used: the branch already exists)`, `main (not used: the branch exists on origin)`)

#### Scenario: Cleanup confirmation
- **WHEN** the confirmation view is shown for a cleanup operation with Mode="aggressive"
- **THEN** the view SHALL display `viewTitle("Confirm Cleanup")` with Mode="aggressive" as a labeled row, using standard chrome

#### Scenario: A value wider than the screen
- **WHEN** a row's value or the CLI command is wider than the terminal
- **THEN** it SHALL wrap, continuing under the value's first line, and SHALL NOT be cut

#### Scenario: No border wrapping
- **WHEN** any confirmation view is rendered
- **THEN** the view SHALL NOT use `styleDialogBox` or any border wrapping — it SHALL use the same full-width layout as all other views

### Requirement: Confirmation view shows equivalent CLI command
The confirmation view SHALL display the equivalent CLI command in summary/result views instead of in the confirmation dialog, so users discover the CLI equivalent after seeing what happened.

#### Scenario: CLI command in removal summary
- **WHEN** the removal summary view is rendered after deleting worktrees
- **THEN** the summary SHALL include the equivalent CLI command (e.g., `sentei remove --branches feature-a,feature-b`)

#### Scenario: CLI command in cleanup result
- **WHEN** the cleanup result view is rendered after cleanup completes
- **THEN** the result SHALL include the equivalent CLI command (e.g., `sentei cleanup --mode safe`)

#### Scenario: CLI command not in confirmation dialog
- **WHEN** any confirmation dialog is rendered (CLI path)
- **THEN** the CLI command echo SHALL still be shown (CLI users benefit from seeing what they asked for)

### Requirement: Confirmation view accepts enter to proceed or esc to go back
The confirmation view SHALL use `viewKeyHints` to render consistent key hints and transition to the execution phase when the user presses Enter. Escape SHALL return to the previous step when there is one, and exit otherwise.

#### Scenario: User confirms
- **WHEN** the user presses Enter on the confirmation view
- **THEN** the TUI SHALL transition to the progress/execution view

#### Scenario: Escape on a confirmation opened by flags
- **WHEN** the user entered the confirmation view via flags (`sentei cleanup`, `sentei create --branch --base`, `sentei clone --url`, `sentei migrate`) and presses Escape
- **THEN** the TUI SHALL exit cleanly: no earlier step exists to go back to

#### Scenario: User goes back to the menu
- **WHEN** the user reached the migrate confirmation from the menu and presses Escape
- **THEN** the TUI SHALL navigate back to the menu

#### Scenario: User quits
- **WHEN** the user presses 'q' or Ctrl+C on the confirmation view
- **THEN** the TUI SHALL exit cleanly

#### Scenario: Key hints use shared renderer
- **WHEN** the confirmation view renders key hints
- **THEN** it SHALL use `viewKeyHints(KeyHint{"enter", "confirm"}, KeyHint{"esc", "back"}, KeyHint{"q", "quit"})` for consistent formatting

