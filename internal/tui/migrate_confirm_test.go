package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func makeMigrateConfirmModel(opts *MigrateOpts) Model {
	m := NewMenuModel(nil, nil, "/some/repo", &config.Config{}, repo.ContextNonBareRepo)
	m.width = 80
	m.height = 24
	if opts != nil {
		m.SetMigrateOpts(opts)
	} else {
		m.view = migrateConfirmView
	}
	return m
}

func readyMigrateInfo() MigrateInfo {
	return MigrateInfo{Loaded: true, Plan: repo.MigratePlan{
		RepoPath: "/some/repo", Branch: "feature/x", WorktreePath: "/some/repo/feature-x", DefaultBranch: "trunk",
	}}
}

func TestSetMigrateOpts_EntersConfirmView(t *testing.T) {
	m := NewMenuModel(nil, nil, "/some/repo", &config.Config{}, repo.ContextNonBareRepo)

	opts := &MigrateOpts{RepoPath: "/some/repo"}
	m.SetMigrateOpts(opts)

	if m.view != migrateConfirmView {
		t.Errorf("expected view=migrateConfirmView, got %d", m.view)
	}
	if m.migrateOpts != opts {
		t.Error("expected migrateOpts to be set")
	}
}

func TestSetMigrateOpts_OverridesRepoPath(t *testing.T) {
	m := NewMenuModel(nil, nil, "/original", &config.Config{}, repo.ContextNonBareRepo)

	m.SetMigrateOpts(&MigrateOpts{RepoPath: "/overridden/repo"})

	if m.repoPath != "/overridden/repo" {
		t.Errorf("expected repoPath='/overridden/repo', got %q", m.repoPath)
	}
}

func TestSetMigrateOpts_EmptyRepoPathKeepsOriginal(t *testing.T) {
	m := NewMenuModel(nil, nil, "/original", &config.Config{}, repo.ContextNonBareRepo)

	m.SetMigrateOpts(&MigrateOpts{RepoPath: ""})

	if m.repoPath != "/original" {
		t.Errorf("expected repoPath='/original', got %q", m.repoPath)
	}
}

func TestMigrateConfirmView_DirectLaunchShowsPathAndCommand(t *testing.T) {
	m := makeMigrateConfirmModel(&MigrateOpts{RepoPath: "/my/repo"})

	output := stripAnsi(m.viewMigrateConfirm())

	for _, want := range []string{"Confirm migration", "/my/repo", "sentei migrate"} {
		if !strings.Contains(output, want) {
			t.Errorf("direct-launch view missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "backup") {
		t.Errorf("migrate no longer makes a backup; the view must not mention one:\n%s", output)
	}
}

func TestMigrateConfirmView_WaitsForThePreflight(t *testing.T) {
	m := makeMigrateConfirmModel(nil)

	output := stripAnsi(m.viewMigrateConfirm())

	if !strings.Contains(output, "Checking repository") {
		t.Errorf("expected a checking notice before the pre-flight answers:\n%s", output)
	}
	// Nothing about the repository may be claimed before it has been read.
	for _, premature := range []string{"clean", "This will", "enter confirm"} {
		if strings.Contains(output, premature) {
			t.Errorf("view shows %q before the pre-flight answered:\n%s", premature, output)
		}
	}
}

func TestMigrateConfirmView_ShowsThePlan(t *testing.T) {
	m := makeMigrateConfirmModel(nil)
	m.repo.migrateInfo = readyMigrateInfo()

	output := stripAnsi(m.viewMigrateConfirm())

	for _, want := range []string{"Migrate to bare repository", "feature/x", "trunk", "Move .git to .bare", "repo/feature-x", "Nothing is copied or deleted", "enter confirm"} {
		if !strings.Contains(output, want) {
			t.Errorf("view missing %q:\n%s", want, output)
		}
	}
}

func TestMigrateConfirmView_ShowsTheRefusal(t *testing.T) {
	m := makeMigrateConfirmModel(nil)
	m.repo.migrateInfo = MigrateInfo{Loaded: true, Err: errors.New("HEAD is detached; check out a branch first")}

	output := stripAnsi(m.viewMigrateConfirm())

	if !strings.Contains(output, "cannot be migrated yet") || !strings.Contains(output, "HEAD is detached") {
		t.Errorf("expected the refusal and its reason:\n%s", output)
	}
	if strings.Contains(output, "enter confirm") {
		t.Errorf("a refused migration must not offer confirm:\n%s", output)
	}
}

func TestUpdateMigrateConfirm_EnterIsInertUntilThePreflightAllows(t *testing.T) {
	tests := []struct {
		name string
		info MigrateInfo
	}{
		{"still checking", MigrateInfo{}},
		{"refused", MigrateInfo{Loaded: true, Err: errors.New("HEAD is detached")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := makeMigrateConfirmModel(nil)
			m.repo.migrateInfo = test.info

			updated, cmd := m.updateMigrateConfirm(ConfirmProceedMsg{})

			if got := updated.(Model).view; got != migrateConfirmView || cmd != nil {
				t.Errorf("view = %d, cmd nil = %v; the migration must not start", got, cmd == nil)
			}
		})
	}
}

func TestUpdateMigrateConfirm_EnterStartsTheMigration(t *testing.T) {
	tests := []struct {
		name  string
		model func() Model
	}{
		{"menu launch after a clear pre-flight", func() Model {
			m := makeMigrateConfirmModel(nil)
			m.repo.migrateInfo = readyMigrateInfo()
			return m
		}},
		// A direct launch shows no pre-flight; the migration checks first.
		{"direct launch", func() Model { return makeMigrateConfirmModel(&MigrateOpts{RepoPath: t.TempDir()}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := test.model()
			m.runner = &stubRunner{responses: map[string]stubResponse{}}

			updated, cmd := m.updateMigrateConfirm(ConfirmProceedMsg{})

			started := updated.(Model)
			if started.view != migrateProgressView || cmd == nil {
				t.Fatalf("view = %d, cmd nil = %v; want the migration running", started.view, cmd == nil)
			}
			drainRepoPipeline(t, started)
		})
	}
}

func TestUpdateMigrateConfirm_BackQuitsWhenDirectLaunch(t *testing.T) {
	m := makeMigrateConfirmModel(&MigrateOpts{RepoPath: "/repo"})

	updated, cmd := m.updateMigrateConfirm(ConfirmBackMsg{})
	_ = updated.(Model)

	if cmd == nil {
		t.Fatal("expected quit cmd when launched directly with opts")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestUpdateMigrateConfirm_BackReturnsToMenuAndForgetsThePreflight(t *testing.T) {
	m := makeMigrateConfirmModel(nil)
	m.repo.migrateInfo = readyMigrateInfo()

	updated, cmd := m.updateMigrateConfirm(ConfirmBackMsg{})
	result := updated.(Model)

	if result.view != menuView {
		t.Errorf("expected view=menuView, got %d", result.view)
	}
	if cmd != nil {
		t.Error("expected nil cmd when going back to menu")
	}
	if result.repo.migrateInfo.Loaded {
		t.Error("a stale pre-flight answer would let the next visit confirm before its own check")
	}
}

func TestUpdateMigrateConfirm_MigrateInfoUpdatesState(t *testing.T) {
	m := makeMigrateConfirmModel(nil)
	refusal := errors.New("refused")

	updated, _ := m.updateMigrateConfirm(migrateInfoMsg{plan: repo.MigratePlan{Branch: "main"}, err: refusal})
	info := updated.(Model).repo.migrateInfo

	if !info.Loaded || info.Plan.Branch != "main" || !errors.Is(info.Err, refusal) {
		t.Errorf("migrateInfo = %+v, want the loaded plan and refusal", info)
	}
}

func TestUpdateMigrateConfirm_WindowSizeMsg(t *testing.T) {
	m := makeMigrateConfirmModel(nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := updated.(Model)

	if result.width != 120 {
		t.Errorf("expected width=120, got %d", result.width)
	}
}

func TestUpdateMigrateConfirm_QuitKey(t *testing.T) {
	m := makeMigrateConfirmModel(nil)

	_, cmd := m.updateMigrateConfirm(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected quit cmd for q key")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

// The confirm screen asks the same pre-flight the migration runs, against a
// real repository built for this test.
func TestLoadMigrateInfo(t *testing.T) {
	runner := &git.GitRunner{}
	root := filepath.Join(t.TempDir(), "proj")
	mustRun := func(dir string, args ...string) {
		t.Helper()
		if _, err := runner.Run(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustRun(filepath.Dir(root), "init", "-q", "-b", "main", root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(root, "add", ".")
	mustRun(root, "commit", "-qm", "init")

	info, ok := loadMigrateInfo(runner, root)().(migrateInfoMsg)
	if !ok || info.err != nil {
		t.Fatalf("msg = %#v, want a clear pre-flight", info)
	}
	if info.plan.Branch != "main" || info.plan.WorktreePath != filepath.Join(root, "main") {
		t.Errorf("plan = %+v", info.plan)
	}

	mustRun(root, "checkout", "-q", "--detach")
	info, _ = loadMigrateInfo(runner, root)().(migrateInfoMsg)
	if info.err == nil || !strings.Contains(info.err.Error(), "HEAD is detached") {
		t.Errorf("err = %v, want the detached HEAD refusal", info.err)
	}
}

// columnOf returns the display column at which text starts on its line.
func columnOf(t *testing.T, view, text string) int {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if index := strings.Index(line, text); index >= 0 {
			return len([]rune(line[:index]))
		}
	}
	t.Fatalf("%q not found in:\n%s", text, view)
	return 0
}

func TestMigrateConfirmView_WrappedRefusalKeepsItsIndent(t *testing.T) {
	m := makeMigrateConfirmModel(nil)
	m.repo.migrateInfo = MigrateInfo{Loaded: true, Err: errors.New(strings.Repeat("blocked ", 30))}

	assertWrappedIndent(t, stripAnsi(m.viewMigrateConfirm()), "blocked")
}

// assertWrappedIndent checks that a message long enough to wrap keeps the
// four-space indent on every line it spans.
func assertWrappedIndent(t *testing.T, view, word string) {
	t.Helper()
	wrapped := 0
	for _, line := range strings.Split(view, "\n") {
		if !strings.Contains(line, word) {
			continue
		}
		wrapped++
		if !strings.HasPrefix(line, "    ") {
			t.Errorf("wrapped line lost its indent: %q", line)
		}
	}
	if wrapped < 2 {
		t.Fatalf("fixture: the message was meant to wrap, got %d line(s):\n%s", wrapped, view)
	}
}
