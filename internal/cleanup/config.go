package cleanup

import (
	"fmt"
	"slices"
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
	branches, err := localBranches(runner, repoPath)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]bool, len(branches))
	for _, b := range branches {
		existing[b] = true
	}

	keys, err := runner.Run(repoPath, "config", "--local", "--name-only", "--list")
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var orphans []string
	seen := make(map[string]bool)
	for _, key := range strings.Split(keys, "\n") {
		name, ok := branchOfKey(key)
		if !ok || existing[name] || seen[name] {
			continue
		}
		seen[name] = true
		orphans = append(orphans, name)
	}
	return orphans, nil
}

// branchOfKey returns the branch whose section holds a config key. Such a key
// reads branch.<name>.<variable>. The name may contain dots, the variable
// cannot, so the name ends at the last dot.
func branchOfKey(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, "branch.")
	if !ok {
		return "", false
	}
	end := strings.LastIndex(rest, ".")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

// DedupConfig removes the repeated lines of the repository's config: a key
// whose lines all hold the same value keeps one of them. A key whose values
// differ is left alone, because git reads the last value of a single-valued
// key and every value of a multi-valued one. git does the writing, so
// config.lock is honoured.
//
// deleted names the branches this run deletes. Their sections go with them,
// like the orphaned ones, so the lines in both are not counted: a dry run
// still has them in the file.
func DedupConfig(runner git.CommandRunner, repoPath string, deleted []string, opts Options, emit func(Event)) (ConfigResult, error) {
	emit(Event{Step: "dedup-config", Message: "Removing duplicate config lines...", Level: LevelStep})

	repeats, err := repeatedConfigValues(runner, repoPath, deleted)
	if err != nil {
		return ConfigResult{}, err
	}

	duplicates := 0
	for _, repeat := range repeats {
		duplicates += repeat.extra
	}
	if duplicates == 0 {
		emit(Event{Step: "dedup-config", Message: "No duplicate config lines", Level: LevelInfo})
		return ConfigResult{}, nil
	}

	if opts.DryRun {
		emit(Event{Step: "dedup-config", Message: fmt.Sprintf("Would remove %d duplicate config line(s)", duplicates), Level: LevelDetail})
		return ConfigResult{Removed: duplicates}, nil
	}

	// ponytail: one git process per repeated key, because `git config` writes
	// one key per call. Each call takes config.lock and rewrites the file.
	var result ConfigResult
	for _, repeat := range repeats {
		// The value is also the pattern of lines to replace, so a different
		// value another process wrote since the read is kept.
		if _, err := runner.Run(repoPath, "config", "--local", "--fixed-value", "--replace-all", repeat.key, repeat.value, repeat.value); err != nil {
			return result, fmt.Errorf("removing duplicate lines of %s: %w", repeat.key, err)
		}
		result.Removed += repeat.extra
	}

	emit(Event{Step: "dedup-config", Message: fmt.Sprintf("Removed %d duplicate config line(s)", result.Removed), Level: LevelInfo})
	return result, nil
}

// configRepeat is a key that holds one value on more than one line.
type configRepeat struct {
	key, value string
	extra      int // the lines beyond the one that stays
}

// repeatedConfigValues returns the keys of the repository's own config file
// that are safe to collapse, in file order. It leaves out the sections of
// branches that do not exist or are named in deleted.
func repeatedConfigValues(runner git.CommandRunner, repoPath string, deleted []string) ([]configRepeat, error) {
	branches, err := localBranches(runner, repoPath)
	if err != nil {
		return nil, err
	}

	// --null ends every entry with a NUL and separates key from value with a
	// newline, so a value is read exactly, whatever it contains: the closing
	// NUL keeps the runner from trimming the last one. A key written without
	// "=" has no newline and no value to write back.
	output, err := runner.Run(repoPath, "config", "--local", "--null", "--list")
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	type lines struct {
		value     string
		count     int
		identical bool
	}
	byKey := make(map[string]*lines)
	var keys []string
	for _, entry := range strings.Split(output, "\x00") {
		if entry == "" {
			continue
		}
		key, value, hasValue := strings.Cut(entry, "\n")
		found, ok := byKey[key]
		if !ok {
			found = &lines{value: value, identical: hasValue}
			byKey[key] = found
			keys = append(keys, key)
		}
		found.count++
		found.identical = found.identical && hasValue && value == found.value
	}

	var repeats []configRepeat
	for _, key := range keys {
		found := byKey[key]
		if found.count < 2 || !found.identical {
			continue
		}
		if branch, ok := branchOfKey(key); ok && (!slices.Contains(branches, branch) || slices.Contains(deleted, branch)) {
			continue
		}
		repeats = append(repeats, configRepeat{key: key, value: found.value, extra: found.count - 1})
	}
	return repeats, nil
}
