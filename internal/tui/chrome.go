package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

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

// shellLines lays out a command sentei shows for the user to copy, as lines
// that each start with indent, within width cells. A command that fits stays
// on one line. Otherwise each argument takes a line ending in a backslash, and a
// word still too wide breaks after a break point with a bare backslash and
// continues at column 0: the shell removes a backslash-newline even inside a
// word, so a mouse copy of the lines pastes back as the same command. An
// indent there would split the word. Width 0 means unknown.
func shellLines(command, indent string, width int) []string {
	if width <= 0 || ansi.StringWidth(indent+command) <= width {
		return []string{indent + command}
	}
	units := commandUnits(command)
	var lines []string
	for i, unit := range units {
		prefix, end := indent+"  ", " \\"
		if i == 0 {
			prefix = indent
		}
		if i == len(units)-1 {
			end = ""
		}
		lines = append(lines, shellUnit(unit, prefix, end, width)...)
	}
	return lines
}

// commandUnits splits a command into what shellLines keeps on one line: the
// program with its subcommand, then each flag with its value. Values hold no
// spaces; they are branch names, URLs and comma lists.
func commandUnits(command string) []string {
	var units []string
	for _, field := range strings.Fields(command) {
		if n := len(units); n > 0 && !strings.HasPrefix(field, "-") && !strings.Contains(units[n-1], " ") {
			units[n-1] += " " + field
			continue
		}
		units = append(units, field)
	}
	return units
}

// shellUnit lays out one unit of shellLines: on its line when it fits, else
// with its value on the next line when the value fits there, else broken as
// shellLines describes.
func shellUnit(unit, prefix, end string, width int) []string {
	if ansi.StringWidth(prefix+unit+end) <= width {
		return []string{prefix + unit + end}
	}
	var lines []string
	if head, value, ok := strings.Cut(unit, " "); ok && ansi.StringWidth(prefix+value+end) <= width {
		return []string{prefix + head + " \\", prefix + value + end}
	}
	for unit != "" && ansi.StringWidth(prefix+unit+end) > width {
		// Room for the backslash, or for the end, so the last piece is never
		// empty.
		cut := breakIndex(unit, width-ansi.StringWidth(prefix)-max(1, ansi.StringWidth(end)))
		lines = append(lines, prefix+unit[:cut]+"\\")
		unit, prefix = unit[cut:], ""
	}
	return append(lines, prefix+unit+end)
}

// breakIndex returns where to break word so its first part fits room cells:
// just after the last break point that fits, else as much as fits. It always
// moves forward, so a loop over it ends.
func breakIndex(word string, room int) int {
	fit, lastBreak, cells := 0, 0, 0
	for i, r := range word {
		cells += ansi.StringWidth(string(r))
		if cells > room {
			break
		}
		fit = i + utf8.RuneLen(r)
		if strings.ContainsRune("/-_:.", r) {
			lastBreak = fit
		}
	}
	if lastBreak > 0 {
		return lastBreak
	}
	if fit == 0 {
		_, fit = utf8.DecodeRuneInString(word)
	}
	return fit
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
// width. A value of several lines, or one wider than width, continues in the
// value column. Below a value column of stackBelow cells the label takes its
// own line, as on a confirmation. Width 0 means unknown.
func summaryRow(label, value string, width int) string {
	prefix := "    " + styleDim.Render(fmt.Sprintf("%-10s", label)) + " "
	if width <= 0 {
		return prefix + strings.ReplaceAll(value, "\n", "\n"+strings.Repeat(" ", 15)) + "\n"
	}
	var b strings.Builder
	if width-ansi.StringWidth(prefix) < stackBelow {
		b.WriteString("    " + styleDim.Render(label) + "\n")
		prefix = "      "
	}
	writeProgressDetailValue(&b, prefix, value, width)
	return b.String()
}

// summaryPathRow is a summaryRow for a path or a URL. One that does not fit
// width gives up its head: the end names what the summary is about. Width 0
// means unknown.
func summaryPathRow(label, path string, width int) string {
	if width > 0 {
		path = fitPathTail(path, max(width-16, 20))
	}
	return summaryRow(label, path, width)
}
