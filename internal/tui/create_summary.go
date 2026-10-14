package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

func (m Model) updateCreateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Confirm):
			m.view = menuView
			return m, nil
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewCreateSummary() string {
	var b strings.Builder

	branch := m.create.branchInput.Value()
	base := m.create.baseInput.Value()

	result := m.create.result
	wtPath := git.WorktreePath(m.repoPath, branch)
	if result != nil {
		wtPath = result.WorktreePath
	}
	// Nothing on disk: no "created" title, no path and no cd line to offer.
	if wtPath == "" {
		return m.viewCreateFailed(branch, result)
	}

	b.WriteString(viewTitle(titleWorktreeCreated))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	if result != nil && result.HasFailures() {
		b.WriteString(m.headline(styleIndicatorWarning.Render(indicatorWarning), branch, "created with issues"))
	} else {
		b.WriteString(m.headline(styleIndicatorDone.Render(indicatorDone), branch, "ready"))
	}
	if result != nil && result.Err != nil {
		for _, line := range errorPeekLines(result.Err.Error(), max(m.width-8, 20)) {
			fmt.Fprintf(&b, "      %s\n", styleError.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString(summaryPathRow("Path", wtPath, m.width))
	existing := ""
	if result != nil {
		existing = result.ExistingBranch
	}
	b.WriteString(summaryRow("Branch", fmt.Sprintf("%s (%s)", branch, branchOrigin(branch, base, existing)), m.width))

	// Dependency steps are always listed; a setup step only when it failed,
	// so "with issues" names its cause.
	if result != nil {
		for _, phase := range result.Phases {
			label := "Setup"
			if phase.Name == "Dependencies" {
				label = "Deps"
			}
			for _, step := range phase.Steps {
				failed := step.Status == progress.StepFailed
				if !failed && label == "Setup" {
					continue
				}
				status := styleIndicatorDone.Render(indicatorDone)
				if failed {
					status = styleIndicatorFailed.Render(indicatorFailed)
				}
				b.WriteString(summaryRow(label, step.Name+" "+status, m.width))
				if failed && step.Error != nil {
					peek := errorPeekLines(step.Error.Error(), max(m.width-8, 20))
					for i, line := range peek {
						style := styleDim
						if i == 1 || len(peek) == 1 {
							style = styleError
						}
						fmt.Fprintf(&b, "      %s\n", style.Render(line))
					}
				}
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(m.cdLine(wtPath))
	b.WriteString("\n")
	b.WriteString(viewFooter(m.width, summaryMenuFooter) + "\n")
	return b.String()
}

// cdLine is the command that takes the user into what a flow created. It is
// the flow's one actionable artifact, so it is never truncated: it breaks as
// shellLines does, so a mouse copy still runs, with the home directory as ~.
func (m Model) cdLine(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+string(filepath.Separator)) {
		path = "~" + path[len(home):]
	}
	var b strings.Builder
	for _, line := range shellLines("cd "+path, "    ", m.width) {
		b.WriteString(line + "\n")
	}
	return b.String()
}

// viewCreateFailed is the summary for a run that left no worktree behind. The
// error is the screen's whole content, so it wraps instead of being clipped.
func (m Model) viewCreateFailed(branch string, result *creator.Result) string {
	var b strings.Builder

	b.WriteString(viewTitle(titleCreateWorktree))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(m.headline(styleIndicatorFailed.Render(indicatorFailed), branch, "creation failed"))

	failure := result.Err
	if failure == nil {
		_, step, _ := progress.FirstFailure(result.Phases)
		failure = step.Error
	}
	if failure != nil {
		b.WriteString(m.errorBlock(failure))
		b.WriteString("\n")
	}

	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, summaryMenuFooter) + "\n")
	return b.String()
}

// headline renders a summary's verdict line, "✦ <name> ready", and its blank
// line. The name shares the line with the verdict, so it gives up its middle
// to fit (fitNameMiddle). Width 0 means unknown.
func (m Model) headline(indicator, name, verdict string) string {
	if m.width > 0 {
		name = fitNameMiddle(name, m.width-ansi.StringWidth("  "+indicator+"  "+verdict))
	}
	return fmt.Sprintf("  %s %s %s\n\n", indicator, name, verdict)
}

// errorBlock renders an error as the body of a failure screen: wrapped to the
// terminal, with every line under the same indent.
func (m Model) errorBlock(err error) string {
	var b strings.Builder
	for _, line := range m.wrapSummaryLine(err.Error()) {
		fmt.Fprintf(&b, "    %s\n", styleError.Render(line))
	}
	return b.String()
}

// dimParagraph renders prose wrapped to the terminal under a two-space indent.
func (m Model) dimParagraph(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(lipgloss.NewStyle().Width(max(m.width-4, 20)).Render(text), "\n") {
		b.WriteString(styleDim.Render("  "+strings.TrimRight(line, " ")) + "\n")
	}
	return b.String()
}

func (m Model) wrapSummaryLine(text string) []string {
	return strings.Split(lipgloss.NewStyle().Width(max(m.width-8, 20)).Render(text), "\n")
}
