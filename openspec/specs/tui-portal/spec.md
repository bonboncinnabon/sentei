# tui-portal Specification

## Purpose
TBD - created by archiving change detail-portal-component. Update Purpose after archive.
## Requirements
### Requirement: DetailPortal renders scrollable overlay
The system SHALL provide a `DetailPortal` component that renders pre-formatted content in a scrollable viewport composited centered over the current background view using lipgloss cell-accurate layer compositing, with no background glyphs bleeding through the overlay region.

#### Scenario: Portal displayed over background
- **WHEN** the portal is opened with content and the current view is the removal progress view
- **THEN** the portal SHALL render as a bordered, styled box centered over the background view, which stays visible around the box

#### Scenario: Scrollable content
- **WHEN** the portal content exceeds the viewport height (terminal height minus chrome)
- **THEN** the viewport SHALL be scrollable with j/k, up/down, and page up/page down keys

#### Scenario: Short content fits without scrolling
- **WHEN** the portal content fits within the viewport
- **THEN** no scroll indicators SHALL be shown

#### Scenario: No background bleed-through
- **WHEN** the portal is open over a list with an active cursor row
- **THEN** no background glyph (including the cursor marker) SHALL appear inside or beside the portal box region


### Requirement: Portal chrome
The portal SHALL render with consistent chrome: a title bar at the top, scroll indicator when scrollable, and a key hints line at the bottom.

#### Scenario: Title bar
- **WHEN** the portal is opened with title "Cleanup details"
- **THEN** the portal SHALL display the title in bold white at the top of the overlay

#### Scenario: Scroll indicator when scrollable
- **WHEN** the content is scrollable and the user is not at the bottom
- **THEN** the portal SHALL display a scroll hint (e.g., `↓ more` or percentage) near the bottom

#### Scenario: Dismiss hint
- **WHEN** the portal is visible
- **THEN** the key hints line SHALL show `esc close · j/k scroll`

### Requirement: Portal show/hide lifecycle
The portal SHALL support opening with content and a title, and closing to restore the previous view.

#### Scenario: Open portal
- **WHEN** a view opens the portal with title and content
- **THEN** the portal SHALL become visible, scroll position SHALL reset to top, and the background view SHALL remain rendered behind it

#### Scenario: Close portal with Esc
- **WHEN** the user presses Esc while the portal is visible
- **THEN** the portal SHALL close and the background view SHALL be fully restored

#### Scenario: Close portal with ?
- **WHEN** the user presses `?` while the portal was opened via `?`
- **THEN** the portal SHALL close (toggle behavior)

### Requirement: Portal intercepts keys when visible
The portal SHALL intercept all key events when visible, only processing scroll and dismiss keys.

#### Scenario: Navigation keys blocked
- **WHEN** the portal is visible and the user presses `j` (which would normally navigate a list)
- **THEN** `j` SHALL scroll the portal content down, NOT navigate the background view

#### Scenario: Quit key passes through
- **WHEN** the portal is visible and the user presses `q` or `ctrl+c`
- **THEN** the application SHALL quit (quit is never blocked)

### Requirement: Portal sizing
The portal SHALL size itself relative to the terminal dimensions, leaving a margin around the edges.

#### Scenario: Standard terminal
- **WHEN** the terminal is 80x24
- **THEN** the portal SHALL render with approximately 2-character horizontal margin and 2-line vertical margin (inner size ~76x20)

#### Scenario: Terminal resize while portal is open
- **WHEN** a `WindowSizeMsg` is received while the portal is visible
- **THEN** the portal SHALL resize to fit the new terminal dimensions

### Requirement: Mouse wheel scrolls the portal viewport
While the portal is visible, mouse wheel events SHALL scroll the portal's viewport content and SHALL NOT reach the background view.

#### Scenario: Wheel scrolls portal content
- **WHEN** the portal is open with scrollable content and the user scrolls the wheel down
- **THEN** the portal content SHALL scroll down, identical to pressing `j`

#### Scenario: Background view unaffected by wheel while portal open
- **WHEN** the portal is open over the worktree list and the user scrolls the wheel
- **THEN** the list cursor SHALL NOT move

