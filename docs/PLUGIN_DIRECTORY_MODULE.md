# Declarative directory plugin module v1

Directory packages may include an optional `frontend.json` beside `plugin.json`.
The host reads and validates this file as data. It does **not** import or evaluate JavaScript from the plugin directory.

```json
{
  "schemaVersion": 1,
  "commands": [
    {
      "id": "refresh",
      "title": "Refresh",
      "name": "refresh",
      "targetCommandId": "pane.nav.refresh"
    }
  ],
  "toolbar": [
    { "id": "refresh", "commandId": "refresh", "when": "always" }
  ],
  "contextMenu": [
    { "id": "refresh", "commandId": "refresh", "when": "selection" }
  ]
}
```

`targetCommandId` must reference an existing host/backend command. The directory loader creates a plugin-owned command descriptor that delegates to that target and inherits its enabled state.

Placement `when` values in schema v1 are `always`, `selection`, and `single-selection`.

This loader intentionally cannot execute arbitrary frontend code, start processes, load DLLs, or load WASM. Those require a separate sandboxed executable-plugin design.
