package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
)

// resolvedCleanupOpts returns the effective cleanup options, using defaults
// when no options have been explicitly set.
func (m Model) resolvedCleanupOpts() cleanup.Options {
	opts := cleanup.Options{}
	if m.cleanupOpts != nil {
		opts = *m.cleanupOpts
	}
	if opts.Mode == "" {
		opts.Mode = cleanup.ModeSafe
	}
	return opts
}

// cleanupCLICommand renders opts as the non-interactive command that does the
// same thing. Every option that changes the run appears in it.
func cleanupCLICommand(opts cleanup.Options) string {
	flags := map[string]string{"mode": string(opts.Mode)}
	if opts.DryRun {
		flags["dry-run"] = "true"
	}
	if opts.Force {
		flags["force"] = "true"
	}
	return BuildCLICommand("cleanup", flags)
}

// cleanupConfirmationVM builds the ConfirmationViewModel for the cleanup flow.
func (m Model) cleanupConfirmationVM() ConfirmationViewModel {
	opts := m.resolvedCleanupOpts()

	dryRun := "no"
	if opts.DryRun {
		dryRun = "yes"
	}

	// --force is the one option that discards unmerged work, and only
	// aggressive mode acts on it.
	force := "no"
	switch {
	case opts.Force && opts.Mode == cleanup.ModeAggressive:
		force = styleWarning.Render("yes (unmerged branches are deleted too)")
	case opts.Force:
		force = "yes (no effect in safe mode)"
	}

	return ConfirmationViewModel{
		Width: m.width,
		Title: titleConfirmCleanup,
		Items: []ConfirmationItem{
			{Label: "Mode:", Value: string(opts.Mode)},
			{Label: "Dry run:", Value: dryRun},
			{Label: "Force:", Value: force},
		},
		CLICommand: cleanupCLICommand(opts),
	}
}

func (m Model) viewCleanupConfirm() string {
	return m.cleanupConfirmationVM().View()
}

// startCleanupRun executes cleanup with opts and switches to the result view,
// which shows the run in progress and then its outcome.
func (m Model) startCleanupRun(opts cleanup.Options) (tea.Model, tea.Cmd) {
	m.view = cleanupResultView
	m.cleanupResult = nil
	m.cleanupRanMode = opts.Mode
	return m, runCleanupWithOpts(m.runner, m.repoPath, opts)
}

// cleanupRan returns the options of the run the result view reports: the
// flags sentei was launched with, or else the mode picked on the preview,
// which never forces and never dry-runs.
func (m Model) cleanupRan() cleanup.Options {
	if m.cleanupOpts != nil {
		return m.resolvedCleanupOpts()
	}
	return cleanup.Options{Mode: m.cleanupRanMode}
}

// runCleanupWithOpts runs cleanup with the given options.
func runCleanupWithOpts(runner git.CommandRunner, repoPath string, opts cleanup.Options) tea.Cmd {
	return func() tea.Msg {
		result := cleanup.Run(runner, repoPath, opts, func(_ cleanup.Event) {})
		return standaloneCleanupDoneMsg{result: result}
	}
}
