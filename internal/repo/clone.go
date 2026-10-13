package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/fileutil"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

type CloneOptions struct {
	URL      string
	Location string
	Name     string
}

type CloneResult struct {
	RepoPath      string
	WorktreePath  string
	DefaultBranch string
	OriginURL     string
	Phases        []progress.Phase
	Err           error
}

const (
	cloneWorktreePhaseID progress.PhaseID = "clone:worktree"
	cloneRollbackPhaseID progress.PhaseID = "clone:rollback"
)

func DeriveRepoName(url string) string {
	url = strings.TrimSpace(url)
	if idx := strings.IndexAny(url, "?#"); idx != -1 {
		url = url[:idx]
	}
	url = strings.TrimRight(url, "/")
	if idx := strings.LastIndex(url, ":"); idx != -1 && !strings.Contains(url, "://") {
		url = url[idx+1:]
	}
	name := url
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}
	return strings.TrimSuffix(name, ".git")
}

func Clone(runner git.CommandRunner, opts CloneOptions, emit func(progress.Event)) CloneResult {
	clone, finish := prepareClone(runner, opts)
	return finish(clone.run(emit))
}

// prepareClone declares the clone flow and returns the function that turns
// its outcome into a CloneResult.
func prepareClone(runner git.CommandRunner, opts CloneOptions) (flow, func([]progress.Phase, error) CloneResult) {
	repoPath := filepath.Join(opts.Location, opts.Name)
	barePath := filepath.Join(repoPath, ".bare")
	branch := ""
	clone := flow{name: "repository clone"}
	clone.add("clone:validate", "Validate", step{id: "target", label: "Validate target", run: func() (string, error) {
		switch {
		case opts.Name == "":
			return "", errors.New("could not derive a repository name from the URL; pass --name")
		case opts.Name == "." || opts.Name == ".." || strings.ContainsAny(opts.Name, `/\`):
			return "", fmt.Errorf("invalid repository name %q: must be a directory name, not a path", opts.Name)
		}
		if _, err := os.Stat(repoPath); err == nil {
			return "", fmt.Errorf("target already exists: %s", repoPath)
		}
		return "", nil
	}})
	clone.add("clone:bare", "Clone", step{
		id: "bare-repository", label: "Clone bare repository",
		run: func() (string, error) {
			// --single-branch keeps the default branch as the only local head.
			// A plain bare clone copies every remote branch into refs/heads,
			// where the copies go stale and shadow origin/<branch>. The other
			// branches arrive as remote-tracking refs with the fetch below.
			_, err := runner.Run(opts.Location, "clone", "--bare", "--single-branch", "--", opts.URL, barePath)
			return "", err
		},
		undo: func() error { return fileutil.RemoveAllRetry(repoPath) },
	})
	clone.add("clone:structure", "Structure", step{id: "git-pointer", label: "Create .git pointer", run: func() (string, error) {
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			return "", err
		}
		return "", os.WriteFile(filepath.Join(repoPath, ".git"), []byte("gitdir: .bare\n"), 0644)
	}})
	clone.add("clone:structure", "Structure", step{id: "refspec", label: "Configure refspec", run: func() (string, error) {
		_, err := runner.Run(barePath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
		return "", err
	}})
	clone.add(cloneWorktreePhaseID, "Worktree", step{id: "default-branch", label: "Detect default branch", run: func() (string, error) {
		branch = git.DetectDefaultBranch(runner, barePath)
		if !git.BranchExists(runner, barePath, branch) {
			// A remote whose HEAD is detached names no branch, so the
			// single-branch clone has no local head to detect. Take the
			// remote's heads, as a plain bare clone does, and look again.
			if _, err := runner.Run(barePath, "fetch", "origin", "+refs/heads/*:refs/heads/*"); err == nil {
				branch = git.DetectDefaultBranch(runner, barePath)
			}
		}
		return branch, nil
	}})
	clone.add(cloneWorktreePhaseID, "Worktree", step{id: "checkout", label: "Create worktree", run: func() (string, error) {
		if !git.BranchExists(runner, barePath, branch) {
			return "", fmt.Errorf("remote has no commits on %q yet (nothing to check out)", branch)
		}
		_, err := runner.Run(repoPath, "worktree", "add", git.WorktreePath(repoPath, branch), branch)
		return "", err
	}})
	clone.add(cloneWorktreePhaseID, "Worktree", step{
		id: "tracking", label: "Set upstream tracking", optional: true,
		run: func() (string, error) {
			if _, err := runner.Run(barePath, "fetch", "origin"); err != nil {
				return "", fmt.Errorf("no tracking: %w", err)
			}
			if _, err := runner.Run(git.WorktreePath(repoPath, branch), "branch", fmt.Sprintf("--set-upstream-to=origin/%s", branch)); err != nil {
				return "", fmt.Errorf("no tracking: %w", err)
			}
			return "", nil
		},
	})
	clone.addRollback(cloneRollbackPhaseID, "remove-partial-checkout", "Remove partial checkout")

	return clone, func(phases []progress.Phase, err error) CloneResult {
		result := CloneResult{OriginURL: opts.URL, RepoPath: repoPath, Phases: phases, Err: err}
		if stepDone(phases, cloneWorktreePhaseID, "default-branch") {
			result.DefaultBranch = branch
		}
		if stepDone(phases, cloneWorktreePhaseID, "checkout") {
			result.WorktreePath = git.WorktreePath(repoPath, branch)
		}
		return result
	}
}
