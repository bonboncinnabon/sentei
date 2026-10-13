package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// footerHelp renders footers from key bindings; its styles are wired to the
// active palette in applyPalette.
var footerHelp = newFooterHelp()

func newFooterHelp() help.Model {
	h := help.New()
	h.ShortSeparator = " · "
	return h
}

// viewTitle renders the standard view title: `  sentei ─ <title>` in bold white.
func viewTitle(title string) string {
	text := "  sentei ─"
	if title != "" {
		text += " " + title
	}
	return styleTitle.Render(text)
}

// viewSeparator renders a dotted separator line in dim gray spanning
// width-4 characters. Widths of 4 or less fall back to a default of 40.
func viewSeparator(width int) string {
	if width <= 4 {
		width = 40
	}
	return styleSeparator.Render("  " + strings.Repeat("┄", width-4))
}

// viewFooter renders a view's hint footer from bindings declared in keys.go:
// `key action · key action` in dim, 2-space pad. Hints that exceed width are
// dropped with an ellipsis rather than wrapping.
func viewFooter(width int, bindings []key.Binding) string {
	if len(bindings) == 0 {
		return ""
	}
	return "  " + footerHints(width-2, bindings)
}

// viewFooterDanger renders a footer whose FIRST binding performs an
// irreversible action: that hint carries the warning style so danger never
// hides in dim text. Remaining hints render as usual.
func viewFooterDanger(width int, bindings []key.Binding) string {
	if len(bindings) == 0 {
		return ""
	}
	danger := bindings[0].Help()
	out := "  " + styleWarning.Render(danger.Key+" "+danger.Desc)
	if len(bindings) > 1 {
		out += styleDim.Render(" · ") + footerHints(width-2-lipgloss.Width(out), bindings[1:])
	}
	return out
}

// footerHints renders the hint row itself within budget columns; callers
// that prepend their own prefix (the list status bar) pass the remaining
// budget directly.
func footerHints(budget int, bindings []key.Binding) string {
	if len(bindings) == 0 {
		return ""
	}
	h := footerHelp
	if budget > 0 {
		h.SetWidth(budget)
	}
	out := h.ShortHelpView(bindings)
	// bubbles/help stops truncating when the boundary item leaves no room
	// for its ellipsis marker; enforce the budget by dropping trailing
	// hints until the row fits, then mark the omission ourselves. Exact
	// fits never drop; once dropping, reserve room for the marker.
	dropped := false
	for budget > 0 && len(bindings) > 1 {
		limit := budget
		if dropped {
			limit = budget - 2
		}
		if lipgloss.Width(out) <= limit {
			break
		}
		dropped = true
		bindings = bindings[:len(bindings)-1]
		out = h.ShortHelpView(bindings)
	}
	if dropped {
		out += styleDim.Render(" …")
	}
	return out
}

// truncateWithEllipsis cuts s to fit width display cells, replacing the
// overflow with a trailing `…`. It measures what the terminal draws: styled
// text keeps its escape sequences whole and wide runes count as two cells.
// The only sanctioned way to fit overflowing paths, branch names, and error
// text. Chrome content must never hard-clip at the terminal edge.
func truncateWithEllipsis(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

// fitLines truncates every line of view to width cells. Views call it last,
// so a terminal narrower than their content shows ellipses instead of rows
// that wrap and push the rest of the frame down. Width 0 means unknown.
func fitLines(view string, width int) string {
	if width <= 0 {
		return view
	}
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		lines[i] = truncateWithEllipsis(line, width)
	}
	return strings.Join(lines, "\n")
}

// viewStatLine renders the windowing legend: per-status counts (zero counts
// omitted) followed by how many of the total items are visible. activeGlyph
// is the already-styled active indicator (spinner frame or static fallback).
func viewStatLine(stats WindowStats, activeGlyph string) string {
	var parts []string
	if stats.Done > 0 {
		parts = append(parts, fmt.Sprintf("%s %d done", styleIndicatorDone.Render(indicatorDone), stats.Done))
	}
	if stats.Active > 0 {
		parts = append(parts, fmt.Sprintf("%s %d active", activeGlyph, stats.Active))
	}
	if stats.Pending > 0 {
		parts = append(parts, fmt.Sprintf("%s %d pending", styleIndicatorPending.Render(indicatorPending), stats.Pending))
	}
	if stats.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%s %d failed", styleIndicatorFailed.Render(indicatorFailed), stats.Failed))
	}
	legend := strings.Join(parts, "  ")
	showing := styleDim.Render(fmt.Sprintf("  showing %d of %d", stats.Showing, stats.Total))
	return "    " + legend + showing
}

// inputFieldLabel renders a form field's label line: accent while the
// field has focus, plain otherwise. Fields render persistently — focus
// moves the accent, never the geometry.
func inputFieldLabel(label string, focused bool) string {
	if focused {
		return styleAccent.Render("  "+label) + "\n"
	}
	return "  " + label + "\n"
}

// summaryRow renders one "label value" line of a summary. The label is padded
// before it is styled: escape codes would otherwise count toward the column
// width.
func summaryRow(label, value string) string {
	return fmt.Sprintf("    %s %s\n", styleDim.Render(fmt.Sprintf("%-10s", label)), value)
}

// summaryPathRow is a summaryRow for a path or a URL. One that does not fit
// width gives up its head: the end names what the summary is about. Width 0
// means unknown.
func summaryPathRow(label, path string, width int) string {
	if width > 0 {
		path = fitPathTail(path, max(width-16, 20))
	}
	return summaryRow(label, path)
}
