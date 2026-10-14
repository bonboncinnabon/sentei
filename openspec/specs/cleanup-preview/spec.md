# cleanup-preview Specification

## Purpose
TBD - created by archiving change cleanup-preview-redesign. Update Purpose after archive.
## Requirements
### Requirement: Dry-run scan with loading state
The system SHALL run a dry-run cleanup scan when the user selects cleanup from the TUI menu, displaying a loading indicator while scanning.

#### Scenario: Loading state during scan
- **WHEN** the user selects cleanup from the TUI menu
- **THEN** the view SHALL display `viewTitle("Cleanup Preview")`, a separator, and `◐ Scanning repository…` using the active indicator

#### Scenario: Scan completes and shows preview
- **WHEN** the dry-run scan completes
- **THEN** the loading state SHALL transition to the preview with at least `MinProgressDisplay` elapsed (animation buffer)

#### Scenario: Scan with nothing to clean
- **WHEN** the dry-run scan finds nothing for safe or aggressive mode
- **THEN** the view SHALL display `● Repository is clean` with `enter back · q quit` key hints

### Requirement: Cleanup preview displays safe results
The system SHALL display the results of a safe-mode dry-run grouped by category with counts.

#### Scenario: Safe results with actions
- **WHEN** the scan finds 3 stale refs, 1 orphaned config section, and 1 stale worktree
- **THEN** the preview SHALL display under "Safe cleanup:" each category with `●` indicator and count

#### Scenario: Safe results with no-ops
- **WHEN** the scan finds no stale refs
- **THEN** the preview SHALL display `· No stale remote refs` using the pending indicator

#### Scenario: Gone-upstream branches are named
- **WHEN** the scan finds branches whose upstream is gone
- **THEN** the preview SHALL name them under their count, up to four rows; with more, three names and "and N more", and `?` SHALL open the full list

### Requirement: Aggressive upgrade offer
The system SHALL display an aggressive cleanup section when the dry-run detects items that aggressive mode would clean beyond safe mode.

#### Scenario: Aggressive branches available
- **WHEN** the scan finds 5 merged branches not in any worktree
- **THEN** the preview SHALL display "Aggressive cleanup available:" with a count, the first 2-3 branch names inline, and "and N more — ? for details" if more exist

#### Scenario: Inline preview with few branches
- **WHEN** the scan finds 2 merged branches
- **THEN** the preview SHALL display both branch names inline with no "and N more" text

#### Scenario: No aggressive items
- **WHEN** the scan finds nothing additional for aggressive mode
- **THEN** the aggressive section SHALL NOT appear, and the `a` key hint SHALL NOT be shown

#### Scenario: Key hints with aggressive available
- **WHEN** aggressive cleanup items are available
- **THEN** the key hints SHALL show `enter run safe · a run aggressive · ? details · esc back`

#### Scenario: Key hints without aggressive
- **WHEN** no aggressive items are available
- **THEN** the key hints SHALL show `enter run safe · esc back`

### Requirement: Detail portal for cleanup branches
The user SHALL be able to press `?` from the cleanup preview to open a scrollable detail portal showing the branch lists the preview shortens: the gone-upstream branches safe cleanup deletes, when the preview does not name them all, and the branches aggressive mode would delete, with metadata.

#### Scenario: Open details
- **WHEN** the user presses `?` on the cleanup preview with aggressive items available
- **THEN** the detail portal SHALL open with title "Cleanup details" and a scrollable list of branch names with merge date and last commit subject

#### Scenario: No details when nothing is shortened
- **WHEN** the user presses `?` on the cleanup preview with no aggressive items and every gone-upstream branch named
- **THEN** nothing SHALL happen

### Requirement: Execute cleanup from preview
The user SHALL be able to execute safe or aggressive cleanup directly from the preview.

#### Scenario: Run safe cleanup
- **WHEN** the user presses Enter on the cleanup preview
- **THEN** the system SHALL execute cleanup in safe mode and transition to the cleanup result view

#### Scenario: Run aggressive cleanup
- **WHEN** the user presses `a` on the cleanup preview (aggressive items available)
- **THEN** the system SHALL execute cleanup in aggressive mode and transition to the cleanup result view

#### Scenario: Aggressive cleanup requires confirmation
- **WHEN** the user presses `a` on the cleanup preview
- **THEN** the system SHALL show a brief confirmation: "Delete N branches? y/n" before proceeding

### Requirement: Dry-run API
The cleanup package SHALL expose a `DryRun(runner, repoPath) DryRunResult` function returning structured results for both safe and aggressive modes from a single scan.

#### Scenario: DryRunResult structure
- **WHEN** `DryRun` is called
- **THEN** the result SHALL contain: `StaleRefs int`, `ConfigDuplicates int`, `GoneBranches []BranchInfo`, `OrphanedConfigs int`, `StaleWorktrees int`, `NonWtBranches []BranchInfo` (branches aggressive would delete), each `BranchInfo` containing Name, MergeDate, and LastCommitSubject

#### Scenario: Empty repository
- **WHEN** `DryRun` is called on a clean repository
- **THEN** all counts SHALL be zero and all slices SHALL be empty


### Requirement: Aggressive preview states effective counts
The aggressive section's headline SHALL state how many branches aggressive mode would actually delete: the full count when all candidates are deletable, `N of M` when some are unmerged, and an explicit "none deletable without --force" form when zero. The `a` affordance (footer hint and confirm gate) SHALL exist only when the effective count is positive, and the clean preview SHALL list the safe-cleanup checks so coverage is visible before running.

#### Scenario: Mixed candidates
- **WHEN** three candidates exist and two are unmerged
- **THEN** the headline SHALL read `1 of 3 branches` would be deleted and the confirm prompt SHALL say `Delete 1 branch? (2 unmerged will be skipped)`

#### Scenario: Nothing deletable
- **WHEN** every candidate is unmerged
- **THEN** the headline SHALL state none are deletable without --force, the `a` hint SHALL be absent, and pressing `a` SHALL NOT open a confirm prompt

#### Scenario: Clean repository teaches coverage
- **WHEN** the scan finds nothing
- **THEN** the preview SHALL list each safe-cleanup check as a dim no-op line
