# @reassembly/core

Cross-language contracts and framework-neutral runtime shared by Manager and plugins.

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

Frontend code is deliberately domain-oriented rather than a flat list.
The Go module owns stable wire contracts shared by Manager and future backend plugins.

Angular components, Wails bindings, workspace persistence, and concrete provider
implementations stay outside core.
