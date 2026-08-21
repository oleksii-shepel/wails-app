# Reassembly Core

`@reassembly/core` and the Go module `github.com/epikodelabs/reassembly/core` are the two language
faces of one architectural package.

```text
packages/core/
  frontend/
    src/
      contracts/
        files/
        input/
        panes/
        sources/
      runtime/
        panes/
  go/
    contracts/
```

The frontend uses domain folders instead of a flat source list. New frontend
code should import from `@github.com/epikodelabs/reassembly/core/contracts/...` or
`@reassembly/core/runtime/...`.

The Go module owns stable wire contracts. Manager keeps local aliases during
migration, so existing `manager/internal/contracts` imports remain valid while
the definitions live in core.

Core must not import Manager, Angular components, Wails bindings, or concrete
plugins.
