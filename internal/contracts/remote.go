package contracts

// RemoteProtocol identifies the transport for a remote connection.
type RemoteProtocol string

const (
	ProtocolSMB      RemoteProtocol = "smb"
	ProtocolS3       RemoteProtocol = "s3"
	ProtocolGDrive   RemoteProtocol = "gdrive"
	ProtocolDropbox  RemoteProtocol = "dropbox"
	ProtocolOneDrive RemoteProtocol = "onedrive"
)

// ProviderCapabilities mirrors the concrete mounted adapter behavior.
type ProviderCapabilities struct {
	Pagination   bool `json:"pagination"`
	RandomAccess bool `json:"randomAccess"`
	TotalCount   bool `json:"totalCount"`
	ServerSort   bool `json:"serverSort"`
	ServerFilter bool `json:"serverFilter"`
	RangeRead    bool `json:"rangeRead"`
	Watch        bool `json:"watch"`
	Rename       bool `json:"rename"`
	AtomicMove   bool `json:"atomicMove"`
	Trash        bool `json:"trash"`
}

// RemoteProviderDescriptor describes connection semantics without exposing
// provider implementation details. The frontend can use it to reason about
// authentication, mutability and hierarchy without hard-coding backend rules.
type RemoteProviderDescriptor struct {
	Protocol     RemoteProtocol       `json:"protocol"`
	Name         string               `json:"name"`
	Auth         string               `json:"auth"`
	ReadOnly     bool                 `json:"readOnly"`
	Hierarchy    string               `json:"hierarchy"`
	Capabilities ProviderCapabilities `json:"capabilities"`
}

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

	// Token-based cloud API access.
	AccessToken string `json:"accessToken,omitempty"`
}

// RemoteOAuthBeginRequest starts an interactive browser-based OAuth flow for a
// cloud provider. Client credentials remain session-only and are never written
// to disk.
type RemoteOAuthBeginRequest struct {
	Protocol RemoteProtocol `json:"protocol"`
	ClientID string         `json:"clientId"`
	Tenant   string         `json:"tenant,omitempty"`
	Scopes   []string       `json:"scopes,omitempty"`
}

// RemoteOAuthBeginResponse describes the browser flow that should be opened by
// the frontend.
type RemoteOAuthBeginResponse struct {
	SessionID   string         `json:"sessionId"`
	Protocol    RemoteProtocol `json:"protocol"`
	AuthURL     string         `json:"authUrl"`
	RedirectURI string         `json:"redirectUri"`
}

// RemoteOAuthStatus reports the current state of a browser-based OAuth
// session.
type RemoteOAuthStatus struct {
	SessionID    string         `json:"sessionId"`
	Protocol     RemoteProtocol `json:"protocol"`
	Status       string         `json:"status"`
	Error        string         `json:"error,omitempty"`
	AccountName  string         `json:"accountName,omitempty"`
	AccountEmail string         `json:"accountEmail,omitempty"`
	ExpiresAt    int64          `json:"expiresAt,omitempty"`
}

// RemoteOAuthConnectRequest mounts a previously authorized OAuth session.
type RemoteOAuthConnectRequest struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name,omitempty"`
}

// RemoteMount describes an active remote filesystem mount.
type RemoteMount struct {
	Protocol     RemoteProtocol       `json:"protocol"`
	Host         string               `json:"host"`
	Name         string               `json:"name"`       // human-readable display name
	PublicPath   string               `json:"publicPath"` // e.g. "smb://host/share"
	MountPoint   string               `json:"mountPoint"` // internal VFS path, e.g. "/remote/smb/host:445/share"
	ReadOnly     bool                 `json:"readOnly"`
	Capabilities ProviderCapabilities `json:"capabilities"`
}
