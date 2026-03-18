# Refactor Roadmap

## Purpose

This roadmap turns the current refactor effort into a staged plan with clear boundaries, target files, and acceptance criteria. The goal is to keep behavior stable while reducing the size and coupling of the largest orchestration files in both the Go backend and Angular frontend.

## Working Rules

- Preserve current user-visible behavior unless a phase explicitly says otherwise.
- Keep archive behavior, pane behavior, and view-operation parity intact during each split.
- Prefer extraction behind existing interfaces over broad rewrites.
- Validate each phase with `npm run build` and `go build .` unless the phase is docs-only.
- Do not combine large frontend and backend behavioral changes in the same phase.

## Current Snapshot

- Completed: backend archive facade via [archive_service.go](archive_service.go)
- Completed: backend extraction split via [archive_extract.go](archive_extract.go)
- Completed: backend rewrite split via [archive_rewrite.go](archive_rewrite.go)
- Completed: frontend pane navigation extraction via [frontend/src/app/components/file-pane-navigation.controller.ts](frontend/src/app/components/file-pane-navigation.controller.ts)
- Completed: frontend pane state extraction via [frontend/src/app/components/file-pane-state.controller.ts](frontend/src/app/components/file-pane-state.controller.ts)
- Completed: frontend archive workflow extraction via [frontend/src/app/components/file-pane-archive.controller.ts](frontend/src/app/components/file-pane-archive.controller.ts)
- Completed: frontend pane directory orchestration extraction via [frontend/src/app/components/file-pane-directory.controller.ts](frontend/src/app/components/file-pane-directory.controller.ts)
- Completed: frontend pane tab/workspace orchestration extraction via [frontend/src/app/components/file-pane-tabs.controller.ts](frontend/src/app/components/file-pane-tabs.controller.ts)
- Completed: frontend pane session/bootstrap extraction via [frontend/src/app/components/file-pane-dir-session.controller.ts](frontend/src/app/components/file-pane-dir-session.controller.ts)
- Completed: frontend pane UI preference extraction via [frontend/src/app/components/file-pane-ui.controller.ts](frontend/src/app/components/file-pane-ui.controller.ts)
- Completed: frontend pane path/archive-path extraction via [frontend/src/app/components/file-pane-path.controller.ts](frontend/src/app/components/file-pane-path.controller.ts)
- Completed: frontend pane focus/activation extraction via [frontend/src/app/components/file-pane-focus.controller.ts](frontend/src/app/components/file-pane-focus.controller.ts)
- Completed: frontend pane operations extraction via [frontend/src/app/components/file-pane-operations.controller.ts](frontend/src/app/components/file-pane-operations.controller.ts)
- Completed: frontend pane preview extraction via [frontend/src/app/components/file-pane-preview.controller.ts](frontend/src/app/components/file-pane-preview.controller.ts)
- Completed: directory tab state integration via [frontend/src/app/surfaces/dir/dir-tab-state-controller.ts](frontend/src/app/surfaces/dir/dir-tab-state-controller.ts)
- Completed: [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts) reduced to a controller-backed host/facade
- Remaining orchestration hotspot: [app.go](app.go)

## Phase 1: Finish Frontend Pane Decomposition

Status: Complete

### Objective

Shrink [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts) into a host/facade and move the remaining mixed filesystem/archive action flows into smaller controllers.

### Target Files

- [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts)
- [frontend/src/app/components/file-browser.component.ts](frontend/src/app/components/file-browser.component.ts)
- [frontend/src/app/components/file-pane-archive.controller.ts](frontend/src/app/components/file-pane-archive.controller.ts)
- [frontend/src/app/components/file-pane-state.controller.ts](frontend/src/app/components/file-pane-state.controller.ts)
- New: `frontend/src/app/components/file-pane-operations.controller.ts`
- New: `frontend/src/app/components/file-pane-preview.controller.ts` if quick view and frozen preview logic remains coupled

### Scope

- Extract delete, rename, copy-to-other, move-to-other, drag-drop, and destination-path logic into an operations controller.
- Extract preview tab opening and archive-aware viewer-tab bootstrapping if it still mixes with pane navigation.
- Leave template ownership in `file-pane.component.ts`; move only orchestration logic.

### Acceptance Criteria

- `file-pane.component.ts` acts mainly as a host and event adapter.
- Archive operations and filesystem operations are separated behind explicit host interfaces.
- Copy, move, delete, rename, quick preview, and drag-drop behavior remain unchanged in list, grid, details, and tree views.
- No new direct cross-controller coupling is added inside the component.
- `npm run build` passes.

## Phase 2: Stabilize Frontend Surface Boundaries

Status: Complete

### Objective

Make the dynamic surface system easier to reason about by reducing implicit state sharing between pane, surface runtime, and viewer host layers.

### Target Files

- [frontend/src/app/surfaces/surface-outlet.component.ts](frontend/src/app/surfaces/surface-outlet.component.ts)
- [frontend/src/app/surfaces/surface-context.ts](frontend/src/app/surfaces/surface-context.ts)
- [frontend/src/app/surfaces/dir/dir-surface-session.ts](frontend/src/app/surfaces/dir/dir-surface-session.ts)
- [frontend/src/app/surfaces/dir/file-pane-dir-surface-runtime.ts](frontend/src/app/surfaces/dir/file-pane-dir-surface-runtime.ts)
- [frontend/src/app/viewers/viewer-host-surface.component.ts](frontend/src/app/viewers/viewer-host-surface.component.ts)
- [frontend/src/app/components/file-view-container.component.ts](frontend/src/app/components/file-view-container.component.ts)

### Scope

- Clarify which state belongs to the pane, which belongs to a dir session, and which belongs to a rendered surface.
- Remove redundant surface re-resolution triggers.
- Reduce best-effort imperative calls from pane to child components where a VM contract can carry the same intent.

### Acceptance Criteria

- Surface re-rendering occurs only on meaningful context changes.
- Directory surface runtime responsibilities are documented in code by interface shape, not only by naming.
- Viewer host and dir surface host use the same stability rules for dynamic rendering.
- No blank-screen or partial-render regressions in `wails dev`.
- `npm run build` passes.

### Progress Notes

- Completed: viewer host stability now keys rerendering on resolved viewer contribution identity.
- Completed: surface context is now discriminated by rendered kind instead of sharing dir and viewer payloads in one shape.
- Completed: dir tab restore now splits durable session state from UI runtime follow-up.
- Completed: pane-to-dir-surface coupling no longer reaches directly into tree internals; view-container adapter methods now define that boundary.
- Completed: tree build, sync, and lazy-load orchestration moved out of [frontend/src/app/components/file-view-container.component.ts](frontend/src/app/components/file-view-container.component.ts) into [frontend/src/app/components/file-view-tree.controller.ts](frontend/src/app/components/file-view-tree.controller.ts).
- Completed: active-view runtime, event passthrough, and sorting helpers now live in dedicated controllers instead of branching directly in [frontend/src/app/components/file-view-container.component.ts](frontend/src/app/components/file-view-container.component.ts).
- Completed: drag/drop normalization and view-switch orchestration now live in [frontend/src/app/components/file-view-interaction.controller.ts](frontend/src/app/components/file-view-interaction.controller.ts).
- Completed: remaining tree event adapters and child-load dispatch now delegate through the controller layer, leaving [frontend/src/app/components/file-view-container.component.ts](frontend/src/app/components/file-view-container.component.ts) primarily as a surface host plus explicit child accessors.
- Completed: lifecycle-time tree/sort coordination now delegates through [frontend/src/app/components/file-view-lifecycle.controller.ts](frontend/src/app/components/file-view-lifecycle.controller.ts), leaving [frontend/src/app/components/file-view-container.component.ts](frontend/src/app/components/file-view-container.component.ts) with minimal orchestration.
- Completed: pane/session/view host construction for tabs, dir-session, UI, path, and focus now routes through [frontend/src/app/components/file-pane-surface-hosts.factory.ts](frontend/src/app/components/file-pane-surface-hosts.factory.ts), and the repaired pane build now validates successfully.
- Completed: pane navigation and state host construction now routes through [frontend/src/app/components/file-pane-runtime-hosts.factory.ts](frontend/src/app/components/file-pane-runtime-hosts.factory.ts), further reducing inline controller adapter assembly in [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts).
- Completed: pane archive, directory, operations, preview, and open host construction now routes through [frontend/src/app/components/file-pane-action-hosts.factory.ts](frontend/src/app/components/file-pane-action-hosts.factory.ts), leaving [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts) primarily as state, VM, and event orchestration.
- Completed: dir surface VM assembly now routes through [frontend/src/app/components/file-pane-dir-vm.factory.ts](frontend/src/app/components/file-pane-dir-vm.factory.ts), removing the large inline `DirSurfaceVm` implementation from [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts).
- Completed: pane runtime-state transitions for archive reset, dir-tab persistence, and post-navigation restore now delegate through [frontend/src/app/components/file-pane-runtime-state.controller.ts](frontend/src/app/components/file-pane-runtime-state.controller.ts), and navigation host state is now live-backed instead of snapshot-copied.
- Completed: active tab/viewer resolution and discriminated rendered-surface selection now route through [frontend/src/app/components/file-pane-surface-context.factory.ts](frontend/src/app/components/file-pane-surface-context.factory.ts) instead of being assembled inline in [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts).
- Completed: keyboard-facing selection, tab, and focus shortcut commands now route through [frontend/src/app/components/file-pane-shortcut-commands.factory.ts](frontend/src/app/components/file-pane-shortcut-commands.factory.ts), allowing [frontend/src/app/components/file-browser.component.ts](frontend/src/app/components/file-browser.component.ts) to use grouped pane command objects instead of many single-purpose pane methods.
- Completed: remaining pane state/runtime adapter helpers now route through [frontend/src/app/components/file-pane-state-runtime.factory.ts](frontend/src/app/components/file-pane-state-runtime.factory.ts), removing redundant inline preview-target, selection, and scroll adapter methods from [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts); `npm run build` passes after this slice.
- Completed: browser-facing preview, navigation, and selection actions now route through [frontend/src/app/components/file-pane-browser-commands.factory.ts](frontend/src/app/components/file-pane-browser-commands.factory.ts), letting browser-side controllers consume grouped pane commands while [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts) drops another set of thin public forwarding methods.
- Completed: dir-surface-only entry action adapters for open-in-new-tab, rename, and drag-drop are now private helpers inside [frontend/src/app/components/file-pane.component.ts](frontend/src/app/components/file-pane.component.ts), further tightening the pane facade without changing VM behavior.
- Completed: archive breadcrumb segment clicks now bind from [frontend/src/app/components/dir-surface.component.ts](frontend/src/app/components/dir-surface.component.ts) through the dir VM into the pane archive controller, replacing a dead pane adapter with a live surface path.

## Phase 3: Split Backend Archive Domain Internals

Status: Complete

### Objective

Continue breaking archive logic into cohesive files so mutation, rewrite, listing, nested traversal, and shared helpers do not remain concentrated in a small number of large files.

### Target Files

- [archive.go](archive.go)
- [archive_service.go](archive_service.go)
- [archive_extract.go](archive_extract.go)
- [archive_mutation.go](archive_mutation.go)
- [archive_rewrite.go](archive_rewrite.go)
- Possible new files:
  - `archive_browse.go`
  - `archive_nested.go`
  - `archive_helpers.go`
  - `archive_formats.go`

### Scope

- Move archive browse/list/read behavior out of `archive.go` into smaller files.
- Separate nested archive traversal helpers from top-level archive behavior.
- Keep the `ArchiveService` facade thin and stable.
- Avoid changing exported Wails-facing method names unless the generated frontend bindings also change in the same phase.

### Acceptance Criteria

- `archive.go` is no longer the default dumping ground for browse and helper logic.
- Each archive file has a single dominant responsibility.
- Wails bindings still generate cleanly.
- Archive copy, move, delete, read, and browse behavior remains unchanged for top-level and nested archives.
- `go build .` passes.

### Progress Notes

- Completed: archive request/result contracts now live in [archive_types.go](archive_types.go), leaving the old [archive.go](archive.go) placeholder removable and making the archive file split more explicit.
- Completed: archive read and in-memory extraction helpers now live in [archive_read.go](archive_read.go) instead of the misleading [archive_nested.go](archive_nested.go), tightening the Phase 3 file layout around actual responsibility.
- Completed: generic extraction helpers now live in [archive_extract_helpers.go](archive_extract_helpers.go), leaving [archive_extract.go](archive_extract.go) focused on extraction flow rather than file/path utility details.
- Completed: browse-only entry processing and sorting helpers now live in [archive_browse_helpers.go](archive_browse_helpers.go), reducing non-browse detail inside [archive_browse.go](archive_browse.go).
- Completed: shared archive path and matching helpers are now tighter, with extract-path rebasing grouped into [archive_helpers.go](archive_helpers.go) and format-selection rules clarified in [archive_format_helpers.go](archive_format_helpers.go).
- Completed: browse format-specific walkers now live in [archive_browse_formats.go](archive_browse_formats.go), leaving [archive_browse.go](archive_browse.go) as a thin browse dispatcher instead of mixing dispatch with ZIP, tar, reader, and archiver traversal details.
- Completed: read format-specific extract helpers now live in [archive_read_formats.go](archive_read_formats.go), leaving [archive_read.go](archive_read.go) focused on read orchestration and nested-chain traversal.
- Completed: rewrite format-specific implementations now live in [archive_rewrite_formats.go](archive_rewrite_formats.go), leaving [archive_rewrite.go](archive_rewrite.go) focused on delete orchestration and format dispatch.
- Completed: rewrite format selection and read-only-format rejection now live in [archive_rewrite_dispatch.go](archive_rewrite_dispatch.go), further thinning [archive_rewrite.go](archive_rewrite.go) to request validation plus operation/result orchestration.
- Completed: archive copy and move flows now live in [archive_copy.go](archive_copy.go) and [archive_move.go](archive_move.go), removing the last combined mutation file and giving each archive mutation flow its own file-level responsibility.

## Phase 4: Extract App-Level Backend Services

Status: Complete

### Progress Notes

- Completed: elevated session state, lifecycle, and approval waiters now live in [elevation_session_service.go](elevation_session_service.go), leaving [app.go](app.go) with thin Wails-facing elevation delegates and moving the first app-owned service cluster behind an explicit Phase 4 boundary.
- Completed: navigation and workspace wrapper clusters now delegate through [navigation_service.go](navigation_service.go) and [workspace_service.go](workspace_service.go), so [app.go](app.go) no longer owns those manager-level orchestration details directly.
- Completed: filesystem entrypoints now delegate through [filesystem_service.go](filesystem_service.go), and batch orchestration now lives under [batch_service.go](batch_service.go) plus its internal helper receivers in [batch_operations.go](batch_operations.go), further shrinking the service surface owned directly by [app.go](app.go).
- Completed: directory stream lifecycle/state now delegates through [internal/runtime/dir_stream.go](internal/runtime/dir_stream.go), and the remaining small utility cluster for file opening, link resolution, and file-drop normalization now delegates through [internal/utility/service.go](internal/utility/service.go), leaving [app.go](app.go) closer to a pure composition root.
- Completed: app lifecycle/runtime bridge responsibilities now delegate through [app_runtime_service.go](app_runtime_service.go), moving `startup`, `shutdown`, native Wails file-drop subscription, and app-level event emission out of [app.go](app.go) and leaving the Wails boundary closer to a pure composition root.

### Objective

Reduce the size and centrality of [app.go](app.go) by pushing domain logic into explicit service structs rather than method clusters on `App`.

### Target Files

- [app.go](app.go)
- [filesystem.go](filesystem.go)
- [operations.go](operations.go)
- [navigation.go](navigation.go)
- [workspace.go](workspace.go)
- [batch_operations.go](batch_operations.go)
- [elevated_service.go](elevated_service.go)
- Possible new files:
  - `filesystem_service.go`
  - `navigation_service.go`
  - `workspace_service.go`
  - `batch_service.go`

### Scope

- Introduce thin service boundaries where `App` currently owns orchestration directly.
- Keep `App` as the Wails binding boundary and composition root.
- Move cross-domain helper logic only when ownership is clear.

### Acceptance Criteria

- `App` primarily wires services and exposes Wails-safe entry points.
- Filesystem, navigation, workspace, and batch behavior are grouped by domain instead of by historical growth.
- Service dependencies are explicit in construction.
- `go build .` passes.

## Phase 5: Consolidate Shared Contracts And Documentation

Status: In Progress

### Objective

Make the refactored structure maintainable by documenting boundaries and tightening shared contracts between frontend and backend.

### Target Files

- [README.md](README.md)
- [frontend/README.md](frontend/README.md)
- [SHORTCUTS_AND_HANDLERS.md](SHORTCUTS_AND_HANDLERS.md)
- [NAVIGATION_FEATURE_STATUS.md](NAVIGATION_FEATURE_STATUS.md)
- [frontend/src/app/shared/protocol.ts](frontend/src/app/shared/protocol.ts)
- [frontend/src/app/shared/protocol-enhanced.ts](frontend/src/app/shared/protocol-enhanced.ts)

### Scope

- Document the new controller and service boundaries.
- Remove stale docs that still describe pre-surface or pre-controller architecture.
- Tighten duplicated protocol types if they drifted during refactor work.

### Acceptance Criteria

- Repo docs describe the current architecture rather than the previous one.
- New contributors can identify where pane logic, archive logic, and service boundaries live.
- Shared protocol definitions are not duplicated without reason.
- `npm run build` and `go build .` still pass after doc-adjacent cleanup.

### Progress Notes

- Completed: [README.md](README.md) now describes the current Angular plus Go file-manager architecture instead of the original Wails template baseline.
- Completed: [frontend/README.md](frontend/README.md) now documents the pane host, controller/factory layer, and surface/view boundary rules used by the refactored frontend.
- Completed: [SHORTCUTS_AND_HANDLERS.md](SHORTCUTS_AND_HANDLERS.md) now explains the current interaction-routing architecture so shortcut additions follow the controller and command-surface boundaries.
- Completed: [NAVIGATION_FEATURE_STATUS.md](NAVIGATION_FEATURE_STATUS.md) now references the current backend service split for navigation, filesystem listing, and directory streaming.
- Completed: shared protocol cleanup removed the `DriveInfo` duplication, moved archive types and `SpecialFolder` into [frontend/src/app/shared/protocol.ts](frontend/src/app/shared/protocol.ts), and left [frontend/src/app/shared/protocol-enhanced.ts](frontend/src/app/shared/protocol-enhanced.ts) focused on app-level workspace, tab, filter, and progress models.

## Phase Order

1. Finish frontend pane decomposition.
2. Stabilize frontend surface boundaries.
3. Split backend archive domain internals.
4. Extract app-level backend services.
5. Consolidate shared contracts and documentation.

## Phase Exit Checklist

Use this checklist at the end of each non-doc phase.

- Target files are smaller and have clearer ownership than before.
- Public behavior in file operations, tab behavior, archive behavior, and preview behavior is unchanged.
- No new circular dependencies or host/controller leaks were introduced.
- `npm run build` passes.
- `go build .` passes.
- Any architecture notes changed by the phase are updated in this roadmap or linked docs.
