package git

import "strings"

var protectedBranches = map[string]bool{
	"main":    true,
	"master":  true,
	"develop": true,
	"dev":     true,
}

// IsProtectedBranch reports whether a branch matches the built-in protected
// convention set. Matching is case-insensitive so "Main"/"MASTER" are caught.
// It does not know the repo's actual default branch; prefer IsProtectedBranchWith
// when that is available.
func IsProtectedBranch(branch string) bool {
	name := strings.ToLower(strings.TrimPrefix(branch, "refs/heads/"))
	return protectedBranches[name]
}

// IsProtectedBranchWith additionally protects the repo's actual default branch,
// which may be non-standard (e.g. "production", "trunk", "mainline"). A bare
// detected name (no refs/heads/ prefix) is expected for defaultBranch.
func IsProtectedBranchWith(branch, defaultBranch string) bool {
	if defaultBranch != "" {
		name := strings.TrimPrefix(branch, "refs/heads/")
		if strings.EqualFold(name, defaultBranch) {
			return true
		}
	}
	return IsProtectedBranch(branch)
}

// DetectDefaultBranch returns the repository's default branch: the branch HEAD
// of the shared git directory points at. repoPath may be the bare directory,
// the layout's root or any worktree. It is the single source of truth for
// default-branch detection across clone, remove, cleanup and protection.
func DetectDefaultBranch(runner CommandRunner, repoPath string) string {
	// In a worktree HEAD names that worktree's own branch, so read the shared
	// directory's. A bare clone records the remote's default branch there, and
	// that survives a non-standard default; refs/remotes/origin/HEAD is not
	// created by a bare clone.
	if commonDir, err := CommonDir(runner, repoPath); err == nil {
		repoPath = commonDir
	}
	// The full ref, not --short: when a tag shares the branch's name the short
	// form is "heads/<name>", which names no branch and protects nothing.
	if ref, err := runner.Run(repoPath, "symbolic-ref", "HEAD"); err == nil {
		if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok && branch != "" {
			return branch
		}
	}

	// Fallback: try main, then master.
	for _, candidate := range []string{"main", "master"} {
		if BranchExists(runner, repoPath, candidate) {
			return candidate
		}
	}

	return "main" // last resort
}
