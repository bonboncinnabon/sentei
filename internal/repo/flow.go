package repo

import (
	"errors"
	"fmt"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

// step is one declared unit of work in a repository flow.
type step struct {
	phaseID progress.PhaseID
	id      progress.StepID
	label   string
	run     progress.StepFunc
	// undo reverses run, including a run that failed part-way. It is called
	// when this step or a later one fails.
	undo func() error
	// optional steps never block the flow: a failure is recorded as skipped.
	optional bool
}

// flow is an ordered list of steps shared by create, clone and migrate. It
// runs until the first failure, skips what was still pending, and undoes every
// step that had started.
type flow struct {
	name     string // names the flow in errors, for example "repository clone"
	plan     progress.Plan
	steps    []step
	rollback *step // reports the undo; nil when no step has one
}

// add declares a step in the named phase. Steps of one phase must be added
// together: a new phase starts whenever the phase ID changes.
func (f *flow) add(phaseID progress.PhaseID, phaseLabel string, s step) {
	f.steps = append(f.steps, f.declare(phaseID, phaseLabel, s))
}

// addRollback declares the closing step that reports the undo of a failed
// run. Call it after every other step.
func (f *flow) addRollback(phaseID progress.PhaseID, id progress.StepID, label string) {
	rollback := f.declare(phaseID, "Rollback", step{id: id, label: label})
	f.rollback = &rollback
}

func (f *flow) declare(phaseID progress.PhaseID, phaseLabel string, s step) step {
	if last := len(f.plan.Phases) - 1; last < 0 || f.plan.Phases[last].ID != phaseID {
		f.plan.Phases = append(f.plan.Phases, progress.PlannedPhase{ID: phaseID, Label: phaseLabel})
	}
	phase := &f.plan.Phases[len(f.plan.Phases)-1]
	phase.Steps = append(phase.Steps, progress.PlannedStep{ID: s.id, Label: s.label})
	s.phaseID = phaseID
	return s
}

// run executes the flow and returns its per-step results. The error is
// reserved for a broken event stream; step failures live in the phases.
func (f *flow) run(emit func(progress.Event)) ([]progress.Phase, error) {
	execution, err := progress.Start(f.plan, emit)
	if err != nil {
		return nil, fmt.Errorf("starting %s: %w", f.name, err)
	}
	runErr := f.runSteps(execution)
	finishErr := execution.Finish(f.name + " finished")
	return execution.Phases(), errors.Join(runErr, finishErr)
}

func (f *flow) runSteps(execution *progress.Execution) error {
	var undos []func() error
	for index, s := range f.steps {
		if s.undo != nil {
			undos = append(undos, s.undo)
		}
		failed, err := runStep(execution, s)
		if err != nil {
			// The event stream is broken, so nothing more can be reported
			// as a step. The undo still has to happen.
			return errors.Join(fmt.Errorf("executing %s: %w", s.label, err), undoAll(undos))
		}
		if !failed {
			continue
		}
		for _, blocked := range f.steps[index+1:] {
			if _, err := execution.Skip(blocked.phaseID, blocked.id, "blocked by "+s.label); err != nil {
				return errors.Join(fmt.Errorf("skipping %s: %w", blocked.label, err), undoAll(undos))
			}
		}
		return f.rollBack(execution, undos)
	}
	return f.rollBack(execution, nil)
}

// runStep reports whether the step failed in a way that blocks the flow.
func runStep(execution *progress.Execution, s step) (failed bool, err error) {
	if !s.optional {
		result, err := execution.Run(s.phaseID, s.id, s.run)
		return result.Status == progress.StepFailed, err
	}
	if err := execution.Running(s.phaseID, s.id, 0, ""); err != nil {
		return false, err
	}
	message, runErr := s.run()
	if runErr != nil {
		_, err = execution.Skip(s.phaseID, s.id, runErr.Error())
		return false, err
	}
	_, err = execution.Done(s.phaseID, s.id, message)
	return false, err
}

// rollBack runs the undos and records the outcome on the rollback step. The
// undos run even when the step cannot be claimed.
func (f *flow) rollBack(execution *progress.Execution, undos []func() error) error {
	if f.rollback == nil {
		return undoAll(undos)
	}
	phaseID, id := f.rollback.phaseID, f.rollback.id
	if len(undos) == 0 {
		_, err := execution.Skip(phaseID, id, "rollback not required")
		return err
	}
	if err := execution.Running(phaseID, id, 0, ""); err != nil {
		return errors.Join(err, undoAll(undos))
	}
	undoErr := undoAll(undos)
	var err error
	if undoErr != nil {
		_, err = execution.Fail(phaseID, id, undoErr)
	} else {
		_, err = execution.Done(phaseID, id, "")
	}
	if err != nil {
		// The step could not carry the outcome, so the error has to.
		return errors.Join(err, undoErr)
	}
	return nil
}

// undoAll runs the undos newest first. It keeps going after a failure so one
// stuck undo does not strand the others.
func undoAll(undos []func() error) error {
	var undoErr error
	for index := len(undos) - 1; index >= 0; index-- {
		undoErr = errors.Join(undoErr, undos[index]())
	}
	return undoErr
}

// stepResult returns the recorded result of a step, or the zero value when
// the phases do not contain it.
func stepResult(phases []progress.Phase, phaseID progress.PhaseID, stepID progress.StepID) progress.StepResult {
	for _, phase := range phases {
		if phase.ID != phaseID {
			continue
		}
		for _, result := range phase.Steps {
			if result.ID == stepID {
				return result
			}
		}
	}
	return progress.StepResult{}
}
