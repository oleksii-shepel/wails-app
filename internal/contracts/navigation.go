package contracts

type HistoryEntry struct {
	Path      string `json:"path"`
	Timestamp int64  `json:"timestamp"`
	PaneID    string `json:"paneId"`
}

type NavigationHistory struct {
	Entries    []HistoryEntry `json:"entries"`
	MaxEntries int            `json:"maxEntries"`
}
