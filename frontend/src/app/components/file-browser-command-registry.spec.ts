import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { BC, BROWSER_BINDINGS, createFileBrowserCommandDescriptors, type FileBrowserCommandHandlers } from './file-browser-command-registry';

function handlers(): FileBrowserCommandHandlers {
  const noop = () => undefined;
  return {
    openCommandPalette: noop, openGlobalSearch: noop, showPathHistory: noop, quickPathJump: noop,
    navigateUp: noop, refresh: noop, showKeyboardHelp: noop, openSettings: noop, openOperations: noop,
    openSelectionBasket: noop, addToSelectionBasket: noop, clearSelectionBasket: noop,
    cursorUp: noop, cursorDown: noop, cursorLeft: noop, cursorRight: noop, cursorHome: noop, cursorEnd: noop,
    cursorPageUp: noop, cursorPageDown: noop, newTab: noop, closeTab: noop, nextTab: noop, prevTab: noop,
    jumpToTab: fn(), togglePinTab: noop, openItem: noop, quickView: noop, copyCursorPath: noop,
    copy: noop, cut: noop, paste: noop, deleteSelected: noop, selectAll: noop, deselectAll: noop,
    newFolder: noop, rename: noop, toggleCurrent: noop, inlineDirectorySize: noop, extendSelUp: noop,
    extendSelDown: noop, selectByMask: noop, unselectByMask: noop, invertSelection: noop,
    selectSameExt: noop, focusFilter: noop, advancedFilter: noop, clearFilter: noop,
    showWorkspaceSwitcher: noop, newWorkspace: noop, toggleHidden: noop, switchPane: noop,
    copyToOtherPane: noop, moveToOtherPane: noop, focusAddressBar: noop,
    comparePanes: noop, recursiveComparePanes: noop, reviewSyncPlan: noop, selectLeftDifferences: noop, selectRightDifferences: noop,
    copyMissingLeftToRight: noop, copyMissingRightToLeft: noop, diskUsage: noop, findDuplicates: noop,
    batchRename: noop, openWithExternal: noop,
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
    const enabled = fn((id: string) => id !== BC.deleteSelected);
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

describe('browser keybinding collisions', () => {
  it('reserves Ctrl+Shift+P for the command palette and uses Ctrl+Alt+P for tab pinning', () => {
    const palette = BROWSER_BINDINGS.find(binding => binding.id === 'pane.nav.openCommandPalette.ctrlShiftP');
    const pin = BROWSER_BINDINGS.find(binding => binding.id === 'pane.tabs.pin.ctrlAltP');

    expect(palette?.commandId).toBe(BC.openCommandPalette);
    expect(pin?.commandId).toBe(BC.togglePinTab);
    expect(BROWSER_BINDINGS.some(binding => binding.id === 'pane.tabs.pin.ctrlShiftP')).toBe(false);
  });


  it('keeps Ctrl+/ dedicated to the keyboard quick reference', () => {
    const help = BROWSER_BINDINGS.find(binding => binding.id === 'pane.nav.showKeyboardHelp.ctrlSlash');
    expect(help?.commandId).toBe(BC.showKeyboardHelp);
    expect(help?.trigger).toEqual({
      device: 'keyboard',
      chord: { steps: [{ key: '/', modifiers: ['ctrl'] }] },
    });
  });

  it('leaves F1 to the manager-level Input & Shortcuts command', () => {
    const f1Bindings = BROWSER_BINDINGS.filter(binding =>
      binding.trigger.device === 'keyboard'
      && binding.trigger.chord.steps.length === 1
      && binding.trigger.chord.steps[0].key.toLowerCase() === 'f1'
      && (binding.trigger.chord.steps[0].modifiers?.length ?? 0) === 0,
    );

    expect(f1Bindings).toEqual([]);
  });

  it('does not register duplicate browser keyboard chords in the same default context', () => {
    const seen = new Map<string, string>();
    for (const binding of BROWSER_BINDINGS) {
      if (binding.trigger.device !== 'keyboard') continue;
      const chord = binding.trigger.chord.steps
        .map(step => `${[...(step.modifiers ?? [])].sort().join('+')}:${step.key}`)
        .join('>');
      const key = `${binding.contextId}|${binding.when ?? ''}|${chord}`;
      expect(seen.get(key), `duplicate chord ${key}`).toBeUndefined();
      seen.set(key, binding.id);
    }
  });
});