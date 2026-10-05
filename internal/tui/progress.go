package tui

import (
	"errors"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

const (
	statusPending  = "pending"
	statusRemoving = "removing"
	statusRemoved  = "removed"
	statusFailed   = "failed"
)

type cleanupCompleteMsg struct {
	Result cleanup.Result
}

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

func runPrune(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		err := worktree.PruneWorktrees(runner, repoPath)
		return pruneCompleteMsg{Err: err}
	}
}

func runCleanup(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		result := cleanup.Run(runner, repoPath, cleanup.Options{Mode: cleanup.ModeSafe}, func(cleanup.Event) {})
		return cleanupCompleteMsg{Result: result}
	}
}

func (m Model) updateProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		return m, nil

	case teardownCompleteMsg:
		// Yes switches to progressView before teardown runs, so completion
		// lands here, not in updateConfirm.
		m.remove.run.teardownRunning = false
		m.remove.run.teardownResults = msg.results
		m.remove.run.result.Err = errors.Join(m.remove.run.result.Err, msg.err)
		return m.startDeletions()

	case removalEventMsg:
		m.remove.run.events = append(m.remove.run.events, msg.event)
		if msg.event.Phase != worktree.RemovalPhaseID {
			return m, tea.Batch(m.syncProgressBar(), waitForRemovalEvent(m.remove.run.progressCh))
		}
		path := msg.event.Step
		for _, target := range m.remove.run.targets {
			if target.StepID == msg.event.Step {
				path = target.Worktree.Path
				break
			}
		}
		switch msg.event.Status {
		case progress.StepRunning:
			m.remove.run.statuses[path] = statusRemoving
		case progress.StepDone:
			m.remove.run.statuses[path] = statusRemoved
		case progress.StepFailed:
			m.remove.run.statuses[path] = statusFailed
		}
		return m, tea.Batch(m.syncProgressBar(), waitForRemovalEvent(m.remove.run.progressCh))

	case deletionsCompleteMsg:
		msg.Result.Err = errors.Join(m.remove.run.result.Err, msg.Result.Err)
		m.remove.run.result = msg.Result
		return m, tea.Batch(m.syncProgressBar(), runPrune(m.runner, m.repoPath))

	case pruneCompleteMsg:
		pruneErr := msg.Err
		m.remove.run.pruneErr = &pruneErr
		if m.remove.run.execution != nil {
			if msg.Err != nil {
				_, transitionErr := m.remove.run.execution.Fail(cleanupPhaseID, pruneStepID, msg.Err)
				m.remove.run.result.Err = errors.Join(m.remove.run.result.Err, transitionErr)
			} else {
				_, transitionErr := m.remove.run.execution.Done(cleanupPhaseID, pruneStepID, "Pruned")
				m.remove.run.result.Err = errors.Join(m.remove.run.result.Err, transitionErr)
			}
		}
		return m, tea.Batch(m.syncProgressBar(), runCleanup(m.runner, m.repoPath))

	case cleanupCompleteMsg:
		m.remove.run.cleanupResult = &msg.Result
		if m.remove.run.execution == nil {
			m.remove.selected = make(map[string]bool)
			m.worktreeGeneration++
			syncCmd := m.syncProgressBar()
			updated, holdCmd := m.holdOrAdvance(summaryView)
			return updated, tea.Batch(syncCmd, holdCmd,
				recordRemovals(m.repoPath, m.remove.run.result.SuccessCount),
				loadWorktreeContext(m.runner, m.repoPath, m.worktreeGeneration))
		}
		if len(msg.Result.Errors) > 0 {
			cleanupErrors := make([]error, len(msg.Result.Errors))
			for i, operationErr := range msg.Result.Errors {
				cleanupErrors[i] = operationErr.Err
			}
			cleanupErr := errors.Join(cleanupErrors...)
			m.remove.run.result.Err = errors.Join(m.remove.run.result.Err, cleanupErr)
			_, transitionErr := m.remove.run.execution.Fail(cleanupPhaseID, cleanupStepID, cleanupErr)
			m.remove.run.result.Err = errors.Join(m.remove.run.result.Err, transitionErr)
		} else {
			_, transitionErr := m.remove.run.execution.Done(cleanupPhaseID, cleanupStepID, "Cleaned")
			m.remove.run.result.Err = errors.Join(m.remove.run.result.Err, transitionErr)
		}
		m.remove.run.result.Err = errors.Join(m.remove.run.result.Err, m.remove.run.execution.Finish("removal run complete"))
		m.remove.run.result.Phases = m.remove.run.execution.Phases()
		close(m.remove.run.progressCh)
		return m, m.syncProgressBar()

	case removalEventsCompleteMsg:
		m.remove.selected = make(map[string]bool)
		m.worktreeGeneration++
		// Final spring target before the hold: all phases are complete, so
		// the bar settles at full while the completion frame holds.
		syncCmd := m.syncProgressBar()
		updated, holdCmd := m.holdOrAdvance(summaryView)
		return updated, tea.Batch(syncCmd, holdCmd,
			recordRemovals(m.repoPath, m.remove.run.result.SuccessCount),
			loadWorktreeContext(m.runner, m.repoPath, m.worktreeGeneration))

	}
	return m, nil
}

func (m Model) removalLayout() ProgressLayout {
	return m.withProgressDetails(ProgressLayout{
		Title:     titleRemoving,
		Completed: m.remove.run.cleanupResult != nil,
		Phases:    m.buildRemovalPhases(),
		Width:     m.width,
		Height:    m.progressHeight(),
		Hints:     progressFooter,
	})
}

func (m Model) viewProgress() string {
	return m.renderProgressLayout(m.removalLayout())
}

// buildRemovalPhases maps the current removal run onto the shared phase
// shape consumed by ProgressLayout.
func (m Model) buildRemovalPhases() []progress.PhaseState {
	return progress.Snapshot(m.remove.run.events)
}
