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

// repoOutcome is what the summary of a create-repository or a clone run shows
// and acts on. The two flows differ in these values and in nothing else.
type repoOutcome struct {
	title    string
	verb     string // names the flow on the failure line: "<verb> failed"
	repoPath string
	branch   string // the default branch, "" when it is not known
	worktree string
	// failed means the run left no usable repository: the summary is a
	// failure screen, and enter must not open sentei in it.
	failed bool
	err    error
	ready  string // follows the repository name on the success line
	rows   string // the rows after Branch: GitHub or Origin
}

func (m Model) repoOutcome() (repoOutcome, bool) {
	switch result := m.repo.result.(type) {
	case repo.CreateResult:
		outcome := repoOutcome{title: titleRepoCreated, verb: "create", repoPath: result.RepoPath, worktree: result.WorktreePath}
		// A Setup-phase failure means the local repository is broken (for
		// example the initial commit could not be made).
		if outcome.failed, outcome.err = result.SetupFailed(); !outcome.failed {
			outcome.branch = "main"
			outcome.ready, outcome.rows = m.githubOutcome(result)
		}
		return outcome, true
	case repo.CloneResult:
		outcome := repoOutcome{
			title: titleRepoCloned, verb: "clone", ready: "ready",
			repoPath: result.RepoPath, worktree: result.WorktreePath, branch: result.DefaultBranch,
		}
		outcome.failed, outcome.err = cloneFailed(result)
		if result.OriginURL != "" {
			outcome.rows = summaryPathRow("Origin", result.OriginURL, m.width)
		}
		return outcome, true
	}
	return repoOutcome{}, false
}

// githubOutcome describes the publish step of a created repository: what
// follows its name on the success line, and the GitHub row. A failed publish
// is a soft failure: the local repository is fine, only unpublished.
func (m Model) githubOutcome(result repo.CreateResult) (ready, row string) {
	ready = "ready"
	failure := ""
	for _, phase := range result.Phases {
		if phase.Name == repo.PhaseGitHub && phase.HasFailures() {
			ready, failure = "ready (local only)", "failed to publish"
			if _, step, _ := progress.FirstFailure([]progress.Phase{phase}); step.Error != nil {
				failure = step.Error.Error()
			}
		}
	}
	switch {
	case result.GitHubURL != "":
		row = summaryRow("GitHub", result.GitHubURL+" "+styleSuccess.Render("\u25cf"))
	case failure != "":
		row = summaryRow("GitHub", styleError.Width(max(m.width-16, 30)).Render(indicatorFailed+" "+failure))
	}
	return ready, row
}

func (m Model) updateRepoSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, keys.Confirm):
			// Never relaunch into a repo whose worktree was never created.
			if outcome, _ := m.repoOutcome(); !outcome.failed && outcome.repoPath != "" {
				return m, relaunchSentei(outcome.repoPath)
			}
			return m, tea.Quit
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewRepoSummary() string {
	outcome, ok := m.repoOutcome()
	if !ok {
		return "  Operation complete.\n"
	}
	var b strings.Builder

	b.WriteString(viewTitle(outcome.title))
	b.WriteString("\n\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")

	if outcome.failed {
		fmt.Fprintf(&b, "  %s %s failed\n\n", styleIndicatorFailed.Render(indicatorFailed), outcome.verb)
		if outcome.err != nil {
			b.WriteString("    " + styleError.Width(max(m.width-8, 30)).Render(outcome.err.Error()))
			b.WriteString("\n\n")
		}
		b.WriteString(summaryPathRow("Path", outcome.repoPath, m.width))
		if outcome.branch != "" {
			b.WriteString(summaryRow("Branch", outcome.branch))
		}
		// A failed run leaves no usable worktree, so there is no cd line to
		// offer and nothing to open.
		b.WriteString("\n")
		b.WriteString(viewSeparator(m.width))
		b.WriteString("\n\n")
		b.WriteString(viewFooter(m.width, quitOnlyFooter))
		b.WriteString("\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  %s %s %s\n\n", styleIndicatorDone.Render(indicatorDone), filepath.Base(outcome.repoPath), outcome.ready)
	b.WriteString(summaryPathRow("Path", outcome.repoPath, m.width))
	b.WriteString(summaryRow("Branch", outcome.branch))
	b.WriteString(outcome.rows)

	worktree := outcome.worktree
	if worktree == "" && outcome.repoPath != "" && outcome.branch != "" {
		worktree = git.WorktreePath(outcome.repoPath, outcome.branch)
	}

	b.WriteString("\n")
	b.WriteString(viewSeparator(m.width))
	b.WriteString("\n\n")
	b.WriteString(m.cdLine(worktree))
	b.WriteString("\n")
	b.WriteString(viewFooter(m.width, repoSummaryFooter))
	b.WriteString("\n")

	return b.String()
}
