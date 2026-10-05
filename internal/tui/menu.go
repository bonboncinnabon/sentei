package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

type worktreeContextMsg struct {
	worktrees     []git.Worktree
	defaultBranch string
	err           error
	generation    uint64
}

func loadWorktreeContext(runner git.CommandRunner, repoPath string, generation uint64) tea.Cmd {
	return func() tea.Msg {
		wts, err := git.ListWorktrees(runner, repoPath)
		if err != nil {
			return worktreeContextMsg{err: err, generation: generation}
		}
		wts = worktree.EnrichWorktrees(runner, wts, worktree.DefaultEnrichConcurrency)
		var filtered []git.Worktree
		for _, wt := range wts {
			if !wt.IsBare && !wt.IsPrunable {
				filtered = append(filtered, wt)
			}
		}
		return worktreeContextMsg{
			worktrees:     filtered,
			defaultBranch: git.DetectDefaultBranch(runner, repoPath),
			generation:    generation,
		}
	}
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit), key.Matches(msg, keys.Back):
			return m, tea.Quit

		case key.Matches(msg, keys.Down):
			for {
				m.menuCursor++
				if m.menuCursor >= len(m.menuItems) {
					m.menuCursor = len(m.menuItems) - 1
					break
				}
				if m.menuItems[m.menuCursor].enabled {
					break
				}
			}

		case key.Matches(msg, keys.Up):
			for {
				m.menuCursor--
				if m.menuCursor < 0 {
					m.menuCursor = 0
					break
				}
				if m.menuItems[m.menuCursor].enabled {
					break
				}
			}

		case key.Matches(msg, keys.Confirm):
			if m.menuCursor >= 0 && m.menuCursor < len(m.menuItems) && m.menuItems[m.menuCursor].enabled {
				label := m.menuItems[m.menuCursor].label
				switch label {
				case "Create new worktree":
					m = m.withCreateFlowReset()
					m.view = createBranchView
					return m, m.create.branchInput.Focus()
				case "Manage integrations":
					m.view = integrationListView
					return m, m.loadIntegrationState()
				case "Remove worktrees":
					m.view = listView
					m.remove.selected = make(map[string]bool)
					if len(m.remove.worktrees) == 0 {
						m.worktreeGeneration++
						return m, loadWorktreeContext(m.runner, m.repoPath, m.worktreeGeneration)
					}
				case "Cleanup & exit":
					return m.startCleanupScan()
				case "Create new repository":
					m.repo.nameInput.SetValue("")
					m.repo.locationInput.SetValue(m.repoPath)
					m.repo.focusedField = 0
					m.repo.validationErr = ""
					m.view = repoNameView
					return m, m.repo.nameInput.Focus()
				case "Clone repository as bare":
					m.repo.urlInput.SetValue("")
					m.repo.cloneNameInput.SetValue("")
					m.repo.cloneFocusedField = 0
					m.repo.nameManuallyEdited = false
					m.view = cloneInputView
					return m, m.repo.urlInput.Focus()
				case "Migrate to bare repository":
					m.view = migrateConfirmView
					return m, loadMigrateInfo(m.runner, m.repoPath)
				}
			}
		}
	}
	return m, nil
}

func (m *Model) updateMenuHints() {
	if m.context != repo.ContextBareRepo {
		return
	}
	if len(m.menuItems) < 3 {
		return
	}
	count := len(m.remove.worktrees)
	if count > 0 {
		m.menuItems[2].hint = fmt.Sprintf("%d available", count)
		m.menuItems[2].enabled = true
	} else {
		m.menuItems[2].hint = "none"
		m.menuItems[2].enabled = false
	}
	m.menuItems[2].loading = false
}

func (m Model) viewMenu() string {
	var b strings.Builder

	repoName := filepath.Base(m.repoPath)
	b.WriteString(viewTitle(titleMenu))
	b.WriteString("\n\n")

	switch m.context {
	case repo.ContextBareRepo:
		b.WriteString(styleDim.Render(truncateWithEllipsis(fmt.Sprintf("  %s (bare) · %s", repoName, m.repoPath), max(m.width, 40))))
		b.WriteString("\n")
		if len(m.remove.worktrees) > 0 {
			clean, dirty, locked := 0, 0, 0
			for _, wt := range m.remove.worktrees {
				switch {
				case wt.IsLocked:
					locked++
				case wt.HasUncommittedChanges || wt.HasUntrackedFiles:
					dirty++
				default:
					clean++
				}
			}
			b.WriteString(styleDim.Render(fmt.Sprintf("  %d worktrees %s %d clean, %d dirty, %d locked",
				len(m.remove.worktrees), "\u00b7", clean, dirty, locked)))
			b.WriteString("\n")
		}
	case repo.ContextNonBareRepo:
		b.WriteString(styleDim.Render(fmt.Sprintf("  %s %s %s", repoName, "\u00b7", m.repoPath)))
		b.WriteString("\n")
	case repo.ContextNoRepo:
		b.WriteString(styleDim.Render(fmt.Sprintf("  %s", m.repoPath)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	for i, item := range m.menuItems {
		cursor := "  "
		if i == m.menuCursor {
			cursor = "▸ "
		}

		label := item.label
		if !item.enabled {
			label = styleDim.Render(label)
		}

		hint := ""
		if item.hint != "" {
			hint = "  " + styleDim.Render(item.hint)
		}
		if item.loading {
			hint = "  " + shimmerLine(starFrame(m.motionTick)+" "+item.hint, rampDim, m.motionTick)
		}

		if i == m.menuCursor {
			// The selected row carries the accent itself, not just the marker.
			if item.enabled {
				label = styleAccent.Render(item.label)
			}
			b.WriteString(styleAccent.Render(cursor) + label + hint)
		} else {
			b.WriteString("  " + label + hint)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, menuFooter))
	b.WriteString("\n")

	return b.String()
}
