package cleanup

import (
	"slices"
	"strings"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// DryRunResult is one scan's answer to "what would cleanup do?", structured
// for display: counts for the bulk categories and branch names where the
// preview shows them. Safe and aggressive findings come from the same scan
// so the TUI never runs the repository inspection twice.
type DryRunResult struct {
	StaleRefs         int
	GoneBranches      []string // safe mode deletes these (gone upstream, fully merged, not in a worktree)
	OrphanedConfigs   int
	PrunableWorktrees int

	// AggressiveBranches are local branches in no worktree and not protected,
	// less GoneBranches: the additional set only aggressive mode deletes.
	AggressiveBranches []BranchInfo

	// Errors holds the checks that could not be made. A count of zero next to
	// an error for the same step means "unknown", not "nothing to do".
	Errors []OperationError
}

// BranchInfo carries the metadata the detail portal shows per branch.
type BranchInfo struct {
	Name              string
	LastCommitDate    time.Time
	LastCommitSubject string
	// Merged reports whether the branch is fully merged into HEAD: without
	// --force, aggressive cleanup only deletes merged branches, and the
	// preview must not promise more than that.
	Merged bool
}

// SafeHasWork reports whether a safe-mode cleanup would change anything.
func (r DryRunResult) SafeHasWork() bool {
	return r.StaleRefs > 0 || len(r.GoneBranches) > 0 ||
		r.OrphanedConfigs > 0 || r.PrunableWorktrees > 0
}

// AggressiveHasWork reports whether aggressive mode would delete branches
// beyond what safe mode does.
func (r DryRunResult) AggressiveHasWork() bool {
	return len(r.AggressiveBranches) > 0
}

// UnmergedAggressiveCount reports how many aggressive candidates are not
// fully merged and therefore survive aggressive mode unless --force is given.
func (r DryRunResult) UnmergedAggressiveCount() int {
	n := 0
	for _, b := range r.AggressiveBranches {
		if !b.Merged {
			n++
		}
	}
	return n
}

// DryRun inspects the repository without mutating it and returns what both
// cleanup modes would do. It runs the same steps as Run in dry-run mode, so
// the preview and the run cannot disagree about what a step finds. Individual
// probe failures are collected in Errors; only a path that is not a
// repository aborts the scan.
func DryRun(runner git.CommandRunner, repoPath string) (DryRunResult, error) {
	if err := git.ValidateRepository(runner, repoPath); err != nil {
		return DryRunResult{}, err
	}

	noop := func(Event) {}
	probe := Options{Mode: ModeSafe, DryRun: true}
	var result DryRunResult
	record := func(step string, err error) {
		if err != nil {
			result.Errors = append(result.Errors, OperationError{Step: step, Err: err})
		}
	}

	stale, err := PruneRemoteRefs(runner, repoPath, probe, noop)
	record("prune-refs", err)
	result.StaleRefs = len(stale)

	gone, err := DeleteGoneBranches(runner, repoPath, stale, probe, noop)
	record("gone-branches", err)
	result.GoneBranches = gone.Deleted

	orphans, err := PurgeOrphanedBranchConfigs(runner, repoPath, probe, noop)
	record("orphaned-configs", err)
	result.OrphanedConfigs = orphans.Removed

	result.PrunableWorktrees, err = pruneWorktrees(runner, repoPath, probe, noop)
	record("worktree-prune", err)

	candidates, err := listNonWorktreeCandidates(runner, repoPath, gone.Deleted)
	record("non-wt-branches", err)
	if len(candidates) > 0 {
		meta, err := branchMetadata(runner, repoPath)
		record("branch-metadata", err)
		for _, name := range candidates {
			result.AggressiveBranches = append(result.AggressiveBranches, BranchInfo{
				Name:              name,
				LastCommitDate:    meta[name].LastCommitDate,
				LastCommitSubject: meta[name].LastCommitSubject,
				Merged:            branchDeletableByGit(runner, repoPath, name, stale),
			})
		}
	}

	return result, nil
}

// branchDeletableByGit reports whether `git branch -d` accepts branch. git
// refuses unless the branch is fully merged into its upstream when that ref
// exists, and into HEAD otherwise. It checks the upstream, not HEAD, when both
// exist. An upstream listed in stale (as PruneRemoteRefs returns it) counts as
// missing: the prune removes it before any branch is deleted.
//
// The preview uses this to announce only what an unforced run deletes, and
// deleteBranches uses it to tell an unmerged branch from a failed deletion.
func branchDeletableByGit(runner git.CommandRunner, repoPath, branch string, stale []string) bool {
	base := "HEAD"
	if upstream, err := runner.Run(repoPath, "rev-parse", "--abbrev-ref", branch+"@{upstream}"); err == nil {
		if u := strings.TrimSpace(upstream); u != "" && !slices.Contains(stale, u) {
			base = u
		}
	}
	// `merge-base --is-ancestor` exits 0 when branch is reachable from base
	// (i.e. merged) and non-zero otherwise; a bad/gone base also fails, which
	// conservatively marks the branch unmerged.
	_, err := runner.Run(repoPath, "merge-base", "--is-ancestor", branch, base)
	return err == nil
}

// branchMetadata fetches commit date and subject for every local branch in
// one git call.
func branchMetadata(runner git.CommandRunner, repoPath string) (map[string]BranchInfo, error) {
	// lstrip=2, not short: see localBranches, whose names key the map.
	output, err := runner.Run(repoPath, "for-each-ref",
		"--format=%(refname:lstrip=2)\x1f%(committerdate:iso8601-strict)\x1f%(subject)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	meta := make(map[string]BranchInfo)
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) != 3 {
			continue
		}
		date, _ := time.Parse(time.RFC3339, parts[1])
		meta[parts[0]] = BranchInfo{Name: parts[0], LastCommitDate: date, LastCommitSubject: parts[2]}
	}
	return meta, nil
}

// DeletableAggressiveCount reports how many aggressive candidates aggressive
// mode would actually delete without --force.
func (r DryRunResult) DeletableAggressiveCount() int {
	return len(r.AggressiveBranches) - r.UnmergedAggressiveCount()
}
