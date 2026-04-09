# Wails File Manager

## About

This repository is a Wails desktop file manager with an Angular frontend and a Go backend. It supports dual-pane browsing, multiple file views, archive browsing and mutation, batch operations, workspace persistence, and Windows-aware elevation fallbacks.

Frontend implementation notes, including view-operation parity expectations and pane/surface architecture, are documented in [frontend/README.md](frontend/README.md).

The staged refactor history and current architectural checkpoints are documented in [REFACTOR_ROADMAP.md](REFACTOR_ROADMAP.md).

The next-step backend packaging and layout proposal is documented in [BACKEND_REORGANIZATION_PLAN.md](BACKEND_REORGANIZATION_PLAN.md).

Recent feature-specific documentation:

- Image preview format coverage and conversion behavior: [SUPPORTED_FILES_BY_VIEWER.md](SUPPORTED_FILES_BY_VIEWER.md)
- Backend image conversion pipeline and streamed preview transport: [IMAGE_CONVERTER_PIPELINE.md](IMAGE_CONVERTER_PIPELINE.md)
- AVI preview behavior and current fallback settings: [AVI_PLAYBACK_SETTINGS.md](AVI_PLAYBACK_SETTINGS.md)
- Native child webview bidirectional messaging and bridge contract: [BROWSER_PANE_BRIDGE.md](BROWSER_PANE_BRIDGE.md)
- Navigation, editable address bar, and search scheme behavior: [NAVIGATION_FEATURE_STATUS.md](NAVIGATION_FEATURE_STATUS.md)
- Keyboard and address-bar edit behavior: [SHORTCUTS_AND_HANDLERS.md](SHORTCUTS_AND_HANDLERS.md)
- Remote cloud mount OAuth flow and browser-based localhost callback design: [REMOTE_OAUTH_FLOW.md](REMOTE_OAUTH_FLOW.md)

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

## Bundled External Tools

The app resolves external executables from a bundled tools directory relative to the shipped executable before checking the system `PATH`.

Supported root discovery order:

1. `WAILS_TOOLS_DIR`
2. `WAILS_EXTERNAL_TOOLS_DIR`
3. `tools` next to the built executable
4. `portable-tools` next to the built executable
5. `tools` one level above the built executable
6. `tools` in the current working directory

Recommended layout:

```text
tools/
	ffmpeg/
		ffmpeg.exe
		ffprobe.exe
		...runtime DLLs...
	libvips/
		vips.exe
		vipsthumbnail.exe
		libvips*.dll
		...runtime DLLs / loaders...
	mupdf/
		mutool.exe
		...runtime files...
	libreoffice/
		soffice.exe
		...runtime files...
	djvulibre/
		ddjvu.exe
		...runtime files...
```

For packaged builds, `tools/` next to the executable is the intended delivery layout. The resolver checks the root directly and then each first-level subdirectory, so the executables can live either directly under `tools/` or inside per-tool folders.

For portable delivery on Windows, copy each tool's full install/runtime directory, not just the `.exe`, because tools such as libvips, MuPDF, and LibreOffice depend on adjacent DLLs and support files.

To refresh the bundled tool directory from the current machine, run [scripts/collect-portable-tools.ps1](scripts/collect-portable-tools.ps1).

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

## Memory Filesystem (VFS sidecar)

The app uses an out-of-process sidecar (`in-memory-fs`) to host a persistent in-memory filesystem that is mounted into the app's virtual filesystem at the path `/memory`.

The sidecar is intended to outlive the Wails UI process. When the app starts again, it reconnects to an already running memfs instance through the control IPC channel and reuses the existing hidden backing path instead of starting with a fresh store.

The sidecar is launched by `ProcessManager` ([vfs/memory/process_unix.go](../vfs/memory/process_unix.go) / Windows equivalent) and communicates with the app via a platform-specific IPC channel:

| Platform | IPC channel | Details |
|---|---|---|
| Windows | Named pipe `\\\\.\\ pipe\\memfs-control` | Restricted to the current user via a Windows security descriptor |
| Linux | Unix socket `/tmp/memfs-control-<uid>.sock` | `chmod 0600`; removed on clean exit |
| macOS | Unix socket `/tmp/memfs-control-<uid>.sock` | `chmod 0600`; removed on clean exit |

Both Unix platforms rely on macFUSE (macOS) or the kernel FUSE driver (Linux) and require the sidecar binary to be present on `PATH` or at the path configured in preferences.

**Startup and reconnect sequence (all platforms)**:

1. `DefaultRootProvider` ([vfs/memory/fs.go](../vfs/memory/fs.go)) checks `WAILS_MEMFS_ROOT` and `MEMFS_HIDDEN_PATH` first.
2. If neither env override is set, it connects to the sidecar IPC channel and calls `get-connection-info`.
3. If memfs is already running, the returned `hiddenPath` is reused, so restarting the Wails app reconnects to the same memfs instance.
4. If the sidecar is not running, startup code can launch it and wait for the mount to become visible.
5. The resolved root path is wrapped as `osfs.FS` and registered at VFS path `/memory` by `InitVFS` ([internal/runtime/vfs.go](../internal/runtime/vfs.go)).

**macOS prerequisites**: macFUSE must be installed (`/Library/Filesystems/macfuse.fs`). The preflight check in [vfs/memory/preflight_unix.go](../vfs/memory/preflight_unix.go) will return a user-friendly error if it is absent.

## Live Development

To run in live development mode, run `wails dev` in the project directory. This launches the Angular frontend in Wails live-reload mode together with the Go backend so frontend and backend changes can be iterated together.

## Building

To build a redistributable, production mode package, use `wails build`.
