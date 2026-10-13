package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/cli"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
	"github.com/bonboncinnabon/sentei/internal/testutil/layouts"
)

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = testtmp.HermeticGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, out)
	}
}

// newBareRepo builds, under t.TempDir(), a bare repository with one commit on
// main. It has no remote, so a clean worktree in it is not at risk.
func newBareRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "repo.git")
	seed := filepath.Join(root, "seed")
	mustGit(t, root, "init", "--bare", "--initial-branch=main", bare)
	mustGit(t, root, "clone", bare, seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, seed, "add", ".")
	mustGit(t, seed, "commit", "-m", "initial commit")
	mustGit(t, seed, "push", "origin", "HEAD:main")
	return bare
}

// addWorktree checks out a new branch in a worktree inside the bare repo and
// returns the worktree's path.
func addWorktree(t *testing.T, bare, branch string) string {
	t.Helper()
	path := filepath.Join(bare, branch)
	mustGit(t, bare, "worktree", "add", "-b", branch, path, "main")
	return path
}

// captureStdout runs fn and returns what it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	fn()
	w.Close()
	return <-done
}

// buildSentei compiles the binary into a temp dir and returns its path.
func buildSentei(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "sentei")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

// The playground's temp repo is removed by a deferred cleanup, which os.Exit
// skips: every exit path after setup has to return instead.
func TestPlaygroundDryRun_RemovesTempRepo(t *testing.T) {
	bin := buildSentei(t)
	tmp := t.TempDir()

	cmd := exec.Command(bin, "--playground", "--dry-run")
	cmd.Dir = t.TempDir()
	cmd.Env = append(testtmp.HermeticGitEnv(), "TMPDIR="+tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei --playground --dry-run failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "feature/wip") {
		t.Errorf("expected the playground worktree summary, got:\n%s", out)
	}
	leftovers, err := filepath.Glob(filepath.Join(tmp, "sentei-playground-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Errorf("playground repo left behind: %v", leftovers)
	}
}

// Interactive decision commands used to detect the repository from the
// current directory and ignore the positional path, so
// `sentei cleanup /path/repoB` opened (and cleaned) the repository in the cwd.
func TestParseDecision_PositionalPathSelectsRepository(t *testing.T) {
	other := t.TempDir()
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{"cleanup with path", "cleanup", []string{"--mode", "aggressive", other}, other},
		{"create with path", "create", []string{"--branch", "x", "--base", "main", other}, other},
		{"remove with path", "remove", []string{"--all", other}, other},
		{"migrate with path", "migrate", []string{other}, other},
		{"remove without path", "remove", []string{"--all"}, cwd},
		{"clone takes no repo path", "clone", []string{"--url", "git@example.com:a/b.git"}, cwd},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseDecision(tt.command, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if parsed.repoPath != tt.want {
				t.Errorf("repoPath = %q, want %q", parsed.repoPath, tt.want)
			}
		})
	}
}

func TestParseDecision_ResolvesRelativePath(t *testing.T) {
	parsed, err := parseDecision("migrate", []string{"."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(parsed.repoPath) {
		t.Errorf("repoPath %q must be absolute before the migrate flow uses it", parsed.repoPath)
	}
}

func TestParseDecision_ReturnsFlagErrors(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
	}{
		{"remove: argument after the repo path", "remove", []string{"--all", "/repo", "--dry-run"}},
		{"cleanup: argument after the repo path", "cleanup", []string{"--mode", "safe", "/repo", "--dry-run"}},
		{"create: argument after the repo path", "create", []string{"--branch", "x", "/repo", "--copy-env"}},
		{"migrate: argument after the repo path", "migrate", []string{"/repo", "--delete-backup"}},
		{"cleanup: invalid mode", "cleanup", []string{"--mode", "reckless"}},
		{"remove: invalid stale duration", "remove", []string{"--stale", "soon"}},
		{"clone: unknown flag", "clone", []string{"--depth", "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseDecision(tt.command, tt.args)
			if err == nil {
				t.Fatalf("expected an error, got %+v", parsed)
			}
			if parsed.repoPath != "" {
				t.Errorf("a failed parse must not name a repository, got %q", parsed.repoPath)
			}
		})
	}
}

// -h must surface as flag.ErrHelp so main exits 0 instead of reporting it.
func TestParseDecision_HelpIsErrHelp(t *testing.T) {
	for _, command := range []string{"cleanup", "create", "clone", "remove", "migrate"} {
		if _, err := parseDecision(command, []string{"--help"}); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("%s --help: got %v, want flag.ErrHelp", command, err)
		}
	}
}

// Each command fills its own options and nobody else's: the TUI launch
// switches on the command name and dereferences the matching pointer.
func TestParseDecision_FillsTheCommandsOptions(t *testing.T) {
	t.Run("cleanup", func(t *testing.T) {
		parsed, err := parseDecision("cleanup", []string{"--mode", "aggressive", "--dry-run"})
		if err != nil {
			t.Fatal(err)
		}
		if parsed.cleanup == nil || parsed.cleanup.Mode != cleanup.ModeAggressive || !parsed.cleanup.DryRun {
			t.Errorf("cleanup options = %+v", parsed.cleanup)
		}
		if parsed.create != nil || parsed.clone != nil || parsed.remove != nil || parsed.migrate != nil {
			t.Errorf("only the cleanup options may be set: %+v", parsed)
		}
	})
	t.Run("create", func(t *testing.T) {
		parsed, err := parseDecision("create", []string{"--branch", "feature/x", "--base", "main", "--copy-env"})
		if err != nil {
			t.Fatal(err)
		}
		if parsed.create == nil || parsed.create.Branch != "feature/x" || parsed.create.Base != "main" || !parsed.create.CopyEnv {
			t.Errorf("create options = %+v", parsed.create)
		}
	})
	t.Run("clone", func(t *testing.T) {
		parsed, err := parseDecision("clone", []string{"--url", "git@example.com:a/b.git", "--name", "b"})
		if err != nil {
			t.Fatal(err)
		}
		if parsed.clone == nil || parsed.clone.URL != "git@example.com:a/b.git" || parsed.clone.Name != "b" {
			t.Errorf("clone options = %+v", parsed.clone)
		}
	})
	t.Run("remove", func(t *testing.T) {
		parsed, err := parseDecision("remove", []string{"--merged", "--stale", "2w"})
		if err != nil {
			t.Fatal(err)
		}
		if parsed.remove == nil || !parsed.remove.Merged || parsed.remove.Stale == 0 || parsed.remove.All {
			t.Errorf("remove options = %+v", parsed.remove)
		}
	})
	t.Run("migrate", func(t *testing.T) {
		dir := t.TempDir()
		parsed, err := parseDecision("migrate", []string{dir})
		if err != nil {
			t.Fatal(err)
		}
		if parsed.migrate == nil || parsed.migrate.RepoPath != dir {
			t.Errorf("migrate options = %+v", parsed.migrate)
		}
	})
}

func TestRun_UnknownCommand(t *testing.T) {
	// An unknown name that is a directory opens the TUI there: run from an
	// empty directory so no entry of the source tree can match.
	t.Chdir(t.TempDir())
	err := run(buildRegistry(), []string{"integrations"})
	if !cli.IsUnknownCommand(err) {
		t.Errorf("got %v, want an unknown-command error so main prints the command list", err)
	}
}

func TestRun_EveryCommandIsRegistered(t *testing.T) {
	registry := buildRegistry()
	for _, name := range []string{"ecosystems", "clone", "create", "cleanup", "migrate", "remove"} {
		result, err := registry.Dispatch([]string{name})
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if result.Command == nil || result.Command.RunCLI == nil {
			t.Errorf("%s has no non-interactive handler", name)
		}
	}
}

func TestRun_OutputCommandRunsWithoutNonInteractive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()

	var err error
	out := captureStdout(t, func() {
		err = run(buildRegistry(), []string{"ecosystems", dir})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Ecosystems (") {
		t.Errorf("expected the ecosystem table, got:\n%s", out)
	}
}

// --non-interactive and --yes both route a decision command to its CLI
// handler, whose validation errors come back to main unchanged.
func TestRun_NonInteractiveFlagsReachTheCLIHandler(t *testing.T) {
	dir := t.TempDir()
	// clone takes no repository path and works in the current directory.
	t.Chdir(dir)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"remove --non-interactive", []string{"remove", "--non-interactive", dir}, "at least one filter required"},
		{"remove --yes", []string{"remove", "--yes", dir}, "at least one filter required"},
		{"remove -y", []string{"remove", "-y", dir}, "at least one filter required"},
		{"cleanup --yes", []string{"cleanup", "--yes", dir}, "missing required flag: --mode"},
		{"clone --non-interactive", []string{"clone", "--non-interactive"}, "missing required flag: --url"},
		{"flag after the path", []string{"remove", "--all", dir, "--dry-run", "--yes"}, "flags must come before the path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(buildRegistry(), tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

// The global flag scan takes --force out of the arguments; it has to be handed
// back to remove, or the at-risk override could never be given.
func TestRun_ForceReachesRemove(t *testing.T) {
	bare := newBareRepo(t)
	wtPath := addWorktree(t, bare, "feature")
	if err := os.WriteFile(filepath.Join(wtPath, "draft.txt"), []byte("unsaved\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := run(buildRegistry(), []string{"remove", "--all", "--non-interactive", bare})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("got %v, want the at-risk gate to refuse and name --force", err)
	}
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Fatalf("the gate must leave the worktree in place: %v", statErr)
	}

	captureStdout(t, func() {
		err = run(buildRegistry(), []string{"remove", "--all", "--yes", "--force", bare})
	})
	if err != nil {
		t.Fatalf("--force must reach remove: %v", err)
	}
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Errorf("expected %s to be removed (stat err: %v)", wtPath, statErr)
	}
}

// Without --non-interactive a decision command opens the TUI, but its flags
// are parsed first: a bad command line must fail before any terminal is needed.
func TestRun_InteractiveCommandRejectsBadFlagsBeforeTheTUI(t *testing.T) {
	bare := newBareRepo(t)
	wtPath := addWorktree(t, bare, "feature")
	// `cleanup --help` names no repository: if it ever got past flag parsing
	// it would open the TUI on the current directory.
	t.Chdir(t.TempDir())

	err := run(buildRegistry(), []string{"remove", "--all", bare, "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "flags must come before the path") {
		t.Errorf("got %v, want the trailing flag rejected", err)
	}
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Errorf("a rejected command line must not touch the worktree: %v", statErr)
	}

	if err := run(buildRegistry(), []string{"cleanup", "--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("cleanup --help: got %v, want flag.ErrHelp", err)
	}
}

func TestRunRoot_Version(t *testing.T) {
	var err error
	out := captureStdout(t, func() {
		err = run(buildRegistry(), []string{"--version"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "sentei dev (none, unknown)\n"; out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestRunRoot_DryRun(t *testing.T) {
	t.Run("lists worktrees", func(t *testing.T) {
		bare := newBareRepo(t)
		addWorktree(t, bare, "feature")

		var err error
		out := captureStdout(t, func() {
			err = run(buildRegistry(), []string{"--dry-run", bare})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, want := range []string{"STATUS", "feature", "initial commit"} {
			if !strings.Contains(out, want) {
				t.Errorf("summary missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("no worktrees", func(t *testing.T) {
		bare := newBareRepo(t)

		var err error
		out := captureStdout(t, func() {
			err = run(buildRegistry(), []string{"--dry-run", bare})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "No worktrees found") {
			t.Errorf("got:\n%s", out)
		}
	})

	t.Run("requires a bare repository", func(t *testing.T) {
		err := run(buildRegistry(), []string{"--dry-run", t.TempDir()})
		if err == nil || !strings.Contains(err.Error(), "--dry-run requires a bare repository") {
			t.Errorf("got %v", err)
		}
	})

	// `sentei <path> --dry-run` would otherwise drop the flag and open the TUI.
	t.Run("rejects a flag after the path", func(t *testing.T) {
		err := run(buildRegistry(), []string{t.TempDir(), "--dry-run"})
		if err == nil || !strings.Contains(err.Error(), "flags must come before the path") {
			t.Errorf("got %v", err)
		}
	})
}

func TestVersionFlag(t *testing.T) {
	bin := t.TempDir() + "/sentei"
	build := exec.Command("go", "build", "-ldflags", "-X main.version=v1.2.3 -X main.commit=abc1234 -X main.date=1970-01-01", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	t.Run("prints version and exits", func(t *testing.T) {
		out, err := exec.Command(bin, "--version").Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(string(out))
		want := "sentei v1.2.3 (abc1234, 1970-01-01)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("default version is dev", func(t *testing.T) {
		devBin := t.TempDir() + "/sentei"
		build := exec.Command("go", "build", "-o", devBin, ".")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build failed: %v\n%s", err, out)
		}
		out, err := exec.Command(devBin, "--version").Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(string(out))
		want := "sentei dev (none, unknown)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("version takes precedence over other flags", func(t *testing.T) {
		out, err := exec.Command(bin, "--version", "--dry-run").Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(string(out))
		want := "sentei v1.2.3 (abc1234, 1970-01-01)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// A config that fails to load used to be a warning printed just before the
// alternate screen hid it, and the TUI then ran without the config.
func TestNewMenuModel_StopsOnConfigLoadFailure(t *testing.T) {
	bare := newBareRepo(t)
	// A plain bare repository keeps its config inside itself.
	if err := os.WriteFile(filepath.Join(bare, ".sentei.yaml"), []byte("instal:\n  command: make\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := newMenuModel(&git.GitRunner{}, bare, repo.ContextBareRepo)

	if err == nil || !strings.Contains(err.Error(), "loading config") || !strings.Contains(err.Error(), "instal") {
		t.Fatalf("error = %v, want the config load failure naming the unknown key", err)
	}
}

// The summary is the same whichever path names the repository. Worktrees of a
// bare directory not named .bare used to be refused as regular repositories.
func TestRunRoot_DryRunFromEveryEntryPoint(t *testing.T) {
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		var err error
		out := captureStdout(t, func() {
			err = run(buildRegistry(), []string{"--dry-run", entry})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, want := range []string{"feature", "scratch", layouts.DefaultBranch + " [P]"} {
			if !strings.Contains(out, want) {
				t.Errorf("summary missing %q:\n%s", want, out)
			}
		}
	})
}

// The menu of a bare layout opens on the layout's root from every entry
// point, with every worktree loaded.
func TestMenu_BareLayoutFromEveryEntryPoint(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		runner := &git.GitRunner{}
		context, root := detectRepo(runner, entry)
		if context != repo.ContextBareRepo || root != layout.Root {
			t.Fatalf("detectRepo(%q) = %v, %q; want %v, %q", entry, context, root, repo.ContextBareRepo, layout.Root)
		}
		model, err := newMenuModel(runner, root, context)
		if err != nil {
			t.Fatal(err)
		}

		tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 24))
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte("Create new worktree")) && bytes.Contains(out, []byte("3 worktrees"))
		}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
		tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
		tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	})
}

// A regular clone is still offered the conversion, from its main working tree
// and from a linked worktree.
func TestMenu_RegularCloneOffersMigrate(t *testing.T) {
	regular := t.TempDir()
	mustGit(t, regular, "init", "--quiet")
	mustGit(t, regular, "commit", "--quiet", "--allow-empty", "-m", "initial commit")
	linked := filepath.Join(t.TempDir(), "linked")
	mustGit(t, regular, "worktree", "add", "--quiet", "-b", "linked", linked)

	for _, entry := range []string{regular, linked} {
		runner := &git.GitRunner{}
		context, root := detectRepo(runner, entry)
		if context != repo.ContextNonBareRepo || root != entry {
			t.Fatalf("detectRepo(%q) = %v, %q; want %v and the path unchanged", entry, context, root, repo.ContextNonBareRepo)
		}
		model, err := newMenuModel(runner, root, context)
		if err != nil {
			t.Fatal(err)
		}
		if menu := model.View().Content; !strings.Contains(menu, "Migrate to bare repository") {
			t.Errorf("menu for %s does not offer migrate:\n%s", entry, menu)
		}
	}
}
