package creator

import (
	"fmt"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktreefile"
)

func (p preparedCreation) runWorktreeFileCopy(execution *progress.Execution) error {
	_, err := execution.Run(setupPhaseID, p.worktreeFileStepID, func() (string, error) {
		outcome, err := worktreefile.Copy(p.opts.RepoPath, p.worktreePath, p.opts.WorktreeFiles)
		return outcome.Message(), err
	})
	if err != nil {
		return fmt.Errorf("executing worktree file copy: %w", err)
	}
	return nil
}
