# Registry Plugin

`@reassembly/registry-plugin` owns the frontend Registry experience for
`builtin.registry`.

It contains:

- the `registry://` panel-source implementation;
- Registry pane presentation;
- Registry key and value editors;
- Registry commands and context-menu contributions;
- Registry type/payload presentation semantics.

The package does not import Manager internals. Manager supplies a narrow
`RegistryPanelSourceHost` and `RegistryPluginOperations` adapter backed by the
existing VFS, `ApiService`, dialogs, and elevation-aware filesystem operations.

The Registry backend remains in `manager/vfs/registry` until a backend plugin
ABI exists.

## Build

From the repository root:

```sh
npm install
npm run build:plugins
```

The package is source-linked into `manager/frontend` through the workspace. Its
standalone `build` command performs strict TypeScript validation; the Angular
application performs the final AOT compilation when Manager is built.
