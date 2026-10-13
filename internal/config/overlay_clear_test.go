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

const defaultPnpmInstall = "pnpm install --frozen-lockfile --prefer-offline"

func TestLoadConfig_EmptyCommandClearsInheritedInstall(t *testing.T) {
	for _, command := range []string{`""`, `''`} {
		cfg, err := loadWithOverlays(t, "", "ecosystems:\n  - name: pnpm\n    install:\n      command: "+command+"\n")
		if err != nil {
			t.Fatalf("LoadConfig() error: %v", err)
		}

		pnpm := ecosystemNamed(t, cfg, "pnpm")
		if pnpm.Install.Command != "" {
			t.Errorf("command: %s left pnpm install = %q, want none: the repo config wrote an empty command", command, pnpm.Install.Command)
		}
		if !pnpm.IsEnabled() || len(pnpm.EnvFiles) == 0 || len(pnpm.Detect.Files) == 0 {
			t.Errorf("pnpm = %+v, want the keys the overlay did not name left alone", pnpm)
		}
		if yarn := ecosystemNamed(t, cfg, "yarn"); yarn.Install.Command == "" {
			t.Error("yarn install was cleared by an overlay that only named pnpm")
		}
	}
}

// The empty command is found in a second decode of the file and matched to
// the first by position. An entry the decoder drops, or one that carries
// other keys, must not shift the match.
func TestLoadConfig_EmptyCommandClearsOnlyTheEntryThatWritesIt(t *testing.T) {
	cfg, err := loadWithOverlays(t, "", `ecosystems:
  -
  - name: yarn
    install:
      parallel: true
  - name: pnpm
    install:
      command: ""
  - name: npm
    env_files: []
`)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if got := ecosystemNamed(t, cfg, "pnpm").Install.Command; got != "" {
		t.Errorf("pnpm install = %q, want none: the repo config wrote an empty command", got)
	}
	for _, name := range []string{"yarn", "npm"} {
		if ecosystemNamed(t, cfg, name).Install.Command == "" {
			t.Errorf("%s install was cleared by the empty command written for pnpm", name)
		}
	}
}

func TestLoadConfig_AbsentOrNullCommandKeepsInheritedInstall(t *testing.T) {
	for _, overlay := range []string{
		"ecosystems:\n  - name: pnpm\n    env_files: []\n",
		"ecosystems:\n  - name: pnpm\n    install: {}\n",
		"ecosystems:\n  - name: pnpm\n    install:\n",
		"ecosystems:\n  - name: pnpm\n    install:\n      command:\n",
		"ecosystems:\n  - name: pnpm\n    install:\n      command: ~\n",
		// A file written for an earlier version: the retired key is skipped.
		"ecosystems:\n  - name: pnpm\n    install:\n      parallel: true\n",
	} {
		cfg, err := loadWithOverlays(t, "", overlay)
		if err != nil {
			t.Fatalf("LoadConfig(%q) error: %v", overlay, err)
		}
		if got := ecosystemNamed(t, cfg, "pnpm").Install.Command; got != defaultPnpmInstall {
			t.Errorf("overlay %q left pnpm install = %q, want the built-in %q", overlay, got, defaultPnpmInstall)
		}
	}
}

func TestLoadConfig_EmptyCommandIsDecidedLayerByLayer(t *testing.T) {
	overlay := func(command string) string {
		return "ecosystems:\n  - name: pnpm\n    install:\n      command: " + command + "\n"
	}
	tests := []struct {
		name, global, repo, want string
	}{
		{name: "repo clears what global set", global: overlay("pnpm i"), repo: overlay(`""`), want: ""},
		{name: "repo sets what global cleared", global: overlay(`""`), repo: overlay("pnpm i"), want: "pnpm i"},
		{name: "repo is silent after global cleared", global: overlay(`""`), repo: "ecosystems:\n  - name: pnpm\n    enabled: true\n", want: ""},
		{name: "global clears, no repo config", global: overlay(`""`), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadWithOverlays(t, tt.global, tt.repo)
			if err != nil {
				t.Fatalf("LoadConfig() error: %v", err)
			}
			if got := ecosystemNamed(t, cfg, "pnpm").Install.Command; got != tt.want {
				t.Errorf("pnpm install = %q, want %q", got, tt.want)
			}
		})
	}
}
