package pluginpackage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const StateFileName = "plugin-state.json"
const AutoQuarantineFailureThreshold = 3

type ActivationStatus string

const (
	ActivationNever   ActivationStatus = "never"
	ActivationSuccess ActivationStatus = "success"
	ActivationFailed  ActivationStatus = "failed"
)

type PackageState struct {
	PluginID                      string           `json:"pluginId"`
	Enabled                       bool             `json:"enabled"`
	Trusted                       bool             `json:"trusted"`
	TrustedVersion                string           `json:"trustedVersion,omitempty"`
	GrantedPermissions            []string         `json:"grantedPermissions,omitempty"`
	Quarantined                   bool             `json:"quarantined"`
	QuarantineReason              string           `json:"quarantineReason,omitempty"`
	LastActivationStatus          ActivationStatus `json:"lastActivationStatus"`
	LastActivationError           string           `json:"lastActivationError,omitempty"`
	LastActivatedAt               int64            `json:"lastActivatedAt,omitempty"`
	ConsecutiveActivationFailures int              `json:"consecutiveActivationFailures,omitempty"`
	LastSeenVersion               string           `json:"lastSeenVersion,omitempty"`
	UpdatedAt                     int64            `json:"updatedAt"`
}

type PackageStatePatch struct {
	Enabled            *bool     `json:"enabled,omitempty"`
	Trusted            *bool     `json:"trusted,omitempty"`
	TrustedVersion     *string   `json:"-"`
	GrantedPermissions *[]string `json:"grantedPermissions,omitempty"`
	ClearQuarantine    bool      `json:"clearQuarantine,omitempty"`
}

type stateFile struct {
	Version int                     `json:"version"`
	Plugins map[string]PackageState `json:"plugins"`
}

var stateFileMu sync.Mutex

type StateStore struct {
	path string
}

func DefaultStatePath(appName string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, appName, StateFileName), nil
}

func NewStateStore(appName string) (*StateStore, error) {
	path, err := DefaultStatePath(appName)
	if err != nil {
		return nil, err
	}
	return NewStateStoreAt(path)
}

func NewStateStoreAt(path string) (*StateStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("plugin state path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return &StateStore{path: path}, nil
}

func (s *StateStore) List() ([]PackageState, error) {
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	out := make([]PackageState, 0, len(data.Plugins))
	for _, state := range data.Plugins {
		state.GrantedPermissions = copyStrings(state.GrantedPermissions)
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PluginID < out[j].PluginID })
	return out, nil
}

func (s *StateStore) Get(pluginID string) (PackageState, error) {
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return PackageState{}, err
	}
	state, ok := data.Plugins[pluginID]
	if !ok {
		state = defaultPackageState(pluginID)
	}
	state.GrantedPermissions = copyStrings(state.GrantedPermissions)
	return state, nil
}

func (s *StateStore) Update(pluginID string, patch PackageStatePatch) (PackageState, error) {
	if strings.TrimSpace(pluginID) == "" {
		return PackageState{}, fmt.Errorf("plugin id is required")
	}
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return PackageState{}, err
	}
	state, ok := data.Plugins[pluginID]
	if !ok {
		state = defaultPackageState(pluginID)
	}
	if patch.Enabled != nil {
		state.Enabled = *patch.Enabled
	}
	if patch.Trusted != nil {
		state.Trusted = *patch.Trusted
		if state.Trusted && patch.TrustedVersion != nil {
			state.TrustedVersion = strings.TrimSpace(*patch.TrustedVersion)
		} else if !state.Trusted {
			state.TrustedVersion = ""
		}
	}
	if patch.GrantedPermissions != nil {
		state.GrantedPermissions = uniqueSorted(*patch.GrantedPermissions)
	}
	if patch.ClearQuarantine {
		state.Quarantined = false
		state.QuarantineReason = ""
		state.ConsecutiveActivationFailures = 0
	}
	state.UpdatedAt = nowMillis()
	data.Plugins[pluginID] = state
	if err := s.saveLocked(data); err != nil {
		return PackageState{}, err
	}
	state.GrantedPermissions = copyStrings(state.GrantedPermissions)
	return state, nil
}

func (s *StateStore) Quarantine(pluginID, reason string) (PackageState, error) {
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return PackageState{}, err
	}
	state, ok := data.Plugins[pluginID]
	if !ok {
		state = defaultPackageState(pluginID)
	}
	state.Quarantined = true
	state.QuarantineReason = strings.TrimSpace(reason)
	state.LastActivationStatus = ActivationFailed
	state.LastActivationError = state.QuarantineReason
	state.UpdatedAt = nowMillis()
	data.Plugins[pluginID] = state
	if err := s.saveLocked(data); err != nil {
		return PackageState{}, err
	}
	return state, nil
}

func (s *StateStore) RecordActivation(pluginID string, success bool, activationErr string) (PackageState, error) {
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return PackageState{}, err
	}
	state, ok := data.Plugins[pluginID]
	if !ok {
		state = defaultPackageState(pluginID)
	}
	state.LastActivatedAt = nowMillis()
	state.LastActivationError = strings.TrimSpace(activationErr)
	if success {
		state.LastActivationStatus = ActivationSuccess
		state.LastActivationError = ""
		state.ConsecutiveActivationFailures = 0
	} else {
		state.LastActivationStatus = ActivationFailed
		state.ConsecutiveActivationFailures++
		if state.ConsecutiveActivationFailures >= AutoQuarantineFailureThreshold {
			state.Quarantined = true
			if state.LastActivationError != "" {
				state.QuarantineReason = fmt.Sprintf("Automatically quarantined after %d consecutive activation failures: %s", AutoQuarantineFailureThreshold, state.LastActivationError)
			} else {
				state.QuarantineReason = fmt.Sprintf("Automatically quarantined after %d consecutive activation failures.", AutoQuarantineFailureThreshold)
			}
		}
	}
	state.UpdatedAt = nowMillis()
	data.Plugins[pluginID] = state
	if err := s.saveLocked(data); err != nil {
		return PackageState{}, err
	}
	return state, nil
}

// Reconcile updates observational metadata for packages currently visible on disk.
// It never broadens enabled/permission decisions. Trust is version-bound: a package
// update remains enabled but must be explicitly trusted again before activation.
func (s *StateStore) Reconcile(packages []DiscoveredPackage) error {
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return err
	}
	changedAny := false
	now := nowMillis()
	for _, pkg := range packages {
		if !pkg.Valid || strings.TrimSpace(pkg.Manifest.ID) == "" {
			continue
		}
		state, ok := data.Plugins[pkg.Manifest.ID]
		changed := false
		if !ok {
			state = defaultPackageState(pkg.Manifest.ID)
			changed = true
		}
		if state.LastSeenVersion != pkg.Manifest.Version {
			state.LastSeenVersion = pkg.Manifest.Version
			changed = true
		}
		if state.Trusted && state.TrustedVersion != "" && state.TrustedVersion != pkg.Manifest.Version {
			state.Trusted = false
			state.TrustedVersion = ""
			changed = true
		}
		if changed {
			state.UpdatedAt = now
			data.Plugins[pkg.Manifest.ID] = state
			changedAny = true
		}
	}
	if !changedAny {
		return nil
	}
	return s.saveLocked(data)
}

func defaultPackageState(pluginID string) PackageState {
	return PackageState{
		PluginID:             pluginID,
		Enabled:              false,
		Trusted:              false,
		GrantedPermissions:   []string{},
		LastActivationStatus: ActivationNever,
		UpdatedAt:            nowMillis(),
	}
}

func (s *StateStore) loadLocked() (stateFile, error) {
	data := stateFile{Version: 1, Plugins: map[string]PackageState{}}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return data, err
	}
	if len(raw) == 0 {
		return data, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return stateFile{}, fmt.Errorf("invalid plugin state file: %w", err)
	}
	if data.Version != 1 {
		return stateFile{}, fmt.Errorf("unsupported plugin state version %d", data.Version)
	}
	if data.Plugins == nil {
		data.Plugins = map[string]PackageState{}
	}
	return data, nil
}

func (s *StateStore) saveLocked(data stateFile) error {
	data.Version = 1
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func copyStrings(values []string) []string { return append([]string(nil), values...) }
func nowMillis() int64                     { return time.Now().UnixMilli() }

// ValidateGrantedPermissions ensures persisted grants cannot exceed the permissions
// requested by the package manifest. Unknown grants are rejected at the backend boundary.
func ValidateGrantedPermissions(manifest Manifest, granted []string) error {
	requested := make(map[string]struct{}, len(manifest.Permissions))
	for _, permission := range manifest.Permissions {
		requested[permission] = struct{}{}
	}
	for _, permission := range granted {
		if _, ok := requested[permission]; !ok {
			return fmt.Errorf("plugin %s did not request permission %q", manifest.ID, permission)
		}
	}
	return nil
}