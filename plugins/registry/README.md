# Reassembly Registry Plugin

Windows Registry domain package for Reassembly.

## Ownership

The package owns Registry semantics, not the pane chrome:

- `RegistryPanelSource` and Registry entry contracts
- Registry create/edit/delete command behavior
- Registry toolbar contributions
- Registry entry/background context-menu contributions
- key and value editor services/components
- the universal Registry backend adapter

The Manager owns the visual pane shell. `manager/frontend/src/app/components/registry-pane.component.ts`
is intentionally a Directory-pane sibling so Directory and Registry keep the same visual and interaction language.
The shared Manager context-menu and toolbar renderers display plugin contributions; the plugin does not ship a second pane UI.

## Context menu contract

Registry uses the common Manager context-menu renderer with Registry-owned actions:

- background: **New Key**, **New Value**, **Refresh**
- entry: **Edit**, **Delete**

The host must execute the `PresentedCommand` created for the right-click context so `targetPath`,
`currentDirectory`, and `selectedPaths` are preserved exactly.

## Backend protocol

The backend understands:

- `hello`
- `registry.list`
- `registry.read`
- `registry.write`
- `registry.createKey`
- `registry.delete`
- `registry.rename`

## Build

```bash
npm run build:frontend
npm run test
npm run build:backend
```

`registry-pane.component.ts` is intentionally **not** part of this package's public API.
