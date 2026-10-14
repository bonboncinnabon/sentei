package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// ConfirmProceedMsg is sent when the user presses Enter to confirm.
type ConfirmProceedMsg struct{}

// ConfirmBackMsg is sent when the user presses Esc to go back.
type ConfirmBackMsg struct{}

// ConfirmationItem represents a key-value row in the confirmation view.
type ConfirmationItem struct {
	Label string
	Value string
}

// ConfirmationViewModel holds the state for a reusable confirmation view.
type ConfirmationViewModel struct {
	Title      string
	Items      []ConfirmationItem
	CLICommand string
	Width      int // separator width; 0 falls back to viewSeparator's default
}

// UpdateConfirmation handles key messages for the confirmation view and returns
// the appropriate command. Returns nil if the message was not handled.
func UpdateConfirmation(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Confirm):
			return func() tea.Msg { return ConfirmProceedMsg{} }
		case key.Matches(msg, keys.Back):
			return func() tea.Msg { return ConfirmBackMsg{} }
		case key.Matches(msg, keys.Quit):
			return tea.Quit
		}
	}
	return nil
}

// updateLaunchConfirm handles the confirmation screen that a launch with flags
// opens (sentei cleanup, create, clone): enter starts the operation. Nothing
// is behind the screen, so esc quits like q.
func (m Model) updateLaunchConfirm(msg tea.Msg, start func(Model) (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, keys.Confirm):
			return start(m)
		case key.Matches(msg, keys.Back), key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the confirmation view using the standard chrome: a
// confirmation is a full-screen step in a flow, not a floating modal.
func (c ConfirmationViewModel) View() string {
	var b strings.Builder

	b.WriteString(viewTitle(c.Title))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(c.Width))
	b.WriteString("\n\n")

	if len(c.Items) > 0 {
		writeConfirmationItems(&b, c.Items, c.Width)
		b.WriteString("\n")
	}

	if c.CLICommand != "" {
		writeWrapped(&b, "  ", styleDim.Render(c.CLICommand), c.Width)
		b.WriteString("\n")
	}

	b.WriteString(viewSeparator(c.Width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(c.Width, confirmationFooter) + "\n")

	return b.String()
}

// writeConfirmationItems renders items as "label  value" rows with the values
// in one column. A value wider than the screen wraps under itself: the end of
// a path or a branch name is what tells two values apart.
func writeConfirmationItems(b *strings.Builder, items []ConfirmationItem, width int) {
	labelWidth := 0
	for _, item := range items {
		labelWidth = max(labelWidth, len(item.Label))
	}
	for _, item := range items {
		writeWrapped(b, fmt.Sprintf("  %-*s  ", labelWidth, item.Label), item.Value, width)
	}
}

// writeWrapped writes prefix and value, the value wrapped to width under its
// first line. Width 0 (no size message yet) writes the line as it is.
func writeWrapped(b *strings.Builder, prefix, value string, width int) {
	if width <= 0 {
		b.WriteString(prefix + value + "\n")
		return
	}
	writeProgressDetailValue(b, prefix, value, width)
}
