# System Surfaces

System resources use the same provider and panel presentation model as files, archives, remote stores, and result collections. They are not implemented as mode switches inside a large system-management component.

## Built-in system providers

The built-in set is:

- `registry://` — Windows Registry
- `autorun://` — startup entries
- `tasks://` — scheduled tasks
- `services://` — Windows services
- `processes://` — running processes
- `devices://` — Plug and Play devices
- `network://` — adapters and TCP connections
- `environment://` — process, user, and machine environment variables
- `shell-extensions://` — registered Explorer context-menu handlers

Registry and Autorun retain their specialized backend semantics, but both already enter the UI through `PanelSource`. The remaining domains use `SystemSurfacePanelSource`, parameterized by a descriptor that owns the scheme, title, and contributed columns.

## Presentation contract

A provider returns ordinary `FileInfo`-shaped rows with provider metadata attached as `systemFields`. The directory pane does not know the system resource type. Sorting, filtering, selection, keyboard input, context menus, configurable columns, and virtual collection rendering therefore remain shared.

Synthetic system rows explicitly opt out of generic file preview. Operations on them belong to contributed commands/context-menu items rather than fake filesystem paths.

## Scale

System providers deliberately do not introduce a second list renderer. Large directories, Registry collections, process tables, and result sets all pass through the same pane projection and virtual collection machinery. Provider-specific paging/streaming can be added behind the `PanelSource` boundary without changing row components or surface components.

## Backend boundary

`SystemSurfaceList(kind)` is one generic Wails binding returning `{ id, name, fields }` records. The wire contract is intentionally open-ended so a new provider can add columns without extending the core file protocol. On Windows the initial collectors use PowerShell/CIM/Windows cmdlets; non-Windows builds expose an empty collection rather than leaking Windows-specific behavior into the frontend.
