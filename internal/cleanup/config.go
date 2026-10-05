package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func DedupConfig(configPath string, opts Options, emit func(Event)) (ConfigResult, error) {
	emit(Event{Step: "dedup-config", Message: "Deduplicating git config...", Level: LevelStep})

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ConfigResult{}, fmt.Errorf("reading config: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	before := len(lines)

	var out []string
	seen := make(map[string]bool)

	for _, line := range lines {
		if strings.HasPrefix(line, "[") {
			seen = make(map[string]bool)
			out = append(out, line)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, line)
			continue
		}

		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, line)
	}

	after := len(out)
	removed := before - after
	result := ConfigResult{Before: before, After: after, Removed: removed}

	if removed == 0 {
		emit(Event{Step: "dedup-config", Message: fmt.Sprintf("Config already clean (%d lines)", before), Level: LevelInfo})
		return result, nil
	}

	if opts.DryRun {
		emit(Event{Step: "dedup-config", Message: fmt.Sprintf("Would remove %d duplicate lines (%d → %d)", removed, before, after), Level: LevelDetail})
		return result, nil
	}

	if err := atomicWriteConfig(configPath, strings.Join(out, "\n")); err != nil {
		return result, fmt.Errorf("writing deduped config: %w", err)
	}

	emit(Event{Step: "dedup-config", Message: fmt.Sprintf("Deduplicated config: removed %d lines (%d → %d)", removed, before, after), Level: LevelInfo})
	return result, nil
}

func PurgeOrphanedBranchConfigs(runner git.CommandRunner, repoPath string, configPath string, opts Options, emit func(Event)) (ConfigResult, error) {
	emit(Event{Step: "orphaned-configs", Message: "Removing config sections for deleted branches...", Level: LevelStep})

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ConfigResult{}, fmt.Errorf("reading config: %w", err)
	}

	branchOutput, err := runner.Run(repoPath, "branch", "--format=%(refname:short)")
	if err != nil {
		return ConfigResult{}, fmt.Errorf("listing branches: %w", err)
	}

	existing := make(map[string]bool)
	for _, line := range strings.Split(branchOutput, "\n") {
		b := strings.TrimSpace(line)
		if b != "" {
			existing[b] = true
		}
	}

	lines := strings.Split(string(data), "\n")
	before := len(lines)
	var out []string
	skip := false
	orphanCount := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "[branch \"") {
			branchName := line
			branchName = strings.TrimPrefix(branchName, "[branch \"")
			branchName = strings.TrimSuffix(branchName, "\"]")
			if !existing[branchName] {
				skip = true
				orphanCount++
				continue
			}
			skip = false
			out = append(out, line)
			continue
		}

		if strings.HasPrefix(line, "[") {
			skip = false
			out = append(out, line)
			continue
		}

		if !skip {
			out = append(out, line)
		}
	}

	after := len(out)
	result := ConfigResult{Before: before, After: after, Removed: orphanCount}

	if orphanCount == 0 {
		emit(Event{Step: "orphaned-configs", Message: "No orphaned branch config sections", Level: LevelInfo})
		return result, nil
	}

	if opts.DryRun {
		emit(Event{Step: "orphaned-configs", Message: fmt.Sprintf("Would remove %d orphaned branch config section(s)", orphanCount), Level: LevelDetail})
		return result, nil
	}

	if err := atomicWriteConfig(configPath, strings.Join(out, "\n")); err != nil {
		return result, fmt.Errorf("writing purged config: %w", err)
	}

	emit(Event{Step: "orphaned-configs", Message: fmt.Sprintf("Removed %d orphaned config sections (%d → %d lines)", orphanCount, before, after), Level: LevelInfo})
	return result, nil
}

func atomicWriteConfig(configPath string, content string) error {
	bakPath := configPath + ".bak"

	info, err := os.Stat(configPath)
	var perm os.FileMode = 0644
	if err == nil {
		perm = info.Mode().Perm()
	}

	if _, statErr := os.Stat(bakPath); os.IsNotExist(statErr) {
		original, err := os.ReadFile(configPath)
		if err == nil {
			if err := os.WriteFile(bakPath, original, perm); err != nil {
				return fmt.Errorf("creating backup: %w", err)
			}
		}
	}

	dir := filepath.Dir(configPath)
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, perm); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
