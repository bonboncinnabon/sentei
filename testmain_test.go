package main

import (
	"os"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

// TestMain gives the tests a controlled global git config and a
// Spotlight-excluded TMPDIR. run() calls git in-process, so without this the
// developer's own config (hooks, aliases, status and diff settings) decides
// what those calls see. See internal/testtmp.
func TestMain(m *testing.M) {
	os.Exit(testtmp.RunWithIsolatedTemp(m))
}
