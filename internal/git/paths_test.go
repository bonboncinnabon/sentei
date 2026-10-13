package git

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/layouts"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestWorktreeDirName(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{name: "slash replaced with dash", branch: "feature/auth", want: "feature-auth"},
		{name: "multiple slashes", branch: "bugfix/login/redirect", want: "bugfix-login-redirect"},
		{name: "no slash unchanged", branch: "hotfix", want: "hotfix"},
		{name: "trailing slash stripped", branch: "feature/", want: "feature-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreeDirName(tt.branch)
			if got != tt.want {
				t.Errorf("WorktreeDirName(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}

func TestWorktreePath(t *testing.T) {
	got := WorktreePath("/repo", "feature/auth")
	want := filepath.Join("/repo", "feature-auth")
	if got != want {
		t.Errorf("WorktreePath = %q, want %q", got, want)
	}
}

func TestCommonDir(t *testing.T) {
	tests := []struct {
		name      string
		repoPath  string
		commonDir string
		runErr    error
		want      string
		wantErr   bool
	}{
		{
			name:      "absolute sentei layout",
			repoPath:  "/repo/main",
			commonDir: "/repo/.bare",
			want:      "/repo/.bare",
		},
		{
			name:      "absolute non-sentei bare name",
			repoPath:  "/play/wt-feature",
			commonDir: "/play/repo.git",
			want:      "/play/repo.git",
		},
		{
			name:      "relative common dir joined with repo path",
			repoPath:  "/repo/main",
			commonDir: "../.bare",
			want:      "/repo/.bare",
		},
		{
			name:     "runner error propagates",
			repoPath: "/repo/main",
			runErr:   errors.New("not a git repository"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{
				Responses: map[string]mock.Response{
					tt.repoPath + ":[rev-parse --git-common-dir]": {Output: tt.commonDir, Err: tt.runErr},
				},
			}
			got, err := CommonDir(runner, tt.repoPath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CommonDir(%q) error = nil, want error", tt.repoPath)
				}
				return
			}
			if err != nil {
				t.Fatalf("CommonDir(%q) unexpected error: %v", tt.repoPath, err)
			}
			if got != tt.want {
				t.Errorf("CommonDir(%q) = %q, want %q", tt.repoPath, got, tt.want)
			}
		})
	}
}

// Worktree placement, the config file and repository-wide commands all use
// this root, so it has to be the same wherever sentei is started.
func TestRepoRoot_SameFromEveryEntryPoint(t *testing.T) {
	layouts.EachEntry(t, func(t *testing.T, layout layouts.Layout, entry string) {
		if got := RepoRoot(&GitRunner{}, entry); got != layout.Root {
			t.Errorf("RepoRoot(%q) = %q, want %q", entry, got, layout.Root)
		}
		// A command given no path asks about ".". Inside a bare directory
		// git answers "." too, which hid the directory's name, so .bare was
		// taken for its own root.
		t.Chdir(entry)
		if got := RepoRoot(&GitRunner{}, "."); got != layout.Root {
			t.Errorf("RepoRoot(\".\") in %s = %q, want %q", entry, got, layout.Root)
		}
	})
}

func TestRepoRoot(t *testing.T) {
	runner := &GitRunner{}
	run := func(dir string, args ...string) {
		t.Helper()
		if _, err := runner.Run(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	writePointer := func(dir, target string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: "+target+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tempDir := func() string {
		t.Helper()
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return dir
	}

	regular := tempDir()
	run(regular, "init", "--quiet")
	run(regular, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "--quiet", "--allow-empty", "-m", "init")
	linked := filepath.Join(tempDir(), "linked")
	run(regular, "worktree", "add", "--quiet", "-b", "linked", linked)
	nested := filepath.Join(regular, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	// A container built by hand: the bare directory has another name, and the
	// .git pointer beside it is what makes the parent the container.
	container := tempDir()
	named := filepath.Join(container, "repo.git")
	run(container, "init", "--quiet", "--bare", named)
	writePointer(container, "./repo.git")

	// The same shape, but the pointer leads to another repository: commands
	// run in the parent would act on that one.
	foreignParent := tempDir()
	besideForeignPointer := filepath.Join(foreignParent, "project.git")
	run(foreignParent, "init", "--quiet", "--bare", besideForeignPointer)
	other := filepath.Join(tempDir(), "other.git")
	run(foreignParent, "init", "--quiet", "--bare", other)
	writePointer(foreignParent, other)

	// A pointer that leads nowhere.
	brokenParent := tempDir()
	besideBrokenPointer := filepath.Join(brokenParent, "project.git")
	run(brokenParent, "init", "--quiet", "--bare", besideBrokenPointer)
	writePointer(brokenParent, "./gone.git")

	outside := tempDir()

	tests := []struct {
		name     string
		repoPath string
		want     string
	}{
		{"regular repository", regular, regular},
		{"directory inside a regular repository", nested, regular},
		{"linked worktree of a regular repository", linked, regular},
		{"named bare directory with a pointer beside it", named, container},
		{"container of a named bare directory", container, container},
		{"bare directory beside a pointer to another repository", besideForeignPointer, besideForeignPointer},
		{"bare directory beside a broken pointer", besideBrokenPointer, besideBrokenPointer},
		{"outside a repository", outside, outside},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RepoRoot(runner, tt.repoPath); got != tt.want {
				t.Errorf("RepoRoot(%q) = %q, want %q", tt.repoPath, got, tt.want)
			}
		})
	}
}
