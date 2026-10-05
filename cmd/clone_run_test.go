package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestCloneResultErrorPropagatesContractError(t *testing.T) {
	want := errors.New("delivery")
	if got := cloneResultError(repo.CloneResult{Err: want}); !errors.Is(got, want) {
		t.Fatalf("error = %v", got)
	}
}

func TestRunClone_ParseError(t *testing.T) {
	err := RunClone([]string{"--no-such-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRunClone_MissingURL(t *testing.T) {
	err := RunClone(nil)
	if err == nil {
		t.Fatal("expected error for missing --url")
	}
	if !strings.Contains(err.Error(), "missing required flag: --url") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunClone_Success(t *testing.T) {
	source := setupBareRepo(t)
	dest := t.TempDir()
	t.Chdir(dest)

	var err error
	out := captureStdout(t, func() {
		err = RunClone([]string{"--url", source, "--name", "cloned"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Cloned to") {
		t.Errorf("expected 'Cloned to' confirmation, got:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(dest, "cloned")); statErr != nil {
		t.Errorf("expected cloned directory to exist: %v", statErr)
	}
}

func TestRunClone_FailureReportsFailedSteps(t *testing.T) {
	t.Chdir(t.TempDir())

	var err error
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			err = RunClone([]string{"--url", "/nonexistent/path/to/repo.git"})
		})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent clone source")
	}
	if !strings.Contains(err.Error(), "clone failed") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "✗") {
		t.Errorf("expected failed step on stderr, got:\n%s", stderr)
	}
}
