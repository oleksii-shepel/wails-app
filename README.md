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
