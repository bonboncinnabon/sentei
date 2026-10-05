package tui

import (
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/state"
)

// loadRepoState resolves the repo's bare directory via git and loads sentei
// state from it. Callers that can render without state should fall back to an
// empty State on error.
func (m Model) loadRepoState() (*state.State, error) {
	bareDir, err := git.CommonDir(m.runner, m.repoPath)
	if err != nil {
		return nil, err
	}
	return state.Load(bareDir)
}
