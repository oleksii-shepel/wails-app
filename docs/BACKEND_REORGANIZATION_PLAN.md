# Backend Reorganization Plan

## Goal

Reorganize the Go backend into a structure that is easier to maintain, easier to onboard into, and safer to extend, without breaking current Wails bindings or archive/filesystem behavior.

This plan is intentionally incremental. The backend already completed the first major refactor step: large behaviors were split into service-oriented files while `App` remained the Wails binding boundary. The next step is to turn that file split into a clearer package and directory structure.

## Current Assessment

The backend is in a better state than a typical single-file Wails app:

- archive behavior is already split across focused files
- filesystem, navigation, workspace, batch, directory stream, runtime, and utility behavior already have service seams
- `App` mostly acts as a composition root and Wails delegate surface

The main weaknesses are structural rather than purely behavioral:

- the repository root is still crowded with backend files
- `app.go` is still the main Wails-facing method registry and remains the obvious hotspot
- platform-specific concerns sit beside domain concerns in one flat layout
- low-level helpers and domain logic are still mixed under the same package boundary
- new contributors still have to learn the codebase by filename pattern rather than by package boundary

## Professional Target Shape

The backend should evolve toward this structure:

```text
internal/
  app/
    app.go
    runtime.go
    bindings_*.go
    events.go
  archive/
    service.go
    browse.go
    read.go
    extract.go
    rewrite.go
    copy.go
    move.go
    types.go
    helpers.go
  filesystem/
    service.go
    listing.go
    readwrite.go
    filters.go
    stat.go
  navigation/
    service.go
    manager.go
  workspace/
    service.go
    manager.go
  batch/
    service.go
    operations.go
    types.go
  elevation/
    session.go
    helper.go
    client.go
    platform_windows.go
    platform_unix.go
  runtime/
    dir_stream.go
    pane_watch.go
    filedrop_windows.go
  utility/
    service.go
    clipboard_windows.go
    clipboard_stub.go
  platform/
    system.go
    special_folders*.go
    pathutils.go
    platform_*.go
    platform_times_*.go
    trash_*.go
  ops/
    operations.go
  contracts/
    types.go

vfs/
  ...

cmd/
  wails-app/
    main.go
```

This is the professional end state because it separates:

- Wails binding concerns
- domain services
- runtime/process integration
- platform-specific code
- shared contracts
- reusable VFS infrastructure

## Exact Package Boundaries From The Current Tree

The current repository already has enough service seams to define a concrete target ownership map.

### `internal/app`

Owns the Wails-facing composition root and binding surface.

Practical constraint:

- in Go, methods bound to `App` cannot be moved piecemeal into another package while `App` remains defined in `main`
- that makes `internal/app` a whole-type migration target rather than a safe incremental move for individual binding files
- until that broader move is justified, the compile-safe path is to keep Wails binding methods in `main` and continue shrinking the root by moving service ownership and composition helpers behind them

Current files that belong here first:

- `app.go`
- `internal/app/core.go`
- `internal/app/runner.go`
- `internal/app/services.go`
- `internal/app/runtime.go`
- `app_bindings_archive.go`
- `app_bindings_batch.go`
- `app_bindings_filesystem.go`
- `app_bindings_navigation.go`
- `app_bindings_workspace.go`
- `app_bindings_operations.go`
- `app_bindings_runtime.go`
- `app_bindings_utility.go`
- `app_bindings_elevation.go`

Likely later additions:

- app-level event adapter helpers
- the Wails bootstrap/options glue has already started moving under `internal/app`, the core manager/cache bootstrap now also lives there, internal service assembly for `NewApp` has started moving there too, and startup/shutdown runtime coordination now also routes there; the larger remaining move is relocating the `App` type and all of its bound methods together

Suggested compile-safe batch plan for the full `App` move:

1. move `NewApp`, startup/shutdown wiring, and composition-only helpers behind a new internal app facade while keeping the concrete `App` type in `main`
2. move root-owned non-method helpers that are still referenced by many `App` methods into internal packages first, so the later package flip touches fewer files
3. migrate the `App` type together with the full set of `func (a *App)` files in one package change, rather than moving binding files one by one
4. keep `main.go` as the final thin bootstrap layer that instantiates the internal app package and binds the resulting app instance to Wails

First pre-migration cleanup batch:

- filesystem raw operation helpers: `listDirectoryRaw`, `readFileRaw`, `writeFileRaw`, `deleteFileRaw`, `createDirectoryRaw`, `moveFileRaw`, and `copyFileRaw` in `filesystem.go` now delegate to `internal/filesystem/raw_service.go`
- filesystem support helpers that those raw ops still depend on: `buildFileInfoWithHidden`, `buildFallbackLinkInfo`, and `copyPath` now delegate to `internal/filesystem`; generic path normalization now lives in `internal/platform/paths.go`, with root wrappers delegating there
- elevation/filesystem coordination helpers: the old root `opTargetSingle` and `opTargetPair` helpers have been absorbed into `FilesystemService`; platform trash/elevation launch hooks still remain root-owned
- cross-domain helpers that still couple composition to `main`: `classifyError` has moved into `internal/contracts/errors.go`, and nested archive chain resolution now lives in `internal/archive/resolve.go`
- `NewApp` internal-service bootstrap now routes through `internal/app/services.go`, so `main` is no longer the only place that knows how to assemble archive, navigation, workspace, utility, and runtime-stream services
- batch constructor wiring now also routes through `internal/app/services.go`, so `main` no longer owns the `internal/batch.ServiceConfig` assembly block
- `App` now depends on narrow filesystem and operation-manager ports instead of the root concrete types directly, which reduces the type-level coupling that still blocks the eventual package move
- archive-operation adapter wiring and batch error-envelope construction now also live in `internal/app/services.go`, further reducing `NewApp` to callback binding rather than service-specific adapter code
- elevated list/copy/move/delete callback assembly now routes through `FilesystemService` methods, so `NewApp` no longer rebuilds those elevated filesystem adapters inline
- the remaining runtime/service composition hooks in `NewApp` now mostly use named `App` helper methods rather than anonymous closures, which further shrinks the eventual `App` relocation diff
- the `App` filesystem port no longer exposes low-level elevation-target helpers or raw elevated dispatch methods, only the higher-level operations that the composition layer actually uses
- `App` now stores narrow ports for archive, runtime, batch, dir-stream, utility, navigation, workspace, elevation, and pane-watch behavior as well, so most of the remaining concrete-type coupling has been collapsed to constructor-time assembly instead of struct ownership
- `FilesystemService` now also depends on a narrow elevation client provider rather than the concrete elevation service type, which trims one more root-level concrete dependency from filesystem orchestration
- `main.App` now wraps an `internal/app` state holder instead of owning the full concrete backend field set directly, so app state ownership has started moving behind the internal package boundary while the Wails-facing wrapper stays stable
- the service-port definitions that `App` used to duplicate in `main` are now owned by `internal/app/state.go`, so the root wrapper no longer carries its own parallel state-interface layer
- state/runtime/service assembly for `NewApp` now also routes through `internal/app/builder.go`, so root construction is down to core bootstrap, root-only adapter creation, and wrapper instantiation
- VFS host state and mount/list/open/unmount behavior now also live on `internal/app.State`, with `app_vfs.go` reduced to Wails-facing wrapper methods
- startup, shutdown, and event emission now also route through methods on `internal/app.State`, leaving root `app.go` with less direct lifecycle behavior
- pane-watch creation/cleanup, dir-stream cancellation, elevation session shutdown, and archive-cache close now also live on `internal/app.State`, trimming more state-only composition behavior out of the root wrapper
- operation start, conflict-event emission, and nested-archive extraction callbacks now also route through `internal/app.State`, leaving fewer state behaviors behind in root helper files
- the old root `App` composition-helper methods are now gone; the remaining root-only construction glue is down to plain package adapters such as the elevated-client launcher and file-info callback
- root `app.go` no longer owns startup/shutdown/event wrapper methods either; `main.go` now passes direct closures into the internal state lifecycle, leaving `app.go` effectively as wrapper type plus constructor

Why this batch comes first:

- those helpers are shared by many `func (a *App)` methods indirectly through filesystem, batch, VFS, and elevation flows
- moving them first reduces the number of mixed package references that would otherwise block a whole-type `App` package flip
- the remaining `App` move then becomes closer to a package relocation than a behavior rewrite

### `internal/archive`

Owns archive-specific browsing, reading, extraction, rewrite, and mutation behavior.

Current files that map here:

- `internal/archive/service.go`
- `internal/archive/helpers.go`
- `internal/archive/extract.go`
- `internal/archive/extract_helpers.go`
- `internal/archive/rewrite.go`
- `internal/archive/nested.go`
- `internal/archive/resolve.go`
- `internal/contracts/archive.go`
- `archive_types.go`

Important note:

- the archive service wrapper has moved into `internal/archive/service.go` and now follows the same injected-closure pattern as navigation, which removes one more service boundary from `main`
- the cache-backed archive browse and archive read logic now live in that internal archive service as well, so the remaining root-owned archive files are mostly mutation, extraction, and helper flows
- archive copy and move orchestration now live in `internal/archive/service.go` too, with `main` only bridging operation lifecycle and the still-root-owned extract and rewrite implementations
- archive delete orchestration now lives in `internal/archive/service.go` as well, and rewrite dispatch plus concrete rewrite implementations moved there too
- archive extraction orchestration now also lives in `internal/archive/service.go`; nested-archive extraction plumbing moved there as a shared archive-domain helper and batch now depends on the archive service for chain resolution instead of calling an App-owned helper
- concrete extraction helpers and extraction-specific helper functions also moved under `internal/archive`, and batch now imports archive-format helpers from that package directly instead of depending on duplicate root helpers
- the old root archive read-format implementation became redundant after the nested and read flows moved, and the last duplicate root archive helper files have now been removed as batch/archive ownership converges under `internal/archive`
- archive request/result contracts have now started moving into `internal/contracts`, with `archive_types.go` reduced to aliases at the main-package boundary.
- archive browse/list and archive read response contracts now share ownership with filesystem listing/read contracts under `internal/contracts/filesystem.go`, since the underlying wire shapes are the same `ArchiveListing`, `FileInfo`, and `FileContent` structures used across both domains.
- VFS nested archive chain opening now also routes through `internal/archive/resolve.go`, so `main` no longer owns the temp-file nested-archive opener used by the archive cache and VFS mount flow.

### `internal/filesystem`

Owns real filesystem orchestration, listing, read/write operations, filtering, disk usage, and stat-related helpers.

Current files that map here:

- `filesystem_service.go`
- `filesystem.go`
- `disk_usage.go`
- `disk_usage_cache.go`
- `disk_usage_windows.go`
- `disk_usage_unix.go`
- `stat.go`

Files that may stay adjacent but depend on this package:

- `app_bindings_filesystem.go`
- `internal/runtime/dir_stream.go`

Current status:

- elevated request dispatch already routes through an `internal/filesystem` adapter
- raw filesystem operations and their copy/file-info helper implementations now live behind `internal/filesystem/raw_service.go`
- disk-usage caching now also lives under `internal/filesystem/disk_usage_cache.go`
- the remaining root filesystem code is mostly Wails-facing delegation plus platform callback wiring

### `internal/platform`

Owns generic path normalization plus OS-specific helpers and low-level platform details.

Current files that map here:

- `internal/platform/paths.go`
- `internal/platform/links_windows.go`
- `internal/platform/links_unix.go`
- `internal/platform/metadata_windows.go`
- `internal/platform/metadata_unix.go`
- `internal/platform/metadata_linux.go`
- `internal/platform/metadata_darwin.go`
- `internal/platform/trash_windows.go`
- `internal/platform/trash_unix.go`
- `internal/platform/system.go`
- `internal/platform/system_windows.go`
- `internal/platform/system_unix.go`
- `internal/platform/special_folders_windows.go`
- `internal/platform/special_folders_unix.go`
- `osinfo.go`
- `special_folders.go`

Current status:

- generic slash-cleaning, normalized join, parent-path, split-path, and breadcrumb helpers now live in `internal/platform/paths.go`
- filesystem metadata callbacks now route through `internal/platform/metadata_*.go`, and trash handling now routes through `internal/platform/trash_*.go`
- utility-facing shell open and link-resolution behavior now routes through `internal/platform/links_*.go`
- OS info, drive enumeration, and special-folder discovery now route through `internal/platform/system*.go` and `internal/platform/special_folders_*.go`, with `main` only re-exporting contracts and Wails delegates
- root files like `pathutils.go` and `utils.go` now mostly expose Wails-facing or package-local wrappers over that internal platform boundary
- disk-usage wire types now also live under `internal/contracts/filesystem.go`, with `main` only re-exporting them via aliases

### `internal/navigation`

Owns pane path history and navigation orchestration.

Current files that map here:

- `internal/navigation/manager.go`
- `internal/navigation/service.go`

Current status:

- the navigation history manager has been extracted into `internal/navigation`
- filesystem listing contracts now live under `internal/contracts`
- the navigation service wrapper has moved into `internal/navigation`

### `internal/workspace`

Owns workspaces, persisted preferences, and workspace manager behavior.

Current files that map here:

- `internal/workspace/store.go`
- `internal/workspace/models.go`
- `internal/workspace/manager.go`
- `internal/workspace/service.go`

Current status:

- the byte-level persistence store lives in `internal/workspace`
- workspace-local persistence models live in `internal/workspace/models.go`
- the workspace manager and service have moved into `internal/workspace`
- frontend-facing workspace and navigation wire types now live under `internal/contracts` and are re-exported from `main` via type aliases in `types.go`

### `internal/batch`

Owns long-running batch operations and result typing.

Current files that map here:

- `internal/contracts/batch.go`
- `internal/contracts/operations.go`
- `internal/batch/service.go`
- `internal/batch/conflicts.go`
- `internal/batch/extract.go`
- `internal/batch/fileops.go`
- `internal/batch/progress.go`
- `internal/batch/selective_extract.go`
- `internal/batch/transfers.go`
- `internal/ops/manager.go`
- `batch_service.go`
- `batch_types.go`

Important note:

- if `operations.go` remains the shared progress-operation registry rather than pure batch behavior, it can also live under `internal/ops`; the deciding factor should be whether non-batch domains need to own operation lifecycle directly.
- batch request/result and conflict contracts now live in `internal/contracts/batch.go`, with `batch_types.go` reduced to main-package aliases plus internal-only helper types.
- operation type, status, and progress wire models now live in `internal/contracts/operations.go`, and the live operation manager/runtime bookkeeping now live in `internal/ops/manager.go`; `main` only re-exports the types and wires the event emitter during startup.
- the public batch composition root now lives in `internal/batch/service.go`; it owns conflict coordination, the conflict prompt adapter boundary, throttled progress tracking, streaming file operations, full-archive extraction, selective extraction, and transfer loop orchestration.
- copy, move, and delete loop orchestration now also live under `internal/batch/transfers.go`, and `batch_service.go` has been reduced to Wails-facing method forwarding.
- the former root batch core and `batch_operations.go` have been removed, and batch callback/config assembly now routes through `internal/app/services.go`, so `main` no longer owns batch conflict resolution or constructor wiring.

### `internal/elevation`

Owns elevated-session state, helper coordination, and platform-specific elevation behavior.

Current files that map here:

- `internal/elevation/service.go`
- `elevated_service.go`
- `elevation_helper.go`
- `elevation_windows.go`
- `elevation_unix.go`

Current status:

- the elevated session lifecycle and approval wait-state now live in `internal/elevation/service.go`
- the elevated IPC/client/helper control flow now lives under `internal/elevation`, with the root package reduced to the raw elevated request handler plus platform launch helpers
- the remaining root elevation code is now mostly callback wiring into an `internal/filesystem` raw-operation adapter, rather than request-switch orchestration logic

### `internal/runtime`

Owns runtime bridge behavior that exists because the app is a live desktop host.

Current files that map here:

- `internal/runtime/dir_stream.go`
- `internal/runtime/pane_watch.go`
- `internal/runtime/native_filedrop.go`
- `internal/runtime/filedrop_windows.go`
- `internal/runtime/vfs.go`
- `filedrop_windows.go`
- `app_vfs.go`

Important note:

- `internal/runtime/dir_stream.go` is close to filesystem behavior, but its lifecycle and event-stream semantics make it a better runtime-facing boundary than a pure domain package.
- native Wails file-drop event parsing and the Windows host drop helper now also live under `internal/runtime`, with `main` keeping only the bound wrapper methods
- VFS mount/open/list orchestration now also lives under `internal/runtime/vfs.go`, with `app_vfs.go` reduced to thin `App` wrappers around the unified VFS host

### `internal/utility`

Owns small cross-domain helpers that do not justify larger domain packages.

Current files that map here:

- `internal/utility/service.go`
- `internal/utility/clipboard_windows.go`
- `internal/utility/clipboard_stub.go`

Current status:

- file opening and link resolution now route from `App` into `internal/utility/service.go`, which in turn depends on `internal/platform`
- file-drop normalization and clipboard-backed Explorer fallback now live under `internal/utility`, removing the last root-owned utility service implementation files

### `internal/contracts`

Owns frontend-facing or cross-domain wire contracts once they are cleaned up.

Current files that likely contribute here:

- `internal/contracts/filesystem.go`
- `internal/contracts/errors.go`
- `internal/contracts/batch.go`
- `internal/contracts/platform.go`
- `internal/contracts/workspace.go`
- `internal/contracts/navigation.go`
- `types.go` now re-exports selected contracts via type aliases
- parts of `archive_types.go`
- parts of `batch_types.go`

Important note:

- this package is now introduced for workspace and navigation wire types. Archive, batch, and filesystem contracts should move later only when their ownership is equally clear.

### `vfs`

This should remain a top-level package tree, not be absorbed under `internal/*`, because it is already a coherent subsystem with its own adapters and archive providers.

Current files and folders that stay here:

- `vfs/vfs.go`
- `vfs/unified.go`
- `vfs/osfs`
- `vfs/mount`
- `vfs/registry`
- `vfs/archive`

## Important Constraint

Do not jump straight from the current flat `package main` layout into a many-package rewrite.

That kind of migration usually creates:

- circular dependencies
- duplicated wire types
- package churn without real ownership clarity
- broken Wails bindings during a long transition

The correct approach is a staged migration with compile-safe boundaries.

## Recommended Phases

### Phase A: Finish The Flat-Layout Cleanup

Objective: make the current root-level layout internally coherent before moving files into packages.

Actions:

- split remaining `app.go` Wails delegates into focused binding files such as:
  - `app_bindings_filesystem.go`
  - `app_bindings_navigation.go`
  - `app_bindings_workspace.go`
  - `app_bindings_operations.go`
  - `app_bindings_runtime.go`
  - `app_bindings_utility.go`
  - `app_bindings_elevation.go`
- keep `app.go` focused on struct definition, construction, and high-level lifecycle
- move any remaining non-binding helper logic out of binding files
- standardize constructor patterns and field naming across services

Current status:

- `app.go` has already been reduced to composition-root and lifecycle responsibilities
- Wails-facing filesystem, navigation, workspace, operations, runtime, utility, and elevation delegates now live in domain-specific `app_bindings_*.go` files
- batch-facing Wails delegates now also live in `app_bindings_batch.go`, and path/system helper bindings have been consolidated into the existing filesystem and utility binding files
- archive-specific VFS browse helpers now live outside `app.go` in `app_archive_vfs.go`

Why this phase matters:

- it reduces `app.go` as a hotspot without introducing package churn
- it makes later package extraction mostly a file-move exercise

Acceptance criteria:

- `app.go` contains composition-root logic only
- Wails-facing methods are grouped by domain in thin files
- `go build .` passes with no behavior changes

### Phase B: Introduce Internal Domain Packages

Objective: move real backend ownership into `internal/*` packages while keeping `App` as the only Wails binding surface.

Suggested extraction order:

1. `internal/navigation`
2. `internal/workspace`
3. `internal/batch`
4. `internal/elevation`
5. `internal/filesystem`
6. `internal/archive`

Why this order:

- navigation and workspace are relatively self-contained
- batch and elevation already have visible service seams
- filesystem and archive have the heaviest dependency surfaces and should move later

Reality check:

- `internal/navigation` can move first in a partial form, with the pure history manager extracted before the Wails-facing service wrapper
- `internal/workspace` is the first slice likely to force a contracts decision, because `WorkspaceManager` owns `WorkspaceConfig`, `WorkspaceList`, and `UserPreferences` from the main wire-type set
- the safe first workspace slice is the storage layer, which has now been extracted without moving the wire-type-owning manager

Rules:

- `App` imports services; services do not import `App`
- cross-domain dependencies must go through narrow interfaces, not concrete `App` references
- wire/request/response types should be owned deliberately, not copied package-by-package

Acceptance criteria:

- service packages compile independently of Wails
- `main.go` and Wails binding setup still only depend on `App`
- no circular imports

### Phase C: Separate Runtime Integration From Domain Logic

Objective: isolate behavior that exists only because this is a desktop app host.

Move these concerns away from domain packages:

- startup/shutdown runtime hooks
- event emission adapters
- pane directory watch orchestration
- file-drop integration
- elevated helper process launch and transport glue

Suggested target packages:

- `internal/app`
- `internal/runtime`
- `internal/elevation`

Acceptance criteria:

- runtime glue is distinct from file-management domain logic
- domain services can be understood without knowing Wails internals

### Phase D: Consolidate Platform Code

Objective: stop scattering OS-specific behavior across the repository root.

Group these under a clearer platform boundary:

- trash
- special folders
- path and filesystem metadata helpers
- process elevation helpers
- file timestamps
- OS info helpers

Suggested target package:

- `internal/platform`

Rules:

- platform package provides capability functions
- domain services decide when to call them
- avoid putting business rules into platform helpers

### Phase E: Stabilize Contracts

Objective: prevent type drift while package boundaries get stricter.

Actions:

- identify wire types exposed to the frontend and keep them in one owned location
- identify internal domain types and avoid leaking them directly into Wails bindings
- stop using generic helper files as shared dependency dumping grounds

Suggested boundaries:

- frontend-facing contracts: one package or one clearly-owned file cluster
- internal service-only types: local to each domain package unless truly shared

## What To Avoid

Avoid these common failure modes:

- creating `pkg/` packages for app-internal code that is not meant to be reused externally
- moving files into folders without changing ownership or dependency direction
- extracting packages that still need broad access to `App`
- duplicating request/result structs in each package to avoid dependency design
- mixing platform helpers with domain orchestration in the same package
- doing archive and filesystem package extraction in the same slice as large frontend changes

## Immediate, Low-Risk Next Step

The best next implementation slice is:

1. keep the current flat root temporarily
2. split `app.go` into domain-specific Wails binding files
3. document the intended package ownership in code comments and docs
4. then start with `internal/navigation` and `internal/workspace`

That gives a visible improvement quickly and lowers the risk of later package moves.

## Proposed Near-Term File Layout

Before introducing `internal/*`, a cleaner root layout can already look like this:

```text
app.go
app_archive_vfs.go
app_bindings_archive.go
app_bindings_batch.go
app_bindings_filesystem.go
app_bindings_navigation.go
app_bindings_operations.go
app_bindings_workspace.go
app_bindings_runtime.go
app_bindings_utility.go
app_bindings_elevation.go

archive_*.go
batch_*.go
filesystem*.go
navigation*.go
workspace*.go
elevation*.go
dir_stream*.go
operations.go
types.go

platform_*.go
platform_times_*.go
trash_*.go
osinfo.go
pathutils.go
stat.go

app_vfs.go
filedrop_windows.go
main.go
```

This is not the final state, but it makes ownership more obvious immediately.

## Suggested Decision

Yes, the backend can be reorganized professionally.

The right definition of “professional” here is:

- thin Wails binding boundary
- explicit domain packages
- isolated runtime/platform glue
- no `App`-centric dependency gravity
- stable contracts
- staged migration with working builds after every slice

Do not treat this as a one-shot folder shuffle. Treat it as a controlled package-boundary program.

## Recommended First Milestone

If you want to start now, the first milestone should be:

### Milestone 1: Binding Surface Cleanup

Scope:

- split `app.go` into domain-specific binding files
- keep `NewApp`, struct wiring, and lifecycle in `app.go`
- leave all backend packages as-is for now

Current result:

- this milestone is now partially implemented in the current tree
- the next step is no longer "split the bindings"; it is "start extracting the safest service packages"

Success criteria:

- `app.go` becomes short and boring
- Wails binding methods become easy to locate by domain
- no user-visible behavior changes
- `go build .` passes

That is the best foundation for the next package-level reorganization.
