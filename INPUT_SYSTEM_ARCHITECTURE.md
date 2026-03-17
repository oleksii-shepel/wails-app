# Input System Architecture

## Objective

Replace the current mix of global shortcuts and component-local key handlers with a unified input system that supports:

- VS Code-style keybinding resolution
- Chord shortcuts such as `Ctrl+K Ctrl+C`
- Mouse gestures
- Plugin-provided bindings
- Dynamic rebinding at runtime
- Context priority resolution

The immediate goal of this document is to define the dialog and overlay binding surface so the refactor starts from a complete inventory instead of ad hoc handlers.

## Current State

The current frontend input model is mid-migration and still has three visible layers:

- Engine-native contexts and bindings in `frontend/src/app/services/input-engine.service.ts`
- Legacy shortcut definitions registered through `frontend/src/app/services/keyboard.service.ts` and `frontend/src/app/services/shortcut-registry.service.ts`
- Local `HostListener` or template `keydown` handlers inside surfaces that have not been fully moved to command-only handling

This creates three concrete problems:

- Dialog bindings are not registered in one place, so conflicts and gaps are hard to see.
- Overlay components use inconsistent ownership; some handle keys locally while others have no keyboard layer at all.
- The current global shortcut service only supports single key combinations and a flat `Map`, which cannot model chords, plugin precedence, or scoped rebinding.

## Dialog And Overlay Inventory

### Real Dialogs / Modals

| Surface | Current Visibility Owner | Current Input Owner | Current Bindings | Gaps |
|---|---|---|---|---|
| Confirm dialog | `ConfirmDialogService.activeRequest$` | `confirm-dialog.component.ts` | `Escape` closes; alerts map `Escape` to confirm; overlay click closes | No `Enter` default action; no focus model; no centralized binding ID |
| Archive move warning | `archiveMoveWarning` on pane/view model | local buttons only | Backdrop click cancels | No keyboard bindings at all |
| Global search | `global-search.component.ts` | local `HostListener` | `Escape`, `ArrowUp`, `ArrowDown`, `Enter` | No command IDs; settings modal is nested but not its own context |
| Keyboard help | `keyboard-help.component.ts` | local `HostListener` plus global registry | `?`, `Ctrl+/`, `Escape` | Self-toggle logic lives outside shared resolver; no scoped search navigation |
| Path history viewer | `path-history-viewer.component.ts` | buttons and inputs only | implicit text input behavior | No dialog-level `Escape`, list navigation, or open/remove bindings |
| Filter custom dialog | `filter-bar.component.ts` state | parent closes on `Escape` | `Escape` only via parent doc listener | No local dialog context or explicit actions |

### Overlay Panels That Need First-Class Input Contexts

| Surface | Current Visibility Owner | Current Input Owner | Current Bindings | Gaps |
|---|---|---|---|---|
| Address bar quick access | `address-bar.component.ts` | `address-bar-quick-access.component.ts` | `Tab`, `Shift+Tab`, `ArrowUp`, `ArrowDown`, `Enter` | No `Escape` locally; global service special-cases it indirectly |
| Address bar edit | `address-bar-edit.component.ts` | local input handler | `Enter`, `Escape`, `Tab`, `Shift+Tab`, arrows | Not part of central registry |
| Drive dropdown | `address-bar.component.ts` | none | click only | No keyboard support |
| Sibling dropdown | `address-bar.component.ts` | none | click only | No keyboard support |
| Workspace selector menu | `workspace-selector.component.ts` | none | click only | No keyboard support |
| Filter preset/category dropdowns | `filter-bar.component.ts` | parent doc listener | `Escape` closes | No arrow navigation or select bindings |
| Filter widget | `filter-widget.component.ts` | local input handler | `Escape`, `Enter`, `Shift+Enter`, `F3`, `Shift+F3` | Not described as a scoped context in the global system |

## Target Architecture

### 1. Commands, Not Inline Closures

Bindings should target command IDs rather than component closures.

Examples:

- `dialog.confirm.accept`
- `dialog.confirm.cancel`
- `dialog.globalSearch.nextResult`
- `dialog.globalSearch.openSelected`
- `overlay.quickAccess.nextItem`
- `overlay.workspaceSelector.close`

Components can still implement the behavior, but command registration and binding registration must be separate concerns.

### 2. Resolver Model

The input engine should resolve events using this order:

1. Active modal/dialog context
2. Active overlay context
3. Focused widget context
4. Active pane context
5. Global context

Within the same context level, choose the highest priority binding by:

1. Context specificity
2. Source priority: user > workspace > plugin > default
3. Explicit numeric priority
4. Most recent registration as a final deterministic tie-breaker

### 3. Context Stack

The engine should maintain a stack of input contexts rather than infer everything from DOM traversal.

Each context should define:

- `id`
- `kind`: `global`, `pane`, `dialog`, `overlay`, `widget`
- `priority`
- optional parent context
- activation predicate or lifecycle hook

This is required for nested surfaces such as:

- Global search -> settings dialog
- Keyboard help -> import file flow -> confirm alert
- Pane -> address bar -> quick access overlay

### 4. Multi-Step Sequences

The engine must support keybinding sequences, not only single combinations.

Examples:

- `Ctrl+K Ctrl+C`
- `Ctrl+K Ctrl+S`
- `G G`

Sequence handling requirements:

- timeout window per chord sequence
- prefix matching
- cancellation on unmatched continuation
- visible pending-chord state so the UI can hint when the engine is waiting for the second step

### 5. Mouse Gestures

Mouse bindings should be first-class entries, not ad hoc pointer handlers.

Initial gesture families:

- click variants: left, middle, right, double-click
- wheel with modifiers
- drag gestures with button and modifier state
- path gestures such as right-drag menu triggers

Examples:

- `mouse.doubleClick.left` -> `pane.openSelection`
- `mouse.drag.right` -> `pane.showDragOperationMenu`
- `mouse.wheel.ctrl` -> `viewer.zoom`

### 6. Dynamic Rebinding

Bindings must be editable at runtime and reloaded without restarting the app.

Required layers:

- built-in defaults
- plugin defaults
- workspace overrides
- user overrides

The current import/export behavior in `keyboard.service.ts` can become one persistence layer, but the runtime engine should treat it as data rather than direct mutable state on shortcut objects.

### 7. Plugin Bindings

Plugins should be able to contribute:

- commands
- contexts
- keybindings
- mouse gestures

Plugin bindings must declare:

- stable source ID
- binding source type `plugin`
- optional when/context expression
- optional weight/priority

## Canonical Dialog Binding Matrix

This matrix defines the initial target command surface for every dialog-like UI.

### Confirm Dialog

Context ID: `dialog.confirm`

| Binding | Command |
|---|---|
| `Enter` | `dialog.confirm.accept` |
| `Escape` | `dialog.confirm.cancel` |
| `Tab` / `Shift+Tab` | `dialog.confirm.focusNext` / `dialog.confirm.focusPrev` |

### Archive Move Warning

Context ID: `dialog.archiveMoveWarning`

| Binding | Command |
|---|---|
| `Enter` | `dialog.archiveMoveWarning.confirm` |
| `Escape` | `dialog.archiveMoveWarning.cancel` |
| `Tab` / `Shift+Tab` | `dialog.archiveMoveWarning.focusNext` / `dialog.archiveMoveWarning.focusPrev` |

### Global Search

Context ID: `dialog.globalSearch`

| Binding | Command |
|---|---|
| `Escape` | `dialog.globalSearch.close` |
| `ArrowUp` | `dialog.globalSearch.prevResult` |
| `ArrowDown` | `dialog.globalSearch.nextResult` |
| `Enter` | `dialog.globalSearch.openSelected` |
| `Ctrl+,` | `dialog.globalSearch.openSettings` |

Nested context ID: `dialog.globalSearch.settings`

| Binding | Command |
|---|---|
| `Escape` | `dialog.globalSearch.settings.close` |
| `Enter` | `dialog.globalSearch.settings.apply` |

### Keyboard Help

Context ID: `dialog.keyboardHelp`

| Binding | Command |
|---|---|
| `Escape` | `dialog.keyboardHelp.close` |
| `Ctrl+F` | `dialog.keyboardHelp.focusSearch` |
| `ArrowUp` | `dialog.keyboardHelp.prevItem` |
| `ArrowDown` | `dialog.keyboardHelp.nextItem` |
| `Space` | `dialog.keyboardHelp.toggleSelectedBinding` |
| `Ctrl+S` | `dialog.keyboardHelp.exportBindings` |

### Path History Viewer

Context ID: `dialog.pathHistory`

| Binding | Command |
|---|---|
| `Escape` | `dialog.pathHistory.close` |
| `Ctrl+F` | `dialog.pathHistory.focusFilter` |
| `ArrowUp` | `dialog.pathHistory.prevItem` |
| `ArrowDown` | `dialog.pathHistory.nextItem` |
| `Enter` | `dialog.pathHistory.openSelected` |
| `Delete` | `dialog.pathHistory.removeSelected` |
| `Ctrl+Delete` | `dialog.pathHistory.clearAll` |

### Filter Widget

Context ID: `widget.filterInline`

| Binding | Command |
|---|---|
| `Escape` | `widget.filterInline.close` |
| `Enter` | `widget.filterInline.nextMatch` |
| `Shift+Enter` | `widget.filterInline.prevMatch` |
| `F3` | `widget.filterInline.nextMatch` |
| `Shift+F3` | `widget.filterInline.prevMatch` |

### Address Bar Quick Access

Context ID: `overlay.quickAccess`

| Binding | Command |
|---|---|
| `Escape` | `overlay.quickAccess.close` |
| `Tab` | `overlay.quickAccess.nextSection` |
| `Shift+Tab` | `overlay.quickAccess.prevSection` |
| `ArrowUp` | `overlay.quickAccess.prevItem` |
| `ArrowDown` | `overlay.quickAccess.nextItem` |
| `Enter` | `overlay.quickAccess.openSelected` |
| `Delete` | `overlay.quickAccess.removePinnedSelected` |

### Address Bar Drive Dropdown

Context ID: `overlay.addressBar.drives`

| Binding | Command |
|---|---|
| `Escape` | `overlay.addressBar.drives.close` |
| `ArrowUp` | `overlay.addressBar.drives.prevItem` |
| `ArrowDown` | `overlay.addressBar.drives.nextItem` |
| `Enter` | `overlay.addressBar.drives.openSelected` |

### Address Bar Sibling Dropdown

Context ID: `overlay.addressBar.siblings`

| Binding | Command |
|---|---|
| `Escape` | `overlay.addressBar.siblings.close` |
| `ArrowUp` | `overlay.addressBar.siblings.prevItem` |
| `ArrowDown` | `overlay.addressBar.siblings.nextItem` |
| `Enter` | `overlay.addressBar.siblings.openSelected` |

### Workspace Selector

Context ID: `overlay.workspaceSelector`

| Binding | Command |
|---|---|
| `Escape` | `overlay.workspaceSelector.close` |
| `ArrowUp` | `overlay.workspaceSelector.prevItem` |
| `ArrowDown` | `overlay.workspaceSelector.nextItem` |
| `Enter` | `overlay.workspaceSelector.activateSelected` |
| `Delete` | `overlay.workspaceSelector.deleteSelected` |

## Migration Plan

### Phase A

- Introduce typed input-system model and dialog binding catalog.
- Bridge existing keyboard shortcut registrations through the new engine.
- Treat the new catalog as read-only source of truth for design and audit.

### Phase B

- Replace `keyboard.service.ts` flat `Map` with a resolver that understands contexts, source priority, and chords.
- Move dialog key handlers from local `HostListener` logic into command bindings where feasible.

### Phase C

- Add mouse gesture registry.
- Add plugin contribution API.
- Add UI for live rebinding and conflict inspection.

## Start Point For This Refactor

The next implementation step should be to introduce a new input-system service alongside the existing keyboard service, then migrate dialog contexts one by one in this order:

1. Confirm dialog
2. Global search
3. Keyboard help
4. Path history viewer
5. Address bar overlays

## Implementation Status

Implemented so far:

- `frontend/src/app/services/input-engine.service.ts` provides the first live input-engine scaffold with context registration, command registration, source-priority binding resolution, plugin binding registration, and pending chord state.
- dialog contexts are live for:
	- `frontend/src/app/components/confirm-dialog.component.ts`
	- `frontend/src/app/components/global-search.component.ts`
	- `frontend/src/app/components/keyboard-help.component.ts`
	- `frontend/src/app/components/path-history-viewer.component.ts`
	- `frontend/src/app/components/archive-move-warning.component.ts`
- overlay contexts are live for:
	- `frontend/src/app/components/address-bar-quick-access.component.ts`
	- `frontend/src/app/components/address-bar-drive-dropdown.component.ts`
	- `frontend/src/app/components/address-bar-sibling-dropdown.component.ts`
	- `frontend/src/app/components/workspace-selector.component.ts`
	- `frontend/src/app/components/filter-dropdown.component.ts`
	- preset-overlay command routing in `frontend/src/app/components/filter-bar.component.ts`
- widget contexts are live for:
	- `frontend/src/app/components/filter-widget.component.ts`
	- `frontend/src/app/components/address-bar-edit.component.ts`
- `frontend/src/app/services/input-context.service.ts` provides lease-counted context activation so overlays and dialogs can opt in without duplicating lifecycle code.
- command registration is now owner-aware in `frontend/src/app/services/input-engine.service.ts`, which allows identical command IDs to be registered safely from multiple component instances.

Also implemented:

- `frontend/src/app/services/keyboard.service.ts` now acts as a compatibility adapter that registers legacy global shortcuts into `frontend/src/app/services/input-engine.service.ts` instead of dispatching them through a separate listener.
- `frontend/src/app/services/shortcut-registry.service.ts` still defines the default global shortcut set, but those bindings now resolve through the shared engine with DOM-aware `when` guards and per-binding propagation control.

Still pending:

- live chord bindings now exist for global navigation sequences, with pending-chord state exposed to the UI
- runtime mouse gesture dispatch now exists in the input engine for click, double-click, button, and wheel gestures
- browser-shell mouse bindings now cover file-entry double-click open and viewer `Ctrl+Wheel` zoom through the shared input engine, using DOM-aware `when` guards for `fileEntry` and `viewerSurface`
- some components still retain local key handlers as fallback behavior, and mouse gesture coverage is still intentionally sparse until more commands are migrated

That order gives the highest coverage first while keeping the migration risk low.