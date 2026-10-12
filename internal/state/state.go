package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const stateFile = "sentei.json"

// State holds persistent data for a bare repository. Unknown keys in an
// existing file (such as the retired "integrations" list) are ignored on load
// and dropped on the next save.
type State struct {
	// LifetimeRemoved counts every worktree this repository has pruned
	// through the TUI, across all sessions. Garnish, not bookkeeping:
	// it exists so summaries can whisper at milestones.
	LifetimeRemoved int `json:"lifetime_removed,omitempty"`
}

// Load reads the state from bareDir/sentei.json. If the file does not exist,
// an empty State is returned. If the file exists but contains invalid JSON,
// an error is returned.
func Load(bareDir string) (*State, error) {
	path := filepath.Join(bareDir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{}, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	return &s, nil
}

// Save writes s to bareDir/sentei.json atomically using a temp file + rename.
func Save(bareDir string, s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding state: %w", err)
	}
	data = append(data, '\n')

	target := filepath.Join(bareDir, stateFile)
	tmp, err := os.CreateTemp(bareDir, ".sentei-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing state: %w", err)
	}
	// Flush to disk before the rename so a crash can't leave a renamed-but-empty
	// file (the rename is only atomic against concurrent readers, not power loss).
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("syncing state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming state file: %w", err)
	}
	return nil
}
