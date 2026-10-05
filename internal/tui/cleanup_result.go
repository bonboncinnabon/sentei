package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
)

type standaloneCleanupDoneMsg struct {
	result cleanup.Result
}

func (m Model) updateCleanupResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case standaloneCleanupDoneMsg:
		m.cleanupResult = &msg.result
		return m, nil

	case tea.KeyPressMsg:
		if m.cleanupResult == nil {
			return m, nil
		}
		switch {
		case key.Matches(msg, keys.Confirm), key.Matches(msg, keys.Quit), key.Matches(msg, keys.Back):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewCleanupResult() string {
	var b strings.Builder

	r := m.cleanupResult
	if r == nil {
		b.WriteString(viewTitle(titleRunningCleanup))
		b.WriteString("\n\n")
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		fmt.Fprintf(&b, "  %s\n", shimmerLine(starFrame(m.motionTick)+" Running cleanup\u2026", rampAccent, m.motionTick))
		return b.String()
	}

	b.WriteString(viewTitle(titleCleanupComplete))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	// Check if anything was actually done
	totalActions := r.StaleRefsRemoved + r.ConfigDedupResult.Removed +
		r.GoneBranchesDeleted + r.ConfigOrphanResult.Removed +
		r.NonWtBranchesDeleted + r.WorktreesPruned

	switch {
	case len(r.Errors) == 0 && totalActions == 0 && len(r.BranchesSkipped) > 0:
		// Nothing happened but branches were skipped: never claim clean.
		fmt.Fprintf(&b, "  %s Nothing cleaned — %d %s remain (unmerged)\n\n",
			styleIndicatorWarning.Render(indicatorWarning),
			len(r.BranchesSkipped), pluralize(len(r.BranchesSkipped), "branch", "branches"))
	case len(r.Errors) == 0 && totalActions == 0:
		fmt.Fprintf(&b, "  %s Repository is clean\n\n",
			styleIndicatorDone.Render(indicatorDone))
	case totalActions > 0 && len(r.BranchesSkipped) > 0:
		fmt.Fprintf(&b, "  %s Cleanup complete — %d %s remain (unmerged)\n\n",
			styleIndicatorWarning.Render(indicatorWarning),
			len(r.BranchesSkipped), pluralize(len(r.BranchesSkipped), "branch", "branches"))
	}

	// Stale refs
	if r.StaleRefsRemoved > 0 {
		fmt.Fprintf(&b, "  %s Pruned %d stale remote %s\n",
			styleIndicatorDone.Render(indicatorDone),
			r.StaleRefsRemoved,
			pluralize(r.StaleRefsRemoved, "ref", "refs"))
	} else if len(r.Errors) == 0 {
		fmt.Fprintf(&b, "  %s No stale remote refs\n",
			styleIndicatorPending.Render(indicatorPending))
	}

	// Gone branches
	if r.GoneBranchesDeleted > 0 {
		fmt.Fprintf(&b, "  %s Deleted %d %s with gone upstream\n",
			styleIndicatorDone.Render(indicatorDone),
			r.GoneBranchesDeleted,
			pluralize(r.GoneBranchesDeleted, "branch", "branches"))
	} else if len(r.Errors) == 0 {
		fmt.Fprintf(&b, "  %s No branches with gone upstream\n",
			styleIndicatorPending.Render(indicatorPending))
	}

	// Config dedup
	if r.ConfigDedupResult.Removed > 0 {
		fmt.Fprintf(&b, "  %s Removed %d config %s\n",
			styleIndicatorDone.Render(indicatorDone),
			r.ConfigDedupResult.Removed,
			pluralize(r.ConfigDedupResult.Removed, "duplicate", "duplicates"))
	} else if len(r.Errors) == 0 {
		fmt.Fprintf(&b, "  %s No config duplicates\n",
			styleIndicatorPending.Render(indicatorPending))
	}

	// Orphaned configs
	if r.ConfigOrphanResult.Removed > 0 {
		fmt.Fprintf(&b, "  %s Removed %d orphaned config %s\n",
			styleIndicatorDone.Render(indicatorDone),
			r.ConfigOrphanResult.Removed,
			pluralize(r.ConfigOrphanResult.Removed, "section", "sections"))
	} else if len(r.Errors) == 0 {
		fmt.Fprintf(&b, "  %s No orphaned config sections\n",
			styleIndicatorPending.Render(indicatorPending))
	}

	// Pruned worktrees
	if r.WorktreesPruned > 0 {
		fmt.Fprintf(&b, "  %s Pruned %d stale %s\n",
			styleIndicatorDone.Render(indicatorDone),
			r.WorktreesPruned,
			pluralize(r.WorktreesPruned, "worktree", "worktrees"))
	} else if len(r.Errors) == 0 {
		fmt.Fprintf(&b, "  %s No stale worktrees\n",
			styleIndicatorPending.Render(indicatorPending))
	}

	// Skipped branches: a confirmed aggressive run must never look like a
	// silent success when the engine skipped unmerged branches.
	if len(r.BranchesSkipped) > 0 {
		fmt.Fprintf(&b, "  %s %d %s skipped (not fully merged)\n",
			styleIndicatorWarning.Render(indicatorWarning),
			len(r.BranchesSkipped),
			pluralize(len(r.BranchesSkipped), "branch", "branches"))
		for _, s := range r.BranchesSkipped {
			fmt.Fprintf(&b, "      %s\n", styleDim.Render(truncateWithEllipsis(s.Name, max(m.width-8, 20))))
		}
	}

	// Errors
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "  %s %s: %s\n",
			styleIndicatorFailed.Render(indicatorFailed),
			e.Step,
			styleError.Render(e.Err.Error()))
	}

	// Tip: after a safe run, point at aggressive mode; after an aggressive
	// run that skipped unmerged branches, point at --force instead of
	// re-recommending the mode that just ran.
	if m.cleanupRanMode == cleanup.ModeAggressive {
		if len(r.BranchesSkipped) > 0 {
			b.WriteString("\n")
			b.WriteString(styleDim.Render("  Tip: run `sentei cleanup --mode aggressive --force` to delete unmerged branches."))
			b.WriteString("\n")
		}
	} else if r.NonWtBranchesRemaining > 0 {
		b.WriteString("\n")
		b.WriteString(styleDim.Render(fmt.Sprintf("  Tip: %d local %s not in any worktree.",
			r.NonWtBranchesRemaining,
			pluralize(r.NonWtBranchesRemaining, "branch", "branches"))))
		b.WriteString("\n")
		b.WriteString(styleDim.Render("       Run `sentei cleanup --mode aggressive` to remove them."))
		b.WriteString("\n")
	}

	if m.cleanupRanMode != "" {
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  ran: " + BuildCLICommand("cleanup", map[string]string{"mode": string(m.cleanupRanMode)})))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, cleanupDoneFooter))
	b.WriteString("\n")

	return b.String()
}
