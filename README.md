# Reassembly Workspace

This repository is organized as a workspace with the host application and plugins as sibling projects.

```text
.
├── manager/
│   ├── frontend/
│   ├── internal/
│   ├── vfs/
│   ├── go.mod
│   └── wails.json
├── plugins/
│   └── registry/
├── docs/
├── .github/
└── go.work
```

`manager` is the Wails host application. Plugins must depend only on public plugin contracts/capabilities exposed by the host; they must not import Manager component or service internals.

The Registry frontend plugin now lives in `plugins/registry`. Registry presentation, editors, panel-source behavior, commands, and context-menu contributions are owned there. The Registry backend remains in `manager/vfs/registry` until a backend plugin ABI exists.

## Starting the app

Install the JavaScript dependencies once from the repository root, then run Wails from `manager/`:

```bash
npm install          # repo root
cd manager
wails dev            # starts the host; Angular dev server runs on :3000
```

On Windows, add `wails dev -ldflags=-H=windowsgui` so elevated restarts keep the app as a GUI binary (`scripts/run-wails-dev.mjs` does the same).

## Building the Windows desktop app

Use Node.js, Go, the Wails CLI, and WebView2 on Windows. From the repository root:

```powershell
npm ci
Set-Location manager
wails build
```

The executable is written to `manager/build/bin/`. Run it on Windows before
publishing a release.

## Running the local browser preview

From the repository root, run `npm ci`, `npm run build`, then `npm run dev`.
Open [127.0.0.1:3000](http://127.0.0.1:3000). The preview can modify local files
and listens only on the local machine.

For frontend development, run `npm run dev:watch` in a second terminal.
The preview serves the latest files from `manager/frontend/dist/`; reload the
page after the watcher finishes rebuilding.
