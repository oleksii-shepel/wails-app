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

Recently Closed Tabs restores a tab to its previous pane and position. Filter
by pane, title, or path. In the filter field, Enter restores the latest match
and the arrow keys move focus to the results. Clearing the recovery list only
affects the current workspace.

Location History can copy an individual location or the entire history as JSON.
Autocomplete resolves explicit relative paths against the current local pane.

Quick Access keeps pinned locations in their own section even when Smart also
suggests them. Use Copy Location to copy the selected target, or select a pin
to rename or unpin it without leaving the dialog.

Remote Connections filters mounted locations by name, protocol, path, or access
level. Use Open or Enter to navigate to the selected mount, or copy its public
location without leaving the dialog.

Folder Inventory lists the active folder's immediate entries, including hidden
items when requested. Filter by name, path, or entry type and sort by name, type,
size, or modification time. Copy paths or export either the matching rows or the
selected rows as CSV. The selection summary includes entries outside the active
filter. Select entries to send them to the Selection Basket, open a folder in
the pane, or open a file preview directly from its name. Switching tools keeps
the inventory filters and selection until Tools Center closes.

Operation History can filter for failed, reversible, or undone and redone
entries, search by outcome or path, and export or copy the matches as JSON. Expand an
entry to read or copy its affected paths. Clearing the journal keeps undo and
redo actions available.

Search current and recent jobs from the Operations page in Tools Center.
Operation progress uses byte totals when available and file counts otherwise.
When neither total is known, the progress bar indicates that work is ongoing
without displaying a percentage.

In Tools Center, press 5 to open Folder Inventory or 6 to open Split & Join. Duplicate scans can be
cancelled between file operations from Tools Center or the standalone Duplicate
Finder. Expand the warning summary in Tools Center to see skipped paths.
The Checksums page can copy the full set of digests and accepts a raw checksum or
a checksum line with a filename. In batch mode, select files to generate SHA-256 digests and compare them with a manifest.

Trash can filter by location and search by name or path. Sort by deletion time
to find recent or older items. After a partially successful operation, the
list refreshes and keeps failed items selected for retry.

Batch Rename checks destination names before applying changes and flags
conflicting names in the same destination folder. The ZIP creation dialog
checks the filename and destination, and ZIP creation also refuses to replace
an existing destination if another process creates it after the check.

The Text Viewer can copy the selection or the full document. The Binary
Viewer accepts decimal offsets and hexadecimal offsets with a `0x` prefix,
and reports invalid or out-of-range offsets.

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
