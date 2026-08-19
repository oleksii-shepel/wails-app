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

The Registry backend currently remains in `manager/vfs/registry`. The `plugins/registry` project is the extraction target for Registry presentation, commands, toolbar/context-menu contributions, and Registry editors.
