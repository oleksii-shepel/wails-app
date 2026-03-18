package filesystem

import (
	"fmt"

	contracts "wails-app/internal/contracts"
	elevation "wails-app/internal/elevation"
)

type RawOperations struct {
	ListDirectory   func(path string, showHidden bool) (*contracts.DirectoryListing, error)
	ReadFile        func(path string, encoding string) (*contracts.FileContent, error)
	WriteFile       func(path string, content string, encoding string) (*contracts.OperationResult, error)
	DeleteFile      func(path string, recursive bool, useTrash *bool) (*contracts.OperationResult, error)
	CreateDirectory func(path string, recursive bool) (*contracts.OperationResult, error)
	MoveFile        func(source, destination string) (*contracts.OperationResult, error)
	CopyFile        func(source, destination string, recursive bool) (*contracts.OperationResult, error)
}

func NewElevatedHandler(ops RawOperations) elevation.Handler {
	return func(req elevation.Request) elevation.Response {
		switch req.Op {
		case "list":
			listing, err := ops.ListDirectory(req.Path, req.ShowHidden)
			if err != nil {
				return elevation.Response{Success: false, Error: err.Error()}
			}
			return elevation.Response{Success: true, DirListing: listing}
		case "read":
			file, err := ops.ReadFile(req.Path, req.Encoding)
			if err != nil {
				return elevation.Response{Success: false, Error: err.Error()}
			}
			return elevation.Response{Success: true, File: file}
		case "write":
			op, err := ops.WriteFile(req.Path, req.Content, req.Encoding)
			if err != nil {
				return elevation.Response{Success: false, Error: err.Error()}
			}
			return elevation.Response{Success: true, Operation: op}
		case "delete":
			op, err := ops.DeleteFile(req.Path, req.Recursive, req.UseTrash)
			if err != nil {
				return elevation.Response{Success: false, Error: err.Error()}
			}
			return elevation.Response{Success: true, Operation: op}
		case "mkdir":
			op, err := ops.CreateDirectory(req.Path, req.Recursive)
			if err != nil {
				return elevation.Response{Success: false, Error: err.Error()}
			}
			return elevation.Response{Success: true, Operation: op}
		case "move":
			op, err := ops.MoveFile(req.Source, req.Destination)
			if err != nil {
				return elevation.Response{Success: false, Error: err.Error()}
			}
			return elevation.Response{Success: true, Operation: op}
		case "copy":
			op, err := ops.CopyFile(req.Source, req.Destination, req.Recursive)
			if err != nil {
				return elevation.Response{Success: false, Error: err.Error()}
			}
			return elevation.Response{Success: true, Operation: op}
		default:
			return elevation.Response{Success: false, Error: fmt.Sprintf("unknown elevated op: %s", req.Op)}
		}
	}
}