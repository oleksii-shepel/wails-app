package plugin

import (
	"context"

	contracts "wails-app/internal/contracts"
)

// CommandPlugin allows plugins to register custom commands that appear in the
// toolbar and context menu.
type CommandPlugin interface {
	Plugin
	// Commands returns the list of commands this plugin provides.
	Commands() []CommandInfo
	// IsApplicable returns true if the command can act on the given paths.
	IsApplicable(commandID string, paths []string) bool
	// Execute runs the command.
	Execute(ctx context.Context, commandID string, paths []string) error
}

// CommandResult is the standard result shape for command execution.
type CommandResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ToOperationResult converts a CommandResult to the standard operation result.
func (r CommandResult) ToOperationResult(paths ...string) contracts.OperationResult {
	return contracts.OperationResult{
		Success:       r.Success,
		Message:       r.Message,
		AffectedPaths: paths,
	}
}