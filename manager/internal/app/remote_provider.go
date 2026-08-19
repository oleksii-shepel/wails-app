package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	contracts "wails-app/internal/contracts"
	vfslib "wails-app/vfs"
	"wails-app/vfs/cloudutil"
	vfsdropbox "wails-app/vfs/dropbox"
	vfsgdrive "wails-app/vfs/gdrive"
	vfsonedrive "wails-app/vfs/onedrive"
	vfss3 "wails-app/vfs/s3"
	vfssmb "wails-app/vfs/smb"
)

// remoteProvider is the backend provider contract used by RemoteConnect.
// It intentionally lives below the plugin layer: Phase 69 can adapt plugin
// contributions to this same contract without changing connection semantics.
type remoteProvider interface {
	Descriptor() contracts.RemoteProviderDescriptor
	Connect(context.Context, contracts.RemoteConfig, cloudutil.AccessTokenSource) (vfslib.FileSystem, error)
	MountPath(contracts.RemoteConfig) string
	DisplayName(contracts.RemoteConfig) string
	Host(contracts.RemoteConfig) string
}

type builtinRemoteProvider struct {
	descriptor contracts.RemoteProviderDescriptor
	connect    func(context.Context, contracts.RemoteConfig, cloudutil.AccessTokenSource) (vfslib.FileSystem, error)
}

func (p builtinRemoteProvider) Descriptor() contracts.RemoteProviderDescriptor { return p.descriptor }
func (p builtinRemoteProvider) MountPath(cfg contracts.RemoteConfig) string {
	return remoteMountPathFor(p.descriptor.Protocol, cfg)
}
func (p builtinRemoteProvider) DisplayName(cfg contracts.RemoteConfig) string {
	return remoteDisplayNameFor(p.descriptor.Protocol, cfg)
}
func (p builtinRemoteProvider) Host(cfg contracts.RemoteConfig) string {
	return remoteMountHostFor(p.descriptor.Protocol, cfg)
}
func (p builtinRemoteProvider) Connect(ctx context.Context, cfg contracts.RemoteConfig, token cloudutil.AccessTokenSource) (vfslib.FileSystem, error) {
	return p.connect(ctx, cfg, token)
}

var builtinRemoteProviders = map[contracts.RemoteProtocol]remoteProvider{
	contracts.ProtocolSMB: builtinRemoteProvider{
		descriptor: contracts.RemoteProviderDescriptor{Protocol: contracts.ProtocolSMB, Name: "SMB", Auth: "credentials", ReadOnly: false, Hierarchy: "posix-like", Capabilities: contracts.ProviderCapabilities{Pagination: false, RandomAccess: true, TotalCount: true, ServerSort: false, ServerFilter: false, RangeRead: true, Watch: false, Rename: true, AtomicMove: true, Trash: false}},
		connect: func(ctx context.Context, cfg contracts.RemoteConfig, _ cloudutil.AccessTokenSource) (vfslib.FileSystem, error) {
			return vfssmb.New(ctx, vfssmb.Config{Host: cfg.Host, Port: cfg.Port, Domain: cfg.Domain, Username: cfg.Username, Password: cfg.Password, Share: cfg.Share})
		},
	},
	contracts.ProtocolS3: builtinRemoteProvider{
		descriptor: contracts.RemoteProviderDescriptor{Protocol: contracts.ProtocolS3, Name: "S3", Auth: "access-key", ReadOnly: true, Hierarchy: "object-prefix", Capabilities: contracts.ProviderCapabilities{Pagination: true, RandomAccess: true, TotalCount: false, ServerSort: false, ServerFilter: false, RangeRead: true, Watch: false, Rename: false, AtomicMove: false, Trash: false}},
		connect: func(ctx context.Context, cfg contracts.RemoteConfig, _ cloudutil.AccessTokenSource) (vfslib.FileSystem, error) {
			return vfss3.New(ctx, vfss3.Config{Region: cfg.Region, Bucket: cfg.Bucket, Prefix: cfg.Prefix, Endpoint: cfg.Endpoint, AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey, SessionToken: cfg.SessionToken, Anonymous: cfg.Anonymous, UsePathStyle: cfg.UsePathStyle, Insecure: cfg.Insecure})
		},
	},
	contracts.ProtocolGDrive: builtinRemoteProvider{
		descriptor: contracts.RemoteProviderDescriptor{Protocol: contracts.ProtocolGDrive, Name: "Google Drive", Auth: "oauth-or-token", ReadOnly: true, Hierarchy: "id-backed", Capabilities: contracts.ProviderCapabilities{Pagination: true, RandomAccess: false, TotalCount: false, ServerSort: false, ServerFilter: false, RangeRead: false, Watch: false, Rename: false, AtomicMove: false, Trash: false}},
		connect: func(ctx context.Context, cfg contracts.RemoteConfig, token cloudutil.AccessTokenSource) (vfslib.FileSystem, error) {
			return vfsgdrive.New(ctx, vfsgdrive.Config{AccessToken: cfg.AccessToken, TokenSource: token})
		},
	},
	contracts.ProtocolDropbox: builtinRemoteProvider{
		descriptor: contracts.RemoteProviderDescriptor{Protocol: contracts.ProtocolDropbox, Name: "Dropbox", Auth: "oauth-or-token", ReadOnly: true, Hierarchy: "path-backed", Capabilities: contracts.ProviderCapabilities{Pagination: false, RandomAccess: false, TotalCount: false, ServerSort: false, ServerFilter: false, RangeRead: false, Watch: false, Rename: false, AtomicMove: false, Trash: false}},
		connect: func(ctx context.Context, cfg contracts.RemoteConfig, token cloudutil.AccessTokenSource) (vfslib.FileSystem, error) {
			return vfsdropbox.New(ctx, vfsdropbox.Config{AccessToken: cfg.AccessToken, TokenSource: token})
		},
	},
	contracts.ProtocolOneDrive: builtinRemoteProvider{
		descriptor: contracts.RemoteProviderDescriptor{Protocol: contracts.ProtocolOneDrive, Name: "OneDrive", Auth: "oauth-or-token", ReadOnly: true, Hierarchy: "id-backed", Capabilities: contracts.ProviderCapabilities{Pagination: true, RandomAccess: false, TotalCount: false, ServerSort: false, ServerFilter: false, RangeRead: false, Watch: false, Rename: false, AtomicMove: false, Trash: false}},
		connect: func(ctx context.Context, cfg contracts.RemoteConfig, token cloudutil.AccessTokenSource) (vfslib.FileSystem, error) {
			return vfsonedrive.New(ctx, vfsonedrive.Config{AccessToken: cfg.AccessToken, TokenSource: token})
		},
	},
}

func remoteProviderFor(protocol contracts.RemoteProtocol) (remoteProvider, error) {
	provider := builtinRemoteProviders[protocol]
	if provider == nil {
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
	return provider, nil
}

func remoteProviderDescriptors() []contracts.RemoteProviderDescriptor {
	result := make([]contracts.RemoteProviderDescriptor, 0, len(builtinRemoteProviders))
	for _, provider := range builtinRemoteProviders {
		result = append(result, provider.Descriptor())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Protocol < result[j].Protocol })
	return result
}

func remoteMountPathFor(protocol contracts.RemoteProtocol, cfg contracts.RemoteConfig) string {
	switch protocol {
	case contracts.ProtocolSMB:
		port := cfg.Port
		if port == 0 {
			port = 445
		}
		share := strings.Trim(cfg.Share, `/\\`)
		if share != "" {
			return fmt.Sprintf("/remote/smb/%s:%d/%s", cfg.Host, port, share)
		}
		return fmt.Sprintf("/remote/smb/%s:%d", cfg.Host, port)
	case contracts.ProtocolS3, contracts.ProtocolGDrive, contracts.ProtocolDropbox, contracts.ProtocolOneDrive:
		name := sanitizeRemoteMountSegment(cfg.Name)
		if name == "" {
			return ""
		}
		return fmt.Sprintf("/remote/%s/%s", protocol, name)
	default:
		return ""
	}
}

func remoteDisplayNameFor(protocol contracts.RemoteProtocol, cfg contracts.RemoteConfig) string {
	name := strings.TrimSpace(cfg.Name)
	if protocol == contracts.ProtocolS3 {
		bucket := strings.TrimSpace(cfg.Bucket)
		if name != "" && bucket != "" {
			return fmt.Sprintf("%s (%s)", name, bucket)
		}
		if name != "" {
			return name
		}
		if bucket != "" {
			return fmt.Sprintf("s3://%s", bucket)
		}
	}
	if name != "" {
		return name
	}
	if provider := builtinRemoteProviders[protocol]; provider != nil {
		return provider.Descriptor().Name
	}
	return fmt.Sprintf("%s://%s", protocol, remoteMountHostFor(protocol, cfg))
}

func remoteMountHostFor(protocol contracts.RemoteProtocol, cfg contracts.RemoteConfig) string {
	if protocol == contracts.ProtocolS3 {
		if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
			return endpoint
		}
		if name := strings.TrimSpace(cfg.Name); name != "" {
			return name
		}
	}
	switch protocol {
	case contracts.ProtocolGDrive:
		return "googleapis.com"
	case contracts.ProtocolDropbox:
		return "api.dropboxapi.com"
	case contracts.ProtocolOneDrive:
		return "graph.microsoft.com"
	default:
		return cfg.Host
	}
}