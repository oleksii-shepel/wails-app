# Registry Plugin

The Windows Registry is now owned by the built-in `builtin.registry` plugin rather than the core `PanelSourceRegistryService`.

The plugin contributes:

- the `registry://` `PanelSource`;
- Registry-specific Details columns for kind, value type, and preview data;
- New Key and New Value actions;
- Edit Value and Delete Entry actions.

All reads and mutations continue through the existing Registry VFS and common `ApiService` operation paths. The plugin does not introduce a parallel Registry backend. Existing elevation behavior remains in the filesystem service.

Registry rename remains unsupported because the underlying Registry VFS intentionally does not implement rename.
