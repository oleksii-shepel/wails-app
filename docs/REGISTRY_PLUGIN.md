# Registry Plugin

The Windows Registry frontend is owned by the sibling
`@reassembly/registry-plugin` package in `plugins/registry`.

The plugin owns:

- the `registry://` `PanelSource` implementation;
- Registry-specific pane presentation;
- New Key, New Value, Edit, Delete and Refresh toolbar contributions;
- Registry context-menu contributions;
- Registry key and value editors;
- Registry type and payload presentation semantics.

Manager supplies only narrow host adapters backed by its public directory/VFS
capabilities and operation services. Reads and mutations continue through the
existing Registry VFS and common `ApiService` paths, so privilege escalation
uses the same filesystem/elevation mechanics as regular operations.

The backend remains in `manager/vfs/registry` until a backend plugin ABI is
introduced. Registry key/value rename and typed value replacement are backend
capabilities consumed by the frontend plugin; the plugin does not duplicate
those Windows Registry operations.
