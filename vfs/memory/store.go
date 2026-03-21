package memory

import (
	"os"
	"path/filepath"
)

type Store struct {
	rootPath  string
	statePath string
	dataPath  string
}

func NewStore(appName string) (*Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}

	dir := filepath.Join(base, appName, "memoryfs")
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0755); err != nil {
		return nil, err
	}

	return &Store{
		rootPath:  dir,
		statePath: filepath.Join(dir, "state.json"),
		dataPath:  filepath.Join(dir, "data"),
	}, nil
}

func (s *Store) ReadState() ([]byte, error) {
	return os.ReadFile(s.statePath)
}

func (s *Store) WriteState(data []byte) error {
	return os.WriteFile(s.statePath, data, 0644)
}

func (s *Store) DataPath() string {
	return s.dataPath
}

func (s *Store) RootPath() string {
	return s.rootPath
}
