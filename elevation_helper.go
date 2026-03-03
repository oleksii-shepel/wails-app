package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const elevatedHelperFlag = "--elevated-helper"

type elevatedRequest struct {
	Op          string `json:"op"`
	Path        string `json:"path,omitempty"`
	ShowHidden  bool   `json:"showHidden,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	Content     string `json:"content,omitempty"`
	Recursive   bool   `json:"recursive,omitempty"`
	UseTrash    *bool  `json:"useTrash,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
}

type elevatedResponse struct {
	Success    bool              `json:"success"`
	Error      string            `json:"error,omitempty"`
	DirListing *DirectoryListing `json:"dirListing,omitempty"`
	File       *FileContent      `json:"file,omitempty"`
	Operation  *OperationResult  `json:"operation,omitempty"`
}

func maybeRunElevatedHelper() (bool, error) {
	args := os.Args
	if len(args) < 2 || args[1] != elevatedHelperFlag {
		return false, nil
	}

	reqB64, resultPath, err := parseElevatedHelperArgs(args[2:])
	if err != nil {
		return true, err
	}

	reqData, err := base64.RawURLEncoding.DecodeString(reqB64)
	if err != nil {
		return true, err
	}
	var req elevatedRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		return true, err
	}

	resp := executeElevatedRequest(req)
	out, _ := json.Marshal(resp)
	if err := os.WriteFile(resultPath, out, 0600); err != nil {
		return true, err
	}
	return true, nil
}

func parseElevatedHelperArgs(args []string) (requestB64 string, resultPath string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 < len(args) {
				requestB64 = args[i+1]
				i++
			}
		case "--result":
			if i+1 < len(args) {
				resultPath = args[i+1]
				i++
			}
		}
	}
	if requestB64 == "" || resultPath == "" {
		return "", "", errors.New("missing --request/--result for elevated helper")
	}
	return requestB64, resultPath, nil
}

func executeElevatedRequest(req elevatedRequest) elevatedResponse {
	switch req.Op {
	case "list":
		listing, err := listDirectoryRaw(req.Path, req.ShowHidden)
		if err != nil {
			return elevatedResponse{Success: false, Error: err.Error()}
		}
		return elevatedResponse{Success: true, DirListing: listing}
	case "read":
		file, err := readFileRaw(req.Path, req.Encoding)
		if err != nil {
			return elevatedResponse{Success: false, Error: err.Error()}
		}
		return elevatedResponse{Success: true, File: file}
	case "write":
		op, err := writeFileRaw(req.Path, req.Content, req.Encoding)
		if err != nil {
			return elevatedResponse{Success: false, Error: err.Error()}
		}
		return elevatedResponse{Success: true, Operation: op}
	case "delete":
		op, err := deleteFileRaw(req.Path, req.Recursive, req.UseTrash)
		if err != nil {
			return elevatedResponse{Success: false, Error: err.Error()}
		}
		return elevatedResponse{Success: true, Operation: op}
	case "mkdir":
		op, err := createDirectoryRaw(req.Path, req.Recursive)
		if err != nil {
			return elevatedResponse{Success: false, Error: err.Error()}
		}
		return elevatedResponse{Success: true, Operation: op}
	case "move":
		op, err := moveFileRaw(req.Source, req.Destination)
		if err != nil {
			return elevatedResponse{Success: false, Error: err.Error()}
		}
		return elevatedResponse{Success: true, Operation: op}
	case "copy":
		op, err := copyFileRaw(req.Source, req.Destination, req.Recursive)
		if err != nil {
			return elevatedResponse{Success: false, Error: err.Error()}
		}
		return elevatedResponse{Success: true, Operation: op}
	default:
		return elevatedResponse{Success: false, Error: fmt.Sprintf("unknown elevated op: %s", req.Op)}
	}
}

func runElevatedRequest(req elevatedRequest) (elevatedResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return elevatedResponse{}, err
	}
	reqB64 := base64.RawURLEncoding.EncodeToString(reqBytes)

	resultPath := filepath.Join(os.TempDir(), fmt.Sprintf("wails-elev-%d.json", time.Now().UnixNano()))
	defer os.Remove(resultPath)

	exePath, err := os.Executable()
	if err != nil {
		return elevatedResponse{}, err
	}
	if err := launchElevatedProcess(exePath, reqB64, resultPath); err != nil {
		return elevatedResponse{}, err
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return elevatedResponse{}, err
	}

	var resp elevatedResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return elevatedResponse{}, err
	}
	if !resp.Success {
		if resp.Error == "" {
			return elevatedResponse{}, errors.New("elevated operation failed")
		}
		return elevatedResponse{}, errors.New(resp.Error)
	}
	return resp, nil
}

func isPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "eacces") ||
		strings.Contains(msg, "eperm")
}

func shouldRetryDeleteWithElevation(err error) bool {
	if isPermissionDenied(err) {
		return true
	}
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "shfileoperationw returned error code 0x78")
}
