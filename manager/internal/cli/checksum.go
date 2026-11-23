package cli

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var algorithms = map[string]func() hash.Hash{
	"md5": md5.New, "sha1": sha1.New, "sha256": sha256.New, "sha512": sha512.New,
}

func openRegular(path string) (*os.File, fs.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("not a regular file: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, info, nil
}

func digest(path string, names ...string) (map[string]string, int64, error) {
	f, info, err := openRegular(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	hashers := make([]hash.Hash, len(names))
	writers := make([]io.Writer, len(names))
	for i, name := range names {
		hashers[i] = algorithms[name]()
		writers[i] = hashers[i]
	}
	if _, err := io.Copy(io.MultiWriter(writers...), f); err != nil {
		return nil, 0, err
	}
	sums := make(map[string]string, len(names))
	for i, h := range hashers {
		sums[names[i]] = hex.EncodeToString(h.Sum(nil))
	}
	return sums, info.Size(), nil
}

func hashes(s *session, args []string) error {
	o, raw, err := parse(args, "--manifest", "--algo")
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return usageError("hash requires at least one file")
	}
	algo := strings.ToLower(o.value("sha256", "--algo"))
	if algorithms[algo] == nil && algo != "all" {
		return usageError("unknown hash algorithm: " + algo)
	}
	if o.has("--manifest") {
		algo = "sha256"
	}
	names := []string{algo}
	if algo == "all" {
		names = []string{"sha256", "sha512", "sha1", "md5"}
	}
	paths, err := s.paths(raw)
	if err != nil {
		return err
	}
	type file struct {
		Path   string            `json:"path"`
		Name   string            `json:"name"`
		Size   int64             `json:"size"`
		Hashes map[string]string `json:"hashes"`
	}
	files := make([]file, 0, len(paths))
	for i, path := range paths {
		sums, size, err := digest(path, names...)
		if err != nil {
			return err
		}
		files = append(files, file{path, raw[i], size, sums})
	}
	return s.emit(map[string]any{"algo": algo, "files": files}, func() {
		for _, f := range files {
			if len(names) == 1 {
				sum := f.Hashes[algo]
				if !o.has("--manifest") {
					sum = s.paint("36", sum)
				}
				fmt.Fprintf(&s.output, "%s  %s\n", sum, f.Name)
				continue
			}
			fmt.Fprintln(&s.output, f.Name)
			for _, name := range names {
				fmt.Fprintf(&s.output, "  %-6s %s\n", name, f.Hashes[name])
			}
		}
	})
}

var manifestLine = regexp.MustCompile("^([0-9a-fA-F]+)[ \t]+\\*?(.+)$")
var digestByLength = map[int]string{32: "md5", 40: "sha1", 64: "sha256", 128: "sha512"}

func verify(s *session, args []string) error {
	o, raw, err := parse(args, "", "--root")
	if err != nil {
		return err
	}
	if len(raw) != 1 {
		return usageError("verify requires one manifest")
	}
	path, err := localPath(s.Cwd, raw[0])
	if err != nil {
		return err
	}
	root, err := localPath(s.Cwd, o.value(filepath.Dir(path), "--root"))
	if err != nil {
		return err
	}
	f, _, err := openRegular(path)
	if err != nil {
		return err
	}
	defer f.Close()
	items := []map[string]any{}
	var passed, failed, missing int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := manifestLine.FindStringSubmatch(line)
		if len(fields) != 3 || digestByLength[len(fields[1])] == "" {
			return fmt.Errorf("malformed manifest line: %s", line)
		}
		expected, name := strings.ToLower(fields[1]), strings.TrimSpace(fields[2])
		algo := digestByLength[len(expected)]
		target, err := localPath(root, name)
		if err != nil {
			return err
		}
		sums, _, err := digest(target, algo)
		item := map[string]any{"file": name, "algo": algo, "expected": expected, "actual": nil}
		switch {
		case os.IsNotExist(err):
			missing++
			item["status"] = "missing"
		case err != nil:
			return err
		case sums[algo] == expected:
			passed++
			item["status"], item["actual"] = "ok", sums[algo]
		default:
			failed++
			item["status"], item["actual"] = "mismatch", sums[algo]
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	ok := failed+missing == 0
	if !ok {
		s.ExitCode = 1
	}
	return s.emit(map[string]any{"manifest": path, "baseDir": root, "ok": ok, "passed": passed, "failed": failed, "missing": missing, "entries": items}, func() {
		for _, item := range items {
			fmt.Fprintf(&s.output, "%s  %s\n", strings.ToUpper(item["status"].(string)), item["file"])
		}
		fmt.Fprintf(&s.output, "%d passed, %d mismatched, %d missing\n", passed, failed, missing)
	})
}

var sizePattern = regexp.MustCompile("(?i)^([0-9]+(?:\\.[0-9]+)?)\\s*([kmgtp]?)(?:i?b)?$")

func parseSize(raw string) (int64, error) {
	parts := sizePattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(parts) != 3 {
		return 0, usageError("invalid size: " + raw)
	}
	value, err := strconv.ParseFloat(parts[1], 64)
	for _, unit := range []string{"k", "m", "g", "t", "p"} {
		if parts[2] == "" {
			break
		}
		value *= 1024
		if strings.EqualFold(parts[2], unit) {
			break
		}
	}
	if err != nil || value >= float64(1<<63) {
		return 0, usageError("size is out of range: " + raw)
	}
	return int64(value), nil
}

type duplicateFile struct {
	Path string `json:"path"`
	Rel  string `json:"relPath"`
}

type duplicateGroup struct {
	SHA256      string          `json:"sha256"`
	Size        int64           `json:"size"`
	Reclaimable int64           `json:"reclaimableBytes"`
	Files       []duplicateFile `json:"files"`
}

func duplicates(s *session, args []string) error {
	o, paths, err := parse(args, "-a --all", "--min-size")
	if err != nil {
		return err
	}
	root, err := s.root(paths)
	if err != nil {
		return err
	}
	minSize, err := parseSize(o.value("1", "--min-size"))
	if err != nil {
		return err
	}
	buckets := map[int64][]duplicateFile{}
	var scanned int
	err = walk(root, o.has("-a", "--all"), -1, true, func(path string, d fs.DirEntry, _ int) error {
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() < minSize {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		buckets[info.Size()] = append(buckets[info.Size()], duplicateFile{path, rel})
		scanned++
		return nil
	})
	if err != nil {
		return err
	}
	groups := []duplicateGroup{}
	var reclaim int64
	for size, files := range buckets {
		if len(files) < 2 {
			continue
		}
		byHash := map[string][]duplicateFile{}
		for _, file := range files {
			sums, actualSize, err := digest(file.Path, "sha256")
			if err != nil {
				return err
			}
			if actualSize != size {
				continue
			}
			sum := sums["sha256"]
			byHash[sum] = append(byHash[sum], file)
		}
		for sum, matches := range byHash {
			if len(matches) < 2 {
				continue
			}
			waste := int64(len(matches)-1) * size
			reclaim += waste
			groups = append(groups, duplicateGroup{sum, size, waste, matches})
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Reclaimable != groups[j].Reclaimable {
			return groups[i].Reclaimable > groups[j].Reclaimable
		}
		if groups[i].Size != groups[j].Size {
			return groups[i].Size > groups[j].Size
		}
		return groups[i].SHA256 < groups[j].SHA256
	})
	return s.emit(map[string]any{"root": root, "scannedFiles": scanned, "groupCount": len(groups), "reclaimableBytes": reclaim, "groups": groups}, func() {
		fmt.Fprintf(&s.output, "%d duplicate group(s) across %d files, %s reclaimable\n", len(groups), scanned, humanSize(reclaim))
		for _, g := range groups {
			for _, f := range g.Files {
				fmt.Fprintf(&s.output, "  %s  %s\n", g.SHA256[:12], f.Rel)
			}
		}
	})
}
