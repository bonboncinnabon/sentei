package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func (m Model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Confirm), key.Matches(msg, keys.Back):
			m.view = menuView
		}
	}
	return m, nil
}

func (m Model) renderRemovalSummary() string {
	var b strings.Builder

	b.WriteString(viewTitle(titleRemovalComplete))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	r := m.remove.run.result
	if !r.HasFailures() {
		fmt.Fprintf(&b, "  %s %s\n",
			styleIndicatorDone.Render(indicatorDone),
			styleSuccess.Render(fmt.Sprintf("%d %s removed successfully", r.SuccessCount, pluralize(r.SuccessCount, "worktree", "worktrees"))))
	} else {
		fmt.Fprintf(&b, "  %s, %s\n",
			styleSuccess.Render(fmt.Sprintf("%d removed", r.SuccessCount)),
			styleError.Render(fmt.Sprintf("%d failed", r.FailureCount)),
		)
		mark := styleIndicatorFailed.Render(indicatorFailed)
		if r.FailureCount > 0 {
			b.WriteString("\n")
			b.WriteString(styleError.Render("  Removal failures:") + "\n")
			for _, o := range r.Outcomes {
				if !o.Success {
					writeHanging(&b, "    "+mark+" ", lipgloss.NewStyle(), fmt.Sprintf("%s: %v", o.Path, o.Error))
				}
			}
		}
		if r.Err != nil {
			b.WriteString("\n")
			writeHanging(&b, "  "+mark+" ", styleError, r.Err.Error())
		}
		// A failed step of the removal phase is one of the outcomes above.
		var steps strings.Builder
		for _, phase := range r.Phases {
			for _, step := range phase.Steps {
				if step.Status == progress.StepFailed && phase.ID != worktree.RemovalPhaseID {
					writeHanging(&steps, "    "+mark+" ", lipgloss.NewStyle(), fmt.Sprintf("%s / %s: %v", phase.Name, step.Name, step.Error))
				}
			}
		}
		if steps.Len() > 0 {
			b.WriteString("\n")
			b.WriteString(styleError.Render("  Execution failures:") + "\n")
			b.WriteString(steps.String())
		}
	}

	switch err := m.remove.run.pruneErr; {
	case err != nil:
		b.WriteString("\n")
		writeHanging(&b, "  ", styleWarning, fmt.Sprintf("Warning: failed to prune worktree metadata: %s", err))
		b.WriteString(styleDim.Render("  Run `git worktree prune` in the repository to retry."))
		b.WriteString("\n")
	case m.remove.run.pruned:
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  Pruned orphaned worktree metadata"))
		b.WriteString("\n")
	}

	if m.remove.cliCommand != "" {
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  " + m.remove.cliCommand))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, summaryMenuFooter))
	b.WriteString("\n")

	return b.String()
}

// writeHanging writes text after prefix, one row per line of text: the first
// line follows prefix and every further line starts in the same column, so a
// multi-line git error never restarts at column 0. Each line is styled on its
// own because a style rendered across a newline pads every line to the widest.
func writeHanging(b *strings.Builder, prefix string, style lipgloss.Style, text string) {
	for i, line := range nonEmptyLines(text) {
		if i == 1 {
			prefix = strings.Repeat(" ", lipgloss.Width(prefix))
		}
		b.WriteString(prefix + style.Render(line) + "\n")
	}
}

func (m Model) viewSummary() string {
	return boundRemovalSummary(m.renderRemovalSummary(), m.width, m.progressHeight())
}

func boundRemovalSummary(summary string, width, height int) string {
	lines := strings.Split(strings.TrimSuffix(summary, "\n"), "\n")
	widthOmitted := false
	for i, line := range lines {
		if fit := truncateWithEllipsis(line, width); fit != line {
			widthOmitted = true
			lines[i] = fit
		}
	}
	if widthOmitted {
		const footerRows = 4
		at := max(len(lines)-footerRows, 0)
		hint := truncateWithEllipsis("  ? details for omitted text", width)
		lines = append(lines, "")
		copy(lines[at+1:], lines[at:])
		lines[at] = hint
	}
	if !widthOmitted && (height <= 0 || len(lines) <= height) {
		return summary
	}
	if height <= 0 || len(lines) <= height {
		return strings.Join(lines, "\n")
	}
	const footerRows = 4
	if height <= footerRows {
		return strings.Join(lines[len(lines)-height:], "\n")
	}
	previewRows := height - footerRows - 1
	omitted := len(lines) - previewRows - footerRows
	visible := append([]string(nil), lines[:previewRows]...)
	visible = append(visible, truncateWithEllipsis(fmt.Sprintf("  ? details for %d omitted %s", omitted, pluralize(omitted, "line", "lines")), width))
	visible = append(visible, lines[len(lines)-footerRows:]...)
	return strings.Join(visible, "\n")
}

func (m Model) removalSummaryDetailContent() (string, string) {
	full := m.renderRemovalSummary()
	lines := strings.Split(strings.TrimSuffix(full, "\n"), "\n")
	omitted := m.progressHeight() > 0 && len(lines) > m.progressHeight()
	for _, line := range lines {
		omitted = omitted || truncateWithEllipsis(line, m.width) != line
	}
	if !omitted {
		return "", ""
	}
	var detail strings.Builder
	for _, line := range lines {
		// A wrapped row continues under its own text: past the row's indent
		// and, on a failure row, past the mark.
		plain := ansi.Strip(line)
		lead := len(plain) - len(strings.TrimLeft(plain, " "))
		if strings.HasPrefix(plain[lead:], indicatorFailed+" ") {
			lead += 2
		}
		writeProgressDetailValue(&detail, ansi.Cut(line, 0, lead), ansi.Cut(line, lead, ansi.StringWidth(line)), m.portal.contentWidth())
	}
	return "Removal details", strings.TrimSuffix(detail.String(), "\n")
}
