package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type entry struct {
	Name  string    `json:"name"`
	Path  string    `json:"path"`
	Type  string    `json:"type"`
	Size  int64     `json:"size"`
	Mtime time.Time `json:"mtime"`
	Mode  string    `json:"mode"`
}

func describe(path string, info fs.FileInfo) entry {
	kind, size := "file", info.Size()
	if info.IsDir() {
		kind, size = "dir", 0
	} else if info.Mode()&fs.ModeSymlink != 0 {
		kind = "symlink"
	}
	return entry{info.Name(), path, kind, size, info.ModTime(), fmt.Sprintf("%03o", info.Mode().Perm())}
}

func list(s *session, args []string) error {
	o, paths, err := parse(args, "-a --all -l --long -al -la --dirs-first --no-dirs-first", "-s --sort")
	if err != nil {
		return err
	}
	root, err := s.root(paths)
	if err != nil {
		return err
	}
	children, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	order := o.value("name", "-s", "--sort")
	if order != "name" && order != "size" && order != "mtime" {
		return usageError("sort must be name, size, or mtime")
	}
	items := make([]entry, 0, len(children))
	for _, child := range children {
		if !o.has("-a", "--all", "-al", "-la") && strings.HasPrefix(child.Name(), ".") {
			continue
		}
		info, err := child.Info()
		if err != nil {
			return err
		}
		items = append(items, describe(filepath.Join(root, child.Name()), info))
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if !o.has("--no-dirs-first") && (a.Type == "dir") != (b.Type == "dir") {
			return a.Type == "dir"
		}
		if order == "size" && a.Size != b.Size {
			return a.Size > b.Size
		}
		if order == "mtime" && !a.Mtime.Equal(b.Mtime) {
			return a.Mtime.After(b.Mtime)
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return s.emit(map[string]any{"path": root, "count": len(items), "entries": items}, func() {
		if len(items) == 0 {
			fmt.Fprintln(&s.output, "(empty directory)")
			return
		}
		long := o.has("-l", "--long", "-al", "-la")
		rows := [][]string{{"TYPE", "MODE", "SIZE", "MODIFIED", "NAME"}}
		for _, item := range items {
			name := item.Name
			switch item.Type {
			case "dir":
				name = s.paint("36", name+"/")
			case "symlink":
				name = s.paint("35", name+"@")
			}
			if long {
				rows = append(rows, []string{item.Type, item.Mode, humanSize(item.Size), item.Mtime.Format("2006-01-02 15:04"), name})
			} else {
				fmt.Fprintln(&s.output, name)
			}
		}
		if long {
			s.table(rows)
		}
	})
}

func humanSize(size int64) string {
	value := float64(size)
	for _, unit := range []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"} {
		if value < 1024 || unit == "PiB" {
			if unit == "B" {
				return fmt.Sprintf("%d B", size)
			}
			return fmt.Sprintf("%.1f %s", value, unit)
		}
		value /= 1024
	}
	return strconv.FormatInt(size, 10)
}

var ignoredDirectories = map[string]bool{".git": true, "node_modules": true, ".angular": true, "dist": true, "wailsjs": true}

func walk(root string, all bool, depth int, skipDependencies bool, visit func(string, fs.DirEntry, int) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path != root && os.IsPermission(err) {
				return nil
			}
			return err
		}
		if path == root && d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		level := strings.Count(rel, string(filepath.Separator)) + 1
		if path == root {
			level = 0
		}
		if path != root && ((!all && strings.HasPrefix(d.Name(), ".")) || (skipDependencies && d.IsDir() && ignoredDirectories[d.Name()])) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if err := visit(path, d, level); err != nil {
			return err
		}
		if depth >= 0 && level >= depth && d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
}

type usageEntry struct {
	Path    string  `json:"path"`
	RelPath string  `json:"relPath"`
	Type    string  `json:"type"`
	Size    int64   `json:"size"`
	Percent float64 `json:"percent"`
}

func diskUsage(s *session, args []string) error {
	o, paths, err := parse(args, "-a --all", "-d --depth --top")
	if err != nil {
		return err
	}
	root, err := s.root(paths)
	if err != nil {
		return err
	}
	depth, err := o.integer(2, "-d", "--depth")
	if err != nil {
		return err
	}
	top, err := o.integer(25, "--top")
	if err != nil {
		return err
	}
	var total int64
	var files, dirs int
	buckets := make(map[string]usageEntry)
	err = walk(root, o.has("-a", "--all"), -1, false, func(path string, d fs.DirEntry, level int) error {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		var size int64
		if d.IsDir() {
			dirs++
		} else if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size = info.Size()
			total += size
			files++
		} else {
			return nil
		}
		// Attribute each file to its displayed ancestors.
		for current := path; current != root; current = filepath.Dir(current) {
			r, err := filepath.Rel(root, current)
			if err != nil {
				return err
			}
			if strings.Count(r, string(filepath.Separator))+1 > depth {
				continue
			}
			item := buckets[r]
			item.Path, item.RelPath = current, r
			item.Type = "dir"
			if current == path && !d.IsDir() {
				item.Type = "file"
			}
			item.Size += size
			buckets[r] = item
		}
		if level == 0 {
			buckets[rel] = usageEntry{path, rel, "file", size, 0}
		}
		return nil
	})
	if err != nil {
		return err
	}
	ranked := make([]usageEntry, 0, len(buckets))
	for _, item := range buckets {
		if total > 0 {
			item.Percent = float64(item.Size) * 100 / float64(total)
		}
		ranked = append(ranked, item)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Size != ranked[j].Size {
			return ranked[i].Size > ranked[j].Size
		}
		return ranked[i].RelPath < ranked[j].RelPath
	})
	ranked = ranked[:min(top, len(ranked))]
	return s.emit(map[string]any{"root": root, "totalBytes": total, "fileCount": files, "dirCount": dirs, "entries": ranked}, func() {
		fmt.Fprintf(&s.output, "%s  %d file(s), %d directorie(s)\n", humanSize(total), files, dirs)
		rows := [][]string{{"SIZE", "%", "PATH"}}
		for _, item := range ranked {
			rows = append(rows, []string{humanSize(item.Size), fmt.Sprintf("%.1f", item.Percent), item.RelPath})
		}
		s.table(rows)
	})
}

type match struct {
	Path           string `json:"path"`
	RelPath        string `json:"relPath"`
	Type           string `json:"type"`
	Size           int64  `json:"size"`
	Line           int    `json:"line,omitempty"`
	Snippet        string `json:"snippet,omitempty"`
	ContentMatches int    `json:"contentMatches,omitempty"`
}

func find(s *session, args []string) error {
	o, paths, err := parse(args, "-a --all -c --case-sensitive --no-recursive", "-p --pattern -g --grep -d --max-depth")
	if err != nil {
		return err
	}
	root, err := s.root(paths)
	if err != nil {
		return err
	}
	depth := -1
	if o.has("-d", "--max-depth") {
		value, err := o.integer(0, "-d", "--max-depth")
		if err != nil {
			return err
		}
		if value < int(^uint(0)>>1) {
			depth = value + 1
		}
	}
	if o.has("--no-recursive") {
		depth = 1
	}
	rawPattern, rawNeedle := o.value("", "-p", "--pattern"), o.value("", "-g", "--grep")
	pattern, needle := rawPattern, rawNeedle
	if pattern == "" {
		pattern = "*"
	}
	if _, err := filepath.Match(pattern, ""); err != nil {
		return usageError(err.Error())
	}
	matchCase := o.has("-c", "--case-sensitive")
	if !matchCase {
		pattern, needle = strings.ToLower(pattern), strings.ToLower(needle)
	}
	const matchLimit = 500
	matches := []match{}
	err = walk(root, o.has("-a", "--all"), depth, true, func(path string, d fs.DirEntry, _ int) error {
		if len(matches) == matchLimit {
			return fs.SkipAll
		}
		if !d.IsDir() && !d.Type().IsRegular() {
			return nil
		}
		name := d.Name()
		if !matchCase {
			name = strings.ToLower(name)
		}
		selected, _ := filepath.Match(pattern, name)
		if !selected || (d.IsDir() && needle != "") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			rel = d.Name()
		}

		result := match{Path: path, RelPath: rel, Type: "dir"}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			result.Type, result.Size = "file", info.Size()
		}
		if needle != "" {
			err := grepFile(&result, needle, matchCase)
			if os.IsPermission(err) || os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if result.ContentMatches == 0 {
				return nil
			}
		}
		matches = append(matches, result)
		return nil
	})
	if err != nil {
		return err
	}
	return s.emit(map[string]any{"root": root, "pattern": stringOrNil(rawPattern), "grep": stringOrNil(rawNeedle), "count": len(matches), "matches": matches}, func() {
		for _, m := range matches {
			switch {
			case m.Line != 0:
				fmt.Fprintf(&s.output, "%s:L%d (%dx) %s\n", m.RelPath, m.Line, m.ContentMatches, m.Snippet)
			case m.Type == "dir":
				fmt.Fprintln(&s.output, s.paint("36", m.RelPath+"/"))
			default:
				fmt.Fprintf(&s.output, "%s (%s)\n", m.RelPath, humanSize(m.Size))
			}
		}
		if len(matches) == 0 {
			fmt.Fprintln(&s.output, "No matches.")
		}
	})
}

func stringOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func grepFile(result *match, needle string, matchCase bool) error {
	const limit = 2 << 20
	if result.Size == 0 || result.Size > limit {
		return nil
	}
	f, _, err := openRegular(result.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return err
	}
	if len(data) > limit || bytes.IndexByte(data[:min(len(data), 4096)], 0) >= 0 {
		return nil
	}
	for i, text := range strings.Split(string(data), "\n") {
		haystack := text
		if !matchCase {
			haystack = strings.ToLower(text)
		}
		if !strings.Contains(haystack, needle) {
			continue
		}
		if result.ContentMatches == 0 {
			result.Line = i + 1
			snippet := []rune(strings.TrimSpace(text))
			result.Snippet = string(snippet[:min(140, len(snippet))])
		}
		result.ContentMatches++
	}
	return nil
}
