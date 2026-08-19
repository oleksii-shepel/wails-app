# Plugins

Plugins are sibling projects of `manager`. Each plugin owns its source-specific presentation and contributions while consuming only the public plugin SDK/contracts.

A plugin should not import from `manager/frontend/src/app/components`, private services, or backend implementation packages. Host operations are accessed through published plugin capabilities.
