package pluginpackage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
)

type backendEnvelope struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type backendReply struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  interface{}     `json:"error,omitempty"`
}

type BackendProcess struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	nextID int64
}

func StartBackend(pkg UniversalPackage) (*BackendProcess, error) {
	if pkg.CachePath == "" {
		return nil, fmt.Errorf("plugin %s is not prepared", pkg.Manifest.ID)
	}
	entry := filepath.Join(pkg.CachePath, filepath.FromSlash(pkg.Manifest.Backend.Entry))
	if !withinRoot(pkg.CachePath, entry) {
		return nil, fmt.Errorf("backend entry escapes plugin cache")
	}

	cmd := exec.Command(entry)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	// stderr intentionally inherits no protocol channel. The host can add a
	// bounded diagnostics buffer later without mixing it with stdout RPC.
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, err
	}
	return &BackendProcess{
		cmd: cmd, stdin: stdin, reader: bufio.NewReader(stdout),
	}, nil
}

func (p *BackendProcess) Call(method string, params json.RawMessage) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil {
		return nil, fmt.Errorf("backend is stopped")
	}
	p.nextID++
	request := backendEnvelope{ID: p.nextID, Method: method, Params: params}
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if _, err := p.stdin.Write(data); err != nil {
		return nil, err
	}

	line, err := p.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var reply backendReply
	if err := json.Unmarshal(line, &reply); err != nil {
		return nil, fmt.Errorf("invalid backend reply: %w", err)
	}
	if reply.ID != request.ID {
		return nil, fmt.Errorf("backend reply id %d does not match request %d", reply.ID, request.ID)
	}
	if reply.Error != nil {
		return nil, fmt.Errorf("backend %s failed: %v", method, reply.Error)
	}
	return reply.Result, nil
}

func (p *BackendProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil {
		return nil
	}
	_ = p.stdin.Close()
	err := p.cmd.Process.Kill()
	_, _ = p.cmd.Process.Wait()
	p.cmd = nil
	return err
}

type BackendManager struct {
	mu       sync.Mutex
	backends map[string]*BackendProcess
}

func NewBackendManager() *BackendManager {
	return &BackendManager{backends: make(map[string]*BackendProcess)}
}

func (m *BackendManager) Call(pkg UniversalPackage, method string, params json.RawMessage) (json.RawMessage, error) {
	m.mu.Lock()
	backend := m.backends[pkg.Manifest.ID]
	if backend == nil {
		var err error
		backend, err = StartBackend(pkg)
		if err != nil {
			m.mu.Unlock()
			return nil, err
		}
		m.backends[pkg.Manifest.ID] = backend
	}
	m.mu.Unlock()
	return backend.Call(method, params)
}

func (m *BackendManager) Stop(pluginID string) error {
	m.mu.Lock()
	backend := m.backends[pluginID]
	delete(m.backends, pluginID)
	m.mu.Unlock()
	if backend == nil {
		return nil
	}
	return backend.Stop()
}

var DefaultBackendManager = NewBackendManager()
