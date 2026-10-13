package git

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}

	if len(args) < 3 || args[0] != "-C" {
		fmt.Fprintf(os.Stderr, "expected -C <dir> <cmd...>, got: %v", args)
		os.Exit(1)
	}

	dir := args[1]
	gitArgs := args[2:]

	switch {
	case len(gitArgs) >= 2 && gitArgs[0] == "rev-parse" && gitArgs[1] == "--git-dir":
		if strings.Contains(dir, "not-a-repo") {
			fmt.Fprintf(os.Stderr, "fatal: not a git repository (or any of the parent directories): .git")
			os.Exit(128)
		}
		fmt.Print(".")
		os.Exit(0)

	case len(gitArgs) >= 3 && gitArgs[0] == "worktree" && gitArgs[1] == "list" && gitArgs[2] == "--porcelain":
		fmt.Print("worktree /tmp/test-repo\nbare\n\nworktree /tmp/test-repo/main\nHEAD abc123\nbranch refs/heads/main")
		os.Exit(0)

	default:
		fmt.Fprintf(os.Stderr, "unexpected git command: %v", gitArgs)
		os.Exit(1)
	}
}

func installFakeGit(t *testing.T) {
	t.Helper()

	testBin, err := os.Executable()
	if err != nil {
		t.Fatalf("could not find test executable: %v", err)
	}

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("could not create bin dir: %v", err)
	}

	script := filepath.Join(binDir, "git")
	content := fmt.Sprintf("#!/bin/sh\nexec %q -test.run=^TestHelperProcess$ -- \"$@\"\n", testBin)
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("could not write fake git: %v", err)
	}

	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	chosenSSH(t)
}

// chosenSSH gives the test an ssh command of its own, so GitRunner does not
// ask a stand-in git for core.sshCommand before the command under test.
func chosenSSH(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_SSH_COMMAND", "ssh")
}

func TestGitRunner_SuccessfulCommand(t *testing.T) {
	installFakeGit(t)
	runner := &GitRunner{}

	output, err := runner.Run("/tmp/some-repo", "rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "." {
		t.Errorf("output = %q, want %q", output, ".")
	}
}

func TestGitRunner_FailedCommand(t *testing.T) {
	installFakeGit(t)
	runner := &GitRunner{}

	_, err := runner.Run("/tmp/not-a-repo", "rev-parse", "--git-dir")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error = %q, want it to contain 'not a git repository'", err.Error())
	}
}

func TestGitRunner_StdoutTrimmed(t *testing.T) {
	installFakeGit(t)
	runner := &GitRunner{}

	output, err := runner.Run("/tmp/some-repo", "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasPrefix(output, " ") || strings.HasPrefix(output, "\n") {
		t.Errorf("output has leading whitespace: %q", output)
	}
	if strings.HasSuffix(output, " ") || strings.HasSuffix(output, "\n") {
		t.Errorf("output has trailing whitespace: %q", output)
	}
}

func TestGitRunner_WorktreeListIntegration(t *testing.T) {
	installFakeGit(t)
	runner := &GitRunner{}

	output, err := runner.Run("/tmp/some-repo", "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	worktrees, err := ParsePorcelain(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(worktrees) != 2 {
		t.Fatalf("got %d worktrees, want 2", len(worktrees))
	}
	if !worktrees[0].IsBare {
		t.Error("first worktree should be bare")
	}
	if worktrees[1].Branch != "refs/heads/main" {
		t.Errorf("second worktree branch = %q, want refs/heads/main", worktrees[1].Branch)
	}
}

// childEnv returns the environment a process started by git sees, read through
// a shell alias so the assertion covers what really reaches the child.
func childEnv(t *testing.T, runner *GitRunner) map[string]string {
	t.Helper()
	return childEnvIn(t, runner, t.TempDir())
}

// childEnvIn is childEnv for a git started in dir. words are passed to git
// after the alias and ignored by it; a test adds a command name such as
// "fetch" to see the environment of a command GitRunner treats as that one.
func childEnvIn(t *testing.T, runner *GitRunner, dir string, words ...string) map[string]string {
	t.Helper()
	out, err := runner.Run(dir, append([]string{"-c", "alias.child-env=!env #", "child-env"}, words...)...)
	if err != nil {
		t.Fatalf("reading the child environment: %v", err)
	}
	env := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if name, value, ok := strings.Cut(line, "="); ok {
			env[name] = value
		}
	}
	return env
}

func TestGitRunner_IgnoresInheritedGitDir(t *testing.T) {
	runner := &GitRunner{}
	foreign, target := t.TempDir(), t.TempDir()
	for _, dir := range []string{foreign, target} {
		if _, err := runner.Run(dir, "init"); err != nil {
			t.Fatalf("git init %s: %v", dir, err)
		}
	}

	// What a hook or a `git` alias exports to the commands it starts.
	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(foreign, ".git", "index"))

	got, err := runner.Run(target, "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(target, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(got); err != nil || resolved != want {
		t.Errorf("git dir = %q, want %q: the inherited GIT_DIR redirected the command", got, want)
	}
}

func TestGitRunner_DropsInheritedRepositoryEnv(t *testing.T) {
	const marker = "sentei-inherited-marker"
	for _, name := range repositoryEnv {
		t.Setenv(name, marker)
	}

	for name, value := range childEnv(t, &GitRunner{}) {
		if value == marker {
			t.Errorf("%s reached the git child process", name)
		}
	}
}

func TestGitRunner_DisablesTerminalPrompts(t *testing.T) {
	t.Setenv("GIT_TERMINAL_PROMPT", "1")

	if got := childEnv(t, &GitRunner{})["GIT_TERMINAL_PROMPT"]; got != "0" {
		t.Errorf("GIT_TERMINAL_PROMPT = %q, want 0: a credential prompt would be drawn under the TUI", got)
	}
}

func TestGitRunner_ExtraEnvReachesChild(t *testing.T) {
	runner := &GitRunner{Env: []string{"GIT_COMMITTER_DATE=2001-02-03T04:05:06"}}

	if got := childEnv(t, runner)["GIT_COMMITTER_DATE"]; got != "2001-02-03T04:05:06" {
		t.Errorf("GIT_COMMITTER_DATE = %q, want the value from Env", got)
	}
}

// installScriptGit puts a shell script named git first on PATH. A background
// process whose pid the script writes to "$0.pid" is killed when the test ends.
func installScriptGit(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "git")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(path)+string(os.PathListSeparator)+os.Getenv("PATH"))
	chosenSSH(t)
	t.Cleanup(func() {
		data, err := os.ReadFile(path + ".pid")
		if err != nil {
			return
		}
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				_ = process.Kill()
			}
		}
	})
}

func TestGitRunner_TimeoutNamesCommandAndDuration(t *testing.T) {
	installScriptGit(t, "exec sleep 3600")
	runner := &GitRunner{timeout: 50 * time.Millisecond}

	_, err := runner.Run(t.TempDir(), "fetch", "--all")
	if err == nil {
		t.Fatal("expected a timeout error from a git command that never finishes")
	}
	for _, want := range []string{"git fetch --all", "timed out after 50ms"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
}

func TestGitRunner_TimeoutReturnsWhenAnOrphanHoldsTheOutput(t *testing.T) {
	// The script survives SIGTERM and leaves a child holding stdout and stderr,
	// like a transport helper stuck on a prompt after git itself is gone.
	installScriptGit(t, "trap '' TERM\nsleep 3600 &\necho $! > \"$0.pid\"\nwait")
	runner := &GitRunner{timeout: 50 * time.Millisecond, waitDelay: 50 * time.Millisecond}

	_, err := runner.Run(t.TempDir(), "clone", "url")
	if err == nil || !strings.Contains(err.Error(), "timed out after 50ms") {
		t.Errorf("error = %v, want a timeout error", err)
	}
}

func TestGitRunner_SucceedsWhenABackgroundProcessHoldsTheOutput(t *testing.T) {
	// A hook that starts a daemon without redirecting its output keeps the
	// pipes open after git has exited successfully.
	installScriptGit(t, "echo done\nsleep 3600 &\necho $! > \"$0.pid\"")
	// Output still unread when the delay ends is dropped with the pipe, so the
	// delay is long enough for a loaded machine to schedule the read.
	runner := &GitRunner{waitDelay: time.Second}

	out, err := runner.Run(t.TempDir(), "worktree", "add", "x")
	if err != nil {
		t.Fatalf("a git command that exited 0 must not fail: %v", err)
	}
	if out != "done" {
		t.Errorf("output = %q, want %q", out, "done")
	}
}

func TestRunShell_DropsInheritedGitDir(t *testing.T) {
	runner := &GitRunner{}
	foreign, target := t.TempDir(), t.TempDir()
	for _, dir := range []string{foreign, target} {
		if _, err := runner.Run(dir, "init"); err != nil {
			t.Fatalf("git init %s: %v", dir, err)
		}
	}
	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))

	// An install script that asks git where it is (husky does) must see the
	// worktree it runs in.
	got, err := (&DefaultShellRunner{}).RunShell(target, "git rev-parse --git-dir")
	if err != nil {
		t.Fatalf("RunShell: %v", err)
	}
	if got != ".git" {
		t.Errorf("git dir seen by the install command = %q, want .git", got)
	}
}

func TestGitRunner_ForcesCLocale(t *testing.T) {
	t.Setenv("LC_ALL", "de_DE.UTF-8")

	if got := childEnv(t, &GitRunner{})["LC_ALL"]; got != "C" {
		t.Errorf("LC_ALL = %q, want C: sentei matches on git's English messages", got)
	}
}

func TestGitRunner_MessagesStayEnglishUnderGermanLocale(t *testing.T) {
	runner := &GitRunner{}
	repo := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "wt")
	for _, args := range [][]string{
		{"init"},
		{"commit", "--allow-empty", "-m", "init"},
		{"worktree", "add", worktree},
	} {
		if _, err := runner.Run(repo, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	t.Setenv("LANG", "de_DE.UTF-8")
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	t.Setenv("LANGUAGE", "de")

	// The message worktree removal matches on to tell "nothing to unlock"
	// from a real failure.
	const want = "is not locked"
	unlock := []string{"-C", repo, "worktree", "unlock", worktree}
	if translated, _ := exec.Command("git", unlock...).CombinedOutput(); strings.Contains(string(translated), want) {
		t.Skip("git prints English under de_DE here (locale or translation not installed)")
	}

	_, err := runner.Run(repo, unlock[2:]...)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v, want it to contain %q", err, want)
	}
}

func TestGitRunner_TimeoutLetsGitCleanUp(t *testing.T) {
	// A server that accepts and never answers, so the clone blocks after git
	// has created its destination directory.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	t.Cleanup(func() {
		close(released)
		_ = listener.Close()
	})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		<-released
		_ = conn.Close()
	}()

	destination := filepath.Join(t.TempDir(), "clone")
	runner := &GitRunner{timeout: 500 * time.Millisecond}
	_, err = runner.Run(t.TempDir(), "clone", "git://"+listener.Addr().String()+"/repo.git", destination)
	if err == nil || !strings.Contains(err.Error(), "timed out after 500ms") {
		t.Fatalf("error = %v, want a timeout error", err)
	}

	// git removes a partial clone when it is asked to stop. Killed outright it
	// leaves the directory behind, and the next clone into it fails.
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the partial clone at %s was left behind (stat error: %v)", destination, err)
	}
}

func TestGitRunner_ChildReadsNoInput(t *testing.T) {
	// Input that never ends: a child that inherited it would wait forever,
	// the way git waits on a terminal for an answer nobody can type.
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = stdin
		_ = writer.Close()
		_ = reader.Close()
	})

	runner := &GitRunner{timeout: 2 * time.Second, waitDelay: 100 * time.Millisecond}
	out, err := runner.Run(t.TempDir(), "-c", "alias.read-input=!cat", "read-input")
	if err != nil {
		t.Fatalf("a git child that reads input must see the end of it at once: %v", err)
	}
	if out != "" {
		t.Errorf("the child read %q from sentei's input, want nothing", out)
	}
}

func TestGitRunner_ExplainsADisabledCredentialPrompt(t *testing.T) {
	installScriptGit(t, `echo "fatal: could not read Username for 'https://example.com': terminal prompts disabled" >&2; exit 128`)

	_, err := (&GitRunner{}).Run(t.TempDir(), "clone", "https://example.com/repo.git")
	if err == nil {
		t.Fatal("expected the clone to fail")
	}
	for _, want := range []string{"terminal prompts disabled", "credential helper", "SSH remote"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
}

// sentei runs status in the background across every worktree. Left alone, git
// rewrites the index it has just refreshed, taking index.lock under whatever
// git command the user runs at that moment.
func TestGitRunner_StatusLeavesTheIndexAlone(t *testing.T) {
	dir := t.TempDir()
	runner := &GitRunner{}
	run := func(args ...string) {
		t.Helper()
		if _, err := runner.Run(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "--quiet")
	tracked := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(tracked, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "--quiet", "-m", "init")
	// Same content, newer mtime: the stat data cached in the index is stale,
	// which is what makes a plain status write the index back.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(tracked, later, later); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(dir, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}

	run("status", "--porcelain")

	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("git status rewrote the index: read-only commands must not take optional locks")
	}
}
