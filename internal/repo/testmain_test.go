package repo

import (
	"os"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

// TestMain makes the real-git tests hermetic: a controlled global config and
// a Spotlight-excluded temp dir. See internal/testtmp.
func TestMain(m *testing.M) {
	os.Exit(testtmp.RunWithIsolatedTemp(m))
}
