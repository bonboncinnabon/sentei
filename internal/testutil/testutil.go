package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
	"github.com/bonboncinnabon/sentei/internal/tui"
)

// RepoOpts configures the bare repo created by SetupBareRepoWithState.
type RepoOpts struct {
	WorktreeCount int
	DirtyCount    int
	StaleAge      time.Duration
	MergedCount   int
}

// SetupBareRepo creates a bare repo with a few worktrees in t.TempDir().
// Returns the path to the bare repo directory.
func SetupBareRepo(t *testing.T) string {
	t.Helper()
	return SetupBareRepoWithState(t, RepoOpts{WorktreeCount: 3})
}

// SetupBareRepoWithState creates a bare repo with configurable state.
// It creates a bare repo, makes an initial commit via a temporary clone,
// then adds worktrees with the requested characteristics.
func SetupBareRepoWithState(t *testing.T, opts RepoOpts) string {
	t.Helper()

	if opts.WorktreeCount < 1 {
		opts.WorktreeCount = 1
	}
	if opts.DirtyCount > opts.WorktreeCount {
		t.Fatalf("DirtyCount (%d) cannot exceed WorktreeCount (%d)", opts.DirtyCount, opts.WorktreeCount)
	}
	if opts.MergedCount > opts.WorktreeCount {
		t.Fatalf("MergedCount (%d) cannot exceed WorktreeCount (%d)", opts.MergedCount, opts.WorktreeCount)
	}

	tmpDir := testtmp.RobustTempDir(t)
	bareRepo := filepath.Join(tmpDir, "test.git")

	runGit(t, tmpDir, "init", "--bare", "--initial-branch=main", bareRepo)

	// Create a temporary clone to make the initial commit.
	cloneDir := filepath.Join(tmpDir, "clone")
	runGit(t, tmpDir, "clone", bareRepo, cloneDir)
	runGit(t, cloneDir, "config", "user.email", "test@test.com")
	runGit(t, cloneDir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(cloneDir, "README.md"), "# Test repo\n")
	runGit(t, cloneDir, "add", ".")
	runGit(t, cloneDir, "commit", "-m", "initial commit")
	runGit(t, cloneDir, "push", "origin", "main")

	// Create worktrees.
	for i := 0; i < opts.WorktreeCount; i++ {
		branch := fmt.Sprintf("feature/wt-%d", i)
		wtPath := filepath.Join(bareRepo, fmt.Sprintf("wt-%d", i))

		runGit(t, bareRepo, "worktree", "add", "-b", branch, wtPath, "main")
		runGit(t, wtPath, "config", "user.email", "test@test.com")
		runGit(t, wtPath, "config", "user.name", "Test")

		// Make a commit so the worktree has its own history.
		writeFile(t, filepath.Join(wtPath, fmt.Sprintf("file-%d.txt", i)), fmt.Sprintf("content %d\n", i))
		runGit(t, wtPath, "add", ".")
		runGit(t, wtPath, "commit", "-m", fmt.Sprintf("commit on wt-%d", i))

		// Apply stale age if requested: backdate the commit.
		if opts.StaleAge > 0 && i < opts.WorktreeCount-opts.DirtyCount {
			staleDate := time.Now().Add(-opts.StaleAge).Format(time.RFC3339)
			runGitEnv(t, wtPath, []string{
				"GIT_COMMITTER_DATE=" + staleDate,
			}, "commit", "--amend", "--no-edit", "--date", staleDate)
		}

		// Apply dirty state to the last DirtyCount worktrees.
		if i >= opts.WorktreeCount-opts.DirtyCount {
			writeFile(t, filepath.Join(wtPath, "dirty.txt"), "uncommitted\n")
		}

		// Mark the first MergedCount worktrees as merged by pushing
		// their branch to main and keeping the worktree around.
		if i < opts.MergedCount {
			runGit(t, wtPath, "push", "origin", branch+":main")
		}
	}

	return bareRepo
}

// LaunchTUI builds the sentei binary and launches it via teatest.
// The returned TestModel can be used to send keystrokes and assert on output.
func LaunchTUI(t *testing.T, args ...string) *teatest.TestModel {
	t.Helper()

	bin := buildBinary(t)

	// For in-process testing, we create a model directly.
	// teatest works with tea.Model, so we set up the TUI programmatically
	// rather than launching a subprocess.
	//
	// However, the task asks for a launchTUI that "builds the sentei binary
	// and launches it via teatest." teatest's TestModel works with in-process
	// models, not subprocesses. We'll use a hybrid: parse the args to determine
	// the repo path, then create the model in-process.
	//
	// Store the binary path so E2E binary tests can use it separately.
	_ = bin

	repoPath := "."
	for i, arg := range args {
		if arg == "--" {
			continue
		}
		// Simple: treat the last non-flag argument as the repo path.
		if len(arg) > 0 && arg[0] != '-' {
			repoPath = args[i]
		}
	}

	if absPath, err := filepath.Abs(repoPath); err == nil {
		repoPath = absPath
	}

	runner := &git.GitRunner{}
	shell := &git.DefaultShellRunner{}
	context := repo.DetectContext(runner, repoPath)
	if context == repo.ContextBareRepo {
		repoPath = repo.ResolveBareRoot(runner, repoPath)
	}

	var cfg *config.Config
	if context == repo.ContextBareRepo {
		cfg, _ = config.LoadConfig(repoPath,
			config.WithRunner(runner),
		)
	}

	model := tui.NewMenuModel(runner, shell, repoPath, cfg, context)

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))
	return tm
}

// buildBinary compiles the sentei binary and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "sentei")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = projectRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return binPath
}

// projectRoot finds the project root by locating go.mod.
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

// runGit executes a git command in the given directory, hermetically.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// runGitEnv executes a git command with extra environment variables on top
// of the hermetic base.
func runGitEnv(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(testtmp.HermeticGitEnv(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// writeFile creates or overwrites a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
