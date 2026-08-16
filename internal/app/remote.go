package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	vfslib "wails-app/vfs"
	"wails-app/vfs/cloudutil"
)

var remoteMountSegmentPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// RemoteConnect establishes a connection to a remote server, mounts the
// resulting filesystem in the VFS, and returns metadata about the mount.
// The public path (e.g. "smb://host/share") can be used for navigation immediately.
func (s *State) RemoteConnect(cfg contracts.RemoteConfig) (*contracts.RemoteMount, error) {
	return s.remoteConnect(cfg, nil)
}

func (s *State) remoteConnect(cfg contracts.RemoteConfig, tokenSource cloudutil.AccessTokenSource) (*contracts.RemoteMount, error) {
	if s.vfs == nil {
		return nil, fmt.Errorf("vfs not initialized")
	}

	provider, err := remoteProviderFor(cfg.Protocol)
	if err != nil {
		return nil, err
	}
	mountPath := provider.MountPath(cfg)
	if mountPath == "" {
		return nil, fmt.Errorf("invalid %s mount configuration", cfg.Protocol)
	}

	// Check for a pre-existing mount at the same path.
	s.remotesMu.Lock()
	if existing, ok := s.remotes[mountPath]; ok {
		s.remotesMu.Unlock()
		return &existing, nil
	}
	s.remotesMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	remoteFS, err := provider.Connect(ctx, cfg, tokenSource)
	if err != nil {
		return nil, err
	}

	displayName := provider.DisplayName(cfg)
	if _, err := s.vfs.Mount(mountPath, remoteFS, false, displayName, vfslib.VolumeRemote); err != nil {
		if c, ok := remoteFS.(interface{ Close() error }); ok {
			_ = c.Close()
		}
		return nil, fmt.Errorf("vfs: mount %s: %w", mountPath, err)
	}

	mount := contracts.RemoteMount{
		Protocol:   cfg.Protocol,
		Host:       provider.Host(cfg),
		Name:       displayName,
		PublicPath: pathref.FromVFS(mountPath),
		MountPoint: mountPath,
		ReadOnly:   provider.Descriptor().ReadOnly,
	}

	s.remotesMu.Lock()
	s.remotes[mountPath] = mount
	s.remotesMu.Unlock()

	return &mount, nil
}

// RemoteDisconnect unmounts the remote filesystem at mountPoint and closes
// the underlying connection.
func (s *State) RemoteDisconnect(mountPoint string) error {
	if err := s.vfs.Unmount(mountPoint); err != nil {
		return err
	}
	s.remotesMu.Lock()
	delete(s.remotes, mountPoint)
	s.remotesMu.Unlock()
	s.releaseRemoteOAuthMount(mountPoint)
	return nil
}

// RemoteListConnections returns all currently active remote mounts.
func (s *State) RemoteListConnections() []contracts.RemoteMount {
	s.remotesMu.Lock()
	defer s.remotesMu.Unlock()
	result := make([]contracts.RemoteMount, 0, len(s.remotes))
	for _, m := range s.remotes {
		result = append(result, m)
	}
	return result
}

// RemoteListProviders returns the provider contract visible to the frontend.
func (s *State) RemoteListProviders() []contracts.RemoteProviderDescriptor {
	return remoteProviderDescriptors()
}

// remoteMountPath computes the internal VFS mount point for a remote config.
// The scheme-to-vfsRoot mapping mirrors the aliasSpecs in pathref/registry.go:
//
//	smb://host:445/shr  → /remote/smb/host:445/shr
func remoteMountPath(cfg contracts.RemoteConfig) string {
	return remoteMountPathFor(cfg.Protocol, cfg)
}

func remoteDisplayName(cfg contracts.RemoteConfig) string {
	return remoteDisplayNameFor(cfg.Protocol, cfg)
}

func remoteMountHost(cfg contracts.RemoteConfig) string {
	return remoteMountHostFor(cfg.Protocol, cfg)
}

func sanitizeRemoteMountSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, "\\", "-")
	trimmed = strings.ReplaceAll(trimmed, "/", "-")
	trimmed = remoteMountSegmentPattern.ReplaceAllString(trimmed, "-")
	trimmed = strings.Trim(trimmed, "-.")
	if trimmed == "." || trimmed == ".." {
		return ""
	}
	return trimmed
}
