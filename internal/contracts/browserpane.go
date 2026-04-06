package contracts

type BrowserPaneSyncRequest struct {
	SurfaceID string `json:"surfaceId"`
	URL       string `json:"url"`
	Theme     string `json:"theme"`
	Visible   bool   `json:"visible"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}
