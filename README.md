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

For Windows live development, use `npm run dev:app`. That wrapper runs `wails dev` from `manager/` and adds `-ldflags "-H windowsgui"` so `wails-app` stays a GUI binary during elevated restarts.
