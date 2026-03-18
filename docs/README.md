# Wails File Manager

## About

This repository is a Wails desktop file manager with an Angular frontend and a Go backend. It supports dual-pane browsing, multiple file views, archive browsing and mutation, batch operations, workspace persistence, and Windows-aware elevation fallbacks.

Frontend implementation notes, including view-operation parity expectations and pane/surface architecture, are documented in [frontend/README.md](frontend/README.md).

The staged refactor history and current architectural checkpoints are documented in [REFACTOR_ROADMAP.md](REFACTOR_ROADMAP.md).

The next-step backend packaging and layout proposal is documented in [BACKEND_REORGANIZATION_PLAN.md](BACKEND_REORGANIZATION_PLAN.md).

## Architecture

The current backend structure is organized around explicit internal package boundaries, while `App` remains the Wails binding wrapper exposed to Wails.

- Wails-facing app wrapper: [app.go](app.go)
- Internal app state/runtime/service assembly: [internal/app](internal/app)
- Archive domain facade: [internal/archive/service.go](internal/archive/service.go)
- Filesystem orchestration: [filesystem_service.go](filesystem_service.go)
- Batch orchestration: [internal/batch/service.go](internal/batch/service.go)
- Navigation orchestration: [internal/navigation/service.go](internal/navigation/service.go)
- Workspace/preferences orchestration: [internal/workspace/service.go](internal/workspace/service.go)
- Elevated session lifecycle and IPC: [internal/elevation](internal/elevation)
- Directory stream and pane watch runtime: [internal/runtime](internal/runtime)
- Platform/path helpers: [internal/platform](internal/platform)
- Small cross-domain utilities: [internal/utility/service.go](internal/utility/service.go)

The frontend is organized around a thin pane host plus controller/factory layers rather than a single large smart component.

- Pane host/facade: [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts)
- Surface/view host: [frontend/src/app/components/file-view-container.component.ts](frontend/src/app/components/file-view-container.component.ts)
- Dynamic surface runtime: [frontend/src/app/surfaces](frontend/src/app/surfaces)
- Viewer host/runtime: [frontend/src/app/viewers](frontend/src/app/viewers)

## Configuration

Project-level Wails configuration lives in [wails.json](wails.json). Wails project setting details are documented at https://wails.io/docs/reference/project-config.

## File View Operations Matrix

All file views share the same operation pipeline and are treated as equivalent interaction surfaces.

| Operation | List | Grid | Details | Tree |
|---|---:|---:|---:|---:|
| Single select | ✅ | ✅ | ✅ | ✅ |
| Multi-select (Ctrl/Cmd toggle) | ✅ | ✅ | ✅ | ✅ |
| Range select (Shift) | ✅ | ✅ | ✅ | ✅ |
| Select all / Deselect all | ✅ | ✅ | ✅ | ✅ |
| Open item (Enter / double click) | ✅ | ✅ | ✅ | ✅ |
| Rename selected (F2 / Shift+F6) | ✅ | ✅ | ✅ | ✅ |
| Copy to other pane (F5) | ✅ | ✅ | ✅ | ✅ |
| Move to other pane (F6) | ✅ | ✅ | ✅ | ✅ |
| New folder (F7) | ✅ | ✅ | ✅ | ✅ |
| Delete selected (F8 / Delete) | ✅ | ✅ | ✅ | ✅ |
| Drag & drop copy/move (Ctrl/Cmd = copy) | ✅ | ✅ | ✅ | ✅ |

### Shortcut Notes

- `F5` copy to other pane
- `F6` move to other pane
- `Shift+F6` rename selected item
- `F7` create folder
- `F8` delete selected items
- `Ctrl+R` refresh current pane
- `Ctrl+A` select all
- `Ctrl+D` clear selection

Tree view participates in the same operation flow as the other views (selection, keyboard actions, and drag/drop routing).

## Live Development

To run in live development mode, run `wails dev` in the project directory. This launches the Angular frontend in Wails live-reload mode together with the Go backend so frontend and backend changes can be iterated together.

## Building

To build a redistributable, production mode package, use `wails build`.
