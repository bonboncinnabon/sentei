package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

func (m Model) updateCreateProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		return m, nil

	case createEventMsg:
		m.create.events = append(m.create.events, msg.Event)
		return m, tea.Batch(m.syncProgressBar(), m.waitForCreateEvent())

	case createCompleteMsg:
		m.create.result = &msg.Result
		m.worktreeGeneration++
		syncCmd := m.syncProgressBar()
		updated, holdCmd := m.holdOrAdvance(createSummaryView)
		return updated, tea.Batch(syncCmd, holdCmd, loadWorktreeContext(m.runner, m.repoPath, m.worktreeGeneration))
	}
	return m, nil
}

func (m Model) createLayout() ProgressLayout {
	return m.withProgressDetails(ProgressLayout{
		Title:     titleCreatingWorktree,
		Completed: m.create.result != nil,
		Subtitle:  fmt.Sprintf("%s \u2192 from %s", m.create.branchInput.Value(), m.create.baseInput.Value()),
		Phases:    progress.Snapshot(m.create.events),
		Width:     m.width,
		Height:    m.progressHeight(),
		Hints:     progressFooter,
	})
}

func (m Model) viewCreateProgress() string {
	return m.renderProgressLayout(m.createLayout())
}
