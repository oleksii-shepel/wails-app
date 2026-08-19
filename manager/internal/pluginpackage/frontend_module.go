package pluginpackage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	FrontendModuleFileName = "frontend.json"
	MaxFrontendModuleBytes = 1 << 20
)

type FrontendCommand struct {
	ID              string   `json:"id"`
	Title           string   `json:"title,omitempty"`
	Name            string   `json:"name,omitempty"`
	Aliases         []string `json:"aliases,omitempty"`
	TargetCommandID string   `json:"targetCommandId"`
}

type FrontendPlacement struct {
	ID        string `json:"id"`
	CommandID string `json:"commandId"`
	When      string `json:"when,omitempty"`
}

type FrontendModule struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Commands      []FrontendCommand   `json:"commands,omitempty"`
	Toolbar       []FrontendPlacement `json:"toolbar,omitempty"`
	ContextMenu   []FrontendPlacement `json:"contextMenu,omitempty"`
}

func ReadFrontendModule(root string, pkg DiscoveredPackage) (FrontendModule, error) {
	if !pkg.Valid {
		return FrontendModule{}, fmt.Errorf("plugin %s is not a valid discovered package", pkg.Manifest.ID)
	}

	modulePath := filepath.Join(pkg.Path, FrontendModuleFileName)
	if !withinRoot(root, modulePath) || !withinRoot(pkg.Path, modulePath) {
		return FrontendModule{}, fmt.Errorf("frontend module escapes plugin directory")
	}

	info, err := os.Lstat(modulePath)
	if err != nil {
		if os.IsNotExist(err) {
			return FrontendModule{SchemaVersion: 1}, nil
		}
		return FrontendModule{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return FrontendModule{}, fmt.Errorf("frontend module must be a regular file")
	}

	f, err := os.Open(modulePath)
	if err != nil {
		return FrontendModule{}, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, MaxFrontendModuleBytes+1))
	if err != nil {
		return FrontendModule{}, err
	}
	if len(data) > MaxFrontendModuleBytes {
		return FrontendModule{}, fmt.Errorf("frontend module exceeds %d bytes", MaxFrontendModuleBytes)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var module FrontendModule
	if err := dec.Decode(&module); err != nil {
		return FrontendModule{}, fmt.Errorf("invalid frontend module JSON: %w", err)
	}
	if dec.More() {
		return FrontendModule{}, fmt.Errorf("frontend module contains trailing JSON values")
	}
	if err := validateFrontendModule(pkg.Manifest.ID, module); err != nil {
		return FrontendModule{}, err
	}
	return module, nil
}

func validateFrontendModule(pluginID string, module FrontendModule) error {
	if module.SchemaVersion != 1 {
		return fmt.Errorf("unsupported frontend module schema %d", module.SchemaVersion)
	}

	commandIDs := make(map[string]struct{}, len(module.Commands))
	for _, command := range module.Commands {
		if !pluginIDPattern.MatchString(command.ID) {
			return fmt.Errorf("invalid frontend command id %q", command.ID)
		}
		if strings.TrimSpace(command.TargetCommandID) == "" {
			return fmt.Errorf("frontend command %q is missing targetCommandId", command.ID)
		}
		if _, exists := commandIDs[command.ID]; exists {
			return fmt.Errorf("duplicate frontend command id %q", command.ID)
		}
		commandIDs[command.ID] = struct{}{}
	}

	for _, placement := range append(append([]FrontendPlacement{}, module.Toolbar...), module.ContextMenu...) {
		if strings.TrimSpace(placement.ID) == "" {
			return fmt.Errorf("frontend placement id is required")
		}
		if _, ok := commandIDs[placement.CommandID]; !ok {
			return fmt.Errorf("frontend placement %q references unknown command %q", placement.ID, placement.CommandID)
		}
		switch placement.When {
		case "", "always", "selection", "single-selection":
		default:
			return fmt.Errorf("frontend placement %q has unsupported when value %q", placement.ID, placement.When)
		}
	}
	return nil
}