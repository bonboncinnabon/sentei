package worktree

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func ParseStatusPorcelain(output string) (hasUncommitted bool, hasUntracked bool) {
	output = strings.TrimSpace(output)
	if output == "" {
		return false, false
	}

	for _, line := range strings.Split(output, "\n") {
		if len(line) < 2 {
			continue
		}
		if strings.HasPrefix(line, "??") {
			hasUntracked = true
		} else {
			hasUncommitted = true
		}
	}
	return hasUncommitted, hasUntracked
}

func ParseCommitDate(output string) (time.Time, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02 15:04:05 -0700", output)
}

func enrichWorktree(runner git.CommandRunner, wt *git.Worktree, repoHasRemotes bool) {
	if wt.IsBare || wt.IsPrunable {
		return
	}

	dateOutput, err := runner.Run(wt.Path, "log", "-1", "--format=%ai")
	if err != nil {
		wt.EnrichmentError = err.Error()
		return
	}

	commitDate, err := ParseCommitDate(dateOutput)
	if err != nil {
		wt.EnrichmentError = err.Error()
		return
	}
	wt.LastCommitDate = commitDate

	subject, err := runner.Run(wt.Path, "log", "-1", "--format=%s")
	if err != nil {
		wt.EnrichmentError = err.Error()
		return
	}
	wt.LastCommitSubject = strings.TrimSpace(subject)

	statusOutput, err := runner.Run(wt.Path, "status", "--porcelain")
	if err != nil {
		wt.EnrichmentError = err.Error()
		return
	}
	wt.HasUncommittedChanges, wt.HasUntrackedFiles = ParseStatusPorcelain(statusOutput)

	// In a repo with remotes, "unpushed" means HEAD has commits that no
	// remote-tracking branch contains. Counting against every remote ref (not
	// just @{upstream}) keeps a branch whose upstream was deleted after merge
	// from being false-flagged: its commits still live in the default branch
	// on the remote. Without any remote, "unpushed" is meaningless.
	if repoHasRemotes {
		if countOutput, err := runner.Run(wt.Path, "rev-list", "--count", "HEAD", "--not", "--remotes"); err != nil {
			wt.HasUnpushedCommits = true
		} else {
			wt.HasUnpushedCommits = ParseAheadCount(countOutput) > 0
		}
	}

	wt.IsEnriched = true
}

// ParseAheadCount parses `git rev-list --count` output; unparsable output
// counts as ahead so the safety gate errs toward caution.
func ParseAheadCount(output string) int {
	n, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil {
		return 1
	}
	return n
}

// DefaultEnrichConcurrency is the default parallelism for worktree enrichment.
const DefaultEnrichConcurrency = 10

func EnrichWorktrees(runner git.CommandRunner, worktrees []git.Worktree, maxConcurrency int) []git.Worktree {
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}

	repoHasRemotes := false
	for i := range worktrees {
		if worktrees[i].IsBare || worktrees[i].IsPrunable {
			continue
		}
		out, err := runner.Run(worktrees[i].Path, "remote")
		repoHasRemotes = err == nil && strings.TrimSpace(out) != ""
		break
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for i := range worktrees {
		if worktrees[i].IsBare || worktrees[i].IsPrunable {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			enrichWorktree(runner, &worktrees[idx], repoHasRemotes)
		}(i)
	}

	wg.Wait()
	return worktrees
}
