# Windows release checklist

Build with `npm run build:app` from a clean checkout after `npm ci`, or download
the Windows artifact produced by the build workflow for the same commit.
Record the commit ID, Windows version, and result of each check.

## Packaged application

- Launch the executable as a standard user. Confirm the main window opens,
  both panes load, and closing and reopening restores the workspace.
- Use a disposable folder with samples whose names contain Unicode and
  spaces. Include an empty folder and two copies of the same small file.
- Copy and move files between panes. Check the destination contents and
  confirm the source remains after copy and disappears after move.
- Rename a file and verify the new name in the pane and on disk. Attempt a
  conflicting move and verify that skipping preserves both originals.
- Send a file to the recycle bin and restore it. Check its contents after
  restoration. Cancel one conflicting or queued operation.
- Open an archive and extract sample files into the disposable folder.
  Check the contents and confirm unrelated files remain intact.
- Verify a checksum, export an inventory CSV, and cancel a duplicate scan.
  Confirm the dialogs can be opened and dismissed by keyboard.

Ship once the executable launches and the file-operation checks pass.
Keep the disposable test data separate from personal or project files.
