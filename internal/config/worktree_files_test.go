package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parseWorktreeFilesConfig(t *testing.T, input string) Config {
	t.Helper()
	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error: %v", err)
	}
	return cfg
}

func TestWorktreeFilesUnmarshal(t *testing.T) {
	cfg := parseWorktreeFilesConfig(t, `
worktree_files:
  - source: .sentei/private/codex-config.toml
    destination: .codex/config.toml
  - source: .sentei/private/tool.json
    destination: .tool/config.json
    overwrite: true
`)

	if len(cfg.WorktreeFiles) != 2 {
		t.Fatalf("WorktreeFiles length = %d, want 2", len(cfg.WorktreeFiles))
	}
	first := cfg.WorktreeFiles[0]
	if first.Source != ".sentei/private/codex-config.toml" {
		t.Errorf("Source = %q", first.Source)
	}
	if first.Destination != ".codex/config.toml" {
		t.Errorf("Destination = %q", first.Destination)
	}
	if first.Overwrite {
		t.Error("Overwrite defaults to true, want false")
	}
	if !cfg.WorktreeFiles[1].Overwrite {
		t.Error("explicit Overwrite = false, want true")
	}
}

func TestMergeConfigsWorktreeFiles(t *testing.T) {
	base := parseWorktreeFilesConfig(t, `
worktree_files:
  - source: templates/base
    destination: .config/base
`)
	replacement := parseWorktreeFilesConfig(t, `
worktree_files:
  - source: templates/repo
    destination: .config/repo
`)

	replaced := mergeConfigs(&base, &replacement, "per-repo")
	if len(replaced.WorktreeFiles) != 1 || replaced.WorktreeFiles[0].Source != "templates/repo" {
		t.Fatalf("replacement WorktreeFiles = %#v", replaced.WorktreeFiles)
	}

	preserved := mergeConfigs(&base, &Config{}, "per-repo")
	if len(preserved.WorktreeFiles) != 1 || preserved.WorktreeFiles[0].Source != "templates/base" {
		t.Fatalf("preserved WorktreeFiles = %#v", preserved.WorktreeFiles)
	}
}

func TestValidateWorktreeFiles(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		destination string
		wantError   string
	}{
		{name: "valid nested paths", source: ".sentei/private/config", destination: ".codex/config.toml"},
		{name: "empty source", source: " ", destination: ".codex/config.toml", wantError: "source"},
		{name: "empty destination", source: "template", destination: "", wantError: "destination"},
		{name: "dot source", source: ".", destination: "config", wantError: "source"},
		{name: "dot destination", source: "template", destination: ".", wantError: "destination"},
		{name: "absolute source", source: "/private/template", destination: "config", wantError: "source"},
		{name: "absolute destination", source: "template", destination: "/tmp/config", wantError: "destination"},
		{name: "source traversal", source: "templates/../secret", destination: "config", wantError: "source"},
		{name: "destination traversal", source: "template", destination: ".codex/../../secret", wantError: "destination"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := parseWorktreeFilesConfig(t, `
worktree_files:
  - source: `+tc.source+`
    destination: `+tc.destination+`
`)
			err := validate(&cfg)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("validate() error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("validate() error = %v, want path error containing %q", err, tc.wantError)
			}
		})
	}
}

func TestValidateWorktreeFilesRejectsDuplicateDestination(t *testing.T) {
	cfg := parseWorktreeFilesConfig(t, `
worktree_files:
  - source: templates/first
    destination: .codex/config.toml
  - source: templates/second
    destination: .codex/config.toml
`)

	err := validate(&cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate destination") {
		t.Fatalf("validate() error = %v, want duplicate destination error", err)
	}
}
