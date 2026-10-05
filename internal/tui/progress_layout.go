package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	progressbar "charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

// ProgressLayout renders the standard progress view shared by every
// long-running flow: title, optional subtitle, separator, phases with
// indented steps (windowed to the terminal height), separator, overall
// progress bar, and key hints. Phase data is the progress.PhaseState shape that
// buildPhaseDisplays produces from pipeline events; views with bespoke
// event types construct the same shape themselves.
type ProgressLayout struct {
	Title    string
	Subtitle string
	Phases   []progress.PhaseState
	Width    int
	Height   int
	Hints    []key.Binding

	// Bar, Elapsed, and ActiveGlyph are injected by the model's render path
	// (animated spring bar, stopwatch readout, spinner frame). Left empty,
	// View falls back to the static bar, no elapsed line, and the static
	// active-indicator fallback: direct constructions stay pure.
	Bar         string
	Elapsed     string
	ActiveGlyph string

	// Motion carries the live shimmer closures and star frame; nil falls
	// back to static styles so direct constructions stay pure.
	Motion *Motion

	// Completed marks a flow whose result has arrived: phases that never
	// discovered work render as skipped and stop counting as outstanding.
	Completed bool
}

func (m Model) withProgressDetails(layout ProgressLayout) ProgressLayout {
	if progressNeedsDetails(layout, m.progressTopLevelError()) {
		layout.Hints = append(append([]key.Binding(nil), layout.Hints...), detailsHint)
	}
	return layout
}

// activeGlyph returns the styled active indicator: the injected animation
// frame, or the static fallback for pure constructions.
func (l ProgressLayout) activeGlyph() string {
	if l.ActiveGlyph != "" {
		return l.ActiveGlyph
	}
	return styleIndicatorActive.Render(indicatorActiveFallback)
}

// overall returns the bar's fill source: checkpoints reached over declared
// across all phases (headers keep counting steps), falling back to step
// counts for phases without checkpoint declarations, and counting each
// undiscovered phase as outstanding work so the bar never reads 100%
// beside pending phases.
func (l ProgressLayout) overall() (int, int) {
	done, total := 0, 0
	for _, p := range l.Phases {
		reached, declared := progress.CheckpointProgress([]progress.PhaseState{p})
		if declared == 0 {
			reached, declared = p.Done, p.Total
		}
		done += reached
		total += declared
		if p.Total == 0 && !l.Completed {
			total++
		}
	}
	return done, total
}

func (l ProgressLayout) View() string {
	height := max(l.Height, 0)
	if height == 0 {
		return ""
	}
	width := max(l.Width, 1)
	viewport := BuildProgressViewport(l.Phases, height, l.Completed)
	bar := fitProgressLine(l.barLine(), width)
	footer := fitProgressLine(viewFooter(width, l.Hints), width)
	title := fitProgressLine(viewTitle(l.Title), width)

	if viewport.Tier == progressViewportEmergency {
		switch height {
		case 1:
			return bar
		case 2:
			return strings.Join([]string{bar, footer}, "\n")
		default:
			return strings.Join([]string{title, bar, footer}, "\n")
		}
	}

	live := l.liveRegionLines(viewport, width)
	lines := make([]string, 0, height)
	switch viewport.Tier {
	case progressViewportNormal:
		lines = append(lines, title)
		if l.Subtitle != "" {
			lines = append(lines, fitProgressLine(styleAccent.Render("  "+l.Subtitle), width))
		} else {
			lines = append(lines, "")
		}
		lines = append(lines, fitProgressLine(viewSeparator(width), width), "")
		liveRows := viewport.DetailRows + len(viewport.History)
		if viewport.HistoryOmitted > 0 {
			liveRows++
		}
		if viewport.Queued > 0 {
			liveRows++
		}
		lines = append(lines, padProgressLines(live, liveRows)...)
		lines = append(lines, fitProgressLine(viewSeparator(width), width), "", bar, "", footer)
	case progressViewportCompact:
		lines = append(lines, title, fitProgressLine(viewSeparator(width), width))
		lines = append(lines, padProgressLines(live, liveRegionRows(height, viewport.Tier))...)
		lines = append(lines, bar, footer)
	case progressViewportMinimal:
		lines = append(lines, title)
		lines = append(lines, padProgressLines(live, liveRegionRows(height, viewport.Tier))...)
		lines = append(lines, bar, footer)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (l ProgressLayout) liveRegionLines(viewport ProgressViewport, width int) []string {
	var lines []string
	if viewport.HistoryOmitted > 0 {
		lines = append(lines, fitProgressLine(styleDim.Render(fmt.Sprintf("  … %d earlier phases", viewport.HistoryOmitted)), width))
	}
	for _, phase := range viewport.History {
		lines = append(lines, fitProgressLine(l.phaseHeadline(phase), width))
	}
	if viewport.Focus != nil && viewport.DetailRows > 0 {
		lines = append(lines, l.focusLines(*viewport.Focus, viewport.DetailRows, width)...)
	}
	if viewport.Queued > 0 {
		lines = append(lines, fitProgressLine(styleDim.Render(fmt.Sprintf("  · %d %s waiting", viewport.Queued, pluralize(viewport.Queued, "phase", "phases"))), width))
	}
	return lines
}

func (l ProgressLayout) focusLines(phase progress.PhaseState, rows, width int) []string {
	if rows <= 0 {
		return nil
	}
	lines := []string{fitProgressLine(l.phaseHeadline(phase), width)}
	window := WindowSteps(phase.Steps, rows-1)
	for _, step := range window.Steps {
		lines = append(lines, fitProgressLine(l.stepLine(step), width))
	}
	if window.Windowed && len(lines) < rows {
		lines = append(lines, fitProgressLine(viewStatLine(window.Stats, l.activeGlyph()), width))
	}
	return lines[:min(len(lines), rows)]
}

func (l ProgressLayout) phaseHeadline(phase progress.PhaseState) string {
	if phase.Total == 0 {
		if l.Completed {
			return fmt.Sprintf("  %s %s  %s", styleDim.Render("–"), stylePhasePending.Render(phase.Name), styleDim.Render("skipped"))
		}
		return fmt.Sprintf("  %s %s  %s", styleIndicatorPending.Render(indicatorPending), stylePhasePending.Render(phase.Name), styleDim.Render("pending"))
	}
	if phase.Settled() && phase.Failed == 0 {
		return fmt.Sprintf("  %s %s  %s", styleIndicatorDone.Render(indicatorDone), stylePhaseDone.Render(phase.Name), styleDim.Render(fmt.Sprintf("%d/%d  100%%", phase.Total, phase.Total)))
	}
	counts := fmt.Sprintf("%d/%d  %d%%", min(phase.Done, phase.Total), phase.Total, min((phase.Done*100)/phase.Total, 100))
	if phase.Failed > 0 {
		counts = fmt.Sprintf("%d/%d", min(phase.Done, phase.Total), phase.Total)
		return fmt.Sprintf("  %s %s  %s", styleIndicatorFailed.Render(indicatorFailed), stylePhaseActive.Render(phase.Name), styleDim.Render(counts))
	}
	if l.Motion != nil && phaseHasStatus(phase, progress.StepRunning) {
		return fmt.Sprintf("  %s  %s", l.Motion.Accent(l.Motion.Frame+" "+phase.Name), styleDim.Render(counts))
	}
	return fmt.Sprintf("  %s %s  %s", l.activeGlyph(), stylePhaseActive.Render(phase.Name), styleDim.Render(counts))
}

func (l ProgressLayout) stepLine(step progress.StepState) string {
	label := step.Name
	switch step.Status {
	case progress.StepRunning:
		return "    " + l.activeGlyph() + " " + label
	case progress.StepDone:
		return "    " + styleIndicatorDone.Render(indicatorDone) + " " + label
	case progress.StepFailed:
		if step.Error != nil {
			label += ": " + step.Error.Error()
		}
		return "    " + styleIndicatorFailed.Render(indicatorFailed) + " " + label
	case progress.StepSkipped:
		reason := ""
		if step.Message != "" {
			reason = " (" + step.Message + ")"
		}
		return styleDim.Render("    – " + label + " – skipped" + reason)
	default:
		return "    " + styleIndicatorPending.Render(indicatorPending) + " " + label
	}
}

func (l ProgressLayout) barLine() string {
	if l.Bar != "" {
		withElapsed := l.Bar
		if l.Elapsed != "" {
			withElapsed += "  " + l.Elapsed
		}
		if lipgloss.Width(withElapsed) <= l.Width {
			return withElapsed
		}
		if lipgloss.Width(l.Bar) <= l.Width {
			return l.Bar
		}
	}
	done, total := l.overall()
	pct := 0
	if total > 0 {
		pct = min((done*100)/total, 100)
	} else if l.Completed {
		pct = 100
	}
	if l.Width < minProgressBarWidth+progressBarPercentReserve+2 {
		return fmt.Sprintf("  %d%%", pct)
	}
	return renderProgressBar(done, total, l.Width-2-progressBarPercentReserve)
}

func fitProgressLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	return ansi.Truncate(line, width, "…")
}

func padProgressLines(lines []string, rows int) []string {
	if len(lines) > rows {
		return lines[:rows]
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	return lines
}

// newOverallBar constructs the spring-animated overall bar: block fill
// characters, gradient scaled to the filled portion, sized responsively
// from WindowSizeMsg (the construction width is only the pre-frame floor).
func newOverallBar() progressbar.Model {
	return progressbar.New(
		progressbar.WithWidth(minProgressBarWidth),
		progressbar.WithFillCharacters('█', '░'),
		progressbar.WithColors(colorBarStart, colorBarEnd),
		progressbar.WithScaled(true),
		// Luxurious spring: a full sweep takes ~1.2s (bar visually full at
		// ~1.0s), inside the hold plus settle floor. Critically damped:
		// the component clamps at 100%, so any overshoot would render as
		// the bar retreating.
		progressbar.WithSpringOptions(6, 1),
	)
}

// determinateProgressActive reports whether a ProgressLayout-rendered flow
// is on screen: the only place bar frames and stopwatch ticks may animate.
func (m Model) determinateProgressActive() bool {
	switch m.view {
	case progressView, createProgressView, repoProgressView, migrateProgressView, integrationProgressView:
		return true
	}
	return false
}

// activeProgressLayout returns the layout for the flow on screen, the single
// source for done/total shared by rendering and the spring target.
func (m Model) activeProgressLayout() (ProgressLayout, bool) {
	switch m.view {
	case progressView:
		return m.removalLayout(), true
	case createProgressView:
		return m.createLayout(), true
	case repoProgressView, migrateProgressView:
		return m.repoLayout(), true
	case integrationProgressView:
		return m.integrationLayout(), true
	}
	return ProgressLayout{}, false
}

// syncProgressBar springs the bar toward the active flow's completion and
// starts the elapsed stopwatch on the flow's first event.
func (m *Model) syncProgressBar() tea.Cmd {
	l, ok := m.activeProgressLayout()
	if !ok {
		return nil
	}
	done, total := l.overall()
	pct := 0.0
	switch {
	case total > 0:
		pct = min(float64(done)/float64(total), 1)
	case l.Completed:
		// Nothing was ever discovered and the flow is done: that is 100%.
		pct = 1
	}
	m.progressTarget = pct
	var cmds []tea.Cmd
	if m.motionPreference == MotionFull {
		cmds = append(cmds, m.bar.SetPercent(pct))
	}
	if !m.watch.Running() {
		cmds = append(cmds, m.watch.Start())
	}
	return tea.Batch(cmds...)
}

// renderProgressLayout injects the animated bar and elapsed readout into a
// flow's layout. The percentage text states actual progress; only the cells
// ease toward it. Bar colors come from the live palette tokens.
func (m Model) renderProgressLayout(l ProgressLayout) string {
	bar := m.bar
	// Colors re-read the live palette tokens each render: the adaptive
	// palette can arrive after the bar is constructed. A completed flow
	// settles the fill green for its hold — the bar joins the ✦ moment —
	// but a flow that finished with failures keeps the standard gradient:
	// the truth-hold applies to every ending, the celebration does not.
	hasFailures := m.progressTopLevelError() != nil
	for _, p := range l.Phases {
		if p.Failed > 0 {
			hasFailures = true
			break
		}
	}
	if l.Completed && !hasFailures {
		progressbar.WithColors(colorBarDoneStart, colorBarDoneEnd)(&bar)
	} else {
		progressbar.WithColors(colorBarStart, colorBarEnd)(&bar)
	}
	bar.EmptyColor = colorDim
	// The native percentage follows the displayed fill, so bar and label
	// never disagree; the phase headers state actual counts.
	if m.motionPreference == MotionOff {
		l.Bar = "  " + bar.ViewAs(m.progressTarget)
	} else {
		l.Bar = "  " + bar.View()
	}
	// Sub-2s elapsed is noise that implies precision the 1Hz ticker does
	// not have; the layout reserve keeps the bar from reflowing when the
	// readout appears.
	if elapsed := time.Since(m.progressStartedAt); elapsed >= 2*time.Second {
		l.Elapsed = styleDim.Render(fmt.Sprintf("elapsed %ds", int(elapsed.Seconds())))
	}
	if m.motionPreference == MotionFull {
		l.Motion = m.motion()
	}
	return l.View()
}
