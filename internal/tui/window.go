package tui

import "github.com/bonboncinnabon/sentei/internal/progress"

// WindowStats summarizes a windowed step list for the stat line.
type WindowStats struct {
	Done    int
	Active  int
	Pending int
	Failed  int
	Showing int
	Total   int
}

// WindowResult is the outcome of windowing a step list.
type WindowResult struct {
	Steps    []progress.StepState
	Windowed bool
	Stats    WindowStats
}

// WindowSteps selects which steps to display when the list may exceed the
// available terminal lines. Priority: failed and active steps are always
// visible; remaining room shows the most recently completed steps (up to
// WindowCompletedTrail) and the next pending steps (up to WindowPendingLead).
// One line is reserved for the stat line when windowing engages.
func WindowSteps(steps []progress.StepState, availableLines int) WindowResult {
	stats := WindowStats{Total: len(steps)}
	for _, s := range steps {
		switch s.Status {
		case progress.StepDone, progress.StepSkipped:
			stats.Done++
		case progress.StepRunning:
			stats.Active++
		case progress.StepFailed:
			stats.Failed++
		default:
			stats.Pending++
		}
	}

	availableLines = max(availableLines, 0)
	if len(steps) <= availableLines {
		stats.Showing = len(steps)
		return WindowResult{Steps: steps, Windowed: false, Stats: stats}
	}

	if availableLines == 0 {
		return WindowResult{Windowed: len(steps) > 0, Stats: stats}
	}

	budget := availableLines - 1 // reserve the omission stat
	priority := make([]int, 0, len(steps))
	for i, step := range steps {
		if step.Status == progress.StepRunning {
			priority = append(priority, i)
		}
	}
	latestFailure := -1
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Status == progress.StepFailed {
			latestFailure = i
			priority = append(priority, i)
			break
		}
	}
	for i, step := range steps {
		if step.Status == progress.StepFailed && i != latestFailure {
			priority = append(priority, i)
		}
	}
	resolved := 0
	for i := len(steps) - 1; i >= 0 && resolved < WindowCompletedTrail; i-- {
		if steps[i].Status == progress.StepDone || steps[i].Status == progress.StepSkipped {
			priority = append(priority, i)
			resolved++
		}
	}
	pending := 0
	for i, step := range steps {
		if step.Status == progress.StepPending && pending < WindowPendingLead {
			priority = append(priority, i)
			pending++
		}
	}

	visible := make([]bool, len(steps))
	for _, index := range priority {
		if budget == 0 {
			break
		}
		if !visible[index] {
			visible[index] = true
			budget--
		}
	}
	var windowed []progress.StepState
	for i, s := range steps {
		if visible[i] {
			windowed = append(windowed, s)
		}
	}
	stats.Showing = len(windowed)
	return WindowResult{Steps: windowed, Windowed: true, Stats: stats}
}
