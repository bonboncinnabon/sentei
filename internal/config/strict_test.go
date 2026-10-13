package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".sentei.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFile_RejectsUnknownKeys(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "misspelled top-level key",
			content: "ecosytems: []\n",
			want:    []string{`line 1: unknown key "ecosytems"`},
		},
		{
			name:    "misspelled ecosystem key",
			content: "ecosystems:\n  - name: pnpm\n    instal:\n      command: pnpm i\n",
			want:    []string{`line 3: unknown key "instal"`},
		},
		{
			name:    "misspelled install key",
			content: "ecosystems:\n  - name: pnpm\n    install:\n      comand: pnpm i\n",
			want:    []string{`line 4: unknown key "comand"`},
		},
		{
			name:    "misspelled worktree file key",
			content: "worktree_files:\n  - source: a\n    dest: b\n",
			want:    []string{`line 3: unknown key "dest"`},
		},
		{
			name:    "every unknown key is reported, retired keys are not",
			content: "protected_branches: [main]\nfoo: 1\nbar: 2\n",
			want:    []string{`line 2: unknown key "foo"`, `line 3: unknown key "bar"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfigFile(t, tt.content)
			_, err := loadFile(path)
			if err == nil {
				t.Fatal("expected an error for an unknown key")
			}
			for _, want := range append(tt.want, path) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err, want)
				}
			}
			for _, leaked := range []string{"config.Config", "config.EcosystemConfig", "protected_branches"} {
				if strings.Contains(err.Error(), leaked) {
					t.Errorf("error = %q, must not mention %q", err, leaked)
				}
			}
		})
	}
}

func TestLoadFile_AcceptsRetiredKeys(t *testing.T) {
	cfg, err := loadFile(writeConfigFile(t, `
ecosystems:
  - name: node
    detect:
      files: [package.json]
    install:
      command: npm install
      workspace_detect: package.json
      workspace_install: npm install --workspaces
      parallel: true
    post_install:
      - npm run build
protected_branches: [main]
integrations_enabled: [code-review-graph]
`))
	if err != nil {
		t.Fatalf("a config written for an earlier version must keep loading: %v", err)
	}
	if len(cfg.Ecosystems) != 1 || cfg.Ecosystems[0].Install.Command != "npm install" {
		t.Errorf("ecosystems = %+v, want the node entry with its install command", cfg.Ecosystems)
	}
}

func TestLoadFile_TypeErrorsAreKept(t *testing.T) {
	_, err := loadFile(writeConfigFile(t, "ecosystems:\n  - name: pnpm\n    enabled: sometimes\n"))
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Errorf("error = %v, want the yaml type error for line 3", err)
	}
}

func TestLoadFile_EmptyFile(t *testing.T) {
	for _, content := range []string{"", "# nothing configured yet\n"} {
		cfg, err := loadFile(writeConfigFile(t, content))
		if err != nil || cfg == nil {
			t.Errorf("loadFile(%q) = %v, %v; want an empty config", content, cfg, err)
		}
	}
}
