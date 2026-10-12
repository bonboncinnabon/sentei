package cli

import "testing"

func yesTestRegistry() *Registry {
	r := NewRegistry()
	r.Register(&Command{Name: "cleanup", Type: Decision})
	r.Register(&Command{Name: "remove", Type: Decision})
	return r
}

func TestDispatch_YesFlagExtracted(t *testing.T) {
	r := yesTestRegistry()
	for _, flag := range []string{"--yes", "-y"} {
		result, err := r.Dispatch([]string{"cleanup", flag, "--mode", "safe"})
		if err != nil {
			t.Fatalf("Dispatch(%s) error: %v", flag, err)
		}
		if !result.Yes {
			t.Errorf("%s must set Yes", flag)
		}
		for _, a := range result.Args {
			if a == flag {
				t.Errorf("%s must be consumed, found in remaining args %v", flag, result.Args)
			}
		}
	}
}

func TestDispatch_YesAndNonInteractiveAreEquivalent(t *testing.T) {
	r := yesTestRegistry()
	for _, flag := range []string{"--yes", "--non-interactive"} {
		result, err := r.Dispatch([]string{"cleanup", flag})
		if err != nil {
			t.Fatalf("%s alone must dispatch (command safeties apply): %v", flag, err)
		}
		if result.Force {
			t.Errorf("%s must not imply --force", flag)
		}
	}
}
