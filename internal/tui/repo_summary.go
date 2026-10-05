package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// cloneFailed reports whether the clone left the repo in a failed state and, if
// so, the error from the first failed step.
func cloneFailed(result repo.CloneResult) (bool, error) {
	if result.Err != nil {
		return true, result.Err
	}
	if !result.HasFailures() {
		return false, nil
	}
	_, step, _ := progress.FirstFailure(result.Phases)
	return true, step.Error
}

func (m Model) updateRepoSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		var repoPath string
		canRelaunch := false
		switch r := m.repo.result.(type) {
		case repo.CreateResult:
			repoPath = r.RepoPath
			hardFailed, _ := r.SetupFailed()
			canRelaunch = !hardFailed
		case repo.CloneResult:
			repoPath = r.RepoPath
			canRelaunch = !r.HasFailures()
		}

		switch {
		case key.Matches(msg, keys.Confirm):
			// Never relaunch into a repo whose worktree was never created.
			if canRelaunch && repoPath != "" {
				return m, relaunchSentei(repoPath)
			}
			return m, tea.Quit
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewRepoSummary() string {
	var b strings.Builder

	switch result := m.repo.result.(type) {
	case repo.CreateResult:
		return m.viewCreateRepoSummary(result)
	case repo.CloneResult:
		return m.viewCloneRepoSummary(result)
	}

	b.WriteString("  Operation complete.\n")
	return b.String()
}

func (m Model) viewCreateRepoSummary(result repo.CreateResult) string {
	var b strings.Builder

	repoName := filepath.Base(result.RepoPath)

	b.WriteString(viewTitle(titleRepoCreated))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	// A Setup-phase failure means the local repo is broken (e.g. the initial
	// commit could not be made). Render a failure screen, not "ready".
	if hardFailed, hardErr := result.SetupFailed(); hardFailed {
		fmt.Fprintf(&b, "  %s create failed\n\n",
			styleIndicatorFailed.Render(indicatorFailed))
		if hardErr != nil {
			errWidth := max(m.width-8, 30)
			b.WriteString("    " + styleError.Width(errWidth).Render(hardErr.Error()))
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Path"), result.RepoPath)
		b.WriteString("\n")
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		b.WriteString(viewFooter(m.width, quitOnlyFooter))
		b.WriteString("\n")
		return b.String()
	}

	// GitHub publish is a soft failure: the local repo is fine, just unpublished.
	ghFailed := false
	var ghErrMsg string
	for _, phase := range result.Phases {
		if phase.Name == repo.PhaseGitHub && phase.HasFailures() {
			ghFailed = true
			for _, step := range phase.Steps {
				if step.Error != nil {
					ghErrMsg = step.Error.Error()
					break
				}
			}
			break
		}
	}

	if ghFailed {
		fmt.Fprintf(&b, "  %s %s ready (local only)\n\n",
			styleIndicatorDone.Render(indicatorDone), repoName)
	} else {
		fmt.Fprintf(&b, "  %s %s ready\n\n",
			styleIndicatorDone.Render(indicatorDone), repoName)
	}

	fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Path"), result.RepoPath)
	fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Branch"), "main")

	if result.GitHubURL != "" {
		fmt.Fprintf(&b, "    %-10s %s %s\n", styleDim.Render("GitHub"),
			result.GitHubURL, styleSuccess.Render("\u25cf"))
	} else if ghFailed {
		msg := "failed to publish"
		if ghErrMsg != "" {
			msg = ghErrMsg
		}
		errWidth := max(m.width-14, 30) // 14 chars for indentation + label
		wrappedErr := styleError.Width(errWidth).Render(indicatorFailed + " " + msg)
		fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("GitHub"), wrappedErr)
	}

	worktreePath := result.WorktreePath
	if worktreePath == "" && result.RepoPath != "" {
		worktreePath = git.WorktreePath(result.RepoPath, "main")
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "    cd %s\n\n", styleDim.Render(worktreePath))
	b.WriteString(viewFooter(m.width, repoSummaryFooter))
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewCloneRepoSummary(result repo.CloneResult) string {
	var b strings.Builder

	repoName := filepath.Base(result.RepoPath)

	b.WriteString(viewTitle(titleRepoCloned))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	if failed, failErr := cloneFailed(result); failed {
		fmt.Fprintf(&b, "  %s clone failed\n\n",
			styleIndicatorFailed.Render(indicatorFailed))
		if failErr != nil {
			errWidth := max(m.width-8, 30)
			b.WriteString("    " + styleError.Width(errWidth).Render(failErr.Error()))
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Path"), result.RepoPath)
		if result.DefaultBranch != "" {
			fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Branch"), result.DefaultBranch)
		}
		// A failed clone never has a usable worktree (tracking-only failures are
		// StepSkipped, not failures), so there is no cd hint to offer.
		b.WriteString("\n")
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		b.WriteString(viewFooter(m.width, quitOnlyFooter))
		b.WriteString("\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  %s %s ready\n\n",
		styleIndicatorDone.Render(indicatorDone), repoName)

	fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Path"), result.RepoPath)
	fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Branch"), result.DefaultBranch)
	if result.OriginURL != "" {
		fmt.Fprintf(&b, "    %-10s %s\n", styleDim.Render("Origin"), result.OriginURL)
	}

	worktreePath := result.WorktreePath
	if worktreePath == "" && result.RepoPath != "" && result.DefaultBranch != "" {
		worktreePath = git.WorktreePath(result.RepoPath, result.DefaultBranch)
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "    cd %s\n\n", styleDim.Render(worktreePath))
	b.WriteString(viewFooter(m.width, repoSummaryFooter))
	b.WriteString("\n")

	return b.String()
}
