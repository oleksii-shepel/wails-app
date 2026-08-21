# Environment Pane Ownership

`environment://` is a Manager-owned pane.

## Presentation

`EnvironmentPaneComponent` starts as a clone of `DirectoryPaneComponent`, then
adapts only Environment semantics. It uses the same Manager pane shell,
selection, tabs, address bar, status row, filtering, virtualization, responsive
toolbar, and Details row styling.

The Environment pane owns these Details columns:

- Name
- Scope
- Value
- Type

The Environment pane is Details-only.

## Navigation

The root is intentionally file-like:

- `environment://process` — variables inherited by the running Manager process
- `environment://user` — persistent current-user variables
- `environment://system` — persistent machine-wide variables

Process variables are read-only snapshots. User and System are separated now so
create/edit/delete operations can be added without ambiguous scope semantics.

## Backend

Environment enumeration is native and does not launch PowerShell.

Windows:
- Process: `os.Environ()`
- User: `HKCU\Environment`
- System: `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`

`REG_SZ` and `REG_EXPAND_SZ` are preserved as `String` and `Expandable`.

Non-Windows builds expose process environment variables only.

## First-pass commands

- Refresh
- Copy name
- Copy value

Mutation/editor commands intentionally come next, after the ownership,
navigation, and native listing boundaries are stable.
