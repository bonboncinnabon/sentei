package tui

import "time"

// Layout constants shared by all progress views.
const (
	// WindowCompletedTrail is how many recently completed steps stay visible
	// when a step list is windowed.
	WindowCompletedTrail = 2

	// WindowPendingLead is how many upcoming pending steps stay visible when
	// a step list is windowed.
	WindowPendingLead = 1

	// minProgressBarWidth is the narrowest the overall progress bar may
	// render; below this the fill stops being readable.
	minProgressBarWidth = 20

	// progressBarElapsedReserve is the line budget held back from the bar
	// for the elapsed readout ("  elapsed 9999s"). A fixed reserve keeps the
	// bar from shrinking a cell every time the elapsed digits grow.
	progressBarElapsedReserve = 15
)

// overallBarWidth is the single sizing rule for the overall bar: fill the
// content width, hold back the elapsed reserve, never go below the floor.
func overallBarWidth(viewWidth int) int {
	return max(minProgressBarWidth, viewWidth-2-progressBarElapsedReserve)
}

// viewChromeRows is the vertical chrome budget subtracted from the terminal
// height when sizing scrollable view bodies: title block, separators, and
// the footer area.
const viewChromeRows = 6

// listExtraChromeRows is what the worktree list draws beyond viewChromeRows:
// the repo line and its blank line, the column header, the status bar's top
// padding and the legend.
const listExtraChromeRows = 5

// progressSettleBeat is how long a flow's final progress frame stays visibly
// settled (displayed fill at its spring target) before advancing to the
// summary. State-relative: the beat starts when the spring arrives, not when
// the final event fires, so endings show the truth in every run mode.
const progressSettleBeat = 600 * time.Millisecond

// progressSettleTimeout hard-bounds the settle wait from the final event
// (covering the ~1.2s spring glide plus the beat, with margin) so the view
// can never wedge if the displayed fill cannot reach its target.
const progressSettleTimeout = 3 * time.Second

// confirmNameWidthCap bounds the confirm screen's name column so one long
// branch cannot push the risk notes off to the right.
const confirmNameWidthCap = 28

// confirmRowLead is what a confirm row draws before its name: the indent,
// the badge gutter and the gap after it.
const confirmRowLead = 10

// confirmNameWidthMin is the narrowest name column that may still sit beside
// the risk notes. Below it the notes are dropped and the names get the row.
const confirmNameWidthMin = 12

// formInputWidth is the shared cell width of text-input fields: wide enough
// for real URLs, and required because the v2 textinput renders only the
// first placeholder rune when its width is unset.
const formInputWidth = 48

// stackBelow is the narrowest value column a "label  value" row keeps. Below
// it the label goes on its own line and the value under it: at 40 columns a
// branch name otherwise takes four lines of a 21-cell column.
const stackBelow = 30
