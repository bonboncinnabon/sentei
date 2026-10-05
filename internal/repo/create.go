package repo

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

// GhRunner executes gh CLI commands directly without a shell, preventing shell injection.
type GhRunner interface {
	RunGh(dir string, args ...string) (string, error)
}

// DefaultGhRunner is the production GhRunner that invokes gh via exec.Command.
type DefaultGhRunner struct{}

func (r *DefaultGhRunner) RunGh(dir string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

type CreateOptions struct {
	Name          string
	Location      string
	PublishGitHub bool
	Visibility    string // "private" or "public"
	Description   string
}

// PhaseGitHub is the name of the create flow's publish phase. A failure here is
// "soft" (the local repo is fine, just unpublished); a failure in any other
// phase is "hard" (the local repo is broken). Single source for that test.
const PhaseGitHub = "GitHub"

type CreateResult struct {
	RepoPath     string
	WorktreePath string
	GitHubURL    string
	Phases       []progress.Phase
	Err          error
}

const createSetupPhaseID progress.PhaseID = "create:setup"
const createGitHubPhaseID progress.PhaseID = "create:github"

type createOperation struct {
	phaseID progress.PhaseID
	stepID  progress.StepID
	label   string
	run     progress.StepFunc
}

type preparedCreate struct {
	result     CreateResult
	plan       progress.Plan
	operations []createOperation
	publish    bool
	name       string
	githubUser *string
}

// SetupFailed reports whether a non-GitHub phase failed (the local repo itself is
// broken, not merely unpublished) and the first such error.
func (r CreateResult) SetupFailed() (bool, error) {
	if r.Err != nil {
		return true, r.Err
	}
	for _, p := range r.Phases {
		if p.Name != PhaseGitHub && p.HasFailures() {
			_, step, _ := progress.FirstFailure([]progress.Phase{p})
			return true, step.Error
		}
	}
	return false, nil
}

func Create(runner git.CommandRunner, shell git.ShellRunner, opts CreateOptions, emit func(progress.Event)) CreateResult {
	return CreateWithGh(runner, shell, &DefaultGhRunner{}, opts, emit)
}

func CreateWithGh(runner git.CommandRunner, _ git.ShellRunner, gh GhRunner, opts CreateOptions, emit func(progress.Event)) CreateResult {
	return prepareCreate(runner, gh, opts).run(emit)
}

func prepareCreate(runner git.CommandRunner, gh GhRunner, opts CreateOptions) preparedCreate {
	repoPath := filepath.Join(opts.Location, opts.Name)
	barePath := filepath.Join(repoPath, ".bare")
	mainPath := git.WorktreePath(repoPath, "main")
	ghUser := ""
	prepared := preparedCreate{result: CreateResult{RepoPath: repoPath}, publish: opts.PublishGitHub, name: opts.Name, githubUser: &ghUser}
	add := func(phaseID progress.PhaseID, phaseLabel, id, label string, run progress.StepFunc) {
		if len(prepared.plan.Phases) == 0 || prepared.plan.Phases[len(prepared.plan.Phases)-1].ID != phaseID {
			prepared.plan.Phases = append(prepared.plan.Phases, progress.PlannedPhase{ID: phaseID, Label: phaseLabel})
		}
		prepared.plan.Phases[len(prepared.plan.Phases)-1].Steps = append(prepared.plan.Phases[len(prepared.plan.Phases)-1].Steps, progress.PlannedStep{ID: id, Label: label})
		prepared.operations = append(prepared.operations, createOperation{phaseID: phaseID, stepID: id, label: label, run: run})
	}
	add(createSetupPhaseID, "Setup", "directory", "Create directory", func() (string, error) {
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			return "", err
		}
		// Abort if the directory already had content.
		if entries, _ := os.ReadDir(repoPath); len(entries) > 0 {
			return "", fmt.Errorf("directory already exists and is not empty: %s", repoPath)
		}
		return "", nil
	})
	add(createSetupPhaseID, "Setup", "bare-init", "Init bare repository", func() (string, error) {
		if err := os.MkdirAll(barePath, 0755); err != nil {
			return "", err
		}
		_, err := runner.Run(barePath, "init", "--bare")
		return "", err
	})
	add(createSetupPhaseID, "Setup", "git-pointer", "Create .git pointer", func() (string, error) {
		return "", os.WriteFile(filepath.Join(repoPath, ".git"), []byte("gitdir: .bare\n"), 0644)
	})
	add(createSetupPhaseID, "Setup", "refspec", "Configure refspec", func() (string, error) {
		_, err := runner.Run(barePath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
		return "", err
	})
	add(createSetupPhaseID, "Setup", "main-worktree", "Create main worktree", func() (string, error) {
		_, err := runner.Run(repoPath, "worktree", "add", mainPath, "-b", "main")
		return "", err
	})
	add(createSetupPhaseID, "Setup", "initial-commit", "Initial commit", func() (string, error) {
		if err := os.MkdirAll(mainPath, 0755); err != nil {
			return "", err
		}
		readmePath := filepath.Join(mainPath, "README.md")
		if err := os.WriteFile(readmePath, fmt.Appendf(nil, "# %s\n", opts.Name), 0644); err != nil {
			return "", err
		}
		if _, err := runner.Run(mainPath, "add", "-A"); err != nil {
			return "", err
		}
		_, err := runner.Run(mainPath, "commit", "-m", "Initial commit")
		return "", err
	})
	if !opts.PublishGitHub {
		return prepared
	}
	add(createGitHubPhaseID, PhaseGitHub, "github-user", "Look up GitHub user", func() (string, error) {
		var err error
		ghUser, err = gh.RunGh(repoPath, "api", "user", "--jq", ".login")
		return ghUser, err
	})
	add(createGitHubPhaseID, PhaseGitHub, "github-repository", "Create GitHub repository", func() (string, error) {
		ghArgs := []string{"repo", "create", opts.Name, "--" + opts.Visibility}
		if opts.Description != "" {
			ghArgs = append(ghArgs, "--description", opts.Description)
		}
		_, err := gh.RunGh(repoPath, ghArgs...)
		return "", err
	})
	add(createGitHubPhaseID, PhaseGitHub, "github-remote", "Configure remote", func() (string, error) {
		_, err := runner.Run(barePath, "remote", "set-url", "origin", ghRemoteURL(gh, repoPath, ghUser, opts.Name))
		return "", err
	})
	add(createGitHubPhaseID, PhaseGitHub, "github-push", "Push to GitHub", func() (string, error) {
		if _, err := runner.Run(mainPath, "push", "-u", "origin", "main"); err != nil {
			// The empty remote repo from "Create GitHub repository" is left behind;
			// tell the user so they can delete it or push manually before retrying.
			return "", fmt.Errorf("%w (an empty GitHub repo %q now exists; delete it or push to it manually before retrying)", err, opts.Name)
		}
		return "", nil
	})
	add(createGitHubPhaseID, PhaseGitHub, "github-head", "Set remote HEAD", func() (string, error) {
		_, err := runner.Run(barePath, "remote", "set-head", "origin", "main")
		return "", err
	})
	return prepared
}

func (p preparedCreate) run(emit func(progress.Event)) CreateResult {
	result := p.result
	execution, err := progress.Start(p.plan, emit)
	if err != nil {
		result.Err = fmt.Errorf("starting repository create: %w", err)
		return result
	}
	for index, operation := range p.operations {
		step, transitionErr := execution.Run(operation.phaseID, operation.stepID, operation.run)
		if transitionErr != nil {
			result.Err = fmt.Errorf("executing %s: %w", operation.label, transitionErr)
			break
		}
		if operation.phaseID == createSetupPhaseID && index == 5 && step.Status == progress.StepDone {
			result.WorktreePath = git.WorktreePath(result.RepoPath, "main")
		}
		if step.Status != progress.StepFailed {
			continue
		}
		_ = execution.SkipPending(operation.phaseID, "blocked by "+operation.label)
		if operation.phaseID == createSetupPhaseID && p.publish {
			_ = execution.SkipPending(createGitHubPhaseID, "blocked by "+operation.label)
		}
		break
	}
	finishErr := execution.Finish("repository create finished")
	result.Phases = execution.Phases()
	result.Err = errors.Join(result.Err, finishErr)
	if result.Err == nil && result.WorktreePath != "" && p.publish && !progress.PhasesHaveFailures(result.Phases) {
		result.GitHubURL = fmt.Sprintf("github.com/%s/%s", *p.githubUser, p.name)
	}
	return result
}

// ghRemoteURL returns the origin URL matching gh's configured git protocol, so
// the push uses the auth the user actually has. gh defaults to HTTPS; only an
// explicit "ssh" protocol yields an SSH URL.
func ghRemoteURL(gh GhRunner, repoPath, user, name string) string {
	if proto, err := gh.RunGh(repoPath, "config", "get", "git_protocol"); err == nil && strings.TrimSpace(proto) == "ssh" {
		return fmt.Sprintf("git@github.com:%s/%s.git", user, name)
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", user, name)
}
