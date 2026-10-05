package tui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

type teardownCompleteMsg struct {
	results []progress.StepResult
	err     error
}

const (
	unlockPhaseID   progress.PhaseID = "unlock-worktrees"
	teardownPhaseID progress.PhaseID = "teardown-integrations"
	cleanupPhaseID  progress.PhaseID = "prune-and-cleanup"
	pruneStepID     progress.StepID  = "prune-worktree-metadata"
	cleanupStepID   progress.StepID  = "repository-cleanup"
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
// state, teardown of integration artifacts when present, then the deletion
// progress. Entered from the at-risk confirmation gate, or directly from
// the list when every selected worktree is clean and pushed.
func (m Model) beginRemoval() (tea.Model, tea.Cmd) {
	m.progressStartedAt = time.Now()
	m.progressToken++
	m.view = progressView
	selected := m.selectedWorktrees()
	m.remove.run = newRemovalRun(selected)

	integrations := integration.All()
	prepared, err := prepareRemoval(selected, integrations)
	if err != nil {
		m.remove.run.result.Err = err
		m.view = summaryView
		return m, nil
	}
	ch := make(chan progress.Event, (len(prepared.targets)+len(prepared.teardownOps)+len(prepared.unlockOps)+2)*5)
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
	m.remove.run.teardownOps = prepared.teardownOps
	declarationEvents := len(prepared.unlockOps) + len(prepared.teardownOps) + len(prepared.targets) + 2 + len(prepared.plan.Phases)
	for range declarationEvents {
		m.remove.run.events = append(m.remove.run.events, <-ch)
	}
	for _, operation := range prepared.unlockOps {
		_, transitionErr := execution.Run(unlockPhaseID, operation.stepID, func() (string, error) {
			return "", worktree.UnlockWorktree(m.runner, m.repoPath, operation.worktree.Path)
		})
		m.remove.run.result.Err = errors.Join(m.remove.run.result.Err, transitionErr)
	}

	if len(prepared.teardownOps) > 0 {
		m.remove.run.teardownRunning = true
		for _, op := range prepared.teardownOps {
			m.remove.run.teardownPlanned = append(m.remove.run.teardownPlanned, op.label)
		}
		return m, tea.Batch(waitForRemovalEvent(ch), m.runFrozenTeardownPhase(prepared.teardownOps, execution))
	}

	updated, cmd := m.startDeletions()
	return updated, tea.Batch(waitForRemovalEvent(ch), cmd)
}

func prepareRemoval(worktrees []git.Worktree, integrations []integration.Integration) (removalPreparation, error) {
	prepared := removalPreparation{}
	seenWorktrees := map[string]bool{}
	seenTeardowns := map[string]bool{}
	unlock := progress.PlannedPhase{ID: unlockPhaseID, Label: "Unlock"}
	teardown := progress.PlannedPhase{ID: teardownPhaseID, Label: "Teardown"}
	removal := progress.PlannedPhase{ID: worktree.RemovalPhaseID, Label: worktree.RemovalPhaseName}

	for _, wt := range worktrees {
		identity := normalizedWorktreeIdentity(wt)
		if seenWorktrees[identity] {
			return removalPreparation{}, fmt.Errorf("preparing removal: duplicate worktree identity %q", identity)
		}
		seenWorktrees[identity] = true
		if wt.IsLocked {
			stepID := removalSemanticStepID("unlock", identity)
			prepared.unlockOps = append(prepared.unlockOps, unlockOperation{stepID: stepID, worktree: wt})
			unlock.Steps = append(unlock.Steps, progress.PlannedStep{ID: stepID, Label: worktreeLabel(wt)})
		}
		for _, integ := range integrations {
			for _, dir := range integ.Teardown.Dirs {
				if _, err := integration.ResolveManagedPath(wt.Path, dir); err != nil {
					return removalPreparation{}, fmt.Errorf("preparing removal for integration %q: %w", integ.Name, err)
				}
			}
		}
		for _, artifact := range creator.ScanArtifacts(wt.Path, integrations) {
			integ := findIntegrationByName(integrations, artifact.IntegrationName)
			if integ == nil {
				continue
			}
			type managedArtifact struct {
				dir string
			}
			artifacts := make([]managedArtifact, len(artifact.Dirs))
			for i, dir := range artifact.Dirs {
				artifacts[i].dir = filepath.Clean(strings.TrimSuffix(dir, string(filepath.Separator)))
				if _, err := integration.ResolveManagedPath(wt.Path, artifacts[i].dir); err != nil {
					return removalPreparation{}, fmt.Errorf("preparing removal for integration %q: %w", integ.Name, err)
				}
			}
			sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].dir < artifacts[j].dir })
			groupIdentity := strings.Join([]string{identity, strings.TrimSpace(integ.Name)}, "\x00")
			if seenTeardowns[groupIdentity] {
				return removalPreparation{}, fmt.Errorf("preparing removal: duplicate teardown identity for %s", integ.Name)
			}
			seenTeardowns[groupIdentity] = true
			if integ.Teardown.Command != "" {
				commandIdentity := groupIdentity + "\x00" + strings.TrimSpace(integ.Teardown.Command)
				stepID := removalSemanticStepID("teardown-command", commandIdentity)
				op := teardownOperation{groupID: groupIdentity, stepID: stepID, label: integration.TeardownStepName(*integ), kind: teardownCommand, wtPath: wt.Path, command: integ.Teardown.Command}
				prepared.teardownOps = append(prepared.teardownOps, op)
				teardown.Steps = append(teardown.Steps, progress.PlannedStep{ID: stepID, Label: op.label})
			}
			for _, artifact := range artifacts {
				stepID := removalSemanticStepID("remove-artifact", groupIdentity+"\x00"+filepath.ToSlash(artifact.dir))
				op := teardownOperation{groupID: groupIdentity, stepID: stepID, label: integration.RemoveDirStepName(artifact.dir, wt.Path), kind: teardownArtifacts, wtPath: wt.Path, managedPaths: []string{artifact.dir}}
				prepared.teardownOps = append(prepared.teardownOps, op)
				teardown.Steps = append(teardown.Steps, progress.PlannedStep{ID: stepID, Label: op.label})
			}
		}
		stepID := removalSemanticStepID("remove", identity)
		prepared.targets = append(prepared.targets, worktree.RemovalTarget{Worktree: wt, StepID: stepID})
		removal.Steps = append(removal.Steps, progress.PlannedStep{ID: stepID, Label: worktreeLabel(wt), Checkpoints: 2})
	}
	if len(unlock.Steps) > 0 {
		prepared.plan.Phases = append(prepared.plan.Phases, unlock)
	}
	if len(teardown.Steps) > 0 {
		prepared.plan.Phases = append(prepared.plan.Phases, teardown)
	}
	prepared.plan.Phases = append(prepared.plan.Phases, removal, progress.PlannedPhase{
		ID: cleanupPhaseID, Label: "Prune & cleanup",
		Steps: []progress.PlannedStep{{ID: pruneStepID, Label: "Prune worktree metadata"}, {ID: cleanupStepID, Label: "Repository cleanup"}},
	})
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
// nowhere else: uncommitted or untracked changes, commits not on a remote,
// or a lock someone placed deliberately.
func worktreeAtRisk(wt git.Worktree) bool {
	return wt.HasUncommittedChanges || wt.HasUntrackedFiles || wt.HasUnpushedCommits || wt.IsLocked
}

// startDeletions kicks off the deletion goroutine for the current run's
// worktree snapshot and begins consuming its events.
func (m Model) startDeletions() (tea.Model, tea.Cmd) {
	execution := m.remove.run.execution
	targets := append([]worktree.RemovalTarget(nil), m.remove.run.targets...)
	return m, func() tea.Msg {
		result := worktree.DeleteWorktrees(execution, worktree.RemovalPhaseID, os.RemoveAll, targets, 5)
		return deletionsCompleteMsg{Result: result}
	}
}

func findIntegrationByName(integrations []integration.Integration, name string) *integration.Integration {
	for i := range integrations {
		if integrations[i].Name == name {
			return &integrations[i]
		}
	}
	return nil
}

const maxTeardownConcurrency = 5

func (m Model) runFrozenTeardownPhase(operations []teardownOperation, execution *progress.Execution) tea.Cmd {
	return func() tea.Msg {
		shell := m.shell
		type indexedResults struct {
			index   int
			results []progress.StepResult
			err     error
		}

		type operationGroup struct {
			id         string
			operations []teardownOperation
		}
		var groups []operationGroup
		for _, operation := range operations {
			if len(groups) == 0 || groups[len(groups)-1].id != operation.groupID {
				groups = append(groups, operationGroup{id: operation.groupID})
			}
			groups[len(groups)-1].operations = append(groups[len(groups)-1].operations, operation)
		}

		resultsCh := make(chan indexedResults, len(groups))
		sem := make(chan struct{}, maxTeardownConcurrency)

		for i, group := range groups {
			sem <- struct{}{}
			go func(idx int, group operationGroup) {
				defer func() { <-sem }()
				indexed := indexedResults{index: idx}
				for _, op := range group.operations {
					result, transitionErr := execution.Run(teardownPhaseID, op.stepID, func() (string, error) {
						switch op.kind {
						case teardownCommand:
							return shell.RunShell(op.wtPath, op.command)
						case teardownArtifacts:
							for _, managedPath := range op.managedPaths {
								path, err := integration.ResolveManagedPath(op.wtPath, managedPath)
								if err != nil {
									return "", fmt.Errorf("revalidating artifact path: %w", err)
								}
								if err := os.RemoveAll(path); err != nil {
									return "", err
								}
							}
							return "removed artifact dirs", nil
						default:
							return "", fmt.Errorf("unknown teardown operation kind %d", op.kind)
						}
					})
					indexed.results = append(indexed.results, result)
					indexed.err = errors.Join(indexed.err, transitionErr)
				}
				resultsCh <- indexed
			}(i, group)
		}

		collected := make([]indexedResults, len(groups))
		for range groups {
			ir := <-resultsCh
			collected[ir.index] = ir
		}

		var allResults []progress.StepResult
		var runErr error
		for _, indexed := range collected {
			allResults = append(allResults, indexed.results...)
			runErr = errors.Join(runErr, indexed.err)
		}
		return teardownCompleteMsg{results: allResults, err: runErr}
	}
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
	nameWidth := 0
	for _, wt := range selected {
		nameWidth = max(nameWidth, len([]rune(worktreeLabel(wt))))
	}
	nameWidth = min(nameWidth, confirmNameWidthCap)

	var dirtyCount, untrackedCount, lockedCount, unpushedCount int
	for _, wt := range selected {
		var badge, note string
		risky := true
		switch {
		case wt.IsLocked:
			badge, note = "[L]", "locked — will force-remove"
			lockedCount++
		case wt.HasUncommittedChanges:
			badge, note = "[~]", "uncommitted changes — will be lost"
			dirtyCount++
		case wt.HasUntrackedFiles:
			badge, note = "[!]", "untracked files — will be lost"
			untrackedCount++
		case wt.HasUnpushedCommits:
			badge, note = "[^]", "commits not on any remote"
			unpushedCount++
		default:
			badge, risky = "[ok]", false
		}

		badgeStyle := styleStatusClean
		if risky {
			badgeStyle = styleWarning
		}
		name := truncateWithEllipsis(worktreeLabel(wt), nameWidth)
		// Pad by rune count: fmt's %-*s pads by bytes and drifts on …
		pad := strings.Repeat(" ", max(nameWidth-len([]rune(name)), 0))
		line := fmt.Sprintf("    %s  %s%s", badgeStyle.Render(fmt.Sprintf("%-4s", badge)), name, pad)
		if risky {
			line += "  " + styleWarning.Render(note)
		}
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}

	b.WriteString("\n")

	// Integration teardown info
	integrations := integration.All()
	dirCounts := make(map[string]int)
	for _, wt := range selected {
		artifacts := creator.ScanArtifacts(wt.Path, integrations)
		for _, a := range artifacts {
			for _, d := range a.Dirs {
				dirCounts[d]++
			}
		}
	}
	if len(dirCounts) > 0 {
		b.WriteString("  Cleaning up:\n\n")
		for dir, count := range dirCounts {
			fmt.Fprintf(&b, "    %-28s in %d %s\n", dir, count, pluralize(count, "worktree", "worktrees"))
		}
		b.WriteString("\n")
	}

	if dirtyCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  ⚠ %d %s with uncommitted changes that will be lost", dirtyCount, pluralize(dirtyCount, "worktree", "worktrees")),
		))
		b.WriteString("\n")
	}
	if untrackedCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  ⚠ %d %s with untracked files that will be lost", untrackedCount, pluralize(untrackedCount, "worktree", "worktrees")),
		))
		b.WriteString("\n")
	}
	if lockedCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  ⚠ %d %s locked and will be force-removed", lockedCount, pluralize(lockedCount, "worktree", "worktrees")),
		))
		b.WriteString("\n")
	}
	if unpushedCount > 0 {
		b.WriteString(styleWarning.Render(
			fmt.Sprintf("  ⚠ %d %s with commits not pushed to any remote", unpushedCount, pluralize(unpushedCount, "worktree", "worktrees")),
		))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooterDanger(m.width, confirmFooter) + "\n")

	return b.String()
}
