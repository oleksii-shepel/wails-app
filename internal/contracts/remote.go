package contracts

// RemoteProtocol identifies the transport for a remote connection.
type RemoteProtocol string

const (
	ProtocolFTP    RemoteProtocol = "ftp"
	ProtocolSFTP   RemoteProtocol = "sftp"
	ProtocolSMB    RemoteProtocol = "smb"
	ProtocolWebDAV RemoteProtocol = "webdav"
)

// RemoteConfig describes how to connect to a remote server.
// Credentials are never persisted to disk — they are held only for the
// lifetime of the application session.
type RemoteConfig struct {
	Protocol RemoteProtocol `json:"protocol"`
	Host     string         `json:"host"`
	Port     int            `json:"port,omitempty"`
	Username string         `json:"username,omitempty"`
	Password string         `json:"password,omitempty"`

	// SFTP-specific
	PrivateKey    string `json:"privateKey,omitempty"`    // PEM text
	IgnoreHostKey bool   `json:"ignoreHostKey,omitempty"` // dev/testing only

	// SMB-specific
	Domain string `json:"domain,omitempty"`
	Share  string `json:"share,omitempty"` // e.g. "Documents"

	// WebDAV-specific
	Schema   string `json:"schema,omitempty"`   // "http" or "https"
	BasePath string `json:"basePath,omitempty"` // server-side path prefix
	Insecure bool   `json:"insecure,omitempty"` // skip TLS verification
}

// RemoteMount describes an active remote filesystem mount.
type RemoteMount struct {
	Protocol   RemoteProtocol `json:"protocol"`
	Host       string         `json:"host"`
	Name       string         `json:"name"`       // human-readable display name
	PublicPath string         `json:"publicPath"` // e.g. "ftp://host:21"
	MountPoint string         `json:"mountPoint"` // internal VFS path, e.g. "/remote/ftp/host:21"
	ReadOnly   bool           `json:"readOnly"`
}
