// Package progress is the single vocabulary for multi-phase operation
// progress (worktree removal, worktree creation, repo
// create/clone/migrate, cleanup): step statuses, per-step
// results grouped into phases, the events emitted while a phase runs, and
// the fold from an event stream to per-phase display state.
package progress

// PhaseID and StepID are stable identities. Labels are presentation and may
// be repeated or changed without changing the work they identify.
//
// These remain aliases temporarily so legacy producers can populate Event
// fields from strings while they migrate to Execution.
type PhaseID = string
type StepID = string

type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepDone
	StepFailed
	StepSkipped
)

// StepResult is the recorded outcome of a single step within a phase.
type StepResult struct {
	ID      StepID
	Name    string
	Status  StepStatus
	Message string
	Error   error
}

// Phase is an identified, named group of step results.
type Phase struct {
	ID    PhaseID
	Name  string
	Steps []StepResult
}

// Event is a progress notification emitted while a flow runs.
//
// Declaration rides the same stream: a Pending event with Of set declares a
// step and its checkpoint count upfront; an event with Close set marks the
// phase as complete-in-plan (no more steps will be added). A Running event
// with Checkpoint set reports intra-step progress ("reached k of Of").
type Event struct {
	Phase      PhaseID
	PhaseLabel string
	Step       StepID
	StepLabel  string
	Status     StepStatus
	Checkpoint int  // reached checkpoint (1-based) within the step, on Running events
	Of         int  // the step's declared checkpoint count (>= 1 when set)
	Close      bool // phase-close marker: the phase's step set is final
	Message    string
	Error      error
}

// HasFailures reports whether any step in the phase failed.
func (p *Phase) HasFailures() bool {
	for _, s := range p.Steps {
		if s.Status == StepFailed {
			return true
		}
	}
	return false
}

// PhasesHaveFailures reports whether any step across the phases failed.
func PhasesHaveFailures(phases []Phase) bool {
	for i := range phases {
		if phases[i].HasFailures() {
			return true
		}
	}
	return false
}

// FirstFailure returns the first failed step across the phases, along with
// the name of the phase it belongs to.
func FirstFailure(phases []Phase) (phaseName string, step StepResult, ok bool) {
	for _, p := range phases {
		for _, s := range p.Steps {
			if s.Status == StepFailed {
				return p.Name, s, true
			}
		}
	}
	return "", StepResult{}, false
}
