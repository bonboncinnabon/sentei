package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

func TestPrepareRemoval_SemanticIDsSurviveSelectionReordering(t *testing.T) {
	a := git.Worktree{Path: "/repo/../repo/a", Branch: "refs/heads/feature/a", IsLocked: true}
	b := git.Worktree{Path: "/repo/b", Branch: "refs/heads/feature/b", IsLocked: true}

	first, err := prepareRemoval([]git.Worktree{a, b}, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareRemoval([]git.Worktree{b, a}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(stepIDsByLabel(first.plan), stepIDsByLabel(second.plan)) {
		t.Fatalf("IDs changed after reorder:\nfirst=%v\nsecond=%v", stepIDsByLabel(first.plan), stepIDsByLabel(second.plan))
	}
	for _, phase := range first.plan.Phases {
		if phase.ID == cleanupPhaseID {
			continue
		}
		for _, step := range phase.Steps {
			if !strings.Contains(string(step.ID), ":") || strings.HasSuffix(string(step.ID), "-0") || strings.HasSuffix(string(step.ID), "-1") {
				t.Fatalf("step ID %q is not semantic", step.ID)
			}
		}
	}
}

func TestPrepareRemoval_RejectsDuplicateSemanticWorktree(t *testing.T) {
	_, err := prepareRemoval([]git.Worktree{
		{Path: "/repo/a", Branch: "refs/heads/feature/a"},
		{Path: "/repo/x/../a", Branch: "refs/heads/feature/a"},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate worktree identity") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestPrepareRemoval_RejectsDuplicateSemanticTeardown(t *testing.T) {
	wtPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(wtPath, ".artifact"), 0o755); err != nil {
		t.Fatal(err)
	}
	duplicate := integration.Integration{Name: "tool", Teardown: integration.TeardownSpec{Command: "tool clean", Dirs: []string{".artifact/"}}}
	_, err := prepareRemoval([]git.Worktree{{Path: wtPath, Branch: "refs/heads/a"}}, []integration.Integration{duplicate, duplicate})
	if err == nil || !strings.Contains(err.Error(), "duplicate teardown identity") {
		t.Fatalf("duplicate teardown error = %v", err)
	}
}

func TestPrepareRemoval_RejectsUnsafeArtifactPathBeforeScanning(t *testing.T) {
	wtPath := t.TempDir()
	integrations := []integration.Integration{{
		Name: "tool", Teardown: integration.TeardownSpec{Dirs: []string{"../victim"}},
	}}

	_, err := prepareRemoval([]git.Worktree{{Path: wtPath, Branch: "refs/heads/a"}}, integrations)
	if err == nil || !strings.Contains(err.Error(), "managed path") {
		t.Fatalf("prepareRemoval error = %v, want managed path validation error", err)
	}
}

func TestPrepareRemoval_RejectsSymlinkedArtifactPathBeforeScanning(t *testing.T) {
	wtPath := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(wtPath, "escape")); err != nil {
		t.Fatal(err)
	}
	integrations := []integration.Integration{{
		Name: "tool", Teardown: integration.TeardownSpec{Dirs: []string{"escape/victim"}},
	}}

	_, err := prepareRemoval([]git.Worktree{{Path: wtPath, Branch: "refs/heads/a"}}, integrations)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("prepareRemoval error = %v, want symlink validation error", err)
	}
}

func TestPrepareRemoval_ArtifactOperationsHaveStableSortedSemanticIDs(t *testing.T) {
	wtPath := t.TempDir()
	for _, dir := range []string{"zeta", "alpha"} {
		if err := os.Mkdir(filepath.Join(wtPath, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	integrationWithDirs := func(dirs []string) integration.Integration {
		return integration.Integration{
			Name: "tool", Teardown: integration.TeardownSpec{Command: "tool clean", Dirs: dirs},
		}
	}

	first, err := prepareRemoval([]git.Worktree{{Path: wtPath, Branch: "refs/heads/a"}}, []integration.Integration{integrationWithDirs([]string{"zeta", "alpha"})})
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareRemoval([]git.Worktree{{Path: wtPath, Branch: "refs/heads/a"}}, []integration.Integration{integrationWithDirs([]string{"alpha", "zeta"})})
	if err != nil {
		t.Fatal(err)
	}

	if len(first.teardownOps) != 3 {
		t.Fatalf("teardown operations = %d, want command plus two removals", len(first.teardownOps))
	}
	for i := range first.teardownOps {
		if first.teardownOps[i].stepID != second.teardownOps[i].stepID || first.teardownOps[i].label != second.teardownOps[i].label {
			t.Fatalf("operation %d changed under input reorder: first=%+v second=%+v", i, first.teardownOps[i], second.teardownOps[i])
		}
	}
	wantLabels := []string{integration.RemoveDirStepName("alpha", wtPath), integration.RemoveDirStepName("zeta", wtPath)}
	if got := []string{first.teardownOps[1].label, first.teardownOps[2].label}; !reflect.DeepEqual(got, wantLabels) {
		t.Fatalf("artifact operation order = %v", got)
	}
}

func stepIDsByLabel(plan progress.Plan) map[string]progress.StepID {
	ids := map[string]progress.StepID{}
	for _, phase := range plan.Phases {
		for _, step := range phase.Steps {
			ids[string(phase.ID)+"\x00"+step.Label] = step.ID
		}
	}
	return ids
}
