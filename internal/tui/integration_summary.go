package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

type integrationStepOutcome struct {
	step string
	ev   progress.Event
}

type integrationWorktreeOutcomes struct {
	worktree string
	steps    []integrationStepOutcome
	closed   bool
}

// groupIntegrationEvents projects the canonical progress snapshot into the
// summary's presentation shape without recomputing status, counts, or closure.
func groupIntegrationEvents(events []progress.Event) []integrationWorktreeOutcomes {
	states := progress.Snapshot(events)
	groups := make([]integrationWorktreeOutcomes, 0, len(states))
	for _, phase := range states {
		group := integrationWorktreeOutcomes{worktree: phase.Name, closed: phase.Closed}
		for _, step := range phase.Steps {
			group.steps = append(group.steps, integrationStepOutcome{
				step: step.Name,
				ev:   progress.Event{Status: step.Status, Message: step.Message, Error: step.Error},
			})
		}
		groups = append(groups, group)
	}
	return groups
}

func (m Model) updateIntegrationSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Confirm), key.Matches(msg, keys.Back):
			m.integ.lifecycle = integrationIdle
			if m.integ.returnView == migrateNextView {
				m.view = migrateNextView
				return m, nil
			}
			// Reload from disk so the list's active/staged markers always
			// match persisted state, on success and save-failure alike.
			m.view = integrationListView
			return m, m.loadIntegrationState()
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

// inlineSummaryPreview is how many worktree outcomes the apply summary shows
// inline before deferring the rest to the detail portal (`?`).
const inlineSummaryPreview = 3

// countIntegrationOutcomes tallies done and failed steps across all groups.
func countIntegrationOutcomes(groups []integrationWorktreeOutcomes) (applied, failed int) {
	for _, g := range groups {
		for _, s := range g.steps {
			switch s.ev.Status {
			case progress.StepDone:
				applied++
			case progress.StepFailed:
				failed++
			}
		}
	}
	return applied, failed
}

func groupHasFailure(g integrationWorktreeOutcomes) bool {
	for _, s := range g.steps {
		if s.ev.Status == progress.StepFailed {
			return true
		}
	}
	return false
}

// orderOutcomesFailuresFirst moves worktrees with failures ahead of clean ones
// so the summary's limited inline space surfaces problems first. Order within
// each partition is preserved.
func orderOutcomesFailuresFirst(groups []integrationWorktreeOutcomes) []integrationWorktreeOutcomes {
	ordered := make([]integrationWorktreeOutcomes, 0, len(groups))
	for _, g := range groups {
		if groupHasFailure(g) {
			ordered = append(ordered, g)
		}
	}
	for _, g := range groups {
		if !groupHasFailure(g) {
			ordered = append(ordered, g)
		}
	}
	return ordered
}

// renderIntegrationOutcomes writes the per-worktree breakdown to b: a worktree
// header followed by a `✦`/`✗` line per resolved step. Shared by the inline
// summary peek and the detail portal.
// width bounds failure peeks; width <= 0 renders the full untrimmed error
// (the detail portal's mode — nothing is lost, only deferred).
func renderIntegrationOutcomes(b *strings.Builder, groups []integrationWorktreeOutcomes, width int) {
	for _, g := range groups {
		fmt.Fprintf(b, "  %s\n", filepath.Base(g.worktree))
		for _, s := range g.steps {
			switch s.ev.Status {
			case progress.StepDone:
				fmt.Fprintf(b, "    %s %s\n", styleIndicatorDone.Render(indicatorDone), s.step)
			case progress.StepSkipped:
				reason := ""
				if s.ev.Message != "" {
					reason = " (" + s.ev.Message + ")"
				}
				fmt.Fprintf(b, "    %s\n", styleDim.Render("– "+s.step+" – skipped"+reason))
			case progress.StepFailed:
				fmt.Fprintf(b, "    %s %s\n", styleIndicatorFailed.Render(indicatorFailed), s.step)
				if s.ev.Error == nil {
					continue
				}
				if width <= 0 {
					for _, line := range nonEmptyLines(s.ev.Error.Error()) {
						fmt.Fprintf(b, "      %s\n", styleError.Render(line))
					}
					continue
				}
				peek := errorPeekLines(s.ev.Error.Error(), max(width-8, 20))
				for i, line := range peek {
					style := styleDim
					if i == 1 || len(peek) == 1 {
						style = styleError
					}
					fmt.Fprintf(b, "      %s\n", style.Render(line))
				}
			}
		}
		b.WriteString("\n")
	}
}

// integrationSummaryDetailContent renders the full per-worktree breakdown for
// the detail portal, exposed when outcomes overflow the inline peek or any
// failure carries error output (the inline peek promises "? for full output").
func (m Model) integrationSummaryDetailContent() (string, string) {
	groups := orderOutcomesFailuresFirst(groupIntegrationEvents(m.integ.events))
	var b strings.Builder
	for _, stage := range []struct {
		label string
		err   error
	}{
		{"Preparation", m.integ.prepareErr},
		{"Execution", m.integ.executionErr},
		{"Saving state", m.integ.saveErr},
	} {
		if stage.err == nil {
			continue
		}
		writeProgressDetailValue(&b, "", styleError.Render(stage.label+" failed:"), m.portal.contentWidth())
		writeProgressDetailValue(&b, "  ", stage.err.Error(), m.portal.contentWidth())
		b.WriteString("\n")
	}
	if b.Len() == 0 && len(groups) <= inlineSummaryPreview && !outcomesHaveErrorOutput(groups) {
		return "", ""
	}
	var outcomes strings.Builder
	renderIntegrationOutcomes(&outcomes, groups, 0)
	for _, line := range strings.Split(strings.TrimRight(outcomes.String(), "\n"), "\n") {
		writeProgressDetailValue(&b, "", line, m.portal.contentWidth())
	}
	return portalApplyDetails, strings.TrimRight(b.String(), "\n")
}

func (m Model) viewIntegrationSummary() string {
	groups := orderOutcomesFailuresFirst(groupIntegrationEvents(m.integ.events))
	applied, failed := countIntegrationOutcomes(groups)

	// "Complete" would oversell a run with failures: the title states the
	// outcome, and the headline leads with the count that matters.
	title := titleApplyComplete
	if failed > 0 || m.integ.prepareErr != nil || m.integ.executionErr != nil || m.integ.saveErr != nil {
		title = titleApplyErrors
	}
	header := []string{viewTitle(title), "", viewSeparator(m.width), ""}

	if m.integ.saveErr != nil {
		header = append(header, styleError.Render(truncateWithEllipsis(
			fmt.Sprintf("  %s Integration state was not saved: %s", indicatorFailed, m.integ.saveErr),
			max(m.width, 40))), styleDim.Render("    The list will show what is actually on disk."), "")
	}
	if m.integ.prepareErr != nil {
		header = append(header, styleError.Render(truncateWithEllipsis(
			fmt.Sprintf("  %s Integration preparation failed: %s", indicatorFailed, m.integ.prepareErr),
			max(m.width, 40))), "")
	}
	if m.integ.executionErr != nil {
		header = append(header, styleError.Render(truncateWithEllipsis(
			fmt.Sprintf("  %s Integration execution failed: %s", indicatorFailed, m.integ.executionErr),
			max(m.width, 40))), "")
	}

	if m.integ.prepareErr == nil && m.integ.executionErr == nil && m.integ.saveErr == nil {
		var verdict strings.Builder
		switch {
		case failed > 0:
			verdict.WriteString("  ")
			verdict.WriteString(styleError.Render(fmt.Sprintf("%s %d failed", indicatorFailed, failed)))
			if applied > 0 {
				verdict.WriteString(", ")
				verdict.WriteString(styleSuccess.Render(fmt.Sprintf("%d %s applied", applied, pluralize(applied, "step", "steps"))))
			}
		case applied == 0:
			verdict.WriteString(styleDim.Render("  No integration work was needed"))
		default:
			verdict.WriteString(styleSuccess.Render(fmt.Sprintf("  %s %d %s applied", indicatorDone, applied, pluralize(applied, "step", "steps"))))
		}
		header = append(header, verdict.String(), "")
	}

	hints := []key.Binding{integrationsOpenHint}
	if _, detail := m.integrationSummaryDetailContent(); detail != "" {
		hints = append(hints, detailsHint)
	}
	hints = append(hints, keys.Quit)
	footer := []string{viewSeparator(m.width), "", viewFooter(m.width, hints)}
	height := m.windowHeight
	if height <= 0 {
		height = m.height
	}
	header, footer = compactIntegrationSummaryChrome(header, footer, height)

	lines := append([]string(nil), header...)
	available := max(height-len(header)-len(footer), 0)
	shown := 0
	limit := min(len(groups), inlineSummaryPreview)
	for shown < limit {
		block := inlineIntegrationGroupLines(groups[shown], m.width, false)
		reserve := 0
		if shown+1 < len(groups) {
			reserve = 2
		}
		if len(block)+reserve > available {
			break
		}
		lines = append(lines, block...)
		available -= len(block)
		shown++
	}
	if shown == 0 && len(groups) > 0 && groupHasFailure(groups[0]) {
		block := inlineIntegrationGroupLines(groups[0], m.width, true)
		if len(block) <= available {
			lines = append(lines, block...)
			available -= len(block)
			shown = 1
		}
	}
	if rest := len(groups) - shown; rest > 0 && available >= 2 {
		lines = append(lines, styleDim.Render(fmt.Sprintf("  and %d more %s — ? for details",
			rest, pluralize(rest, "worktree", "worktrees"))), "")
	}
	lines = append(lines, footer...)
	for i := range lines {
		lines[i] = fitProgressLine(lines[i], m.width)
	}
	return strings.Join(lines, "\n")
}

func compactIntegrationSummaryChrome(header, footer []string, height int) ([]string, []string) {
	for len(header)+len(footer) > height {
		removed := false
		for i := len(header) - 1; i >= 0; i-- {
			if header[i] == "" {
				header = append(header[:i], header[i+1:]...)
				removed = true
				break
			}
		}
		if removed {
			continue
		}
		for i := len(footer) - 1; i >= 0; i-- {
			if footer[i] == "" {
				footer = append(footer[:i], footer[i+1:]...)
				removed = true
				break
			}
		}
		if removed {
			continue
		}
		// The top separator is decorative; the fixed footer separator remains.
		if len(header) > 1 {
			header = append(header[:1], header[2:]...)
			continue
		}
		// In emergency tiers, keep the title and action footer before optional
		// error prose, then keep the title alone at one row.
		if len(header) > 1 {
			header = header[:len(header)-1]
			continue
		}
		if len(footer) > 1 {
			footer = footer[1:]
			continue
		}
		if len(footer) == 1 {
			footer = nil
			continue
		}
		break
	}
	return header, footer
}

func inlineIntegrationGroupLines(group integrationWorktreeOutcomes, width int, concise bool) []string {
	var b strings.Builder
	if !concise {
		renderIntegrationOutcomes(&b, []integrationWorktreeOutcomes{group}, width)
	} else {
		fmt.Fprintf(&b, "  %s\n", filepath.Base(group.worktree))
		for _, step := range group.steps {
			if step.ev.Status == progress.StepFailed {
				fmt.Fprintf(&b, "    %s %s\n", styleIndicatorFailed.Render(indicatorFailed), step.step)
			}
		}
		b.WriteString("\n")
	}
	text := strings.TrimSuffix(b.String(), "\n")
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = fitProgressLine(lines[i], width)
	}
	return lines
}

// outcomesHaveErrorOutput reports whether any failed step carries error text
// worth a portal visit.
func outcomesHaveErrorOutput(groups []integrationWorktreeOutcomes) bool {
	for _, g := range groups {
		for _, s := range g.steps {
			if s.ev.Status == progress.StepFailed && s.ev.Error != nil {
				return true
			}
		}
	}
	return false
}
