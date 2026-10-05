# Config, Ecosystem & Integration Foundation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add config loading (embedded + global + per-repo), ecosystem detection registry, and integration definitions — the foundation layer for sentei's expansion to full worktree lifecycle manager.

**Architecture:** Three new packages (`internal/config/`, `internal/ecosystem/`, `internal/integration/`) with no changes to existing packages. Config uses `go:embed` for defaults with YAML merge. Ecosystems are config-driven; integrations are Go structs. Two new CLI subcommands (`sentei ecosystems`, `sentei integrations`) for auditability.

**Tech Stack:** Go, `gopkg.in/yaml.v3` for YAML parsing, `go:embed` for bundled defaults, `path/filepath` for glob matching, existing `git.CommandRunner` for repo detection.

**Spec:** `docs/superpowers/specs/2026-03-29-config-ecosystem-integration-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/config/config.go` | Create | `Config`, `EcosystemConfig`, and related types; `LoadConfig()`, `loadFile()`, `mergeEcosystems()`, `validate()` |
| `internal/config/config_test.go` | Create | Unit tests: merge semantics, validation, error cases |
| `internal/config/defaults/ecosystems.yaml` | Create | Embedded default ecosystem definitions (16 ecosystems) |
| `internal/config/testdata/` | Create | YAML fixtures for merge/validation tests |
| `internal/ecosystem/ecosystem.go` | Create | `Ecosystem` type, `Registry`, `NewRegistry()`, `Detect()`, `All()` |
| `internal/ecosystem/ecosystem_test.go` | Create | Unit tests: detection, priority, disabled, globs |
| `internal/ecosystem/workspace.go` | Create | `DetectWorkspaces()`, per-format parsers (pnpm, npm, cargo, go) |
| `internal/ecosystem/workspace_test.go` | Create | Unit tests: workspace parsing for each format |
| `internal/integration/integration.go` | Create | `Integration`, `Dependency`, spec types, `All()`, `Get()` |
| `internal/integration/crg.go` | Create | code-review-graph definition |
| `internal/integration/ccc.go` | Create | cocoindex-code definition |
| `internal/integration/integration_test.go` | Create | Registry tests: completeness, field validation |
| `cmd/ecosystems.go` | Create | `RunEcosystems()` CLI handler |
| `cmd/integrations.go` | Create | `RunIntegrations()` CLI handler |
| `main.go` | Modify | Add subcommand dispatch for `ecosystems` and `integrations` |
| `go.mod` | Modify | Add `gopkg.in/yaml.v3` dependency |

---

## Task 1: Add YAML dependency and config types

**Files:**
- Modify: `go.mod`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Add yaml.v3 dependency**

Run:
```bash
cd /Users/abiswas/code/personal/sentei/main && go get gopkg.in/yaml.v3
```

- [ ] **Step 2: Write test for Config struct unmarshaling**

Create `internal/config/config_test.go`:

```go
package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Config
		wantErr bool
	}{
		{
			name: "full config",
			input: `
ecosystems:
  - name: pnpm
    detect:
      files: ["pnpm-lock.yaml"]
    install:
      command: "pnpm install"
      workspace_detect: "pnpm-workspace.yaml"
      workspace_install: "pnpm install --filter {dir}"
      parallel: true
    env_files: [".env", ".env.local"]
protected_branches:
  - main
  - master
integrations_enabled:
  - code-review-graph
`,
			want: Config{
				Ecosystems: []EcosystemConfig{
					{
						Name: "pnpm",
						Detect: DetectConfig{
							Files: []string{"pnpm-lock.yaml"},
						},
						Install: InstallConfig{
							Command:          "pnpm install",
							WorkspaceDetect:  "pnpm-workspace.yaml",
							WorkspaceInstall: "pnpm install --filter {dir}",
							Parallel:         boolPtr(true),
						},
						EnvFiles: []string{".env", ".env.local"},
					},
				},
				ProtectedBranches:   []string{"main", "master"},
				IntegrationsEnabled: []string{"code-review-graph"},
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
  - name: pip
    enabled: false
    detect:
      files: ["requirements.txt"]
    install:
      command: "pip install -r requirements.txt"
`,
			want: Config{
				Ecosystems: []EcosystemConfig{
					{
						Name:    "pip",
						Enabled: boolPtr(false),
						Detect:  DetectConfig{Files: []string{"requirements.txt"}},
						Install: InstallConfig{Command: "pip install -r requirements.txt"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Config
			err := yaml.Unmarshal([]byte(tt.input), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			assertConfigEqual(t, tt.want, got)
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func assertConfigEqual(t *testing.T, want, got Config) {
	t.Helper()
	if len(want.Ecosystems) != len(got.Ecosystems) {
		t.Fatalf("Ecosystems count: want %d, got %d", len(want.Ecosystems), len(got.Ecosystems))
	}
	for i, wantEco := range want.Ecosystems {
		gotEco := got.Ecosystems[i]
		if wantEco.Name != gotEco.Name {
			t.Errorf("Ecosystem[%d].Name: want %q, got %q", i, wantEco.Name, gotEco.Name)
		}
		if wantEco.Enabled != nil && gotEco.Enabled != nil && *wantEco.Enabled != *gotEco.Enabled {
			t.Errorf("Ecosystem[%d].Enabled: want %v, got %v", i, *wantEco.Enabled, *gotEco.Enabled)
		}
		if wantEco.Install.Command != gotEco.Install.Command {
			t.Errorf("Ecosystem[%d].Install.Command: want %q, got %q", i, wantEco.Install.Command, gotEco.Install.Command)
		}
		if wantEco.Install.WorkspaceDetect != gotEco.Install.WorkspaceDetect {
			t.Errorf("Ecosystem[%d].Install.WorkspaceDetect: want %q, got %q", i, wantEco.Install.WorkspaceDetect, gotEco.Install.WorkspaceDetect)
		}
		if wantEco.Install.IsParallel() != gotEco.Install.IsParallel() {
			t.Errorf("Ecosystem[%d].Install.Parallel: want %v, got %v", i, wantEco.Install.IsParallel(), gotEco.Install.IsParallel())
		}
	}
	if len(want.ProtectedBranches) != len(got.ProtectedBranches) {
		t.Errorf("ProtectedBranches: want %v, got %v", want.ProtectedBranches, got.ProtectedBranches)
	}
	if len(want.IntegrationsEnabled) != len(got.IntegrationsEnabled) {
		t.Errorf("IntegrationsEnabled: want %v, got %v", want.IntegrationsEnabled, got.IntegrationsEnabled)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/config/ -v -run TestConfigUnmarshal`

Expected: Compilation error — `config` package doesn't exist yet.

- [ ] **Step 4: Write Config types**

Create `internal/config/config.go`:

```go
package config

// Config is the merged sentei configuration from all layers.
type Config struct {
	Ecosystems          []EcosystemConfig `yaml:"ecosystems"`
	ProtectedBranches   []string          `yaml:"protected_branches"`
	IntegrationsEnabled []string          `yaml:"integrations_enabled"`
}

// EcosystemConfig defines a package manager / build tool that sentei can detect.
type EcosystemConfig struct {
	Name        string        `yaml:"name"`
	Enabled     *bool         `yaml:"enabled,omitempty"`
	Detect      DetectConfig  `yaml:"detect"`
	Install     InstallConfig `yaml:"install"`
	EnvFiles    []string      `yaml:"env_files"`
	PostInstall []string      `yaml:"post_install"`
	Source      string        `yaml:"-"` // "embedded", "global", or "per-repo" — set during merge, not from YAML
}

// IsEnabled returns true if the ecosystem is enabled (nil means enabled).
func (e *EcosystemConfig) IsEnabled() bool {
	return e.Enabled == nil || *e.Enabled
}

// DetectConfig defines how to detect an ecosystem in a worktree.
type DetectConfig struct {
	Files []string `yaml:"files"`
}

// InstallConfig defines how to install dependencies for an ecosystem.
type InstallConfig struct {
	Command          string `yaml:"command"`
	WorkspaceDetect  string `yaml:"workspace_detect,omitempty"`
	WorkspaceInstall string `yaml:"workspace_install,omitempty"`
	Parallel         *bool  `yaml:"parallel,omitempty"`
}

// IsParallel returns true if parallel install is enabled.
func (i *InstallConfig) IsParallel() bool {
	return i.Parallel != nil && *i.Parallel
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/config/ -v -run TestConfigUnmarshal`

Expected: All 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config/
git commit -m "feat(config): add Config types with YAML unmarshaling and tests"
```

---

## Task 2: Embedded default ecosystems

**Files:**
- Create: `internal/config/defaults/ecosystems.yaml`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write test for loading embedded defaults**

Add to `internal/config/config_test.go`:

```go
func TestLoadEmbeddedDefaults(t *testing.T) {
	cfg, err := loadEmbeddedDefaults()
	if err != nil {
		t.Fatalf("loadEmbeddedDefaults() error: %v", err)
	}

	if len(cfg.Ecosystems) != 16 {
		t.Errorf("want 16 ecosystems, got %d", len(cfg.Ecosystems))
	}

	// Verify priority order: pnpm is first
	if cfg.Ecosystems[0].Name != "pnpm" {
		t.Errorf("first ecosystem: want pnpm, got %s", cfg.Ecosystems[0].Name)
	}

	// Verify each ecosystem has required fields
	for i, eco := range cfg.Ecosystems {
		if eco.Name == "" {
			t.Errorf("Ecosystem[%d]: empty name", i)
		}
		if len(eco.Detect.Files) == 0 {
			t.Errorf("Ecosystem[%d] %q: no detect files", i, eco.Name)
		}
		if eco.Install.Command == "" {
			t.Errorf("Ecosystem[%d] %q: no install command", i, eco.Name)
		}
	}

	// Spot-check workspace ecosystems
	found := map[string]bool{}
	for _, eco := range cfg.Ecosystems {
		found[eco.Name] = true
		if eco.Name == "pnpm" {
			if eco.Install.WorkspaceDetect != "pnpm-workspace.yaml" {
				t.Errorf("pnpm WorkspaceDetect: want pnpm-workspace.yaml, got %q", eco.Install.WorkspaceDetect)
			}
			if !eco.Install.IsParallel() {
				t.Error("pnpm should have parallel: true")
			}
		}
	}

	// Verify all 16 ecosystems are present
	expectedNames := []string{
		"pnpm", "yarn", "npm", "bun", "cargo", "go", "uv", "poetry",
		"pip", "ruby", "php", "dotnet", "elixir", "swift", "dart", "deno",
	}
	for _, name := range expectedNames {
		if !found[name] {
			t.Errorf("missing ecosystem: %s", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/config/ -v -run TestLoadEmbeddedDefaults`

Expected: Compilation error — `loadEmbeddedDefaults` not defined.

- [ ] **Step 3: Create embedded ecosystems YAML**

Create `internal/config/defaults/ecosystems.yaml`:

```yaml
ecosystems:
  - name: pnpm
    detect:
      files: ["pnpm-lock.yaml"]
    install:
      command: "pnpm install"
      workspace_detect: "pnpm-workspace.yaml"
      workspace_install: "pnpm install --filter {dir}"
      parallel: true
    env_files: [".env", ".env.local"]

  - name: yarn
    detect:
      files: ["yarn.lock"]
    install:
      command: "yarn install"
      workspace_detect: "package.json"
      workspace_install: "yarn workspace {dir} install"
      parallel: true
    env_files: [".env", ".env.local"]

  - name: npm
    detect:
      files: ["package-lock.json"]
    install:
      command: "npm install"
      workspace_detect: "package.json"
      workspace_install: "npm install --workspace {dir}"
      parallel: true
    env_files: [".env", ".env.local"]

  - name: bun
    detect:
      files: ["bun.lockb"]
    install:
      command: "bun install"
      workspace_detect: "package.json"
      workspace_install: "bun install --filter {dir}"
      parallel: true
    env_files: [".env", ".env.local"]

  - name: cargo
    detect:
      files: ["Cargo.toml"]
    install:
      command: "cargo build"
      workspace_detect: "Cargo.toml"

  - name: go
    detect:
      files: ["go.mod"]
    install:
      command: "go mod download"
      workspace_detect: "go.work"

  - name: uv
    detect:
      files: ["uv.lock"]
    install:
      command: "uv sync"
    env_files: [".env"]

  - name: poetry
    detect:
      files: ["poetry.lock"]
    install:
      command: "poetry install"
    env_files: [".env"]

  - name: pip
    detect:
      files: ["requirements.txt"]
    install:
      command: "pip install -r requirements.txt"
    env_files: [".env"]

  - name: ruby
    detect:
      files: ["Gemfile.lock"]
    install:
      command: "bundle install"
    env_files: [".env"]

  - name: php
    detect:
      files: ["composer.lock"]
    install:
      command: "composer install"
    env_files: [".env"]

  - name: dotnet
    detect:
      files: ["*.sln", "*.csproj"]
    install:
      command: "dotnet restore"

  - name: elixir
    detect:
      files: ["mix.lock"]
    install:
      command: "mix deps.get"
    env_files: [".env"]

  - name: swift
    detect:
      files: ["Package.swift"]
    install:
      command: "swift package resolve"

  - name: dart
    detect:
      files: ["pubspec.lock"]
    install:
      command: "dart pub get"

  - name: deno
    detect:
      files: ["deno.lock"]
    install:
      command: "deno install"
```

- [ ] **Step 4: Write loadEmbeddedDefaults**

Add to `internal/config/config.go`:

```go
import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/ecosystems.yaml
var defaultEcosystemsYAML []byte

func loadEmbeddedDefaults() (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(defaultEcosystemsYAML, &cfg); err != nil {
		return nil, fmt.Errorf("parsing embedded defaults: %w", err)
	}
	return &cfg, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/config/ -v -run TestLoadEmbeddedDefaults`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add embedded default ecosystem definitions (16 ecosystems)"
```

---

## Task 3: Config file loading and three-layer merge

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Create: `internal/config/testdata/global_override.yaml`
- Create: `internal/config/testdata/per_repo.yaml`
- Create: `internal/config/testdata/malformed.yaml`

- [ ] **Step 1: Write tests for loadFile, mergeEcosystems, validate, and LoadConfig**

Add to `internal/config/config_test.go`:

```go
import (
	"os"
	"path/filepath"
)

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
  - name: custom
    detect:
      files: ["custom.lock"]
    install:
      command: "custom install"
`,
		},
		{
			name:    "malformed yaml",
			content: "ecosystems:\n  - name: [invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := loadFile(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("loadFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(cfg.Ecosystems) == 0 {
				t.Error("expected at least one ecosystem")
			}
		})
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	cfg, err := loadFile("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if cfg != nil {
		t.Error("missing file should return nil config")
	}
}

func TestMergeEcosystems(t *testing.T) {
	tests := []struct {
		name     string
		base     []EcosystemConfig
		overlay  []EcosystemConfig
		wantLen  int
		wantName string // name of first ecosystem after merge
		check    func(t *testing.T, result []EcosystemConfig)
	}{
		{
			name: "overlay adds new ecosystem",
			base: []EcosystemConfig{
				{Name: "pnpm", Install: InstallConfig{Command: "pnpm install"}},
			},
			overlay: []EcosystemConfig{
				{Name: "custom", Detect: DetectConfig{Files: []string{"custom.lock"}}, Install: InstallConfig{Command: "custom install"}},
			},
			wantLen:  2,
			wantName: "pnpm",
		},
		{
			name: "overlay overrides existing ecosystem field",
			base: []EcosystemConfig{
				{Name: "pnpm", Install: InstallConfig{Command: "pnpm install"}},
			},
			overlay: []EcosystemConfig{
				{Name: "pnpm", Install: InstallConfig{Command: "pnpm install --frozen-lockfile"}},
			},
			wantLen:  1,
			wantName: "pnpm",
			check: func(t *testing.T, result []EcosystemConfig) {
				if result[0].Install.Command != "pnpm install --frozen-lockfile" {
					t.Errorf("want overridden command, got %q", result[0].Install.Command)
				}
			},
		},
		{
			name: "overlay disables ecosystem",
			base: []EcosystemConfig{
				{Name: "pip", Detect: DetectConfig{Files: []string{"requirements.txt"}}, Install: InstallConfig{Command: "pip install -r requirements.txt"}},
			},
			overlay: []EcosystemConfig{
				{Name: "pip", Enabled: boolPtr(false)},
			},
			wantLen:  1,
			wantName: "pip",
			check: func(t *testing.T, result []EcosystemConfig) {
				if result[0].IsEnabled() {
					t.Error("pip should be disabled")
				}
			},
		},
		{
			name:     "nil overlay",
			base:     []EcosystemConfig{{Name: "go"}},
			overlay:  nil,
			wantLen:  1,
			wantName: "go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeEcosystems(tt.base, tt.overlay, "test")
			if len(result) != tt.wantLen {
				t.Fatalf("want %d ecosystems, got %d", tt.wantLen, len(result))
			}
			if result[0].Name != tt.wantName {
				t.Errorf("first ecosystem: want %q, got %q", tt.wantName, result[0].Name)
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Ecosystems: []EcosystemConfig{
					{Name: "go", Detect: DetectConfig{Files: []string{"go.mod"}}, Install: InstallConfig{Command: "go mod download"}},
				},
			},
		},
		{
			name: "missing ecosystem name",
			cfg: &Config{
				Ecosystems: []EcosystemConfig{
					{Detect: DetectConfig{Files: []string{"go.mod"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "ecosystem with no detect files",
			cfg: &Config{
				Ecosystems: []EcosystemConfig{
					{Name: "broken"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Set up a temp directory structure simulating XDG + bare repo
	tmpDir := t.TempDir()
	xdgConfig := filepath.Join(tmpDir, "xdg")
	globalDir := filepath.Join(xdgConfig, "sentei")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Global config: override pnpm command
	globalYAML := `
ecosystems:
  - name: pnpm
    detect:
      files: ["pnpm-lock.yaml"]
    install:
      command: "pnpm install --frozen-lockfile"
`
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Per-repo config: enable integrations
	repoYAML := `
integrations_enabled:
  - code-review-graph
`
	if err := os.WriteFile(filepath.Join(repoDir, ".sentei.yaml"), []byte(repoYAML), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	cfg, err := LoadConfig(repoDir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	// Embedded defaults should be present (16 ecosystems)
	if len(cfg.Ecosystems) < 16 {
		t.Errorf("want >= 16 ecosystems, got %d", len(cfg.Ecosystems))
	}

	// Global override should be applied
	for _, eco := range cfg.Ecosystems {
		if eco.Name == "pnpm" {
			if eco.Install.Command != "pnpm install --frozen-lockfile" {
				t.Errorf("pnpm command not overridden: got %q", eco.Install.Command)
			}
		}
	}

	// Per-repo integrations should be present
	if len(cfg.IntegrationsEnabled) != 1 || cfg.IntegrationsEnabled[0] != "code-review-graph" {
		t.Errorf("IntegrationsEnabled: want [code-review-graph], got %v", cfg.IntegrationsEnabled)
	}
}

func TestLoadConfig_EmbeddedOnly(t *testing.T) {
	// No global or per-repo config — should still work with embedded defaults
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "empty"))

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if len(cfg.Ecosystems) != 16 {
		t.Errorf("want 16 ecosystems from embedded defaults, got %d", len(cfg.Ecosystems))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/config/ -v -run "TestLoadFile|TestMerge|TestValidate|TestLoadConfig"`

Expected: Compilation errors — `loadFile`, `mergeEcosystems`, `validate`, `LoadConfig` not defined.

- [ ] **Step 3: Implement loadFile, mergeEcosystems, validate, and LoadConfig**

Add to `internal/config/config.go`:

```go
import (
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// loadFile reads and parses a YAML config file. Returns (nil, nil) if the file doesn't exist.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// mergeEcosystems merges overlay ecosystems into base. Keyed by name:
// existing entries are overridden field-by-field, new entries are appended.
// overlaySource is the source label ("global" or "per-repo") for tracking.
func mergeEcosystems(base, overlay []EcosystemConfig, overlaySource string) []EcosystemConfig {
	if len(overlay) == 0 {
		return base
	}

	index := make(map[string]int, len(base))
	result := make([]EcosystemConfig, len(base))
	copy(result, base)

	for i, eco := range result {
		index[eco.Name] = i
	}

	for _, over := range overlay {
		if idx, ok := index[over.Name]; ok {
			merged := result[idx]
			merged.Source = overlaySource
			if over.Enabled != nil {
				merged.Enabled = over.Enabled
			}
			if len(over.Detect.Files) > 0 {
				merged.Detect = over.Detect
			}
			if over.Install.Command != "" {
				merged.Install.Command = over.Install.Command
			}
			if over.Install.WorkspaceDetect != "" {
				merged.Install.WorkspaceDetect = over.Install.WorkspaceDetect
			}
			if over.Install.WorkspaceInstall != "" {
				merged.Install.WorkspaceInstall = over.Install.WorkspaceInstall
			}
			if over.Install.Parallel != nil {
				merged.Install.Parallel = over.Install.Parallel
			}
			if len(over.EnvFiles) > 0 {
				merged.EnvFiles = over.EnvFiles
			}
			if len(over.PostInstall) > 0 {
				merged.PostInstall = over.PostInstall
			}
			result[idx] = merged
		} else {
			over.Source = overlaySource
			result = append(result, over)
			index[over.Name] = len(result) - 1
		}
	}

	return result
}

// mergeConfigs merges overlay into base. Ecosystems merge by name; scalar lists replace entirely.
func mergeConfigs(base, overlay *Config, overlaySource string) *Config {
	if overlay == nil {
		return base
	}

	result := *base
	result.Ecosystems = mergeEcosystems(base.Ecosystems, overlay.Ecosystems, overlaySource)

	if len(overlay.ProtectedBranches) > 0 {
		result.ProtectedBranches = overlay.ProtectedBranches
	}
	if len(overlay.IntegrationsEnabled) > 0 {
		result.IntegrationsEnabled = overlay.IntegrationsEnabled
	}

	return &result
}

// validate checks that the merged config is well-formed.
// Returns an error for structural problems. Prints warnings to stderr for non-fatal issues.
func validate(cfg *Config) error {
	for i, eco := range cfg.Ecosystems {
		if eco.Name == "" {
			return fmt.Errorf("ecosystem[%d]: missing name", i)
		}
		if len(eco.Detect.Files) == 0 {
			return fmt.Errorf("ecosystem %q: no detect files", eco.Name)
		}
	}

	// Warn about unknown integration names (non-fatal)
	knownIntegrations := map[string]bool{
		"code-review-graph": true,
		"cocoindex-code":    true,
	}
	for _, name := range cfg.IntegrationsEnabled {
		if !knownIntegrations[name] {
			fmt.Fprintf(os.Stderr, "warning: unknown integration %q in integrations_enabled\n", name)
		}
	}

	return nil
}

// globalConfigPath returns the path to the global sentei config file,
// respecting XDG_CONFIG_HOME.
func globalConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "sentei", "config.yaml")
}

// resolveRepoRoot resolves the bare repo root from a path that may be inside a worktree.
// Falls back to repoPath if git commands fail.
func resolveRepoRoot(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return repoPath
	}
	commonDir := strings.TrimSpace(string(out))
	if filepath.IsAbs(commonDir) {
		// commonDir is like /repo/.bare — parent is the bare repo root
		return filepath.Dir(commonDir)
	}
	// Relative path (e.g., ".git") — resolve relative to repoPath
	abs := filepath.Join(repoPath, commonDir)
	return filepath.Dir(abs)
}

// LoadConfig loads the merged config from embedded defaults, global config, and per-repo config.
func LoadConfig(repoPath string) (*Config, error) {
	cfg, err := loadEmbeddedDefaults()
	if err != nil {
		return nil, err
	}
	// Tag embedded ecosystems with source
	for i := range cfg.Ecosystems {
		cfg.Ecosystems[i].Source = "embedded"
	}

	globalCfg, err := loadFile(globalConfigPath())
	if err != nil {
		return nil, err
	}
	cfg = mergeConfigs(cfg, globalCfg, "global")

	// Resolve to bare repo root so .sentei.yaml is found at the right level
	repoRoot := resolveRepoRoot(repoPath)
	repoConfigPath := filepath.Join(repoRoot, ".sentei.yaml")
	repoCfg, err := loadFile(repoConfigPath)
	if err != nil {
		return nil, err
	}
	cfg = mergeConfigs(cfg, repoCfg, "per-repo")

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/config/ -v`

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): implement three-layer config loading with merge and validation"
```

---

## Task 4: Ecosystem registry and detection

**Files:**
- Create: `internal/ecosystem/ecosystem.go`
- Create: `internal/ecosystem/ecosystem_test.go`

- [ ] **Step 1: Write tests for ecosystem detection**

Create `internal/ecosystem/ecosystem_test.go`:

```go
package ecosystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		files     []string // files to create in temp dir
		wantNames []string // expected ecosystem names in order
	}{
		{
			name:      "go project",
			files:     []string{"go.mod"},
			wantNames: []string{"go"},
		},
		{
			name:      "pnpm project",
			files:     []string{"pnpm-lock.yaml", "package.json"},
			wantNames: []string{"pnpm"},
		},
		{
			name:      "multi-language project",
			files:     []string{"go.mod", "package-lock.json"},
			wantNames: []string{"npm", "go"},
		},
		{
			name:      "no ecosystem",
			files:     []string{"README.md"},
			wantNames: nil,
		},
		{
			name:      "pnpm wins over npm when both present",
			files:     []string{"pnpm-lock.yaml", "package-lock.json"},
			wantNames: []string{"pnpm", "npm"},
		},
	}

	ecosystems := []config.EcosystemConfig{
		{Name: "pnpm", Detect: config.DetectConfig{Files: []string{"pnpm-lock.yaml"}}},
		{Name: "npm", Detect: config.DetectConfig{Files: []string{"package-lock.json"}}},
		{Name: "go", Detect: config.DetectConfig{Files: []string{"go.mod"}}},
	}

	reg := NewRegistry(ecosystems)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
			}

			detected, err := reg.Detect(dir)
			if err != nil {
				t.Fatalf("Detect() error: %v", err)
			}

			var gotNames []string
			for _, d := range detected {
				gotNames = append(gotNames, d.Name)
			}

			if len(gotNames) != len(tt.wantNames) {
				t.Fatalf("want %v, got %v", tt.wantNames, gotNames)
			}
			for i, want := range tt.wantNames {
				if gotNames[i] != want {
					t.Errorf("detected[%d]: want %q, got %q", i, want, gotNames[i])
				}
			}
		})
	}
}

func TestDetect_DisabledEcosystem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	disabled := false
	ecosystems := []config.EcosystemConfig{
		{Name: "go", Enabled: &disabled, Detect: config.DetectConfig{Files: []string{"go.mod"}}},
	}

	reg := NewRegistry(ecosystems)
	detected, err := reg.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if len(detected) != 0 {
		t.Errorf("disabled ecosystem should not be detected, got %v", detected)
	}
}

func TestDetect_GlobPattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "MyApp.sln"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	ecosystems := []config.EcosystemConfig{
		{Name: "dotnet", Detect: config.DetectConfig{Files: []string{"*.sln", "*.csproj"}}},
	}

	reg := NewRegistry(ecosystems)
	detected, err := reg.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if len(detected) != 1 || detected[0].Name != "dotnet" {
		t.Errorf("want [dotnet], got %v", detected)
	}
}

func TestRegistryAll(t *testing.T) {
	ecosystems := []config.EcosystemConfig{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	}
	reg := NewRegistry(ecosystems)
	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("want 3, got %d", len(all))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/ecosystem/ -v`

Expected: Compilation error — package doesn't exist.

- [ ] **Step 3: Implement ecosystem registry**

Create `internal/ecosystem/ecosystem.go`:

```go
package ecosystem

import (
	"os"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/config"
)

// Ecosystem represents a detected package manager / build tool.
type Ecosystem struct {
	Name    string
	Config  config.EcosystemConfig
}

// Registry holds ecosystem definitions in priority order.
type Registry struct {
	ecosystems []config.EcosystemConfig
}

// NewRegistry creates a registry from config definitions. Order is preserved for priority.
func NewRegistry(cfg []config.EcosystemConfig) *Registry {
	return &Registry{ecosystems: cfg}
}

// Detect returns all ecosystems whose detection files are found in the given directory.
// Results are in priority order (same order as the registry).
func (r *Registry) Detect(dir string) ([]Ecosystem, error) {
	var detected []Ecosystem

	for _, eco := range r.ecosystems {
		if !eco.IsEnabled() {
			continue
		}

		if matchesAny(dir, eco.Detect.Files) {
			detected = append(detected, Ecosystem{
				Name:   eco.Name,
				Config: eco,
			})
		}
	}

	return detected, nil
}

// All returns all registered ecosystems in priority order.
func (r *Registry) All() []Ecosystem {
	result := make([]Ecosystem, len(r.ecosystems))
	for i, eco := range r.ecosystems {
		result[i] = Ecosystem{Name: eco.Name, Config: eco}
	}
	return result
}

// matchesAny returns true if any of the patterns match a file in dir.
func matchesAny(dir string, patterns []string) bool {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err == nil && !info.IsDir() {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/ecosystem/ -v`

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ecosystem/
git commit -m "feat(ecosystem): add registry with priority-ordered detection and glob support"
```

---

## Task 5: Workspace detection

**Files:**
- Create: `internal/ecosystem/workspace.go`
- Create: `internal/ecosystem/workspace_test.go`

- [ ] **Step 1: Write workspace detection tests**

Create `internal/ecosystem/workspace_test.go`:

```go
package ecosystem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDetectWorkspaces_Pnpm(t *testing.T) {
	dir := t.TempDir()

	// Create pnpm-workspace.yaml
	yaml := `packages:
  - "packages/*"
  - "apps/*"
`
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	// Create matching directories
	for _, d := range []string{"packages/ui", "packages/core", "apps/web"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	dirs, err := DetectWorkspaces(dir, "pnpm-workspace.yaml")
	if err != nil {
		t.Fatalf("DetectWorkspaces() error: %v", err)
	}

	sort.Strings(dirs)
	want := []string{"apps/web", "packages/core", "packages/ui"}
	if len(dirs) != len(want) {
		t.Fatalf("want %v, got %v", want, dirs)
	}
	for i, w := range want {
		if dirs[i] != w {
			t.Errorf("dirs[%d]: want %q, got %q", i, w, dirs[i])
		}
	}
}

func TestDetectWorkspaces_Npm(t *testing.T) {
	dir := t.TempDir()

	pkg := map[string]interface{}{
		"name":       "monorepo",
		"workspaces": []string{"packages/*"},
	}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "packages", "lib"), 0755); err != nil {
		t.Fatal(err)
	}

	dirs, err := DetectWorkspaces(dir, "package.json")
	if err != nil {
		t.Fatalf("DetectWorkspaces() error: %v", err)
	}

	if len(dirs) != 1 || dirs[0] != "packages/lib" {
		t.Errorf("want [packages/lib], got %v", dirs)
	}
}

func TestDetectWorkspaces_PackageJsonNoWorkspaces(t *testing.T) {
	dir := t.TempDir()

	pkg := map[string]interface{}{"name": "single"}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	dirs, err := DetectWorkspaces(dir, "package.json")
	if err != nil {
		t.Fatalf("DetectWorkspaces() error: %v", err)
	}
	if len(dirs) != 0 {
		t.Errorf("want no workspaces, got %v", dirs)
	}
}

func TestDetectWorkspaces_GoWork(t *testing.T) {
	dir := t.TempDir()

	gowork := `go 1.21

use (
	./cmd/api
	./pkg/core
)
`
	if err := os.WriteFile(filepath.Join(dir, "go.work"), []byte(gowork), 0644); err != nil {
		t.Fatal(err)
	}

	for _, d := range []string{"cmd/api", "pkg/core"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	dirs, err := DetectWorkspaces(dir, "go.work")
	if err != nil {
		t.Fatalf("DetectWorkspaces() error: %v", err)
	}

	sort.Strings(dirs)
	want := []string{"cmd/api", "pkg/core"}
	if len(dirs) != len(want) {
		t.Fatalf("want %v, got %v", want, dirs)
	}
	for i, w := range want {
		if dirs[i] != w {
			t.Errorf("dirs[%d]: want %q, got %q", i, w, dirs[i])
		}
	}
}

func TestDetectWorkspaces_CargoToml(t *testing.T) {
	dir := t.TempDir()

	toml := `[workspace]
members = [
    "crates/lib",
    "crates/cli",
]
`
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}

	for _, d := range []string{"crates/lib", "crates/cli"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	dirs, err := DetectWorkspaces(dir, "Cargo.toml")
	if err != nil {
		t.Fatalf("DetectWorkspaces() error: %v", err)
	}

	sort.Strings(dirs)
	want := []string{"crates/cli", "crates/lib"}
	if len(dirs) != len(want) {
		t.Fatalf("want %v, got %v", want, dirs)
	}
}

func TestDetectWorkspaces_MissingFile(t *testing.T) {
	dir := t.TempDir()
	dirs, err := DetectWorkspaces(dir, "nonexistent.yaml")
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if len(dirs) != 0 {
		t.Errorf("want no workspaces, got %v", dirs)
	}
}

func TestDetectWorkspaces_MalformedPnpmYaml(t *testing.T) {
	dir := t.TempDir()

	// Malformed YAML
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte("packages: [invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	dirs, err := DetectWorkspaces(dir, "pnpm-workspace.yaml")
	if err != nil {
		t.Fatalf("malformed workspace should not error, got: %v", err)
	}
	if len(dirs) != 0 {
		t.Errorf("malformed workspace should return no dirs, got %v", dirs)
	}
}

func TestDetectWorkspaces_GlobNoMatches(t *testing.T) {
	dir := t.TempDir()

	yaml := `packages:
  - "packages/*"
`
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	// Don't create any packages/ directories

	dirs, err := DetectWorkspaces(dir, "pnpm-workspace.yaml")
	if err != nil {
		t.Fatalf("DetectWorkspaces() error: %v", err)
	}
	if len(dirs) != 0 {
		t.Errorf("want no workspaces, got %v", dirs)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/ecosystem/ -v -run TestDetectWorkspaces`

Expected: Compilation error — `DetectWorkspaces` not defined.

- [ ] **Step 3: Implement workspace detection**

Create `internal/ecosystem/workspace.go`:

```go
package ecosystem

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// DetectWorkspaces parses a workspace config file and returns relative directory paths
// for each workspace member. Returns nil if the file doesn't exist or has no workspaces.
func DetectWorkspaces(rootDir, configFile string) ([]string, error) {
	fullPath := filepath.Join(rootDir, configFile)
	if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	switch {
	case configFile == "pnpm-workspace.yaml":
		return parsePnpmWorkspace(rootDir, fullPath)
	case configFile == "package.json":
		return parseNpmWorkspace(rootDir, fullPath)
	case configFile == "go.work":
		return parseGoWork(rootDir, fullPath)
	case configFile == "Cargo.toml":
		return parseCargoWorkspace(rootDir, fullPath)
	default:
		return nil, fmt.Errorf("unsupported workspace config: %s", configFile)
	}
}

func parsePnpmWorkspace(rootDir, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var ws struct {
		Packages []string `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil, nil // malformed → treat as no workspaces
	}

	return resolveGlobs(rootDir, ws.Packages)
}

func parseNpmWorkspace(rootDir, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var pkg struct {
		Workspaces []string `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, nil
	}
	if len(pkg.Workspaces) == 0 {
		return nil, nil
	}

	return resolveGlobs(rootDir, pkg.Workspaces)
}

func parseGoWork(rootDir, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// Parse "use" directives: single line or block
	var dirs []string
	lines := strings.Split(string(data), "\n")
	inUseBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inUseBlock {
			if trimmed == ")" {
				inUseBlock = false
				continue
			}
			dir := strings.Trim(trimmed, "./")
			if dir != "" {
				dirs = append(dirs, dir)
			}
			continue
		}

		if strings.HasPrefix(trimmed, "use (") {
			inUseBlock = true
			continue
		}
		if strings.HasPrefix(trimmed, "use ") {
			dir := strings.TrimPrefix(trimmed, "use ")
			dir = strings.Trim(dir, "./")
			if dir != "" {
				dirs = append(dirs, dir)
			}
		}
	}

	return filterExistingDirs(rootDir, dirs), nil
}

var cargoMembersRe = regexp.MustCompile(`"([^"]+)"`)

func parseCargoWorkspace(rootDir, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	content := string(data)

	// Find [workspace] section and extract members
	wsIdx := strings.Index(content, "[workspace]")
	if wsIdx == -1 {
		return nil, nil
	}

	membersIdx := strings.Index(content[wsIdx:], "members")
	if membersIdx == -1 {
		return nil, nil
	}

	// Extract the array content between [ and ]
	start := strings.Index(content[wsIdx+membersIdx:], "[")
	if start == -1 {
		return nil, nil
	}
	end := strings.Index(content[wsIdx+membersIdx+start:], "]")
	if end == -1 {
		return nil, nil
	}

	arrayContent := content[wsIdx+membersIdx+start : wsIdx+membersIdx+start+end+1]
	matches := cargoMembersRe.FindAllStringSubmatch(arrayContent, -1)

	var globs []string
	for _, m := range matches {
		globs = append(globs, m[1])
	}

	return resolveGlobs(rootDir, globs)
}

// resolveGlobs expands glob patterns relative to rootDir and returns matching directory paths
// as relative paths from rootDir.
func resolveGlobs(rootDir string, patterns []string) ([]string, error) {
	var dirs []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(rootDir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(rootDir, match)
			if err != nil {
				continue
			}
			if !seen[rel] {
				seen[rel] = true
				dirs = append(dirs, rel)
			}
		}
	}

	return dirs, nil
}

// filterExistingDirs returns only directories that exist under rootDir.
func filterExistingDirs(rootDir string, dirs []string) []string {
	var existing []string
	for _, dir := range dirs {
		info, err := os.Stat(filepath.Join(rootDir, dir))
		if err == nil && info.IsDir() {
			existing = append(existing, dir)
		}
	}
	return existing
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/ecosystem/ -v`

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ecosystem/
git commit -m "feat(ecosystem): add workspace detection for pnpm, npm, go, and cargo"
```

---

## Task 6: Integration registry

**Files:**
- Create: `internal/integration/integration.go`
- Create: `internal/integration/crg.go`
- Create: `internal/integration/ccc.go`
- Create: `internal/integration/integration_test.go`

- [ ] **Step 1: Write integration registry tests**

Create `internal/integration/integration_test.go`:

```go
package integration

import (
	"testing"
)

func TestAll(t *testing.T) {
	all := All()
	if len(all) != 2 {
		t.Fatalf("want 2 integrations, got %d", len(all))
	}

	names := map[string]bool{}
	for _, integ := range all {
		names[integ.Name] = true
	}

	if !names["code-review-graph"] {
		t.Error("missing code-review-graph integration")
	}
	if !names["cocoindex-code"] {
		t.Error("missing cocoindex-code integration")
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantNil bool
	}{
		{name: "existing", query: "code-review-graph"},
		{name: "existing ccc", query: "cocoindex-code"},
		{name: "not found", query: "nonexistent", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Get(tt.query)
			if tt.wantNil && got != nil {
				t.Error("want nil, got non-nil")
			}
			if !tt.wantNil && got == nil {
				t.Errorf("want integration %q, got nil", tt.query)
			}
		})
	}
}

func TestIntegrationFieldsComplete(t *testing.T) {
	for _, integ := range All() {
		t.Run(integ.Name, func(t *testing.T) {
			if integ.Name == "" {
				t.Error("empty Name")
			}
			if integ.Description == "" {
				t.Error("empty Description")
			}
			if integ.URL == "" {
				t.Error("empty URL")
			}
			if len(integ.Dependencies) == 0 {
				t.Error("no Dependencies")
			}
			if integ.Detect.Command == "" && integ.Detect.BinaryName == "" {
				t.Error("no Detect command or binary name")
			}
			if integ.Install.Command == "" {
				t.Error("empty Install.Command")
			}
			if integ.Setup.Command == "" {
				t.Error("empty Setup.Command")
			}
			if integ.Setup.WorkingDir != "repo" && integ.Setup.WorkingDir != "worktree" {
				t.Errorf("Setup.WorkingDir must be 'repo' or 'worktree', got %q", integ.Setup.WorkingDir)
			}
			if integ.Teardown.Command == "" && len(integ.Teardown.Dirs) == 0 {
				t.Error("no Teardown command or dirs")
			}
			if len(integ.GitignoreEntries) == 0 {
				t.Error("no GitignoreEntries")
			}
		})
	}
}

func TestDependencyFieldsComplete(t *testing.T) {
	for _, integ := range All() {
		for i, dep := range integ.Dependencies {
			t.Run(integ.Name+"/dep/"+dep.Name, func(t *testing.T) {
				if dep.Name == "" {
					t.Errorf("Dependency[%d]: empty Name", i)
				}
				if dep.Detect == "" {
					t.Errorf("Dependency[%d] %q: empty Detect", i, dep.Name)
				}
			})
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/integration/ -v`

Expected: Compilation error — package doesn't exist.

- [ ] **Step 3: Write integration types and registry**

Create `internal/integration/integration.go`:

```go
package integration

// Integration defines a built-in tool that sentei can set up and tear down in worktrees.
type Integration struct {
	Name             string
	Description      string
	URL              string
	Dependencies     []Dependency
	Detect           DetectSpec
	Install          InstallSpec
	Setup            SetupSpec
	Teardown         TeardownSpec
	GitignoreEntries []string
}

// Dependency is a prerequisite tool that must be installed before the integration.
type Dependency struct {
	Name    string
	Detect  string // shell command to check if installed
	Install string // shell command to install (may be empty if manual)
}

// DetectSpec defines how to check if the integration tool is installed.
type DetectSpec struct {
	Command    string // command to run (e.g., "code-review-graph --version")
	BinaryName string // fallback: check if this binary is on PATH
}

// InstallSpec defines how to install the integration tool.
type InstallSpec struct {
	Command      string
	FirstRunNote string // displayed in TUI before first install
}

// SetupSpec defines the command to run after worktree creation.
type SetupSpec struct {
	Command    string
	WorkingDir string // "repo" or "worktree"
}

// TeardownSpec defines how to clean up integration artifacts on worktree removal.
type TeardownSpec struct {
	Command string   // preferred cleanup command
	Dirs    []string // fallback: delete these directories
}

var registry []Integration

func register(i Integration) {
	registry = append(registry, i)
}

// All returns all registered integrations.
func All() []Integration {
	return registry
}

// Get returns the integration with the given name, or nil if not found.
func Get(name string) *Integration {
	for i := range registry {
		if registry[i].Name == name {
			return &registry[i]
		}
	}
	return nil
}
```

- [ ] **Step 4: Write code-review-graph definition**

Create `internal/integration/crg.go`:

```go
package integration

func init() {
	register(Integration{
		Name:        "code-review-graph",
		Description: "Build code graph for AI-assisted code review",
		URL:         "https://github.com/tirth8205/code-review-graph",
		Dependencies: []Dependency{
			{
				Name:   "python3.10+",
				Detect: `python3 -c "import sys; assert sys.version_info >= (3,10)"`,
			},
			{
				Name:    "pipx",
				Detect:  "pipx --version",
				Install: "brew install pipx",
			},
		},
		Detect: DetectSpec{
			Command: "code-review-graph --version",
		},
		Install: InstallSpec{
			Command: "pipx install code-review-graph",
		},
		Setup: SetupSpec{
			Command:    "code-review-graph build --repo {path}",
			WorkingDir: "repo",
		},
		Teardown: TeardownSpec{
			Dirs: []string{".code-review-graph/"},
		},
		GitignoreEntries: []string{".code-review-graph/"},
	})
}
```

- [ ] **Step 5: Write cocoindex-code definition**

Create `internal/integration/ccc.go`:

```go
package integration

func init() {
	register(Integration{
		Name:        "cocoindex-code",
		Description: "Semantic code search index",
		URL:         "https://github.com/cocoindex-io/cocoindex-code",
		Dependencies: []Dependency{
			{
				Name:   "python3.11+",
				Detect: `python3 -c "import sys; assert sys.version_info >= (3,11)"`,
			},
			{
				Name:    "uv",
				Detect:  "uv --version",
				Install: "brew install uv",
			},
		},
		Detect: DetectSpec{
			BinaryName: "ccc",
		},
		Install: InstallSpec{
			Command:      `uv tool install --upgrade cocoindex-code --prerelease explicit --with "cocoindex>=1.0.0a24"`,
			FirstRunNote: "Downloads ~87MB embedding model on first use",
		},
		Setup: SetupSpec{
			Command:    "ccc init && ccc index",
			WorkingDir: "worktree",
		},
		Teardown: TeardownSpec{
			Command: "ccc reset --all --force",
			Dirs:    []string{".cocoindex_code/"},
		},
		GitignoreEntries: []string{".cocoindex_code/"},
	})
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./internal/integration/ -v`

Expected: All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/integration/
git commit -m "feat(integration): add registry with code-review-graph and cocoindex-code definitions"
```

---

## Task 7: CLI subcommands — `sentei ecosystems` and `sentei integrations`

**Files:**
- Create: `cmd/ecosystems.go`
- Create: `cmd/integrations.go`
- Modify: `main.go`

- [ ] **Step 1: Write the ecosystems CLI handler**

Create `cmd/ecosystems.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/ecosystem"
)

func RunEcosystems(args []string) {
	repoPath := "."
	if len(args) > 0 {
		repoPath = args[0]
	}

	cfg, err := config.LoadConfig(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	reg := ecosystem.NewRegistry(cfg.Ecosystems)
	all := reg.All()

	fmt.Printf("Ecosystems (%d registered)\n\n", len(all))
	fmt.Printf("  %-14s %-24s %-30s %-10s %s\n", "NAME", "DETECT FILES", "INSTALL", "SOURCE", "STATUS")

	for _, eco := range all {
		files := strings.Join(eco.Config.Detect.Files, ", ")
		status := "enabled"
		if !eco.Config.IsEnabled() {
			status = "disabled"
		}
		source := eco.Config.Source
		if source == "" {
			source = "embedded"
		}
		fmt.Printf("  %-14s %-24s %-30s %-10s %s\n",
			eco.Name,
			truncate(files, 22),
			truncate(eco.Config.Install.Command, 28),
			source,
			status,
		)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
```

- [ ] **Step 2: Write the integrations CLI handler**

Create `cmd/integrations.go`:

```go
package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/integration"
)

func RunIntegrations() {
	all := integration.All()

	fmt.Printf("Integrations (%d registered)\n\n", len(all))
	fmt.Printf("  %-22s %-12s %s\n", "NAME", "STATUS", "DESCRIPTION")

	for _, integ := range all {
		status := detectStatus(integ)
		fmt.Printf("  %-22s %-12s %s\n", integ.Name, status, integ.Description)
		fmt.Printf("  %-22s %-12s %s\n", "", "", integ.URL)
	}
}

func detectStatus(integ integration.Integration) string {
	if integ.Detect.Command != "" {
		parts := strings.Fields(integ.Detect.Command)
		if len(parts) > 0 {
			cmd := exec.Command(parts[0], parts[1:]...)
			if err := cmd.Run(); err == nil {
				return "installed"
			}
		}
	}
	if integ.Detect.BinaryName != "" {
		if _, err := exec.LookPath(integ.Detect.BinaryName); err == nil {
			return "installed"
		}
	}
	return "not found"
}
```

- [ ] **Step 3: Wire subcommands into main.go**

Modify `main.go` — add subcommand dispatch after the existing `cleanup` check:

```go
// Add after the cleanup check, before flag.Parse()
if len(os.Args) > 1 && os.Args[1] == "cleanup" {
	cmd.RunCleanup(os.Args[2:])
	return
}

if len(os.Args) > 1 && os.Args[1] == "ecosystems" {
	cmd.RunEcosystems(os.Args[2:])
	return
}

if len(os.Args) > 1 && os.Args[1] == "integrations" {
	cmd.RunIntegrations()
	return
}
```

- [ ] **Step 4: Build and test manually**

Run:
```bash
cd /Users/abiswas/code/personal/sentei/main && go build -o sentei . && ./sentei ecosystems && echo "---" && ./sentei integrations
```

Expected: Table output listing 16 ecosystems and 2 integrations with install status.

- [ ] **Step 5: Commit**

```bash
git add cmd/ecosystems.go cmd/integrations.go main.go
git commit -m "feat(cli): add 'sentei ecosystems' and 'sentei integrations' subcommands"
```

---

## Task 8: E2E tests

**Files:**
- Create: `internal/config/config_e2e_test.go`
- Create: `internal/ecosystem/ecosystem_e2e_test.go`

- [ ] **Step 1: Write config E2E tests**

Create `internal/config/config_e2e_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_E2E_FullStack(t *testing.T) {
	// Simulate: XDG global config + bare repo with .sentei.yaml
	tmpDir := t.TempDir()

	// Global config: add custom ecosystem, override pnpm
	xdgConfig := filepath.Join(tmpDir, "config")
	globalDir := filepath.Join(xdgConfig, "sentei")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}

	globalYAML := `
ecosystems:
  - name: pnpm
    detect:
      files: ["pnpm-lock.yaml"]
    install:
      command: "pnpm install --frozen-lockfile"
  - name: custom-tool
    detect:
      files: ["custom.lock"]
    install:
      command: "custom install"
protected_branches:
  - main
  - staging
`
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Per-repo config: enable integrations, replace protected branches
	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	repoYAML := `
integrations_enabled:
  - code-review-graph
  - cocoindex-code
protected_branches:
  - main
  - production
`
	if err := os.WriteFile(filepath.Join(repoDir, ".sentei.yaml"), []byte(repoYAML), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	cfg, err := LoadConfig(repoDir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	// Embedded 16 + 1 custom from global = 17
	if len(cfg.Ecosystems) != 17 {
		t.Errorf("want 17 ecosystems, got %d", len(cfg.Ecosystems))
	}

	// pnpm should have global override
	for _, eco := range cfg.Ecosystems {
		if eco.Name == "pnpm" {
			if eco.Install.Command != "pnpm install --frozen-lockfile" {
				t.Errorf("pnpm not overridden: %q", eco.Install.Command)
			}
		}
	}

	// custom-tool should be present
	found := false
	for _, eco := range cfg.Ecosystems {
		if eco.Name == "custom-tool" {
			found = true
		}
	}
	if !found {
		t.Error("custom-tool not found in merged config")
	}

	// Per-repo protected_branches should replace global
	if len(cfg.ProtectedBranches) != 2 {
		t.Fatalf("want 2 protected branches, got %d", len(cfg.ProtectedBranches))
	}
	if cfg.ProtectedBranches[0] != "main" || cfg.ProtectedBranches[1] != "production" {
		t.Errorf("want [main, production], got %v", cfg.ProtectedBranches)
	}

	// Per-repo integrations
	if len(cfg.IntegrationsEnabled) != 2 {
		t.Errorf("want 2 integrations enabled, got %d", len(cfg.IntegrationsEnabled))
	}
}

func TestLoadConfig_E2E_MalformedGlobalErrors(t *testing.T) {
	tmpDir := t.TempDir()
	xdgConfig := filepath.Join(tmpDir, "config")
	globalDir := filepath.Join(xdgConfig, "sentei")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte("ecosystems:\n  - name: [bad"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	_, err := LoadConfig(tmpDir)
	if err == nil {
		t.Error("expected error for malformed global config")
	}
}

func TestLoadConfig_E2E_MalformedRepoErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "empty"))

	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, ".sentei.yaml"), []byte("invalid: [yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(repoDir)
	if err == nil {
		t.Error("expected error for malformed repo config")
	}
}
```

- [ ] **Step 2: Write ecosystem E2E tests**

Create `internal/ecosystem/ecosystem_e2e_test.go`:

```go
package ecosystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
)

func TestDetect_E2E_GoProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	reg := NewRegistry(cfg.Ecosystems)
	detected, err := reg.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(detected) != 1 {
		t.Fatalf("want 1 ecosystem, got %d: %v", len(detected), detected)
	}
	if detected[0].Name != "go" {
		t.Errorf("want 'go', got %q", detected[0].Name)
	}
}

func TestDetect_E2E_PnpmMonorepo(t *testing.T) {
	dir := t.TempDir()

	// pnpm-lock.yaml
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte("lockfileVersion: 5.4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// pnpm-workspace.yaml with packages
	wsYAML := "packages:\n  - \"packages/*\"\n  - \"apps/*\"\n"
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte(wsYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create workspace directories
	for _, d := range []string{"packages/ui", "packages/core", "apps/web"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	reg := NewRegistry(cfg.Ecosystems)
	detected, err := reg.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(detected) != 1 || detected[0].Name != "pnpm" {
		t.Fatalf("want [pnpm], got %v", detected)
	}

	// Verify workspace detection
	wsDirs, err := DetectWorkspaces(dir, detected[0].Config.Install.WorkspaceDetect)
	if err != nil {
		t.Fatalf("DetectWorkspaces: %v", err)
	}
	if len(wsDirs) != 3 {
		t.Errorf("want 3 workspace dirs, got %d: %v", len(wsDirs), wsDirs)
	}
}

func TestDetect_E2E_MultiLanguage(t *testing.T) {
	dir := t.TempDir()

	// Go + Node project
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	reg := NewRegistry(cfg.Ecosystems)
	detected, err := reg.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(detected) != 2 {
		t.Fatalf("want 2 ecosystems, got %d: %v", len(detected), detected)
	}

	names := map[string]bool{}
	for _, d := range detected {
		names[d.Name] = true
	}
	if !names["npm"] || !names["go"] {
		t.Errorf("want npm + go, got %v", names)
	}

	// npm should come before go (priority order from defaults)
	if detected[0].Name != "npm" {
		t.Errorf("npm should have higher priority, got %q first", detected[0].Name)
	}
}
```

- [ ] **Step 3: Write CLI subprocess E2E tests**

Create `cmd/cli_e2e_test.go`:

```go
package cmd_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestEcosystemsCLI(t *testing.T) {
	// Build the binary
	tmpBin := t.TempDir() + "/sentei"
	build := exec.Command("go", "build", "-o", tmpBin, ".")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	cmd := exec.Command(tmpBin, "ecosystems")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei ecosystems failed: %s\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Ecosystems (") {
		t.Error("missing header")
	}
	if !strings.Contains(output, "pnpm") {
		t.Error("missing pnpm ecosystem")
	}
	if !strings.Contains(output, "go") {
		t.Error("missing go ecosystem")
	}
	if !strings.Contains(output, "SOURCE") {
		t.Error("missing SOURCE column header")
	}
}

func TestIntegrationsCLI(t *testing.T) {
	tmpBin := t.TempDir() + "/sentei"
	build := exec.Command("go", "build", "-o", tmpBin, ".")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	cmd := exec.Command(tmpBin, "integrations")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei integrations failed: %s\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Integrations (2") {
		t.Error("missing header with count")
	}
	if !strings.Contains(output, "code-review-graph") {
		t.Error("missing code-review-graph")
	}
	if !strings.Contains(output, "cocoindex-code") {
		t.Error("missing cocoindex-code")
	}
	if !strings.Contains(output, "https://github.com/") {
		t.Error("missing URL")
	}
}
```

- [ ] **Step 4: Run all tests**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./... -v -count=1`

Expected: All tests PASS including new E2E tests.

- [ ] **Step 6: Run linting and formatting**

Run:
```bash
cd /Users/abiswas/code/personal/sentei/main && go fmt ./... && go vet ./...
```

Expected: No issues.

- [ ] **Step 7: Verify build**

Run: `cd /Users/abiswas/code/personal/sentei/main && go build -o sentei .`

Expected: Clean build, no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/config/config_e2e_test.go internal/ecosystem/ecosystem_e2e_test.go cmd/cli_e2e_test.go
git commit -m "test: add E2E tests for config, ecosystem detection, and CLI subcommands"
```

---

## Task 9: Final verification

- [ ] **Step 1: Run full test suite with coverage**

Run: `cd /Users/abiswas/code/personal/sentei/main && go test ./... -cover -count=1`

Expected: All tests PASS. Coverage should be high for new packages.

- [ ] **Step 2: Test CLI subcommands end-to-end**

Run:
```bash
cd /Users/abiswas/code/personal/sentei/main && go build -o sentei . && ./sentei ecosystems && echo "---" && ./sentei integrations
```

Expected: Formatted table output for both commands.

- [ ] **Step 3: Verify existing functionality is unbroken**

Run: `cd /Users/abiswas/code/personal/sentei/main && ./sentei --dry-run`

Expected: Existing dry-run output works as before.

- [ ] **Step 4: Clean up binary**

Run: `rm -f /Users/abiswas/code/personal/sentei/main/sentei`
