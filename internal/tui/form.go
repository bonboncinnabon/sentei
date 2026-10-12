package tui

import (
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
func fitPathTail(path string, width int) string {
	if over := ansi.StringWidth(path) - max(width, 1); over > 0 {
		return ansi.TruncateLeft(path, over+1, "…")
	}
	return path
}

// destinationPreview is the dim line under a form that names the directory
// about to be created.
func destinationPreview(dest string, width int) string {
	const indent = "    → "
	return styleDim.Render(indent + fitPathTail(dest, max(width, 20)-ansi.StringWidth(indent)))
}
