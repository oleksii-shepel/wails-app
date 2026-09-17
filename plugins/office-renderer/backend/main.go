package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"office/pkg/ooxml/opc"
	pptopen "office/pkg/presentation"
	pptlegacy "office/pkg/presentation/parsers/legacy"
	pptrender "office/pkg/presentation/render"
	sheetlegacy "office/pkg/sheet/parsers/legacy"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
	sheetresolve "office/pkg/sheet/parsers/ooxml/resolve"
	sheetrender "office/pkg/sheet/render"
	wordlegacy "office/pkg/word/parsers/legacy"
	wordparts "office/pkg/word/parsers/ooxml/parts"
	wordresolve "office/pkg/word/parsers/ooxml/resolve"
	wordrender "office/pkg/word/render"
)

type request struct {
	ID     any             `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type response struct {
	ID     any       `json:"id,omitempty"`
	Result any       `json:"result,omitempty"`
	Error  *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = encoder.Encode(response{Error: &rpcError{Code: "invalid_request", Message: err.Error()}})
			continue
		}
		result, err := dispatch(req.Method, req.Params)
		if err != nil {
			_ = encoder.Encode(response{ID: req.ID, Error: &rpcError{Code: "office_error", Message: err.Error()}})
			continue
		}
		_ = encoder.Encode(response{ID: req.ID, Result: result})
	}
}

func dispatch(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "hello":
		return map[string]any{"protocol": 1, "plugin": "aether.office-renderer", "target": "windows-x64"}, nil
	case "office.render":
		var params struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		html, err := render(params.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"html": html}, nil
	default:
		return nil, fmt.Errorf("unknown method %q", method)
	}
}

func render(path string) (string, error) {
	path, err := nativePath(path)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", path)
	}

	switch documentKind(filepath.Ext(path)) {
	case "doc":
		doc, err := wordlegacy.ParseDoc(path)
		if err != nil {
			return "", err
		}
		return wordrender.HTML(doc)
	case "xls":
		book, err := sheetlegacy.ParseXls(path)
		if err != nil {
			return "", err
		}
		return sheetrender.HTML(book)
	case "ppt":
		presentation, err := pptlegacy.ParsePpt(path)
		if err != nil {
			return "", err
		}
		return pptrender.HTML(presentation)
	case "docx":
		pkg, err := opc.Open(path)
		if err != nil {
			return "", err
		}
		parts, err := wordparts.LoadPackage(pkg)
		if err != nil {
			return "", err
		}
		parts.SourcePath = path
		doc, err := wordresolve.ResolvePackage(parts)
		if err != nil {
			return "", err
		}
		return wordrender.HTML(doc)
	case "xlsx":
		pkg, err := opc.Open(path)
		if err != nil {
			return "", err
		}
		parts, err := sheetparts.LoadPackage(pkg)
		if err != nil {
			return "", err
		}
		book, err := sheetresolve.ResolvePackage(parts)
		if err != nil {
			return "", err
		}
		return sheetrender.HTML(book)
	case "pptx":
		presentation, err := pptopen.Open(path)
		if err != nil {
			return "", err
		}
		html, _, err := pptrender.HTMLWithOptions(presentation, pptrender.HTMLOptions{})
		return html, err
	default:
		return "", fmt.Errorf("unsupported Office format %q", filepath.Ext(path))
	}
}

// documentKind maps legacy formats and all supported Word, Excel, and
// PowerPoint OOXML variants to the parser that understands their package.
func documentKind(extension string) string {
	switch strings.ToLower(extension) {
	case ".doc":
		return "doc"
	case ".xls":
		return "xls"
	case ".ppt":
		return "ppt"
	case ".docx", ".docm", ".dotx", ".dotm":
		return "docx"
	case ".xlsx", ".xlsm", ".xltx", ".xltm", ".xlam":
		return "xlsx"
	case ".pptx", ".pptm", ".ppsx", ".ppsm", ".potx", ".potm":
		return "pptx"
	default:
		return ""
	}
}

// nativePath converts Manager's public local-filesystem URI into the Windows
// path expected by the Office parser. Other URI schemes have no local file the
// standalone plugin backend can open.
func nativePath(value string) (string, error) {
	path := strings.TrimSpace(value)
	const scheme = "filesystem://"
	if len(path) < len(scheme) || !strings.EqualFold(path[:len(scheme)], scheme) {
		return path, nil
	}

	body := strings.TrimPrefix(path[len(scheme):], "/")
	if len(body) == 0 || len(body) < 1 || !isDriveLetter(body[0]) {
		return "", fmt.Errorf("invalid filesystem URI %q", value)
	}
	if len(body) > 1 && body[1] != '/' && body[1] != '\\' {
		return "", fmt.Errorf("invalid filesystem URI %q", value)
	}
	rest := ""
	if len(body) > 1 {
		rest = strings.TrimLeft(body[1:], "/\\")
	}
	if rest == "" {
		return string(body[0]) + `:\`, nil
	}
	return string(body[0]) + `:\` + filepath.FromSlash(rest), nil
}

func isDriveLetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
