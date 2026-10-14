package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// MigrateInfo is the pre-flight answer shown on the migrate confirm screen.
type MigrateInfo struct {
	Plan   repo.MigratePlan
	Err    error // why the repository cannot be migrated
	Loaded bool  // false until the pre-flight has answered
}

type migrateInfoMsg struct {
	plan repo.MigratePlan
	err  error
}

func loadMigrateInfo(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		plan, err := repo.PlanMigrate(runner, repoPath)
		return migrateInfoMsg{plan: plan, err: err}
	}
}

// migrateBlocked reports whether the confirm screen must not start the
// migration yet: it waits for the pre-flight and stops on a refusal, however
// the screen was reached.
func (m Model) migrateBlocked() bool {
	info := m.repo.migrateInfo
	return !info.Loaded || info.Err != nil
}

func (m Model) updateMigrateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case migrateInfoMsg:
		m.repo.migrateInfo = MigrateInfo{Plan: msg.plan, Err: msg.err, Loaded: true}
		return m, nil

	case ConfirmProceedMsg:
		if m.migrateBlocked() {
			return m, nil
		}
		opts := repo.MigrateOptions{
			RepoPath: m.repoPath,
		}
		m.repo.events = nil
		m.repo.result = nil
		m.repo.opType = "migrate"
		m.progressStartedAt = time.Now()
		m.progressToken++
		m.view = migrateProgressView
		return m, m.startRepoPipeline(opts)

	case ConfirmBackMsg:
		if m.migrateOpts != nil {
			return m, tea.Quit
		}
		m.repo.migrateInfo = MigrateInfo{}
		m.view = menuView
		return m, nil
	}

	if cmd := UpdateConfirmation(msg); cmd != nil {
		return m, cmd
	}

	return m, nil
}

func (m Model) viewMigrateConfirm() string {
	var b strings.Builder

	b.WriteString(viewTitle(titleMigrate))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  " + fitPathTail(m.repoPath, max(m.width, 20)-2)))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	info := m.repo.migrateInfo
	footer := confirmationFooter
	switch {
	case !info.Loaded:
		b.WriteString(styleDim.Render("  Checking repository…"))
		b.WriteString("\n")
		footer = migrateBlockedFooter

	case info.Err != nil:
		fmt.Fprintf(&b, "  %s This repository cannot be migrated yet\n\n",
			styleIndicatorFailed.Render(indicatorFailed))
		b.WriteString(m.errorBlock(info.Err))
		footer = migrateBlockedFooter

	default:
		worktree := filepath.Join(filepath.Base(info.Plan.RepoPath), filepath.Base(info.Plan.WorktreePath))
		writeConfirmationItems(&b, []ConfirmationItem{
			{Label: "Current branch:", Value: info.Plan.Branch},
			{Label: "Default branch:", Value: info.Plan.DefaultBranch},
		}, m.width)
		b.WriteString("\n")
		b.WriteString("  This will:\n")
		changes := []string{"Move .git to .bare", "Move the working tree into " + worktree}
		if count := info.Plan.LinkedWorktrees; count > 0 {
			changes = append(changes, fmt.Sprintf("Relink %d linked %s, left where %s",
				count, pluralize(count, "worktree", "worktrees"), pluralize(count, "it is", "they are")))
		}
		if count := info.Plan.Submodules; count > 0 {
			changes = append(changes, fmt.Sprintf("Reattach %d %s to the new worktree", count, pluralize(count, "submodule", "submodules")))
		}
		if remotes := info.Plan.RelativeRemotes; len(remotes) > 0 {
			changes = append(changes, "Make the relative path of "+strings.Join(remotes, ", ")+" absolute")
		}
		for _, change := range changes {
			writeWrapped(&b, "    "+styleAccent.Render(indicatorWouldAct)+" ", change, m.width)
		}
		b.WriteString("\n")
		b.WriteString(m.dimParagraph("Nothing is copied or deleted. Uncommitted, untracked and ignored files move as they are; a failed step is undone."))
	}

	if m.migrateOpts != nil {
		// Reached by a command: it is echoed, as on every such confirmation.
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  sentei migrate"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, footer))
	b.WriteString("\n")

	return b.String()
}
