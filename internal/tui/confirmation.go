package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/bonboncinnabon/sentei/internal/cli"
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

// View renders the confirmation view using the standard chrome: a
// confirmation is a full-screen step in a flow, not a floating modal.
func (c ConfirmationViewModel) View() string {
	var b strings.Builder

	b.WriteString(viewTitle(c.Title))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(c.Width))
	b.WriteString("\n\n")

	if len(c.Items) > 0 {
		labelWidth := 0
		for _, item := range c.Items {
			if len(item.Label) > labelWidth {
				labelWidth = len(item.Label)
			}
		}

		for _, item := range c.Items {
			fmt.Fprintf(&b, "  %-*s  %s\n", labelWidth, item.Label, item.Value)
		}
		b.WriteString("\n")
	}

	if c.CLICommand != "" {
		b.WriteString(styleDim.Render("  "+c.CLICommand) + "\n\n")
	}

	b.WriteString(viewSeparator(c.Width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(c.Width, confirmationFooter) + "\n")

	return b.String()
}

// BuildCLICommand constructs a CLI command string from a command name and flags.
// Delegates to cli.BuildFlagString for consistent flag formatting.
func BuildCLICommand(command string, flags map[string]string) string {
	return cli.BuildFlagString("sentei "+command, flags)
}
