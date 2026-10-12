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

	if err := state.Save(dir, &state.State{LifetimeRemoved: 7}); err != nil {
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
	if got.LifetimeRemoved != 7 {
		t.Errorf("round-trip mismatch: LifetimeRemoved = %d, want 7", got.LifetimeRemoved)
	}
}

func TestLoad_FileNotExist_ReturnsEmpty(t *testing.T) {
	s, err := state.Load(t.TempDir())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil state")
	}
	if s.LifetimeRemoved != 0 {
		t.Errorf("expected an empty state, got %+v", s)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sentei.json"), []byte(`{"lifetime_removed":12}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := state.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.LifetimeRemoved != 12 {
		t.Errorf("LifetimeRemoved = %d, want 12", s.LifetimeRemoved)
	}
}

// Files written before the integrations feature was removed still carry its
// list. They must keep loading, and the key goes away on the next save.
func TestLoad_IgnoresRetiredIntegrationsKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sentei.json")
	legacy := `{"integrations":["code-review-graph","cocoindex-code"],"lifetime_removed":3}` + "\n"
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := state.Load(dir)
	if err != nil {
		t.Fatalf("a legacy state file must load: %v", err)
	}
	if s.LifetimeRemoved != 3 {
		t.Errorf("LifetimeRemoved = %d, want 3", s.LifetimeRemoved)
	}

	if err := state.Save(dir, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "integrations") {
		t.Errorf("the retired key should be dropped on save, got:\n%s", data)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sentei.json"), []byte("not-json{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := state.Load(dir); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSave_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	if err := state.Save(dir, &state.State{LifetimeRemoved: 1}); err != nil {
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
	if got.LifetimeRemoved != 1 {
		t.Errorf("LifetimeRemoved = %d, want 1", got.LifetimeRemoved)
	}
	if data[len(data)-1] != '\n' {
		t.Error("expected trailing newline")
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	if err := state.Save(dir, &state.State{LifetimeRemoved: 1}); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(dir, &state.State{LifetimeRemoved: 9}); err != nil {
		t.Fatalf("expected no error on overwrite, got %v", err)
	}

	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.LifetimeRemoved != 9 {
		t.Errorf("LifetimeRemoved after overwrite = %d, want 9", s.LifetimeRemoved)
	}
}
