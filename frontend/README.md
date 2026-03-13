# Frontend Notes

## Current Architecture

The frontend is no longer centered on one large pane component. The current structure is built around a thin pane host, controller-backed orchestration, and explicit surface/view boundaries.

- Pane host/facade: [src/app/components/file-pane.component.ts](src/app/components/file-pane.component.ts)
- Pane controller and factory layer: [src/app/components](src/app/components)
- Dynamic rendered surfaces: [src/app/surfaces](src/app/surfaces)
- Viewer hosting layer: [src/app/viewers](src/app/viewers)

The key rule is that `file-pane.component.ts` owns composition and event adaptation, while reusable orchestration lives in controllers, factories, or surface VMs.

## View Operation Parity

All view components must support the same file-manager operations and route them through shared pane-level handlers.

| Operation | List | Grid | Details | Tree |
|---|---:|---:|---:|---:|
| Single select | ✅ | ✅ | ✅ | ✅ |
| Multi-select (Ctrl/Cmd toggle) | ✅ | ✅ | ✅ | ✅ |
| Range select (Shift) | ✅ | ✅ | ✅ | ✅ |
| Select all / Deselect all | ✅ | ✅ | ✅ | ✅ |
| Open item (Enter / double click) | ✅ | ✅ | ✅ | ✅ |
| Rename selected (F2 / Shift+F6) | ✅ | ✅ | ✅ | ✅ |
| Copy to other pane (F5) | ✅ | ✅ | ✅ | ✅ |
| Move to other pane (F6) | ✅ | ✅ | ✅ | ✅ |
| New folder (F7) | ✅ | ✅ | ✅ | ✅ |
| Delete selected (F8 / Delete) | ✅ | ✅ | ✅ | ✅ |
| Drag & drop copy/move (Ctrl/Cmd = copy) | ✅ | ✅ | ✅ | ✅ |

### Shortcut Baseline

- `F5`: copy to other pane
- `F6`: move to other pane
- `Shift+F6`: rename selected
- `F7`: create folder
- `F8`: delete selected
- `Ctrl+R`: refresh current pane
- `Ctrl+A`: select all
- `Ctrl+D`: clear selection

### Implementation Rule

If a new operation is added to one view, wire it through `file-view-container` and `file-pane` so list/grid/details/tree stay behaviorally equivalent.

## Boundary Rules

- New pane behavior should prefer controller or factory extraction over adding more inline orchestration to `file-pane.component.ts`.
- Surface-specific state belongs to the corresponding surface VM/runtime, not directly to the pane host.
- Browser-facing actions should stay grouped behind pane command objects instead of adding many single-purpose public pane methods.
- If a view-level feature changes list/grid/details/tree behavior, update the parity matrix above.
