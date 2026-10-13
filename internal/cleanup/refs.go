package cleanup

import (
	"fmt"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// PruneRemoteRefs removes origin's remote-tracking refs whose branch no longer
// exists on the remote. It returns the names of those refs relative to
// refs/remotes/ (for example "origin/feature/x"). A dry run returns the same
// names and leaves the refs in place.
func PruneRemoteRefs(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) ([]string, error) {
	emit(Event{Step: "prune-refs", Message: "Pruning stale remote refs...", Level: LevelStep})

	remoteOutput, err := runner.Run(repoPath, "remote")
	if err != nil {
		return nil, fmt.Errorf("listing remotes: %w", err)
	}
	if !hasRemote(remoteOutput, "origin") {
		emit(Event{Step: "prune-refs", Message: "No origin remote; skipping", Level: LevelInfo})
		return nil, nil
	}

	output, err := runner.Run(repoPath, "remote", "prune", "origin", "--dry-run")
	if err != nil {
		return nil, fmt.Errorf("checking stale refs: %w", err)
	}

	stale := parseStaleRefs(output)

	if len(stale) == 0 {
		emit(Event{Step: "prune-refs", Message: "No stale remote refs", Level: LevelInfo})
		return nil, nil
	}

	if opts.DryRun {
		emit(Event{Step: "prune-refs", Message: fmt.Sprintf("Would prune %d stale remote ref(s)", len(stale)), Level: LevelDetail})
		return stale, nil
	}

	if _, err := runner.Run(repoPath, "fetch", "--prune", "origin"); err != nil {
		return nil, fmt.Errorf("pruning remote refs: %w", err)
	}

	emit(Event{Step: "prune-refs", Message: fmt.Sprintf("Pruned %d stale remote ref(s)", len(stale)), Level: LevelInfo})
	return stale, nil
}

// parseStaleRefs reads the ref names out of `git remote prune --dry-run`,
// which prints one " * [would prune] origin/<branch>" line per stale ref. The
// bracketed label is translated, so a line is recognised by its bullet and
// the name is its last field (ref names contain no whitespace).
func parseStaleRefs(output string) []string {
	var stale []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "*" {
			stale = append(stale, fields[len(fields)-1])
		}
	}
	return stale
}

func hasRemote(remoteOutput, name string) bool {
	for _, line := range strings.Split(remoteOutput, "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}
