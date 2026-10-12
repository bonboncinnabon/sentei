# tui-chrome Specification

## Purpose
TBD - created by archiving change ui-chrome-unification. Update Purpose after archive.
## Requirements
### Requirement: View title rendering
The system SHALL provide a `viewTitle(title string) string` function that renders `  sentei ─ <title>` using bold white (color 15) styling.

#### Scenario: Standard title
- **WHEN** `viewTitle("Removing Worktrees")` is called
- **THEN** the output SHALL contain "sentei ─ Removing Worktrees" with bold white styling and 2-space left padding

#### Scenario: Empty title
- **WHEN** `viewTitle("")` is called
- **THEN** the output SHALL contain "sentei ─" with no trailing text

### Requirement: Separator rendering
The system SHALL provide a `viewSeparator(width int) string` function that renders a dotted line (`┄`) in gray (color 241) spanning `width - 4` characters with 2-space left padding.

#### Scenario: Standard width
- **WHEN** `viewSeparator(80)` is called
- **THEN** the output SHALL contain 76 `┄` characters with 2-space left padding in gray styling

#### Scenario: Narrow width
- **WHEN** `viewSeparator(4)` or `viewSeparator(0)` is called
- **THEN** the output SHALL fall back to a default width of 40

### Requirement: Key hints rendering
The system SHALL render view footers from `key.Binding` sets via `bubbles/help`, through a `viewFooter` helper that produces key-action pairs separated by ` · ` in the dim palette token with 2-space left padding. Render sites SHALL pass bindings declared in `keys.go`, never raw key or label strings.

#### Scenario: Multiple hints
- **WHEN** a footer is rendered for bindings (enter "confirm"), (esc "back"), (q "quit")
- **THEN** the output SHALL render `  enter confirm · esc back · q quit` in dim styling

#### Scenario: Single hint
- **WHEN** a footer is rendered for the single binding (q "quit")
- **THEN** the output SHALL render `  q quit` with no separator

#### Scenario: Narrow width truncates gracefully
- **WHEN** the hint row exceeds the available terminal width
- **THEN** trailing hints SHALL be dropped with an ellipsis rather than wrapping or hard-clipping

### Requirement: Progress layout rendering
The system SHALL provide a `ProgressLayout` struct with a `View() string` method that renders the standard progress view layout: title, optional subtitle, separator, phases with steps, separator, overall progress bar, and key hints.

#### Scenario: Complete layout with subtitle
- **WHEN** a `ProgressLayout` is rendered with Title="Creating Worktree", Subtitle="feature/foo → from main", and 3 phases
- **THEN** the output SHALL contain the title line, subtitle in accent color (62), top separator, phase sections, bottom separator, and overall progress bar

#### Scenario: Layout without subtitle
- **WHEN** a `ProgressLayout` is rendered with no Subtitle
- **THEN** the output SHALL omit the subtitle line and render title directly followed by the separator

#### Scenario: Completed phase collapses steps
- **WHEN** a phase has Done == Total and no failed steps
- **THEN** the phase SHALL render as a single line `  ● <Name>  <total>/<total>  100%` and no individual step lines

#### Scenario: Completed phase with failures keeps steps visible
- **WHEN** a phase has Done == Total but Failed > 0
- **THEN** the phase SHALL render with expanded steps so failed items are visible

#### Scenario: Active phase shows steps
- **WHEN** a phase has Done < Total
- **THEN** the phase SHALL render with expanded step lines using status indicators (● done, ◐ active, · pending, ✗ failed)

#### Scenario: Pending phase
- **WHEN** a phase has Total == 0 (no events received yet)
- **THEN** the phase SHALL render as a single line with "pending" in gray text

#### Scenario: Step indentation
- **WHEN** steps are rendered under a phase header
- **THEN** each step SHALL use 4-space indentation (phase headers use 2-space)

#### Scenario: Overall progress bar
- **WHEN** the layout is rendered with phases totaling 10 done out of 20 total steps
- **THEN** the bottom progress bar SHALL render as `████████████░░░░░░░░ 50%` with 20-character bar width

#### Scenario: Phase status line format
- **WHEN** a phase named "Removing worktrees" has 12 done out of 30 total
- **THEN** the phase header SHALL render as `  ◐ Removing worktrees   12/30  40%` with the status indicator left of the phase name and the count and percentage after it

### Requirement: Adaptive windowing for step lists
The system SHALL provide a `WindowSteps(steps []ProgressStep, availableLines int) WindowResult` function that selects which steps to display when the list exceeds available terminal space.

#### Scenario: All items fit
- **WHEN** 5 steps are provided with availableLines=10
- **THEN** all 5 steps SHALL be returned and `Windowed` SHALL be false

#### Scenario: Items exceed budget
- **WHEN** 30 steps are provided with availableLines=8
- **THEN** `Windowed` SHALL be true, `Showing` SHALL equal the number of visible items, and the stat line budget (1 line) SHALL be reserved

#### Scenario: Failed items always visible
- **WHEN** 30 steps include 3 failed items and availableLines=5
- **THEN** all 3 failed items SHALL appear in the visible set regardless of budget pressure

#### Scenario: Active items always visible
- **WHEN** 30 steps include 5 active items and availableLines=6
- **THEN** all 5 active items SHALL appear in the visible set

#### Scenario: Remaining budget fills with recent completed and next pending
- **WHEN** windowing is active with budget remaining after failed + active items
- **THEN** the remaining lines SHALL show the most recently completed items first, then the next pending items

#### Scenario: Budget of zero
- **WHEN** availableLines=0
- **THEN** only failed and active items SHALL be returned (minimum viable display)

### Requirement: Stat line rendering
The system SHALL provide a `viewStatLine(stats WindowStats) string` function that renders an indicator legend with counts when windowing is active.

#### Scenario: Standard stat line
- **WHEN** stats show 10 done, 3 active, 17 pending, 0 failed, showing 6 of 30
- **THEN** the output SHALL render `    ● 10 done  ◐ 3 active  · 17 pending  showing 6 of 30` with appropriate indicator colors and gray text (entries joined by two spaces so the `·` pending indicator never collides with a separator)

#### Scenario: Stat line with failures
- **WHEN** stats include 2 failed items
- **THEN** the output SHALL include `✗ 2 failed` in the legend in red (color 196)

#### Scenario: Zero count omission
- **WHEN** a status category has zero items (e.g., 0 failed)
- **THEN** that category SHALL be omitted from the stat line

### Requirement: Styled overall progress bar
The system SHALL render the overall progress bar with the filled portion in the accent color and the unfilled track in dim; the percentage label SHALL use the default foreground and SHALL reflect the displayed fill, so the bar and its label never disagree. Actual completion counts SHALL remain visible in the phase headers. In live progress views the bar fill SHALL animate smoothly (spring easing) toward each new completion target and SHALL visibly settle at the target within the completion hold; a dim `elapsed Ns` readout SHALL render beside the bar. A bar SHALL never render uncolored.

#### Scenario: Bar colors
- **WHEN** a progress view renders a bar at 53%
- **THEN** the filled `█` cells SHALL carry the accent token and the `░` track cells SHALL carry the dim token

#### Scenario: Bounds clamped
- **WHEN** the done count exceeds the total for any reason
- **THEN** the bar SHALL clamp to 100% and never panic on a negative repeat count

#### Scenario: Label follows the fill
- **WHEN** the fill is easing through 40% toward a 100% target
- **THEN** the label SHALL read 40%, and the phase headers SHALL state the actual completion counts

#### Scenario: Completion settles within the hold
- **WHEN** a flow completes and the view holds before transitioning
- **THEN** the bar SHALL visibly reach a full fill with a 100% label during the hold

#### Scenario: Elapsed readout
- **WHEN** a progress flow has been running for 12 seconds
- **THEN** the bar line SHALL include a dim `elapsed 12s` readout


### Requirement: Ellipsis truncation for overflowing text
The system SHALL provide a truncation helper used wherever paths, branch names, or error messages can exceed the available width, cutting the string to fit with a trailing `…`. Raw hard clipping at the terminal edge SHALL NOT occur in chrome-rendered content.

#### Scenario: Long path truncated
- **WHEN** a worktree path longer than the available width is rendered in a summary or step line
- **THEN** the rendered line SHALL end with `…` within the width budget instead of being cut mid-character by the terminal

#### Scenario: Short text untouched
- **WHEN** the text fits within the available width
- **THEN** it SHALL render unchanged with no ellipsis


### Requirement: Progress flows end truthfully
When a progress flow completes, its layout SHALL reach a coherent terminal state before transitioning: the overall bar's final target SHALL be 100%, and any phase that never discovered work SHALL render as a dim `– <Name>  skipped` line and SHALL NOT count as outstanding work. Quitting the application from a live progress view SHALL print a one-line stderr notice naming the interrupted operation.

#### Scenario: No-work phases at completion
- **WHEN** a worktree creation completes with no dependencies enabled
- **THEN** those phases SHALL read `– skipped` (dim) and the overall bar SHALL target and settle at 100%

#### Scenario: Mid-run pending unchanged
- **WHEN** the same phases have not yet run while the flow is still in flight
- **THEN** they SHALL keep the existing `· pending` treatment and count as outstanding

#### Scenario: Quit leaves a trace
- **WHEN** the user quits during repository creation
- **THEN** stderr SHALL carry a warning naming the interrupted operation after exit
