# tui-design-system Specification

## Purpose
TBD - created by archiving change ui-chrome-unification. Update Purpose after archive.
## Requirements
### Requirement: Component patterns documented in .impeccable.md
The `.impeccable.md` file SHALL include a "Component Patterns" section documenting the standard view chrome, progress layout, status indicators, stat line format, timing constants, and key mapping.

#### Scenario: View chrome documented
- **WHEN** a developer reads `.impeccable.md`
- **THEN** the Component Patterns section SHALL specify: title format (`sentei ─ <Title>` via `viewTitle()`), separator format (dotted `┄` line via `viewSeparator()`), and key hints format (`key action · key action` via `viewKeyHints()`)

#### Scenario: Status indicators documented
- **WHEN** a developer reads `.impeccable.md`
- **THEN** the Component Patterns section SHALL include a table mapping each indicator symbol (●, ◐, ·, ✗, ⚠) to its name, color, and semantic meaning

#### Scenario: Key mapping documented
- **WHEN** a developer reads `.impeccable.md`
- **THEN** the Component Patterns section SHALL specify `?` as contextual details, `F1` as global help, and all standard navigation keys (j/k, arrows, enter, esc, q, space)

### Requirement: Key bindings defined in single source file
All key bindings SHALL be defined in `internal/tui/keys.go` as `key.Binding` variables, with contextual keys (meaning varies by view) and global keys (same meaning everywhere) clearly separated. Per-view key presentation (footer hint subsets and named help sections) SHALL also be declared in `keys.go`, reusing the canonical key strings and overriding only descriptions; render sites SHALL reference these declarations and SHALL NOT contain raw key or label strings.

#### Scenario: Contextual details key
- **WHEN** `keyDetails` is referenced
- **THEN** it SHALL bind to `?` with help text "details"

#### Scenario: Global help key
- **WHEN** `keyGlobalHelp` is referenced
- **THEN** it SHALL bind to `F1` with help text "help"

#### Scenario: No duplicate key definitions
- **WHEN** a key binding is needed in any view
- **THEN** it SHALL reference the binding from `keys.go` rather than creating a local binding

#### Scenario: Contextual description declared once per view
- **WHEN** the enter key means "delete" in the confirm view and "continue" in an input view
- **THEN** each description SHALL be declared once in that view's `keys.go` presentation data and nowhere else

#### Scenario: Render sites carry no hint literals
- **WHEN** any view renders its footer or the help portal renders its content
- **THEN** the key strings and action labels SHALL come from `keys.go` declarations, with no string literals at the render site

### Requirement: Layout constants defined centrally
Layout constants SHALL be defined in `internal/tui/constants.go` and used by all progress views. Progress hold timing already exists on the Model (`minProgressDuration` via `WithMinProgressDuration` and `holdOrAdvance`) and SHALL remain the single timing mechanism; `progressSettleFloor` is part of that mechanism (the post-completion settle beat when holds are enabled), not a parallel one.

#### Scenario: Windowing constants
- **WHEN** windowing logic needs completed trail or pending lead counts
- **THEN** it SHALL use `WindowCompletedTrail` and `WindowPendingLead` from `constants.go`

#### Scenario: Progress bar width floor
- **WHEN** any view renders the overall progress bar
- **THEN** the bar width SHALL be the content width minus the rendered width of its right-hand meta (percentage and elapsed readout), never narrower than the shared floor constant in `constants.go`

### Requirement: Charm v2 rendering platform
The TUI SHALL build on the Charm v2 stack (`charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`). Terminal features (alternate screen, mouse mode, keyboard enhancements) SHALL be declared as fields on the root model's `tea.View`; no view other than the root model SHALL declare terminal features.

#### Scenario: Alt screen declared on the root view
- **WHEN** the root model's `View()` is rendered
- **THEN** it SHALL return a `tea.View` with the alternate screen enabled, and `tea.NewProgram` SHALL receive no terminal-feature options

#### Scenario: Rendered content unchanged by the platform swap
- **WHEN** any existing view renders after the migration
- **THEN** its visible content (text, layout, colors, indicators) SHALL be unchanged from the v1 implementation

### Requirement: Keyboard enhancements for quick create
The TUI SHALL request keyboard enhancements so terminals supporting the kitty keyboard protocol can distinguish `ctrl+enter` from `enter`. The quick-create binding SHALL start worktree creation with default options directly from the branch input on supporting terminals.

#### Scenario: Quick create on a supporting terminal
- **WHEN** the user presses `ctrl+enter` on the create-branch input with a valid branch name on a terminal with the kitty keyboard protocol
- **THEN** creation SHALL start immediately with default options, skipping the options view

#### Scenario: Graceful degradation without the protocol
- **WHEN** the terminal does not support keyboard enhancements
- **THEN** `enter` (continue to options) SHALL remain fully functional and no error SHALL surface

### Requirement: Adaptive palette
The TUI SHALL select its palette from the terminal's reported background: dark terminals keep the dark palette (the documented baseline), light terminals get the light palette, and no report defaults to dark. Light-palette tokens SHALL hold readable contrast on white: dim text uses 243 and warning orange uses 130.

#### Scenario: Light terminal gets the light palette
- **WHEN** the terminal reports a light background
- **THEN** rendering SHALL use the light palette tokens

#### Scenario: Dark terminal keeps the dark palette
- **WHEN** the terminal reports a dark background
- **THEN** rendering SHALL use the dark palette tokens

#### Scenario: No background report defaults to dark
- **WHEN** the terminal does not answer the background query
- **THEN** rendering SHALL use the dark palette

#### Scenario: Light dim and warning are readable
- **WHEN** the light palette is active
- **THEN** dim text SHALL render in 243 and warnings in 130

### Requirement: Indeterminate wait indicator
Indeterminate waits (operations with no measurable progress: the cleanup repository scan and the menu's worktree-context load) SHALL display an animated spinner in the accent color. Determinate progress SHALL keep the static indicator vocabulary and the progress bar. Spinner ticks SHALL run only while an indeterminate wait is visible.

#### Scenario: Cleanup scan animates
- **WHEN** the cleanup preview is entered and the scan has not completed
- **THEN** the scanning line SHALL render an animated spinner frame followed by "Scanning repository…"

#### Scenario: Menu load animates
- **WHEN** the bare-repo menu is shown before the worktree context has loaded
- **THEN** the "Remove worktrees" item hint SHALL render an animated spinner frame followed by "loading…"

#### Scenario: Ticks stop when the wait ends
- **WHEN** a spinner tick message arrives and no indeterminate wait is visible
- **THEN** the model SHALL NOT schedule another tick

#### Scenario: Determinate progress unchanged
- **WHEN** any progress view renders phases and steps
- **THEN** step states SHALL keep the static indicator vocabulary (`◐` active) and the overall bar

### Requirement: Terminal tab presence
The root view SHALL set the terminal window title to `sentei · <repo>` at rest and `sentei · <repo> · <verb> <done>/<total>` during live flows, and SHALL mirror flow progress into the terminal's native progress indicator (OSC 9;4): indeterminate during spinner waits, the completion value during progress flows, the error state when a phase has failures, and none otherwise.

#### Scenario: Title at rest
- **WHEN** the menu is shown for repository "repo"
- **THEN** the window title SHALL be `sentei · repo`

#### Scenario: Title and native progress in flight
- **WHEN** a removal is running at 2 of 3
- **THEN** the title SHALL be `sentei · repo · removing 2/3` and the native progress SHALL carry the matching value

#### Scenario: Failures surface natively
- **WHEN** any phase reports failed steps
- **THEN** the native progress SHALL use the error state

### Requirement: Selection and danger weight
Cursor rows SHALL use the `▸` marker with selected entry text carrying the accent in menu-style views; destructive confirmation footers SHALL render their destructive hint in the warning style; worktrees SHALL be named by one canonical label across all views (short HEAD hash for detached); the confirm screen SHALL use the list's badge vocabulary; action hints SHALL NOT advertise impossible actions (no delete hint at zero selected).

#### Scenario: Danger hint weighted
- **WHEN** the deletion confirm screen renders
- **THEN** `y delete` SHALL carry the warning style while `n go back` stays dim

#### Scenario: Canonical detached label
- **WHEN** a detached worktree appears in the list and on the confirm screen
- **THEN** both SHALL show the same short HEAD hash

#### Scenario: No dead-end hints
- **WHEN** the list has zero selections
- **THEN** the status bar SHALL NOT offer `enter delete`

### Requirement: One working spinner
A single animated indicator SHALL mark everything sentei is actively doing: the star twinkle (`· ✢ ✳ ✻ ✽ ✻ ✳ ✢`, single-cell frames, 120ms), used by active phase lines, step lines, the windowed-steps stat line, the cleanup running line, the cleanup scan, and the menu worktree load. All motion derives from one deterministic clock: a gated tick counter from which star frames, star colors, and shimmer band positions are computed as pure functions. Ticks run only while a working surface is visible, and each entry path starts the tick chain in exactly one place (Init when the model starts in a working state, the dispatch wrapper on transitions into one). Pure layout constructions without a live frame SHALL fall back to the static `✻`.

#### Scenario: Active phase twinkles
- **WHEN** a determinate progress view renders a phase that is running
- **THEN** the phase indicator SHALL be the star's current frame

#### Scenario: One vocabulary
- **WHEN** any two working surfaces are visible in the same session (e.g. a scan, then a removal)
- **THEN** both SHALL render frames from the same clock

#### Scenario: Ticks gated to visible work
- **WHEN** no working surface is on screen
- **THEN** motion ticks SHALL stop

#### Scenario: No double tick chains
- **WHEN** a flow entry path would start the motion clock while a tick chain is already running
- **THEN** at most one tick chain SHALL drive the clock

#### Scenario: Pure layouts stay static
- **WHEN** a `ProgressLayout` is rendered without injected motion
- **THEN** active indicators SHALL render the static `✻` fallback

### Requirement: Gradient bar fill
The overall progress bar's filled portion SHALL blend from the `barStart` to the `barEnd` palette token, scaled to span exactly the filled cells, in both themes. The fill characters remain `█`/`░`. The spring SHALL be tuned for a visible glide: fills ease over most of a second rather than snapping, while still settling within the completion hold. The static fallback bar (pure constructions) keeps the solid accent fill.

#### Scenario: Gradient spans the fill
- **WHEN** the animated overall bar renders at any percentage above zero
- **THEN** the filled cells SHALL blend `barStart` to `barEnd`, with the blend endpoints at the first and last filled cell

#### Scenario: Adaptive endpoints
- **WHEN** the light palette is active
- **THEN** the blend SHALL use the light palette's `barStart`/`barEnd` tokens

#### Scenario: Settles within the hold
- **WHEN** a flow completes and the completion hold begins
- **THEN** the bar SHALL visibly reach full before the hold expires

#### Scenario: Flows that outlive the hold still settle
- **WHEN** a flow's final event arrives after the hold duration has already elapsed
- **THEN** the view SHALL stay up for at least the settle floor so the bar finishes at 100% instead of cutting away mid-glide

### Requirement: Element motion, instant cuts
View-to-view navigation SHALL cut instantly; no slide, fade, or transition animation may delay a keypress. Motion belongs to state-driven elements only: the bar's spring, the working spinner, and hold pacing.

#### Scenario: Navigation is immediate
- **WHEN** the user presses a key that changes views
- **THEN** the next view SHALL render immediately with no transition animation

### Requirement: Verdict and state glyphs
The star family carries the item lifecycle and the success verdict: pending `·` (the star's resting frame), working star morph, done `✦` on item rows, and `✦` in bold success color on operation-level summary headlines. `✗` marks failure at both levels; `⚠` marks warnings; the cleanup preview's would-act lines use accent `▸`. The full circle `●` and the checkmark `✓` SHALL NOT appear anywhere in the TUI. The core rule: anything moving is being worked on; anything still is settled.

#### Scenario: Success headline gets the crystallized star
- **WHEN** a summary or result view renders its operation-level success line
- **THEN** the line SHALL lead with `✦` in the success color

#### Scenario: Done rows crystallize
- **WHEN** a list of steps, actions, or items renders a completed item
- **THEN** the item SHALL be marked `✦` alongside its `·`/working-frame peers

#### Scenario: No full circles
- **WHEN** any view renders
- **THEN** the glyph `●` SHALL NOT appear in the output

### Requirement: Working-text shimmer
Every working line SHALL carry a gradient shimmer band sweeping its text in the text's own color family: accent base to light peak on phase headlines, scans, and the cleanup running line; body base to white peak on working step labels; dim ramp on menu loading hints. The star sits inside its line's band. Counts, percentages, and meta text do not shimmer; done and pending lines are still. Ramp endpoints are palette tokens, adapted per theme.

#### Scenario: Phase headline shimmers accent
- **WHEN** a phase is running
- **THEN** its label SHALL render with the accent shimmer band, the star inside the band

#### Scenario: Working steps shimmer in body color
- **WHEN** a step is running
- **THEN** its label SHALL render with the body shimmer band

#### Scenario: Shimmer preserves content
- **WHEN** any text is shimmered
- **THEN** the stripped text SHALL equal the input and the rune count SHALL be unchanged

#### Scenario: Settled text is still
- **WHEN** a line is done or pending
- **THEN** it SHALL render with static styling only

### Requirement: Confirm rows are columnar
The confirm-deletion screen SHALL list selected worktrees with the status badge in a fixed-width left gutter, names aligned in a column after it (pre-truncated to a stable width), and risk notes trailing only on at-risk rows. Clean rows carry no note text.

#### Scenario: Badges align in a gutter
- **WHEN** the confirm screen renders any mix of clean and at-risk selections
- **THEN** every badge SHALL start at the same column and every name SHALL start at the same column

#### Scenario: Long names do not shift the columns
- **WHEN** a selected worktree's label exceeds the name-column cap
- **THEN** the label SHALL truncate with an ellipsis and the columns SHALL hold

### Requirement: One voice, declared once
Every view and portal title SHALL be declared exactly once in the copy registry (`internal/tui/copy.go`) and referenced by const at render sites. Titles use sentence case. The same physical key SHALL carry the same verb across views unless the action genuinely differs (`?` is always "details").

#### Scenario: Title changes are one-line edits
- **WHEN** a view title needs rewording
- **THEN** exactly one declaration SHALL change

#### Scenario: Sentence case
- **WHEN** any view renders its title
- **THEN** the title SHALL be sentence case (first word capitalized only, proper nouns excepted)

### Requirement: Portal boxes carry no brand
The detail portal SHALL render its bare title inside the box; the `sentei ─` brand appears only on the view chrome behind it.

#### Scenario: Portal title is bare
- **WHEN** any portal opens
- **THEN** its title line SHALL NOT contain `sentei ─`

### Requirement: Completion settles the bar green
While a flow is working, the overall bar fills with the accent gradient; once the flow's result has arrived, the fill SHALL render in the success gradient (`barDoneStart→barDoneEnd`, per theme) for the remainder of the hold.

#### Scenario: Working bar is accent
- **WHEN** a flow is still producing events
- **THEN** the bar fill SHALL use the accent gradient

#### Scenario: Done bar is green
- **WHEN** the flow's completion result has arrived
- **THEN** the bar fill SHALL use the success gradient until the view transitions

### Requirement: Golden chrome pinning
The stable views (worktree list, confirm, removal summary, cleanup result, create input) SHALL be pinned by golden-file tests capturing their exact rendered output including styling. Golden updates SHALL be explicit (`-update`), never incidental.

#### Scenario: Chrome regression fails loudly
- **WHEN** any change alters a pinned view's exact output
- **THEN** the golden test SHALL fail until the golden is intentionally regenerated

### Requirement: P3 presentation rules
Sort arrows SHALL describe the displayed values' order (the Age column flips relative to its underlying date sort). Portal scroll hints SHALL appear only when content scrolls. Option footers SHALL include navigation hints. Tabbing into a prefilled input SHALL place the cursor at the end. Option-view cursors use `▸` and no view renders `●`.

#### Scenario: Age arrow matches the column
- **WHEN** the list sorts by age, date-ascending
- **THEN** the Age header SHALL show ▼ (the displayed ages descend)

#### Scenario: Fitting portal content offers no scroll keys
- **WHEN** portal content fits its viewport
- **THEN** the footer SHALL offer only close

#### Scenario: Tab lands at the end
- **WHEN** the user tabs into a field holding text
- **THEN** the cursor SHALL sit after the last character

### Requirement: Persistent input fields
Text-input views SHALL render every field's input persistently: focus changes the label accent, never the layout. Blurred empty fields show their placeholders; the clone destination preview is always visible and tracks the URL live. Text inputs share one declared width.

#### Scenario: Focus moves only the accent
- **WHEN** the user tabs between fields
- **THEN** no line SHALL appear, disappear, or change indentation

#### Scenario: Placeholders survive blur
- **WHEN** an empty field loses focus
- **THEN** its placeholder SHALL remain visible
