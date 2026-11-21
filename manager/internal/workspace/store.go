package workspace

import (
	"os"
	"path/filepath"
)

// Store owns byte-level persistence for workspace state without depending on
// app wire types or Wails-facing contracts.
type Store struct {
	statePath string
}

func NewStore(appName string) (*Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}

	dir := filepath.Join(base, appName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &Store{statePath: filepath.Join(dir, "state.json")}, nil
}

func (s *Store) ReadState() ([]byte, error) {
	return os.ReadFile(s.statePath)
}

func (s *Store) WriteState(data []byte) error {
	// Write through a temp file so a crash mid-write cannot corrupt the
	// persisted state; a direct os.WriteFile would truncate first.
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.statePath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Store) StatePath() string {
	return s.statePath
}