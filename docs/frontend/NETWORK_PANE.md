# Network Pane Ownership

`network://` is a Manager-owned feature.

- Backend/listing: Manager `SystemSurfaceList('network')`
- Panel source: Manager `SystemSurfacePanelSource`
- Presentation: Manager `NetworkPaneComponent`
- Renderer identity: `network`
- No Network plugin package is required or distributed separately.

The generic `SystemTablePaneComponent` continues to serve tasks, services,
processes, devices, environment, and shell extensions.

This split is intentional: Network can now evolve its own columns, commands,
toolbar and context-menu behavior without coupling those choices to every
system-table surface.
