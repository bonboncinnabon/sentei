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

type migrateInfoMsg struct {
	branch  string
	isDirty bool
	err     error
}

func loadMigrateInfo(runner git.CommandRunner, repoPath string) tea.Cmd {
	return func() tea.Msg {
		branch, err := runner.Run(repoPath, "branch", "--show-current")
		if err != nil {
			return migrateInfoMsg{err: err}
		}
		status, err := runner.Run(repoPath, "status", "--porcelain")
		if err != nil {
			return migrateInfoMsg{err: err}
		}
		isDirty := strings.TrimSpace(status) != ""
		return migrateInfoMsg{branch: branch, isDirty: isDirty}
	}
}

func (m Model) updateMigrateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case migrateInfoMsg:
		if msg.err != nil {
			m.repo.validationErr = fmt.Sprintf("failed to load repo info: %v", msg.err)
		} else {
			m.repo.migrateInfo = MigrateInfo{
				Branch:  msg.branch,
				IsDirty: msg.isDirty,
			}
		}
		return m, nil

	case ConfirmProceedMsg:
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
	deleteBackup := "no"
	if m.migrateOpts != nil && m.migrateOpts.DeleteBackup {
		deleteBackup = "yes"
	}

	flags := make(map[string]string)
	if m.migrateOpts != nil && m.migrateOpts.DeleteBackup {
		flags["delete-backup"] = "true"
	}

	return ConfirmationViewModel{
		Width: m.width,
		Title: titleConfirmMigration,
		Items: []ConfirmationItem{
			{Label: "Repo path:", Value: m.repoPath},
			{Label: "Delete backup:", Value: deleteBackup},
		},
		CLICommand: BuildCLICommand("migrate", flags),
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

	if m.repo.validationErr != "" {
		b.WriteString(styleError.Render("  " + m.repo.validationErr))
		b.WriteString("\n\n")
	}

	branch := m.repo.migrateInfo.Branch
	if branch == "" {
		branch = styleDim.Render("detecting\u2026")
	}

	fmt.Fprintf(&b, "  %-18s %s\n", styleDim.Render("Current branch"), branch)

	if m.repo.migrateInfo.IsDirty {
		fmt.Fprintf(&b, "  %-18s %s\n", styleDim.Render("Status"),
			styleIndicatorWarning.Render(indicatorWarning+" uncommitted changes"))
	} else {
		fmt.Fprintf(&b, "  %-18s %s\n", styleDim.Render("Status"), styleSuccess.Render("clean"))
	}

	b.WriteString("\n")
	b.WriteString("  This will:\n")
	fmt.Fprintf(&b, "    %s Back up current repo\n", styleDim.Render("\u25cf"))
	fmt.Fprintf(&b, "    %s Convert to bare repository structure\n", styleDim.Render("\u25cf"))
	fmt.Fprintf(&b, "    %s Create worktree for %s\n", styleDim.Render("\u25cf"),
		filepath.Base(m.repoPath)+"/"+branch)

	if m.repo.migrateInfo.IsDirty {
		b.WriteString("\n")
		b.WriteString(styleIndicatorWarning.Render(fmt.Sprintf("  %s Uncommitted changes will be preserved in the backup", indicatorWarning)))
		b.WriteString("\n")
		b.WriteString(styleDim.Render("    but not in the new worktree"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, confirmationFooter))
	b.WriteString("\n")

	return b.String()
}
