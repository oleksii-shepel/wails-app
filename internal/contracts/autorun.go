package contracts

type AutorunEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Command    string `json:"command"`
	Executable string `json:"executable,omitempty"`
	SourcePath string `json:"sourcePath"`
	Scope      string `json:"scope"`
	Origin     string `json:"origin"`
	Enabled    bool   `json:"enabled"`
}
