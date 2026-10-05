package cleanup

import (
	"fmt"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

type BranchCleanResult struct {
	Deleted   int
	Remaining int
	Skipped   []SkippedBranch
}

func DeleteGoneBranches(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) (BranchCleanResult, error) {
	emit(Event{Step: "gone-branches", Message: "Deleting branches with gone upstream...", Level: LevelStep})

	output, err := runner.Run(repoPath, "branch", "-vv")
	if err != nil {
		return BranchCleanResult{}, fmt.Errorf("listing branches: %w", err)
	}

	gone, worktreeGone := parseGoneBranches(output)

	var result BranchCleanResult

	for _, b := range worktreeGone {
		result.Skipped = append(result.Skipped, SkippedBranch{Name: b, Reason: SkipInWorktree})
	}

	if len(gone) == 0 {
		if len(worktreeGone) == 0 {
			emit(Event{Step: "gone-branches", Message: "No branches with gone upstream", Level: LevelInfo})
		}
		return result, nil
	}

	if opts.DryRun {
		result.Deleted = len(gone)
		emit(Event{Step: "gone-branches", Message: fmt.Sprintf("Would delete %d branch(es) with gone upstream", len(gone)), Level: LevelDetail})
		return result, nil
	}

	// Force-delete (-D) of unmerged branches is an aggressive-mode action (per the
	// --force help text). In safe mode keep -d even with --force, which here is
	// only the non-interactive destructive gate — otherwise a required gate flag
	// would silently discard unmerged work in "safe" mode.
	deleteFlag := "-d"
	if opts.Force && opts.Mode == ModeAggressive {
		deleteFlag = "-D"
	}

	for _, b := range gone {
		if _, err := runner.Run(repoPath, "branch", deleteFlag, b); err != nil {
			result.Skipped = append(result.Skipped, SkippedBranch{Name: b, Reason: SkipUnmerged})
		} else {
			result.Deleted++
		}
	}

	if result.Deleted > 0 {
		emit(Event{Step: "gone-branches", Message: fmt.Sprintf("Deleted %d branch(es) with gone upstream", result.Deleted), Level: LevelInfo})
	}
	if skipped := len(result.Skipped) - len(worktreeGone); skipped > 0 {
		emit(Event{Step: "gone-branches", Message: fmt.Sprintf("%d branch(es) skipped (not fully merged)", skipped), Level: LevelWarn})
	}

	return result, nil
}

func CleanNonWorktreeBranches(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) (BranchCleanResult, error) {
	emit(Event{Step: "non-wt-branches", Message: "Checking non-worktree branches...", Level: LevelStep})

	candidates, err := listNonWorktreeCandidates(runner, repoPath)
	if err != nil {
		return BranchCleanResult{}, err
	}

	var result BranchCleanResult

	if opts.Mode != ModeAggressive {
		result.Remaining = len(candidates)
		if len(candidates) > 0 {
			emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("%d branch(es) not in any worktree", len(candidates)), Level: LevelDetail})
		} else {
			emit(Event{Step: "non-wt-branches", Message: "No non-worktree branches", Level: LevelInfo})
		}
		return result, nil
	}

	if opts.DryRun {
		result.Deleted = len(candidates)
		emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("Would delete %d non-worktree branch(es)", len(candidates)), Level: LevelDetail})
		return result, nil
	}

	deleteFlag := "-d"
	if opts.Force {
		deleteFlag = "-D"
	}

	for _, b := range candidates {
		if _, err := runner.Run(repoPath, "branch", deleteFlag, b); err != nil {
			result.Skipped = append(result.Skipped, SkippedBranch{Name: b, Reason: SkipUnmerged})
			result.Remaining++
		} else {
			result.Deleted++
		}
	}

	if result.Deleted > 0 {
		emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("Deleted %d non-worktree branch(es)", result.Deleted), Level: LevelInfo})
	}
	if len(result.Skipped) > 0 {
		emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("%d branch(es) skipped (not fully merged — use --force)", len(result.Skipped)), Level: LevelWarn})
	}

	return result, nil
}

// listNonWorktreeCandidates returns local branches that are neither checked
// out in any worktree nor protected — the set aggressive cleanup deletes.
func listNonWorktreeCandidates(runner git.CommandRunner, repoPath string) ([]string, error) {
	wtOutput, err := runner.Run(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}
	wtBranches := parseWorktreeBranches(wtOutput)

	branchOutput, err := runner.Run(repoPath, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}

	var candidates []string
	for _, line := range strings.Split(branchOutput, "\n") {
		b := strings.TrimSpace(line)
		if b == "" {
			continue
		}
		if wtBranches[b] || git.IsProtectedBranch(b) {
			continue
		}
		candidates = append(candidates, b)
	}
	return candidates, nil
}

func parseGoneBranches(output string) (gone []string, worktreeGone []string) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, ": gone]") {
			continue
		}

		trimmed := strings.TrimLeft(line, " ")
		inWorktree := strings.HasPrefix(trimmed, "+ ")
		isCurrent := strings.HasPrefix(trimmed, "* ")
		if inWorktree {
			trimmed = strings.TrimPrefix(trimmed, "+ ")
		} else if isCurrent {
			trimmed = strings.TrimPrefix(trimmed, "* ")
		}

		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		branch := fields[0]

		if inWorktree || isCurrent {
			worktreeGone = append(worktreeGone, branch)
		} else {
			gone = append(gone, branch)
		}
	}
	return
}

func parseWorktreeBranches(output string) map[string]bool {
	branches := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "branch refs/heads/") {
			b := strings.TrimPrefix(line, "branch refs/heads/")
			branches[b] = true
		}
	}
	return branches
}
