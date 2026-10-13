# tui-help Specification

## Purpose
TBD - created by archiving change detail-portal-component. Update Purpose after archive.
## Requirements
### Requirement: Global help overlay via F1
The system SHALL display a help overlay when the user presses `F1` from any view, showing key bindings and descriptions contextual to the active view.

#### Scenario: Help from list view
- **WHEN** the user presses `F1` while on the worktree list view
- **THEN** the portal SHALL open with a title "Help — Worktree List" and content listing all available keys for that view (j/k navigate, space select, enter confirm, / filter, etc.)

#### Scenario: Help from progress view
- **WHEN** the user presses `F1` while on any progress view
- **THEN** the portal SHALL open with a title "Help — <Operation>" and content listing available keys (ctrl+c quit)

#### Scenario: Help from summary view
- **WHEN** the user presses `F1` while on any summary view
- **THEN** the portal SHALL open with a title "Help — <Summary Type>" and content listing available keys

#### Scenario: F1 closes help if already open
- **WHEN** the user presses `F1` while the help overlay is already visible
- **THEN** the help overlay SHALL close (toggle behavior)

### Requirement: Help content is contextual per view
Each view SHALL provide its own help content, so the help overlay shows relevant information for the active view rather than a generic help screen. The content SHALL derive from the same per-view key sections declared in `keys.go` that drive the footer hints, so footer and help can never disagree about bindings.

#### Scenario: Different views show different help
- **WHEN** the user opens help from the list view and then from the menu view
- **THEN** each SHALL display different key bindings and descriptions relevant to that specific view

#### Scenario: Help content format
- **WHEN** help content is rendered for any view
- **THEN** it SHALL display key bindings in a formatted table: key on the left, description on the right, grouped by category (navigation, actions, etc.)

#### Scenario: Footer and help agree
- **WHEN** a binding appears in a view's footer
- **THEN** the same key and description SHALL appear in that view's help sections

#### Scenario: Help follows the state of the view
- **WHEN** a view's keys depend on its state, as on the cleanup preview (scanning, scan failed, nothing to clean, work found, aggressive prompt)
- **THEN** the help SHALL list the keys of the current state, taken from the same list as the footer, so on a preview with nothing to clean Enter is described as "back" and not as running a cleanup

### Requirement: Contextual details via ?
Views that support contextual details SHALL open the portal with view-specific detail content when the user presses `?`.

#### Scenario: ? on a view with details available
- **WHEN** the user presses `?` on a view that has detail content (e.g., cleanup preview with aggressive details)
- **THEN** the portal SHALL open with the detail content

#### Scenario: ? on a view with no details
- **WHEN** the user presses `?` on a view that has no contextual detail content
- **THEN** nothing SHALL happen (key is ignored, no empty portal)

#### Scenario: ? toggles portal closed
- **WHEN** the user presses `?` while the detail portal is already open (opened via `?`)
- **THEN** the portal SHALL close

