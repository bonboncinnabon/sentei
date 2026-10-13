package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/ecosystem"
	"github.com/bonboncinnabon/sentei/internal/git"
)

// defaultBaseBranch is the base-branch input's construction default; the
// reset on menu entry restores it.
const defaultBaseBranch = "main"

// withCreateFlowReset restores the create flow to its construction defaults
// so a menu entry never inherits inputs, toggles, events, or results from a
// previous run (completed or abandoned).
func (m Model) withCreateFlowReset() Model {
	m.create.branchInput.SetValue("")
	m.create.branchInput.Focus()
	m.create.baseInput.SetValue(defaultBaseBranch)
	m.create.baseInput.Blur()
	m.create.focusedField = 0
	m.create.validationErr = ""
	m.create.ecoEnabled = make(map[string]bool)
	m.create.mergeBase = true
	m.create.mergeDefaultFor = ""
	m.create.existingBranch = ""
	m.create.copyEnvFiles = true
	m.create.optionsCursor = 0
	m.create.events = nil
	m.create.result = nil
	return m
}

type branchValidationError struct {
	message string
}

func validateBranchName(name string, existingWorktrees []string) *branchValidationError {
	if name == "" {
		return &branchValidationError{message: "branch name is required"}
	}
	if strings.Contains(name, " ") {
		return &branchValidationError{message: "branch name cannot contain spaces"}
	}
	if strings.Contains(name, "..") {
		return &branchValidationError{message: "branch name cannot contain '..'"}
	}
	sanitized := git.WorktreeDirName(name)
	for _, wt := range existingWorktrees {
		if strings.HasSuffix(wt, "/"+sanitized) || wt == sanitized {
			return &branchValidationError{message: fmt.Sprintf("worktree %q already exists", sanitized)}
		}
	}
	return nil
}

func (m Model) updateCreateBranch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, keys.Back):
			m.view = menuView
			return m, nil

		case key.Matches(msg, keys.Tab):
			return m, switchFormField(&m.create.focusedField, &m.create.branchInput, &m.create.baseInput)

		case key.Matches(msg, keys.QuickCreate):
			if !m.validateCreateBranch() {
				return m, nil
			}
			m.prepareCreateOptions()
			m.startCreation()
			m.progressStartedAt = time.Now()
			m.progressToken++
			m.view = createProgressView
			return m, m.waitForCreateEvent()

		case key.Matches(msg, keys.Confirm):
			if !m.validateCreateBranch() {
				return m, nil
			}
			m.prepareCreateOptions()
			m.view = createOptionsView
			return m, nil
		}

	}
	return m.updateCreateBranchInput(msg)
}

// validateCreateBranch checks the branch name in the form against the
// worktrees that exist and records why it was refused.
func (m *Model) validateCreateBranch() bool {
	var existingPaths []string
	for _, wt := range m.currentWorktrees() {
		existingPaths = append(existingPaths, wt.Path)
	}
	m.create.validationErr = ""
	if err := validateBranchName(m.create.branchInput.Value(), existingPaths); err != nil {
		m.create.validationErr = err.message
	}
	return m.create.validationErr == ""
}

func (m Model) updateCreateBranchInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd, edited := editFormField(m.create.focusedField, &m.create.branchInput, &m.create.baseInput, msg)
	if edited {
		m.create.validationErr = ""
	}
	return m, cmd
}

// defaultMergeBase looks up whether the branch already exists and turns the
// merge off when it does, locally or only on origin: the user is opening that
// branch, and merging the base would add a commit to it. --merge-base keeps
// it on. The default is set once per branch name, so a toggle survives a
// return to the branch form.
func (m *Model) defaultMergeBase() {
	branch := m.create.branchInput.Value()
	if branch == "" || branch == m.create.mergeDefaultFor {
		return
	}
	m.create.mergeDefaultFor = branch
	m.create.existingBranch = creator.ExistingBranch(m.runner, m.repoPath, branch)
	requested := m.createOpts != nil && m.createOpts.MergeBase
	m.create.mergeBase = requested || m.create.existingBranch == ""
}

// branchOrigin says where the branch of a new worktree comes from. The base
// only starts a branch that exists nowhere; existing is what
// creator.ExistingBranch found for it.
func branchOrigin(branch, base, existing string) string {
	switch existing {
	case "":
		return "from " + base
	case branch:
		return "existing branch, not created from " + base
	default:
		return fmt.Sprintf("tracks %s, not created from %s", existing, base)
	}
}

// createSubtitle is the line under the title of the options and progress
// screens: the branch and where it comes from.
func (m Model) createSubtitle() string {
	branch := m.create.branchInput.Value()
	return fmt.Sprintf("%s \u2192 %s", branch, branchOrigin(branch, m.create.baseInput.Value(), m.create.existingBranch))
}

// prepareCreateOptions sets the options for the branch in the form. It offers
// the ecosystems detected in the source worktree: a row starts checked, and a
// choice already made for it (a toggle before going back to this form, or
// --ecosystems) stands.
func (m *Model) prepareCreateOptions() {
	m.defaultMergeBase()
	m.create.ecosystems = nil
	sourceWT := m.findSourceWorktree()
	if m.cfg == nil || sourceWT == "" {
		return
	}
	detected, _ := ecosystem.NewRegistry(m.cfg.Ecosystems).Detect(sourceWT)
	for _, eco := range detected {
		m.create.ecosystems = append(m.create.ecosystems, eco.Config)
		if _, chosen := m.create.ecoEnabled[eco.Name]; !chosen {
			m.create.ecoEnabled[eco.Name] = true
		}
	}
}

// findSourceWorktree returns the worktree that ecosystems are detected in and
// env files are copied from.
func (m Model) findSourceWorktree() string {
	return creator.SourceWorktree(m.currentWorktrees())
}

// currentWorktrees returns the repository's worktrees for the create flow.
// The list loads in the background together with every worktree's status, so
// it can still be empty when the form is submitted, above all on a direct
// `sentei create`. An empty list is read from git here instead of being taken
// to mean "no worktrees".
func (m Model) currentWorktrees() []git.Worktree {
	if len(m.remove.worktrees) > 0 {
		return m.remove.worktrees
	}
	worktrees, _ := git.ListWorktrees(m.runner, m.repoPath)
	return worktrees
}

func (m Model) viewCreateBranch() string {
	var b strings.Builder

	b.WriteString(viewTitle(titleCreateWorktree))
	b.WriteString("\n\n")

	b.WriteString(styleDim.Render(truncateWithEllipsis(fmt.Sprintf("  %s %s %s", filepath.Base(m.repoPath), "\u00b7", m.repoPath), max(m.width, 40))))
	b.WriteString("\n\n")

	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	b.WriteString(inputFieldLabel("Branch name", m.create.focusedField == 0))
	b.WriteString("  " + m.create.branchInput.View())
	b.WriteString("\n")
	if m.create.validationErr != "" {
		b.WriteString(formErrorLine(m.create.validationErr, m.width))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(inputFieldLabel("Base branch", m.create.focusedField == 1))
	b.WriteString("  " + m.create.baseInput.View())
	b.WriteString("\n\n")

	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(viewFooter(m.width, createBranchFooter))
	b.WriteString("\n")

	return b.String()
}
