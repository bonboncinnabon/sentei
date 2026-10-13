package tui

import (
	"path/filepath"
	"strings"
	"testing"
)

// A hook or a git alias that starts sentei exports GIT_DIR to it. The sentei
// started after a create, clone or migrate works on the new repository.
func TestRelaunchCommand_DropsInheritedGitDir(t *testing.T) {
	foreign, repoPath := t.TempDir(), t.TempDir()
	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign)
	t.Setenv("SENTEI_TEST_MARKER", "kept")

	command := relaunchCommand(repoPath)

	if got := command.Args[len(command.Args)-1]; got != repoPath {
		t.Errorf("last argument = %q, want the repository path %q", got, repoPath)
	}
	kept := false
	for _, entry := range command.Env {
		name, value, _ := strings.Cut(entry, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE":
			t.Errorf("%s reaches the relaunched sentei", entry)
		case "SENTEI_TEST_MARKER":
			kept = value == "kept"
		}
	}
	// A nil Env would inherit everything, GIT_DIR included.
	if !kept {
		t.Error("the relaunched sentei lost the rest of the environment")
	}
}
