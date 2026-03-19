package memoryfs

type State struct {
	Nodes      map[string]Node `json:"nodes"`
	StashStack []StashItem     `json:"stashStack,omitempty"`
	NextID     int64           `json:"nextId"`
}

type Node struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	IsDir      bool   `json:"isDir"`
	Size       int64  `json:"size"`
	CreatedAt  int64  `json:"createdAt"`
	ModifiedAt int64  `json:"modifiedAt"`
	AccessedAt int64  `json:"accessedAt"`
	StorageID  string `json:"storageId,omitempty"`
}

type StashItem struct {
	MemoryPath  string `json:"memoryPath"`
	SourcePath  string `json:"sourcePath,omitempty"`
	StashedAt   int64  `json:"stashedAt"`
	Description string `json:"description,omitempty"`
}
