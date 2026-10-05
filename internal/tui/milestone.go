package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/state"
)

// milestoneMsg reports the lifetime counter after a removal run was
// recorded; Crossed is the power of ten this run passed, or 0.
type milestoneMsg struct {
	Crossed int
}

// recordRemovals persists the lifetime counter and reports any milestone
// this run crossed. Garnish degrades silently: on any state error the
// whisper simply does not happen and the counter is left untouched.
func recordRemovals(bareDir string, removed int) tea.Cmd {
	return func() tea.Msg {
		if removed <= 0 {
			return milestoneMsg{}
		}
		s, err := state.Load(bareDir)
		if err != nil {
			return milestoneMsg{}
		}
		before := s.LifetimeRemoved
		s.LifetimeRemoved = before + removed
		if err := state.Save(bareDir, s); err != nil {
			return milestoneMsg{}
		}
		return milestoneMsg{Crossed: crossedPowerOfTen(before, s.LifetimeRemoved)}
	}
}

// crossedPowerOfTen returns the largest power of ten p with
// before < p <= after, or 0 when none was crossed.
func crossedPowerOfTen(before, after int) int {
	crossed := 0
	for p := 10; p <= after; p *= 10 {
		if before < p {
			crossed = p
		}
	}
	return crossed
}
