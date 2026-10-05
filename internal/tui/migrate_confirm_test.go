package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
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

func TestSetMigrateOpts_EntersConfirmView(t *testing.T) {
	m := NewMenuModel(nil, nil, "/some/repo", &config.Config{}, repo.ContextNonBareRepo)

	opts := &MigrateOpts{DeleteBackup: true, RepoPath: "/some/repo"}
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

func TestMigrateConfirmationVM_RepoPathAndDeleteBackup(t *testing.T) {
	m := makeMigrateConfirmModel(&MigrateOpts{DeleteBackup: true, RepoPath: "/my/repo"})
	vm := m.migrateConfirmationVM()

	if vm.Title != "Confirm migration" {
		t.Errorf("expected title 'Confirm migration', got %q", vm.Title)
	}

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "/my/repo") {
		t.Errorf("expected repo path in view, got:\n%s", output)
	}
	if !strings.Contains(output, "yes") {
		t.Errorf("expected delete backup 'yes' in view, got:\n%s", output)
	}
	if !strings.Contains(output, "--delete-backup") {
		t.Errorf("expected '--delete-backup' in CLI command, got:\n%s", output)
	}
}

func TestMigrateConfirmationVM_NoDeleteBackup(t *testing.T) {
	m := makeMigrateConfirmModel(&MigrateOpts{DeleteBackup: false, RepoPath: "/my/repo"})
	vm := m.migrateConfirmationVM()

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "no") {
		t.Errorf("expected delete backup 'no' in view, got:\n%s", output)
	}
	if strings.Contains(output, "--delete-backup") {
		t.Errorf("expected no '--delete-backup' in CLI command, got:\n%s", output)
	}
	if !strings.Contains(output, "sentei migrate") {
		t.Errorf("expected 'sentei migrate' in CLI command, got:\n%s", output)
	}
}

func TestMigrateConfirmView_DirectLaunchUsesConfirmationVM(t *testing.T) {
	m := makeMigrateConfirmModel(&MigrateOpts{DeleteBackup: true, RepoPath: "/my/repo"})

	output := stripAnsi(m.viewMigrateConfirm())

	if !strings.Contains(output, "Confirm migration") {
		t.Error("expected 'Confirm migration' title in direct-launch view")
	}
	if !strings.Contains(output, "/my/repo") {
		t.Error("expected repo path in direct-launch view")
	}
}

func TestMigrateConfirmView_MenuLaunchUsesOriginalView(t *testing.T) {
	m := makeMigrateConfirmModel(nil)

	output := stripAnsi(m.viewMigrateConfirm())

	if !strings.Contains(output, "Migrate to bare repository") {
		t.Errorf("expected 'Migrate to bare repository' in menu-launch view, got:\n%s", output)
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

func TestUpdateMigrateConfirm_BackReturnsToMenu(t *testing.T) {
	m := makeMigrateConfirmModel(nil)

	updated, cmd := m.updateMigrateConfirm(ConfirmBackMsg{})
	result := updated.(Model)

	if result.view != menuView {
		t.Errorf("expected view=menuView, got %d", result.view)
	}
	if cmd != nil {
		t.Error("expected nil cmd when going back to menu")
	}
}

func TestUpdateMigrateConfirm_MigrateInfoUpdatesState(t *testing.T) {
	m := makeMigrateConfirmModel(nil)

	updated, _ := m.updateMigrateConfirm(migrateInfoMsg{branch: "main", isDirty: true})
	result := updated.(Model)

	if result.repo.migrateInfo.Branch != "main" {
		t.Errorf("expected branch='main', got %q", result.repo.migrateInfo.Branch)
	}
	if !result.repo.migrateInfo.IsDirty {
		t.Error("expected isDirty=true")
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

func TestLoadMigrateInfo(t *testing.T) {
	cases := []struct {
		name       string
		responses  map[string]stubResponse
		wantBranch string
		wantDirty  bool
		wantErr    bool
	}{
		{
			"clean repo",
			map[string]stubResponse{
				"/some/repo branch --show-current": {output: "main"},
				"/some/repo status --porcelain":    {output: ""},
			},
			"main", false, false,
		},
		{
			"dirty repo",
			map[string]stubResponse{
				"/some/repo branch --show-current": {output: "main"},
				"/some/repo status --porcelain":    {output: " M file.go"},
			},
			"main", true, false,
		},
		{
			"branch lookup fails",
			map[string]stubResponse{},
			"", false, true,
		},
		{
			"status lookup fails",
			map[string]stubResponse{
				"/some/repo branch --show-current": {output: "main"},
			},
			"", false, true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &stubRunner{responses: tc.responses}

			msg := loadMigrateInfo(runner, "/some/repo")()

			info, ok := msg.(migrateInfoMsg)
			if !ok {
				t.Fatalf("expected migrateInfoMsg, got %T", msg)
			}
			if tc.wantErr {
				if info.err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if info.err != nil {
				t.Fatalf("unexpected error: %v", info.err)
			}
			if info.branch != tc.wantBranch || info.isDirty != tc.wantDirty {
				t.Errorf("info = branch %q dirty %v, want %q/%v", info.branch, info.isDirty, tc.wantBranch, tc.wantDirty)
			}
		})
	}
}
