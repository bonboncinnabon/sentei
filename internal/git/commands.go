package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type CommandRunner interface {
	Run(dir string, args ...string) (string, error)
}

type GitRunner struct{}

func (r *GitRunner) Run(dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", fullArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// When git produced no stderr (e.g. the binary was not found, or it was
		// killed by a signal), fall back to the exec error so the cause is not
		// erased into an empty message.
		if stderrMsg := strings.TrimSpace(stderr.String()); stderrMsg != "" {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), stderrMsg)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ShellRunner executes arbitrary shell commands (not git-specific).
type ShellRunner interface {
	RunShell(dir string, command string) (string, error)
}

type DefaultShellRunner struct{}

func (r *DefaultShellRunner) RunShell(dir string, command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderrMsg := strings.TrimSpace(stderr.String()); stderrMsg != "" {
			return "", fmt.Errorf("%s: %s", command, stderrMsg)
		}
		return "", fmt.Errorf("%s: %w", command, err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func ValidateRepository(runner CommandRunner, repoPath string) error {
	if _, err := runner.Run(repoPath, "rev-parse", "--git-dir"); err != nil {
		// Wrap rather than assert: the real cause (git missing, permission denied,
		// path absent) must survive instead of being replaced by a fixed message.
		return fmt.Errorf("not a git repository %q: %w", repoPath, err)
	}
	return nil
}

// BranchExists reports whether a local branch exists in the repository.
func BranchExists(runner CommandRunner, repoPath, branch string) bool {
	_, err := runner.Run(repoPath, "show-ref", "--verify", "refs/heads/"+branch)
	return err == nil
}

func ListWorktrees(runner CommandRunner, repoPath string) ([]Worktree, error) {
	if err := ValidateRepository(runner, repoPath); err != nil {
		return nil, err
	}

	output, err := runner.Run(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	return ParsePorcelain(output)
}
