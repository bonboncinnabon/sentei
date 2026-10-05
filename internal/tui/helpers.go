package tui

import (
	"os"
	"os/exec"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func relaunchSentei(repoPath string) tea.Cmd {
	senteiPath, err := os.Executable()
	if err != nil {
		senteiPath = "sentei"
	}
	c := exec.Command(senteiPath, repoPath)
	c.Env = os.Environ()
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return tea.Quit()
	})
}

// worktreeLabel returns the canonical display name for a worktree: its
// branch, the short HEAD hash for a detached HEAD (matching the list view),
// or the directory name as a last resort. Every view uses this one label so
// a worktree can always be correlated across screens.
func worktreeLabel(wt git.Worktree) string {
	if branch := stripBranchPrefix(wt.Branch); branch != "" {
		return branch
	}
	switch {
	case wt.IsDetached && len(wt.HEAD) >= 7:
		return wt.HEAD[:7]
	case wt.IsPrunable:
		return "(prunable)"
	}
	return filepath.Base(wt.Path)
}
