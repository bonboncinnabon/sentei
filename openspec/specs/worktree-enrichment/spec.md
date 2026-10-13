## Purpose
Covers parallel enrichment of worktrees with commit metadata and dirty/untracked status, including graceful per-worktree error handling and the public EnrichWorktrees API.

## Requirements

### Requirement: Enrich worktrees with commit metadata
The system SHALL enrich each non-bare, non-prunable worktree with its last commit date (the committer date, so a rebase counts as activity) by running `git -C <path> log -1 --format=%ci` and last commit subject by running `git -C <path> log -1 --format=%s`.

#### Scenario: Normal worktree enrichment
- **WHEN** a worktree has IsBare=false and IsPrunable=false and its directory exists
- **THEN** the system SHALL populate LastCommitDate with the parsed datetime from `git log -1 --format=%ci` and LastCommitSubject with the output of `git log -1 --format=%s`

#### Scenario: Worktree with no commits
- **WHEN** `git log -1` returns empty output for a worktree (e.g., orphan branch with no commits)
- **THEN** the system SHALL set LastCommitDate to the zero time value and LastCommitSubject to empty string, with no error

### Requirement: Enrich worktrees with dirty/untracked status
The system SHALL determine dirty and untracked file status for each non-bare, non-prunable worktree by running `git -C <path> status --porcelain --untracked-files=normal`, so untracked files are seen whatever the user's `status.showUntrackedFiles` says.

#### Scenario: Clean worktree
- **WHEN** `git status --porcelain` returns empty output
- **THEN** HasUncommittedChanges SHALL be false and HasUntrackedFiles SHALL be false

#### Scenario: Worktree with uncommitted changes
- **WHEN** `git status --porcelain` output contains lines starting with `M`, `A`, `D`, `R`, `C`, `U`, or a space followed by one of those letters (staged or unstaged modifications)
- **THEN** HasUncommittedChanges SHALL be true

#### Scenario: Worktree with untracked files only
- **WHEN** `git status --porcelain` output contains only lines starting with `??`
- **THEN** HasUncommittedChanges SHALL be false and HasUntrackedFiles SHALL be true

#### Scenario: Worktree with both uncommitted changes and untracked files
- **WHEN** `git status --porcelain` output contains both tracked-file changes and `??` lines
- **THEN** HasUncommittedChanges SHALL be true and HasUntrackedFiles SHALL be true

### Requirement: Parallel enrichment execution
The system SHALL enrich all worktrees concurrently, bounded by a configurable maximum concurrency limit (default: 10).

#### Scenario: Parallel execution within bounds
- **WHEN** enriching N worktrees with max concurrency M
- **THEN** the system SHALL run at most M enrichment operations simultaneously

#### Scenario: Performance target
- **WHEN** enriching 50 worktrees
- **THEN** the total enrichment time SHALL be less than 5 seconds

### Requirement: Skip enrichment for bare repository entries
The system SHALL skip metadata enrichment for worktrees where IsBare is true, leaving enrichment fields at zero values.

#### Scenario: Bare repo entry
- **WHEN** a worktree has IsBare=true
- **THEN** the system SHALL not execute any git commands for that worktree and SHALL set IsEnriched to false

### Requirement: Skip enrichment for prunable worktrees
The system SHALL skip metadata enrichment for worktrees where IsPrunable is true, leaving enrichment fields at zero values.

#### Scenario: Prunable worktree
- **WHEN** a worktree has IsPrunable=true
- **THEN** the system SHALL not execute any git commands for that worktree and SHALL set IsEnriched to false

### Requirement: Graceful handling of per-worktree enrichment failures
The system SHALL collect enrichment errors per-worktree without failing the entire batch. A failed worktree SHALL have its EnrichmentError field set to the error message.

#### Scenario: Single worktree enrichment fails
- **WHEN** a git command fails for one worktree during enrichment
- **THEN** the system SHALL set EnrichmentError on that worktree, leave enrichment fields at zero values, continue enriching other worktrees, and return the full slice

#### Scenario: All worktrees enrich successfully
- **WHEN** all git commands succeed for all worktrees
- **THEN** EnrichmentError SHALL be empty for every worktree and IsEnriched SHALL be true

#### Scenario: Worktree directory missing at enrichment time
- **WHEN** a worktree's directory does not exist when enrichment runs (deleted between discovery and enrichment)
- **THEN** the system SHALL set EnrichmentError to a descriptive message and not panic or abort

### Requirement: EnrichWorktrees public function
The system SHALL expose a function `EnrichWorktrees(runner CommandRunner, worktrees []Worktree, maxConcurrency int) []Worktree` that enriches all worktrees in place and returns the enriched slice.

#### Scenario: Full enrichment pipeline
- **WHEN** EnrichWorktrees is called with a slice of worktrees from ListWorktrees
- **THEN** it SHALL return the same slice with enrichment fields populated for eligible worktrees
