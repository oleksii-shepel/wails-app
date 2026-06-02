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

The frontend input model has completed its core migration. The architecture now consists of:

- `frontend/src/app/services/input-engine.service.ts` — the primary engine with context-aware resolution, command registration, source-priority binding resolution, plugin binding registration, and pending chord state.
- `frontend/src/app/services/dialog-input-catalog.ts` — canonical binding catalog for all dialogs and overlays.
- Component-level command registration via `InputEngineService` for dialogs, overlays, and widgets.
- `frontend/src/app/services/keyboard.service.ts` — now acts as a compatibility adapter that registers legacy global shortcuts into the shared engine instead of dispatching through a separate listener.
- `frontend/src/app/services/shortcut-registry.service.ts` — still defines the default global shortcut set, but those bindings now resolve through the shared engine with DOM-aware `when` guards and per-binding propagation control.

A small number of local `HostListener` or template `keydown` handlers remain as fallback behavior in a few surfaces, but the canonical path for all new bindings is command-first.

## Dialog And Overlay Inventory

### Dialogs / Modals

All dialogs below now register commands via `InputEngineService` and acquire their input context via `InputContextService`. Bindings are defined in `frontend/src/app/services/dialog-input-catalog.ts`.

| Surface | Context ID | Bindings |
|---|---|---|
| Confirm dialog | `dialog.confirm` | `Enter` (accept), `Escape` (cancel), `Tab` / `Shift+Tab` (focus), `ArrowLeft` / `ArrowRight` (focus) |
| Archive move warning | `dialog.archiveMoveWarning` | `Enter` (confirm), `Escape` (cancel), `Tab` / `Shift+Tab` (focus), `ArrowLeft` / `ArrowRight` (focus) |
| Global search | `dialog.globalSearch` | `Escape` (close), `ArrowUp` / `ArrowDown` (navigate), `Enter` (open), `Ctrl+Enter` (open folder), `Ctrl+,` (settings) |
| Global search settings | `dialog.globalSearch.settings` | `Escape` (close), `Enter` (apply), `ArrowLeft` / `ArrowRight` (focus) |
| Keyboard help | `dialog.keyboardHelp` | `Escape` (close), `Ctrl+F` (focus search), `Ctrl+S` (export), `ArrowUp` / `ArrowDown` (navigate), `Space` (toggle) |
| Path history viewer | `dialog.pathHistory` | `Escape` (close), `Ctrl+F` (focus filter), `ArrowUp` / `ArrowDown` (navigate), `Enter` (open), `Delete` (remove), `Ctrl+Delete` (clear all) |
| Filter custom dialog | `filter-bar.component.ts` | `Escape` closes via parent listener; inline filter widget handles its own keys |

### Overlays

All overlays below now have dedicated input contexts and command bindings.

| Surface | Context ID | Bindings |
|---|---|---|
| Address bar quick access | `overlay.quickAccess` | `Escape` (close), `Tab` / `Shift+Tab` (section), `ArrowUp` / `ArrowDown` (item), `Enter` (open), `Delete` (remove pin) |
| Address bar edit | `overlay.addressBar.edit` | `Enter`, `Escape`, `Tab`, `Shift+Tab`, arrows |
| Drive dropdown | `overlay.addressBar.schemes` | `Escape` (close), `ArrowUp` / `ArrowDown` (item), `ArrowLeft` / `ArrowRight` (column), `Enter` (open) |
| Sibling dropdown | `overlay.addressBar.siblings` | `Escape` (close), `ArrowUp` / `ArrowDown` (item), `Enter` (open) |
| Workspace selector menu | `overlay.workspaceSelector` | `Escape` (close), `ArrowUp` / `ArrowDown` (item), `Enter` (activate), `Delete` (delete) |
| Filter preset/category dropdowns | `overlay.filter.presets` | `Escape` (close), `ArrowUp` / `ArrowDown` (item), `Enter` (select) |
| Filter widget | `widget.filterInline` | `Escape` (close), `Enter` / `F3` (next match), `Shift+Enter` / `Shift+F3` (prev match) |

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

### Phase A ✅ Complete

- Introduced typed input-system model and dialog binding catalog.
- Bridged existing keyboard shortcut registrations through the new engine.
- Catalog is the read-only source of truth for design and audit.

### Phase B ✅ Complete

- `keyboard.service.ts` now acts as a compatibility adapter registering into the shared engine.
- Dialog key handlers moved from local `HostListener` logic into command bindings.
- Resolver supports contexts, source priority, and chords.

### Phase C 🔄 In Progress

- Mouse gesture dispatch exists in the input engine for click, double-click, button, and wheel gestures.
- Browser-shell mouse bindings cover file-entry double-click open and viewer `Ctrl+Wheel` zoom.
- Plugin contribution API surface is designed but not yet wired end-to-end.
- UI for live rebinding and conflict inspection is not yet implemented.

## Start Point For This Refactor

All dialog and overlay contexts listed above are live. Further work should focus on:

1. Expanding mouse gesture coverage as more commands are migrated.
2. Implementing the live-rebinding UI.
3. Completing the plugin binding contribution API.

## Implementation Status

All core input system features are implemented:

- `frontend/src/app/services/input-engine.service.ts` — context registration, command registration, source-priority binding resolution, plugin binding registration, pending chord state, and mouse gesture dispatch.
- `frontend/src/app/services/dialog-input-catalog.ts` — canonical binding catalog for all dialogs and overlays.
- `frontend/src/app/services/input-context.service.ts` — lease-counted context activation.

Dialog contexts:
- `dialog.confirm`
- `dialog.archiveMoveWarning`
- `dialog.globalSearch`
- `dialog.globalSearch.settings`
- `dialog.keyboardHelp`
- `dialog.pathHistory`

Overlay contexts:
- `overlay.quickAccess`
- `overlay.addressBar.schemes`
- `overlay.addressBar.siblings`
- `overlay.workspaceSelector`
- `overlay.filter.presets`

Widget contexts:
- `widget.filterInline`
- `overlay.addressBar.edit`

Legacy adapter:
- `keyboard.service.ts` registers legacy shortcuts into the shared engine.
- `shortcut-registry.service.ts` bindings resolve through the shared engine with DOM-aware `when` guards.

Remaining work (Phase C):
- Plugin binding contribution API end-to-end wiring.
- Live-rebinding UI.
- Expanded mouse gesture coverage as more commands migrate.