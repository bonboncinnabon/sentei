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
			if m.repo.cloneFocusedField == 0 {
				m.repo.cloneFocusedField = 1
				m.repo.urlInput.Blur()
				m.repo.cloneNameInput.CursorEnd()
				return m, m.repo.cloneNameInput.Focus()
			}
			m.repo.cloneFocusedField = 0
			m.repo.cloneNameInput.Blur()
			m.repo.urlInput.CursorEnd()
			return m, m.repo.urlInput.Focus()

		case key.Matches(msg, keys.Confirm):
			url := strings.TrimSpace(m.repo.urlInput.Value())
			if url == "" {
				m.repo.validationErr = "repository URL is required"
				return m, nil
			}

			name := strings.TrimSpace(m.repo.cloneNameInput.Value())
			if name == "" {
				name = repo.DeriveRepoName(url)
			}

			// Use the repo path as location (parent directory for non-bare)
			location := m.repoPath
			if location == "." {
				var err error
				location, err = os.Getwd()
				if err != nil {
					location = "."
				}
			}

			dest := filepath.Join(location, name)
			if _, err := os.Stat(dest); err == nil {
				m.repo.validationErr = fmt.Sprintf("directory already exists: %s", dest)
				return m, nil
			}

			m.repo.validationErr = ""
			opts := repo.CloneOptions{
				URL:      url,
				Location: location,
				Name:     name,
			}
			m.repo.events = nil
			m.repo.result = nil
			m.repo.opType = "clone"
			m.progressStartedAt = time.Now()
			m.progressToken++
			m.view = repoProgressView
			return m, m.startRepoPipeline(opts)
		}

	}
	return m.updateCloneTextInput(msg)
}

func (m Model) updateCloneTextInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.repo.validationErr = ""
	var cmd tea.Cmd
	if m.repo.cloneFocusedField == 0 {
		prevURL := m.repo.urlInput.Value()
		m.repo.urlInput, cmd = m.repo.urlInput.Update(msg)
		newURL := m.repo.urlInput.Value()
		if newURL != prevURL && !m.repo.nameManuallyEdited {
			m.repo.cloneNameInput.SetValue(repo.DeriveRepoName(newURL))
		}
	} else {
		m.repo.nameManuallyEdited = true
		m.repo.cloneNameInput, cmd = m.repo.cloneNameInput.Update(msg)
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

	// Derive clone location for display
	location := m.repoPath
	if location == "." {
		if wd, err := os.Getwd(); err == nil {
			location = wd
		}
	}
	name := m.repo.cloneNameInput.Value()
	if name == "" {
		name = repo.DeriveRepoName(m.repo.urlInput.Value())
	}
	cloneDest := filepath.Join(location, name)

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
