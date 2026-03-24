# Keyboard Shortcuts & Mouse Handlers

## Architecture Notes

Keyboard and mouse interactions now route through the refactored pane/surface boundaries rather than directly through one large component.

The next-stage input-system design, including dialog-by-dialog binding inventory and requirements for chords, plugin bindings, mouse gestures, dynamic rebinding, and context-priority resolution, is documented in [INPUT_SYSTEM_ARCHITECTURE.md](INPUT_SYSTEM_ARCHITECTURE.md).

Current migration status:

- dialog contexts now run through the new input engine for confirm, global search, keyboard help, path history, and archive move warning surfaces
- overlay contexts now run through the new input engine for address-bar quick access, drive and sibling dropdowns, the workspace selector, and filter dropdown overlays
- legacy global shortcuts are now bridged through the new input engine by `keyboard.service.ts` and still sourced from `shortcut-registry.service.ts`
- global chord shortcuts are now live through the same bridge, with pending-chord feedback surfaced in the browser chrome
- mouse gesture dispatch is now live in `input-engine.service.ts` for click, double-click, button, and wheel bindings
- the first browser-shell mouse bindings now route through that engine as well: file-entry double-click opens the active selection, and viewer `Ctrl+Wheel` zooms the active preview

- Global keyboard dispatch starts in the frontend keyboard services and browser shell components.
- Pane-facing browser actions are grouped through pane command factories instead of many standalone pane methods.
- `file-pane.component.ts` acts mainly as a host/facade; deeper interaction logic belongs in pane controllers, view controllers, or surface VMs.
- When adding a shortcut or pointer interaction, preserve behavior parity across list, grid, details, and tree views.

## Keyboard Shortcuts

### Navigation

| Shortcut | Context | Description |
|----------|---------|-------------|
| **Ctrl+K** | Global | Open global search |
| **G then S** | Global | Open global search via chord |
| **Ctrl+H** | Global | Show path history |
| **G then H** | Global | Show path history via chord |
| **Ctrl+P** | Global | Quick path jump |
| **Backspace** | Global | Go to parent directory |
| **Ctrl+R** | Global | Refresh current pane |
| **Ctrl+/** | Global | Show keyboard shortcuts help |
| **Arrow Left** | Window | Navigate back in history (when available) |
| **Arrow Right** | Window | Navigate forward in history (when available) |

### Tab Management

| Shortcut | Context | Description |
|----------|---------|-------------|
| **Ctrl+T** | Global | Open new tab |
| **Ctrl+W** | Global | Close current tab |
| **Ctrl+Tab** | Global | Switch to next tab |
| **Ctrl+Shift+Tab** | Global | Switch to previous tab |
| **Ctrl+1** through **Ctrl+9** | Global | Jump to tab 1-9 |
| **Ctrl+Shift+P** | Global | Pin/unpin current tab |

### File Operations

| Shortcut | Context | Description |
|----------|---------|-------------|
| **F3** | Global | Quick view selected item |
| **Enter** | Global | Open file or folder |
| **Delete** | Global | Delete selected items |
| **F8** | Global | Delete selected items (Total Commander style) |
| **Ctrl+C** | Global | Copy selected items |
| **Ctrl+X** | Global | Cut selected items |
| **Ctrl+V** | Global | Paste items |
| **Ctrl+A** | Global | Select all items |
| **Ctrl+Num +** | Global | Select all items (numpad) |
| **Shift+Num +** | Global | Select all items (numpad) |
| **Ctrl+D** | Global | Deselect all items |
| **Ctrl+Num -** | Global | Deselect all items (numpad) |
| **Shift+Num -** | Global | Deselect all items (numpad) |
| **Ctrl+Shift+N** | Global | Create new folder |
| **F7** | Global | Create new folder (Total Commander style) |
| **F2** | Global | Rename selected item |
| **Shift+F6** | Global | Rename selected item (Total Commander style) |
| **F5** | Global | Copy to other pane (Total Commander style) |
| **F6** | Global | Move to other pane (Total Commander style) |

### Advanced Selection

| Shortcut | Context | Description |
|----------|---------|-------------|
| **Space** | Global | Select current item and move down |
| **Insert** | Global | Toggle current item selection and move down |
| **Shift+Arrow Up** | Global | Extend selection upward |
| **Shift+Arrow Down** | Global | Extend selection downward |
| **Num +** | Global | Select by mask pattern |
| **Num -** | Global | Unselect by mask pattern |
| **Num \*** | Global | Invert selection |
| **Alt+Num +** | Global | Select files with same extension |

### Search & Filter

| Shortcut | Context | Description |
|----------|---------|-------------|
| **Ctrl+F** | Global | Focus filter input |
| **Ctrl+Shift+F** | Global | Advanced filter options |
| **Escape** | Filter | Clear filter and exit filter mode |
| **F3** | Filter Widget | Find next occurrence |
| **Shift+F3** | Filter Widget | Find previous occurrence |
| **Enter** | Filter Widget | Find next occurrence |
| **Shift+Enter** | Filter Widget | Find previous occurrence |

### Workspace

| Shortcut | Context | Description |
|----------|---------|-------------|
| **Ctrl+Shift+W** | Global | Open workspace switcher |
| **Ctrl+Shift+Alt+N** | Global | Create new workspace |
| **Ctrl+Shift+H** | Global | Show/hide hidden files |

### Pane Control

| Shortcut | Context | Description |
|----------|---------|-------------|
| **Tab** | Global | Switch between panes |
| **Ctrl+Shift+C** | Pane | Copy selected to other pane |
| **Ctrl+Shift+M** | Pane | Move selected to other pane |
| **Ctrl+L** | Global | Focus address bar |
| **Alt+D** | Global | Focus address bar (Windows Explorer style) |

### Address Bar

| Shortcut | Context | Description |
|----------|---------|-------------|
| **Ctrl+L** | Address Bar | Enter edit mode |
| **Alt+D** | Address Bar | Enter edit mode |
| **Ctrl+Q** | Address Bar | Toggle Quick Access dropdown |
| **Ctrl+B** | Address Bar | Toggle Pinned Paths dropdown |
| **Ctrl+F** | Quick Access | Focus Quick Access filter |
| **Alt+1** through **Alt+9** | Quick Access | Open item 1-9 in the active Quick Access section |
| **Enter** | Edit Mode | Apply path and exit edit mode |
| **Escape** | Edit Mode | Cancel edit and exit edit mode |
| **Tab** | Edit Mode | Tab-completion: cycle forward through suggestions |
| **Shift+Tab** | Edit Mode | Tab-completion: cycle backward through suggestions |
| **Arrow Up** | Edit Mode | Navigate up in suggestions list |
| **Arrow Down** | Edit Mode | Navigate down in suggestions list |

#### Search Scheme Input

The address bar accepts direct `search://` expressions in edit mode.

Supported forms:

- `search://invoice @ C:/Work`
- `search://invoice @@ C:/Work`
- `search://invoice`
- `search://?path=drive://c/Work&pattern=invoice&recursive=true`

Rules:

- `@` means recursive search.
- `@@` means non-recursive search.
- If no path is supplied, the current pane location is used.
- Enter normalizes these expressions to the canonical search alias consumed by the backend.

### Tree View Navigation

| Shortcut | Context | Description |
|----------|---------|-------------|
| **Arrow Up/Down** | Tree View | Navigate up/down nodes |
| **Arrow Left** | Tree View | Collapse current node or go to parent |
| **Arrow Right** | Tree View | Expand current node or go to first child |
| **Home** | Tree View | Go to first node |
| **End** | Tree View | Go to last visible node |
| **Enter** | Tree View | Navigate into selected directory |
| **Space** | Tree View | Toggle selection |
| **F2** | Tree View | Rename node |
| **Ctrl+A** | Tree View | Select all nodes |

### Modal & Dialog Shortcuts

| Shortcut | Context | Description |
|----------|---------|-------------|
| **Escape** | Modal/Dialog | Close active modal, dialog, or dropdown |
| **Arrow Up/Down** | Dropdown/List | Navigate list items |
| **Enter** | Dropdown/List | Select current item |
| **?** | Global | Show keyboard help (same as Ctrl+/) |

---

## Mouse Event Handlers

### Click Events

#### File Entry Component
**Location:** [file-entry.component.ts](frontend/src/app/components/file-entry.component.ts)
- **Click:** `onClick(event)` - Emits selection event with entry and mouse event
- **Purpose:** Single-click to select file/directory entry

#### Tab Component
**Location:** [tab.component.ts](frontend/src/app/components/tab.component.ts)
- **Click:** `onClick()` - Emits tab click event
- **Purpose:** Switch to clicked tab
- **Click (Close Button):** `onClose(event)` - Stops propagation and closes tab
- **Click (Pin Button):** `onPin(event)` - Stops propagation and toggles pin status

#### Action Bar Component
**Location:** [action-bar.component.ts](frontend/src/app/components/action-bar.component.ts)
- **Click (View Options):** `onViewChange(mode)` - Switch between list/grid/details view modes
- **Click (Sort Button):** `toggleSortMenu(event)` - Open/close sort dropdown
- **Click (Sort Item):** `onSortChange(value)` - Change sort field
- **Click (Sort Direction):** `toggleSortDirection()` - Toggle ascending/descending
- **Click (New Folder):** `onCreate()` - Create new folder
- **Click (Delete):** `onDelete()` - Delete selected items
- **Click (Copy):** `onCopy()` - Copy selected items
- **Click (Move):** `onMove()` - Move selected items
- **Click (Refresh):** `onRefresh()` - Refresh current view

#### Address Bar Component
**Location:** [address-bar.component.ts](frontend/src/app/components/address-bar.component.ts)
- **Click (Star/Pin):** `togglePin()` - Pin/unpin current path
- **Click (Quick Access):** `toggleQuickAccess()` - Open Quick Access dropdown
- **Click (Pin List):** `togglePinDrop()` - Open pinned paths dropdown
- **Click (Drive Letter):** `onDriveLetterClick()` - Open drive selector
- **Click (Breadcrumb Segment):** `onSegClick()` - Navigate to segment path
- **Click (Chevron):** `toggleChev()` - Open sibling directory dropdown
- **Click (Go Up):** `goUp()` - Navigate to parent directory
- **Click (Refresh):** `refresh()` - Refresh current path
- **Click (Copy Path):** `copyPath()` - Copy current path to clipboard

#### File Tree Component
**Location:** [file-tree.component.ts](frontend/src/app/components/file-tree.component.ts)
- **Click (Node):** `onNodeClick(node, event)` - Select node with modifier key support
- **Click (Twistie):** `onTwistieClick(event, node)` - Expand/collapse node
- **Click (Container):** `onContainerClick(event)` - Background click for deselection

#### File List Component
**Location:** [file-list.component.ts](frontend/src/app/components/file-list.component.ts)
- **Click (Background):** `onBackgroundClick(event)` - Clear selection when clicking empty area

#### File Grid Component
**Location:** [file-grid.component.ts](frontend/src/app/components/file-grid.component.ts)
- **Click (Item):** `onItemClick(entry, event)` - Select item with Ctrl/Shift support
- **Click (Background):** `onBackgroundClick(event)` - Clear selection

#### File Details Component
**Location:** [file-details.component.ts](frontend/src/app/components/file-details.component.ts)
- **Click (Column Header):** `onSort(field)` - Sort by column
- **Click (Row):** `onRowClick(entry, event)` - Select row with modifier support
- **Click (Background):** `onBackgroundClick(event)` - Clear selection

#### Filter Bar Component
**Location:** [filter-bar.component.ts](frontend/src/app/components/filter-bar.component.ts)
- **Click (Category):** `toggleCategoryDropdown()` - Open category filter dropdown
- **Click (Clear Search):** `clearSearch()` - Clear search text
- **Click (Preset Dropdown):** Toggle preset dropdown menu
- **Click (Save Preset):** `onSavePreset()` - Save current filters as preset
- **Click (Apply Preset):** `applyPreset(preset)` - Load saved filter preset
- **Click (Delete Preset):** `deletePreset(preset)` - Remove filter preset
- **Click (Clear All):** `clearAllFilters()` - Reset all active filters
- **Click (Search Chip):** `clearSearch()` - Remove search filter

#### Dropdown Components
**Location:** Various dropdown components
- **Click (Backdrop):** Close dropdown overlay
- **Click (Menu Item):** Select item and close dropdown

#### Elevation Status Pill
**Location:** [elevation-status-pill.component.ts](frontend/src/app/components/elevation-status-pill.component.ts)
- **Click:** `onClick()` - Toggle elevation status or show elevation dialog

### Double-Click Events

#### File Entry Component
**Location:** [file-entry.component.ts](frontend/src/app/components/file-entry.component.ts)
- **Double-Click:** `onDoubleClick()` - Emits navigate event to open/enter the entry
- **Purpose:** Open files or enter directories

#### File Tree Component
**Location:** [file-tree.component.ts](frontend/src/app/components/file-tree.component.ts)
- **Double-Click (Node):** `onNodeDoubleClick(node, event)` - Navigate into directory
- **Purpose:** Quick navigation into folders

#### File List Component
**Location:** [file-list.component.ts](frontend/src/app/components/file-list.component.ts)
- **Double-Click (Item):** Navigation handled via file-entry component
- **Purpose:** Open files or enter directories

#### File Grid Component
**Location:** [file-grid.component.ts](frontend/src/app/components/file-grid.component.ts)
- **Double-Click (Item):** `onItemDoubleClick(entry)` - Navigate into entry
- **Purpose:** Open files or enter directories

#### File Details Component
**Location:** [file-details.component.ts](frontend/src/app/components/file-details.component.ts)
- **Double-Click (Row):** `onRowDoubleClick(entry)` - Navigate into entry
- **Purpose:** Open files or enter directories

#### Split Divider Component
**Location:** [split-divider.component.ts](frontend/src/app/components/split-divider.component.ts)
- **Double-Click (@HostListener):** Reset pane split to 50/50
- **Purpose:** Quick reset of pane sizes

### Context Menu (Right-Click)

#### File Entry Component
- **Context Menu:** Handled by parent component (file-list, file-grid, etc.)
- **Purpose:** Show context menu for file operations

#### File Tree Component
**Location:** [file-tree.component.ts](frontend/src/app/components/file-tree.component.ts)
- **Context Menu (Node):** `onContextMenu(event, node)` - Show context menu for node
- **Context Menu (Container):** `onContainerContextMenu(event)` - Show context menu for empty area
- **Purpose:** File/folder operations menu

#### File List Component
**Location:** [file-list.component.ts](frontend/src/app/components/file-list.component.ts)
- **Context Menu (Item):** `onContextMenu(event, entry)` - Show context menu for item
- **Context Menu (Background):** `onBackgroundContextMenu(event)` - Show background context menu
- **Purpose:** File operations menu

#### File Grid Component
**Location:** [file-grid.component.ts](frontend/src/app/components/file-grid.component.ts)
- **Context Menu (Item):** `onContextMenu(event, entry)` - Show context menu for item
- **Context Menu (Background):** `onBackgroundContextMenu(event)` - Show background context menu
- **Purpose:** File operations menu

#### File Details Component
**Location:** [file-details.component.ts](frontend/src/app/components/file-details.component.ts)
- **Context Menu (Row):** `onContextMenu(event, entry)` - Show context menu
- **Context Menu (Background):** `onBackgroundContextMenu(event)` - Show background context menu
- **Purpose:** File operations menu

#### File Pane Component
**Location:** [file-pane.component.ts](frontend/src/app/components/file-pane.component.ts)
- **Context Menu:** `onContextMenu(event)` - Forward context menu event to appropriate view
- **Purpose:** Route context menu to active view component

#### File View Container
**Location:** [file-view-container.component.ts](frontend/src/app/components/file-view-container.component.ts)
- **Context Menu:** `onContextMenu(event)` - Handle context menu for all view types
- **Purpose:** Unified context menu handling

#### Address Bar Component
**Location:** [address-bar.component.ts](frontend/src/app/components/address-bar.component.ts)
- **Context Menu (Segment):** `onSegRightClick(segment, index, event)` - Show segment context menu
- **Purpose:** Navigate to, copy path, pin path, open in other pane

### Auxiliary Click (Middle-Click)

#### Tab Component
**Location:** [tab.component.ts](frontend/src/app/components/tab.component.ts)
- **Aux Click:** `onMouseUp(event)` - Close tab if middle button (button === 1)
- **Purpose:** Quick tab closing

#### File List Component
**Location:** [file-list.component.ts](frontend/src/app/components/file-list.component.ts)
- **Aux Click (Item):** `onItemAuxClick(event, entry)` - Handle middle-click on item
- **Purpose:** Additional item interaction (e.g., open in new tab)

#### File Grid Component
**Location:** [file-grid.component.ts](frontend/src/app/components/file-grid.component.ts)
- **Aux Click (Item):** `onItemAuxClick(event, entry)` - Handle middle-click on item
- **Purpose:** Additional item interaction

#### File Details Component
**Location:** [file-details.component.ts](frontend/src/app/components/file-details.component.ts)
- **Aux Click (Row):** `onRowAuxClick(event, entry)` - Handle middle-click on row
- **Purpose:** Additional item interaction

#### File Tree Component
**Location:** [file-tree.component.ts](frontend/src/app/components/file-tree.component.ts)
- **Aux Click (Node):** `onNodeAuxClick(node, event)` - Handle middle-click on node
- **Purpose:** Additional node interaction

### Mouse Down/Up Events

#### File Tree Component
**Location:** [file-tree.component.ts](frontend/src/app/components/file-tree.component.ts)
- **Mouse Down (Node):** `onNodeMouseDown(node, event)` - Start selection drag
- **Mouse Down (Twistie):** `onTwistieMouseDown(event, node)` - Prevent text selection
- **Purpose:** Handle drag selection and prevent unwanted text selection

#### File List Component
**Location:** [file-list.component.ts](frontend/src/app/components/file-list.component.ts)
- **Mouse Down (List):** `onListMouseDown(event)` - Start rubber-band selection
- **Window Mouse Move:** Track mouse for rubber-band
- **Window Mouse Up:** End rubber-band selection
- **Purpose:** Multi-item rubber-band selection

#### File Grid Component
**Location:** [file-grid.component.ts](frontend/src/app/components/file-grid.component.ts)
- **Mouse Down (Grid):** `onGridMouseDown(event)` - Start rubber-band selection
- **Window Mouse Move:** Track mouse for rubber-band
- **Window Mouse Up:** End rubber-band selection
- **Purpose:** Multi-item rubber-band selection

#### File Details Component
**Location:** [file-details.component.ts](frontend/src/app/components/file-details.component.ts)
- **Mouse Down (Body):** `onBodyMouseDown(event)` - Start rubber-band selection
- **Mouse Down (Column Resize):** `onColumnResizeStart(event, column)` - Start column resize
- **Window Mouse Move:** Track mouse for rubber-band or column resize
- **Window Mouse Up:** End rubber-band or resize
- **Purpose:** Multi-item selection and column resizing

#### Split Divider Component
**Location:** [split-divider.component.ts](frontend/src/app/components/split-divider.component.ts)
- **Pointer Down (@HostListener):** `startDrag(event)` - Start pane resize
- **Document Pointer Move (@HostListener):** Track pointer for resize
- **Document Pointer Up (@HostListener):** `stopDrag()` - End pane resize
- **Purpose:** Resizing split panes

#### File Browser Component
**Location:** [file-browser.component.ts](frontend/src/app/components/file-browser.component.ts)
- **Document Mouse Move:** `onResize()` - Handle split pane resizing
- **Document Mouse Up:** `stopResize()` - End split pane resize
- **Purpose:** Dynamic pane resizing

#### Address Bar Edit Component
**Location:** [address-bar-edit.component.ts](frontend/src/app/components/address-bar-edit.component.ts)
- **Mouse Down (Suggestion):** `suggestionSelect.emit()` - Select suggestion
- **Purpose:** Click to select path suggestion

### Drag & Drop Events

#### File Tree Component
**Location:** [file-tree.component.ts](frontend/src/app/components/file-tree.component.ts)
- **Drag Start (Node):** `onDragStart(event, node)` - Start dragging selected nodes
- **Drag End:** `onDragEnd(event)` - Clean up drag state
- **Drag Over (Node):** `onDragOver(event, node)` - Determine drop position (before/after/inside)
- **Drag Leave:** `onDragLeave(event, node)` - Clear drag-over state
- **Drop (Node):** `onDrop(event, node)` - Handle drop with position
- **Drag Over (Container):** `onContainerDragOver(event)` - Handle drag over empty area
- **Drop (Container):** `onContainerDrop(event)` - Handle drop on empty area
- **Purpose:** Drag and drop file/folder reorganization

#### File List Component
**Location:** [file-list.component.ts](frontend/src/app/components/file-list.component.ts)
- **Drag Start (Entry):** `onDragStart(event, entry)` - Start dragging entries
- **Drag Over (Entry):** `onDragOver(event, entry)` - Show drop indicator
- **Drag Leave:** `onDragLeave(entry)` - Clear drop indicator
- **Drop (Entry):** `onDrop(event, entry)` - Handle file drop
- **Purpose:** Move/copy files via drag and drop

#### File Grid Component
**Location:** [file-grid.component.ts](frontend/src/app/components/file-grid.component.ts)
- **Drag Start (Entry):** `onDragStart(event, entry)` - Start dragging entries
- **Drag Over (Entry):** `onDragOver(event, entry)` - Show drop indicator
- **Drag Leave:** `onDragLeave(entry)` - Clear drop indicator
- **Drop (Entry):** `onDrop(event, entry)` - Handle file drop
- **Purpose:** Move/copy files via drag and drop

#### File Details Component
**Location:** [file-details.component.ts](frontend/src/app/components/file-details.component.ts)
- **Drag Start (Row):** `onDragStart(event, entry)` - Start dragging rows
- **Drag Over (Row):** `onDragOver(event, entry)` - Show drop indicator
- **Drag Leave:** `onDragLeave(entry)` - Clear drop indicator
- **Drop (Row):** `onDrop(event, entry)` - Handle file drop
- **Purpose:** Move/copy files via drag and drop

#### File View Container Component
**Location:** [file-view-container.component.ts](frontend/src/app/components/file-view-container.component.ts)
- **Drag Start (View):** `onViewDragStart(event)` - Forward drag start from list/grid/details
- **Drag Start (Tree):** `onTreeDragStart(event)` - Forward drag start from tree
- **Drag Drop (View):** `onViewDragDrop(event)` - Handle drop into view
- **Drag Drop (Tree):** `onTreeDragDrop(event)` - Handle drop into tree
- **Purpose:** Coordinate drag and drop across view types

#### File Pane Component
**Location:** [file-pane.component.ts](frontend/src/app/components/file-pane.component.ts)
- **Drag Drop:** `onDragDrop(event)` - Handle file drop and initiate move/copy operation
- **Purpose:** Execute file operations from drag and drop

### Scroll & Wheel Events

#### Address Bar Component
**Location:** [address-bar.component.ts](frontend/src/app/components/address-bar.component.ts)
- **Wheel Event:** Custom wheel listener on breadcrumb container
- **Purpose:** Horizontal scrolling with mouse wheel

### Special Event Listeners

#### Dropdown Service
**Location:** [dropdown.service.ts](frontend/src/app/services/dropdown.service.ts)
- **Document Mouse Down (Capture):** `onGlobalMousedown()` - Track clicks for dropdown closing
- **Document Click (Capture):** `onGlobalClick()` - Close dropdowns on outside click
- **Purpose:** Global dropdown management

#### Address Bar Component
**Location:** [address-bar.component.ts](frontend/src/app/components/address-bar.component.ts)
- **Document Mouse Down (Capture):** `onEditModeOutsideClick()` - Exit edit mode on outside click
- **Purpose:** Close edit mode when clicking outside

#### File Browser Component
**Location:** [file-browser.component.ts](frontend/src/app/components/file-browser.component.ts)
- **Window Before Unload (@HostListener):** Check for unsaved changes
- **Purpose:** Prevent data loss on page close

---

## Component-Specific Event Summary

### File Entry Component
- **Events:** Click, Double-Click
- **Purpose:** Basic file/folder interaction (select/open)

### Tab Component
- **Events:** Click, Aux Click, Close Button Click, Pin Button Click, Mouse Enter/Leave
- **Purpose:** Tab management and quick close

### File Tree Component
- **Events:** Click, Double-Click, Aux Click, Context Menu, Mouse Down, Drag Start/End/Over/Leave/Drop, Keyboard (Arrow, Enter, Space, F2, Home, End, Ctrl+A)
- **Purpose:** Hierarchical file browsing with full interaction

### File List/Grid/Details Components
- **Events:** Click, Double-Click, Aux Click, Context Menu, Mouse Down/Move/Up (rubber-band), Drag Start/Over/Leave/Drop
- **Purpose:** File selection, navigation, and drag-and-drop operations

### Address Bar Component
- **Events:** Click (various buttons/segments), Context Menu, Mouse Down (edit suggestions), Wheel, Keyboard (Ctrl+L, Alt+D, Ctrl+Q, Ctrl+B, Tab, Enter, Escape, Arrows)
- **Purpose:** Path navigation and editing

### Split Divider Component
- **Events:** Pointer Down/Move/Up, Double-Click
- **Purpose:** Resizing panes and quick reset

### Action Bar Component
- **Events:** Click (view/sort/action buttons)
- **Purpose:** View mode, sorting, and file operations

### Filter Bar Component
- **Events:** Click (category, presets, clear), Keyboard (Escape in search)
- **Purpose:** Filter and search management

### Dropdown Components
- **Events:** Click (items, backdrop), Keyboard (Arrow Up/Down, Enter)
- **Purpose:** Menu selection and navigation

---

## Implementation Notes

### Global Keyboard Service
- **File:** [keyboard.service.ts](frontend/src/app/services/keyboard.service.ts)
- Acts as a compatibility adapter over the shared input engine rather than owning a separate global keyboard listener
- Registers legacy shortcut definitions as engine bindings on the `global` context
- Preserves legacy context rules (`global`, `pane`, `filter`, `modal`) through DOM-aware `when` guards
- Preserves `allowBubble` behavior by controlling per-binding propagation in the engine

### Shortcut Registry
- **File:** [shortcut-registry.service.ts](frontend/src/app/services/shortcut-registry.service.ts)
- Registers all default keyboard shortcuts during app initialization
- Organized by category: navigation, tabs, files, search, workspace, pane
- Connects keyboard events to callback functions in main component

### Tab Completion System
- **Location:** Address Bar Edit Mode
- Idle-slash pipeline: After 1 second of typing idle, auto-appends `/` and loads directory children
- **Tab:** Cycle forward through suggestions, restart idle timer
- **Shift+Tab:** Cycle backward through suggestions or go up to parent
- Typing any key clears tab session and returns to normal mode
- Search expressions are parsed on commit rather than treated as filesystem paths

### Mouse Selection Modes
1. **Single Click:** Select item (Ctrl for multi-select, Shift for range)
2. **Double Click:** Open/navigate into item
3. **Middle Click:** Alternative action (e.g., close tab)
4. **Right Click:** Show context menu
5. **Drag:** Start rubber-band selection or initiate drag-and-drop
6. **Click + Drag:** Rubber-band multi-select in file views

### Drag and Drop Flow
1. **Drag Start:** Collect selected entries, set drag data, emit dragStart event
2. **Drag Over:** Determine drop target and position, show visual feedback
3. **Drag Leave:** Clear drop indicators
4. **Drop:** Process drop with source entries and target, execute move/copy operation
5. **Drag End:** Clean up drag state

### Context Menu System
- Right-click on files/folders shows operations menu (copy, cut, delete, rename, etc.)
- Right-click on empty space shows background menu (paste, new folder, etc.)
- Right-click on address bar breadcrumbs shows segment menu (navigate to, copy path, pin, open in other pane)
- Context menus support Escape key to close

### Event Propagation Control
- Most interactive elements use `event.stopPropagation()` to prevent bubbling
- Drop-down/modal backdrop handlers stop propagation to keep menu open when clicking inside
- Tab completion needs `allowBubble: true` to reach component handlers while still preventing default browser behavior

---

## Keyboard Shortcut Categories

### Total Commander Compatibility
The application implements Total Commander function key shortcuts:
- **F3:** Quick view
- **F5:** Copy to other pane
- **F6:** Move to other pane
- **F7:** New folder
- **F8:** Delete
- **Shift+F6:** Rename

### Windows Explorer Compatibility
- **Alt+D:** Focus address bar
- **Backspace:** Go to parent directory
- **F2:** Rename
- **Delete:** Delete
- **Ctrl+A:** Select all

### Modern Application Shortcuts
- **Ctrl+T/W:** Tab management
- **Ctrl+Tab:** Tab switching
- **Ctrl+F:** Filter/search
- **Ctrl+K:** Global search (like VS Code)
- **Ctrl+P:** Quick nav (like VS Code)

---

## Quick Reference Card

### Most Common Shortcuts
```
Navigation:
  Ctrl+K         → Global search
  Ctrl+P         → Quick path jump
  Backspace      → Parent directory
  Ctrl+L/Alt+D   → Focus address bar

Files:
  Enter          → Open/Enter
  F3             → Quick view
  Delete/F8      → Delete
  Ctrl+C/X/V     → Copy/Cut/Paste
  F2             → Rename
  Ctrl+A         → Select all

Tabs:
  Ctrl+T         → New tab
  Ctrl+W         → Close tab
  Ctrl+Tab       → Next tab
  Ctrl+1-9       → Jump to tab

Panes:
  Tab            → Switch pane
  F5             → Copy to other pane
  F6             → Move to other pane

Other:
  Ctrl+/         → Show keyboard help
  Escape         → Close modal/dialog
```

---

*Generated from source code analysis of the Wails file manager application.*
