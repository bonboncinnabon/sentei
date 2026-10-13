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
		fmt.Fprintf(&b, "  %s\n", shimmerLine(starFrame(m.motionTick)+" Running cleanup…", rampAccent, m.motionTick))
		return b.String()
	}

	// A dry run changed nothing, so its result speaks the preview's
	// vocabulary: the preview title and would-act lines, never done marks.
	ran := m.cleanupRan()
	did := !ran.DryRun
	failed := len(r.Errors)
	title := titleCleanupComplete
	switch {
	case ran.DryRun:
		title = titleCleanupPreview
	case failed > 0:
		title = titleCleanupErrors
	}

	b.WriteString(viewTitle(title))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	gone, deleted := r.GoneBranchesDeleted, r.NonWtBranchesDeleted
	skipped := len(r.BranchesSkipped)
	totalActions := r.StaleRefsRemoved + len(gone) + r.ConfigOrphanResult.Removed +
		r.ConfigDedupResult.Removed + len(deleted) + r.WorktreesPruned

	switch {
	case ran.DryRun:
		fmt.Fprintf(&b, "  %s\n\n", styleDim.Render("Dry run: nothing was changed"))
	case failed > 0:
		fmt.Fprintf(&b, "  %s %s\n\n", styleIndicatorFailed.Render(indicatorFailed),
			styleError.Render(fmt.Sprintf("%d %s failed", failed, pluralize(failed, "step", "steps"))))
	case totalActions == 0 && skipped > 0:
		// Nothing happened but branches were skipped: never claim clean.
		fmt.Fprintf(&b, "  %s Nothing cleaned: %d %s\n\n",
			styleIndicatorWarning.Render(indicatorWarning), skipped, pluralize(skipped, "branch remains", "branches remain"))
	case totalActions == 0:
		fmt.Fprintf(&b, "  %s Repository is clean\n\n",
			styleIndicatorDone.Render(indicatorDone))
	case skipped > 0:
		fmt.Fprintf(&b, "  %s Cleanup complete: %d %s\n\n",
			styleIndicatorWarning.Render(indicatorWarning), skipped, pluralize(skipped, "branch remains", "branches remain"))
	}
	head := b.String()
	b.Reset()

	// Skipped branches: a confirmed aggressive run must never look like a
	// silent success when the engine skipped branches. Each one carries the
	// reason it was left alone.
	unmerged := 0
	if skipped > 0 {
		verb := "skipped"
		if ran.DryRun {
			verb = "would be skipped"
		}
		fmt.Fprintf(&b, "  %s %d %s %s\n", styleIndicatorWarning.Render(indicatorWarning),
			skipped, pluralize(skipped, "branch", "branches"), verb)
		// Names share one column so the reasons line up; the longest reason
		// is 23 cells, and the column gives way before the reason is cut.
		nameWidth := 0
		for _, s := range r.BranchesSkipped {
			nameWidth = max(nameWidth, len([]rune(s.Name)))
		}
		nameWidth = min(nameWidth, max(m.width-36, 20))
		for _, s := range r.BranchesSkipped {
			if s.Reason == cleanup.SkipUnmerged {
				unmerged++
			}
			fmt.Fprintf(&b, "      %-*s  %s\n", nameWidth, truncateWithEllipsis(s.Name, nameWidth), styleDim.Render(string(s.Reason)))
		}
	}

	writeCleanupErrors(&b, r.Errors, m.width)

	// Tip: after an aggressive run that left unmerged branches, point at
	// --force; otherwise point at aggressive mode when branches remain. Never
	// recommend the command that just ran.
	if ran.Mode == cleanup.ModeAggressive {
		if unmerged > 0 && !ran.Force {
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

	if ran.Mode != "" {
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  ran: " + cleanupCLICommand(ran)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, cleanupDoneFooter))
	b.WriteString("\n")
	tail := b.String()
	b.Reset()

	// Deleted branches are named: git cannot bring them back from a remote.
	// The two lists share the rows the rest of the screen leaves free, each
	// taking half unless the other needs less. An unknown terminal height
	// (no size message yet) leaves them unbounded.
	counts := [...]int{r.StaleRefsRemoved, len(gone), r.ConfigOrphanResult.Removed, r.ConfigDedupResult.Removed, r.WorktreesPruned}
	writeSafeChecks(&b, counts, did, r.Errors, "")
	if len(deleted) > 0 {
		b.WriteString(aggressiveCheck.line(len(deleted), did))
	}
	budget := len(gone) + len(deleted)
	if m.windowHeight > 0 {
		budget = m.windowHeight - strings.Count(head+b.String()+tail, "\n")
	}
	goneBudget := max(min(len(gone), max(budget-len(deleted), budget/2)), 0)
	b.Reset()

	writeSafeChecks(&b, counts, did, r.Errors, fitBranchNames(gone, goneBudget, m.width))
	if len(deleted) > 0 {
		b.WriteString(aggressiveCheck.line(len(deleted), did))
		b.WriteString(fitBranchNames(deleted, budget-goneBudget, m.width))
	}
	return head + b.String() + tail
}

// fitBranchNames lists names one per row in at most budget rows, the last of
// which counts the names that did not fit.
func fitBranchNames(names []string, budget, width int) string {
	hidden := 0
	if len(names) > budget {
		keep := max(budget-1, 0)
		hidden = len(names) - keep
		names = names[:keep]
	}

	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "      %s\n", styleDim.Render(truncateWithEllipsis(name, max(width-8, 20))))
	}
	if hidden > 0 {
		b.WriteString(styleDim.Render(fmt.Sprintf("      and %d more", hidden)) + "\n")
	}
	return b.String()
}
