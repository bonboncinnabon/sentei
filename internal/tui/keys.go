package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	Toggle      key.Binding
	All         key.Binding
	Confirm     key.Binding
	QuickCreate key.Binding
	Quit        key.Binding
	Interrupt   key.Binding
	Yes         key.Binding
	No          key.Binding
	Back        key.Binding
	Tab         key.Binding
	Sort        key.Binding
	ReverseSort key.Binding
	Filter      key.Binding
	Info        key.Binding
	GlobalHelp  key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("k/up", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("j/down", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdown", "page down"),
	),
	Toggle: key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "toggle"),
	),
	All: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "all"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "delete"),
	),
	QuickCreate: key.NewBinding(
		key.WithKeys("ctrl+enter"),
		key.WithHelp("ctrl+enter", "quick create"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	// Interrupt quits from every view, text inputs included, where q is
	// just a letter. The model handles it before any view sees the key.
	Interrupt: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
	Yes: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "yes"),
	),
	No: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "no"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch field"),
	),
	Sort: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "sort"),
	),
	ReverseSort: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "reverse sort"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Info: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "details"),
	),
	GlobalHelp: key.NewBinding(
		key.WithKeys("f1"),
		key.WithHelp("F1", "help"),
	),
}

// keySection is a named group of bindings for the help portal; the same
// bindings drive footer hints so footer and help can never disagree.
type keySection struct {
	name     string
	bindings []key.Binding
}

// withDesc derives a presentation binding from a canonical one, overriding
// only the description: key strings stay defined exactly once above.
func withDesc(b key.Binding, desc string) key.Binding {
	b.SetHelp(b.Help().Key, desc)
	return b
}

// hintOnly declares a presentation-only entry (e.g. a combined "j/k" pair)
// rendered in footers and help but never matched against input. The label
// doubles as the key list because bubbles/help skips keyless bindings as
// disabled; these bindings are never passed to key.Matches.
func hintOnly(label, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(label), key.WithHelp(label, desc))
}

// Per-view presentation: footer subsets and help sections. Render sites
// reference these and contain no key or label literals.
var (
	navHint = hintOnly("j/k", "navigate")

	helpGlobalSection = keySection{name: "Global", bindings: []key.Binding{
		withDesc(keys.GlobalHelp, "toggle this help"),
		hintOnly("q / ctrl+c", "quit"),
	}}
	scrollHint = hintOnly("j/k", "scroll")

	menuFooter   = []key.Binding{navHint, withDesc(keys.Confirm, "select"), keys.GlobalHelp, keys.Quit}
	menuSections = []keySection{{name: "Navigation", bindings: []key.Binding{
		hintOnly("j/k, ↑/↓", "move between entries"),
		withDesc(keys.Confirm, "select"),
	}}}

	// Curated to fit the 80-col minimum beside the selection-count prefix;
	// select-all and sort stay discoverable in the help sections (? opens
	// them).
	listFooter = []key.Binding{
		keys.Toggle, withDesc(keys.Confirm, "delete"),
		keys.Filter, keys.Info, keys.Quit,
	}
	listFooterNoSelection = []key.Binding{
		keys.Toggle, keys.Filter, keys.Info, keys.Quit,
	}
	listFilterFooter = []key.Binding{withDesc(keys.Confirm, "apply"), withDesc(keys.Back, "cancel")}
	listSections     = []keySection{
		{name: "Navigation", bindings: []key.Binding{
			hintOnly("j/k, ↑/↓", "move cursor"),
			hintOnly("pgup/pgdn", "page"),
		}},
		{name: "Selection", bindings: []key.Binding{
			withDesc(keys.Toggle, "toggle worktree"),
			withDesc(keys.All, "select all"),
			withDesc(keys.Confirm, "delete selected"),
		}},
		{name: "Organize", bindings: []key.Binding{
			withDesc(keys.Filter, "filter by name"),
			hintOnly("s / S", "cycle / reverse sort"),
			withDesc(keys.Info, "details for highlighted worktree"),
		}},
	}

	confirmFooter   = []key.Binding{withDesc(keys.Yes, "delete"), withDesc(keys.No, "go back")}
	confirmSections = []keySection{{name: "Actions", bindings: []key.Binding{
		withDesc(keys.Yes, "delete the selected worktrees"),
		withDesc(keys.No, "go back to the list"),
	}}}

	confirmationFooter = []key.Binding{withDesc(keys.Confirm, "confirm"), keys.Back, keys.Quit}

	createBranchFooter = []key.Binding{
		withDesc(keys.Confirm, "continue"), keys.QuickCreate, keys.Tab, keys.Back,
	}
	cloneInputFooter = []key.Binding{withDesc(keys.Confirm, "clone"), keys.Tab, keys.Back}
	repoNameFooter   = []key.Binding{withDesc(keys.Confirm, "continue"), keys.Tab, keys.Back}
	inputSections    = []keySection{{name: "Editing", bindings: []key.Binding{
		keys.Tab,
		withDesc(keys.Confirm, "continue"),
		keys.Back,
	}}}

	optionsFooter   = []key.Binding{navHint, keys.Toggle, withDesc(keys.Confirm, "create"), keys.Back}
	optionsSections = []keySection{{name: "Actions", bindings: []key.Binding{
		hintOnly("j/k", "move"),
		withDesc(keys.Toggle, "toggle option"),
		withDesc(keys.Confirm, "create"),
		keys.Back,
	}}}

	summaryMenuFooter = []key.Binding{withDesc(keys.Confirm, "menu"), keys.Quit}
	summaryQuitFooter = []key.Binding{withDesc(keys.Confirm, "quit"), withDesc(keys.Back, "quit")}
	createSummaryQuit = []key.Binding{withDesc(keys.Confirm, "quit"), keys.Quit}
	repoSummaryFooter = []key.Binding{withDesc(keys.Confirm, "open in sentei"), keys.Quit}
	quitOnlyFooter    = []key.Binding{keys.Quit}
	cleanupDoneFooter = []key.Binding{withDesc(keys.Confirm, "quit")}
	summarySections   = []keySection{{name: "Actions", bindings: []key.Binding{
		withDesc(keys.Confirm, "continue"),
		keys.Back,
	}}}

	// The cleanup preview's keys depend on what the scan found; see
	// cleanupPreviewActions.
	cleanupSafeHint = withDesc(keys.Confirm, "safe cleanup")
	aggressiveHint  = withDesc(keys.All, "aggressive")
	detailsHint     = keys.Info

	progressFooter   = []key.Binding{keys.Quit}
	progressSections = []keySection{{name: "Actions", bindings: []key.Binding{
		hintOnly("q / ctrl+c", "quit (asks first: the operation stops unfinished)"),
	}}}
	// quitPromptFooter replaces a view's footer after the first quit key
	// during a running operation. The danger hint leads, as on every
	// destructive footer.
	quitPromptFooter = []key.Binding{
		withDesc(keys.Quit, "quit now, leaving the operation unfinished"),
		hintOnly("any other key", "keep going"),
	}

	confirmationSections = []keySection{{name: "Actions", bindings: []key.Binding{
		withDesc(keys.Confirm, "confirm"),
		keys.Back,
	}}}

	portalFooter = []key.Binding{withDesc(keys.Back, "close"), scrollHint}
	// portalFooterStatic is the portal footer when content fits: scroll
	// hints would be dead keys.
	portalFooterStatic = []key.Binding{withDesc(keys.Back, "close")}

	migrateBlockedFooter = []key.Binding{keys.Back, keys.Quit}
	migrateOpenFooter    = []key.Binding{withDesc(keys.Confirm, "open in sentei"), withDesc(keys.Quit, "exit")}
)
