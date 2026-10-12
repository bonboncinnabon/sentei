## Purpose
Covers the TUI progress view during deletion, including progress bar updates, per-worktree status tracking, post-deletion summary, and the Cmd-chained event consumption pattern.
## Requirements
### Requirement: Display deletion progress
The TUI SHALL display a progress view during deletion using the shared `ProgressLayout` renderer, showing a title (`sentei ─ Removing Worktrees`), phase-based layout (Teardown, Removing worktrees, Prune & cleanup), per-worktree status indicators under the active phase with 4-space indentation, adaptive windowing for large selections, an overall progress bar, and key hints. The view SHALL update in real-time as each deletion event is received from the deletion channel.

#### Scenario: Progress bar updates incrementally
- **WHEN** a worktree deletion completes (success or failure) and 2 out of 5 total have now finished
- **THEN** the overall progress bar SHALL re-render showing 40% completion

#### Scenario: Per-worktree status on deletion start
- **WHEN** a `DeletionStarted` event is received for a worktree
- **THEN** the TUI SHALL show `◐` (active indicator in purple) next to that worktree's branch name at 4-space indentation

#### Scenario: Per-worktree status on success
- **WHEN** a `DeletionCompleted` event is received for a worktree
- **THEN** the TUI SHALL show `●` (done indicator in green) next to that worktree's branch name

#### Scenario: Per-worktree status on failure
- **WHEN** a `DeletionFailed` event is received for a worktree
- **THEN** the TUI SHALL show `✗` (failed indicator in red) next to that worktree's branch name

#### Scenario: Adaptive windowing with 30 worktrees on short terminal
- **WHEN** 30 worktrees are being removed and terminal height is 25 lines
- **THEN** the view SHALL show a windowed subset with all active and failed items visible, recent completed items, and a stat line showing `● N done · ◐ N active · · N pending  showing X of 30`

#### Scenario: No windowing with few worktrees
- **WHEN** 5 worktrees are being removed and terminal height is 25 lines
- **THEN** the view SHALL show all 5 worktrees with no stat line

#### Scenario: Parallel removal shows multiple active
- **WHEN** 3 worktrees are being removed concurrently (max concurrency 5)
- **THEN** the view SHALL show 3 items with `◐` active indicator simultaneously

#### Scenario: Title uses standard chrome
- **WHEN** the removal progress view is rendered
- **THEN** the title SHALL use `viewTitle("Removing Worktrees")` (bold white text), NOT the `styleHeader` background badge

#### Scenario: Events forwarded as Bubble Tea messages
- **WHEN** the deletion goroutine sends events over the progress channel
- **THEN** each event SHALL be delivered to the Bubble Tea Update loop as an individual message, not batched

### Requirement: Cmd-chained event consumption
The TUI SHALL consume deletion progress events using a Cmd-chaining pattern where each Cmd reads one event from the channel and returns it as a Msg, and the Update handler returns a new Cmd to read the next event.

#### Scenario: Channel stored on model
- **WHEN** deletion starts
- **THEN** the progress event channel SHALL be stored on the Model so it persists across Update calls

#### Scenario: Sequential event delivery
- **WHEN** multiple deletion events arrive on the channel
- **THEN** each event SHALL be read by a separate Cmd invocation and delivered as a separate Msg to Update

#### Scenario: Channel closure triggers completion
- **WHEN** the progress channel is closed (all deletions finished)
- **THEN** the wait Cmd SHALL return a completion message and the TUI SHALL transition to prune, then cleanup, then summary

### Requirement: Transition to summary after all deletions
The TUI SHALL run `git worktree prune` via a Cmd after all deletions complete, then run cleanup, then transition to the summary view with both results.

#### Scenario: All deletions complete triggers prune
- **WHEN** every selected worktree has either succeeded or failed
- **THEN** the TUI SHALL execute the prune operation as a Bubble Tea Cmd before transitioning to summary

#### Scenario: Prune completes successfully
- **WHEN** the prune Cmd returns with no error
- **THEN** the TUI SHALL store a nil prune error on the model and chain to cleanup

#### Scenario: Prune fails
- **WHEN** the prune Cmd returns with an error
- **THEN** the TUI SHALL store the error on the model and chain to cleanup

### Requirement: Post-deletion summary
The TUI SHALL display a summary using `viewTitle("Removal Complete")`, `viewSeparator`, and `viewKeyHints` for consistent framing. Success markers SHALL use `●` (done indicator in green), not `"v"`.

#### Scenario: All successful with prune success
- **WHEN** all 3 selected worktrees were deleted successfully and prune succeeded
- **THEN** the summary SHALL show "3 worktrees removed successfully" with `●` indicator, "Pruned orphaned worktree metadata", and no error section

#### Scenario: Mixed results with prune success
- **WHEN** 2 worktrees succeeded, 1 failed, and prune succeeded
- **THEN** the summary SHALL show "2 removed, 1 failed" with `●` for success and `✗` for failure

#### Scenario: Prune failed
- **WHEN** deletions are complete and prune failed
- **THEN** the summary SHALL show "Warning: failed to prune worktree metadata" with the error

### Requirement: Exit from summary
The TUI SHALL exit or return to menu when the user presses the appropriate key from the summary view.

#### Scenario: Quit from summary (standalone mode)
- **WHEN** the user presses 'q', Enter, or Escape on the summary view and no menu is available
- **THEN** the application SHALL exit cleanly

#### Scenario: Return to menu from summary
- **WHEN** the user presses Enter or Escape on the summary view and a menu is available
- **THEN** the TUI SHALL return to the menu view

### Requirement: Create worktree progress uses shared layout
The create worktree progress view SHALL use the shared `ProgressLayout` renderer with phases (Setup, Dependencies), an overall progress bar, and standard chrome.

#### Scenario: Create progress renders with bar
- **WHEN** the create worktree operation is in progress with Setup complete and Dependencies at 50%
- **THEN** the view SHALL show Setup with `100% ●`, Dependencies expanded with steps, and an overall progress bar

#### Scenario: Create progress uses standard title
- **WHEN** the create progress view is rendered
- **THEN** the title SHALL use `viewTitle("Creating Worktree")` with the subtitle showing branch and base

### Requirement: Repo operations progress uses shared layout
The repo operations progress view (create, clone, migrate) SHALL use the shared `ProgressLayout` renderer with operation-specific title, phases, overall progress bar, and standard chrome.

#### Scenario: Repo create progress
- **WHEN** a repository create operation is in progress
- **THEN** the view SHALL use `viewTitle("Creating Repository")` with shared layout

#### Scenario: Repo clone progress
- **WHEN** a repository clone operation is in progress
- **THEN** the view SHALL use `viewTitle("Cloning Repository")` with shared layout

