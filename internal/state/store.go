// Package state handles persistence of CommitForge session state to JSON.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	// ErrNotFound is returned when no state file exists yet.
	ErrNotFound = errors.New("state file not found")
	// ErrCorrupted is returned when the state file exists but is invalid JSON.
	ErrCorrupted = errors.New("state file corrupted")
)

const (
	stateDirName  = ".commitforge"
	stateFileName = "state.json"
)

// FilePath returns <dir>/.commitforge/state.json.
func FilePath(dir string) string {
	return filepath.Join(dir, stateDirName, stateFileName)
}

// Save writes state to <dir>/.commitforge/state.json, creating parent dirs as needed.
func Save(dir string, s PersistedState) error {
	if dir == "" {
		return fmt.Errorf("save state: empty dir")
	}
	path := FilePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("save state: mkdir: %w", err)
	}
	if s.Version == 0 {
		s.Version = 1
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("save state: marshal: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("save state: write: %w", err)
	}
	return nil
}

// Load reads and parses <dir>/.commitforge/state.json.
func Load(dir string) (PersistedState, error) {
	if dir == "" {
		return PersistedState{}, fmt.Errorf("load state: empty dir")
	}
	path := FilePath(dir)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PersistedState{}, ErrNotFound
		}
		return PersistedState{}, fmt.Errorf("load state: read: %w", err)
	}
	var s PersistedState
	if err := json.Unmarshal(b, &s); err != nil {
		return PersistedState{}, fmt.Errorf("%w: %v", ErrCorrupted, err)
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.DateCounts == nil {
		s.DateCounts = map[string]int{}
	}
	if s.GeneratedDateCounts == nil {
		s.GeneratedDateCounts = map[string]int{}
	}
	return s, nil
}
