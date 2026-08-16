package contracts

type PathCapacity struct {
	TotalSpace int64   `json:"totalSpace"`
	FreeSpace  int64   `json:"freeSpace"`
	FileSystem *string `json:"fileSystem,omitempty"`
}
