import type { ActionAvailability } from '../services/action-availability';

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

export interface FileContextMenuDescriptor {
  readonly id: BuiltinFileContextAction;
  readonly label: string;
  readonly shortcut?: string;
  readonly enabled: boolean;
  readonly separatorBefore?: boolean;
}

export function buildBuiltinFileContextMenu(input: {
  availability: ActionAvailability;
  selectedCount: number;
  isDirectory: boolean;
}): FileContextMenuDescriptor[] {
  const a = input.availability;
  return [
    { id: 'open', label: 'Open', shortcut: 'Enter', enabled: true },
    { id: 'quickView', label: 'Quick View', shortcut: 'F3', enabled: !input.isDirectory },
    { id: 'openInNewTab', label: 'Open in New Tab', enabled: true },
    { id: 'openWithExternal', label: 'Open with External App', enabled: !input.isDirectory },
    { id: 'copyPath', label: 'Copy Path', shortcut: 'Ctrl+Shift+Enter', enabled: true },
    { id: 'copy', label: 'Copy', shortcut: 'Ctrl+C', enabled: a.copySelected, separatorBefore: true },
    { id: 'cut', label: 'Cut', shortcut: 'Ctrl+X', enabled: a.cutSelected },
    { id: 'delete', label: 'Delete', shortcut: 'Del', enabled: a.deleteSelected },
    { id: 'rename', label: 'Rename', shortcut: 'F2', enabled: input.selectedCount === 1 && a.rename },
    { id: 'copyToOther', label: 'Copy to Other Pane', shortcut: 'F5', enabled: a.copyToOther, separatorBefore: true },
    { id: 'moveToOther', label: 'Move to Other Pane', shortcut: 'F6', enabled: a.moveToOther },
  ];
}

export type BackgroundFileContextAction = 'newFolder' | 'paste' | 'refresh';

export function buildBackgroundFileContextMenu(
  availability: ActionAvailability,
): Array<{
  readonly id: BackgroundFileContextAction;
  readonly label: string;
  readonly shortcut?: string;
  readonly enabled: boolean;
  readonly separatorBefore?: boolean;
}> {
  return [
    { id: 'newFolder', label: 'New Folder', shortcut: 'F7', enabled: availability.createFolder },
    { id: 'paste', label: 'Paste', shortcut: 'Ctrl+V', enabled: availability.paste },
    { id: 'refresh', label: 'Refresh', shortcut: 'Ctrl+R', enabled: true, separatorBefore: true },
  ];
}
