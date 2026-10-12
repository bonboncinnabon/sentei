package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/repo"
)

func (m Model) updateCloneInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, keys.Back):
			m.view = menuView
			return m, nil

		case key.Matches(msg, keys.Tab):
			return m, switchFormField(&m.repo.cloneFocusedField, &m.repo.urlInput, &m.repo.cloneNameInput)

		case key.Matches(msg, keys.Confirm):
			url := strings.TrimSpace(m.repo.urlInput.Value())
			if url == "" {
				m.repo.validationErr = "repository URL is required"
				return m, nil
			}

			dest := filepath.Join(m.repoPath, m.cloneName())
			if _, err := os.Stat(dest); err == nil {
				m.repo.validationErr = fmt.Sprintf("directory already exists: %s", dest)
				return m, nil
			}
			return m.startClone()
		}

	}
	return m.updateCloneTextInput(msg)
}

// cloneName is the directory the clone lands in: the name field, or the name
// derived from the URL while that field is empty.
func (m Model) cloneName() string {
	if name := strings.TrimSpace(m.repo.cloneNameInput.Value()); name != "" {
		return name
	}
	return repo.DeriveRepoName(m.repo.urlInput.Value())
}

// startClone runs the clone the URL and name fields describe. The input form
// and the `sentei clone --url` confirmation both start it here.
func (m Model) startClone() (tea.Model, tea.Cmd) {
	m.repo.validationErr = ""
	m.repo.events = nil
	m.repo.result = nil
	m.repo.opType = "clone"
	m.progressStartedAt = time.Now()
	m.progressToken++
	m.view = repoProgressView
	return m, m.startRepoPipeline(repo.CloneOptions{
		URL:      strings.TrimSpace(m.repo.urlInput.Value()),
		Location: m.repoPath,
		Name:     m.cloneName(),
	})
}

func (m Model) updateCloneTextInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd, edited := editFormField(m.repo.cloneFocusedField, &m.repo.urlInput, &m.repo.cloneNameInput, msg)
	if !edited {
		return m, cmd
	}
	m.repo.validationErr = ""
	// The name follows the URL until the user types one of their own.
	if m.repo.cloneFocusedField == 1 {
		m.repo.nameManuallyEdited = true
	} else if !m.repo.nameManuallyEdited {
		m.repo.cloneNameInput.SetValue(repo.DeriveRepoName(m.repo.urlInput.Value()))
	}
	return m, cmd
}

func (m Model) viewCloneInput() string {
	var b strings.Builder

	b.WriteString(viewTitle(titleCloneRepo))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	// Both fields render persistently (the huh spike's layout win,
	// without the library): focus moves the accent, never the geometry.
	b.WriteString(inputFieldLabel("Repository URL", m.repo.cloneFocusedField == 0))
	b.WriteString("  " + m.repo.urlInput.View())
	b.WriteString("\n\n")

	cloneDest := filepath.Join(m.repoPath, m.cloneName())

	b.WriteString(inputFieldLabel("Clone to", m.repo.cloneFocusedField == 1))
	b.WriteString("  " + m.repo.cloneNameInput.View())
	b.WriteString("\n")
	// The destination preview lives with the field and tracks the URL live
	// (the huh spike's other win).
	b.WriteString(styleDim.Render(fmt.Sprintf("    → %s", cloneDest)))
	b.WriteString("\n")

	if m.repo.validationErr != "" {
		b.WriteString("\n  " + styleError.Render(m.repo.validationErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, cloneInputFooter))
	b.WriteString("\n")

	return b.String()
}
