# Provider / Plugin Convergence

Phase 69 makes panel providers part of the same contribution model as viewers and actions.

## Plugin contribution surface

A scoped plugin can now contribute:

- `PanelSource` implementations (`panelSources` permission)
- details-view columns (`columns` permission)
- viewers (`viewers`)
- toolbar and context-menu actions (`toolbar`, `contextMenu`)
- surfaces, related locations, file categories, commands, and result collections

Every scoped registration records a cleanup action. Deactivating the plugin removes its panel sources and columns in the same way it already removes viewers and actions.

## Panel source registry

`PanelSourceRegistryService` is now an ordered runtime registry rather than an immutable constructor list. Archive frames retain highest precedence, contributed sources resolve before ordinary built-ins, and the filesystem provider remains the non-replaceable terminal fallback.

The legacy catch-all remote source remains installed as a compatibility fallback. It is no longer the primary identity for built-in remote schemes once plugin contributions are active.

## Built-in remote providers

SMB, S3, Google Drive, Dropbox, and OneDrive are registered as built-in plugin backends. Each contributes its own scheme-scoped `RemotePanelSource` through `PluginRegistryAPI.registerPanelSource()`.

This means cloud navigation now exercises the same contribution path available to third-party providers. The backend provider/VFS contracts remain responsible for authentication and storage semantics; the frontend plugin contribution owns panel presentation and source identity.

## Columns

Plugins can contribute details columns with a label, optional CSS width, applicability score, and value projection. Columns are resolved against the active panel source context (`currentDirectory`, `sourceId`, `sourceKind`) and rendered only in Details view.

This intentionally avoids adding provider-specific fields to the core `FileInfo` UI contract. A cloud, archive, metadata, or VCS plugin can present additional information without changing the base pane model.
