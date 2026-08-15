import { BROWSER_BINDINGS, BC, createFileBrowserCommandDescriptors, type FileBrowserCommandHandlers } from './file-browser-command-registry';

function handlers(): FileBrowserCommandHandlers {
  const fn = () => undefined;
  const jumpToTab = jasmine.createSpy('jumpToTab');
  return {
    openCommandPalette: fn, openGlobalSearch: fn, showPathHistory: fn, quickPathJump: fn,
    navigateUp: fn, refresh: fn, showKeyboardHelp: fn, openSettings: fn, openOperations: fn,
    openSelectionBasket: fn, addToSelectionBasket: fn, clearSelectionBasket: fn,
    cursorUp: fn, cursorDown: fn, cursorLeft: fn, cursorRight: fn, cursorHome: fn, cursorEnd: fn,
    cursorPageUp: fn, cursorPageDown: fn, newTab: fn, closeTab: fn, nextTab: fn, prevTab: fn,
    jumpToTab, togglePinTab: fn, openItem: fn, quickView: fn, copyCursorPath: fn,
    copy: fn, cut: fn, paste: fn, deleteSelected: fn, selectAll: fn, deselectAll: fn,
    newFolder: fn, rename: fn, toggleCurrent: fn, inlineDirectorySize: fn, extendSelUp: fn,
    extendSelDown: fn, selectByMask: fn, unselectByMask: fn, invertSelection: fn,
    selectSameExt: fn, focusFilter: fn, advancedFilter: fn, clearFilter: fn,
    showWorkspaceSwitcher: fn, newWorkspace: fn, toggleHidden: fn, switchPane: fn,
    copyToOtherPane: fn, moveToOtherPane: fn, focusAddressBar: fn,
    comparePanes: fn, recursiveComparePanes: fn, reviewSyncPlan: fn, selectLeftDifferences: fn, selectRightDifferences: fn,
    copyMissingLeftToRight: fn, copyMissingRightToLeft: fn, diskUsage: fn, findDuplicates: fn,
    batchRename: fn, openWithExternal: fn,
  };
}

describe('createFileBrowserCommandDescriptors', () => {
  it('keeps metadata and execution together for every browser command', () => {
    const list = createFileBrowserCommandDescriptors(handlers());
    const ids = new Set(list.map(command => command.id));

    expect(ids.has(BC.openCommandPalette)).toBe(true);
    expect(ids.has(BC.deleteSelected)).toBe(true);
    expect(ids.has(BC.comparePanes)).toBe(true);
    expect(ids.has(BC.openSelectionBasket)).toBe(true);
    expect(ids.has(BC.addToSelectionBasket)).toBe(true);
    expect(ids.has(BC.jumpToTab(9))).toBe(true);
    expect(list.every(command => typeof command.execute === 'function')).toBe(true);
  });

  it('binds dynamic availability without embedding browser policy in the engine', () => {
    const enabled = jasmine.createSpy('enabled').and.callFake((id: string) => id !== BC.deleteSelected);
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


describe('BROWSER_BINDINGS', () => {
  it('reserves Ctrl+Shift+P for the command palette and uses Ctrl+Alt+P for tab pinning', () => {
    const palette = BROWSER_BINDINGS.find(binding => binding.id === 'pane.nav.openCommandPalette.ctrlShiftP');
    const pin = BROWSER_BINDINGS.find(binding => binding.id === 'pane.tabs.pin.ctrlAltP');
    const stalePin = BROWSER_BINDINGS.find(binding => binding.id === 'pane.tabs.pin.ctrlShiftP');

    expect(palette?.commandId).toBe(BC.openCommandPalette);
    expect(pin?.commandId).toBe(BC.togglePinTab);
    expect(stalePin).toBeUndefined();
  });

  it('keeps the command palette available while typing, except inside quick access overlays', () => {
    const palette = BROWSER_BINDINGS.find(binding => binding.id === 'pane.nav.openCommandPalette.ctrlShiftP');
    const paletteF1 = BROWSER_BINDINGS.find(binding => binding.id === 'pane.nav.openCommandPalette.f1');

    expect(palette?.when).toBe('!quickAccess');
    expect(paletteF1?.when).toBe('!quickAccess');
  });
});
