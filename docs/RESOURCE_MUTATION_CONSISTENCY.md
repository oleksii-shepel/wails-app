# Resource Mutation Consistency

Successful mutations are treated as changes to resources and locations, not as commands owned by one pane.

## Invariants

- Every visible pane showing a mutated directory refreshes.
- Deleting an open directory, or any ancestor of it, recovers that tab to the nearest surviving parent.
- Renaming or moving an open directory remaps open descendant directory tabs to the corresponding destination path.
- Inactive directory tabs receive the same recovery/remapping before activation.
- Viewer tabs follow moved targets and close when their target (or an ancestor) is deleted.
- Source and destination directories are invalidated for copy/move operations.
- Archive extraction invalidates its destination directory; archive edits invalidate other views of the archive.
- Filesystem watcher notifications are location-based, not pane-id gated.
- Each pane removes only its own Wails event listeners.

## Mutation producers

`ResourceMutationService` receives structured `create`, `delete`, `move`, `copy`, `modify`, and `restore` notifications from `ApiService` and archive operations. External filesystem events continue through the Wails watcher path.

## Example

If the left pane shows `filesystem://c/Work`, the right pane shows `filesystem://c/Work/Project/src`, and `filesystem://c/Work/Project` is deleted, the right pane recovers to `filesystem://c/Work` instead of remaining on a dead path.
