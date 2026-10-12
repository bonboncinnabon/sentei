package tui

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cli"
)

// SetCreateOpts applies the flags of `sentei create` and picks the first view:
// the confirmation when branch and base are both given, the options screen
// when only the branch is, the branch form otherwise.
func (m *Model) SetCreateOpts(opts *CreateOpts) {
	m.createOpts = opts

	if opts.Branch != "" {
		m.create.branchInput.SetValue(opts.Branch)
	}
	if opts.Base != "" {
		m.create.baseInput.SetValue(opts.Base)
	}
	m.prepareCreateOptions()

	// --ecosystems names the installs. On the options screen it preselects
	// among the detected ecosystems, which stay editable there.
	direct := opts.Branch != "" && opts.Base != ""
	if direct || len(opts.Ecosystems) > 0 {
		m.create.ecoEnabled = make(map[string]bool, len(opts.Ecosystems))
		for _, name := range opts.Ecosystems {
			m.create.ecoEnabled[name] = true
		}
	}

	switch {
	case direct:
		// No options screen follows, so the flags are the whole decision,
		// exactly as in the non-interactive run.
		m.create.mergeBase = opts.MergeBase
		m.create.copyEnvFiles = opts.CopyEnv
		m.offerConfiguredEcosystems(opts.Ecosystems)
		m.view = createConfirmView
	case opts.Branch != "":
		m.view = createOptionsView
	default:
		m.view = createBranchView
	}
}

// offerConfiguredEcosystems adds the named ecosystems that were not detected
// in the source worktree: the non-interactive run installs any configured
// ecosystem that --ecosystems names, detected or not.
func (m *Model) offerConfiguredEcosystems(names []string) {
	if m.cfg == nil {
		return
	}
	offered := make(map[string]bool, len(m.create.ecosystems))
	for _, eco := range m.create.ecosystems {
		offered[eco.Name] = true
	}
	for _, eco := range m.cfg.Ecosystems {
		if slices.Contains(names, eco.Name) && !offered[eco.Name] {
			m.create.ecosystems = append(m.create.ecosystems, eco)
		}
	}
}

// createConfirmationVM builds the ConfirmationViewModel for the create flow.
func (m Model) createConfirmationVM() ConfirmationViewModel {
	branch := m.create.branchInput.Value()
	base := m.create.baseInput.Value()

	items := []ConfirmationItem{
		{Label: "Branch:", Value: branch},
		{Label: "Base:", Value: base},
	}

	var enabledEcos []string
	for _, eco := range m.create.ecosystems {
		if m.create.ecoEnabled[eco.Name] {
			enabledEcos = append(enabledEcos, eco.Name)
		}
	}
	if len(enabledEcos) > 0 {
		items = append(items, ConfirmationItem{
			Label: "Ecosystems:",
			Value: strings.Join(enabledEcos, ", "),
		})
	}

	mergeBase := "no"
	if m.create.mergeBase {
		mergeBase = "yes"
	}
	items = append(items, ConfirmationItem{Label: "Merge base:", Value: mergeBase})

	copyEnv := "no"
	if m.create.copyEnvFiles {
		copyEnv = "yes"
	}
	items = append(items, ConfirmationItem{Label: "Copy env:", Value: copyEnv})
	if m.cfg != nil && len(m.cfg.WorktreeFiles) > 0 {
		items = append(items, ConfirmationItem{
			Label: worktreeFilesLabel,
			Value: automaticWorktreeFiles(len(m.cfg.WorktreeFiles)),
		})
	}

	// Build CLI command from current model state.
	flags := make(map[string]string)
	if branch != "" {
		flags["branch"] = branch
	}
	if base != "" {
		flags["base"] = base
	}
	if len(enabledEcos) > 0 {
		flags["ecosystems"] = strings.Join(enabledEcos, ",")
	}
	if m.create.mergeBase {
		flags["merge-base"] = "true"
	}
	if m.create.copyEnvFiles {
		flags["copy-env"] = "true"
	}

	return ConfirmationViewModel{
		Width:      m.width,
		Title:      titleConfirmCreate,
		Items:      items,
		CLICommand: cli.BuildFlagString("sentei create", flags),
	}
}

func (m Model) updateCreateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ConfirmProceedMsg:
		m.startCreation()
		m.progressStartedAt = time.Now()
		m.progressToken++
		m.view = createProgressView
		return m, m.waitForCreateEvent()

	case ConfirmBackMsg:
		if m.createOpts != nil {
			return m, tea.Quit
		}
		m.view = createOptionsView
		return m, nil
	}

	if cmd := UpdateConfirmation(msg); cmd != nil {
		return m, cmd
	}

	return m, nil
}

func (m Model) viewCreateConfirm() string {
	return m.createConfirmationVM().View()
}
