package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadWithOverlays loads the embedded defaults plus the given global and repo
// config files (an empty string means no file).
func loadWithOverlays(t *testing.T, global, repo string) (*Config, error) {
	t.Helper()
	xdg, repoDir := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	write := func(path, content string) {
		if content == "" {
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(xdg, "sentei", "config.yaml"), global)
	write(filepath.Join(repoDir, ".sentei.yaml"), repo)
	return LoadConfig(repoDir)
}

func ecosystemNamed(t *testing.T, cfg *Config, name string) EcosystemConfig {
	t.Helper()
	for _, e := range cfg.Ecosystems {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("ecosystem %q not found", name)
	return EcosystemConfig{}
}

func TestLoadConfig_EmptyListClearsInheritedValue(t *testing.T) {
	cfg, err := loadWithOverlays(t, "", "ecosystems:\n  - name: pnpm\n    env_files: []\n")
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	pnpm := ecosystemNamed(t, cfg, "pnpm")
	if len(pnpm.EnvFiles) != 0 {
		t.Errorf("pnpm env_files = %v, want none: the repo config set an empty list", pnpm.EnvFiles)
	}
	if pnpm.Install.Command == "" || len(pnpm.Detect.Files) == 0 {
		t.Errorf("pnpm = %+v, want the keys the overlay did not name left alone", pnpm)
	}
	if yarn := ecosystemNamed(t, cfg, "yarn"); len(yarn.EnvFiles) == 0 {
		t.Error("yarn env_files were cleared by an overlay that only named pnpm")
	}
}

func TestLoadConfig_AbsentOrNullListKeepsInheritedValue(t *testing.T) {
	for _, overlay := range []string{
		"ecosystems:\n  - name: pnpm\n    enabled: true\n",
		"ecosystems:\n  - name: pnpm\n    env_files:\n",
	} {
		cfg, err := loadWithOverlays(t, "", overlay)
		if err != nil {
			t.Fatalf("LoadConfig() error: %v", err)
		}
		if pnpm := ecosystemNamed(t, cfg, "pnpm"); len(pnpm.EnvFiles) == 0 {
			t.Errorf("overlay %q cleared pnpm env_files without an explicit []", overlay)
		}
	}
}

func TestLoadConfig_EmptyWorktreeFilesClearsGlobalList(t *testing.T) {
	global := "worktree_files:\n  - source: templates/base\n    destination: .config/base\n"

	cfg, err := loadWithOverlays(t, global, "worktree_files: []\n")
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(cfg.WorktreeFiles) != 0 {
		t.Errorf("worktree_files = %v, want none: the repo config set an empty list", cfg.WorktreeFiles)
	}

	cfg, err = loadWithOverlays(t, global, "ecosystems: []\n")
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(cfg.WorktreeFiles) != 1 {
		t.Errorf("worktree_files = %v, want the global list: the repo config did not name the key", cfg.WorktreeFiles)
	}
}

func TestLoadConfig_ClearedDetectFilesIsAnError(t *testing.T) {
	_, err := loadWithOverlays(t, "", "ecosystems:\n  - name: pnpm\n    detect:\n      files: []\n")
	if err == nil {
		t.Fatal("an enabled ecosystem with no detect files must be rejected, not silently keep the default")
	}
	for _, want := range []string{`"pnpm"`, "detect.files", "enabled: false"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
}
