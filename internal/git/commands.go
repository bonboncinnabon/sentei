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
	"sync"
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

// Environ returns the process environment without repositoryEnv. Every
// process sentei starts gets it, so a GIT_DIR exported to sentei cannot point
// a child at another repository.
func Environ() []string {
	return slices.DeleteFunc(os.Environ(), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return slices.Contains(repositoryEnv, name)
	})
}

// batchSSH is the ssh command git children get when the user has chosen none.
// ssh opens the terminal itself to confirm a host key or read a passphrase,
// where GIT_TERMINAL_PROMPT has no say. In batch mode it fails instead.
const batchSSH = "GIT_SSH_COMMAND=ssh -oBatchMode=yes"

// sshUsers lists the git commands that can end up running ssh: those that
// connect to a remote, and those that check files out, where a filter such as
// git-lfs or a lazy fetch in a partial clone connects on their behalf.
var sshUsers = []string{
	"clone", "fetch", "ls-remote", "pull", "push", "remote", "submodule",
	"checkout", "merge", "worktree",
}

// sshChoices holds, per directory, whether the user's git config names an ssh
// command there. It is read once per directory for the life of the process,
// so a run of commands in one repository costs one config read, not one each.
var sshChoices sync.Map // directory -> func() (chosen, conditional bool)

// GitRunner runs git against the repository named by dir and nothing else:
// the child cannot prompt on the terminal, reads no input, ignores an
// inherited GIT_DIR, prints untranslated messages, takes no optional locks,
// runs ssh in batch mode unless the user chose an ssh command, and is stopped
// after gitTimeout.
type GitRunner struct {
	// Env holds extra NAME=value entries for the child, such as GIT_AUTHOR_DATE.
	Env []string

	// Zero means gitTimeout and gitWaitDelay. Tests shorten them.
	timeout, waitDelay time.Duration
}

func (r *GitRunner) Run(dir string, args ...string) (string, error) {
	env := append(Environ(), r.Env...)
	if r.sshIsUnchosen(dir, env, args) {
		env = append(env, batchSSH)
	}
	return r.run(dir, env, args...)
}

// sshIsUnchosen reports whether a command in dir may run ssh and the user has
// left the choice of ssh command to git: neither GIT_SSH nor GIT_SSH_COMMAND
// is in env and core.sshCommand is not configured. GIT_SSH_COMMAND outranks
// the other two, so setting it over a choice the user made would replace
// their key or proxy.
func (r *GitRunner) sshIsUnchosen(dir string, env, args []string) bool {
	// Any argument counts, so options before the command name do not hide it.
	// A branch that shares a name with a command costs one config read.
	if !slices.ContainsFunc(args, func(arg string) bool { return slices.Contains(sshUsers, arg) }) {
		return false
	}
	if slices.ContainsFunc(env, func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return name == "GIT_SSH" || name == "GIT_SSH_COMMAND"
	}) {
		return false
	}
	read, _ := sshChoices.LoadOrStore(dir, sync.OnceValues(func() (chosen, conditional bool) {
		keys, err := r.run(dir, env, "config", "--name-only", "--get-regexp", `^(core\.sshcommand$|includeif\.)`)
		if err != nil {
			// Exit status 1 is "no such key". After any other failure the
			// answer is unknown, and ssh is left as the user has it.
			var exit *exec.ExitError
			return !errors.As(err, &exit) || exit.ExitCode() != 1, false
		}
		return strings.Contains(keys, "core.sshcommand"), strings.Contains(keys, "includeif.")
	}))
	chosen, conditional := read.(func() (bool, bool))()
	// git clone reads config again once the new repository exists. A
	// conditional include can then name an ssh command for it (includeIf
	// "gitdir:..." or "hasconfig:remote.*.url:...") that no read made
	// beforehand can see, so any conditional include counts as a choice.
	return !chosen && (!conditional || !slices.Contains(args, "clone"))
}

// run starts git with env, plus the settings every git child gets, and
// returns its trimmed output.
func (r *GitRunner) run(dir string, env []string, args ...string) (string, error) {
	timeout := cmp.Or(r.timeout, gitTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	// The last entry for a name wins, so the settings below hold whatever the
	// caller's environment says. LC_ALL=C keeps git's messages in English,
	// which callers match on ("is not locked", "[gone]"). Stdin stays nil: the
	// child reads the null device. GIT_OPTIONAL_LOCKS=0 keeps read-only
	// commands read-only: a background status must not rewrite the index and
	// hold index.lock under the user's own git command.
	cmd.Env = append(slices.Clip(env), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
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
			return "", fmt.Errorf("%s: %s%s", command, stderrMsg, promptHint(stderrMsg, slices.Contains(env, batchSSH)))
		}
		return "", fmt.Errorf("%s: %w", command, err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// promptHint returns what to do about a failure that a prompt would have
// prevented, or "" for any other failure. batch says ssh ran under batchSSH:
// the ssh messages mean something else under a command the user chose.
func promptHint(stderrMsg string, batch bool) string {
	switch {
	// What GIT_TERMINAL_PROMPT=0 turns a username or password prompt into.
	case strings.Contains(stderrMsg, "terminal prompts disabled"):
		return "; sentei does not ask for credentials: store them with a git credential helper or use an SSH remote, then run it again"
	case !batch:
		return ""
	case strings.Contains(stderrMsg, "Host key verification failed"):
		return "; sentei runs ssh without prompts: connect to the host once by hand (for example `ssh -T git@<host>`) and accept its key, then run it again"
	// Also what a key that the server does not know ends in, hence the "if".
	case strings.Contains(stderrMsg, "Permission denied ("):
		return "; sentei runs ssh without prompts: if the key has a passphrase, load it into an agent with ssh-add, then run it again"
	}
	return ""
}

// ShellRunner executes arbitrary shell commands (not git-specific).
type ShellRunner interface {
	RunShell(dir string, command string) (string, error)
}

type DefaultShellRunner struct{}

func (r *DefaultShellRunner) RunShell(dir string, command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = Environ()
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
