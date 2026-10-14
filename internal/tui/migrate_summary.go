package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/repo"
)

func (m Model) updateMigrateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	result, ok := m.repo.result.(repo.MigrateResult)
	if !ok {
		return m, tea.Quit
	}

	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Confirm) && !result.HasFailures():
			// Re-launch sentei at the migrated repo.
			return m, relaunchSentei(result.BareRoot)
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

	b.WriteString(viewTitle(titleMigrationComplete))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	if failure := result.Failure(); failure != nil {
		fmt.Fprintf(&b, "  %s Migration failed\n\n",
			styleIndicatorFailed.Render(indicatorFailed))
		b.WriteString(m.errorBlock(failure))
		b.WriteString("\n")
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		b.WriteString(viewFooter(m.width, quitOnlyFooter))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(m.headline(styleIndicatorDone.Render(indicatorDone), filepath.Base(result.BareRoot), "migrated"))

	b.WriteString(summaryPathRow("Path", result.BareRoot, m.width))
	b.WriteString(summaryRow("Branch", result.Branch, m.width))
	b.WriteString("\n")
	b.WriteString(m.dimParagraph("Your files, stashes, remotes, config and hooks are where they were, one directory down. Continue in sentei to add worktrees, or exit to your shell."))
	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(m.cdLine(result.WorktreePath))
	b.WriteString("\n")
	b.WriteString(viewFooter(m.width, migrateOpenFooter))
	b.WriteString("\n")

	return b.String()
}
