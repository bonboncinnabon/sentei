package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

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
			if m.menuItems != nil {
				m.view = menuView
				return m, nil
			}
			return m, tea.Quit
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
		fmt.Fprintf(&b, "  %s %s created with issues\n\n",
			styleIndicatorWarning.Render(indicatorWarning), branch)
	} else {
		fmt.Fprintf(&b, "  %s %s ready\n\n",
			styleIndicatorDone.Render(indicatorDone), branch)
	}
	if result != nil && result.Err != nil {
		for _, line := range errorPeekLines(result.Err.Error(), max(m.width-8, 20)) {
			fmt.Fprintf(&b, "      %s\n", styleError.Render(line))
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Path"), truncateWithEllipsis(wtPath, max(m.width-16, 20)))
	origin := "from " + base
	if result != nil && result.ExistingBranch != "" {
		origin = "existing branch, not created from " + base
		if result.ExistingBranch != branch {
			origin = fmt.Sprintf("tracks %s, not created from %s", result.ExistingBranch, base)
		}
	}
	fmt.Fprintf(&b, "    %-10s %s (%s)\n", styleDim.Render("Branch"), branch, origin)

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
				fmt.Fprintf(&b, "    %-10s %s %s\n", styleDim.Render(label), step.Name, status)
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
	// The cd line is the flow's one actionable artifact: never truncate it.
	for _, line := range m.wrapSummaryLine("cd " + wtPath) {
		fmt.Fprintf(&b, "    %s\n", line)
	}
	b.WriteString("\n")
	b.WriteString(m.createSummaryFooter())
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
	fmt.Fprintf(&b, "  %s %s creation failed\n\n",
		styleIndicatorFailed.Render(indicatorFailed), branch)

	failure := result.Err
	if failure == nil {
		_, step, _ := progress.FirstFailure(result.Phases)
		failure = step.Error
	}
	if failure != nil {
		for _, line := range m.wrapSummaryLine(failure.Error()) {
			fmt.Fprintf(&b, "    %s\n", styleError.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(m.createSummaryFooter())
	return b.String()
}

func (m Model) wrapSummaryLine(text string) []string {
	return strings.Split(lipgloss.NewStyle().Width(max(m.width-8, 20)).Render(text), "\n")
}

func (m Model) createSummaryFooter() string {
	if m.menuItems != nil {
		return viewFooter(m.width, summaryMenuFooter) + "\n"
	}
	return viewFooter(m.width, createSummaryQuit) + "\n"
}
