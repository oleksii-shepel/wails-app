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
	stop   func() error
	nextID int64
}

func StartBackend(pkg PreparedPackage) (*BackendProcess, error) {
	if pkg.CachePath == "" || pkg.Manifest.Backend == nil {
		return nil, fmt.Errorf("plugin %s is not prepared", pkg.Manifest.ID)
	}
	entry := filepath.Join(pkg.CachePath, filepath.FromSlash(pkg.Manifest.Backend.Entry))
	if !withinRoot(pkg.CachePath, entry) {
		return nil, fmt.Errorf("backend entry escapes plugin cache")
	}

	return startBackendForPrivilege(entry, pkg.Manifest.Backend.Privilege)
}

func startInheritedBackend(entry string) (*BackendProcess, error) {
	cmd := exec.Command(entry)
	configureBackendCommand(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}

	process := &BackendProcess{
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewReader(stdout),
	}
	process.stop = func() error {
		_ = stdin.Close()
		killErr := cmd.Process.Kill()
		_ = cmd.Wait()
		return killErr
	}
	return process, nil
}

func (p *BackendProcess) Call(method string, params json.RawMessage) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin == nil || p.reader == nil {
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

	if p.stdin == nil {
		return nil
	}
	var err error
	if p.stop != nil {
		err = p.stop()
	} else {
		err = p.stdin.Close()
	}
	p.stdin = nil
	p.reader = nil
	p.cmd = nil
	p.stop = nil
	return err
}

type BackendManager struct {
	mu       sync.Mutex
	backends map[string]*BackendProcess
}

func NewBackendManager() *BackendManager {
	return &BackendManager{backends: make(map[string]*BackendProcess)}
}

func (m *BackendManager) Call(pkg PreparedPackage, method string, params json.RawMessage) (json.RawMessage, error) {
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
