# Plugins

Plugins are sibling projects of `manager`. Each plugin owns its source-specific presentation and contributions while consuming only the public plugin SDK/contracts.

A plugin should not import from `manager/frontend/src/app/components`, private services, or backend implementation packages. Host operations are accessed through published plugin capabilities.

## Built-in workspace plugins

`plugins/registry` is a compile-time built-in plugin package. It is source-linked
into Manager through the npm workspace while still using the same
`PluginManagerService.registerBackend()` contribution lifecycle at runtime.

This is intentionally distinct from user-installed directory plugins under the
application configuration directory, whose current frontend modules are
declarative/data-only.
