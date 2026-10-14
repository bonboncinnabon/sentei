package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The entry screens (create worktree, create repository, clone) are forms of
// two text inputs with a focus index. These functions are the behaviour they
// share.

// switchFormField moves focus to the other input, with the cursor at the end
// of its text.
func switchFormField(focused *int, first, second *textinput.Model) tea.Cmd {
	from, to := first, second
	if *focused == 1 {
		from, to = second, first
	}
	*focused = 1 - *focused
	from.Blur()
	to.CursorEnd()
	return to.Focus()
}

// editFormField forwards msg to the focused input and reports whether its
// text changed. Everything a form does in response to typing (clearing a
// validation error, deriving another field) hangs off that report, because
// the messages arriving here also include the cursor blink and mouse motion.
func editFormField(focused int, first, second *textinput.Model, msg tea.Msg) (tea.Cmd, bool) {
	input := first
	if focused == 1 {
		input = second
	}
	before := input.Value()
	var cmd tea.Cmd
	*input, cmd = input.Update(msg)
	return cmd, input.Value() != before
}

// fitPathTail fits a path into width cells by eliding its head. On these
// screens the tail names what is being created, so it is the part to keep.
// The cut moves forward to a directory boundary (…/platform/feature-x) unless
// the last element alone does not fit.
func fitPathTail(path string, width int) string {
	over := ansi.StringWidth(path) - max(width, 1)
	if over <= 0 {
		return path
	}
	tail := ansi.TruncateLeft(path, over+1, "")
	if tail == "" {
		return "" // no room for even the ellipsis
	}
	if slash := strings.Index(tail, "/"); slash >= 0 && slash < len(tail)-1 {
		tail = tail[slash:]
	}
	return "…" + tail
}

// fitNameMiddle fits a branch name into width cells on a line it shares with
// other text. The first segment (feature/) says what kind of work it is and
// the tail tells two branches apart, so the middle gives way, from a break
// point: feature/…multi-currency-ledgers. A name with no segment, or no room
// for one, keeps its tail.
func fitNameMiddle(name string, width int) string {
	if ansi.StringWidth(name) <= width {
		return name
	}
	slash := strings.Index(name, "/")
	room := width - slash - 2 // the head, its slash and the ellipsis
	if slash < 0 || room < 8 {
		return fitPathTail(name, width)
	}
	rest := name[slash+1:]
	tail := ansi.TruncateLeft(rest, ansi.StringWidth(rest)-room, "")
	if brk := strings.IndexAny(tail, "-_/"); brk >= 0 && brk < len(tail)-1 {
		tail = tail[brk+1:]
	}
	return name[:slash+1] + "…" + tail
}

// formErrorLine renders a form's validation message on one row. A message
// that ends in a path ("directory already exists: /a/b/c") gives up the head
// of the path first, like destinationPreview: the last element is the
// directory the message is about. Width 0 means unknown.
func formErrorLine(message string, width int) string {
	const indent = "  "
	if room := width - len(indent); width > 0 {
		if at := strings.LastIndex(message, ": "); at >= 0 {
			head := message[:at+2]
			message = head + fitPathTail(message[at+2:], room-ansi.StringWidth(head))
		}
		message = truncateWithEllipsis(message, room)
	}
	return indent + styleError.Render(message)
}

// destinationPreview is the dim line under a form that names the directory
// about to be created.
func destinationPreview(dest string, width int) string {
	const indent = "    → "
	return styleDim.Render(indent + fitPathTail(dest, max(width, 20)-ansi.StringWidth(indent)))
}
