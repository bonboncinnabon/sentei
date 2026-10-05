package worktreefile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/fileutil"
)

const (
	destinationDirectoryMode os.FileMode = 0o755
	temporaryFilePattern                 = ".sentei-copy-*"
)

// Outcome reports destination paths without exposing copied file contents.
type Outcome struct {
	Copied  []string
	Skipped []string
}

// Message describes copied and preserved destinations without file contents.
func (o Outcome) Message() string {
	var parts []string
	if len(o.Copied) > 0 {
		parts = append(parts, "copied: "+strings.Join(o.Copied, ", "))
	}
	if len(o.Skipped) > 0 {
		parts = append(parts, "preserved existing: "+strings.Join(o.Skipped, ", "))
	}
	return strings.Join(parts, "; ")
}

// Copy applies rules from a repository container to one new worktree.
func Copy(repoRoot, worktreeRoot string, rules []Rule) (Outcome, error) {
	if err := ValidateRules(rules); err != nil {
		return Outcome{}, err
	}
	sourceRoot, err := resolveRoot(repoRoot, "repository")
	if err != nil {
		return Outcome{}, err
	}
	destinationRoot, err := resolveRoot(worktreeRoot, "worktree")
	if err != nil {
		return Outcome{}, err
	}

	var outcome Outcome
	for _, rule := range rules {
		copied, err := copyRule(sourceRoot, destinationRoot, rule)
		if err != nil {
			return outcome, err
		}
		if copied {
			outcome.Copied = append(outcome.Copied, rule.Destination)
		} else {
			outcome.Skipped = append(outcome.Skipped, rule.Destination)
		}
	}
	return outcome, nil
}

func resolveRoot(root, label string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving %s root: %w", label, err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolving %s root: %w", label, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspecting %s root: %w", label, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s root is not a directory", label)
	}
	return resolved, nil
}

func copyRule(sourceRoot, destinationRoot string, rule Rule) (bool, error) {
	sourcePath, sourceMode, err := resolveSource(sourceRoot, rule.Source)
	if err != nil {
		return false, err
	}
	destinationPath, exists, err := prepareDestination(destinationRoot, rule.Destination)
	if err != nil {
		return false, err
	}
	if exists && !rule.Overwrite {
		return false, nil
	}
	if err := copyAtomically(sourcePath, destinationPath, sourceMode); err != nil {
		return false, fmt.Errorf("copying worktree file to %q: %w", rule.Destination, err)
	}
	return true, nil
}

func resolveSource(root, configuredPath string) (string, os.FileMode, error) {
	candidate := filepath.Join(root, portablePath(configuredPath))
	resolved, err := filepath.EvalSymlinks(candidate)
	if errors.Is(err, os.ErrNotExist) {
		return "", 0, fmt.Errorf("worktree file source %q does not exist", configuredPath)
	}
	if err != nil {
		return "", 0, fmt.Errorf("resolving worktree file source %q: %w", configuredPath, err)
	}
	if !isWithin(root, resolved) {
		return "", 0, fmt.Errorf("worktree file source %q resolves outside repository root", configuredPath)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", 0, fmt.Errorf("inspecting worktree file source %q: %w", configuredPath, err)
	}
	if !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("worktree file source %q is not a regular file", configuredPath)
	}
	return resolved, info.Mode().Perm(), nil
}

func prepareDestination(root, configuredPath string) (string, bool, error) {
	relative := portablePath(configuredPath)
	destination := filepath.Join(root, relative)
	if !isWithin(root, destination) {
		return "", false, fmt.Errorf("worktree file destination %q resolves outside worktree root", configuredPath)
	}
	if err := ensureSafeParents(root, filepath.Dir(relative), configuredPath); err != nil {
		return "", false, err
	}
	info, err := os.Lstat(destination)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return destination, false, nil
	case err != nil:
		return "", false, fmt.Errorf("inspecting worktree file destination %q: %w", configuredPath, err)
	case info.Mode()&os.ModeSymlink != 0:
		return "", false, fmt.Errorf("worktree file destination %q is a symlink", configuredPath)
	case !info.Mode().IsRegular():
		return "", false, fmt.Errorf("worktree file destination %q is not a regular file", configuredPath)
	default:
		return destination, true, nil
	}
}

func ensureSafeParents(root, relativeParent, configuredPath string) error {
	if relativeParent == "." {
		return nil
	}
	current := root
	for _, component := range strings.Split(relativeParent, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if err := os.Mkdir(current, destinationDirectoryMode); err != nil {
				return fmt.Errorf("creating parent for worktree file destination %q: %w", configuredPath, err)
			}
		case err != nil:
			return fmt.Errorf("inspecting parent for worktree file destination %q: %w", configuredPath, err)
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("worktree file destination %q contains symlink parent %q", configuredPath, component)
		case !info.IsDir():
			return fmt.Errorf("worktree file destination %q parent %q is not a directory", configuredPath, component)
		}
	}
	return nil
}

func copyAtomically(source, destination string, mode os.FileMode) (copyErr error) {
	temp, err := os.CreateTemp(filepath.Dir(destination), temporaryFilePattern)
	if err != nil {
		return fmt.Errorf("creating temporary destination: %w", err)
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("closing temporary destination: %w", err)
	}
	defer func() {
		err := os.Remove(tempPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			copyErr = errors.Join(copyErr, fmt.Errorf("removing temporary destination: %w", err))
		}
	}()
	if err := fileutil.CopyFile(source, tempPath); err != nil {
		return fmt.Errorf("writing temporary destination: %w", err)
	}
	if err := os.Chmod(tempPath, mode.Perm()); err != nil {
		return fmt.Errorf("setting destination permissions: %w", err)
	}
	if err := os.Rename(tempPath, destination); err != nil {
		return fmt.Errorf("replacing destination: %w", err)
	}
	return nil
}

func portablePath(configuredPath string) string {
	return filepath.FromSlash(strings.ReplaceAll(configuredPath, "\\", "/"))
}

func isWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
