# Declarative directory plugin example

`plugin.json` contains package identity, permissions and activation policy.
`frontend.json` is a data-only contribution document. The directory loader does not import or evaluate JavaScript from plugin folders.

A declarative command delegates to an already registered host/backend command through `targetCommandId`. Toolbar and context-menu entries then place that plugin-owned command in the UI.