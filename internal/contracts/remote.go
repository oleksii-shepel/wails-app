package contracts

// RemoteProtocol identifies the transport for a remote connection.
type RemoteProtocol string

const (
	ProtocolSMB RemoteProtocol = "smb"
	ProtocolS3  RemoteProtocol = "s3"
)

// RemoteConfig describes how to connect to a remote server.
// Credentials are never persisted to disk — they are held only for the
// lifetime of the application session.
type RemoteConfig struct {
	Protocol RemoteProtocol `json:"protocol"`
	Name     string         `json:"name,omitempty"`
	Host     string         `json:"host"`
	Port     int            `json:"port,omitempty"`
	Username string         `json:"username,omitempty"`
	Password string         `json:"password,omitempty"`

	// SMB-specific
	Domain string `json:"domain,omitempty"`
	Share  string `json:"share,omitempty"` // e.g. "Documents"

	// S3-compatible specific
	Endpoint        string `json:"endpoint,omitempty"`
	Region          string `json:"region,omitempty"`
	Bucket          string `json:"bucket,omitempty"`
	Prefix          string `json:"prefix,omitempty"`
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	SessionToken    string `json:"sessionToken,omitempty"`
	Anonymous       bool   `json:"anonymous,omitempty"`
	UsePathStyle    bool   `json:"usePathStyle,omitempty"`
	Insecure        bool   `json:"insecure,omitempty"`
}

// RemoteMount describes an active remote filesystem mount.
type RemoteMount struct {
	Protocol   RemoteProtocol `json:"protocol"`
	Host       string         `json:"host"`
	Name       string         `json:"name"`       // human-readable display name
	PublicPath string         `json:"publicPath"` // e.g. "smb://host/share"
	MountPoint string         `json:"mountPoint"` // internal VFS path, e.g. "/remote/smb/host:445/share"
	ReadOnly   bool           `json:"readOnly"`
}
