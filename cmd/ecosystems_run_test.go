package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"shorter than max", "abc", 5, "abc"},
		{"exactly max", "abcde", 5, "abcde"},
		{"longer than max", "abcdef", 4, "abc…"},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.s, tt.max); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestRunEcosystems_ListsEmbeddedEcosystems(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()

	var err error
	out := captureStdout(t, func() {
		err = RunEcosystems([]string{dir})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "Ecosystems (") {
		t.Errorf("expected ecosystems header, got:\n%s", out)
	}
	for _, column := range []string{"NAME", "DETECT FILES", "INSTALL", "SOURCE", "STATUS"} {
		if !strings.Contains(out, column) {
			t.Errorf("expected column header %q, got:\n%s", column, out)
		}
	}
	if !strings.Contains(out, "embedded") {
		t.Errorf("expected embedded ecosystems to be listed, got:\n%s", out)
	}
}

func TestRunEcosystems_MarksDisabledEcosystems(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, ".sentei.yaml"), "ecosystems:\n  - name: pnpm\n    enabled: false\n")

	var err error
	out := captureStdout(t, func() {
		err = RunEcosystems([]string{dir})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "disabled") {
		t.Errorf("expected disabled ecosystem status, got:\n%s", out)
	}
}
