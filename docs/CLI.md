# Aether CLI

## Desktop command line tab

Press `Ctrl+Alt+C`, choose **Command Line Tab** from the **Tools** menu, or open it from the Command Palette or Quick Access. The tab executes commands in the file manager's Go backend through Wails. It requires no Node.js runtime or HTTP server.

The prompt shows the tab's working directory. Relative paths resolve against it, and **Sync Peer** sets it to the other pane's active folder. Use `pwd` to print it, `cd <directory>` to change it, and `clear` or `Ctrl+L` to clear the transcript. Use **Tab** for command and filename completion and the arrow keys to recall commands. Quote paths containing spaces.

Enter commands directly, such as `ls -l`. The optional `aether` prefix also works. Use `help` for the command list and `help <command>` for its usage. Global options include `--cwd <dir>`, `--json`, `--no-color`, `-h` / `--help`, and `-v` / `--version`.

| Command | Example | Purpose |
| --- | --- | --- |
| `ls` | `ls -l --sort size` | List files and directories |
| `du` | `du --depth 2 --top 15` | Show disk usage and rank large entries |
| `find` | `find . -p "*.ts" -g "openSurfaceTab"` | Find files by name or text |
| `dup` | `dup . --min-size 1K` | Find duplicate files using SHA-256 |
| `hash` | `hash "report.pdf" --algo sha256` | Compute checksums or print a checksum manifest |
| `verify` | `verify CHECKSUMS.sha256` | Verify files against a checksum manifest |
| `diff` | `diff left.txt right.txt --unified` | Compare two files |
| `compare` | `compare ./dirA ./dirB -r --hash` | Compare directories recursively and check file contents |
| `split` | `split large.bin --size 10M` | Create numbered file parts |
| `join` | `join large.bin.part001 -o restored.bin` | Reassemble numbered file parts |
| `rename` | `rename first.png second.png --pattern "shot_{nnn}"` | Preview a batch rename |
| `archive` | `archive list bundle.zip` | List archive entries, create ZIPs, or extract archives |
| `trash` | `trash list` | List, move, restore, or empty system trash |
| `ws` | `ws paths` | Show the active saved workspace's pane paths |
| `doctor` | `doctor` | Report desktop CLI diagnostics |

Rename changes files only with `--apply`. The tab refuses to overwrite split parts, joined files, ZIP outputs, and extracted files. Desktop `diff` accepts files up to 16 MiB each and limits line comparisons. Both `diff` and `compare` return exit code 1 when they find differences.

The desktop tab uses the file manager's saved workspaces and system trash. Use `trash put <paths...>` to move items to trash and `trash restore <id|name>` to restore them. Emptying the system trash requires `trash empty --yes`.

## Standalone CLI

The standalone CLI runs under Node.js. From the repository root, run `node cli/aether.mjs <command>`.

```bash
node cli/aether.mjs --help
node cli/aether.mjs ls -l --sort size
node cli/aether.mjs hash fileA fileB --manifest > CHECKSUMS.sha256
node cli/aether.mjs verify CHECKSUMS.sha256
```

The standalone CLI uses its own workspace and trash storage. Use the desktop tab to inspect the file manager's saved workspaces or manage the system trash.
