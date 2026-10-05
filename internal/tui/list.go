package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/bonboncinnabon/sentei/internal/git"
)

const (
	colCursor   = 0
	colCheckbox = 1
	colStatus   = 2
	colBranch   = 3
	colAge      = 4
)

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func stripBranchPrefix(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

func statusIndicator(wt git.Worktree) string {
	switch {
	case wt.IsLocked:
		return styleStatusLocked.Render("[L]")
	case wt.HasUncommittedChanges:
		return styleStatusDirty.Render("[~]")
	case wt.HasUntrackedFiles:
		return styleStatusUntracked.Render("[!]")
	default:
		return styleStatusClean.Render("[ok]")
	}
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		if m.remove.filterActive {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseWheelDown:
			m = m.listCursorDown()
		case tea.MouseWheelUp:
			m = m.listCursorUp()
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.remove.filterActive {
			return m.updateFilterInput(msg)
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Back):
			if m.remove.filterText != "" {
				m.remove.filterText = ""
				m.reindex()
				return m, nil
			}
			if m.menuItems != nil {
				m.view = menuView
				return m, nil
			}
			return m, tea.Quit

		case key.Matches(msg, keys.Filter):
			m.remove.filterActive = true
			m.remove.filterInput.SetValue(m.remove.filterText)
			return m, m.remove.filterInput.Focus()

		case key.Matches(msg, keys.Sort):
			m.remove.sortField = (m.remove.sortField + 1) % 2
			m.remove.cursor = 0
			m.remove.offset = 0
			m.reindex()

		case key.Matches(msg, keys.ReverseSort):
			m.remove.sortAscending = !m.remove.sortAscending
			m.remove.cursor = 0
			m.remove.offset = 0
			m.reindex()

		case key.Matches(msg, keys.Down):
			m = m.listCursorDown()

		case key.Matches(msg, keys.Up):
			m = m.listCursorUp()

		case key.Matches(msg, keys.PageDown):
			m.remove.cursor += m.height
			if m.remove.cursor >= len(m.remove.visibleIndices) {
				m.remove.cursor = len(m.remove.visibleIndices) - 1
			}
			if m.remove.cursor < 0 {
				m.remove.cursor = 0
			}
			if m.remove.cursor >= m.remove.offset+m.height {
				m.remove.offset = m.remove.cursor - m.height + 1
			}

		case key.Matches(msg, keys.PageUp):
			m.remove.cursor -= m.height
			if m.remove.cursor < 0 {
				m.remove.cursor = 0
			}
			if m.remove.cursor < m.remove.offset {
				m.remove.offset = m.remove.cursor
			}

		case key.Matches(msg, keys.Toggle):
			if len(m.remove.visibleIndices) > 0 {
				wt := m.remove.worktrees[m.remove.visibleIndices[m.remove.cursor]]
				if git.IsProtectedBranchWith(wt.Branch, m.remove.defaultBranch) {
					break
				}
				if m.remove.selected[wt.Path] {
					delete(m.remove.selected, wt.Path)
				} else {
					m.remove.selected[wt.Path] = true
				}
			}

		case key.Matches(msg, keys.All):
			allSelected := true
			for _, idx := range m.remove.visibleIndices {
				wt := m.remove.worktrees[idx]
				if git.IsProtectedBranchWith(wt.Branch, m.remove.defaultBranch) {
					continue
				}
				if !m.remove.selected[wt.Path] {
					allSelected = false
					break
				}
			}
			if allSelected {
				for _, idx := range m.remove.visibleIndices {
					wt := m.remove.worktrees[idx]
					if git.IsProtectedBranchWith(wt.Branch, m.remove.defaultBranch) {
						continue
					}
					delete(m.remove.selected, wt.Path)
				}
			} else {
				for _, idx := range m.remove.visibleIndices {
					wt := m.remove.worktrees[idx]
					if git.IsProtectedBranchWith(wt.Branch, m.remove.defaultBranch) {
						continue
					}
					m.remove.selected[wt.Path] = true
				}
			}

		case key.Matches(msg, keys.Confirm):
			if len(m.remove.selected) == 0 {
				break
			}
			// Safety gate: only at-risk selections need a confirmation stop;
			// clean-and-pushed worktrees delete without friction.
			for _, wt := range m.selectedWorktrees() {
				if worktreeAtRisk(wt) {
					m.view = confirmView
					return m, nil
				}
			}
			return m.beginRemoval()
		}
	}
	if m.remove.filterActive {
		return m.updateFilterInput(msg)
	}
	return m, nil
}

// listCursorDown moves the cursor one row down, scrolling the window when it
// passes the bottom edge. Shared by the j/down keys and the mouse wheel.
func (m Model) listCursorDown() Model {
	if m.remove.cursor < len(m.remove.visibleIndices)-1 {
		m.remove.cursor++
		if m.remove.cursor >= m.remove.offset+m.height {
			m.remove.offset = m.remove.cursor - m.height + 1
		}
	}
	return m
}

// listCursorUp moves the cursor one row up, scrolling the window when it
// passes the top edge. Shared by the k/up keys and the mouse wheel.
func (m Model) listCursorUp() Model {
	if m.remove.cursor > 0 {
		m.remove.cursor--
		if m.remove.cursor < m.remove.offset {
			m.remove.offset = m.remove.cursor
		}
	}
	return m
}

func (m Model) updateFilterInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, keys.Back):
			m.remove.filterActive = false
			m.remove.filterText = ""
			m.remove.filterInput.SetValue("")
			m.remove.filterInput.Blur()
			m.reindex()
			return m, nil

		case key.Matches(msg, keys.Confirm):
			m.remove.filterActive = false
			m.remove.filterText = m.remove.filterInput.Value()
			m.remove.filterInput.Blur()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.remove.filterInput, cmd = m.remove.filterInput.Update(msg)
	m.remove.filterText = m.remove.filterInput.Value()
	m.reindex()
	return m, cmd
}

func (m Model) viewList() string {
	var b strings.Builder

	b.WriteString(viewTitle(titleList))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render(fmt.Sprintf("  %s (bare)", filepath.Base(m.repoPath))))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	if len(m.remove.worktrees) == 0 {
		b.WriteString(styleDim.Render("  No worktrees found."))
		b.WriteString("\n")
		return b.String()
	}

	if len(m.remove.visibleIndices) == 0 {
		b.WriteString(styleDim.Render("  No matches."))
		b.WriteString("\n\n")
		b.WriteString(m.viewStatusOrFilter())
		b.WriteString("\n")
		b.WriteString(m.viewBottomLine())
		return b.String()
	}

	end := min(m.remove.offset+m.height, len(m.remove.visibleIndices))

	arrow := " ▲"
	if !m.remove.sortAscending {
		arrow = " ▼"
	}
	// The Age column displays elapsed time, which runs opposite to the
	// commit-date sort underneath: date-ascending means the largest ages
	// first. The arrow describes what the eye sees, so Age flips it.
	if m.remove.sortField == SortByAge {
		if arrow == " ▲" {
			arrow = " ▼"
		} else {
			arrow = " ▲"
		}
	}
	hdrBranch := "Branch"
	hdrAge := "Age"
	hdrSubject := "Subject"
	switch m.remove.sortField {
	case SortByBranch:
		hdrBranch += arrow
	case SortByAge:
		hdrAge += arrow
	}

	// Column priority: structure survives narrow terminals, detail degrades.
	// Width 0 (untested sizing) counts as wide.
	showAge := m.width == 0 || m.width >= 56
	showSubject := m.width == 0 || m.width >= 72

	headers := []string{"", "", "", hdrBranch}
	if showAge {
		headers = append(headers, hdrAge)
	}
	if showSubject {
		headers = append(headers, hdrSubject)
	}

	t := table.New().
		BorderTop(false).BorderBottom(false).
		BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderHeader(false).BorderRow(false).
		Headers(headers...).
		Wrap(false)

	if m.width > 0 {
		t.Width(m.width)
	}

	fixedWidth := colWidthCursor + colWidthCheckbox + colWidthStatus
	if showAge {
		fixedWidth += colWidthAge
	}
	colPadding := 3
	remaining := max(m.width-fixedWidth-colPadding, 20)
	branchWidth := remaining
	subjectWidth := 0
	if showSubject {
		branchWidth = remaining / 2
		subjectWidth = remaining - branchWidth
	}

	for i := m.remove.offset; i < end; i++ {
		wt := m.remove.worktrees[m.remove.visibleIndices[i]]

		cursor := "  "
		if i == m.remove.cursor {
			cursor = "▸ "
		}

		var checkbox string
		if git.IsProtectedBranchWith(wt.Branch, m.remove.defaultBranch) {
			checkbox = styleStatusProtected.Render("[P]")
		} else if m.remove.selected[wt.Path] {
			checkbox = "[x]"
		} else {
			checkbox = "[ ]"
		}

		status := statusIndicator(wt)

		branch := worktreeLabel(wt)

		age := relativeTime(wt.LastCommitDate)
		subject := wt.LastCommitSubject
		if wt.EnrichmentError != "" {
			age = "error"
			subject = wt.EnrichmentError
		}

		// One-line rows are law: cells truncate with …, never wrap.
		branch = truncateWithEllipsis(branch, max(branchWidth-2, 4))
		row := []string{cursor, checkbox, status, branch}
		if showAge {
			row = append(row, age)
		}
		if showSubject {
			row = append(row, truncateWithEllipsis(subject, max(subjectWidth-2, 4)))
		}
		t.Row(row...)
	}

	sortedCol := -1
	switch m.remove.sortField {
	case SortByBranch:
		sortedCol = colBranch
	case SortByAge:
		if showAge {
			sortedCol = colAge
		}
	}

	// Column widths by dynamic position: fixed leading columns, then the
	// flexible Branch, then whichever detail columns the width allows.
	styleFor := func(base lipgloss.Style, col int) lipgloss.Style {
		switch {
		case col == colCursor:
			return base.Width(colWidthCursor)
		case col == colCheckbox:
			return base.Width(colWidthCheckbox)
		case col == colStatus:
			return base.Width(colWidthStatus)
		case col == colBranch:
			return base.Width(branchWidth).Padding(0, 1)
		case showAge && col == colAge:
			return base.Width(colWidthAge).Padding(0, 1)
		case showSubject:
			return base.Width(subjectWidth).Padding(0, 1)
		}
		return base
	}

	t.StyleFunc(func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			base := styleColumnHeader
			if col == sortedCol {
				base = styleColumnHeaderSorted
			}
			return styleFor(base, col)
		}

		idx := m.remove.offset + row

		var base lipgloss.Style
		switch {
		case idx == m.remove.cursor:
			base = styleCursorRow
		case m.remove.selected[m.remove.worktrees[m.remove.visibleIndices[idx]].Path]:
			base = styleSelectedRow
		default:
			base = styleNormalRow
		}

		return styleFor(base, col)
	})

	b.WriteString(t.Render())
	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n")

	b.WriteString(m.viewStatusOrFilter())
	b.WriteString("\n")
	b.WriteString(m.viewBottomLine())
	return b.String()
}

func (m Model) viewStatusOrFilter() string {
	if m.remove.filterActive {
		// The filter keeps the chrome margin like every other line.
		return "  " + m.remove.filterInput.View()
	}
	return m.viewStatusBar()
}

func (m Model) viewBottomLine() string {
	if m.remove.filterActive {
		return viewFooter(m.width, listFilterFooter)
	}
	return m.viewLegend()
}

func (m Model) viewStatusBar() string {
	count := len(m.remove.selected)

	var filterInfo string
	if m.remove.filterText != "" {
		filterInfo = fmt.Sprintf(" \u00b7 filter: %q (%d/%d)", m.remove.filterText, len(m.remove.visibleIndices), len(m.remove.worktrees))
	}
	if m.remove.filterLabel != "" {
		filterInfo += fmt.Sprintf(" \u00b7 pre-filter: %s", m.remove.filterLabel)
	}

	// A delete affordance with nothing selected is a dead-end keypress.
	hintSet := listFooter
	if count == 0 {
		hintSet = listFooterNoSelection
	}
	prefix := fmt.Sprintf("  %d selected%s \u00b7 ", count, filterInfo)
	hints := footerHints(m.width-lipgloss.Width(prefix), hintSet)
	return styleStatusBar.Render(prefix) + hints
}

func (m Model) viewLegend() string {
	full := styleDim.Render("  ") +
		styleStatusClean.Render("[ok]") + styleDim.Render(" clean  ") +
		styleStatusDirty.Render("[~]") + styleDim.Render(" dirty  ") +
		styleStatusUntracked.Render("[!]") + styleDim.Render(" untracked  ") +
		styleStatusLocked.Render("[L]") + styleDim.Render(" locked  ") +
		styleStatusProtected.Render("[P]") + styleDim.Render(" protected")
	if m.width == 0 || lipgloss.Width(full) <= m.width {
		return full
	}
	// Narrow terminals get the badges alone; F1 help carries the labels.
	return styleDim.Render("  ") +
		styleStatusClean.Render("[ok]") + " " +
		styleStatusDirty.Render("[~]") + " " +
		styleStatusUntracked.Render("[!]") + " " +
		styleStatusLocked.Render("[L]") + " " +
		styleStatusProtected.Render("[P]")
}
