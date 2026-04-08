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
	vfss3 "wails-app/vfs/s3"
	vfssmb "wails-app/vfs/smb"
)

var remoteMountSegmentPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// RemoteConnect establishes a connection to a remote server, mounts the
// resulting filesystem in the VFS, and returns metadata about the mount.
// The public path (e.g. "smb://host/share") can be used for navigation immediately.
func (s *State) RemoteConnect(cfg contracts.RemoteConfig) (*contracts.RemoteMount, error) {
	if s.vfs == nil {
		return nil, fmt.Errorf("vfs not initialized")
	}

	mountPath := remoteMountPath(cfg)
	if mountPath == "" {
		return nil, fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
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
	var remoteFS vfslib.FileSystem
	var err error

	switch cfg.Protocol {
	case contracts.ProtocolSMB:
		remoteFS, err = vfssmb.New(ctx, vfssmb.Config{
			Host:     cfg.Host,
			Port:     cfg.Port,
			Domain:   cfg.Domain,
			Username: cfg.Username,
			Password: cfg.Password,
			Share:    cfg.Share,
		})
	case contracts.ProtocolS3:
		remoteFS, err = vfss3.New(ctx, vfss3.Config{
			Region:          cfg.Region,
			Bucket:          cfg.Bucket,
			Prefix:          cfg.Prefix,
			Endpoint:        cfg.Endpoint,
			AccessKeyID:     cfg.AccessKeyID,
			SecretAccessKey: cfg.SecretAccessKey,
			SessionToken:    cfg.SessionToken,
			UsePathStyle:    cfg.UsePathStyle,
			Insecure:        cfg.Insecure,
		})

	default:
		return nil, fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
	}

	if err != nil {
		return nil, err
	}

	displayName := remoteDisplayName(cfg)
	if _, err := s.vfs.Mount(mountPath, remoteFS, false, displayName, vfslib.VolumeRemote); err != nil {
		if c, ok := remoteFS.(interface{ Close() error }); ok {
			_ = c.Close()
		}
		return nil, fmt.Errorf("vfs: mount %s: %w", mountPath, err)
	}

	mount := contracts.RemoteMount{
		Protocol:   cfg.Protocol,
		Host:       remoteMountHost(cfg),
		Name:       displayName,
		PublicPath: pathref.FromVFS(mountPath),
		MountPoint: mountPath,
		ReadOnly:   false,
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

// remoteMountPath computes the internal VFS mount point for a remote config.
// The scheme-to-vfsRoot mapping mirrors the aliasSpecs in pathref/registry.go:
//
//	smb://host:445/shr  → /remote/smb/host:445/shr
func remoteMountPath(cfg contracts.RemoteConfig) string {
	switch cfg.Protocol {
	case contracts.ProtocolSMB:
		port := cfg.Port
		if port == 0 {
			port = 445
		}
		share := strings.Trim(cfg.Share, `/\`)
		if share != "" {
			return fmt.Sprintf("/remote/smb/%s:%d/%s", cfg.Host, port, share)
		}
		return fmt.Sprintf("/remote/smb/%s:%d", cfg.Host, port)
	case contracts.ProtocolS3:
		name := sanitizeRemoteMountSegment(cfg.Name)
		if name == "" {
			return ""
		}
		return fmt.Sprintf("/remote/s3/%s", name)

	}
	return ""
}

func remoteDisplayName(cfg contracts.RemoteConfig) string {
	switch cfg.Protocol {
	case contracts.ProtocolS3:
		name := strings.TrimSpace(cfg.Name)
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
	return fmt.Sprintf("%s://%s", string(cfg.Protocol), remoteMountHost(cfg))
}

func remoteMountHost(cfg contracts.RemoteConfig) string {
	if cfg.Protocol == contracts.ProtocolS3 {
		if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
			return endpoint
		}
		if name := strings.TrimSpace(cfg.Name); name != "" {
			return name
		}
	}
	return cfg.Host
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
