package tui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Yes):
			return m.beginRemoval()

		case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
			m.view = listView
		}

	}
	return m, nil
}

// beginRemoval starts the deletion of the current selection: fresh run
// state, then the deletion progress. Entered from the at-risk confirmation
// gate, or directly from the list when every selected worktree is clean and
// pushed.
func (m Model) beginRemoval() (tea.Model, tea.Cmd) {
	m.progressStartedAt = time.Now()
	m.progressToken++
	m.view = progressView
	m.remove.run = removalRun{}

	prepared, err := prepareRemoval(m.selectedWorktrees())
	if err != nil {
		m.remove.run.result.Err = err
		m.view = summaryView
		return m, nil
	}
	ch := make(chan progress.Event, (len(prepared.targets)+len(prepared.plan.Phases))*5)
	execution, err := progress.Start(prepared.plan, func(event progress.Event) { ch <- event })
	if err != nil {
		close(ch)
		m.remove.run.result.Err = fmt.Errorf("starting removal progress: %w", err)
		m.view = summaryView
		return m, nil
	}
	m.remove.run.execution = execution
	m.remove.run.progressCh = ch
	m.remove.run.targets = prepared.targets
	declarationEvents := len(prepared.targets) + len(prepared.plan.Phases)
	for range declarationEvents {
		m.remove.run.events = append(m.remove.run.events, <-ch)
	}

	updated, cmd := m.startDeletions()
	return updated, tea.Batch(waitForRemovalEvent(ch), cmd)
}

func prepareRemoval(worktrees []git.Worktree) (removalPreparation, error) {
	prepared := removalPreparation{}
	seenWorktrees := map[string]bool{}
	removal := progress.PlannedPhase{ID: worktree.RemovalPhaseID, Label: worktree.RemovalPhaseName}

	for _, wt := range worktrees {
		identity := normalizedWorktreeIdentity(wt)
		if seenWorktrees[identity] {
			return removalPreparation{}, fmt.Errorf("preparing removal: duplicate worktree identity %q", identity)
		}
		seenWorktrees[identity] = true
		stepID := removalSemanticStepID("remove", identity)
		prepared.targets = append(prepared.targets, worktree.RemovalTarget{Worktree: wt, StepID: stepID})
		removal.Steps = append(removal.Steps, progress.PlannedStep{ID: stepID, Label: worktreeLabel(wt), Checkpoints: 2})
	}
	prepared.plan.Phases = []progress.PlannedPhase{removal}
	return prepared, nil
}

func normalizedWorktreeIdentity(wt git.Worktree) string {
	return filepath.Clean(strings.TrimSpace(wt.Path)) + "\x00" + strings.TrimSpace(wt.Branch)
}

func removalSemanticStepID(kind, identity string) progress.StepID {
	sum := sha256.Sum256([]byte(identity))
	return progress.StepID(fmt.Sprintf("%s:%x", kind, sum[:8]))
}

// worktreeAtRisk reports whether removing wt could lose work that exists
// nowhere else: uncommitted or untracked changes, commits not on a remote, a
// status that could not be read, or a lock someone placed deliberately.
func worktreeAtRisk(wt git.Worktree) bool {
	return worktree.AtRisk(wt) || wt.IsLocked
}

// startDeletions kicks off the deletion goroutine for the current run's
// worktree snapshot and begins consuming its events.
func (m Model) startDeletions() (tea.Model, tea.Cmd) {
	execution := m.remove.run.execution
	targets := append([]worktree.RemovalTarget(nil), m.remove.run.targets...)
	remove := worktreeRemover(m.runner, m.repoPath, targets)
	return m, func() tea.Msg {
		result := worktree.DeleteWorktrees(execution, worktree.RemovalPhaseID, remove, targets, 5)
		return deletionsCompleteMsg{Result: result}
	}
}

// worktreeRemover returns the function that removes one target by path.
//
// The status shown in the list is a snapshot taken when it loaded. A target
// that was at risk then went through the confirmation screen. A target that
// looked clean did not, so its status is read again at the moment of deletion
// and it is left alone if it has gained work since.
//
// Removal always passes --force: without it git refuses every worktree that
// contains submodules, clean or not. A target that was locked when it was
// confirmed gets --force twice, which is how git removes a locked worktree.
// Nothing is unlocked first, so a removal that fails or never starts leaves
// the lock and its reason in place, and a lock placed after the list loaded
// is still refused.
func worktreeRemover(runner git.CommandRunner, repoPath string, targets []worktree.RemovalTarget) func(path string) error {
	unconfirmed := make(map[string]git.Worktree, len(targets))
	locked := make(map[string]bool)
	for _, target := range targets {
		if !worktreeAtRisk(target.Worktree) {
			unconfirmed[target.Worktree.Path] = target.Worktree
		}
		locked[target.Worktree.Path] = target.Worktree.IsLocked
	}
	return func(path string) error {
		if wt, ok := unconfirmed[path]; ok {
			fresh := worktree.EnrichWorktrees(runner, []git.Worktree{wt}, 1)
			if worktree.AtRisk(fresh[0]) {
				return errors.New("changed since the list loaded: reopen the list to review it")
			}
		}
		args := []string{"worktree", "remove", "--force"}
		if locked[path] {
			args = append(args, "--force")
		}
		_, err := runner.Run(repoPath, append(args, path)...)
		return err
	}
}

// confirmRisk is the badge and note the confirm screen shows for wt. A
// worktree with nothing at risk has no note.
func confirmRisk(wt git.Worktree) (badge, note string) {
	switch {
	case wt.IsLocked:
		return "[L]", "locked — will force-remove"
	case wt.HasUncommittedChanges:
		return "[~]", "uncommitted changes — will be lost"
	case wt.HasUntrackedFiles:
		return "[!]", "untracked files — will be lost"
	case wt.HasUnpushedCommits:
		return "[^]", "commits not on any remote"
	case wt.EnrichmentError != "":
		return "[?]", "status could not be read"
	}
	return "[ok]", ""
}

// confirmTotals are the warnings under the rows, one per kind of risk
// present, in this order. Each completes "⚠ N worktrees ...".
var confirmTotals = []struct{ badge, text string }{
	{"[~]", "with uncommitted changes that will be lost"},
	{"[!]", "with untracked files that will be lost"},
	{"[L]", "locked and will be force-removed"},
	{"[^]", "with commits not pushed to any remote"},
	{"[?]", "whose status could not be read and may hold uncommitted work"},
}

func (m Model) viewConfirm() string {
	var b strings.Builder

	selected := m.selectedWorktrees()

	b.WriteString(viewTitle(titleConfirmDeletion))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "  You are about to delete %d %s:\n\n", len(selected), pluralize(len(selected), "worktree", "worktrees"))

	// Columnar rows: badge gutter first so one vertical sweep answers
	// "anything risky?", names aligned after it, notes only on at-risk rows.
	nameWidth, noteWidth := 0, 0
	for _, wt := range selected {
		_, note := confirmRisk(wt)
		nameWidth = max(nameWidth, lipgloss.Width(worktreeLabel(wt)))
		noteWidth = max(noteWidth, lipgloss.Width(note))
	}
	nameWidth = min(nameWidth, confirmNameWidthCap)
	// A narrow terminal takes the room from the name column first. When not
	// even a short name fits beside the notes, the notes go: the badge and
	// the totals below still state every risk.
	showNotes := m.width == 0 || m.width-confirmRowLead-2-noteWidth >= min(nameWidth, confirmNameWidthMin)
	if m.width > 0 {
		room := m.width - confirmRowLead
		if showNotes && noteWidth > 0 {
			room -= 2 + noteWidth
		}
		nameWidth = min(nameWidth, max(room, 1))
	}

	var rows []confirmRow
	counts := map[string]int{}
	for _, wt := range selected {
		badge, note := confirmRisk(wt)
		risky := note != ""
		counts[badge]++

		badgeStyle := styleStatusClean
		if risky {
			badgeStyle = styleWarning
		}
		name := truncateWithEllipsis(worktreeLabel(wt), nameWidth)
		// Pad by display width: fmt's %-*s pads by bytes and drifts on …
		pad := strings.Repeat(" ", max(nameWidth-lipgloss.Width(name), 0))
		line := fmt.Sprintf("    %s  %s%s", badgeStyle.Render(fmt.Sprintf("%-4s", badge)), name, pad)
		if risky && showNotes {
			line += "  " + styleWarning.Render(note)
		}
		rows = append(rows, confirmRow{line: strings.TrimRight(line, " "), risky: risky})
	}

	head := b.String()
	b.Reset()
	b.WriteString("\n")

	for _, total := range confirmTotals {
		if n := counts[total.badge]; n > 0 {
			b.WriteString(styleWarning.Render(fmt.Sprintf("  ⚠ %d %s %s", n, pluralize(n, "worktree", "worktrees"), total.text)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooterDanger(m.width, confirmFooter) + "\n")
	tail := b.String()

	// The totals and the y/n footer must stay on screen. An unknown terminal
	// height (no size message yet) leaves the rows unbounded.
	budget := len(rows)
	if m.windowHeight > 0 {
		budget = m.windowHeight - strings.Count(head, "\n") - strings.Count(tail, "\n")
	}
	return fitLines(head+fitConfirmRows(rows, budget)+tail, m.width)
}

type confirmRow struct {
	line  string
	risky bool
}

// fitConfirmRows renders rows in at most budget lines. When they do not fit,
// the at-risk rows are kept first and the rest is summarised on one line, so
// what follows the list never scrolls off the terminal.
func fitConfirmRows(rows []confirmRow, budget int) string {
	hidden, hiddenRisky := 0, 0
	if len(rows) > budget {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].risky && !rows[j].risky })
		keep := max(budget-1, 0)
		for _, row := range rows[keep:] {
			hidden++
			if row.risky {
				hiddenRisky++
			}
		}
		rows = rows[:keep]
	}

	var b strings.Builder
	for _, row := range rows {
		b.WriteString(row.line + "\n")
	}
	if hidden > 0 {
		more := fmt.Sprintf("    … and %d more", hidden)
		if hiddenRisky > 0 {
			more += fmt.Sprintf(", %d of them at risk", hiddenRisky)
		}
		b.WriteString(styleDim.Render(more) + "\n")
	}
	return b.String()
}
