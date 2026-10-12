package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func (m Model) updateRepoName(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, keys.Back):
			m.view = menuView
			return m, nil

		case key.Matches(msg, keys.Tab):
			return m, switchFormField(&m.repo.focusedField, &m.repo.nameInput, &m.repo.locationInput)

		case key.Matches(msg, keys.Confirm):
			name := strings.TrimSpace(m.repo.nameInput.Value())
			location := m.repoLocation()
			dest := filepath.Join(location, name)

			switch {
			case name == "":
				m.repo.validationErr = "repository name is required"
			case strings.Contains(name, " "):
				m.repo.validationErr = "repository name cannot contain spaces"
			case name == "." || name == ".." || strings.ContainsAny(name, `/\`):
				m.repo.validationErr = "repository name must be a directory name, not a path"
			case !isDir(location):
				m.repo.validationErr = fmt.Sprintf("location is not an existing directory: %s", location)
			case pathExists(dest):
				m.repo.validationErr = fmt.Sprintf("directory already exists: %s", dest)
			default:
				m.repo.validationErr = ""
			}
			if m.repo.validationErr != "" {
				return m, nil
			}

			m.repo.focusedField = 0
			m.repo.locationInput.Blur()
			m.repo.nameInput.Blur()
			m.repo.publishGitHub = false
			m.repo.ghStatus = ""
			m.repo.optionsCursor = repoOptPublish
			m.view = repoOptionsView
			return m, checkGitHubAuth(m.shell)
		}

	}
	return m.updateRepoNameInput(msg)
}

// repoLocation is the directory the new repository is created in: the
// Location field with ~ expanded, resolved against the directory sentei was
// opened on. An empty field is that directory, as its placeholder shows.
func (m Model) repoLocation() string {
	location := strings.TrimSpace(m.repo.locationInput.Value())
	if location == "~" || strings.HasPrefix(location, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			location = filepath.Join(home, location[1:])
		}
	}
	if !filepath.IsAbs(location) {
		location = filepath.Join(m.repoPath, location)
	}
	return filepath.Clean(location)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func (m Model) updateRepoNameInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd, edited := editFormField(m.repo.focusedField, &m.repo.nameInput, &m.repo.locationInput, msg)
	if edited {
		m.repo.validationErr = ""
	}
	return m, cmd
}

func (m Model) viewRepoName() string {
	var b strings.Builder

	b.WriteString(viewTitle(titleCreateRepo))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	b.WriteString(inputFieldLabel("Repository name", m.repo.focusedField == 0))
	b.WriteString("  " + m.repo.nameInput.View())
	b.WriteString("\n\n")

	b.WriteString(inputFieldLabel("Location", m.repo.focusedField == 1))
	b.WriteString("  " + m.repo.locationInput.View())
	b.WriteString("\n")
	// The repository is a directory named after it inside Location.
	dest := filepath.Join(m.repoLocation(), strings.TrimSpace(m.repo.nameInput.Value()))
	b.WriteString(styleDim.Render(fmt.Sprintf("    → %s", dest)))
	b.WriteString("\n")

	if m.repo.validationErr != "" {
		b.WriteString("\n  " + styleError.Render(m.repo.validationErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, repoNameFooter))
	b.WriteString("\n")

	return b.String()
}
