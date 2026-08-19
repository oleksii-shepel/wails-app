import type { ActionAvailability } from '../services/action-availability';
import type { CommandPresentation } from './presented-command';

export type BuiltinFileContextAction =
  | 'open'
  | 'quickView'
  | 'openInNewTab'
  | 'copyPath'
  | 'openWithExternal'
  | 'copy'
  | 'cut'
  | 'delete'
  | 'rename'
  | 'copyToOther'
  | 'moveToOther';

export type FileContextMenuDescriptor = CommandPresentation<BuiltinFileContextAction>;

export function buildBuiltinFileContextMenu(input: {
  availability: ActionAvailability;
  selectedCount: number;
  isDirectory: boolean;
}): FileContextMenuDescriptor[] {
  const a = input.availability;
  return [
    { id: 'open', label: 'Open', shortcut: 'Enter', enabled: true, source: 'builtin', commandId: 'pane.files.open' },
    { id: 'quickView', label: 'Quick View', shortcut: 'F3', enabled: !input.isDirectory, source: 'builtin', commandId: 'pane.files.quickView' },
    { id: 'openInNewTab', label: 'Open in New Tab', enabled: true, source: 'builtin' },
    { id: 'openWithExternal', label: 'Open with External App', enabled: !input.isDirectory, source: 'builtin', commandId: 'pane.files.openWithExternal' },
    { id: 'copyPath', label: 'Copy Path', shortcut: 'Ctrl+Shift+Enter', enabled: true, source: 'builtin', commandId: 'pane.files.copyCursorPath' },
    { id: 'copy', label: 'Copy', shortcut: 'Ctrl+C', enabled: a.copySelected, separatorBefore: true, source: 'builtin', commandId: 'pane.files.copy' },
    { id: 'cut', label: 'Cut', shortcut: 'Ctrl+X', enabled: a.cutSelected, source: 'builtin', commandId: 'pane.files.cut' },
    { id: 'delete', label: 'Delete', shortcut: 'Del', enabled: a.deleteSelected, source: 'builtin', commandId: 'pane.files.delete' },
    { id: 'rename', label: 'Rename', shortcut: 'F2', enabled: input.selectedCount === 1 && a.rename, source: 'builtin', commandId: 'pane.files.rename' },
    { id: 'copyToOther', label: 'Copy to Other Pane', shortcut: 'F5', enabled: a.copyToOther, separatorBefore: true, source: 'builtin', commandId: 'pane.ctrl.copyToOther' },
    { id: 'moveToOther', label: 'Move to Other Pane', shortcut: 'F6', enabled: a.moveToOther, source: 'builtin', commandId: 'pane.ctrl.moveToOther' },
  ];
}

export type BackgroundFileContextAction = 'newFolder' | 'paste' | 'refresh';

export function buildBackgroundFileContextMenu(
  availability: ActionAvailability,
): CommandPresentation<BackgroundFileContextAction>[] {
  return [
    { id: 'newFolder', label: 'New Folder', shortcut: 'F7', enabled: availability.createFolder, source: 'builtin', commandId: 'pane.files.newFolder' },
    { id: 'paste', label: 'Paste', shortcut: 'Ctrl+V', enabled: availability.paste, source: 'builtin', commandId: 'pane.files.paste' },
    { id: 'refresh', label: 'Refresh', shortcut: 'Ctrl+R', enabled: true, separatorBefore: true, source: 'builtin', commandId: 'pane.nav.refresh' },
  ];
}