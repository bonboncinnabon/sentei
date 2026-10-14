package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const batchSSHCommand = "ssh -oBatchMode=yes"

// noSSHChoice removes the two variables that name an ssh command, so a test
// starts from a user who has chosen none whatever the developer's shell holds.
func noSSHChoice(t *testing.T) {
	t.Helper()
	for _, name := range []string{"GIT_SSH", "GIT_SSH_COMMAND"} {
		t.Setenv(name, "") // registers the restore
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

// globalGitConfig makes content the only global git config for the test.
func globalGitConfig(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", path)
}

// fakeSSH writes an executable named ssh that runs script, and returns the
// directory holding it. The name matters: git only passes OpenSSH options to
// a command called ssh.
func fakeSSH(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// sshEnv returns the environment of a git command that may run ssh.
func sshEnv(t *testing.T, runner *GitRunner, dir string) map[string]string {
	t.Helper()
	return childEnvIn(t, runner, dir, "fetch")
}

// The git commands sentei runs that can end up in ssh: clone, fetch and push
// connect themselves, remote does for set-head and prune, and worktree add
// and merge check files out, where git-lfs or a partial clone connects.
func TestGitRunner_RunsSSHInBatchMode(t *testing.T) {
	noSSHChoice(t)

	for _, command := range []string{"clone", "fetch", "push", "remote", "worktree", "merge"} {
		if got := childEnvIn(t, &GitRunner{}, t.TempDir(), command)["GIT_SSH_COMMAND"]; got != batchSSHCommand {
			t.Errorf("git %s: GIT_SSH_COMMAND = %q, want %q: ssh would ask for a host key or passphrase on the terminal", command, got, batchSSHCommand)
		}
	}
}

// A conditional include can hide an ssh command only from the read made
// before a clone, and only when the file it brings in can set one. In a
// repository that exists the read applies the include, so every other command
// runs ssh in batch mode either way.
func TestGitRunner_AConditionalIncludeHoldsBackCloneOnlyWhenItCanNameSSH(t *testing.T) {
	tests := []struct {
		name     string
		included string // content of work.gitconfig beside the global config; "" leaves it missing
		absolute bool   // name the file by its absolute path, not relative to the global config
		holdBack bool
	}{
		// The common use: an identity for some repositories, and no ssh command.
		{name: "identity only", included: "[user]\n\tname = Work\n"},
		{name: "missing file"},
		{name: "ssh command", included: "[core]\n\tsshCommand = ssh -i work\n", holdBack: true},
		{name: "ssh command by absolute path", included: "[core]\n\tsshCommand = ssh -i work\n", absolute: true, holdBack: true},
		{name: "ssh command through a plain include", included: "[include]\n\tpath = more.gitconfig\n", holdBack: true},
		{name: "conditional include of its own", included: "[includeIf \"gitdir:/nonexistent/other/\"]\n\tpath = x\n", holdBack: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noSSHChoice(t)
			configDir := t.TempDir()
			work := filepath.Join(configDir, "work.gitconfig")
			if tt.included != "" {
				if err := os.WriteFile(work, []byte(tt.included), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			// Read only through work.gitconfig's plain include.
			more := "[core]\n\tsshCommand = ssh -i more\n"
			if err := os.WriteFile(filepath.Join(configDir, "more.gitconfig"), []byte(more), 0o644); err != nil {
				t.Fatal(err)
			}
			path := "work.gitconfig"
			if tt.absolute {
				path = work
			}
			global := filepath.Join(configDir, "gitconfig")
			if err := os.WriteFile(global, []byte("[includeIf \"gitdir:/nonexistent/work/\"]\n\tpath = "+path+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GIT_CONFIG_GLOBAL", global)
			runner := &GitRunner{}

			if got := sshEnv(t, runner, t.TempDir())["GIT_SSH_COMMAND"]; got != batchSSHCommand {
				t.Errorf("git fetch: GIT_SSH_COMMAND = %q, want %q: the include does not apply here", got, batchSSHCommand)
			}
			got, present := childEnvIn(t, runner, t.TempDir(), "clone")["GIT_SSH_COMMAND"]
			switch {
			case tt.holdBack && present:
				t.Errorf("git clone: GIT_SSH_COMMAND = %q reached git: the include may name an ssh command for the new repository", got)
			case !tt.holdBack && got != batchSSHCommand:
				t.Errorf("git clone: GIT_SSH_COMMAND = %q (set: %v), want %q: the included file names no ssh command", got, present, batchSSHCommand)
			}
		})
	}
}

func TestGitRunner_LeavesAChosenSSHCommandAlone(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		runner *GitRunner
		config string
		want   string
		absent bool
	}{
		{name: "GIT_SSH_COMMAND", env: map[string]string{"GIT_SSH_COMMAND": "ssh -i work"}, want: "ssh -i work"},
		// git runs a variable that is set and empty; it is still the user's.
		{name: "empty GIT_SSH_COMMAND", env: map[string]string{"GIT_SSH_COMMAND": ""}, want: ""},
		{name: "GIT_SSH", env: map[string]string{"GIT_SSH": "/usr/bin/ssh"}, absent: true},
		{name: "runner Env", runner: &GitRunner{Env: []string{"GIT_SSH_COMMAND=ssh -i extra"}}, want: "ssh -i extra"},
		{name: "global core.sshCommand", config: "[core]\n\tsshCommand = ssh -i work\n", absent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noSSHChoice(t)
			for name, value := range tt.env {
				t.Setenv(name, value)
			}
			if tt.config != "" {
				globalGitConfig(t, tt.config)
			}
			runner := tt.runner
			if runner == nil {
				runner = &GitRunner{}
			}

			got, present := sshEnv(t, runner, t.TempDir())["GIT_SSH_COMMAND"]
			switch {
			case tt.absent && present:
				t.Errorf("GIT_SSH_COMMAND = %q reached git: it outranks the ssh command the user chose", got)
			case !tt.absent && (!present || got != tt.want):
				t.Errorf("GIT_SSH_COMMAND = %q (set: %v), want the user's %q", got, present, tt.want)
			}
		})
	}
}

func TestGitRunner_ReadsTheSSHCommandOfEachRepository(t *testing.T) {
	noSSHChoice(t)
	runner := &GitRunner{}
	chosen, plain := t.TempDir(), t.TempDir()
	for _, dir := range []string{chosen, plain} {
		if _, err := runner.Run(dir, "init", "--quiet"); err != nil {
			t.Fatalf("git init %s: %v", dir, err)
		}
	}
	// A per-repository key or proxy.
	if _, err := runner.Run(chosen, "config", "core.sshCommand", "ssh -i work"); err != nil {
		t.Fatal(err)
	}

	if got, present := sshEnv(t, runner, plain)["GIT_SSH_COMMAND"]; got != batchSSHCommand {
		t.Errorf("GIT_SSH_COMMAND = %q (set: %v) in a repository with no ssh command, want %q", got, present, batchSSHCommand)
	}
	if got, present := sshEnv(t, runner, chosen)["GIT_SSH_COMMAND"]; present {
		t.Errorf("GIT_SSH_COMMAND = %q reached git in a repository that sets core.sshCommand", got)
	}
}

// git clone reads config again once the new repository exists, so a
// conditional include can name an ssh command for it that nothing run before
// the clone can see.
func TestGitRunner_CloneKeepsAnSSHCommandFromAConditionalInclude(t *testing.T) {
	noSSHChoice(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chosen := filepath.Join(fakeSSH(t, `echo ran > "$0.ran"; exit 255`), "ssh")
	// The default ssh, should git be told to use it: it must not reach a network.
	defaultSSH := fakeSSH(t, "exit 255")
	t.Setenv("PATH", defaultSSH+string(os.PathListSeparator)+os.Getenv("PATH"))

	included := filepath.Join(t.TempDir(), "work.gitconfig")
	if err := os.WriteFile(included, []byte("[core]\n\tsshCommand = "+chosen+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	globalGitConfig(t, "[includeIf \"gitdir:"+parent+"/\"]\n\tpath = "+included+"\n")

	const url = "ssh://git@host.invalid/repo.git"
	// Control: plain git picks the included command for a clone into parent.
	_ = exec.Command("git", "-C", parent, "clone", "--bare", "--", url, filepath.Join(parent, "control")).Run()
	if err := os.Remove(chosen + ".ran"); err != nil {
		t.Skipf("this git does not apply a conditional include during clone: %v", err)
	}

	_, err = (&GitRunner{}).Run(parent, "clone", "--bare", "--", url, filepath.Join(parent, "repo", ".bare"))
	if err == nil {
		t.Fatal("expected the clone to fail: the ssh command exits 255")
	}
	if _, statErr := os.Stat(chosen + ".ran"); statErr != nil {
		t.Errorf("the clone did not run the ssh command from the conditional include (%v); clone error: %v", statErr, err)
	}
}

// The config read is an extra git process. Commands that cannot run ssh do
// without it, and the others share one per directory.
func TestGitRunner_ReadsTheSSHCommandOncePerDirectory(t *testing.T) {
	installScriptGit(t, `echo "$3" >> "$0.log"; [ "$3" = config ] && exit 1; echo ok`)
	noSSHChoice(t)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}

	runner, dir := &GitRunner{}, t.TempDir()
	for _, command := range []string{"status", "status", "fetch", "fetch", "push"} {
		if _, err := runner.Run(dir, command); err != nil {
			t.Fatal(err)
		}
	}

	log, err := os.ReadFile(gitPath + ".log")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(strings.Fields(string(log)), " "), "status status config fetch fetch push"; got != want {
		t.Errorf("git ran [%s], want [%s]", got, want)
	}
}

// A config that cannot be read may hold an ssh command, so the answer is not
// "none chosen".
func TestGitRunner_LeavesSSHAloneWhenTheConfigCannotBeRead(t *testing.T) {
	installScriptGit(t, `[ "$3" = config ] && { echo "fatal: bad config line 1" >&2; exit 128; }; env`)
	noSSHChoice(t)

	out, err := (&GitRunner{}).Run(t.TempDir(), "fetch")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "GIT_SSH_COMMAND=") {
		t.Error("GIT_SSH_COMMAND reached git although core.sshCommand could not be read")
	}
}

func TestGitRunner_ExplainsABatchModeSSHFailure(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   []string
	}{
		{
			name:   "host key not accepted yet",
			stderr: "Host key verification failed.",
			want:   []string{"Host key verification failed", "sentei runs ssh without prompts", "by hand", "accept"},
		},
		{
			name:   "key needs a passphrase",
			stderr: "git@host.invalid: Permission denied (publickey).",
			want:   []string{"Permission denied (publickey)", "sentei runs ssh without prompts", "passphrase", "ssh-add"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noSSHChoice(t)
			// Fails the way ssh does only when it was started in batch mode.
			ssh := fakeSSH(t, `case " $* " in *" -oBatchMode=yes "*) echo "`+tt.stderr+`" >&2;; *) echo "not in batch mode: $*" >&2;; esac; exit 255`)
			t.Setenv("PATH", ssh+string(os.PathListSeparator)+os.Getenv("PATH"))

			_, err := (&GitRunner{}).Run(t.TempDir(), "ls-remote", "ssh://git@host.invalid/repo.git")
			if err == nil {
				t.Fatal("expected ls-remote to fail")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}

func TestGitRunner_DoesNotExplainTheUsersOwnSSHCommand(t *testing.T) {
	noSSHChoice(t)
	t.Setenv("GIT_SSH_COMMAND", filepath.Join(fakeSSH(t, `echo "Host key verification failed." >&2; exit 255`), "ssh"))

	_, err := (&GitRunner{}).Run(t.TempDir(), "ls-remote", "ssh://git@host.invalid/repo.git")
	if err == nil || !strings.Contains(err.Error(), "Host key verification failed") {
		t.Fatalf("error = %v, want the ssh failure", err)
	}
	if strings.Contains(err.Error(), "sentei runs ssh") {
		t.Errorf("error = %q: sentei did not choose this ssh command and must not explain it", err)
	}
}
