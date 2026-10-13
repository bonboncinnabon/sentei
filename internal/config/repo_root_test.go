package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// writeRepoConfig writes a .sentei.yaml into dir that adds one ecosystem.
func writeRepoConfig(t *testing.T, dir, ecosystem string) {
	t.Helper()
	content := "ecosystems:\n  - name: " + ecosystem + "\n    detect:\n      files: [marker]\n"
	if err := os.WriteFile(filepath.Join(dir, ".sentei.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasEcosystem(cfg *Config, name string) bool {
	for _, e := range cfg.Ecosystems {
		if e.Name == name {
			return true
		}
	}
	return false
}

func TestLoadConfig_RepoConfigLocation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	runner := &git.GitRunner{}
	mustGit := func(dir string, args ...string) {
		t.Helper()
		if _, err := runner.Run(dir, args...); err != nil {
			t.Fatal(err)
		}
	}

	// Two plain bare repositories side by side, as in ~/code/a.git and b.git.
	parent := t.TempDir()
	plainBare := filepath.Join(parent, "a.git")
	mustGit(parent, "init", "--bare", plainBare)
	plainBareWorktree := filepath.Join(parent, "a-main")
	mustGit(plainBare, "worktree", "add", "--orphan", "-b", "main", plainBareWorktree)
	writeRepoConfig(t, parent, "from-parent")
	writeRepoConfig(t, plainBare, "from-bare-repo")

	// sentei's layout: a container holding .bare, a .git pointer and worktrees.
	container := t.TempDir()
	mustGit(container, "init", "--bare", filepath.Join(container, ".bare"))
	if err := os.WriteFile(filepath.Join(container, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(container, "main")
	mustGit(container, "worktree", "add", "--orphan", "-b", "main", worktree)
	writeRepoConfig(t, container, "from-container")

	// The same layout built by hand, with the bare directory under another name.
	namedContainer := t.TempDir()
	mustGit(namedContainer, "init", "--bare", filepath.Join(namedContainer, "repo.git"))
	if err := os.WriteFile(filepath.Join(namedContainer, ".git"), []byte("gitdir: ./repo.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRepoConfig(t, namedContainer, "from-named-container")

	// A .bare container nobody wrote a .git pointer for.
	bareOnly := t.TempDir()
	mustGit(bareOnly, "init", "--bare", filepath.Join(bareOnly, ".bare"))
	bareOnlyWorktree := filepath.Join(bareOnly, "main")
	mustGit(filepath.Join(bareOnly, ".bare"), "worktree", "add", "--orphan", "-b", "main", bareOnlyWorktree)
	writeRepoConfig(t, bareOnly, "from-bare-only-container")

	regular := t.TempDir()
	mustGit(regular, "init")
	writeRepoConfig(t, regular, "from-regular")

	tests := []struct {
		name     string
		repoPath string
		want     string
		wantNot  string
	}{
		{"plain bare repository reads its own directory", plainBare, "from-bare-repo", "from-parent"},
		{"plain bare repository with a trailing slash", plainBare + string(filepath.Separator), "from-bare-repo", "from-parent"},
		{"worktree of a plain bare repository", plainBareWorktree, "from-bare-repo", "from-parent"},
		{"container", container, "from-container", ""},
		{"worktree inside the container", worktree, "from-container", ""},
		{"the .bare directory inside the container", filepath.Join(container, ".bare"), "from-container", ""},
		{"container whose bare directory has another name", namedContainer, "from-named-container", ""},
		{"worktree of a .bare container without a .git pointer", bareOnlyWorktree, "from-bare-only-container", ""},
		{"regular repository", regular, "from-regular", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(tt.repoPath)
			if err != nil {
				t.Fatalf("LoadConfig() error: %v", err)
			}
			if !hasEcosystem(cfg, tt.want) {
				t.Errorf("config for %s is missing %q", tt.repoPath, tt.want)
			}
			if tt.wantNot != "" && hasEcosystem(cfg, tt.wantNot) {
				t.Errorf("config for %s picked up %q, a file shared with sibling repositories", tt.repoPath, tt.wantNot)
			}
		})
	}
}
