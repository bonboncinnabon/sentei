package tui

// The voice registry: every view and portal title, declared once. Sentence
// case throughout — calm, charm-land. viewTitle adds the brand prefix;
// portal boxes render the bare title (the brand is already on screen).
const (
	titleMenu              = "Git worktree manager"
	titleList              = "Remove worktrees"
	titleConfirmDeletion   = "Confirm deletion"
	titleRemoving          = "Removing worktrees"
	titleRemovalComplete   = "Removal complete"
	titleCreateWorktree    = "Create worktree"
	titleConfirmCreate     = "Confirm create"
	titleCreatingWorktree  = "Creating worktree"
	titleWorktreeCreated   = "Worktree created"
	titleCreateRepo        = "Create repository"
	titleCloneRepo         = "Clone repository"
	titleConfirmClone      = "Confirm clone"
	titleCreatingRepo      = "Creating repository"
	titleCloningRepo       = "Cloning repository"
	titleMigratingRepo     = "Migrating repository"
	titleRepoCreated       = "Repository created"
	titleRepoCloned        = "Repository cloned"
	titleMigrate           = "Migrate to bare repository"
	titleMigrationComplete = "Migration complete"
	titleCleanupPreview    = "Cleanup preview"
	titleConfirmCleanup    = "Confirm cleanup"
	titleRunningCleanup    = "Running cleanup"
	titleCleanupComplete   = "Cleanup complete"
	titleCleanupErrors     = "Cleanup finished with errors"

	portalWorktreeDetails = "Worktree details"
	portalCleanupDetails  = "Cleanup details"
	portalProgressDetails = "Progress details"
)
