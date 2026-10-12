package git

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"
)

type CommandRunner interface {
	Run(dir string, args ...string) (string, error)
}

const (
	// gitTimeout ends a git command that will never finish: a stalled
	// transfer, or a prompt nobody can see. It is far longer than a large
	// clone, fetch or checkout takes, so it never cuts real work short.
	gitTimeout = time.Hour
	// gitWaitDelay is how long a timed out git gets to exit after SIGTERM, and
	// how long output pipes may stay open once git has exited.
	gitWaitDelay = 10 * time.Second
)

// repositoryEnv lists the variables git ties to one repository
// (git rev-parse --local-env-vars), except the two that only carry config.
// A hook or a git alias exports them to what it starts; inherited by a child
// git they replace the repository sentei was asked to work on.
var repositoryEnv = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_COMMON_DIR",
	"GIT_CONFIG",
	"GIT_DIR",
	"GIT_GRAFT_FILE",
	"GIT_IMPLICIT_WORK_TREE",
	"GIT_INDEX_FILE",
	"GIT_NO_REPLACE_OBJECTS",
	"GIT_OBJECT_DIRECTORY",
	"GIT_PREFIX",
	"GIT_REPLACE_REF_BASE",
	"GIT_SHALLOW_FILE",
	"GIT_WORK_TREE",
}

// inheritedEnv returns the process environment without repositoryEnv.
func inheritedEnv() []string {
	return slices.DeleteFunc(os.Environ(), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return slices.Contains(repositoryEnv, name)
	})
}

// GitRunner runs git against the repository named by dir and nothing else:
// the child cannot prompt on the terminal, reads no input, ignores an
// inherited GIT_DIR, prints untranslated messages, and is stopped after
// gitTimeout.
type GitRunner struct {
	// Env holds extra NAME=value entries for the child, such as GIT_AUTHOR_DATE.
	Env []string

	// Zero means gitTimeout and gitWaitDelay. Tests shorten them.
	timeout, waitDelay time.Duration
}

func (r *GitRunner) Run(dir string, args ...string) (string, error) {
	timeout := cmp.Or(r.timeout, gitTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	// The last entry for a name wins, so the settings below hold whatever the
	// caller's environment says. LC_ALL=C keeps git's messages in English,
	// which callers match on ("is not locked", "[gone]"). Stdin stays nil: the
	// child reads the null device.
	cmd.Env = append(append(inheritedEnv(), r.Env...), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	// SIGTERM lets git remove its lock files. A git that ignores it, or a
	// helper it left behind holding the output pipes, is cut off by WaitDelay.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = cmp.Or(r.waitDelay, gitWaitDelay)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// ErrWaitDelay alone means git exited 0 and a background process it
	// started (a hook's daemon) kept the pipes open: the command succeeded.
	if err := cmd.Run(); err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		command := "git " + strings.Join(args, " ")
		if ctx.Err() != nil {
			return "", fmt.Errorf("%s: timed out after %s; check the network connection and that no credential or SSH prompt is waiting for input, then run it again", command, timeout)
		}
		// When git produced no stderr (e.g. the binary was not found, or it was
		// killed by a signal), fall back to the exec error so the cause is not
		// erased into an empty message.
		if stderrMsg := strings.TrimSpace(stderr.String()); stderrMsg != "" {
			return "", fmt.Errorf("%s: %s", command, stderrMsg)
		}
		return "", fmt.Errorf("%s: %w", command, err)
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
	cmd.Env = inheritedEnv()
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
