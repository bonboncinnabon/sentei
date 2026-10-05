package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

type migrateIntegrationDetectedMsg struct {
	integrations []integration.Integration
	detected     map[string]bool
}

func loadMigrateIntegrations(worktreePath string) tea.Cmd {
	return func() tea.Msg {
		all := integration.All()
		detected := integration.DetectAllPresent(worktreePath, all)
		return migrateIntegrationDetectedMsg{
			integrations: all,
			detected:     detected,
		}
	}
}

func (m Model) updateMigrateIntegrations(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case migrateIntegrationDetectedMsg:
		m.integ.integrations = msg.integrations
		m.integ.detected = msg.detected
		m.integ.current = make(map[string]bool) // nothing "currently" enabled during onboarding
		m.integ.staged = make(map[string]bool)
		for _, integ := range msg.integrations {
			m.integ.staged[integ.Name] = msg.detected[integ.Name]
		}
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Down):
			if len(m.integ.integrations) > 0 && m.integ.cursor < len(m.integ.integrations)-1 {
				m.integ.cursor++
			}

		case key.Matches(msg, keys.Up):
			if m.integ.cursor > 0 {
				m.integ.cursor--
			}

		case key.Matches(msg, keys.Toggle):
			if len(m.integ.integrations) > 0 {
				name := m.integ.integrations[m.integ.cursor].Name
				m.integ.staged[name] = !m.integ.staged[name]
			}

		case key.Matches(msg, keys.Confirm):
			hasStagedSelections := false
			for _, v := range m.integ.staged {
				if v {
					hasStagedSelections = true
					break
				}
			}
			if hasStagedSelections {
				m.integ.events = nil
				m.integ.lifecycle = integrationIdle
				m.integ.returnView = migrateNextView
				m.progressStartedAt = time.Now()
				m.progressToken++
				m.view = integrationProgressView
				updated, cmd := m.startMigrateIntegrationApply()
				return updated, cmd
			}
			m.view = migrateNextView
			return m, nil

		case key.Matches(msg, keys.Back):
			m.view = migrateNextView
			return m, nil
		}
	}
	return m, nil
}

func (m Model) startMigrateIntegrationApply() (Model, tea.Cmd) {
	m.integ.prepareErr = nil
	m.integ.executionErr = nil
	m.integ.saveErr = nil
	result, ok := m.repo.result.(repo.MigrateResult)
	if !ok {
		m.integ.prepareErr = errors.New("preparing migration integrations: missing migration result")
		m.integ.lifecycle = integrationSettling
		updated, cmd := m.holdOrAdvance(integrationSummaryView)
		return updated.(Model), cmd
	}

	var toEnable []integration.Integration
	for _, integ := range m.integ.integrations {
		if m.integ.staged[integ.Name] {
			toEnable = append(toEnable, integ)
		}
	}

	wtPath := m.migrateWorktreePath(result)
	m.integ.targetWorktrees = []string{wtPath}
	m.integ.lifecycle = integrationPreparing

	repoPath := result.BareRoot
	shell := m.shell
	return m, func() tea.Msg {
		prepared, err := integration.PrepareApply(shell, repoPath, wtPath, toEnable, nil, []string{wtPath})
		return integrationPreparedMsg{prepared: prepared, err: err}
	}
}

func (m Model) viewMigrateIntegrations() string {
	var b strings.Builder

	b.WriteString(viewTitle(titleSetUpIntegrations))
	b.WriteString("\n\n")
	b.WriteString("  We detected your repo may benefit from\n")
	b.WriteString("  these dev tools. Select any to enable.\n")
	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	for i, integ := range m.integ.integrations {
		cursor := "  "
		if i == m.integ.cursor {
			cursor = "▸ "
		}

		staged := m.integ.staged[integ.Name]

		var checkbox string
		if staged {
			checkbox = styleCheckboxOn.Render("[x]")
		} else {
			checkbox = styleCheckboxOff.Render("[ ]")
		}

		detectedHint := ""
		if m.integ.detected[integ.Name] {
			detectedHint = "  " + styleDim.Render("detected")
		}

		if i == m.integ.cursor {
			b.WriteString(styleAccent.Render(cursor) + checkbox + " " + integ.Name + detectedHint)
		} else {
			b.WriteString("  " + checkbox + " " + integ.Name + detectedHint)
		}
		b.WriteString("\n")
		b.WriteString("       " + styleDim.Render(integ.ShortDescription))
		b.WriteString("\n")

		if i < len(m.integ.integrations)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	legend := fmt.Sprintf("  %s active  %s inactive",
		styleCheckboxOn.Render("[x]"),
		styleCheckboxOff.Render("[ ]"),
	)
	b.WriteString(legend)
	b.WriteString("\n\n")

	b.WriteString(styleDim.Render("  j/k navigate \u00b7 space toggle \u00b7 enter continue \u00b7 ? info \u00b7 esc skip"))
	b.WriteString("\n")

	return b.String()
}

func (m Model) migrateWorktreePath(result repo.MigrateResult) string {
	if result.WorktreePath != "" {
		return result.WorktreePath
	}
	return git.WorktreePath(result.BareRoot, result.Branch)
}
