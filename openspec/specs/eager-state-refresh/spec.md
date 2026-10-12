## Purpose
Covers the eager worktree state refresh mechanism: global message handling, generation token guards, and eager reload on mutation success.

## Requirements

### Requirement: Global worktree context message handling
The `Update()` function SHALL handle `worktreeContextMsg` globally before view-specific dispatch, so that worktree data is applied regardless of which view is active when the async response arrives.

#### Scenario: Response arrives during summary view
- **WHEN** `worktreeContextMsg` arrives while `m.view == summaryView`
- **THEN** the handler SHALL apply the refreshed worktrees to `m.remove.worktrees`, call `reindex()` and `updateMenuHints()`, and return without forwarding the message to the summary handler

#### Scenario: Response arrives during menu view
- **WHEN** `worktreeContextMsg` arrives while `m.view == menuView`
- **THEN** the handler SHALL apply the refreshed worktrees identically to any other view, and the menu SHALL render with the updated count immediately

#### Scenario: Response arrives during progress view
- **WHEN** `holdOrAdvance` returns a tick (min progress duration not yet elapsed) and the reload completes before the tick fires
- **THEN** the handler SHALL apply the refreshed worktrees while still on `progressView`, so data is already fresh when the tick transitions to the summary view

#### Scenario: Response contains an error
- **WHEN** `worktreeContextMsg` arrives with a non-nil error
- **THEN** the handler SHALL discard the message without modifying model state

### Requirement: Generation token guards against stale responses
The model SHALL maintain a `worktreeGeneration` counter (uint64) that is incremented before each `loadWorktreeContext` call. The counter value SHALL be passed into the command and included in the resulting `worktreeContextMsg`. The global handler SHALL only apply a response whose generation matches the model's current generation.

#### Scenario: Sequential reloads with matching generation
- **GIVEN** `worktreeGeneration` is 3
- **WHEN** a `worktreeContextMsg` arrives with generation 3
- **THEN** the handler SHALL apply the refreshed worktrees

#### Scenario: Stale response with outdated generation
- **GIVEN** `worktreeGeneration` is 4 (incremented by a second reload)
- **WHEN** a `worktreeContextMsg` arrives with generation 3 (from the first reload)
- **THEN** the handler SHALL discard the message without modifying model state

#### Scenario: Init reload uses generation token
- **WHEN** the app starts and `Init()` fires `loadWorktreeContext`
- **THEN** `Init()` SHALL increment `worktreeGeneration` and pass it to the command

### Requirement: Eager reload on mutation success
Mutation completion sites that return to the menu SHALL fire `loadWorktreeContext` immediately via `tea.Batch` alongside the `holdOrAdvance` command, instead of setting a deferred flag.

#### Scenario: Worktree removal completes
- **WHEN** `cleanupCompleteMsg` is received in the progress handler
- **THEN** the handler SHALL increment `worktreeGeneration` and return `tea.Batch(holdCmd, loadWorktreeContext(runner, repoPath, generation))`

#### Scenario: Worktree creation completes
- **WHEN** `createCompleteMsg` is received in the create progress handler
- **THEN** the handler SHALL increment `worktreeGeneration` and return `tea.Batch(holdCmd, loadWorktreeContext(runner, repoPath, generation))`

#### Scenario: Mutation in tea.Quit flow does not reload
- **WHEN** `repoDoneMsg` is received in repo progress (flow exits with tea.Quit)
- **THEN** the handler SHALL NOT fire `loadWorktreeContext`

#### Scenario: Standalone cleanup does not reload
- **WHEN** `standaloneCleanupDoneMsg` is received (flow exits with tea.Quit)
- **THEN** the handler SHALL NOT fire `loadWorktreeContext`

### Requirement: Remove stateStale mechanism
The `stateStale` field, the lazy-reload gate in `updateMenu`, and the view-specific `worktreeContextMsg` case in `updateMenu` SHALL be removed entirely.

#### Scenario: No swallowed keypress after mutation
- **WHEN** the user returns to the menu after a removal and presses j/k
- **THEN** the keypress SHALL be processed immediately as cursor movement, not consumed by a reload gate

#### Scenario: Menu count already updated on return
- **WHEN** the user presses Enter on the summary screen to return to the menu after removing 3 of 7 worktrees
- **THEN** the menu SHALL display "4 available" (assuming the eager reload completed during the summary screen)
