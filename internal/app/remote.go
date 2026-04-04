package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	vfslib "wails-app/vfs"
	vfsftp "wails-app/vfs/ftp"
	vfssftp "wails-app/vfs/sftp"
	vfssmb "wails-app/vfs/smb"
	vfswebdav "wails-app/vfs/webdav"
)

// RemoteConnect establishes a connection to a remote server, mounts the
// resulting filesystem in the VFS, and returns metadata about the mount.
// The public path (e.g. "ftp://host:21") can be used for navigation immediately.
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
	case contracts.ProtocolFTP:
		remoteFS, err = vfsftp.New(ctx, vfsftp.Config{
			Host:     cfg.Host,
			Port:     cfg.Port,
			Username: cfg.Username,
			Password: cfg.Password,
		})

	case contracts.ProtocolSFTP:
		sftpCfg := vfssftp.Config{
			Host:          cfg.Host,
			Port:          cfg.Port,
			Username:      cfg.Username,
			Password:      cfg.Password,
			IgnoreHostKey: cfg.IgnoreHostKey,
		}
		if cfg.PrivateKey != "" {
			sftpCfg.PrivateKey = []byte(cfg.PrivateKey)
		}
		remoteFS, err = vfssftp.New(ctx, sftpCfg)

	case contracts.ProtocolSMB:
		remoteFS, err = vfssmb.New(ctx, vfssmb.Config{
			Host:     cfg.Host,
			Port:     cfg.Port,
			Domain:   cfg.Domain,
			Username: cfg.Username,
			Password: cfg.Password,
			Share:    cfg.Share,
		})

	case contracts.ProtocolWebDAV:
		schema := cfg.Schema
		if schema == "" {
			schema = "https"
		}
		remoteFS, err = vfswebdav.New(ctx, vfswebdav.Config{
			Host:     cfg.Host,
			Port:     cfg.Port,
			Schema:   schema,
			BasePath: cfg.BasePath,
			Username: cfg.Username,
			Password: cfg.Password,
			Insecure: cfg.Insecure,
		})

	default:
		return nil, fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
	}

	if err != nil {
		return nil, err
	}

	displayName := fmt.Sprintf("%s://%s", string(cfg.Protocol), cfg.Host)
	if _, err := s.vfs.Mount(mountPath, remoteFS, false, displayName, vfslib.VolumeRemote); err != nil {
		if c, ok := remoteFS.(interface{ Close() error }); ok {
			_ = c.Close()
		}
		return nil, fmt.Errorf("vfs: mount %s: %w", mountPath, err)
	}

	mount := contracts.RemoteMount{
		Protocol:   cfg.Protocol,
		Host:       cfg.Host,
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
//	ftp://host:21       → /remote/ftp/host:21
//	sftp://host:22      → /remote/sftp/host:22
//	smb://host:445/shr  → /remote/smb/host:445/shr
//	webdav://host:443   → /remote/webdav/host:443
func remoteMountPath(cfg contracts.RemoteConfig) string {
	switch cfg.Protocol {
	case contracts.ProtocolFTP:
		port := cfg.Port
		if port == 0 {
			port = 21
		}
		return fmt.Sprintf("/remote/ftp/%s:%d", cfg.Host, port)

	case contracts.ProtocolSFTP:
		port := cfg.Port
		if port == 0 {
			port = 22
		}
		return fmt.Sprintf("/remote/sftp/%s:%d", cfg.Host, port)

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

	case contracts.ProtocolWebDAV:
		port := cfg.Port
		schema := cfg.Schema
		if schema == "" {
			schema = "https"
		}
		if port == 0 {
			if schema == "https" {
				port = 443
			} else {
				port = 80
			}
		}
		base := strings.Trim(cfg.BasePath, "/")
		if base != "" {
			return fmt.Sprintf("/remote/webdav/%s:%d/%s", cfg.Host, port, base)
		}
		return fmt.Sprintf("/remote/webdav/%s:%d", cfg.Host, port)
	}
	return ""
}
