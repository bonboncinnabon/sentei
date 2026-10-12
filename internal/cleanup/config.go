package cleanup

import (
	"fmt"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// PurgeOrphanedBranchConfigs removes the branch.<name> sections of the
// repository's config whose branch no longer exists. git does the writing, so
// config.lock is honoured and the rest of the file is left byte for byte.
func PurgeOrphanedBranchConfigs(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) (ConfigResult, error) {
	emit(Event{Step: "orphaned-configs", Message: "Removing config sections for deleted branches...", Level: LevelStep})

	orphans, err := orphanedBranchSections(runner, repoPath)
	if err != nil {
		return ConfigResult{}, err
	}

	if len(orphans) == 0 {
		emit(Event{Step: "orphaned-configs", Message: "No orphaned branch config sections", Level: LevelInfo})
		return ConfigResult{}, nil
	}

	if opts.DryRun {
		emit(Event{Step: "orphaned-configs", Message: fmt.Sprintf("Would remove %d orphaned branch config section(s)", len(orphans)), Level: LevelDetail})
		return ConfigResult{Removed: len(orphans)}, nil
	}

	var result ConfigResult
	for _, name := range orphans {
		if _, err := runner.Run(repoPath, "config", "--local", "--remove-section", "branch."+name); err != nil {
			return result, fmt.Errorf("removing config section of branch %s: %w", name, err)
		}
		result.Removed++
	}

	emit(Event{Step: "orphaned-configs", Message: fmt.Sprintf("Removed %d orphaned config section(s)", result.Removed), Level: LevelInfo})
	return result, nil
}

// orphanedBranchSections returns the names that have a branch.<name> section
// in the repository's own config file but no local branch.
func orphanedBranchSections(runner git.CommandRunner, repoPath string) ([]string, error) {
	// lstrip=2, not short: `short` abbreviates to "heads/<name>" when a tag
	// shares the branch's name, and the live branch's section would look orphaned.
	branchOutput, err := runner.Run(repoPath, "branch", "--format=%(refname:lstrip=2)")
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}
	existing := make(map[string]bool)
	for _, line := range strings.Split(branchOutput, "\n") {
		if b := strings.TrimSpace(line); b != "" {
			existing[b] = true
		}
	}

	keys, err := runner.Run(repoPath, "config", "--local", "--name-only", "--list")
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var orphans []string
	seen := make(map[string]bool)
	for _, key := range strings.Split(keys, "\n") {
		// A key reads branch.<name>.<variable>. The name may contain dots, the
		// variable cannot, so the name ends at the last dot.
		rest, ok := strings.CutPrefix(key, "branch.")
		if !ok {
			continue
		}
		end := strings.LastIndex(rest, ".")
		if end < 0 {
			continue
		}
		name := rest[:end]
		if existing[name] || seen[name] {
			continue
		}
		seen[name] = true
		orphans = append(orphans, name)
	}
	return orphans, nil
}
