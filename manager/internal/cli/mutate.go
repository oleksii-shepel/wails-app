package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var mutationLock sync.Mutex

type part struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

func split(s *session, args []string) error {
	o, raw, err := parse(args, "", "-s --size -o --out-dir")
	if err != nil {
		return err
	}
	if len(raw) != 1 {
		return usageError("split requires one input file")
	}
	size, err := parseSize(o.value("1M", "-s", "--size"))
	if err != nil {
		return err
	}
	size = max(size, 1024)
	source, err := localPath(s.Cwd, raw[0])
	if err != nil {
		return err
	}
	dir, err := localPath(s.Cwd, o.value(filepath.Dir(source), "-o", "--out-dir"))
	if err != nil {
		return err
	}
	input, info, err := openRegular(source)
	if err != nil {
		return err
	}
	defer input.Close()
	mutationLock.Lock()
	defer mutationLock.Unlock()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	parts := []part{}
	committed := false
	defer func() {
		if !committed {
			for _, p := range parts {
				_ = os.Remove(p.Path)
			}
		}
	}()
	total := info.Size()
	for i, left := 1, total; i == 1 || left > 0; i++ {
		dest := filepath.Join(dir, fmt.Sprintf("%s.part%03d", filepath.Base(source), i))
		length := min(left, size)
		if err := writeNewFile(dest, func(f *os.File) error {
			_, err := io.CopyN(f, input, length)
			return err
		}); err != nil {
			return err
		}
		parts = append(parts, part{dest, length})
		left -= length
	}
	committed = true
	return s.emit(map[string]any{"source": source, "totalBytes": total, "partSize": size, "parts": parts}, func() {
		fmt.Fprintf(&s.output, "Split %s (%s) into %d part(s):\n", filepath.Base(source), humanSize(total), len(parts))
		for _, p := range parts {
			fmt.Fprintf(&s.output, "  %s (%s)\n", p.Path, humanSize(p.Bytes))
		}
	})
}

// Stage the entire output before publishing it so incomplete files stay hidden.
func writeNewFile(dest string, write func(*os.File) error) error {
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("destination exists: %s", dest)
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dest), ".aether-part-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = errors.Join(write(f), f.Close())
	if err != nil {
		return err
	}
	return moveNew(f.Name(), dest)
}

var partSuffix = regexp.MustCompile("(?i)\\.part[0-9]{3,}$")

func join(s *session, args []string) error {
	o, raw, err := parse(args, "", "-o --output")
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return usageError("join requires part files")
	}
	parts, err := s.paths(raw)
	if err != nil {
		return err
	}
	mutationLock.Lock()
	defer mutationLock.Unlock()
	if len(parts) == 1 && partSuffix.MatchString(parts[0]) {
		parts, err = discoverParts(parts[0])
		if err != nil {
			return err
		}
	}
	if len(parts) < 2 {
		return usageError("join requires at least two parts")
	}
	defaultDest := partSuffix.ReplaceAllString(parts[0], "") + ".joined"
	dest, err := localPath(s.Cwd, o.value(defaultDest, "-o", "--output"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	var size int64
	err = writeNewFile(dest, func(out *os.File) error {
		for _, p := range parts {
			input, _, err := openRegular(p)
			if err != nil {
				return err
			}
			n, err := io.Copy(out, input)
			size += n
			if err := errors.Join(err, input.Close()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.emit(map[string]any{"parts": parts, "destination": dest, "bytes": size}, func() {
		fmt.Fprintf(&s.output, "Joined %d parts -> %s (%s)\n", len(parts), dest, humanSize(size))
	})
}

func discoverParts(first string) ([]string, error) {
	if _, err := os.Stat(first); err != nil {
		return nil, err
	}
	dir := filepath.Dir(first)
	stem := partSuffix.ReplaceAllString(filepath.Base(first), "")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, d := range entries {
		if partSuffix.ReplaceAllString(d.Name(), "") != stem {
			continue
		}
		suffix := partSuffix.FindString(d.Name())
		if suffix == "" {
			continue
		}
		n, err := strconv.Atoi(suffix[len(".part"):])
		if err != nil {
			return nil, err
		}
		if _, duplicate := names[n]; duplicate {
			return nil, fmt.Errorf("duplicate part number: %d", n)
		}
		names[n] = filepath.Join(dir, d.Name())
	}
	numbers := make([]int, 0, len(names))
	for n := range names {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	parts := make([]string, 0, len(numbers))
	for i, n := range numbers {
		if n != i+1 {
			return nil, fmt.Errorf("missing part %03d", i+1)
		}
		parts = append(parts, names[n])
	}
	return parts, nil
}

type renameItem struct {
	Source  string `json:"src"`
	Dest    string `json:"dest"`
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
	Changed bool   `json:"changed"`
}

var renameToken = regexp.MustCompile("(?i)\\{(name|ext|n|nnn|full)\\}")

func rename(s *session, args []string) error {
	o, raw, err := parse(args, "--regex --match-case --preserve-ext --no-preserve-ext --apply --dry-run", "--pattern --find --replace --start --step")
	if err != nil {
		return err
	}
	pattern, search := o.value("", "--pattern"), o.value("", "--find")
	if len(raw) == 0 || (pattern == "" && search == "") {
		return usageError("rename requires sources and --pattern or --find")
	}
	start, err := o.integer(1, "--start")
	if err != nil {
		return err
	}
	step, err := o.integer(1, "--step")
	if err != nil {
		return err
	}
	var re *regexp.Regexp
	if pattern == "" {
		if !o.has("--regex") {
			search = regexp.QuoteMeta(search)
		}
		if !o.has("--match-case") {
			search = "(?i)" + search
		}
		re, err = regexp.Compile(search)
		if err != nil {
			return usageError(err.Error())
		}
	}
	preserve := !o.has("--no-preserve-ext")
	paths, err := s.paths(raw)
	if err != nil {
		return err
	}
	apply := o.has("--apply") && !o.has("--dry-run")
	mutationLock.Lock()
	defer mutationLock.Unlock()
	plan := make([]renameItem, 0, len(paths))
	var count int
	for i, src := range paths {
		if _, err := os.Lstat(src); err != nil {
			return err
		}
		old := filepath.Base(src)
		suffix := filepath.Ext(old)
		if suffix == old {
			suffix = ""
		}
		stem := strings.TrimSuffix(old, suffix)
		var name string
		if pattern != "" {
			if i > 0 && step > (int(^uint(0)>>1)-start)/i {
				return usageError("rename sequence overflows")
			}
			number := start + i*step
			tokens := map[string]string{"{name}": stem, "{ext}": strings.TrimPrefix(suffix, "."), "{n}": strconv.Itoa(number), "{nnn}": fmt.Sprintf("%03d", number), "{full}": old}
			name = renameToken.ReplaceAllStringFunc(pattern, func(token string) string { return tokens[strings.ToLower(token)] })
			if preserve && !strings.Contains(strings.ToLower(pattern), "{ext}") && !strings.HasSuffix(name, suffix) {
				name += suffix
			}
		} else {
			input, extension := old, ""
			if preserve {
				input, extension = stem, suffix
			}
			replacement := o.value("", "--replace")
			if o.has("--regex") {
				name = re.ReplaceAllString(input, replacement)
			} else {
				name = re.ReplaceAllStringFunc(input, func(string) string { return replacement })
			}
			name += extension
		}
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") || (runtime.GOOS == "windows" && (strings.Contains(name, ":") || strings.TrimRight(name, ". ") != name)) {
			return usageError("rename requires a valid filename: " + name)
		}
		changed := name != old
		plan = append(plan, renameItem{src, filepath.Join(filepath.Dir(src), name), old, name, changed})
		if changed {
			count++
		}
	}
	if err := validateRename(plan); err != nil {
		return err
	}
	if apply {
		if err := applyRename(plan); err != nil {
			return err
		}
	}
	return s.emit(map[string]any{"applied": apply, "count": count, "items": plan}, func() {
		for _, item := range plan {
			fmt.Fprintf(&s.output, "%s -> %s\n", item.OldName, item.NewName)
		}
		if !apply {
			fmt.Fprintln(&s.output, "Dry run. Pass --apply to rename.")
		}
	})
}

func renameKey(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func validateRename(plan []renameItem) error {
	sources, targets := map[string]bool{}, map[string]bool{}
	for _, item := range plan {
		src, dest := renameKey(item.Source), renameKey(item.Dest)
		if sources[src] || targets[dest] {
			return fmt.Errorf("rename collision: %s", item.Dest)
		}
		sources[src], targets[dest] = true, true
	}
	for _, item := range plan {
		for parent := filepath.Dir(item.Source); parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
			if sources[renameKey(parent)] {
				return fmt.Errorf("rename sources overlap: %s", item.Source)
			}
		}
		if sources[renameKey(item.Dest)] {
			continue
		}
		if _, err := os.Lstat(item.Dest); err == nil {
			return fmt.Errorf("destination exists: %s", item.Dest)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func applyRename(plan []renameItem) (err error) {
	type staged struct {
		item      renameItem
		temp      string
		committed bool
	}
	stages := []staged{}
	defer func() {
		if err == nil {
			for _, s := range stages {
				_ = os.Remove(filepath.Dir(s.temp))
			}
			return
		}
		for i := len(stages) - 1; i >= 0; i-- {
			s := &stages[i]
			if s.committed {
				if undo := moveNew(s.item.Dest, s.temp); undo != nil {
					err = errors.Join(err, undo)
				}
			}
		}
		for i := len(stages) - 1; i >= 0; i-- {
			s := stages[i]
			if undo := moveNew(s.temp, s.item.Source); undo != nil {
				err = errors.Join(err, undo)
			}
			_ = os.Remove(filepath.Dir(s.temp))
		}
	}()
	for _, item := range plan {
		if !item.Changed {
			continue
		}
		dir, err := os.MkdirTemp(filepath.Dir(item.Source), ".aether-rename-*")
		if err != nil {
			return err
		}
		temp := filepath.Join(dir, "item")
		if err := os.Rename(item.Source, temp); err != nil {
			_ = os.Remove(dir)
			return err
		}
		stages = append(stages, staged{item: item, temp: temp})
	}
	for i := range stages {
		stage := &stages[i]
		if _, err := os.Lstat(stage.item.Dest); err == nil {
			return fmt.Errorf("destination exists: %s", stage.item.Dest)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := moveNew(stage.temp, stage.item.Dest); err != nil {
			return err
		}
		stage.committed = true
	}
	return nil
}
