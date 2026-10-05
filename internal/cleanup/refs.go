package cleanup

import (
	"fmt"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func PruneRemoteRefs(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) (int, error) {
	emit(Event{Step: "prune-refs", Message: "Pruning stale remote refs...", Level: LevelStep})

	remoteOutput, err := runner.Run(repoPath, "remote")
	if err != nil {
		return 0, fmt.Errorf("listing remotes: %w", err)
	}
	if !hasRemote(remoteOutput, "origin") {
		emit(Event{Step: "prune-refs", Message: "No origin remote; skipping", Level: LevelInfo})
		return 0, nil
	}

	output, err := runner.Run(repoPath, "remote", "prune", "origin", "--dry-run")
	if err != nil {
		return 0, fmt.Errorf("checking stale refs: %w", err)
	}

	count := strings.Count(output, "[would prune]")

	if count == 0 {
		emit(Event{Step: "prune-refs", Message: "No stale remote refs", Level: LevelInfo})
		return 0, nil
	}

	if opts.DryRun {
		emit(Event{Step: "prune-refs", Message: fmt.Sprintf("Would prune %d stale remote ref(s)", count), Level: LevelDetail})
		return count, nil
	}

	if _, err := runner.Run(repoPath, "fetch", "--prune", "origin"); err != nil {
		return 0, fmt.Errorf("pruning remote refs: %w", err)
	}

	emit(Event{Step: "prune-refs", Message: fmt.Sprintf("Pruned %d stale remote ref(s)", count), Level: LevelInfo})
	return count, nil
}

func hasRemote(remoteOutput, name string) bool {
	for _, line := range strings.Split(remoteOutput, "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}
