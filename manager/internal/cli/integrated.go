package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"wails-app/internal/contracts"
	"wails-app/internal/pathref"
	"wails-app/internal/platform"
	"wails-app/vfs"
	archives "wails-app/vfs/archive"
	_ "wails-app/vfs/archive/7z"
	_ "wails-app/vfs/archive/rar"
	_ "wails-app/vfs/archive/tar"
	_ "wails-app/vfs/archive/zip"
)

func archive(s *session, args []string) error {
	if len(args) == 0 {
		return usageError("usage: archive <list|zip|extract> ...")
	}
	switch args[0] {
	case "list":
		o, raw, err := parse(args[1:], "", "--inner")
		if err != nil {
			return err
		}
		if len(raw) != 1 {
			return usageError("usage: archive list <archive> [--inner path]")
		}
		source, err := localPath(s.Cwd, raw[0])
		if err != nil {
			return err
		}
		a, err := openArchive(source)
		if err != nil {
			return err
		}
		defer a.Close()
		inner := o.value("", "--inner")
		items, err := a.ReadDir(context.Background(), inner)
		if err != nil {
			return err
		}
		results := make([]entry, 0, len(items))
		for _, item := range items {
			results = append(results, describe(item.Path(), item))
		}
		return s.emit(map[string]any{"archive": source, "count": len(items), "entries": results}, func() {
			rows := [][]string{{"TYPE", "SIZE", "PATH"}}
			for _, item := range results {
				rows = append(rows, []string{item.Type, humanSize(item.Size), item.Path})
			}
			s.table(rows)
		})
	case "zip":
		if len(args) < 3 {
			return usageError("usage: archive zip <destination.zip> <sources...>")
		}
		dest, err := localPath(s.Cwd, args[1])
		if err != nil {
			return err
		}
		if !strings.EqualFold(filepath.Ext(dest), ".zip") {
			return usageError("ZIP destination must end in .zip")
		}
		sources, err := s.paths(args[2:])
		if err != nil {
			return err
		}
		if s.CreateZip == nil {
			return fmt.Errorf("file manager ZIP service is unavailable")
		}
		for i, source := range sources {
			sources[i] = pathref.FromNativePath(source)
		}
		result, err := s.CreateZip(sources, pathref.FromNativePath(dest))
		if err != nil {
			return err
		}
		if !result.Success {
			return fmt.Errorf("create ZIP: %s", result.Message)
		}
		info, err := os.Stat(dest)
		if err != nil {
			return err
		}
		return s.emit(map[string]any{"destination": dest, "bytes": info.Size()}, func() {
			fmt.Fprintf(&s.output, "Created %s (%s)\n", dest, humanSize(info.Size()))
		})
	case "extract":
		return extract(s, args[1:])
	default:
		return usageError("usage: archive <list|zip|extract> ...")
	}
}

func openArchive(source string) (archives.Archive, error) {
	h, ok := archives.ForPath(source)
	if !ok || !h.CanRead() {
		return nil, fmt.Errorf("unsupported archive format: %s", source)
	}
	f, _, err := openRegular(source)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	return h.Open(context.Background(), source, archives.OpenOptions{ReadOnly: true})
}

func extract(s *session, args []string) error {
	o, raw, err := parse(args, "--sub --flatten", "")
	if err != nil {
		return err
	}
	if len(raw) < 1 || len(raw) > 2 {
		return usageError("usage: archive extract <archive> [destination] [--sub] [--flatten]")
	}
	source, err := localPath(s.Cwd, raw[0])
	if err != nil {
		return err
	}
	dest, err := localPath(s.Cwd, first(raw[1:], "."))
	if err != nil {
		return err
	}
	subdir := ""
	if o.has("--sub") {
		name := filepath.Base(source)
		h, ok := archives.ForPath(source)
		if ok {
			exts := slices.Clone(h.Extensions())
			slices.SortFunc(exts, func(a, b string) int { return len(b) - len(a) })
			for _, ext := range exts {
				if strings.HasSuffix(strings.ToLower(name), strings.ToLower(ext)) {
					name = name[:len(name)-len(ext)]
					break
				}
			}
		}
		if name == "" || name == "." || name == ".." {
			return fmt.Errorf("invalid extraction folder name: %q", name)
		}
		subdir = name
	}
	a, err := openArchive(source)
	if err != nil {
		return err
	}
	defer a.Close()
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	if subdir != "" {
		if err := root.MkdirAll(subdir, 0755); err != nil {
			return err
		}
		root, err = root.OpenRoot(subdir)
		if err != nil {
			return err
		}
		defer root.Close()
		dest = filepath.Join(dest, subdir)
	}
	var count int
	var size int64
	err = eachArchiveEntry(context.Background(), a, "", func(name string, item vfs.FileInfo) error {
		if item.IsSymlink() || item.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive contains a symbolic link: %s", name)
		}
		if o.has("--flatten") {
			name = path.Base(name)
		}
		target, err := filepath.Localize(name)
		if err != nil {
			return err
		}
		if item.IsDir() {
			if o.has("--flatten") {
				return nil
			}
			return root.MkdirAll(target, 0755)
		}
		if !item.Mode().IsRegular() {
			return fmt.Errorf("archive entry is not a regular file: %s", name)
		}
		if err := root.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		input, err := a.Open(context.Background(), item.Path())
		if err != nil {
			return err
		}
		defer input.Close()
		file, err := root.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		n, copyErr := io.Copy(file, input)
		if err := errors.Join(copyErr, file.Close()); err != nil {
			_ = root.Remove(target)
			return err
		}
		size += n
		count++
		return nil
	})
	if err != nil {
		return fmt.Errorf("extract stopped after %d file(s): %w", count, err)
	}
	return s.emit(map[string]any{"destination": dest, "filesExtracted": count, "bytesExtracted": size}, func() {
		fmt.Fprintf(&s.output, "Extracted %d file(s) (%s) to %s\n", count, humanSize(size), dest)
	})
}

func eachArchiveEntry(ctx context.Context, a archives.Archive, parent string, visit func(string, vfs.FileInfo) error) error {
	items, err := a.ReadDir(ctx, parent)
	if err != nil {
		return err
	}
	for _, item := range items {
		name := item.Name()
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
			return fmt.Errorf("invalid archive entry name: %q", name)
		}
		rel := path.Join(parent, name)
		if err := visit(rel, item); err != nil {
			return err
		}
		if item.IsDir() {
			if err := eachArchiveEntry(ctx, a, rel, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

func workspaces(s *session, args []string) error {
	sub := first(args, "list")
	if len(args) > 1 || (sub != "list" && sub != "active" && sub != "paths") {
		return usageError("usage: ws <list|active|paths>")
	}
	if s.Workspaces == nil {
		return fmt.Errorf("workspace service is unavailable")
	}
	state := s.Workspaces()
	var active *contracts.WorkspaceConfig
	for i := range state.Workspaces {
		if state.Workspaces[i].ID == state.ActiveWorkspaceID {
			active = &state.Workspaces[i]
			break
		}
	}
	if active == nil && len(state.Workspaces) > 0 {
		active = &state.Workspaces[0]
	}
	switch sub {
	case "list":
		return s.emit(state, func() {
			if len(state.Workspaces) == 0 {
				fmt.Fprintln(&s.output, "No saved workspaces.")
				return
			}
			rows := [][]string{{"ACTIVE", "ID", "NAME", "LEFT", "RIGHT"}}
			for _, ws := range state.Workspaces {
				mark := ""
				if active != nil && active.ID == ws.ID {
					mark = "*"
				}
				rows = append(rows, []string{mark, ws.ID, ws.Name, fmt.Sprint(len(ws.LeftPane.Tabs)), fmt.Sprint(len(ws.RightPane.Tabs))})
			}
			s.table(rows)
		})
	case "active":
		return s.emit(map[string]any{"activeWorkspace": active}, func() {
			if active == nil {
				fmt.Fprintln(&s.output, "No active workspace.")
				return
			}
			fmt.Fprintf(&s.output, "%s (%s)\nLeft tabs: %d\nRight tabs: %d\n", active.Name, active.ID, len(active.LeftPane.Tabs), len(active.RightPane.Tabs))
		})
	default:
		payload := map[string]any{"workspaceId": nil, "leftPane": nil, "rightPane": nil}
		var left, right contracts.TabInfo
		if active != nil {
			left, right = activeTab(active.LeftPane), activeTab(active.RightPane)
			payload["workspaceId"], payload["workspaceName"] = active.ID, active.Name
			payload["leftPane"] = map[string]any{"activePath": left.Path, "activeTitle": left.Title, "tabs": active.LeftPane.Tabs}
			payload["rightPane"] = map[string]any{"activePath": right.Path, "activeTitle": right.Title, "tabs": active.RightPane.Tabs}
		}
		return s.emit(payload, func() {
			if active == nil {
				fmt.Fprintln(&s.output, "No active workspace.")
				return
			}
			fmt.Fprintf(&s.output, "%s\n  Left:  %s\n  Right: %s\n", active.Name, left.Path, right.Path)
		})
	}
}

func activeTab(pane contracts.PaneState) contracts.TabInfo {
	for _, t := range pane.Tabs {
		if t.ID == pane.ActiveTabID {
			return t
		}
	}
	for _, t := range pane.Tabs {
		if t.IsActive {
			return t
		}
	}
	if len(pane.Tabs) > 0 {
		return pane.Tabs[0]
	}
	return contracts.TabInfo{}
}

func doctor(s *session, args []string) error {
	if len(args) != 0 {
		return usageError("usage: doctor")
	}
	type check struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	}
	formats := archives.DefaultRegistry.List()
	slices.Sort(formats)
	checks := []check{
		{"Command runner", "ok", "in-process Go (" + runtime.Version() + ")"},
		{"Working directory", "ok", s.Cwd},
		{"Archive formats", "ok", strings.Join(formats, ", ")},
	}
	if s.Workspaces != nil {
		checks = append(checks, check{"Workspace service", "ok", "available"})
	}
	return s.emit(map[string]any{"ok": true, "checks": checks}, func() {
		fmt.Fprintln(&s.output, s.paint("1", "Aether / Reassembly Environment Doctor"))
		rows := [][]string{{"STATUS", "COMPONENT", "DETAIL"}}
		for _, c := range checks {
			rows = append(rows, []string{strings.ToUpper(c.Status), c.Name, c.Detail})
		}
		s.table(rows)
	})
}

func trash(s *session, args []string) error {
	sub := first(args, "list")
	rest := args[min(1, len(args)):]
	switch sub {
	case "list":
		if len(rest) != 0 {
			return usageError("usage: trash list")
		}
		entries, err := listTrash()
		if err != nil {
			return err
		}
		return s.emit(map[string]any{"count": len(entries), "entries": entries}, func() {
			if len(entries) == 0 {
				fmt.Fprintln(&s.output, "Trash is empty.")
				return
			}
			rows := [][]string{{"ID", "NAME", "ORIGINAL PATH"}}
			for _, e := range entries {
				rows = append(rows, []string{e.ID, e.Name, e.OriginalPath})
			}
			s.table(rows)
		})
	case "put":
		if len(rest) == 0 {
			return usageError("usage: trash put <paths...>")
		}
		paths, err := s.paths(rest)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if path == filepath.Dir(path) {
				return fmt.Errorf("cannot trash a filesystem root: %s", path)
			}
			if _, err := os.Lstat(path); err != nil {
				return err
			}
		}
		moved := []string{}
		for _, path := range paths {
			if err := platform.MoveToTrash(path); err != nil {
				return fmt.Errorf("moved %d item(s) before failure: %w", len(moved), err)
			}
			moved = append(moved, path)
		}
		return s.emit(map[string]any{"trashed": moved}, func() {
			for _, path := range moved {
				fmt.Fprintln(&s.output, "Moved to trash:", path)
			}
		})
	case "restore":
		if len(rest) != 1 {
			return usageError("usage: trash restore <id|name>")
		}
		entries, err := listTrash()
		if err != nil {
			return err
		}
		var found *contracts.TrashEntry
		for i := range entries {
			e := &entries[i]
			if e.ID == rest[0] {
				found = e
				break
			}
		}
		if found == nil {
			for i := range entries {
				e := &entries[i]
				if !strings.HasSuffix(e.ID, rest[0]) && e.Name != rest[0] {
					continue
				}
				if found != nil {
					return fmt.Errorf("more than one trash item matches %q; use the full ID", rest[0])
				}
				found = e
			}
		}
		if found == nil {
			return fmt.Errorf("trash entry not found: %s", rest[0])
		}
		dest, err := restoreTrash(*found)
		if err != nil {
			return err
		}
		return s.emit(map[string]any{"restored": found, "destination": dest}, func() { fmt.Fprintln(&s.output, "Restored:", dest) })
	case "empty":
		if len(rest) != 1 || (rest[0] != "--yes" && rest[0] != "-y") {
			return usageError("trash empty requires --yes")
		}
		entries, err := listTrash()
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			if err := emptyTrash(); err != nil {
				return err
			}
		}
		return s.emit(map[string]any{"emptiedCount": len(entries)}, func() { fmt.Fprintf(&s.output, "Emptied %d item(s) from trash.\n", len(entries)) })
	default:
		return usageError("usage: trash <list|put|restore|empty> ...")
	}
}
