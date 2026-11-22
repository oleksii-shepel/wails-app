# Aether / Reassembly Command Line Tool (`aether`)

Reassembly includes both a standalone Node.js CLI (`./cli/aether.mjs`, exposed via `"bin": { "aether", "reassembly" }` and `npm run cli`) and an integrated **Command Line Tab** (`Ctrl+Alt+C`, **Tools → Command Line Tab**, Command Palette, or Quick Access) that acquires a pane tab inside the dual-pane workspace.

## Running Standalone

```bash
# Directly
./cli/aether.mjs --help

# Via npm script
npm run cli -- ls -l --sort size
npm run cli -- du --top 15
npm run cli -- find . -p "*.ts" -g "openSurfaceTab"
npm run cli -- dup . --min-size 1K
npm run cli -- hash fileA fileB --manifest > CHECKSUMS.sha256
npm run cli -- verify CHECKSUMS.sha256
npm run cli -- diff left.txt right.txt --unified
npm run cli -- compare ./dirA ./dirB -r --hash
npm run cli -- split large.bin --size 10M
npm run cli -- join large.bin.part001 -o large.bin
npm run cli -- rename *.png --pattern "shot_{nnn}" --apply
npm run cli -- archive zip bundle.zip ./src
npm run cli -- archive list bundle.zip
npm run cli -- archive extract bundle.zip ./out
npm run cli -- trash list
npm run cli -- ws paths
npm run cli -- doctor
```

Global flags: `--json`, `--no-color`, `--cwd <dir>`, `-h/--help`, `-v/--version`.

## Using the Integrated Pane Tab in Browser Preview

The integrated tab uses the Node preview server started by `npm run dev`. In browser preview, press `Ctrl+Alt+C` (or choose **Tools → Command Line Tab**) to open **Aether CLI** inside one of the dual-pane tabs:

- Shares the active directory with your file panes (**Sync Peer** pulls the peer pane's folder).
- Executes `aether` subcommands directly through `POST /api/cli/exec` with ANSI color output, command history (`↑`/`↓`), `Tab` command & filename completion, `cd`/`pwd`/`clear` builtins, and one-click quick actions (`compare peer`, `ls -l`, `du`, `find`, `dup`, `hash`, `trash`, `ws`, `doctor`).
