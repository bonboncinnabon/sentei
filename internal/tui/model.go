package tui

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	progressbar "charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/stopwatch"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// progressSettleProbeMsg is the completion settle's hard-timeout wake-up:
// settle progress is normally observed on motion ticks and spring frames,
// and this probe guarantees the check runs even if both go quiet.
// The token must match model.progressToken to guard against stale messages.
type progressSettleProbeMsg struct{ token int }

type progressTransitionMsg struct{ token int }

type viewState int

const (
	menuView viewState = iota
	listView
	confirmView
	progressView
	summaryView
	createBranchView
	createOptionsView
	createProgressView
	createSummaryView
	repoNameView
	repoOptionsView
	repoProgressView
	repoSummaryView
	cloneInputView
	migrateConfirmView
	migrateProgressView
	migrateSummaryView
	migrateNextView
	cleanupConfirmView
	cleanupPreviewView
	cleanupResultView
	createConfirmView
	cloneConfirmView
)

type SortField int

const (
	SortByAge SortField = iota
	SortByBranch
)

// RemovePreSelection holds pre-selected worktree paths and a label describing
// the filter that produced them. This avoids importing the cmd package.
type RemovePreSelection struct {
	Paths       []string
	FilterLabel string
	CLICommand  string // exact equivalent command, echoed on the summary
}

// MigrateOpts holds migrate options passed to the TUI from the CLI layer.
// This mirrors cmd.MigrateOptions without creating a dependency on cmd.
type MigrateOpts struct {
	DeleteBackup bool
	RepoPath     string
}

// CreateOpts holds create options passed to the TUI from the CLI layer.
// This mirrors cmd.CreateOptions without creating a dependency on cmd.
type CreateOpts struct {
	Branch     string
	Base       string
	Ecosystems []string
	MergeBase  bool
	CopyEnv    bool
	RepoPath   string
}

// CloneOpts holds clone options passed to the TUI from the CLI layer.
// This mirrors cmd.CloneOptions without creating a dependency on cmd.
type CloneOpts struct {
	URL  string
	Name string
}

// removeState holds all state for the worktree removal flow.
type removeState struct {
	worktrees     []git.Worktree
	defaultBranch string // repo default branch; always protected from removal
	// launchedFrom holds the paths of the worktrees containing the directory
	// sentei was started in; they are protected from removal like a default branch.
	launchedFrom map[string]bool
	selected     map[string]bool
	// preSelect holds the paths a `sentei remove` filter chose. They are
	// selected when the worktree load lands, then the set is dropped.
	preSelect map[string]bool
	// loadErr is why the last worktree load failed, nil once one succeeds.
	loadErr        error
	visibleIndices []int
	cursor         int
	offset         int

	sortField     SortField
	sortAscending bool

	filterText   string
	filterActive bool
	filterInput  textinput.Model
	filterLabel  string // describes filter that produced pre-selection (e.g. "merged", "stale > 30d")
	cliCommand   string // CLI equivalent of the pre-selection, echoed on the summary

	run removalRun
}

// menuItem represents a selectable menu entry. loading marks entries whose
// hint reflects an in-flight load and renders with the spinner.
type menuItem struct {
	label   string
	hint    string
	enabled bool
	loading bool
}

// createState holds all state for the worktree creation flow.
type createState struct {
	branchInput   textinput.Model
	baseInput     textinput.Model
	focusedField  int // 0 = branch, 1 = base
	validationErr string

	ecosystems    []config.EcosystemConfig
	ecoEnabled    map[string]bool
	mergeBase     bool
	copyEnvFiles  bool
	optionsCursor int

	eventCh  chan progress.Event
	resultCh chan creator.Result
	events   []progress.Event
	result   *creator.Result
}

// MigrateInfo holds pre-loaded info about the repo being migrated.
type MigrateInfo struct {
	Branch  string
	IsDirty bool
}

// repoState holds all state for repo create/clone/migrate flows.
type repoState struct {
	// Create repo fields
	nameInput     textinput.Model
	locationInput textinput.Model
	focusedField  int // 0 = name, 1 = location
	validationErr string

	// Options
	createWorktree bool
	publishGitHub  bool
	visibility     string // "private" or "public"
	descInput      textinput.Model
	ghStatus       string // "authenticated", "not authenticated", "gh not found"
	optionsCursor  int

	// Clone fields
	urlInput           textinput.Model
	cloneNameInput     textinput.Model
	cloneFocusedField  int // 0 = url, 1 = name
	nameManuallyEdited bool

	// Migrate fields
	migrateInfo MigrateInfo

	// Shared progress/summary
	eventCh  chan progress.Event
	resultCh chan interface{} // receives CreateResult, CloneResult, or MigrateResult
	events   []progress.Event
	result   interface{}
	opType   string // "create", "clone", "migrate"
}

type Model struct {
	view     viewState
	runner   git.CommandRunner
	shell    git.ShellRunner
	repoPath string
	cfg      *config.Config
	context  repo.RepoContext
	width    int
	height   int
	// windowHeight is the raw terminal height. height remains the shared
	// chrome-budgeted body height used by non-progress views.
	windowHeight int

	menuItems          []menuItem
	menuCursor         int
	worktreeGeneration uint64 // Monotonic token passed to loadWorktreeContext; global handler discards mismatched responses.
	loadedGeneration   uint64 // Generation of the last load that landed; behind worktreeGeneration while one is in flight.

	cleanupOpts   *cleanup.Options
	cleanupResult *cleanup.Result // standalone cleanup flow ("Cleanup & exit" / sentei cleanup)

	// Cleanup preview (TUI menu path): dry-run scan results. Pending holds a
	// finished scan until the minimum scanning display has elapsed.
	cleanupScan              *cleanup.DryRunResult
	cleanupScanPending       *cleanup.DryRunResult
	cleanupScanErr           error
	cleanupAggressiveConfirm bool
	cleanupRanMode           cleanup.Mode // echoed as the CLI equivalent on the result view
	createOpts               *CreateOpts
	cloneOpts                *CloneOpts
	migrateOpts              *MigrateOpts

	remove removeState
	create createState
	repo   repoState
	portal DetailPortal

	// quitPrompt is set by the first quit key during a running operation:
	// the footer asks for a second one. Any key answers and clears it.
	quitPrompt bool

	// motionTick is the one animation clock: star frames and shimmer band
	// positions derive from it as pure functions. The tick chain runs only
	// while a working surface is visible (motionActive).
	motionTick       int
	motionPreference MotionPreference

	// bar springs the overall progress toward each completion target and
	// watch counts elapsed time; both animate only in determinate progress
	// views and reset between flows in holdOrAdvance.
	bar   progressbar.Model
	watch stopwatch.Model

	// Progress hold state — used to enforce minimum visible duration for progress views.
	minProgressDuration       time.Duration // 0 = no entry hold; set via WithMinProgressDuration
	progressStartedAt         time.Time     // set when entering any progress view
	progressToken             int           // bumped on each entry; guards stale timers
	progressTargetView        viewState     // where to transition once settled
	progressTransitionPending bool
	progressTarget            float64 // exact logical progress, separate from the spring's displayed fill

	// Completion settle state: after a flow's final event the view holds
	// until the displayed bar fill reaches its target and stays there for
	// progressSettleBeat, bounded by progressSettleTimeout. Checked on the
	// motion clock and spring frames; applies in every run mode.
	progressSettling      bool
	progressSettlingSince time.Time // when the final event arrived
	progressSettledAt     time.Time // when the displayed fill first arrived; zero while gliding
}

// ModelOption configures a Model at construction time.
type ModelOption func(*Model)

// WithMinProgressDuration sets the minimum time any progress view stays
// visible before the model auto-advances to the summary/result view.
// Default is 0 (advance immediately). Set to ~1.5s in playground mode.
func WithMinProgressDuration(d time.Duration) ModelOption {
	return func(m *Model) {
		m.minProgressDuration = d
	}
}

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

func NewMenuModel(runner git.CommandRunner, shell git.ShellRunner, repoPath string, cfg *config.Config, context repo.RepoContext, opts ...ModelOption) Model {
	branchInput := textinput.New()
	branchInput.Placeholder = "feature/my-branch"
	branchInput.SetWidth(formInputWidth)
	branchInput.Focus()

	baseInput := textinput.New()
	baseInput.Placeholder = "main"
	baseInput.SetWidth(formInputWidth)
	baseInput.SetValue(defaultBaseBranch)

	filterInput := textinput.New()
	filterInput.Prompt = "filter: "

	nameInput := textinput.New()
	nameInput.Placeholder = "my-project"
	nameInput.SetWidth(formInputWidth)

	locationInput := textinput.New()
	locationInput.SetValue(repoPath)
	locationInput.Placeholder = repoPath
	locationInput.SetWidth(formInputWidth)

	descInput := textinput.New()
	descInput.Placeholder = "optional description"
	descInput.SetWidth(formInputWidth)

	urlInput := textinput.New()
	urlInput.Placeholder = "git@github.com:user/repo.git"
	urlInput.SetWidth(formInputWidth)

	cloneNameInput := textinput.New()
	cloneNameInput.Placeholder = "repo"
	cloneNameInput.SetWidth(formInputWidth)

	var items []menuItem
	switch context {
	case repo.ContextBareRepo:
		items = []menuItem{
			{label: "Create new worktree", enabled: true},
			{label: menuRemoveWorktrees, hint: "loading\u2026", enabled: false, loading: true},
			{label: "Cleanup & exit", hint: "prune refs, remove gone branches", enabled: true},
		}
	case repo.ContextNoRepo:
		items = []menuItem{
			{label: "Create new repository", enabled: true},
			{label: "Clone repository as bare", enabled: true},
		}
	case repo.ContextNonBareRepo:
		items = []menuItem{
			{label: "Migrate to bare repository", enabled: true},
			{label: "Clone repository as bare", enabled: true},
			{label: "Create new repository", enabled: true},
		}
	}

	var initGeneration uint64
	if context == repo.ContextBareRepo {
		initGeneration = 1
	}

	m := Model{
		view:               menuView,
		runner:             runner,
		shell:              shell,
		repoPath:           repoPath,
		cfg:                cfg,
		context:            context,
		width:              80,
		height:             20,
		menuItems:          items,
		worktreeGeneration: initGeneration,
		bar:                newOverallBar(),
		watch:              newElapsedTicker(),
		motionPreference:   motionPreference(os.Getenv),
		remove: removeState{
			selected:      make(map[string]bool),
			sortField:     SortByAge,
			sortAscending: true,
			filterInput:   filterInput,
		},
		create: createState{
			branchInput:  branchInput,
			baseInput:    baseInput,
			ecoEnabled:   make(map[string]bool),
			mergeBase:    true,
			copyEnvFiles: true,
		},
		repo: repoState{
			nameInput:      nameInput,
			locationInput:  locationInput,
			descInput:      descInput,
			urlInput:       urlInput,
			cloneNameInput: cloneNameInput,
			visibility:     "private",
		},
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// holdOrAdvance begins the completion settle: the view may not transition to
// targetView until the displayed bar fill has reached its target and stayed
// there for progressSettleBeat (plus, in playground mode, until the entry
// hold has elapsed), hard-bounded by progressSettleTimeout. The caller must
// store all result state into the model before calling. Settle progress is
// observed on the motion clock and spring frames; a timeout tick guarantees
// a wake-up even if both go quiet.
func (m Model) holdOrAdvance(targetView viewState) (tea.Model, tea.Cmd) {
	now := time.Now()
	m.progressTargetView = targetView
	m.progressSettling = true
	m.progressSettlingSince = now
	m.progressSettledAt = time.Time{}
	token := m.progressToken
	if m.motionPreference == MotionOff {
		if settled, advanced := m.observeSettle(now); advanced {
			return settled, settledProgressTransitionCmd(token)
		} else {
			m = settled
		}
		remaining := m.minProgressDuration - now.Sub(m.progressStartedAt)
		return m, tea.Tick(max(remaining, 0), func(time.Time) tea.Msg {
			return progressSettleProbeMsg{token: token}
		})
	}
	return m, tea.Tick(progressSettleTimeout, func(time.Time) tea.Msg {
		return progressSettleProbeMsg{token: token}
	})
}

// settleAdvanceReady reports whether the settled view may advance: the entry
// hold (if any) has elapsed AND the displayed fill has been settled for the
// beat, or the hard timeout has expired.
func (m Model) settleAdvanceReady(now time.Time) bool {
	holdDone := m.minProgressDuration == 0 || now.Sub(m.progressStartedAt) >= m.minProgressDuration
	if m.motionPreference == MotionOff {
		return holdDone
	}
	beatDone := !m.progressSettledAt.IsZero() && now.Sub(m.progressSettledAt) >= progressSettleBeat
	timedOut := now.Sub(m.progressSettlingSince) >= progressSettleTimeout
	return (holdDone && beatDone) || timedOut
}

// observeSettle records settle state transitions and advances the view once
// settleAdvanceReady holds. Called from the motion clock, spring frames, and
// the timeout probe so settling can never stall for lack of a wake-up.
func (m Model) observeSettle(now time.Time) (Model, bool) {
	if !m.progressSettling {
		return m, false
	}
	if m.motionPreference == MotionFull && m.bar.IsAnimating() {
		// Still gliding (or a new spring target arrived): not settled.
		m.progressSettledAt = time.Time{}
	} else if m.progressSettledAt.IsZero() {
		m.progressSettledAt = now
	}
	if !m.settleAdvanceReady(now) {
		return m, false
	}
	m.progressSettling = false
	m.progressTransitionPending = true
	return m, true
}

func settledProgressTransitionCmd(token int) tea.Cmd {
	return tea.Sequence(tea.ClearScreen, func() tea.Msg {
		return progressTransitionMsg{token: token}
	})
}

func (m Model) completeProgressTransition() Model {
	leavingProgress := m.determinateProgressActive()
	m.progressTransitionPending = false
	m.view = m.progressTargetView
	if leavingProgress && m.portal.trigger == portalDetails {
		m.portal = m.portal.Close()
	}
	// Leaving the progress view: discard the spring state so the next
	// flow starts from zero, not easing down from 100%.
	m.bar = newOverallBar()
	m.bar.SetWidth(overallBarWidth(m.width))
	return m
}

// SetCleanupOpts sets the cleanup options and starts at the cleanup confirmation view.
func (m *Model) SetCleanupOpts(opts *cleanup.Options) {
	m.cleanupOpts = opts
	m.view = cleanupConfirmView
}

// SetRemoveOpts pre-selects worktrees matching the given paths and enters the
// list view. The filterLabel is displayed in the status bar to indicate what
// filter produced the selection (e.g. "merged", "stale > 30d").
func (m *Model) SetRemoveOpts(preSelection RemovePreSelection) {
	m.remove.preSelect = make(map[string]bool, len(preSelection.Paths))
	for _, p := range preSelection.Paths {
		m.remove.preSelect[p] = true
	}
	m.applyPreSelection()
	m.remove.filterLabel = preSelection.FilterLabel
	m.remove.cliCommand = preSelection.CLICommand
	m.view = listView
}

// applyPreSelection selects the pre-selected worktrees that are loaded and
// may be removed. Protection is decided here as everywhere else, whatever the
// caller's filter let through.
func (m *Model) applyPreSelection() {
	for _, wt := range m.remove.worktrees {
		if m.remove.preSelect[wt.Path] && !m.removalProtected(wt) {
			m.remove.selected[wt.Path] = true
		}
	}
}

// dropStaleSelections deselects every path that is no longer listed or may
// no longer be removed, so the selection never counts rows that are not there.
func (m *Model) dropStaleSelections() {
	selectable := make(map[string]bool, len(m.remove.worktrees))
	for _, wt := range m.remove.worktrees {
		selectable[wt.Path] = !m.removalProtected(wt)
	}
	maps.DeleteFunc(m.remove.selected, func(path string, _ bool) bool { return !selectable[path] })
}

// worktreesLoading reports whether a worktree-context load is in flight.
func (m Model) worktreesLoading() bool {
	return m.loadedGeneration != m.worktreeGeneration
}

// SetMigrateOpts sets the migrate options and starts at the migrate confirmation view.
func (m *Model) SetMigrateOpts(opts *MigrateOpts) {
	m.migrateOpts = opts
	if opts.RepoPath != "" {
		m.repoPath = opts.RepoPath
	}
	m.view = migrateConfirmView
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor}
	if m.motionActive() {
		cmds = append(cmds, motionTickCmd())
	}
	// Every bare-repo entry point needs the worktrees, not only the menu: a
	// direct `sentei remove` lists them, and any direct flow can end up on
	// the menu.
	if m.context == repo.ContextBareRepo {
		cmds = append(cmds, loadWorktreeContext(m.runner, m.repoPath, m.worktreeGeneration))
	}
	return tea.Batch(cmds...)
}

// indeterminateWaitActive reports whether a spinner-bearing wait is visible:
// the cleanup scan, the menu worktree-context load, or the list before its
// first rows arrive.
func (m Model) indeterminateWaitActive() bool {
	if m.view == cleanupPreviewView && m.cleanupScan == nil {
		return true
	}
	if m.view == listView && m.listAwaitingRows() {
		return true
	}
	if m.view == menuView {
		for _, item := range m.menuItems {
			if item.loading {
				return true
			}
		}
	}
	return false
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if transition, ok := msg.(progressTransitionMsg); ok {
		if transition.token == m.progressToken && m.progressTransitionPending {
			m = m.completeProgressTransition()
		}
		return m, nil
	}

	if probeMsg, ok := msg.(progressSettleProbeMsg); ok {
		if probeMsg.token == m.progressToken {
			var advanced bool
			m, advanced = m.observeSettle(time.Now())
			if advanced {
				return m, settledProgressTransitionCmd(m.progressToken)
			}
		}
		return m, nil
	}

	if ctx, ok := msg.(worktreeContextMsg); ok {
		if ctx.generation != m.worktreeGeneration {
			return m, nil
		}
		m.loadedGeneration = ctx.generation
		m.remove.loadErr = ctx.err
		if ctx.err == nil {
			m.remove.worktrees = ctx.worktrees
			m.remove.defaultBranch = ctx.defaultBranch
			m.remove.launchedFrom = ctx.launchedFrom
			m.applyPreSelection()
			m.remove.preSelect = nil
			m.dropStaleSelections()
			m.reindex()
		}
		m.updateMenuHints()
		return m, nil
	}

	if size, ok := msg.(tea.WindowSizeMsg); ok {
		// Sizing is global: the portal tracks the raw terminal size, views
		// share the chrome-budgeted body height. No view sizes itself.
		m.portal = m.portal.SetSize(size.Width, size.Height)
		m.width = size.Width
		m.windowHeight = max(size.Height, 0)
		m.height = max(size.Height-viewChromeRows, 5)
		m.clampListWindow()
		m.bar.SetWidth(overallBarWidth(size.Width))
		return m, nil
	}

	if frame, ok := msg.(progressbar.FrameMsg); ok {
		if m.motionPreference == MotionOff || !m.determinateProgressActive() {
			return m, nil
		}
		var cmd tea.Cmd
		m.bar, cmd = m.bar.Update(frame)
		var advanced bool
		m, advanced = m.observeSettle(time.Now())
		if advanced {
			return m, tea.Batch(cmd, settledProgressTransitionCmd(m.progressToken))
		}
		return m, cmd
	}

	if tick, ok := msg.(stopwatch.TickMsg); ok {
		if !m.determinateProgressActive() {
			// End the tick cycle and clear the running flag so the next
			// flow restarts the ticker.
			return m, m.watch.Stop()
		}
		var cmd tea.Cmd
		m.watch, cmd = m.watch.Update(tick)
		return m, cmd
	}

	if ss, ok := msg.(stopwatch.StartStopMsg); ok {
		var cmd tea.Cmd
		m.watch, cmd = m.watch.Update(ss)
		return m, cmd
	}

	if _, ok := msg.(motionTickMsg); ok {
		if !m.motionActive() {
			return m, nil
		}
		previousView := m.view
		m.motionTick++
		var advanced bool
		m, advanced = m.observeSettle(time.Now())
		if advanced {
			return m, settledProgressTransitionCmd(m.progressToken)
		}
		if m.view != previousView || !m.motionActive() {
			return m, nil
		}
		return m, motionTickCmd()
	}

	if bg, ok := msg.(tea.BackgroundColorMsg); ok {
		if !bg.IsDark() {
			applyPalette(lightPalette)
		}
		return m, nil
	}

	// Quitting is decided here, before the portal and the views see the key.
	// Bubble Tea delivers ctrl+c as an ordinary key press, so it quits only
	// where something handles it: this covers every view, the portal and
	// every focused text input. While an operation runs, a quit would stop
	// it where it stands, so the first quit key only asks.
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		inFlight := m.operationInFlight()
		prompted := inFlight && m.quitPrompt
		m.quitPrompt = false
		switch {
		case prompted:
			if key.Matches(keyMsg, keys.Quit) {
				return m, tea.Quit
			}
			return m, nil
		case inFlight && key.Matches(keyMsg, keys.Quit):
			m.quitPrompt = true
			m.portal = m.portal.Close()
			return m, nil
		case key.Matches(keyMsg, keys.Interrupt):
			return m, tea.Quit
		}
	}

	if m.portal.Visible() {
		if _, ok := msg.(tea.PasteMsg); ok {
			return m, nil
		}
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.updatePortalKeys(keyMsg)
		}
		if wheel, ok := msg.(tea.MouseWheelMsg); ok {
			// The wheel scrolls the portal viewport and never the background.
			var cmd tea.Cmd
			m.portal, cmd = m.portal.Update(wheel)
			return m, cmd
		}
		// Non-key messages (progress events, timers) keep flowing to views.
	} else if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(keyMsg, keys.GlobalHelp) {
			title, content := m.helpContent()
			m.portal = m.portal.Open(portalHelp, title, content)
			return m, nil
		}
		if key.Matches(keyMsg, keys.Info) {
			if title, content := m.detailContent(); content != "" {
				m.portal = m.portal.Open(portalDetails, title, content)
				return m, nil
			}
			// No details for this view: fall through to the view's own handling.
		}
	}

	// Per-view dispatch, wrapped so any flow entering a live-work view
	// starts the motion clock without each entry site knowing about it.
	wasMoving := m.motionActive()
	updated, cmd := m.dispatchByView(msg)
	if model, ok := updated.(Model); ok {
		if model.portal.trigger == portalDetails && model.determinateProgressActive() {
			if title, content := model.detailContent(); content != "" {
				model.portal = model.portal.Refresh(title, content)
			} else {
				model.portal = model.portal.Close()
			}
		}
		if !wasMoving && model.motionActive() {
			return model, tea.Batch(cmd, motionTickCmd())
		}
		return model, cmd
	}
	return updated, cmd
}

func (m Model) progressHeight() int {
	if m.windowHeight > 0 {
		return m.windowHeight
	}
	return m.height
}

// motionActive reports whether any working surface is on screen: an
// indeterminate wait, a determinate progress view, or the cleanup result's
// running line. The one gate for the one motion clock.
func (m Model) motionActive() bool {
	if m.motionPreference == MotionOff {
		return false
	}
	return m.indeterminateWaitActive() ||
		m.determinateProgressActive() ||
		(m.view == cleanupResultView && m.cleanupResult == nil)
}

func (m Model) dispatchByView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.view {
	case menuView:
		return m.updateMenu(msg)
	case listView:
		return m.updateList(msg)
	case confirmView:
		return m.updateConfirm(msg)
	case progressView:
		return m.updateProgress(msg)
	case summaryView:
		return m.updateSummary(msg)
	case createBranchView:
		return m.updateCreateBranch(msg)
	case createOptionsView:
		return m.updateCreateOptions(msg)
	case createProgressView:
		return m.updateCreateProgress(msg)
	case createSummaryView:
		return m.updateCreateSummary(msg)
	case repoNameView:
		return m.updateRepoName(msg)
	case repoOptionsView:
		return m.updateRepoOptions(msg)
	case repoProgressView, migrateProgressView:
		return m.updateRepoProgress(msg)
	case repoSummaryView:
		return m.updateRepoSummary(msg)
	case cloneInputView:
		return m.updateCloneInput(msg)
	case migrateConfirmView:
		return m.updateMigrateConfirm(msg)
	case migrateSummaryView:
		return m.updateMigrateSummary(msg)
	case migrateNextView:
		return m.updateMigrateNext(msg)
	case cleanupConfirmView:
		return m.updateCleanupConfirm(msg)
	case cleanupPreviewView:
		return m.updateCleanupPreview(msg)
	case cleanupResultView:
		return m.updateCleanupResult(msg)
	case createConfirmView:
		return m.updateCreateConfirm(msg)
	case cloneConfirmView:
		return m.updateCloneConfirm(msg)
	}
	return m, nil
}

// View declares terminal features alongside the frame: the alternate screen
// and cell-motion mouse mode live here, the only place v2 reads them. Basic
// key disambiguation (ctrl+enter) is always requested by the v2 renderer.
func (m Model) View() tea.View {
	content := m.viewContent()
	if m.portal.Visible() {
		content = m.portal.View(content)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = m.windowTitle()
	v.ProgressBar = m.terminalProgress()
	return v
}

// operationInFlight reports whether work is running that a quit would stop
// midway: a progress flow whose result has not arrived, or a cleanup run.
func (m Model) operationInFlight() bool {
	if layout, ok := m.activeProgressLayout(); ok {
		return !layout.Completed
	}
	return m.view == cleanupResultView && m.cleanupResult == nil
}

func (m Model) viewContent() string {
	if m.view == cleanupResultView && m.quitPrompt && m.operationInFlight() {
		// The running-cleanup frame has no footer of its own to replace.
		return m.viewCleanupResult() + "\n" + fitProgressLine(viewFooterDanger(m.width, quitPromptFooter), m.width) + "\n"
	}
	switch m.view {
	case menuView:
		return m.viewMenu()
	case listView:
		return m.viewList()
	case confirmView:
		return m.viewConfirm()
	case progressView:
		return m.viewProgress()
	case summaryView:
		return m.viewSummary()
	case createBranchView:
		return m.viewCreateBranch()
	case createOptionsView:
		return m.viewCreateOptions()
	case createProgressView:
		return m.viewCreateProgress()
	case createSummaryView:
		return m.viewCreateSummary()
	case repoNameView:
		return m.viewRepoName()
	case repoOptionsView:
		return m.viewRepoOptions()
	case repoProgressView, migrateProgressView:
		return m.viewRepoProgress()
	case repoSummaryView:
		return m.viewRepoSummary()
	case cloneInputView:
		return m.viewCloneInput()
	case migrateConfirmView:
		return m.viewMigrateConfirm()
	case migrateSummaryView:
		return m.viewMigrateSummary()
	case migrateNextView:
		return m.viewMigrateNext()
	case cleanupConfirmView:
		return m.viewCleanupConfirm()
	case cleanupPreviewView:
		return m.viewCleanupPreview()
	case cleanupResultView:
		return m.viewCleanupResult()
	case createConfirmView:
		return m.viewCreateConfirm()
	case cloneConfirmView:
		return m.viewCloneConfirm()
	}
	return ""
}

func (m Model) selectedWorktrees() []git.Worktree {
	var result []git.Worktree
	for _, wt := range m.remove.worktrees {
		if m.remove.selected[wt.Path] {
			result = append(result, wt)
		}
	}
	return result
}

func (m *Model) reindex() {
	filterLower := strings.ToLower(m.remove.filterText)

	var indices []int
	for i, wt := range m.remove.worktrees {
		if filterLower != "" {
			branch := strings.ToLower(stripBranchPrefix(wt.Branch))
			if !strings.Contains(branch, filterLower) {
				continue
			}
		}
		indices = append(indices, i)
	}

	sortAsc := m.remove.sortAscending
	sortField := m.remove.sortField
	wts := m.remove.worktrees

	sort.SliceStable(indices, func(a, b int) bool {
		wa, wb := wts[indices[a]], wts[indices[b]]

		switch sortField {
		case SortByAge:
			aZero := wa.LastCommitDate.IsZero()
			bZero := wb.LastCommitDate.IsZero()
			if aZero != bZero {
				return !aZero
			}
			if aZero && bZero {
				return false
			}
			if sortAsc {
				return wa.LastCommitDate.Before(wb.LastCommitDate)
			}
			return wa.LastCommitDate.After(wb.LastCommitDate)

		case SortByBranch:
			ba := strings.ToLower(stripBranchPrefix(wa.Branch))
			bb := strings.ToLower(stripBranchPrefix(wb.Branch))
			if sortAsc {
				return ba < bb
			}
			return ba > bb

		default:
			return false
		}
	})

	m.remove.visibleIndices = indices
	m.clampListWindow()
}

// listRows is how many worktree rows the list can show: the shared body
// height less the chrome only the list draws.
func (m Model) listRows() int {
	return max(m.height-listExtraChromeRows, 1)
}

// clampListWindow puts the cursor on a row, scrolls the window to contain it
// and stops the window running past the end of the list. Everything that
// moves the cursor, changes the rows or resizes the terminal ends here.
func (m *Model) clampListWindow() {
	rows, height := len(m.remove.visibleIndices), m.listRows()
	m.remove.cursor = max(min(m.remove.cursor, rows-1), 0)
	m.remove.offset = min(m.remove.offset, m.remove.cursor)
	m.remove.offset = max(m.remove.offset, m.remove.cursor-height+1)
	m.remove.offset = max(min(m.remove.offset, rows-height), 0)
}

// flowIdentity names the live operation in its two voices: a sentence form
// for the quit trace and a compact verb for the terminal tab title.
func (m Model) flowIdentity() (sentence, verb string) {
	switch m.view {
	case progressView:
		return "worktree removal", "removing"
	case createProgressView:
		return "worktree creation", "creating"
	case repoProgressView:
		switch m.repo.opType {
		case "clone":
			return "repository clone", "cloning"
		case "migrate":
			return "repository migration", "migrating"
		}
		return "repository creation", "creating"
	case migrateProgressView:
		return "repository migration", "migrating"
	}
	return "", ""
}

// InterruptedFlow names the operation in flight when the model is still on a
// progress view, used by main to leave a stderr trace after a mid-flow quit.
func (m Model) InterruptedFlow() string {
	sentence, _ := m.flowIdentity()
	return sentence
}

// windowTitle names the terminal tab: the repo at rest, the live operation
// with its counts in flight.
func (m Model) windowTitle() string {
	title := "sentei · " + filepath.Base(m.repoPath)
	if m.view == cleanupPreviewView && m.cleanupScan == nil {
		return title + " · scanning"
	}
	if _, verb := m.flowIdentity(); verb != "" {
		if l, ok := m.activeProgressLayout(); ok {
			if done, total := l.overall(); total > 0 {
				return fmt.Sprintf("%s · %s %d/%d", title, verb, done, total)
			}
		}
		return title + " · " + verb
	}
	return title
}

// terminalProgress mirrors flow progress into the terminal's native (OSC 9;4)
// indicator, from the same source as the spring target.
func (m Model) terminalProgress() *tea.ProgressBar {
	if (m.view == cleanupPreviewView && m.cleanupScan == nil) || m.indeterminateWaitActive() {
		return &tea.ProgressBar{State: tea.ProgressBarIndeterminate}
	}
	l, ok := m.activeProgressLayout()
	if !ok {
		return nil
	}
	state := tea.ProgressBarDefault
	for _, p := range l.Phases {
		if p.Failed > 0 {
			state = tea.ProgressBarError
		}
	}
	pct := 0
	done, total := l.overall()
	switch {
	case total > 0:
		pct = min((done*100)/total, 100)
	case l.Completed:
		pct = 100
	}
	return &tea.ProgressBar{State: state, Value: pct}
}
