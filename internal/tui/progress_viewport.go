package tui

import "github.com/bonboncinnabon/sentei/internal/progress"

type progressViewportTier uint8

const (
	progressViewportEmergency progressViewportTier = iota
	progressViewportMinimal
	progressViewportCompact
	progressViewportNormal
)

type ProgressViewport struct {
	History        []progress.PhaseState
	HistoryOmitted int
	Focus          *progress.PhaseState
	Queued         int
	DetailRows     int
	Tier           progressViewportTier
}

func BuildProgressViewport(phases []progress.PhaseState, rows int, completed bool) ProgressViewport {
	viewport := ProgressViewport{Tier: progressTier(rows)}
	focusIndex := focusPhaseIndex(phases, completed)
	if focusIndex >= 0 {
		focus := phases[focusIndex]
		viewport.Focus = &focus
	}

	var history []progress.PhaseState
	for i, phase := range phases {
		if i == focusIndex {
			continue
		}
		if phaseSettledOrTerminal(phase) || (completed && phase.Total == 0) {
			history = append(history, phase)
		} else {
			viewport.Queued++
		}
	}

	liveRows := liveRegionRows(rows, viewport.Tier)
	historyLimit := 0
	switch viewport.Tier {
	case progressViewportNormal:
		historyLimit = min(3, max(liveRows-3, 0))
	case progressViewportCompact:
		historyLimit = min(1, max(liveRows-3, 0))
	}
	if len(history) > historyLimit {
		viewport.HistoryOmitted = len(history) - historyLimit
		history = history[len(history)-historyLimit:]
	}
	viewport.History = history
	used := len(viewport.History)
	if viewport.HistoryOmitted > 0 {
		used++
	}
	if viewport.Queued > 0 {
		used++
	}
	viewport.DetailRows = max(liveRows-used, 0)
	return viewport
}

func progressTier(rows int) progressViewportTier {
	switch {
	case rows >= 18:
		return progressViewportNormal
	case rows >= 12:
		return progressViewportCompact
	case rows >= 4:
		return progressViewportMinimal
	default:
		return progressViewportEmergency
	}
}

func liveRegionRows(rows int, tier progressViewportTier) int {
	switch tier {
	case progressViewportNormal:
		return max(rows-9, 0)
	case progressViewportCompact:
		return max(rows-4, 0)
	case progressViewportMinimal:
		return max(rows-3, 0)
	default:
		return 0
	}
}

func focusPhaseIndex(phases []progress.PhaseState, completed bool) int {
	for i, phase := range phases {
		for _, step := range phase.Steps {
			if step.Status == progress.StepRunning {
				return i
			}
		}
	}
	for i := len(phases) - 1; i >= 0; i-- {
		if phases[i].Failed > 0 || phaseHasStatus(phases[i], progress.StepFailed) {
			return i
		}
	}
	if !completed {
		for i, phase := range phases {
			if !phaseSettledOrTerminal(phase) {
				return i
			}
		}
		return -1
	}
	if len(phases) > 0 {
		return len(phases) - 1
	}
	return -1
}

func phaseHasStatus(phase progress.PhaseState, status progress.StepStatus) bool {
	for _, step := range phase.Steps {
		if step.Status == status {
			return true
		}
	}
	return false
}

func phaseSettledOrTerminal(phase progress.PhaseState) bool {
	return phase.Settled() || (phase.Total > 0 && phase.Done >= phase.Total)
}
