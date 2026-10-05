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

const cloneRollbackPhaseID progress.PhaseID = "clone:rollback"

type cloneOperationKind uint8

const (
	cloneRegular cloneOperationKind = iota
	cloneValidation
	cloneBare
	cloneDetect
	cloneWorktree
	cloneTracking
	cloneRollback
)

type cloneOperation struct {
	phaseID progress.PhaseID
	stepID  progress.StepID
	label   string
	kind    cloneOperationKind
	run     progress.StepFunc
}

type preparedClone struct {
	result     CloneResult
	plan       progress.Plan
	operations []cloneOperation
	branch     *string
}

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
	return prepareClone(runner, opts).run(emit)
}

func prepareClone(runner git.CommandRunner, opts CloneOptions) preparedClone {
	repoPath := filepath.Join(opts.Location, opts.Name)
	barePath := filepath.Join(repoPath, ".bare")
	branch := ""
	prepared := preparedClone{result: CloneResult{OriginURL: opts.URL, RepoPath: repoPath}, branch: &branch}
	add := func(phaseID, phaseLabel, stepID, label string, kind cloneOperationKind, run progress.StepFunc) {
		if len(prepared.plan.Phases) == 0 || prepared.plan.Phases[len(prepared.plan.Phases)-1].ID != phaseID {
			prepared.plan.Phases = append(prepared.plan.Phases, progress.PlannedPhase{ID: phaseID, Label: phaseLabel})
		}
		phase := &prepared.plan.Phases[len(prepared.plan.Phases)-1]
		phase.Steps = append(phase.Steps, progress.PlannedStep{ID: stepID, Label: label})
		prepared.operations = append(prepared.operations, cloneOperation{phaseID: phaseID, stepID: stepID, label: label, kind: kind, run: run})
	}
	add("clone:validate", "Validate", "target", "Validate target", cloneValidation, func() (string, error) {
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
	})
	add("clone:bare", "Clone", "bare-repository", "Clone bare repository", cloneBare, func() (string, error) {
		_, err := runner.Run(opts.Location, "clone", "--bare", opts.URL, barePath)
		return "", err
	})
	add("clone:structure", "Structure", "git-pointer", "Create .git pointer", cloneRegular, func() (string, error) {
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			return "", err
		}
		return "", os.WriteFile(filepath.Join(repoPath, ".git"), []byte("gitdir: .bare\n"), 0644)
	})
	add("clone:structure", "Structure", "refspec", "Configure refspec", cloneRegular, func() (string, error) {
		_, err := runner.Run(barePath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
		return "", err
	})
	add("clone:worktree", "Worktree", "default-branch", "Detect default branch", cloneDetect, func() (string, error) {
		branch = git.DetectDefaultBranch(runner, barePath)
		return branch, nil
	})
	add("clone:worktree", "Worktree", "checkout", "Create worktree", cloneWorktree, func() (string, error) {
		if !git.BranchExists(runner, barePath, branch) {
			return "", fmt.Errorf("remote has no commits on %q yet (nothing to check out)", branch)
		}
		_, err := runner.Run(repoPath, "worktree", "add", git.WorktreePath(repoPath, branch), branch)
		return "", err
	})
	add("clone:worktree", "Worktree", "tracking", "Set upstream tracking", cloneTracking, func() (string, error) {
		if _, err := runner.Run(barePath, "fetch", "origin"); err != nil {
			return "", err
		}
		_, err := runner.Run(git.WorktreePath(repoPath, branch), "branch", fmt.Sprintf("--set-upstream-to=origin/%s", branch))
		return "", err
	})
	add(cloneRollbackPhaseID, "Rollback", "remove-partial-checkout", "Remove partial checkout", cloneRollback, func() (string, error) {
		return "", fileutil.RemoveAllRetry(repoPath)
	})
	return prepared
}

func (p preparedClone) run(emit func(progress.Event)) CloneResult {
	result := p.result
	execution, err := progress.Start(p.plan, emit)
	if err != nil {
		result.Err = fmt.Errorf("starting repository clone: %w", err)
		return result
	}
	failedBy := ""
	touched := false
	usable := false
	for _, operation := range p.operations {
		if operation.kind == cloneRollback {
			if !touched || usable {
				_, err = execution.Skip(operation.phaseID, operation.stepID, "rollback not required")
			} else {
				_, err = execution.Run(operation.phaseID, operation.stepID, operation.run)
			}
			if err != nil {
				result.Err = errors.Join(result.Err, fmt.Errorf("executing rollback: %w", err))
			}
			continue
		}
		if failedBy != "" {
			_, err = execution.Skip(operation.phaseID, operation.stepID, "blocked by "+failedBy)
			if err != nil {
				result.Err = errors.Join(result.Err, err)
			}
			continue
		}
		if operation.kind == cloneBare {
			touched = true
		}
		if operation.kind == cloneTracking {
			if err = execution.Running(operation.phaseID, operation.stepID, 0, ""); err == nil {
				_, runErr := operation.run()
				if runErr != nil {
					_, err = execution.Skip(operation.phaseID, operation.stepID, "no tracking: "+runErr.Error())
				} else {
					_, err = execution.Done(operation.phaseID, operation.stepID, "")
				}
			}
		} else {
			var step progress.StepResult
			step, err = execution.Run(operation.phaseID, operation.stepID, operation.run)
			if err == nil && step.Status == progress.StepFailed {
				failedBy = operation.label
			}
		}
		if err != nil {
			result.Err = errors.Join(result.Err, fmt.Errorf("executing %s: %w", operation.label, err))
			failedBy = operation.label
		}
		if operation.kind == cloneDetect && failedBy == "" {
			result.DefaultBranch = *p.branch
		}
		if operation.kind == cloneWorktree && failedBy == "" {
			usable = true
			result.WorktreePath = git.WorktreePath(result.RepoPath, result.DefaultBranch)
		}
	}
	finishErr := execution.Finish("repository clone finished")
	result.Phases = execution.Phases()
	result.Err = errors.Join(result.Err, finishErr)
	return result
}
