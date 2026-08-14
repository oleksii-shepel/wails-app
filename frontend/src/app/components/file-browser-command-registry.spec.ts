import { describe, expect, it, vi } from 'vitest';
import { BC, createFileBrowserCommandDescriptors, type FileBrowserCommandHandlers } from './file-browser-command-registry';

function handlers(): FileBrowserCommandHandlers {
  const fn = () => undefined;
  return {
    openCommandPalette: fn, openGlobalSearch: fn, showPathHistory: fn, quickPathJump: fn,
    navigateUp: fn, refresh: fn, showKeyboardHelp: fn, openSettings: fn,
    cursorUp: fn, cursorDown: fn, cursorLeft: fn, cursorRight: fn, cursorHome: fn, cursorEnd: fn,
    cursorPageUp: fn, cursorPageDown: fn, newTab: fn, closeTab: fn, nextTab: fn, prevTab: fn,
    jumpToTab: vi.fn(), togglePinTab: fn, openItem: fn, quickView: fn, copyCursorPath: fn,
    copy: fn, cut: fn, paste: fn, deleteSelected: fn, selectAll: fn, deselectAll: fn,
    newFolder: fn, rename: fn, toggleCurrent: fn, inlineDirectorySize: fn, extendSelUp: fn,
    extendSelDown: fn, selectByMask: fn, unselectByMask: fn, invertSelection: fn,
    selectSameExt: fn, focusFilter: fn, advancedFilter: fn, clearFilter: fn,
    showWorkspaceSwitcher: fn, newWorkspace: fn, toggleHidden: fn, switchPane: fn,
    copyToOtherPane: fn, moveToOtherPane: fn, focusAddressBar: fn, diskUsage: fn,
    batchRename: fn, openWithExternal: fn,
  };
}

describe('createFileBrowserCommandDescriptors', () => {
  it('keeps metadata and execution together for every browser command', () => {
    const list = createFileBrowserCommandDescriptors(handlers());
    const ids = new Set(list.map(command => command.id));

    expect(ids.has(BC.openCommandPalette)).toBe(true);
    expect(ids.has(BC.deleteSelected)).toBe(true);
    expect(ids.has(BC.jumpToTab(9))).toBe(true);
    expect(list.every(command => typeof command.execute === 'function')).toBe(true);
  });

  it('binds dynamic availability without embedding browser policy in the engine', () => {
    const enabled = vi.fn((id: string) => id !== BC.deleteSelected);
    const list = createFileBrowserCommandDescriptors(handlers(), enabled);
    const remove = list.find(command => command.id === BC.deleteSelected)!;
    const refresh = list.find(command => command.id === BC.refresh)!;

    expect(remove.isEnabled?.()).toBe(false);
    expect(refresh.isEnabled?.()).toBe(true);
  });

  it('binds numbered tab commands to their index', () => {
    const h = handlers();
    const list = createFileBrowserCommandDescriptors(h);
    list.find(command => command.id === BC.jumpToTab(4))!.execute();
    expect(h.jumpToTab).toHaveBeenCalledWith(4);
  });
});
