package tui

import (
	"errors"
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

type removalEventMsg struct{ event progress.Event }
type removalEventsCompleteMsg struct{}
type deletionsCompleteMsg struct{ Result worktree.DeletionResult }
type pruneCompleteMsg struct{ Err error }

func waitForRemovalEvent(ch <-chan progress.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return removalEventsCompleteMsg{}
		}
		return removalEventMsg{event: ev}
	}
}

// runPrune is everything a removal does after the worktrees are gone: one
// local `git worktree prune`. It never touches the network, branches or
// config; those belong to `sentei cleanup`, which the user runs on purpose.
func runPrune(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		err := worktree.PruneWorktrees(runner, repoPath)
		return pruneCompleteMsg{Err: err}
	}
}

func (m Model) updateProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		return m, nil

	case removalEventMsg:
		m.remove.run.events = append(m.remove.run.events, msg.event)
		return m, tea.Batch(m.syncProgressBar(), waitForRemovalEvent(m.remove.run.progressCh))

	case deletionsCompleteMsg:
		msg.Result.Err = errors.Join(m.remove.run.result.Err, msg.Result.Err)
		m.remove.run.result = msg.Result
		return m, tea.Batch(m.syncProgressBar(), runPrune(m.runner, m.repoPath))

	case pruneCompleteMsg:
		// The prune ends the run's work. Its error is kept apart from the
		// result: metadata that could not be pruned is a warning, and the
		// worktrees that were removed stay removed.
		m.remove.run.pruned = true
		m.remove.run.pruneErr = msg.Err
		if execution := m.remove.run.execution; execution != nil {
			m.remove.run.result.Err = errors.Join(m.remove.run.result.Err, execution.Finish("removal run complete"))
			m.remove.run.result.Phases = execution.Phases()
			close(m.remove.run.progressCh)
		}
		return m, m.syncProgressBar()

	case removalEventsCompleteMsg:
		m.remove.selected = make(map[string]bool)
		m.dropRemovedWorktrees()
		m.worktreeGeneration++
		// Final spring target before the hold: all phases are complete, so
		// the bar settles at full while the completion frame holds.
		syncCmd := m.syncProgressBar()
		updated, holdCmd := m.holdOrAdvance(summaryView)
		return updated, tea.Batch(syncCmd, holdCmd,
			loadWorktreeContext(m.runner, m.repoPath, m.worktreeGeneration))

	}
	return m, nil
}

// dropRemovedWorktrees takes the worktrees this run removed off the list at
// once. The reload that follows can take seconds, and until it lands they
// would still be listed and selectable.
func (m *Model) dropRemovedWorktrees() {
	removed := make(map[string]bool)
	for _, outcome := range m.remove.run.result.Outcomes {
		if outcome.Success {
			removed[outcome.Path] = true
		}
	}
	m.remove.worktrees = slices.DeleteFunc(m.remove.worktrees, func(wt git.Worktree) bool { return removed[wt.Path] })
	m.reindex()
	m.updateMenuHints()
}

func (m Model) removalLayout() ProgressLayout {
	return m.withProgressDetails(ProgressLayout{
		Title:     titleRemoving,
		Completed: m.remove.run.pruned,
		Phases:    progress.Snapshot(m.remove.run.events),
		Width:     m.width,
		Height:    m.progressHeight(),
		Hints:     progressFooter,
	})
}

func (m Model) viewProgress() string {
	return m.renderProgressLayout(m.removalLayout())
}
