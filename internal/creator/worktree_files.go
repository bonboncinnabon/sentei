package creator

import (
	"fmt"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktreefile"
)

func worktreeFileIdentity(rules []worktreefile.Rule) string {
	var identity strings.Builder
	for _, rule := range rules {
		fmt.Fprintf(&identity, "%s\x00%s\x00%t\x00", rule.Source, rule.Destination, rule.Overwrite)
	}
	return identity.String()
}

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
