package tui

import (
	"os"

	"charm.land/bubbles/v2/textinput"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// NewModel builds a model that shows the worktree list for worktrees, without
// a menu or a repository context. Production code always constructs its model
// with NewMenuModel; this is the short form the list, confirm and removal
// tests use.
func NewModel(worktrees []git.Worktree, runner git.CommandRunner, repoPath string) Model {
	ti := textinput.New()
	ti.Prompt = "filter: "

	m := Model{
		view:     listView,
		runner:   runner,
		repoPath: repoPath,
		remove: removeState{
			worktrees:     worktrees,
			selected:      make(map[string]bool),
			sortField:     SortByAge,
			sortAscending: true,
			filterInput:   ti,
		},
		width:            80,
		height:           20,
		bar:              newOverallBar(),
		watch:            newElapsedTicker(),
		motionPreference: motionPreference(os.Getenv),
	}
	m.reindex()
	return m
}
