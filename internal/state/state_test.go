package state_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/state"
)

func TestSave_LeavesNoTempFileAndRoundTrips(t *testing.T) {
	dir := t.TempDir()

	if err := state.Save(dir, &state.State{Integrations: []string{"a", "b"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// The atomic temp+fsync+rename must leave no .sentei-*.json temp file behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".sentei-") {
			t.Errorf("leftover temp file after Save: %s", e.Name())
		}
	}

	got, err := state.Load(dir)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(got.Integrations) != 2 || got.Integrations[0] != "a" || got.Integrations[1] != "b" {
		t.Errorf("round-trip mismatch: %v", got.Integrations)
	}
}

func TestLoad_FileNotExist_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()

	s, err := state.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil state")
	}
	if len(s.Integrations) != 0 {
		t.Errorf("expected empty integrations, got %v", s.Integrations)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	data := `{"integrations":["github","linear"]}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sentei.json"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := state.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(s.Integrations) != 2 {
		t.Fatalf("expected 2 integrations, got %d", len(s.Integrations))
	}
	if s.Integrations[0] != "github" || s.Integrations[1] != "linear" {
		t.Errorf("unexpected integrations: %v", s.Integrations)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sentei.json"), []byte("not-json{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := state.Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSave_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	s := &state.State{Integrations: []string{"github"}}

	if err := state.Save(dir, s); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "sentei.json"))
	if err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}

	var got state.State
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("file contains invalid JSON: %v", err)
	}
	if len(got.Integrations) != 1 || got.Integrations[0] != "github" {
		t.Errorf("unexpected integrations: %v", got.Integrations)
	}

	// Verify trailing newline.
	if data[len(data)-1] != '\n' {
		t.Error("expected trailing newline")
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	initial := &state.State{Integrations: []string{"github"}}
	if err := state.Save(dir, initial); err != nil {
		t.Fatal(err)
	}

	updated := &state.State{Integrations: []string{"linear", "jira"}}
	if err := state.Save(dir, updated); err != nil {
		t.Fatalf("expected no error on overwrite, got %v", err)
	}

	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Integrations) != 2 || s.Integrations[0] != "linear" || s.Integrations[1] != "jira" {
		t.Errorf("unexpected integrations after overwrite: %v", s.Integrations)
	}
}

func TestHasIntegration(t *testing.T) {
	s := &state.State{Integrations: []string{"github", "linear"}}

	tests := []struct {
		name string
		want bool
	}{
		{"github", true},
		{"linear", true},
		{"jira", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.HasIntegration(tt.name); got != tt.want {
				t.Errorf("HasIntegration(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
