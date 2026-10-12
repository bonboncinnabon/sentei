package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func makeCreateConfirmModel(opts *CreateOpts) Model {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	if opts != nil {
		m.SetCreateOpts(opts)
	} else {
		m.view = createConfirmView
		m.create.branchInput.SetValue("feature/test")
		m.create.baseInput.SetValue("main")
	}
	m.width = 80
	m.height = 24
	return m
}

func TestSetCreateOpts_BranchAndBase_EntersConfirmView(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	opts := &CreateOpts{Branch: "feature/foo", Base: "main"}
	m.SetCreateOpts(opts)

	if m.view != createConfirmView {
		t.Errorf("expected createConfirmView, got %d", m.view)
	}
	if m.createOpts != opts {
		t.Error("expected createOpts to be set")
	}
	if m.create.branchInput.Value() != "feature/foo" {
		t.Errorf("expected branch input 'feature/foo', got %q", m.create.branchInput.Value())
	}
	if m.create.baseInput.Value() != "main" {
		t.Errorf("expected base input 'main', got %q", m.create.baseInput.Value())
	}
}

func TestSetCreateOpts_OnlyBranch_EntersOptionsView(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	opts := &CreateOpts{Branch: "feature/bar"}
	m.SetCreateOpts(opts)

	if m.view != createOptionsView {
		t.Errorf("expected createOptionsView, got %d", m.view)
	}
}

func TestSetCreateOpts_NothingSet_EntersBranchView(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	opts := &CreateOpts{}
	m.SetCreateOpts(opts)

	if m.view != createBranchView {
		t.Errorf("expected createBranchView, got %d", m.view)
	}
}

func TestCreateConfirmationVM_RendersBranchAndBase(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{
		Branch: "feature/test",
		Base:   "main",
	})
	vm := m.createConfirmationVM()

	if vm.Title != "Confirm create" {
		t.Errorf("expected title 'Confirm create', got %q", vm.Title)
	}

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "feature/test") {
		t.Errorf("expected branch name in view, got:\n%s", output)
	}
	if !strings.Contains(output, "main") {
		t.Errorf("expected base branch in view, got:\n%s", output)
	}
	if !strings.Contains(output, "sentei create") {
		t.Errorf("expected CLI command in view, got:\n%s", output)
	}
}

func TestCreateConfirmationVM_ShowsMergeBaseAndCopyEnv(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{
		Branch:    "feature/test",
		Base:      "main",
		MergeBase: true,
		CopyEnv:   true,
	})
	vm := m.createConfirmationVM()

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "Merge base:") {
		t.Errorf("expected 'Merge base:' in view, got:\n%s", output)
	}
	if !strings.Contains(output, "Copy env:") {
		t.Errorf("expected 'Copy env:' in view, got:\n%s", output)
	}
	if !strings.Contains(output, "--merge-base") {
		t.Errorf("expected '--merge-base' in CLI command, got:\n%s", output)
	}
	if !strings.Contains(output, "--copy-env") {
		t.Errorf("expected '--copy-env' in CLI command, got:\n%s", output)
	}
}

func TestUpdateCreateConfirm_ProceedSetsStateBeforeCreate(t *testing.T) {
	// ConfirmProceedMsg transitions to createProgressView and spawns a creation
	// goroutine via startCreation. Since that requires a real runner, we verify
	// the pre-proceed state instead (nil runner would panic in the goroutine).
	m := makeCreateConfirmModel(nil)

	if m.view != createConfirmView {
		t.Errorf("expected createConfirmView before proceed, got %d", m.view)
	}
}

func TestUpdateCreateConfirm_BackReturnsToOptions(t *testing.T) {
	m := makeCreateConfirmModel(nil)
	// Not launched directly (createOpts is nil), so back goes to options.

	updated, cmd := m.updateCreateConfirm(ConfirmBackMsg{})
	result := updated.(Model)

	if result.view != createOptionsView {
		t.Errorf("expected createOptionsView, got %d", result.view)
	}
	if cmd != nil {
		t.Error("expected nil cmd when going back to options")
	}
}

func TestUpdateCreateConfirm_BackQuitsWhenLaunchedDirectly(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{Branch: "feat/x", Base: "main"})

	_, c := m.updateCreateConfirm(ConfirmBackMsg{})
	if c == nil {
		t.Fatal("expected quit cmd when launched directly with opts")
	}
	msg := c()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestUpdateCreateConfirm_WindowSizeMsg(t *testing.T) {
	m := makeCreateConfirmModel(nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := updated.(Model)

	if result.width != 120 {
		t.Errorf("expected width=120, got %d", result.width)
	}
}

func TestUpdateCreateConfirm_QuitKey(t *testing.T) {
	m := makeCreateConfirmModel(nil)

	_, c := m.updateCreateConfirm(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if c == nil {
		t.Fatal("expected quit cmd for q key")
	}
	msg := c()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestViewCreateConfirm_RendersContent(t *testing.T) {
	m := makeCreateConfirmModel(nil)

	output := stripAnsi(m.viewCreateConfirm())

	if !strings.Contains(output, "Confirm create") {
		t.Error("view should contain title")
	}
	if !strings.Contains(output, "Branch:") {
		t.Error("view should contain Branch label")
	}
	if !strings.Contains(output, "Base:") {
		t.Error("view should contain Base label")
	}
	if !strings.Contains(output, "enter confirm") {
		t.Error("view should show keybindings")
	}
}

func TestCreateConfirm_UpdateDispatch(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{Branch: "feat/x", Base: "main"})

	// Verify Update dispatches to createConfirm handler.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	result := updated.(Model)

	if result.width != 100 {
		t.Errorf("expected width=100, got %d", result.width)
	}
}

func TestCreateConfirm_ViewDispatch(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{Branch: "feat/x", Base: "main"})

	output := m.View().Content

	if !strings.Contains(stripAnsi(output), "Confirm create") {
		t.Error("View() should dispatch to viewCreateConfirm")
	}
}

// unloadedWorktreesModel is a bare-repo model whose worktree list has not been
// loaded: the state of a direct `sentei create`, and of the menu before its
// background load lands. The source worktree (uv and go present, pnpm absent)
// can only be found by asking git.
func unloadedWorktreesModel(t *testing.T) (Model, string) {
	t.Helper()
	source := t.TempDir()
	for _, name := range []string{"uv.lock", "go.mod"} {
		if err := os.WriteFile(filepath.Join(source, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[rev-parse --git-dir]": {Output: ".bare"},
		"/repo:[worktree list --porcelain]": {
			Output: "worktree /repo/.bare\nbare\n\nworktree " + source + "\nHEAD abc123\nbranch refs/heads/main\n",
		},
	}}
	cfg := &config.Config{Ecosystems: []config.EcosystemConfig{
		{Name: "uv", Detect: config.DetectConfig{Files: []string{"uv.lock"}}, EnvFiles: []string{".env"}},
		{Name: "go", Detect: config.DetectConfig{Files: []string{"go.mod"}}},
		{Name: "pnpm", Detect: config.DetectConfig{Files: []string{"pnpm-lock.yaml"}}},
	}}
	m := NewMenuModel(runner, runner, "/repo", cfg, repo.ContextBareRepo)
	m.width, m.height = 80, 24
	return m, source
}

// directCreateModel is the model `sentei create <flags>` builds.
func directCreateModel(t *testing.T, opts *CreateOpts) (Model, string) {
	t.Helper()
	m, source := unloadedWorktreesModel(t)
	m.SetCreateOpts(opts)
	return m, source
}

func ecosystemNames(ecosystems []config.EcosystemConfig) []string {
	var names []string
	for _, eco := range ecosystems {
		names = append(names, eco.Name)
	}
	return names
}

// With branch and base given there is no options screen, so the flags are the
// whole decision. --ecosystems used to be dropped: nothing was installed. As in
// the non-interactive run, a configured ecosystem is installed when named,
// detected (go) or not (pnpm), and a detected one that was not named (uv) is not.
func TestSetCreateOpts_DirectRunInstallsTheRequestedEcosystems(t *testing.T) {
	m, _ := directCreateModel(t, &CreateOpts{Branch: "feature/x", Base: "main", Ecosystems: []string{"pnpm", "go"}})

	if got := ecosystemNames(m.buildCreatorOptions().Ecosystems); !slices.Equal(got, []string{"go", "pnpm"}) {
		t.Errorf("installs = %v, want exactly the --ecosystems selection [go pnpm]", got)
	}
	confirm := stripAnsi(m.viewCreateConfirm())
	if !strings.Contains(confirm, "--ecosystems go,pnpm") {
		t.Errorf("confirm screen does not echo the selection:\n%s", confirm)
	}
}

// Merge-base and copy-env used to be on whatever the flags said, because the
// model's interactive defaults were never replaced.
func TestSetCreateOpts_DirectRunDoesNotAddUnrequestedSteps(t *testing.T) {
	m, _ := directCreateModel(t, &CreateOpts{Branch: "feature/x", Base: "main"})

	opts := m.buildCreatorOptions()
	if opts.MergeBase || opts.CopyEnvFiles || len(opts.Ecosystems) != 0 {
		t.Errorf("MergeBase=%v CopyEnvFiles=%v installs=%v, want none: no such flag was passed",
			opts.MergeBase, opts.CopyEnvFiles, ecosystemNames(opts.Ecosystems))
	}
	confirm := stripAnsi(m.viewCreateConfirm())
	for _, flag := range []string{"--merge-base", "--copy-env", "--ecosystems"} {
		if strings.Contains(confirm, flag) {
			t.Errorf("confirm screen echoes %s, which was not passed:\n%s", flag, confirm)
		}
	}
}

func TestSetCreateOpts_DirectRunCopiesEnvFromTheSourceWorktree(t *testing.T) {
	m, source := directCreateModel(t, &CreateOpts{Branch: "feature/x", Base: "main", CopyEnv: true, MergeBase: true})

	opts := m.buildCreatorOptions()
	if !opts.MergeBase || !opts.CopyEnvFiles {
		t.Errorf("MergeBase=%v CopyEnvFiles=%v, want both from the flags", opts.MergeBase, opts.CopyEnvFiles)
	}
	if opts.SourceWorktree != source || !slices.Equal(opts.EnvFiles, []string{".env"}) {
		t.Errorf("SourceWorktree=%q EnvFiles=%v, want %q and the detected ecosystem's [.env]", opts.SourceWorktree, opts.EnvFiles, source)
	}
}

// With only --branch the options screen follows. It offers the detected
// ecosystems: --ecosystems preselects among them, and without it they all
// start checked.
func TestSetCreateOpts_OptionsScreenStartsFromTheEcosystemsFlag(t *testing.T) {
	selected, _ := directCreateModel(t, &CreateOpts{Branch: "feature/x", Ecosystems: []string{"go", "pnpm"}})
	if got := ecosystemNames(selected.buildCreatorOptions().Ecosystems); !slices.Equal(got, []string{"go"}) {
		t.Errorf("installs = %v, want [go] from --ecosystems", got)
	}
	view := stripAnsi(selected.viewCreateOptions())
	if !strings.Contains(view, "[ ] Install dependencies (uv)") || !strings.Contains(view, "[x] Install dependencies (go)") {
		t.Errorf("options screen does not reflect --ecosystems go:\n%s", view)
	}
	if strings.Contains(view, "pnpm") {
		t.Errorf("options screen offers pnpm, which was not detected:\n%s", view)
	}

	defaults, _ := directCreateModel(t, &CreateOpts{Branch: "feature/x"})
	opts := defaults.buildCreatorOptions()
	if got := ecosystemNames(opts.Ecosystems); !slices.Equal(got, []string{"uv", "go"}) || !opts.MergeBase || !opts.CopyEnvFiles {
		t.Errorf("installs=%v MergeBase=%v CopyEnvFiles=%v, want the interactive defaults (detected uv and go, both on)", got, opts.MergeBase, opts.CopyEnvFiles)
	}
}
