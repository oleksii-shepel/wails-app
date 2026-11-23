package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

func compare(s *session, args []string) error {
	o, raw, err := parse(args, "-r --recursive --hash", "")
	if err != nil {
		return err
	}
	if len(raw) != 2 {
		return usageError("compare requires two directories")
	}
	paths, err := s.paths(raw)
	if err != nil {
		return err
	}
	depth := 1
	if o.has("-r", "--recursive") {
		depth = -1
	}
	maps := [2]map[string]entry{}
	keys := map[string]bool{}
	for i, root := range paths {
		if err := checkDirectory(root); err != nil {
			return err
		}
		items := map[string]entry{}
		err := walk(root, true, depth, true, func(path string, d fs.DirEntry, _ int) error {
			if d.IsDir() && d.Name() == ".git" {
				return fs.SkipDir
			}
			if !d.IsDir() && !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			items[rel] = describe(path, info)
			keys[rel] = true
			return nil
		})
		if err != nil {
			return err
		}
		maps[i] = items
	}
	names := make([]string, 0, len(keys))
	for key := range keys {
		names = append(names, key)
	}
	sort.Strings(names)
	leftOnly, rightOnly, same := []string{}, []string{}, []string{}
	modified := []map[string]any{}
	for _, name := range names {
		l, inLeft := maps[0][name]
		r, inRight := maps[1][name]
		switch {
		case !inLeft:
			rightOnly = append(rightOnly, name)
		case !inRight:
			leftOnly = append(leftOnly, name)
		case l.Type != r.Type:
			modified = append(modified, map[string]any{"rel": name, "reason": "type-mismatch"})
		case l.Size != r.Size:
			modified = append(modified, map[string]any{"rel": name, "reason": "size", "leftSize": l.Size, "rightSize": r.Size})
		case l.Type == "file" && o.has("--hash"):
			lSum, _, err := digest(l.Path, "sha256")
			if err != nil {
				return err
			}
			rSum, _, err := digest(r.Path, "sha256")
			if err != nil {
				return err
			}
			if lSum["sha256"] != rSum["sha256"] {
				modified = append(modified, map[string]any{"rel": name, "reason": "content"})
			} else {
				same = append(same, name)
			}
		default:
			same = append(same, name)
		}
	}
	identical := len(modified)+len(leftOnly)+len(rightOnly) == 0
	if !identical {
		s.ExitCode = 1
	}
	return s.emit(map[string]any{"left": paths[0], "right": paths[1], "identical": identical, "onlyLeft": leftOnly, "onlyRight": rightOnly, "modified": modified, "same": same}, func() {
		fmt.Fprintf(&s.output, "%d identical, %d modified, %d only-left, %d only-right\n", len(same), len(modified), len(leftOnly), len(rightOnly))
		for _, key := range leftOnly {
			fmt.Fprintln(&s.output, s.paint("36", "  < "+key))
		}
		for _, key := range rightOnly {
			fmt.Fprintln(&s.output, s.paint("35", "  > "+key))
		}
		for _, item := range modified {
			fmt.Fprintf(&s.output, "  ~ %s (%s)\n", item["rel"], item["reason"])
		}
	})
}

type diffLine struct {
	Kind string `json:"type"`
	Text string `json:"text"`
}

type diffHunk struct {
	LeftStart  int        `json:"leftStart"`
	RightStart int        `json:"rightStart"`
	LeftCount  int        `json:"leftCount"`
	RightCount int        `json:"rightCount"`
	Lines      []diffLine `json:"lines"`
}

func diff(s *session, args []string) error {
	o, raw, err := parse(args, "-u --unified --full -w --ignore-whitespace -i --ignore-case --stat", "-C --context")
	if err != nil {
		return err
	}
	if len(raw) != 2 {
		return usageError("diff requires two files")
	}
	context, err := o.integer(3, "-C", "--context")
	if err != nil {
		return err
	}
	paths, err := s.paths(raw)
	if err != nil {
		return err
	}
	var data [2][]byte
	for i, path := range paths {
		data[i], err = readDiffFile(path)
		if err != nil {
			return err
		}
	}
	result := map[string]any{"left": paths[0], "right": paths[1]}
	if bytes.IndexByte(data[0], 0) >= 0 || bytes.IndexByte(data[1], 0) >= 0 {
		same := bytes.Equal(data[0], data[1])
		if !same {
			s.ExitCode = 1
		}
		result["binary"], result["identical"] = true, same
		return s.emit(result, func() {
			if same {
				fmt.Fprintln(&s.output, "Binary files are identical.")
			} else {
				fmt.Fprintln(&s.output, "Binary files differ.")
			}
		})
	}
	lines := func(data []byte) []string {
		if len(data) == 0 {
			return nil
		}
		return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	}
	left, right := lines(data[0]), lines(data[1])
	ops, err := lineDiff(left, right, o)
	if err != nil {
		return err
	}
	var added, removed, equal int
	for _, op := range ops {
		switch op.Kind {
		case "insert":
			added++
		case "delete":
			removed++
		default:
			equal++
		}
	}
	same := added+removed == 0
	if !same {
		s.ExitCode = 1
	}
	result["identical"] = same
	result["stats"] = map[string]int{"added": added, "removed": removed, "equal": equal, "leftLines": len(left), "rightLines": len(right)}
	var hunks []diffHunk
	if !o.has("--stat") {
		hunks = buildHunks(ops, context, o.has("--full") && !s.json)
		result["hunks"] = hunks
	}
	return s.emit(result, func() {
		if same {
			fmt.Fprintln(&s.output, "Files are identical.")
			return
		}
		if o.has("--stat") {
			fmt.Fprintf(&s.output, "%d insertion(s), %d deletion(s)\n", added, removed)
			return
		}
		fmt.Fprintf(&s.output, "--- %s\n+++ %s\n", paths[0], paths[1])
		for _, h := range hunks {
			fmt.Fprintf(&s.output, "@@ -%d,%d +%d,%d @@\n", h.LeftStart, h.LeftCount, h.RightStart, h.RightCount)
			for _, op := range h.Lines {
				switch op.Kind {
				case "insert":
					fmt.Fprintln(&s.output, s.paint("32", "+"+op.Text))
				case "delete":
					fmt.Fprintln(&s.output, s.paint("31", "-"+op.Text))
				default:
					fmt.Fprintln(&s.output, " "+op.Text)
				}
			}
		}
	})
}

func readDiffFile(path string) ([]byte, error) {
	f, _, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const limit = 16 << 20
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && len(data) > limit {
		return nil, fmt.Errorf("file is too large for diff: %s", path)
	}
	return data, err
}

func lineDiff(left, right []string, o options) ([]diffLine, error) {
	normalize := func(lines []string) []string {
		result := make([]string, len(lines))
		for i, line := range lines {
			if o.has("-w", "--ignore-whitespace") {
				line = strings.Join(strings.Fields(line), " ")
			}
			if o.has("-i", "--ignore-case") {
				line = strings.ToLower(line)
			}
			result[i] = line
		}
		return result
	}
	l, r := normalize(left), normalize(right)
	prefix, suffix := 0, 0
	for prefix < min(len(l), len(r)) && l[prefix] == r[prefix] {
		prefix++
	}
	for suffix < min(len(l), len(r))-prefix && l[len(l)-1-suffix] == r[len(r)-1-suffix] {
		suffix++
	}
	l, r = l[prefix:len(l)-suffix], r[prefix:len(r)-suffix]
	width := len(r) + 1
	if len(l) > 0 && len(r) > 0 && len(l)+1 > 64_000_000/width {
		return nil, fmt.Errorf("files are too large for line-by-line diff")
	}
	var dp []uint32
	if len(l) > 0 && len(r) > 0 {
		dp = make([]uint32, (len(l)+1)*width)
		for i := len(l) - 1; i >= 0; i-- {
			for j := len(r) - 1; j >= 0; j-- {
				k := i*width + j
				dp[k] = max(dp[k+1], dp[k+width])
				if l[i] == r[j] {
					dp[k] = dp[k+width+1] + 1
				}
			}
		}
	}
	ops := make([]diffLine, 0, max(len(left), len(right)))
	for _, line := range left[:prefix] {
		ops = append(ops, diffLine{"equal", line})
	}
	for i, j := 0, 0; i < len(l) || j < len(r); {
		switch {
		case i < len(l) && j < len(r) && l[i] == r[j]:
			ops = append(ops, diffLine{"equal", left[prefix+i]})
			i++
			j++
		case j == len(r) || (i < len(l) && dp[(i+1)*width+j] >= dp[i*width+j+1]):
			ops = append(ops, diffLine{"delete", left[prefix+i]})
			i++
		default:
			ops = append(ops, diffLine{"insert", right[prefix+j]})
			j++
		}
	}
	for _, line := range left[len(left)-suffix:] {
		ops = append(ops, diffLine{"equal", line})
	}
	return ops, nil
}

func buildHunks(ops []diffLine, context int, full bool) []diffHunk {
	type span struct{ start, end int }
	ranges := []span{}
	context = min(context, len(ops))
	for i, op := range ops {
		if op.Kind == "equal" {
			continue
		}
		start, end := max(0, i-context), min(len(ops), i+context+1)
		last := len(ranges) - 1
		if last >= 0 && ranges[last].end >= start {
			ranges[last].end = end
		} else {
			ranges = append(ranges, span{start, end})
		}
	}
	if full && len(ranges) > 0 {
		ranges = []span{{0, len(ops)}}
	}
	hunks := make([]diffHunk, 0, len(ranges))
	left, right, cursor := 1, 1, 0
	advance := func(until int) {
		for ; cursor < until; cursor++ {
			if ops[cursor].Kind != "insert" {
				left++
			}
			if ops[cursor].Kind != "delete" {
				right++
			}
		}
	}
	for _, r := range ranges {
		advance(r.start)
		l, n := left, right
		advance(r.end)
		h := diffHunk{l, n, left - l, right - n, ops[r.start:r.end]}
		if h.LeftCount == 0 {
			h.LeftStart--
		}
		if h.RightCount == 0 {
			h.RightStart--
		}
		hunks = append(hunks, h)
	}
	return hunks
}
