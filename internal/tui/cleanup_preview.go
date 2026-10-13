package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
)

// inlineBranchPreview is how many aggressive branch names the preview shows
// before deferring to the detail portal.
const inlineBranchPreview = 3

type cleanupScanDoneMsg struct {
	result cleanup.DryRunResult
	err    error
}

// cleanupScanRevealMsg promotes a finished scan to the preview once the
// minimum scanning display has elapsed; the token guards stale timers.
type cleanupScanRevealMsg struct{ token int }

// startCleanupScan fires the dry-run scan for the preview.
func (m Model) startCleanupScan() (Model, tea.Cmd) {
	m.view = cleanupPreviewView
	m.cleanupScan = nil
	m.cleanupScanPending = nil
	m.cleanupScanErr = nil
	m.cleanupAggressiveConfirm = false
	m.progressStartedAt = time.Now()
	m.progressToken++
	runner, repoPath := m.runner, m.repoPath
	// No explicit spinner tick: the dispatch wrapper starts the chain on
	// the transition into this working state; a second start here would
	// double the frame rate.
	return m, func() tea.Msg {
		result, err := cleanup.DryRun(runner, repoPath)
		return cleanupScanDoneMsg{result: result, err: err}
	}
}

func (m Model) updateCleanupPreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cleanupScanDoneMsg:
		if msg.err != nil {
			m.cleanupScanErr = msg.err
			return m, nil
		}
		// Keep the scanning state visible for the minimum display duration,
		// then reveal (same contract as holdOrAdvance, but for data).
		if remaining := m.minProgressDuration - time.Since(m.progressStartedAt); remaining > 0 {
			m.cleanupScanPending = &msg.result
			token := m.progressToken
			return m, tea.Tick(remaining, func(time.Time) tea.Msg {
				return cleanupScanRevealMsg{token: token}
			})
		}
		m.cleanupScan = &msg.result
		return m, nil

	case cleanupScanRevealMsg:
		if msg.token == m.progressToken && m.cleanupScanPending != nil {
			m.cleanupScan = m.cleanupScanPending
			m.cleanupScanPending = nil
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.cleanupAggressiveConfirm {
			switch {
			case key.Matches(msg, keys.Yes):
				return m.startCleanupRun(cleanup.Options{Mode: cleanup.ModeAggressive})
			case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
				m.cleanupAggressiveConfirm = false
				return m, nil
			case key.Matches(msg, keys.Quit):
				return m, tea.Quit
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Back):
			m.view = menuView
			return m, nil

		case key.Matches(msg, keys.Confirm):
			if m.cleanupScan == nil {
				return m, nil
			}
			// With nothing to clean the footer offers enter as "back".
			if cleanupScanIsClean(*m.cleanupScan) {
				m.view = menuView
				return m, nil
			}
			return m.startCleanupRun(cleanup.Options{Mode: cleanup.ModeSafe})

		case key.Matches(msg, keys.All):
			// The gate opens only when aggressive mode would actually delete
			// something; unmerged-only candidates need --force on the CLI.
			if m.cleanupScan != nil && m.cleanupScan.DeletableAggressiveCount() > 0 {
				m.cleanupAggressiveConfirm = true
			}
			return m, nil
		}
	}
	return m, nil
}

// cleanupScanIsClean reports whether the scan checked everything and found
// nothing to do. A scan with a failed check is never clean: what that check
// would have found is unknown.
func cleanupScanIsClean(scan cleanup.DryRunResult) bool {
	return !scan.SafeHasWork() && !scan.AggressiveHasWork() && len(scan.Errors) == 0
}

func (m Model) viewCleanupPreview() string {
	var b strings.Builder

	b.WriteString(viewTitle(titleCleanupPreview))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	if m.cleanupScanErr != nil {
		fmt.Fprintf(&b, "  %s %s\n\n", styleIndicatorFailed.Render(indicatorFailed),
			styleError.Render(truncateWithEllipsis("Scan failed: "+m.cleanupScanErr.Error(), max(m.width-6, 20))))
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		b.WriteString(viewFooter(m.width, cleanupScanFooter))
		b.WriteString("\n")
		return b.String()
	}

	scan := m.cleanupScan
	if scan == nil {
		fmt.Fprintf(&b, "  %s\n\n", shimmerLine(starFrame(m.motionTick)+" Scanning repository…", rampAccent, m.motionTick))
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		b.WriteString(viewFooter(m.width, cleanupScanFooter))
		b.WriteString("\n")
		return b.String()
	}

	safeCounts := [...]int{scan.StaleRefs, len(scan.GoneBranches), scan.OrphanedConfigs, scan.ConfigDuplicates, scan.PrunableWorktrees}

	if cleanupScanIsClean(*scan) {
		fmt.Fprintf(&b, "  %s Repository is clean\n\n", styleIndicatorDone.Render(indicatorDone))
		// Show what was checked: the user should learn what cleanup covers
		// before running it, not after.
		writeSafeChecks(&b, safeCounts, false, nil)
		b.WriteString("\n")
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		b.WriteString(viewFooter(m.width, cleanupEmptyFooter))
		b.WriteString("\n")
		return b.String()
	}

	if failed := len(scan.Errors); failed > 0 {
		fmt.Fprintf(&b, "  %s\n", styleWarning.Render(fmt.Sprintf("%s Scan incomplete: %d %s failed",
			indicatorWarning, failed, pluralize(failed, "check", "checks"))))
		writeCleanupErrors(&b, scan.Errors, m.width)
		b.WriteString("\n")
	}

	b.WriteString(styleTitle.Render("  Safe cleanup:"))
	b.WriteString("\n")
	writeSafeChecks(&b, safeCounts, false, scan.Errors)
	b.WriteString("\n")

	if scan.AggressiveHasWork() {
		n := len(scan.AggressiveBranches)
		deletable := scan.DeletableAggressiveCount()
		b.WriteString(styleTitle.Render("  Aggressive cleanup available:"))
		b.WriteString("\n")
		// The headline states what WILL happen, not what was found; the
		// dim --force note below carries the remainder.
		switch {
		case deletable == 0:
			fmt.Fprintf(&b, "  %s %d %s not in any worktree — none deletable without --force\n",
				styleIndicatorWarning.Render(indicatorWarning), n, pluralize(n, "branch", "branches"))
		case deletable < n:
			fmt.Fprintf(&b, "  %s %d of %d branches not in any worktree would be deleted\n",
				styleIndicatorWarning.Render(indicatorWarning), deletable, n)
		default:
			fmt.Fprintf(&b, "  %s %d %s not in any worktree would be deleted\n",
				styleIndicatorWarning.Render(indicatorWarning), n, pluralize(n, "branch", "branches"))
		}
		shown := min(n, inlineBranchPreview)
		for _, info := range scan.AggressiveBranches[:shown] {
			name := truncateWithEllipsis(info.Name, max(m.width-8, 20))
			if !info.Merged {
				name += " " + styleWarning.Render("(not merged)")
			}
			fmt.Fprintf(&b, "    %s %s\n", styleDim.Render(indicatorPending), name)
		}
		if rest := n - shown; rest > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("    and %d more — ? for details", rest)))
			b.WriteString("\n")
		}
		if unmerged := scan.UnmergedAggressiveCount(); unmerged > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("    %d not fully merged — only deleted with --force on the CLI", unmerged)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if m.cleanupAggressiveConfirm {
		deletable := len(scan.AggressiveBranches) - scan.UnmergedAggressiveCount()
		prompt := fmt.Sprintf("  Delete %d %s?", deletable, pluralize(deletable, "branch", "branches"))
		if unmerged := scan.UnmergedAggressiveCount(); unmerged > 0 {
			prompt += fmt.Sprintf(" (%d unmerged will be skipped)", unmerged)
		}
		b.WriteString(styleWarning.Render(prompt))
		b.WriteString("\n\n")
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		b.WriteString(viewFooter(m.width, confirmFooter))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	hints := []key.Binding{cleanupSafeHint}
	if scan.DeletableAggressiveCount() > 0 {
		hints = append(hints, aggressiveHint)
	}
	if scan.AggressiveHasWork() {
		hints = append(hints, detailsHint)
	}
	hints = append(hints, keys.Back, keys.Quit)
	b.WriteString(viewFooter(m.width, hints))
	b.WriteString("\n")

	return b.String()
}

// cleanupCheck is one category of work cleanup looks for, worded for the two
// vocabularies that list it: what a run would do (the preview and a dry-run
// result) and what a run did. One table for both screens keeps their wording
// and their order in step.
type cleanupCheck struct {
	step             string // the cleanup step that produces the count
	would            string // takes the noun: "stale remote %s would be pruned"
	did              string // takes the count and the noun: "Pruned %d stale remote %s"
	singular, plural string
	none             string // shown when the step ran and found nothing
}

var (
	// safeChecks are the checks every cleanup makes, in display order.
	safeChecks = [...]cleanupCheck{
		{"prune-refs", "stale remote %s would be pruned", "Pruned %d stale remote %s", "ref", "refs", "No stale remote refs"},
		{"gone-branches", "%s with gone upstream would be deleted", "Deleted %d %s with gone upstream", "branch", "branches", "No branches with gone upstream"},
		{"orphaned-configs", "orphaned config %s would be removed", "Removed %d orphaned config %s", "section", "sections", "No orphaned config sections"},
		{"dedup-config", "duplicate config %s would be removed", "Removed %d duplicate config %s", "line", "lines", "No duplicate config lines"},
		{"worktree-prune", "stale %s would be pruned", "Pruned %d stale %s", "worktree", "worktrees", "No stale worktrees"},
	}
	// aggressiveCheck is the one check only aggressive mode acts on.
	aggressiveCheck = cleanupCheck{"non-wt-branches", "%s not in any worktree would be deleted", "Deleted %d %s not in any worktree", "branch", "branches", ""}
)

// line renders the check for count findings: in the past tense with the done
// indicator when did is set, as a would-act line otherwise.
func (c cleanupCheck) line(count int, did bool) string {
	noun := pluralize(count, c.singular, c.plural)
	switch {
	case count > 0 && did:
		return fmt.Sprintf("  %s %s\n", styleIndicatorDone.Render(indicatorDone), fmt.Sprintf(c.did, count, noun))
	case count > 0:
		return fmt.Sprintf("  %s %d %s\n", styleAccent.Render(indicatorWouldAct), count, fmt.Sprintf(c.would, noun))
	case did:
		return fmt.Sprintf("  %s %s\n", styleIndicatorPending.Render(indicatorPending), c.none)
	default:
		return fmt.Sprintf("  %s %s\n", styleIndicatorPending.Render(indicatorPending), styleDim.Render(c.none))
	}
}

// writeSafeChecks renders safeChecks with their counts, given in the same
// order. A check whose step failed and reported nothing is left out: its
// count is unknown, and the failure is listed by writeCleanupErrors.
func writeSafeChecks(b *strings.Builder, counts [len(safeChecks)]int, did bool, errs []cleanup.OperationError) {
	for i, check := range safeChecks {
		if counts[i] == 0 && cleanupStepFailed(errs, check.step) {
			continue
		}
		b.WriteString(check.line(counts[i], did))
	}
}

func cleanupStepFailed(errs []cleanup.OperationError, step string) bool {
	for _, e := range errs {
		if e.Step == step {
			return true
		}
	}
	return false
}

// writeCleanupErrors lists the failed steps, one row per failure. The reason
// is wrapped, not cut: its end is where git says what went wrong. Only the
// first line of a multi-line git error is shown; it names the command and the
// cause.
func writeCleanupErrors(b *strings.Builder, errs []cleanup.OperationError, width int) {
	for _, e := range errs {
		// A step that deletes branches joins one failure per branch.
		failures := []error{e.Err}
		if joined, ok := e.Err.(interface{ Unwrap() []error }); ok {
			failures = joined.Unwrap()
		}
		for _, failure := range failures {
			reason := ""
			if lines := nonEmptyLines(failure.Error()); len(lines) > 0 {
				reason = lines[0]
			}
			prefix := fmt.Sprintf("  %s %s: ", styleIndicatorFailed.Render(indicatorFailed), e.Step)
			// An unknown width (no size message yet) falls back like viewSeparator.
			writeProgressDetailValue(b, prefix, styleError.Render(reason), max(width, 40))
		}
	}
}

// cleanupDetailContent renders the full aggressive branch list for the
// detail portal.
func (m Model) cleanupDetailContent() (string, string) {
	if m.cleanupScan == nil || !m.cleanupScan.AggressiveHasWork() {
		return "", ""
	}
	var b strings.Builder
	n := len(m.cleanupScan.AggressiveBranches)
	fmt.Fprintf(&b, "  %s\n\n", styleDim.Render(fmt.Sprintf("%d %s aggressive cleanup targets:", n, pluralize(n, "branch", "branches"))))
	nameWidth := 0
	for _, info := range m.cleanupScan.AggressiveBranches {
		nameWidth = max(nameWidth, len(info.Name))
	}
	for _, info := range m.cleanupScan.AggressiveBranches {
		date := ""
		if !info.LastCommitDate.IsZero() {
			date = info.LastCommitDate.Format("2006-01-02")
		}
		marker := "  "
		if !info.Merged {
			marker = styleWarning.Render(indicatorWarning) + " "
		}
		fmt.Fprintf(&b, "  %s%s  %s  %s\n",
			marker,
			fmt.Sprintf("%-*s", nameWidth, info.Name),
			styleDim.Render(fmt.Sprintf("%-10s", date)),
			truncateWithEllipsis(info.LastCommitSubject, max(m.portal.contentWidth()-nameWidth-18, 10)))
	}
	if unmerged := m.cleanupScan.UnmergedAggressiveCount(); unmerged > 0 {
		fmt.Fprintf(&b, "\n  %s\n", styleDim.Render(fmt.Sprintf("%s %d not fully merged: skipped unless --force is used on the CLI", indicatorWarning, unmerged)))
	}
	return portalAggressiveDetails, b.String()
}
