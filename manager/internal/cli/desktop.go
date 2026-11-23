package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"wails-app/internal/contracts"
	"wails-app/internal/pathref"
)

type Result struct {
	OK         bool   `json:"ok"`
	Cwd        string `json:"cwd"`
	ChangedCwd bool   `json:"changedCwd"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exitCode"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

type Services struct {
	Workspaces func() contracts.WorkspaceList
	CreateZip  func([]string, string) (*contracts.OperationResult, error)
}

type session struct {
	Services
	Result
	json, color bool
	output      strings.Builder
}

type command struct {
	usage string
	run   func(*session, []string) error
}

func commands() map[string]command {
	// Constructed here so help can use the same command definitions.
	return map[string]command{
		"ls":      {"ls [path] [-al] [--sort name|size|mtime]", list},
		"du":      {"du [path] [-a] [--depth N] [--top N]", diskUsage},
		"find":    {"find [path] [-p glob] [-g text] [-c] [-d N] [--no-recursive] [-a]", find},
		"dup":     {"dup [path] [--min-size 1K] [-a]", duplicates},
		"hash":    {"hash <files...> [--algo sha256|sha512|sha1|md5|all] [--manifest]", hashes},
		"verify":  {"verify <manifest> [--root directory]", verify},
		"diff":    {"diff <left> <right> [--unified|--full] [-w] [-i] [--stat] [--context N]", diff},
		"compare": {"compare <left> <right> [-r] [--hash]", compare},
		"split":   {"split <file> [--size 1M] [--out-dir directory]", split},
		"join":    {"join <part001 | parts...> [--output path]", join},
		"rename":  {"rename <paths...> (--pattern pattern | --find text --replace text) [--apply]", rename},
		"archive": {"archive <list|zip|extract> ...", archive},
		"trash":   {"trash <list|put|restore|empty> ...", trash},
		"ws":      {"ws <list|active|paths>", workspaces},
		"doctor":  {"doctor", doctor},
		"cd":      {"cd [directory]", changeDirectory},
		"pwd":     {"pwd", printDirectory},
	}
}

var aliases = map[string]string{
	"disk-usage": "du", "search": "find", "duplicates": "dup", "checksum": "hash",
	"batch-rename": "rename", "workspace": "ws",
}

func (services Services) Execute(line, cwd string, color bool) Result {
	start := time.Now()
	s := &session{Services: services, Result: Result{Cwd: cwd}, color: color}
	if err := s.execute(line); err != nil {
		s.ExitCode = 1
		var usage usageError
		if errors.As(err, &usage) {
			s.ExitCode = 2
		}
		s.Error = err.Error()
		s.Stderr = s.Error + "\n"
	}
	s.OK = s.ExitCode == 0
	s.Stdout = s.output.String()
	s.DurationMs = time.Since(start).Milliseconds()
	return s.Result
}

func (s *session) execute(line string) error {
	args, err := tokenize(strings.TrimSpace(line))
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return nil
	}
	if args[0] == "aether" || args[0] == "reassembly" {
		args = args[1:]
	}

	if s.Cwd == "" {
		s.Cwd, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}
	s.Cwd, err = localPath("", s.Cwd)
	if err != nil {
		return err
	}

	var name string
	var rest []string
	var help, version bool
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			s.json = true
		case arg == "--no-color":
			s.color = false
		case arg == "-h" || arg == "--help":
			help = true
		case name == "" && (arg == "-v" || arg == "--version"):
			version = true
		case name == "" && (arg == "--cwd" || strings.HasPrefix(arg, "--cwd=")):
			value := strings.TrimPrefix(arg, "--cwd=")
			if arg == "--cwd" {
				i++
				if i == len(args) {
					return usageError("--cwd requires a directory")
				}
				value = args[i]
			}
			s.Cwd, err = localPath(s.Cwd, value)
			if err != nil {
				return err
			}
		case name == "" && strings.HasPrefix(arg, "-"):
			return usageError("unknown global option: " + arg)
		case name == "":
			name = arg
		case arg == "--":
			rest = append(rest, args[i:]...)
			i = len(args)
		default:
			rest = append(rest, arg)
		}
	}
	if version {
		return s.emit(map[string]string{"name": "aether", "version": "0.1.0"}, func() {
			fmt.Fprintln(&s.output, "aether v0.1.0")
		})
	}
	if name == "help" {
		help = true
		name = ""
		if len(rest) > 0 {
			name = rest[0]
		}
	}
	if canonical, ok := aliases[name]; ok {
		name = canonical
	}
	if name == "" {
		s.help()
		return nil
	}
	cmd, ok := commands()[name]
	if !ok {
		return usageError("unknown command: " + name)
	}
	if help {
		fmt.Fprintln(&s.output, "Usage: aether "+cmd.usage)
		return nil
	}
	if err := checkDirectory(s.Cwd); err != nil {
		return err
	}
	return cmd.run(s, rest)
}

func (s *session) help() {
	fmt.Fprintf(&s.output, "%s\n\n", s.paint("1", "Aether CLI"))
	fmt.Fprint(&s.output, "Usage: aether [--cwd directory] [--json] [--no-color] <command>\n\n")
	all := commands()
	for _, name := range strings.Fields("ls du find dup hash verify diff compare split join rename archive trash ws doctor cd pwd") {
		fmt.Fprintln(&s.output, "  "+all[name].usage)
	}
}

func (s *session) paint(code, text string) string {
	if s.color && !s.json {
		return "\x1b[" + code + "m" + text + "\x1b[0m"
	}
	return text
}

func (s *session) emit(value any, render func()) error {
	if s.json {
		enc := json.NewEncoder(&s.output)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	render()
	return nil
}

func (s *session) table(rows [][]string) {
	w := tabwriter.NewWriter(&s.output, 0, 4, 2, ' ', 0)
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	_ = w.Flush()
}

func localPath(cwd, raw string) (string, error) {
	if strings.HasPrefix(strings.ToLower(raw), "filesystem://") {
		rest := strings.TrimLeft(raw[len("filesystem://"):], "/")
		if runtime.GOOS != "windows" {
			if len(rest) >= 1 && len(strings.SplitN(rest, "/", 2)[0]) == 1 {
				rest = strings.TrimPrefix(rest[1:], "/")
			}
			raw = "/" + rest
		} else {
			raw = pathref.ToNativePath(raw)
		}
	}
	if strings.Contains(raw, "://") {
		return "", fmt.Errorf("a local filesystem path is required: %s", raw)
	}
	if raw == "~" || strings.HasPrefix(raw, "~/") || (runtime.GOOS == "windows" && strings.HasPrefix(raw, "~\\")) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		raw = filepath.Join(home, strings.TrimLeft(raw[1:], "/\\"))
	}
	if !filepath.IsAbs(raw) {
		if runtime.GOOS == "windows" && (strings.HasPrefix(raw, "\\") || strings.HasPrefix(raw, "/")) {
			raw = filepath.VolumeName(cwd) + raw
		} else if filepath.VolumeName(raw) == "" {
			raw = filepath.Join(cwd, raw)
		} else {
			return "", fmt.Errorf("use an absolute drive path: %s", raw)
		}
	}
	return filepath.Abs(raw)
}

func (s *session) paths(raw []string) ([]string, error) {
	paths := make([]string, 0, len(raw))
	for _, value := range raw {
		p, err := localPath(s.Cwd, value)
		if err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}

func checkDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	return nil
}

func changeDirectory(s *session, args []string) error {
	if len(args) > 1 {
		return usageError("usage: cd [directory]")
	}
	target, err := localPath(s.Cwd, first(args, "~"))
	if err != nil {
		return err
	}
	if err := checkDirectory(target); err != nil {
		return err
	}
	s.Cwd, s.ChangedCwd = target, true
	return nil
}

func printDirectory(s *session, args []string) error {
	if len(args) != 0 {
		return usageError("usage: pwd")
	}
	fmt.Fprintln(&s.output, s.Cwd)
	return nil
}

func tokenize(input string) ([]string, error) {
	var tokens []string
	var word strings.Builder
	var quote rune
	active := false
	chars := []rune(input)
	for i := 0; i < len(chars); i++ {
		ch := chars[i]
		switch {
		case ch == '\\' && quote != '\'' && runtime.GOOS != "windows":
			i++
			if i == len(chars) {
				return nil, usageError("unterminated escape sequence")
			}
			word.WriteRune(chars[i])
			active = true
		case quote != 0:
			if ch == quote {
				quote = 0
			} else {
				word.WriteRune(ch)
			}
		case ch == '\'' || ch == '"':
			quote, active = ch, true
		case unicode.IsSpace(ch):
			if active {
				tokens = append(tokens, word.String())
				word.Reset()
				active = false
			}
		default:
			word.WriteRune(ch)
			active = true
		}
	}
	if quote != 0 {
		return nil, usageError("unterminated quote")
	}
	if active {
		tokens = append(tokens, word.String())
	}
	return tokens, nil
}

type usageError string

func (err usageError) Error() string { return string(err) }

type options map[string]string

func parse(args []string, switches, values string) (options, []string, error) {
	flags := make(options)
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, inline := strings.Cut(arg, "=")
		switch {
		case arg == "--":
			return flags, append(positional, args[i+1:]...), nil
		case !strings.HasPrefix(arg, "-") || arg == "-":
			positional = append(positional, arg)
		case !inline && strings.Contains(" "+switches+" ", " "+name+" "):
			flags[name] = "true"
		case strings.Contains(" "+values+" ", " "+name+" "):
			if !inline {
				i++
				if i == len(args) {
					return nil, nil, usageError(name + " requires a value")
				}
				value = args[i]
			}
			flags[name] = value
		default:
			return nil, nil, usageError("unknown option: " + arg)
		}
	}
	return flags, positional, nil
}

func (o options) has(names ...string) bool {
	for _, name := range names {
		if _, ok := o[name]; ok {
			return true
		}
	}
	return false
}

func (o options) value(fallback string, names ...string) string {
	for _, name := range names {
		if value, ok := o[name]; ok {
			return value
		}
	}
	return fallback
}

func (o options) integer(fallback int, names ...string) (int, error) {
	value := o.value(strconv.Itoa(fallback), names...)
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, usageError(names[0] + " requires a non-negative integer")
	}
	return n, nil
}

func first(args []string, fallback string) string {
	if len(args) == 0 {
		return fallback
	}
	return args[0]
}

func (s *session) root(args []string) (string, error) {
	if len(args) > 1 {
		return "", usageError("expected at most one path")
	}
	return localPath(s.Cwd, first(args, "."))
}
