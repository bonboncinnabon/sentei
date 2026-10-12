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
// migration yet: the menu path waits for its pre-flight and stops on a
// refusal. A direct launch has no pre-flight on screen; the migration runs
// the same checks as its first step.
func (m Model) migrateBlocked() bool {
	info := m.repo.migrateInfo
	return m.migrateOpts == nil && (!info.Loaded || info.Err != nil)
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

// migrateConfirmationVM builds a ConfirmationViewModel for the migrate flow.
func (m Model) migrateConfirmationVM() ConfirmationViewModel {
	return ConfirmationViewModel{
		Width: m.width,
		Title: titleConfirmMigration,
		Items: []ConfirmationItem{
			{Label: "Repo path:", Value: m.repoPath},
		},
		CLICommand: BuildCLICommand("migrate", nil),
	}
}

func (m Model) viewMigrateConfirm() string {
	if m.migrateOpts != nil {
		return m.migrateConfirmationVM().View()
	}

	var b strings.Builder

	b.WriteString(viewTitle(titleMigrate))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render(fmt.Sprintf("  %s", m.repoPath)))
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
		b.WriteString("    " + styleError.Width(max(m.width-8, 30)).Render(info.Err.Error()))
		b.WriteString("\n")
		footer = migrateBlockedFooter

	default:
		worktree := filepath.Join(filepath.Base(info.Plan.RepoPath), filepath.Base(info.Plan.WorktreePath))
		fmt.Fprintf(&b, "  %-18s %s\n", styleDim.Render("Current branch"), info.Plan.Branch)
		fmt.Fprintf(&b, "  %-18s %s\n", styleDim.Render("Default branch"), info.Plan.DefaultBranch)
		b.WriteString("\n")
		b.WriteString("  This will:\n")
		fmt.Fprintf(&b, "    %s Move .git to .bare\n", styleDim.Render("●"))
		fmt.Fprintf(&b, "    %s Move the working tree into %s\n", styleDim.Render("●"), worktree)
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  Nothing is copied or deleted. Uncommitted, untracked and"))
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  ignored files move as they are; a failed step is undone."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, footer))
	b.WriteString("\n")

	return b.String()
}
