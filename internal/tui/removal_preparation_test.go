package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

func TestPrepareRemoval_SemanticIDsSurviveSelectionReordering(t *testing.T) {
	a := git.Worktree{Path: "/repo/../repo/a", Branch: "refs/heads/feature/a", IsLocked: true}
	b := git.Worktree{Path: "/repo/b", Branch: "refs/heads/feature/b", IsLocked: true}

	first, err := prepareRemoval([]git.Worktree{a, b})
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareRemoval([]git.Worktree{b, a})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(stepIDsByLabel(first.plan), stepIDsByLabel(second.plan)) {
		t.Fatalf("IDs changed after reorder:\nfirst=%v\nsecond=%v", stepIDsByLabel(first.plan), stepIDsByLabel(second.plan))
	}
	for _, phase := range first.plan.Phases {
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
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate worktree identity") {
		t.Fatalf("duplicate error = %v", err)
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
