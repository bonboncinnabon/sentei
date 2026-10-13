package git

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CommonDir resolves git's common directory (returned absolute) for any path
// inside a repo: a worktree, the repo root, or the git dir itself. In sentei's
// bare layout this is the bare repo dir; in a regular
// repo it is the .git dir. Asking git instead of assuming the ".bare" naming
// convention lets layouts with a differently named bare dir (e.g. a
// playground's repo.git) resolve correctly.
func CommonDir(runner CommandRunner, repoPath string) (string, error) {
	out, err := runner.Run(repoPath, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("resolve git common dir for %q: %w", repoPath, err)
	}
	commonDir := strings.TrimSpace(out)
	if !filepath.IsAbs(commonDir) {
		return filepath.Join(repoPath, commonDir), nil
	}
	return filepath.Clean(commonDir), nil
}

// PathContains reports whether dir is root itself or lies inside it. Symlinks
// are resolved first so two spellings of the same place compare equal (on
// macOS os.Getwd reports /private/tmp for a directory reached as /tmp).
func PathContains(root, dir string) bool {
	if root == "" || dir == "" {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	rel, err := filepath.Rel(root, dir)
	return err == nil && filepath.IsLocal(rel)
}

// WorktreeDirName returns the directory name sentei uses for a branch's
// worktree. Slashes are flattened ("feature/auth" -> "feature-auth") so every
// worktree sits directly under the repo root instead of nesting.
func WorktreeDirName(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

// WorktreePath returns the path of the worktree directory for a branch.
func WorktreePath(repoPath, branch string) string {
	return filepath.Join(repoPath, WorktreeDirName(branch))
}
