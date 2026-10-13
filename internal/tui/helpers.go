package tui

import (
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func relaunchSentei(repoPath string) tea.Cmd {
	return tea.ExecProcess(relaunchCommand(repoPath), func(err error) tea.Msg {
		return tea.Quit()
	})
}

// relaunchCommand is the sentei that takes over at repoPath after a create,
// clone or migrate.
func relaunchCommand(repoPath string) *exec.Cmd {
	senteiPath, err := os.Executable()
	if err != nil {
		senteiPath = "sentei"
	}
	command := exec.Command(senteiPath, repoPath)
	command.Env = git.Environ()
	return command
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
