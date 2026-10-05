package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

type repoEventMsg progress.Event

type repoDoneMsg struct {
	result interface{} // CreateResult, CloneResult, or MigrateResult
}

// startRepoPipeline launches the appropriate pipeline based on opts type.
func (m *Model) startRepoPipeline(opts interface{}) tea.Cmd {
	ch := make(chan progress.Event, 50)
	resultCh := make(chan interface{}, 1)
	m.repo.eventCh = ch
	m.repo.resultCh = resultCh

	runner := m.runner
	shell := m.shell

	switch o := opts.(type) {
	case repo.CreateOptions:
		go func() {
			result := repo.Create(runner, shell, o, func(e progress.Event) { ch <- e })
			close(ch)
			resultCh <- result
		}()
	case repo.CloneOptions:
		go func() {
			result := repo.Clone(runner, o, func(e progress.Event) { ch <- e })
			close(ch)
			resultCh <- result
		}()
	case repo.MigrateOptions:
		go func() {
			result := repo.Migrate(runner, shell, o, func(e progress.Event) { ch <- e })
			close(ch)
			resultCh <- result
		}()
	}

	return m.waitForRepoEvent()
}

func (m Model) waitForRepoEvent() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.repo.eventCh
		if !ok {
			result := <-m.repo.resultCh
			return repoDoneMsg{result: result}
		}
		return repoEventMsg(ev)
	}
}

func (m Model) updateRepoProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		return m, nil

	case repoEventMsg:
		m.repo.events = append(m.repo.events, progress.Event(msg))
		return m, tea.Batch(m.syncProgressBar(), m.waitForRepoEvent())

	case repoDoneMsg:
		m.repo.result = msg.result
		targetView := repoSummaryView
		if m.repo.opType == "migrate" {
			targetView = migrateSummaryView
		}
		syncCmd := m.syncProgressBar()
		updated, holdCmd := m.holdOrAdvance(targetView)
		return updated, tea.Batch(syncCmd, holdCmd)
	}
	return m, nil
}

func (m Model) repoLayout() ProgressLayout {
	var title, subject string
	switch m.repo.opType {
	case "create":
		title = titleCreatingRepo
		subject = m.repo.nameInput.Value()
	case "clone":
		title = titleCloningRepo
		subject = m.repo.urlInput.Value()
	case "migrate":
		title = titleMigratingRepo
		subject = m.repoPath
	}

	return m.withProgressDetails(ProgressLayout{
		Title:     title,
		Subtitle:  subject,
		Completed: m.repo.result != nil,
		Phases:    progress.Snapshot(m.repo.events),
		Width:     m.width,
		Height:    m.progressHeight(),
		Hints:     progressFooter,
	})
}

func (m Model) viewRepoProgress() string {
	return m.renderProgressLayout(m.repoLayout())
}
