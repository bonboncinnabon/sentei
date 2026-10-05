package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// Screen 1: Backup cleanup decision
func (m Model) updateMigrateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	result, ok := m.repo.result.(repo.MigrateResult)
	if !ok {
		return m, tea.Quit
	}

	// Any failed phase is critical: Migrate() early-returns on the first failure,
	// so a Validate or Backup failure also leaves the repo unmigrated.
	if result.HasFailures() {
		// Critical failure — only q to quit
		if msg, ok := msg.(tea.KeyPressMsg); ok && key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Yes):
			// Delete backup, then show integration selection screen
			if result.BackupPath != "" {
				_ = repo.DeleteBackup(result.BackupPath)
			}
			m.view = migrateIntegrationsView
			return m, loadMigrateIntegrations(m.migrateWorktreePath(result))

		case key.Matches(msg, keys.No):
			// Keep backup, show integration selection screen
			m.view = migrateIntegrationsView
			return m, loadMigrateIntegrations(m.migrateWorktreePath(result))

		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewMigrateSummary() string {
	var b strings.Builder

	result, ok := m.repo.result.(repo.MigrateResult)
	if !ok {
		return "  Migration result unavailable\n"
	}

	// Any failed phase is critical: Migrate() early-returns on the first failure,
	// so a Validate or Backup failure also leaves the repo unmigrated.
	hasCriticalFailure := result.HasFailures()
	var failErr error
	if hasCriticalFailure {
		if result.Err != nil {
			failErr = result.Err
		} else if _, step, ok := progress.FirstFailure(result.Phases); ok {
			failErr = step.Error
		}
	}

	b.WriteString(viewTitle(titleMigrationComplete))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	if hasCriticalFailure {
		errMsg := "unknown error"
		if failErr != nil {
			errMsg = failErr.Error()
		}
		errWidth := max(m.width-8, 30)
		fmt.Fprintf(&b, "  %s Migration failed\n\n",
			styleIndicatorFailed.Render(indicatorFailed))
		b.WriteString("    " + styleError.Width(errWidth).Render(errMsg))
		b.WriteString("\n\n")
		if result.BackupPath != "" {
			b.WriteString("  Your original repo is backed up at:\n")
			fmt.Fprintf(&b, "    %s\n\n", styleDim.Render(result.BackupPath))
			b.WriteString("  To restore:\n")
			fmt.Fprintf(&b, "    %s\n", result.RestoreCommand())
		}
		b.WriteString("\n")
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		b.WriteString(viewFooter(m.width, quitOnlyFooter))
		b.WriteString("\n")
		return b.String()
	}

	repoName := filepath.Base(result.BareRoot)
	fmt.Fprintf(&b, "  %s %s migrated\n\n",
		styleIndicatorDone.Render(indicatorDone), repoName)

	fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Path"), result.BareRoot)
	fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Branch"), result.Branch)
	if result.BackupPath != "" {
		backupName := filepath.Base(result.BackupPath)
		sizeHint := ""
		if result.BackupSize != "" {
			sizeHint = "  " + styleDim.Render(result.BackupSize)
		}
		fmt.Fprintf(&b, "    %-10s %s%s\n", styleDim.Render("Backup"), backupName, sizeHint)
	}

	b.WriteString("\n")
	b.WriteString("  Delete backup?\n")
	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, migrateConfirmFooter))
	b.WriteString("\n")

	return b.String()
}

// Screen 2: What next — continue in sentei or exit
func (m Model) updateMigrateNext(msg tea.Msg) (tea.Model, tea.Cmd) {
	result, ok := m.repo.result.(repo.MigrateResult)
	if !ok {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Confirm):
			// Re-launch sentei at the migrated repo
			return m, relaunchSentei(result.BareRoot)

		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewMigrateNext() string {
	var b strings.Builder

	result, ok := m.repo.result.(repo.MigrateResult)
	if !ok {
		return "  Migration result unavailable\n"
	}

	repoName := filepath.Base(result.BareRoot)
	worktreePath := result.WorktreePath
	if worktreePath == "" {
		worktreePath = git.WorktreePath(result.BareRoot, result.Branch)
	}

	b.WriteString(viewTitle(titleMigrationComplete))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "  %s %s ready\n\n",
		styleIndicatorDone.Render(indicatorDone), repoName)

	fmt.Fprintf(&b, "    cd %s\n", styleDim.Render(worktreePath))
	b.WriteString("\n")

	b.WriteString(styleDim.Render("  Your repo is ready for worktrees."))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Continue in sentei to create worktrees"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  and set up your workspace, or exit"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  to your shell."))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, migrateOpenFooter))
	b.WriteString("\n")

	return b.String()
}
