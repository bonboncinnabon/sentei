package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadEmbeddedDefaults(t *testing.T) {
	cfg, err := loadEmbeddedDefaults()
	if err != nil {
		t.Fatalf("loadEmbeddedDefaults() error: %v", err)
	}

	if len(cfg.Ecosystems) != 16 {
		t.Fatalf("expected 16 ecosystems, got %d", len(cfg.Ecosystems))
	}

	if cfg.Ecosystems[0].Name != "pnpm" {
		t.Errorf("first ecosystem: got %q, want %q", cfg.Ecosystems[0].Name, "pnpm")
	}

	for i, e := range cfg.Ecosystems {
		if e.Name == "" {
			t.Errorf("Ecosystems[%d].Name is empty", i)
		}
		if len(e.Detect.Files) == 0 {
			t.Errorf("Ecosystems[%d] (%s): Detect.Files is empty", i, e.Name)
		}
		if e.Install.Command == "" {
			t.Errorf("Ecosystems[%d] (%s): Install.Command is empty", i, e.Name)
		}
	}

	// Installs run once at the worktree root. A per-member command such as
	// `pnpm install --filter <dir>` selects no project and installs nothing.
	for _, e := range cfg.Ecosystems {
		if strings.Contains(e.Install.Command, "{dir}") || strings.Contains(e.Install.Command, "--filter") {
			t.Errorf("%s: install command %q must be a root install", e.Name, e.Install.Command)
		}
	}
	// pnpm reads from the shared store and never rewrites the lockfile of a
	// fresh worktree.
	if got, want := cfg.Ecosystems[0].Install.Command, "pnpm install --frozen-lockfile --prefer-offline"; got != want {
		t.Errorf("pnpm install command: got %q, want %q", got, want)
	}

	wantNames := []string{
		"pnpm", "yarn", "npm", "bun", "cargo", "go", "uv", "poetry",
		"pip", "ruby", "php", "dotnet", "elixir", "swift", "dart", "deno",
	}
	for i, want := range wantNames {
		if cfg.Ecosystems[i].Name != want {
			t.Errorf("Ecosystems[%d].Name: got %q, want %q", i, cfg.Ecosystems[i].Name, want)
		}
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func assertConfigEqual(t *testing.T, got, want Config) {
	t.Helper()

	if len(got.Ecosystems) != len(want.Ecosystems) {
		t.Fatalf("Ecosystems length: got %d, want %d", len(got.Ecosystems), len(want.Ecosystems))
	}
	for i, gotE := range got.Ecosystems {
		wantE := want.Ecosystems[i]
		if gotE.Name != wantE.Name {
			t.Errorf("Ecosystems[%d].Name: got %q, want %q", i, gotE.Name, wantE.Name)
		}
		switch {
		case gotE.Enabled == nil && wantE.Enabled == nil:
			// both nil — ok
		case gotE.Enabled == nil || wantE.Enabled == nil:
			t.Errorf("Ecosystems[%d].Enabled: got %v, want %v", i, gotE.Enabled, wantE.Enabled)
		case *gotE.Enabled != *wantE.Enabled:
			t.Errorf("Ecosystems[%d].Enabled: got %v, want %v", i, *gotE.Enabled, *wantE.Enabled)
		}
		if len(gotE.Detect.Files) != len(wantE.Detect.Files) {
			t.Errorf("Ecosystems[%d].Detect.Files length: got %d, want %d", i, len(gotE.Detect.Files), len(wantE.Detect.Files))
		} else {
			for j, f := range gotE.Detect.Files {
				if f != wantE.Detect.Files[j] {
					t.Errorf("Ecosystems[%d].Detect.Files[%d]: got %q, want %q", i, j, f, wantE.Detect.Files[j])
				}
			}
		}
		if gotE.Install.Command != wantE.Install.Command {
			t.Errorf("Ecosystems[%d].Install.Command: got %q, want %q", i, gotE.Install.Command, wantE.Install.Command)
		}
		if len(gotE.EnvFiles) != len(wantE.EnvFiles) {
			t.Errorf("Ecosystems[%d].EnvFiles length: got %d, want %d", i, len(gotE.EnvFiles), len(wantE.EnvFiles))
		} else {
			for j, f := range gotE.EnvFiles {
				if f != wantE.EnvFiles[j] {
					t.Errorf("Ecosystems[%d].EnvFiles[%d]: got %q, want %q", i, j, f, wantE.EnvFiles[j])
				}
			}
		}
	}
}

func TestConfigUnmarshal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Config
	}{
		{
			name: "full config",
			input: `
ecosystems:
  - name: node
    enabled: true
    detect:
      files:
        - package.json
    install:
      command: npm install
      workspace_detect: package.json
      workspace_install: npm install --workspaces
      parallel: true
    env_files:
      - .env
      - .env.local
    post_install:
      - npm run build
protected_branches:
  - main
  - develop
integrations_enabled:
  - github
  - slack
`,
			want: Config{
				Ecosystems: []EcosystemConfig{
					{
						Name:    "node",
						Enabled: boolPtr(true),
						Detect:  DetectConfig{Files: []string{"package.json"}},
						// The input above still carries the retired
						// workspace_detect, workspace_install and parallel
						// keys: existing configs must keep loading, with
						// those keys ignored.
						Install:  InstallConfig{Command: "npm install"},
						EnvFiles: []string{".env", ".env.local"},
					},
				},
			},
		},
		{
			name:  "empty config",
			input: `{}`,
			want:  Config{},
		},
		{
			name: "ecosystem with enabled false",
			input: `
ecosystems:
  - name: python
    enabled: false
    detect:
      files:
        - requirements.txt
    install:
      command: pip install -r requirements.txt
`,
			want: Config{
				Ecosystems: []EcosystemConfig{
					{
						Name:    "python",
						Enabled: boolPtr(false),
						Detect:  DetectConfig{Files: []string{"requirements.txt"}},
						Install: InstallConfig{
							Command: "pip install -r requirements.txt",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parse([]byte(tc.input))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			assertConfigEqual(t, *got, tc.want)
		})
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{"nil means enabled", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := EcosystemConfig{Enabled: tc.enabled}
			if got := e.IsEnabled(); got != tc.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEmbeddedDefaults_BunDetectsTextLockfile(t *testing.T) {
	cfg, err := loadEmbeddedDefaults()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range cfg.Ecosystems {
		if e.Name == "bun" {
			// bun >= 1.2 writes bun.lock (text); bun.lockb is the older binary form.
			if !slices.Contains(e.Detect.Files, "bun.lock") || !slices.Contains(e.Detect.Files, "bun.lockb") {
				t.Errorf("bun detect files = %v, want both bun.lock and bun.lockb", e.Detect.Files)
			}
			return
		}
	}
	t.Fatal("bun ecosystem missing from the embedded defaults")
}

func TestLoadFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid file",
			content: `
ecosystems:
  - name: node
    detect:
      files:
        - package.json
    install:
      command: npm install
`,
			wantErr: false,
		},
		{
			name:    "malformed yaml",
			content: "ecosystems: [\nunclosed",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			cfg, err := loadFile(path)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("loadFile() error: %v", err)
			}
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
		})
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	cfg, err := loadFile("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for missing file, got: %+v", cfg)
	}
}

func TestMergeEcosystems(t *testing.T) {
	base := []EcosystemConfig{
		{
			Name:    "pnpm",
			Detect:  DetectConfig{Files: []string{"pnpm-lock.yaml"}},
			Install: InstallConfig{Command: "pnpm install"},
		},
		{
			Name:    "go",
			Detect:  DetectConfig{Files: []string{"go.mod"}},
			Install: InstallConfig{Command: "go mod download"},
		},
	}

	tests := []struct {
		name    string
		overlay []EcosystemConfig
		check   func(t *testing.T, result []EcosystemConfig)
	}{
		{
			name: "add new ecosystem",
			overlay: []EcosystemConfig{
				{
					Name:    "cargo",
					Detect:  DetectConfig{Files: []string{"Cargo.toml"}},
					Install: InstallConfig{Command: "cargo fetch"},
				},
			},
			check: func(t *testing.T, result []EcosystemConfig) {
				if len(result) != 3 {
					t.Fatalf("expected 3 ecosystems, got %d", len(result))
				}
				last := result[2]
				if last.Name != "cargo" {
					t.Errorf("new ecosystem name: got %q, want %q", last.Name, "cargo")
				}
				if last.Source != "test" {
					t.Errorf("new ecosystem source: got %q, want %q", last.Source, "test")
				}
			},
		},
		{
			name: "override command field",
			overlay: []EcosystemConfig{
				{
					Name:    "pnpm",
					Install: InstallConfig{Command: "pnpm install --frozen-lockfile"},
				},
			},
			check: func(t *testing.T, result []EcosystemConfig) {
				if len(result) != 2 {
					t.Fatalf("expected 2 ecosystems, got %d", len(result))
				}
				pnpm := result[0]
				if pnpm.Install.Command != "pnpm install --frozen-lockfile" {
					t.Errorf("pnpm command: got %q, want %q", pnpm.Install.Command, "pnpm install --frozen-lockfile")
				}
				if pnpm.Source != "test" {
					t.Errorf("pnpm source: got %q, want %q", pnpm.Source, "test")
				}
				// Detect.Files should be preserved from base.
				if len(pnpm.Detect.Files) == 0 {
					t.Error("expected detect.files to be preserved from base")
				}
			},
		},
		{
			name: "disable ecosystem",
			overlay: []EcosystemConfig{
				{
					Name:    "go",
					Enabled: boolPtr(false),
				},
			},
			check: func(t *testing.T, result []EcosystemConfig) {
				for _, e := range result {
					if e.Name == "go" {
						if e.IsEnabled() {
							t.Error("expected go to be disabled")
						}
						return
					}
				}
				t.Error("go ecosystem not found in result")
			},
		},
		{
			name:    "nil overlay returns base unchanged",
			overlay: nil,
			check: func(t *testing.T, result []EcosystemConfig) {
				if len(result) != len(base) {
					t.Fatalf("expected %d ecosystems, got %d", len(base), len(result))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeEcosystems(base, tc.overlay, "test")
			tc.check(t, result)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Ecosystems: []EcosystemConfig{
					{
						Name:   "go",
						Detect: DetectConfig{Files: []string{"go.mod"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			cfg: Config{
				Ecosystems: []EcosystemConfig{
					{
						Name:   "",
						Detect: DetectConfig{Files: []string{"go.mod"}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no detect files",
			cfg: Config{
				Ecosystems: []EcosystemConfig{
					{
						Name:   "go",
						Detect: DetectConfig{Files: nil},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(&tc.cfg)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Set up a fake XDG_CONFIG_HOME with a global config that overrides pnpm command.
	xdgDir := t.TempDir()
	senteiConfigDir := filepath.Join(xdgDir, "sentei")
	if err := os.MkdirAll(senteiConfigDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	globalConfig := `
ecosystems:
  - name: pnpm
    install:
      command: pnpm install --frozen-lockfile
`
	if err := os.WriteFile(filepath.Join(senteiConfigDir, "config.yaml"), []byte(globalConfig), 0o644); err != nil {
		t.Fatalf("WriteFile global config: %v", err)
	}

	// A repo config that still carries a retired key must keep loading.
	repoDir := t.TempDir()
	repoConfig := `
integrations_enabled:
  - code-review-graph
  - cocoindex-code
`
	if err := os.WriteFile(filepath.Join(repoDir, ".sentei.yaml"), []byte(repoConfig), 0o644); err != nil {
		t.Fatalf("WriteFile repo config: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	cfg, err := LoadConfig(repoDir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	// Check that pnpm command was overridden.
	var pnpm *EcosystemConfig
	for i := range cfg.Ecosystems {
		if cfg.Ecosystems[i].Name == "pnpm" {
			pnpm = &cfg.Ecosystems[i]
			break
		}
	}
	if pnpm == nil {
		t.Fatal("pnpm ecosystem not found in config")
	}
	if pnpm.Install.Command != "pnpm install --frozen-lockfile" {
		t.Errorf("pnpm command: got %q, want %q", pnpm.Install.Command, "pnpm install --frozen-lockfile")
	}
}

func TestLoadConfig_EmbeddedOnly(t *testing.T) {
	// Point XDG_CONFIG_HOME to an empty dir and use a repo dir with no .sentei.yaml.
	xdgDir := t.TempDir()
	repoDir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	cfg, err := LoadConfig(repoDir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if len(cfg.Ecosystems) != 16 {
		t.Fatalf("expected 16 ecosystems from defaults, got %d", len(cfg.Ecosystems))
	}
}
