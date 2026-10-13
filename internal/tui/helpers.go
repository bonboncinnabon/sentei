package tui

import (
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
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

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
