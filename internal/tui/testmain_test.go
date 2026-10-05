package tui

import (
	"os"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

// TestMain isolates TMPDIR to a Spotlight-excluded dir so real-git tests don't
// flake on macOS (the indexer transiently holds git object files, breaking
// t.TempDir's RemoveAll). See internal/testtmp.
func TestMain(m *testing.M) {
	_ = os.Setenv("SENTEI_MOTION", "full")
	_ = os.Setenv("TERM", "xterm-256color")
	os.Exit(testtmp.RunWithIsolatedTemp(m))
}
