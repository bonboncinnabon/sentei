package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

func (m Model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Confirm):
			if m.menuItems != nil {
				m.view = menuView
				return m, nil
			}
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			if m.menuItems != nil {
				m.view = menuView
				return m, nil
			}
			return m, tea.Quit
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
		if r.FailureCount > 0 {
			b.WriteString("\n")
			b.WriteString(styleError.Render("  Removal failures:\n"))
			for _, o := range r.Outcomes {
				if !o.Success {
					fmt.Fprintf(&b, "    %s %s: %s\n",
						styleIndicatorFailed.Render(indicatorFailed),
						o.Path,
						fmt.Sprint(o.Error))
				}
			}
		}
		if r.Err != nil {
			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s %s\n", styleIndicatorFailed.Render(indicatorFailed), styleError.Render(r.Err.Error()))
		}
		if progress.PhasesHaveFailures(r.Phases) {
			b.WriteString("\n")
			b.WriteString(styleError.Render("  Execution failures:\n"))
			for _, phase := range r.Phases {
				for _, step := range phase.Steps {
					if step.Status == progress.StepFailed {
						fmt.Fprintf(&b, "    %s %s / %s: %s\n", styleIndicatorFailed.Render(indicatorFailed), phase.Name, step.Name, step.Error)
					}
				}
			}
		}
	}

	b.WriteString("\n")
	if m.remove.run.pruneErr != nil && *m.remove.run.pruneErr != nil {
		b.WriteString(styleWarning.Render(fmt.Sprintf("  Warning: failed to prune worktree metadata: %s", *m.remove.run.pruneErr)))
		b.WriteString("\n")
	} else {
		b.WriteString(styleDim.Render("  Pruned orphaned worktree metadata"))
		b.WriteString("\n")
	}

	if cr := m.remove.run.cleanupResult; cr != nil {
		if len(cr.Errors) > 0 {
			b.WriteString("\n")
			b.WriteString(styleError.Render("  Cleanup failures:"))
			b.WriteString("\n")
			for _, operationErr := range cr.Errors {
				fmt.Fprintf(&b, "    %s %s: %s\n", styleIndicatorFailed.Render(indicatorFailed), operationErr.Step, operationErr.Err)
			}
		}
		cleanupActions := cr.StaleRefsRemoved + cr.ConfigDedupResult.Removed +
			cr.GoneBranchesDeleted + cr.ConfigOrphanResult.Removed
		if cleanupActions > 0 {
			b.WriteString("\n")
			b.WriteString(styleDim.Render("  Cleanup:"))
			b.WriteString("\n")
			if cr.StaleRefsRemoved > 0 {
				fmt.Fprintf(&b, "    %s Pruned %d remote %s\n", styleIndicatorDone.Render(indicatorDone), cr.StaleRefsRemoved, pluralize(cr.StaleRefsRemoved, "ref", "refs"))
			}
			if cr.ConfigDedupResult.Removed > 0 {
				fmt.Fprintf(&b, "    %s Removed %d config %s\n", styleIndicatorDone.Render(indicatorDone), cr.ConfigDedupResult.Removed, pluralize(cr.ConfigDedupResult.Removed, "duplicate", "duplicates"))
			}
			if cr.GoneBranchesDeleted > 0 {
				fmt.Fprintf(&b, "    %s Deleted %d %s with gone upstream\n", styleIndicatorDone.Render(indicatorDone), cr.GoneBranchesDeleted, pluralize(cr.GoneBranchesDeleted, "branch", "branches"))
			}
			if cr.ConfigOrphanResult.Removed > 0 {
				fmt.Fprintf(&b, "    %s Removed %d orphaned config %s\n", styleIndicatorDone.Render(indicatorDone), cr.ConfigOrphanResult.Removed, pluralize(cr.ConfigOrphanResult.Removed, "section", "sections"))
			}
		}
		if cr.NonWtBranchesRemaining > 0 {
			b.WriteString("\n")
			b.WriteString(styleDim.Render(fmt.Sprintf("  Tip: %d local %s not in any worktree.", cr.NonWtBranchesRemaining, pluralize(cr.NonWtBranchesRemaining, "branch", "branches"))))
			b.WriteString("\n")
			b.WriteString(styleDim.Render("       Run `sentei cleanup --mode aggressive` to remove them (unmerged branches need --force)."))
			b.WriteString("\n")
		}
	}

	if m.remove.cliCommand != "" {
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  " + m.remove.cliCommand))
		b.WriteString("\n")
	}

	if m.remove.milestone > 0 {
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  " + fmt.Sprintf(whisperMilestone, m.remove.milestone)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	if m.menuItems != nil {
		b.WriteString(viewFooter(m.width, summaryMenuFooter))
	} else {
		b.WriteString(viewFooter(m.width, summaryQuitFooter))
	}
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewSummary() string {
	return boundRemovalSummary(m.renderRemovalSummary(), m.width, m.progressHeight())
}

func boundRemovalSummary(summary string, width, height int) string {
	lines := strings.Split(strings.TrimSuffix(summary, "\n"), "\n")
	widthOmitted := false
	for i, line := range lines {
		if fit := fitProgressLine(line, width); fit != line {
			widthOmitted = true
			lines[i] = fit
		}
	}
	if widthOmitted {
		const footerRows = 4
		at := max(len(lines)-footerRows, 0)
		hint := fitProgressLine("  ? details for omitted text", width)
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
	visible = append(visible, fitProgressLine(fmt.Sprintf("  ? details for %d omitted %s", omitted, pluralize(omitted, "line", "lines")), width))
	visible = append(visible, lines[len(lines)-footerRows:]...)
	return strings.Join(visible, "\n")
}

func (m Model) removalSummaryDetailContent() (string, string) {
	full := m.renderRemovalSummary()
	lines := strings.Split(strings.TrimSuffix(full, "\n"), "\n")
	omitted := m.progressHeight() > 0 && len(lines) > m.progressHeight()
	for _, line := range lines {
		omitted = omitted || fitProgressLine(line, m.width) != line
	}
	if !omitted {
		return "", ""
	}
	var detail strings.Builder
	for _, line := range lines {
		writeProgressDetailValue(&detail, "", line, m.portal.contentWidth())
	}
	return "Removal details", strings.TrimSuffix(detail.String(), "\n")
}
