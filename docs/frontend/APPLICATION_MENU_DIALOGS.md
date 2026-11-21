# Application Menu Dialog Surface

The application menu is the discoverability boundary for user-openable dialogs,
centers and overlays.

It dispatches ordinary application commands through `InputEngineService`.
`ApplicationMenuBarComponent` must not directly inject individual dialog
services.

## User-openable surfaces

The menu exposes:

- Command Palette
- Global Search
- Location History
- Quick Access
- Pinned Locations
- Workspace Switcher
- Operations Center
- Folder Inventory
- File Checksums
- Selection Basket
- Operation History
- Recently Closed Tabs
- Remote Connections
- Connect Remote
- Trash / Recycle Bin
- Batch Rename
- Keyboard Shortcuts
- Input & Shortcuts
- Plugin Settings
- Settings

Some commands appear in more than one semantically useful menu (for example
Operations in View and Tools). This is intentional discoverability, not a
second implementation.

Folder Inventory lists the active folder's immediate entries, including hidden
items when requested. Filter by name, path, or entry type. Sort by name, size or modification
time, and copy paths or export the filtered listing as CSV. Select entries to
send them to the Selection Basket, or open a folder directly in the pane.

Operation History can export the entries matching its active filters as JSON.
In Tools Center, press 5 to open Folder Inventory. Duplicate scans can be
cancelled between file operations and indicate when a scan limit was reached.

## Contextual-only dialogs

These are deliberately NOT directly openable from the application menu:

- Confirm / alert / prompt
- Archive move warning
- destructive-operation confirmations
- file-operation conflict prompts

They require caller-owned request data and, in many cases, return a value to
that caller. Their services/components remain globally hosted but are not
application destinations.

## Ownership rule

Menu -> stable command ID -> active application shell -> dialog/center service
or component.

Never:

Menu -> concrete DialogService
