package cleanup

import (
	"os"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

// TestMain gives the real-git tests a controlled global git config and a
// Spotlight-excluded TMPDIR. See internal/testtmp.
func TestMain(m *testing.M) {
	os.Exit(testtmp.RunWithIsolatedTemp(m))
}
