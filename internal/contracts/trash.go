package contracts

type TrashEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	OriginalPath string `json:"originalPath"`
	TrashedPath  string `json:"trashedPath"`
	DeletedAt    string `json:"deletedAt"`
	Size         int64  `json:"size"`
	IsDir        bool   `json:"isDir"`
	CanRestore   bool   `json:"canRestore"`
}
