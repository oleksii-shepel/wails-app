package contracts

type PathAttributes struct {
	Permissions string `json:"permissions"`
	UnixMode    uint32 `json:"unixMode"`
	ReadOnly    bool   `json:"readOnly"`
	Hidden      bool   `json:"hidden"`
	System      bool   `json:"system"`
	Archive     bool   `json:"archive"`
}

type PathAttributesUpdate struct {
	UnixMode *uint32 `json:"unixMode,omitempty"`
	ReadOnly *bool   `json:"readOnly,omitempty"`
	Hidden   *bool   `json:"hidden,omitempty"`
	System   *bool   `json:"system,omitempty"`
	Archive  *bool   `json:"archive,omitempty"`
}