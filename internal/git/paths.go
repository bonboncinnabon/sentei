package git

import (
	"fmt"
	"os"
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

// RepoRoot resolves the directory that stands for the whole repository holding
// repoPath: new worktrees are created in it and .sentei.yaml is read from it.
// One rule decides both, so they cannot disagree.
//
// For a common dir named .git or .bare the root is its parent: the top of a
// regular working tree, or the container of a bare layout. A common dir under
// any other name is a plain bare repository and is its own root, reached from
// itself or from any of its worktrees, because its parent may hold unrelated
// repositories. The exception is a parent whose .git pointer file leads to
// that same common dir: that parent is a container too. Outside a repository
// the root is repoPath.
//
// Callers also run git in the root. In the container of a .bare directory
// that works only when the container has its .git pointer file.
func RepoRoot(runner CommandRunner, repoPath string) string {
	commonDir, err := CommonDir(runner, repoPath)
	if err != nil {
		return repoPath
	}
	parent := filepath.Dir(commonDir)
	switch filepath.Base(commonDir) {
	case ".git", ".bare":
		return parent
	}
	// The pointer is followed, not trusted: one that leads to another
	// repository would send every command run at the root to that repository.
	if pointer, err := os.Lstat(filepath.Join(parent, ".git")); err == nil && pointer.Mode().IsRegular() {
		if pointed, err := CommonDir(runner, parent); err == nil && sameDir(pointed, commonDir) {
			return parent
		}
	}
	return commonDir
}

// sameDir reports whether two paths name the same existing directory, however
// each is spelled (symlinks, letter case on a case-insensitive volume).
func sameDir(a, b string) bool {
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(infoA, infoB)
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
