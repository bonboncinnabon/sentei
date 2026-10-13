package tui

import "github.com/bonboncinnabon/sentei/internal/cli"

// SetCloneOpts sets the clone options and starts at the appropriate view.
// If URL is set, starts at cloneConfirmView.
// If nothing is set, starts at cloneInputView (normal flow).
func (m *Model) SetCloneOpts(opts *CloneOpts) {
	m.cloneOpts = opts

	if opts.URL != "" {
		m.repo.urlInput.SetValue(opts.URL)
	}
	if opts.Name != "" {
		m.repo.cloneNameInput.SetValue(opts.Name)
		m.repo.nameManuallyEdited = true
	}

	if opts.URL != "" {
		m.view = cloneConfirmView
	} else {
		m.view = cloneInputView
		// The form opens ready to type, as it does from the menu.
		m.repo.urlInput.Focus()
	}
}

// cloneConfirmationVM builds the ConfirmationViewModel for the clone flow.
func (m Model) cloneConfirmationVM() ConfirmationViewModel {
	url := m.repo.urlInput.Value()
	name := m.cloneName()

	items := []ConfirmationItem{
		{Label: "URL:", Value: url},
	}
	if name != "" {
		items = append(items, ConfirmationItem{Label: "Name:", Value: name})
	}

	flags := make(map[string]string)
	if url != "" {
		flags["url"] = url
	}
	if name != "" {
		flags["name"] = name
	}

	return ConfirmationViewModel{
		Width:      m.width,
		Title:      titleConfirmClone,
		Items:      items,
		CLICommand: cli.BuildFlagString("sentei clone", flags),
	}
}

func (m Model) viewCloneConfirm() string {
	return m.cloneConfirmationVM().View()
}
