# Frontend Notes

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
