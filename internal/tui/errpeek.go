package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// errorPeekLines bounds a multi-line command error to a peek the chrome can
// survive: the first line (the wrapping context or command), the last
// non-empty line (CLI installers end with `error: …`), and how many lines
// were elided between them. Each returned line is truncated to width.
// Nothing here loses information — the detail portal renders the untrimmed
// text.
func errorPeekLines(errText string, width int) []string {
	lines := nonEmptyLines(errText)
	switch len(lines) {
	case 0:
		return nil
	case 1, 2:
		out := make([]string, len(lines))
		for i, l := range lines {
			out[i] = truncateWithEllipsis(l, width)
		}
		return out
	}
	elided := len(lines) - 2
	return []string{
		truncateWithEllipsis(lines[0], width),
		truncateWithEllipsis(lines[len(lines)-1], width),
		truncateWithEllipsis(fmt.Sprintf("… %d more %s — ? for full output",
			elided, pluralize(elided, "line", "lines")), width),
	}
}

// errorPeekLast is the one-line clamp for live progress rows: the error's
// last non-empty line, truncated.
func errorPeekLast(errText string, width int) string {
	lines := nonEmptyLines(errText)
	if len(lines) == 0 {
		return ""
	}
	return truncateWithEllipsis(lines[len(lines)-1], width)
}

func nonEmptyLines(s string) []string {
	// Child-process output embeds carriage returns (spinner rewrites) and
	// ANSI sequences; both would corrupt the chrome — \r resets the cursor
	// to column 0. Treat \r as a line break and strip escapes entirely.
	s = ansi.Strip(s)
	var out []string
	for _, line := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' }) {
		if t := strings.TrimRight(line, " \t"); strings.TrimSpace(t) != "" {
			out = append(out, t)
		}
	}
	return out
}
