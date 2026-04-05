package app

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
)

// ListDirectory is the pane-facing directory entrypoint. It can expose remote
// scheme roots and auto-mount anonymous FTP locations on first navigation.
func (s *State) ListDirectory(path string, showHidden bool) (*contracts.DirectoryListing, error) {
	normalized := pathref.Normalize(path)
	if listing, ok := s.listRemoteRoot(normalized); ok {
		return listing, nil
	}
	if err := s.ensureRemoteMount(normalized); err != nil {
		return nil, err
	}
	return s.Filesystem.ListDirectory(normalized, showHidden)
}

func (s *State) GetFileInfo(path string) (*contracts.FileInfo, error) {
	normalized := pathref.Normalize(path)
	if protocol, ok := remoteProtocolRoot(normalized); ok {
		return &contracts.FileInfo{
			Name: pathref.FromVFS(remoteRootMountPoint(protocol)),
			Path: normalized,
			Type: contracts.FileTypeDirectory,
		}, nil
	}
	if err := s.ensureRemoteMount(normalized); err != nil {
		return nil, err
	}
	return s.Filesystem.GetFileInfo(normalized)
}

func (s *State) ProbeFile(path string) (*contracts.FileProbe, error) {
	normalized := pathref.Normalize(path)
	if _, ok := remoteProtocolRoot(normalized); ok {
		return nil, fmt.Errorf("cannot probe remote root")
	}
	if err := s.ensureRemoteMount(normalized); err != nil {
		return nil, err
	}
	return s.Filesystem.ProbeFile(normalized)
}

func (s *State) ensureRemoteMount(path string) error {
	if s.vfs == nil {
		return nil
	}

	cfg, ok, err := remoteConfigFromPublicPath(path)
	if err != nil || !ok {
		return err
	}

	if cfg.Protocol != contracts.ProtocolFTP {
		return nil
	}

	if s.vfs.IsMounted(pathref.ToVFS(path)) {
		return nil
	}

	_, err = s.RemoteConnect(cfg)
	return err
}

func (s *State) listRemoteRoot(path string) (*contracts.DirectoryListing, bool) {
	protocol, ok := remoteProtocolRoot(path)
	if !ok {
		return nil, false
	}

	s.remotesMu.Lock()
	defer s.remotesMu.Unlock()

	entries := make([]contracts.FileInfo, 0, len(s.remotes))
	for _, mount := range s.remotes {
		if mount.Protocol != protocol {
			continue
		}

		name := strings.TrimPrefix(mount.PublicPath, path)
		if name == "" || name == mount.PublicPath {
			name = mount.Host
		}

		entries = append(entries, contracts.FileInfo{
			Name: name,
			Path: mount.PublicPath,
			Type: contracts.FileTypeDirectory,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Path) < strings.ToLower(entries[j].Path)
	})

	return &contracts.DirectoryListing{
		Path:      path,
		Entries:   entries,
		TotalSize: 0,
	}, true
}

func remoteProtocolRoot(path string) (contracts.RemoteProtocol, bool) {
	switch strings.ToLower(path) {
	case pathref.FTPScheme:
		return contracts.ProtocolFTP, true
	case pathref.SFTPScheme:
		return contracts.ProtocolSFTP, true
	case pathref.SMBScheme:
		return contracts.ProtocolSMB, true
	case pathref.WebDAVScheme:
		return contracts.ProtocolWebDAV, true
	default:
		return "", false
	}
}

func remoteRootMountPoint(protocol contracts.RemoteProtocol) string {
	switch protocol {
	case contracts.ProtocolFTP:
		return "/remote/ftp"
	case contracts.ProtocolSFTP:
		return "/remote/sftp"
	case contracts.ProtocolSMB:
		return "/remote/smb"
	case contracts.ProtocolWebDAV:
		return "/remote/webdav"
	default:
		return ""
	}
}

func remoteConfigFromPublicPath(path string) (contracts.RemoteConfig, bool, error) {
	normalized := pathref.Normalize(path)
	parsed, err := url.Parse(normalized)
	if err != nil {
		return contracts.RemoteConfig{}, false, fmt.Errorf("invalid remote path %q: %w", path, err)
	}

	if parsed.Host == "" {
		return contracts.RemoteConfig{}, false, nil
	}

	switch strings.ToLower(parsed.Scheme) {
	case string(contracts.ProtocolFTP):
		port := 21
		if rawPort := parsed.Port(); rawPort != "" {
			parsedPort, convErr := strconv.Atoi(rawPort)
			if convErr != nil {
				return contracts.RemoteConfig{}, false, fmt.Errorf("invalid FTP port %q", rawPort)
			}
			port = parsedPort
		}

		username := "anonymous"
		password := "anonymous@"
		if parsed.User != nil {
			if user := parsed.User.Username(); user != "" {
				username = user
				if pass, ok := parsed.User.Password(); ok {
					password = pass
				} else {
					password = ""
				}
			}
		}

		return contracts.RemoteConfig{
			Protocol: contracts.ProtocolFTP,
			Host:     parsed.Hostname(),
			Port:     port,
			Username: username,
			Password: password,
		}, true, nil

	default:
		return contracts.RemoteConfig{}, false, nil
	}
}
