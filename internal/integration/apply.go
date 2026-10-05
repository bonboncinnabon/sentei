package integration

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/fileutil"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

const prerequisitesPhase progress.PhaseID = "prerequisites"

func SetupStepName(integ Integration) string          { return "Setup " + integ.Name }
func InstallStepName(integ Integration) string        { return "Install " + integ.Name }
func InstallDependencyStepName(dep Dependency) string { return "Install dependency " + dep.Name }
func TeardownStepName(integ Integration) string       { return "Teardown " + integ.Name }
func RemoveDirStepName(dir, wtPath string) string {
	return fmt.Sprintf("Remove %s in %s", strings.TrimSuffix(dir, "/"), filepath.Base(wtPath))
}

type applyOperationKind uint8

const (
	applyShellCommand applyOperationKind = iota
	applyFailure
	applyRemove
	applyCopy
	applyGitignore
)

type applyOperation struct {
	phaseID   progress.PhaseID
	phaseName string
	stepID    progress.StepID
	label     string
	kind      applyOperationKind
	dir       string
	command   string
	failure   error
	dependsOn []string

	seedSource     string
	seedDest       string
	seedSourceRoot string
	seedSourcePath string
	seedDestRoot   string
	seedDestPath   string
	managedRoot    string
	managedPath    string
	gitignore      []string
	gitignoreDir   string
}

func (op applyOperation) key() string { return op.phaseID + "\x00" + op.stepID }

// PreparedApply is an exact progress declaration plus the frozen operations
// that implement it. Run performs no detection.
type PreparedApply struct {
	plan       progress.Plan
	operations []applyOperation
	files      applyFileOperations
}

type applyFileOperations interface {
	removeAll(path string) error
	copyDir(source, destination string) error
	appendGitignore(dir string, entries []string) error
}

type realApplyFileOperations struct{}

func (realApplyFileOperations) removeAll(path string) error { return os.RemoveAll(path) }
func (realApplyFileOperations) copyDir(source, destination string) error {
	return fileutil.CopyDir(source, destination)
}
func (realApplyFileOperations) appendGitignore(dir string, entries []string) error {
	return appendGitignoreEntries(dir, entries)
}

// Plan returns a defensive copy of the frozen progress declaration.
func (p PreparedApply) Plan() progress.Plan { return p.plan.Clone() }

// Empty reports whether preparation froze no work to execute.
func (p PreparedApply) Empty() bool { return len(p.operations) == 0 }

// PrepareApply performs the read-only detection pass once in the first target
// worktree, then freezes both the declaration and every execution decision.
func PrepareApply(shell git.ShellRunner, repoPath, mainWT string, toEnable, toDisable []Integration, wtPaths []string) (PreparedApply, error) {
	probeDir := ""
	if len(wtPaths) > 0 {
		probeDir = wtPaths[0]
	}
	return PrepareApplyForTarget(shell, repoPath, mainWT, probeDir, toEnable, toDisable, wtPaths)
}

// PrepareApplyForTarget separates the existing worktree used for read-only
// availability probes from future target worktrees. This lets callers freeze
// an apply plan before any target is created.
func PrepareApplyForTarget(shell git.ShellRunner, repoPath, mainWT, probeDir string, toEnable, toDisable []Integration, wtPaths []string) (PreparedApply, error) {
	if (len(toEnable) > 0 || len(toDisable) > 0) && len(wtPaths) == 0 {
		return PreparedApply{}, errors.New("preparing integrations: no target worktree")
	}
	if err := validateApplyInputs(toEnable, toDisable); err != nil {
		return PreparedApply{}, fmt.Errorf("preparing integrations: %w", err)
	}
	if err := validateManagedPaths(mainWT, toEnable, toDisable, wtPaths); err != nil {
		return PreparedApply{}, fmt.Errorf("preparing integrations: %w", err)
	}
	type enabledDecision struct {
		integration Integration
		key         string
		installed   bool
		toolOpKey   string
	}
	decisions := make([]enabledDecision, 0, len(toEnable))
	dependencyPresent := map[string]bool{}
	dependencySpecs := map[string]Dependency{}
	dependencyOp := map[string]applyOperation{}
	var prerequisites []applyOperation

	for _, integ := range toEnable {
		decision := enabledDecision{integration: integ, key: integ.Name}
		decision.installed = detectForApply(shell, probeDir, integ.Detect)
		for _, dep := range integ.Dependencies {
			if _, checked := dependencyPresent[dep.Name]; checked {
				continue
			}
			_, err := shell.RunShell(probeDir, dep.Detect)
			dependencyPresent[dep.Name] = err == nil
			dependencySpecs[dep.Name] = dep
		}
		decisions = append(decisions, decision)
	}

	for _, decision := range decisions {
		if decision.installed {
			continue
		}
		for _, dependency := range decision.integration.Dependencies {
			if dependencyPresent[dependency.Name] {
				continue
			}
			if _, planned := dependencyOp[dependency.Name]; planned {
				continue
			}
			dep := dependencySpecs[dependency.Name]
			op := applyOperation{
				phaseID: prerequisitesPhase, phaseName: "Prerequisites",
				stepID: stableStepID("dependency", dep.Name), label: InstallDependencyStepName(dep),
				dir: probeDir,
			}
			if dep.Install == "" {
				op.kind = applyFailure
				op.failure = fmt.Errorf("dependency %s is missing and has no install command", dep.Name)
			} else {
				op.kind = applyShellCommand
				op.command = dep.Install
			}
			dependencyOp[dep.Name] = op
			prerequisites = append(prerequisites, op)
		}
	}

	for i := range decisions {
		decision := &decisions[i]
		if decision.installed {
			continue
		}
		op := applyOperation{
			phaseID: prerequisitesPhase, phaseName: "Prerequisites",
			stepID: stableStepID("install", decision.key), label: InstallStepName(decision.integration),
			kind: applyShellCommand, dir: probeDir, command: decision.integration.Install.Command,
		}
		if op.command == "" {
			op.kind = applyFailure
			op.failure = fmt.Errorf("%s is missing and has no install command", decision.integration.Name)
		}
		for _, dep := range decision.integration.Dependencies {
			if depOp, missing := dependencyOp[dep.Name]; missing {
				op.dependsOn = append(op.dependsOn, depOp.key())
			}
		}
		decision.toolOpKey = op.key()
		prerequisites = append(prerequisites, op)
	}

	operations := append([]applyOperation(nil), prerequisites...)
	for _, wtPath := range wtPaths {
		phaseID := progress.PhaseID("worktree:" + stableToken(normalizeWorkspaceIdentity(wtPath)))
		for _, decision := range decisions {
			integ := decision.integration
			dependencies := []string(nil)
			if decision.toolOpKey != "" {
				dependencies = append(dependencies, decision.toolOpKey)
			}
			if integ.IndexCopyDir != "" && mainWT != "" && normalizeWorkspaceIdentity(wtPath) != normalizeWorkspaceIdentity(mainWT) {
				source, err := ResolveManagedPath(mainWT, integ.IndexCopyDir)
				if err != nil {
					return PreparedApply{}, fmt.Errorf("preparing integrations: index source: %w", err)
				}
				destination, err := ResolveManagedPath(wtPath, integ.IndexCopyDir)
				if err != nil {
					return PreparedApply{}, fmt.Errorf("preparing integrations: index destination: %w", err)
				}
				info, err := os.Stat(source)
				switch {
				case err == nil && !info.IsDir():
					return PreparedApply{}, fmt.Errorf("preparing integrations: index source %q is not a directory", source)
				case err == nil:
					copyOp := applyOperation{
						phaseID: phaseID, phaseName: wtPath,
						stepID: stableStepID("copy-index", integ.Name), label: "Copy index for " + integ.Name,
						kind: applyCopy, dependsOn: append([]string(nil), dependencies...),
						seedSource: source, seedDest: destination,
						seedSourceRoot: mainWT, seedSourcePath: integ.IndexCopyDir,
						seedDestRoot: wtPath, seedDestPath: integ.IndexCopyDir,
					}
					operations = append(operations, copyOp)
					dependencies = append(dependencies, copyOp.key())
				case os.IsNotExist(err):
				case err != nil:
					return PreparedApply{}, fmt.Errorf("preparing integrations: inspecting index source %q: %w", source, err)
				}
			}
			if strings.TrimSpace(integ.Setup.Command) != "" {
				workDir := wtPath
				if integ.Setup.WorkingDir == "repo" {
					workDir = repoPath
				}
				setupOp := applyOperation{
					phaseID: phaseID, phaseName: wtPath,
					stepID: stableStepID("setup", decision.key), label: SetupStepName(integ),
					kind: applyShellCommand, dir: workDir,
					command:   strings.ReplaceAll(integ.Setup.Command, "{path}", git.ShellQuote(wtPath)),
					dependsOn: append([]string(nil), dependencies...),
				}
				operations = append(operations, setupOp)
				dependencies = append(dependencies, setupOp.key())
			}
			if len(integ.GitignoreEntries) > 0 {
				operations = append(operations, applyOperation{
					phaseID: phaseID, phaseName: wtPath,
					stepID: stableStepID("gitignore", integ.Name), label: "Update .gitignore for " + integ.Name,
					kind: applyGitignore, dependsOn: append([]string(nil), dependencies...),
					gitignore: append([]string(nil), integ.GitignoreEntries...), gitignoreDir: wtPath,
				})
			}
		}
		for _, integ := range toDisable {
			key := integ.Name
			if integ.Teardown.Command != "" {
				operations = append(operations, applyOperation{
					phaseID: phaseID, phaseName: wtPath,
					stepID: stableStepID("teardown", key), label: TeardownStepName(integ),
					kind: applyShellCommand, dir: wtPath, command: integ.Teardown.Command,
				})
			}
			for _, dir := range integ.Teardown.Dirs {
				managedDir, err := ResolveManagedPath(wtPath, dir)
				if err != nil {
					return PreparedApply{}, fmt.Errorf("preparing integrations: teardown directory: %w", err)
				}
				operations = append(operations, applyOperation{
					phaseID: phaseID, phaseName: wtPath,
					stepID: stableStepID("remove", key+":"+normalizeWorkspaceIdentity(dir)), label: RemoveDirStepName(dir, wtPath),
					kind: applyRemove, dir: managedDir, managedRoot: wtPath, managedPath: dir,
				})
			}
		}
	}

	if err := validateOperationGraph(operations); err != nil {
		return PreparedApply{}, fmt.Errorf("preparing integrations: %w", err)
	}
	return PreparedApply{plan: planForOperations(operations), operations: operations, files: realApplyFileOperations{}}, nil
}

func validateManagedPaths(mainWT string, toEnable, toDisable []Integration, wtPaths []string) error {
	for _, integ := range toEnable {
		if integ.IndexCopyDir == "" {
			continue
		}
		if mainWT != "" {
			if _, err := ResolveManagedPath(mainWT, integ.IndexCopyDir); err != nil {
				return fmt.Errorf("integration %q index copy directory: %w", integ.Name, err)
			}
		}
		for _, wtPath := range wtPaths {
			if _, err := ResolveManagedPath(wtPath, integ.IndexCopyDir); err != nil {
				return fmt.Errorf("integration %q index copy directory: %w", integ.Name, err)
			}
		}
	}
	for _, integ := range toDisable {
		for _, dir := range integ.Teardown.Dirs {
			for _, wtPath := range wtPaths {
				if _, err := ResolveManagedPath(wtPath, dir); err != nil {
					return fmt.Errorf("integration %q teardown directory: %w", integ.Name, err)
				}
			}
		}
	}
	return nil
}

// ResolveManagedPath validates a declared integration path and resolves it
// beneath root. A single trailing separator is accepted for directory specs.
func ResolveManagedPath(root, managedPath string) (string, error) {
	if managedPath == "" || strings.TrimSpace(managedPath) != managedPath {
		return "", fmt.Errorf("managed path %q must be nonempty and have no surrounding whitespace", managedPath)
	}
	if filepath.IsAbs(managedPath) {
		return "", fmt.Errorf("managed path %q must be relative", managedPath)
	}
	path := strings.TrimSuffix(managedPath, string(filepath.Separator))
	if path == "" || filepath.Clean(path) != path || path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("managed path %q must be clean and below its root", managedPath)
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving managed path root %q: %w", root, err)
	}
	rootPath, err = resolveExistingPath(rootPath)
	if err != nil {
		return "", fmt.Errorf("resolving managed path root %q: %w", root, err)
	}
	resolved := rootPath
	parts := strings.Split(path, string(filepath.Separator))
components:
	for i, part := range parts {
		candidate := filepath.Join(resolved, part)
		info, err := os.Lstat(candidate)
		switch {
		case err == nil && info.Mode()&os.ModeSymlink != 0:
			return "", fmt.Errorf("managed path %q contains symlink component %q", managedPath, strings.Join(parts[:i+1], string(filepath.Separator)))
		case err == nil:
			resolved = candidate
		case os.IsNotExist(err):
			resolved = filepath.Join(append([]string{resolved}, parts[i:]...)...)
			break components
		case err != nil:
			return "", fmt.Errorf("inspecting managed path %q: %w", managedPath, err)
		}
	}
	relative, err := filepath.Rel(rootPath, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("managed path %q resolves outside root %q", managedPath, root)
	}
	return resolved, nil
}

func resolveExistingPath(path string) (string, error) {
	current := path
	var missing []string
	for {
		_, err := os.Lstat(current)
		switch {
		case err == nil:
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		case !os.IsNotExist(err):
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

// BindPhase returns a copy whose operations share one caller-owned phase.
// Operation identities and dependency edges remain semantic and unchanged
// apart from their phase-qualified keys.
func (p PreparedApply) BindPhase(phaseID progress.PhaseID, phaseLabel string) (PreparedApply, error) {
	if phaseID == "" {
		return PreparedApply{}, errors.New("binding integration apply: empty phase ID")
	}
	operations := append([]applyOperation(nil), p.operations...)
	keys := make(map[string]string, len(operations))
	for i := range operations {
		operations[i].dependsOn = append([]string(nil), operations[i].dependsOn...)
		oldKey := operations[i].key()
		operations[i].phaseID = phaseID
		operations[i].phaseName = phaseLabel
		keys[oldKey] = operations[i].key()
	}
	for i := range operations {
		for j, dependency := range operations[i].dependsOn {
			operations[i].dependsOn[j] = keys[dependency]
		}
	}
	if err := validateOperationGraph(operations); err != nil {
		return PreparedApply{}, fmt.Errorf("binding integration apply: %w", err)
	}
	return PreparedApply{plan: planForOperations(operations), operations: operations, files: p.files}, nil
}

func validateApplyInputs(toEnable, toDisable []Integration) error {
	identities := make(map[string]string, len(toEnable)+len(toDisable))
	dependencies := make(map[string]Dependency)
	for _, group := range []struct {
		name         string
		integrations []Integration
	}{{"enable", toEnable}, {"disable", toDisable}} {
		for _, integ := range group.integrations {
			name := strings.TrimSpace(integ.Name)
			if name == "" {
				return errors.New("integration has empty name")
			}
			if prior, exists := identities[name]; exists {
				return fmt.Errorf("duplicate integration identity %q in %s and %s", name, prior, group.name)
			}
			identities[name] = group.name
			if group.name == "enable" {
				hasCommand := strings.TrimSpace(integ.Detect.Command) != ""
				hasBinary := strings.TrimSpace(integ.Detect.BinaryName) != ""
				if !hasCommand && !hasBinary {
					return fmt.Errorf("integration %q must declare a detection command or binary", name)
				}
			}
			for _, dep := range integ.Dependencies {
				depName := strings.TrimSpace(dep.Name)
				if depName == "" {
					return fmt.Errorf("integration %q has dependency with empty name", name)
				}
				if strings.TrimSpace(dep.Detect) == "" {
					return fmt.Errorf("dependency %q has empty detection command", depName)
				}
				if prior, exists := dependencies[depName]; exists && (prior.Detect != dep.Detect || prior.Install != dep.Install) {
					return fmt.Errorf("dependency %q has conflicting specifications", depName)
				}
				dependencies[depName] = dep
			}
		}
	}
	return nil
}

func normalizeWorkspaceIdentity(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func validateOperationGraph(operations []applyOperation) error {
	all := make(map[string]bool, len(operations))
	for _, op := range operations {
		key := op.key()
		if all[key] {
			return fmt.Errorf("duplicate operation %q", key)
		}
		all[key] = true
	}
	completed := make(map[string]bool, len(operations))
	for _, op := range operations {
		for _, dependency := range op.dependsOn {
			if !all[dependency] {
				return fmt.Errorf("operation %q references missing dependency %q", op.key(), dependency)
			}
			if !completed[dependency] {
				return fmt.Errorf("operation %q depends on out-of-order or cyclic operation %q", op.key(), dependency)
			}
		}
		completed[op.key()] = true
	}
	return nil
}

func (p PreparedApply) Run(shell git.ShellRunner, emit func(progress.Event)) ([]progress.Phase, error) {
	if err := validateOperationGraph(p.operations); err != nil {
		return nil, fmt.Errorf("validating integration apply: %w", err)
	}
	execution, err := progress.Start(p.plan, emit)
	if err != nil {
		return nil, fmt.Errorf("starting integration apply: %w", err)
	}
	runErr := p.RunIn(execution, shell)
	finishErr := execution.Finish("integration apply finished")
	if finishErr != nil {
		finishErr = fmt.Errorf("finishing integration apply: %w", finishErr)
	}
	return execution.Phases(), errors.Join(runErr, finishErr)
}

// RunIn executes the frozen operations through an already-started shared
// execution. It does not start or finish that execution.
func (p PreparedApply) RunIn(execution *progress.Execution, shell git.ShellRunner) error {
	if execution == nil {
		return errors.New("executing integration apply: nil execution")
	}
	if err := validateOperationGraph(p.operations); err != nil {
		return fmt.Errorf("validating integration apply: %w", err)
	}
	results := make(map[string]progress.StepResult, len(p.operations))
	files := p.files
	if files == nil {
		files = realApplyFileOperations{}
	}
	for _, op := range p.operations {
		blockedBy := ""
		for _, dependency := range op.dependsOn {
			result := results[dependency]
			if result.Status == progress.StepFailed || result.Status == progress.StepSkipped {
				blockedBy = result.Name
				break
			}
		}
		if blockedBy != "" {
			result, err := execution.Skip(op.phaseID, op.stepID, "blocked by "+blockedBy)
			if err != nil {
				return fmt.Errorf("skipping %s: %w", op.label, err)
			}
			results[op.key()] = result
			continue
		}
		var result progress.StepResult
		var transitionErr error
		switch op.kind {
		case applyFailure:
			result, transitionErr = execution.Fail(op.phaseID, op.stepID, op.failure)
		case applyRemove:
			result, transitionErr = execution.Run(op.phaseID, op.stepID, func() (string, error) {
				path := op.dir
				if op.managedPath != "" {
					resolved, err := ResolveManagedPath(op.managedRoot, op.managedPath)
					if err != nil {
						return "", fmt.Errorf("revalidating removal path: %w", err)
					}
					path = resolved
				}
				return "", files.removeAll(path)
			})
		case applyCopy:
			result, transitionErr = execution.Run(op.phaseID, op.stepID, func() (string, error) {
				source, destination := op.seedSource, op.seedDest
				if op.seedSourcePath != "" {
					resolved, err := ResolveManagedPath(op.seedSourceRoot, op.seedSourcePath)
					if err != nil {
						return "", fmt.Errorf("revalidating index source: %w", err)
					}
					source = resolved
				}
				if op.seedDestPath != "" {
					resolved, err := ResolveManagedPath(op.seedDestRoot, op.seedDestPath)
					if err != nil {
						return "", fmt.Errorf("revalidating index destination: %w", err)
					}
					destination = resolved
				}
				if err := files.removeAll(destination); err != nil {
					return "", fmt.Errorf("removing existing index: %w", err)
				}
				if err := files.copyDir(source, destination); err != nil {
					return "", fmt.Errorf("copying index: %w", err)
				}
				return "", nil
			})
		case applyGitignore:
			result, transitionErr = execution.Run(op.phaseID, op.stepID, func() (string, error) {
				return "", files.appendGitignore(op.gitignoreDir, op.gitignore)
			})
		default:
			result, transitionErr = execution.Run(op.phaseID, op.stepID, func() (string, error) { return shell.RunShell(op.dir, op.command) })
		}
		if transitionErr != nil {
			return fmt.Errorf("executing %s: %w", op.label, transitionErr)
		}
		results[op.key()] = result
	}
	return nil
}

func detectForApply(shell git.ShellRunner, dir string, detect DetectSpec) bool {
	if strings.TrimSpace(detect.Command) != "" {
		if _, err := shell.RunShell(dir, detect.Command); err == nil {
			return true
		}
	}
	if strings.TrimSpace(detect.BinaryName) != "" {
		if _, err := shell.RunShell(dir, "command -v "+detect.BinaryName); err == nil {
			return true
		}
	}
	return false
}

func stableToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:8])
}

func stableStepID(kind, value string) progress.StepID {
	return progress.StepID(kind + ":" + stableToken(value))
}

func planForOperations(operations []applyOperation) progress.Plan {
	indices := map[progress.PhaseID]int{}
	var plan progress.Plan
	for _, op := range operations {
		index, exists := indices[op.phaseID]
		if !exists {
			index = len(plan.Phases)
			indices[op.phaseID] = index
			plan.Phases = append(plan.Phases, progress.PlannedPhase{ID: op.phaseID, Label: op.phaseName})
		}
		plan.Phases[index].Steps = append(plan.Phases[index].Steps, progress.PlannedStep{ID: op.stepID, Label: op.label})
	}
	return plan
}
