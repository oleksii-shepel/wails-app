# Registry Plugin

Target project for `builtin.registry`.

The first extraction should move Registry-specific frontend ownership here:

- Registry panel/surface presentation;
- Registry toolbar and context-menu contributions;
- Registry commands;
- key/value editors;
- Registry formatting and type semantics.

Keep `manager/vfs/registry` and the common elevation/filesystem operation path in the host until a backend plugin ABI exists.

Do not move files mechanically until the host exposes a custom panel-surface contribution associated with a `PanelSource`; that contract is the boundary that lets `DirectoryPaneComponent` stop knowing about Registry UI.
