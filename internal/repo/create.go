package repo

import (
	"bytes"
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
		// gh that could not be started, or was killed, leaves no stderr. The
		// exec error is then the only explanation there is.
		if stderrMsg := strings.TrimSpace(stderr.String()); stderrMsg != "" {
			return "", fmt.Errorf("gh %s: %s", strings.Join(args, " "), stderrMsg)
		}
		return "", fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
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

func Create(runner git.CommandRunner, opts CreateOptions, emit func(progress.Event)) CreateResult {
	return CreateWithGh(runner, &DefaultGhRunner{}, opts, emit)
}

func CreateWithGh(runner git.CommandRunner, gh GhRunner, opts CreateOptions, emit func(progress.Event)) CreateResult {
	create, finish := prepareCreate(runner, gh, opts)
	return finish(create.run(emit))
}

// prepareCreate declares the create flow and returns the function that turns
// its outcome into a CreateResult.
func prepareCreate(runner git.CommandRunner, gh GhRunner, opts CreateOptions) (flow, func([]progress.Phase, error) CreateResult) {
	repoPath := filepath.Join(opts.Location, opts.Name)
	barePath := filepath.Join(repoPath, ".bare")
	mainPath := git.WorktreePath(repoPath, "main")
	ghUser := ""
	create := flow{name: "repository create"}
	add := func(phaseID progress.PhaseID, phaseLabel, id, label string, run progress.StepFunc) {
		create.add(phaseID, phaseLabel, step{id: id, label: label, run: run})
	}
	finish := func(phases []progress.Phase, err error) CreateResult {
		result := CreateResult{RepoPath: repoPath, Phases: phases, Err: err}
		if stepDone(phases, createSetupPhaseID, "initial-commit") {
			result.WorktreePath = mainPath
		}
		if opts.PublishGitHub && !result.HasFailures() {
			result.GitHubURL = fmt.Sprintf("github.com/%s/%s", ghUser, opts.Name)
		}
		return result
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
		// The branch is named here, not left to init.defaultBranch: HEAD has
		// to name the branch the main worktree is created on.
		_, err := runner.Run(barePath, "init", "--bare", "--initial-branch=main")
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
		return create, finish
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
	return create, finish
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
