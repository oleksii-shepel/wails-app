/**
 * file-browser-command-registry.ts
 *
 * Single source of truth for all keybindings and command IDs used by the
 * file-browser surface.  Replaces the old ShortcutRegistryService +
 * ShortcutCallbacks fan-out pattern.
 *
 * Usage:
 *   const cleanup = registerFileBrowserCommands(this.inputEngine, handlers, 'file-browser');
 *   // …in ngOnDestroy:
 *   cleanup();
 */

import type { InputBindingDefinition } from '../services/input-system.types';
import type { InputEngineService } from '../services/input-engine.service';

// ─── Command IDs ──────────────────────────────────────────────────────────────

export const BC = {
  openGlobalSearch:     'pane.nav.openGlobalSearch',
  showPathHistory:      'pane.nav.showPathHistory',
  quickPathJump:        'pane.nav.quickPathJump',
  navigateUp:           'pane.nav.navigateUp',
  refresh:              'pane.nav.refresh',
  showKeyboardHelp:     'pane.nav.showKeyboardHelp',
  cursorUp:             'pane.cursor.up',
  cursorDown:           'pane.cursor.down',
  cursorLeft:           'pane.cursor.left',
  cursorRight:          'pane.cursor.right',
  cursorHome:           'pane.cursor.home',
  cursorEnd:            'pane.cursor.end',
  cursorPageUp:         'pane.cursor.pageUp',
  cursorPageDown:       'pane.cursor.pageDown',
  newTab:               'pane.tabs.new',
  closeTab:             'pane.tabs.close',
  nextTab:              'pane.tabs.next',
  prevTab:              'pane.tabs.prev',
  jumpToTab:            (i: number) => `pane.tabs.jumpTo${i}`,
  togglePinTab:         'pane.tabs.togglePin',
  openItem:             'pane.files.open',
  quickView:            'pane.files.quickView',
  copy:                 'pane.files.copy',
  cut:                  'pane.files.cut',
  paste:                'pane.files.paste',
  deleteSelected:       'pane.files.delete',
  selectAll:            'pane.files.selectAll',
  deselectAll:          'pane.files.deselectAll',
  newFolder:            'pane.files.newFolder',
  rename:               'pane.files.rename',
  toggleCurrent:        'pane.files.toggleCurrent',
  inlineDirectorySize:  'pane.files.inlineDirectorySize',
  extendSelUp:          'pane.files.extendSelectionUp',
  extendSelDown:        'pane.files.extendSelectionDown',
  selectByMask:         'pane.files.selectByMask',
  unselectByMask:       'pane.files.unselectByMask',
  invertSelection:      'pane.files.invertSelection',
  selectSameExt:        'pane.files.selectSameExtension',
  focusFilter:          'pane.search.focusFilter',
  advancedFilter:       'pane.search.advanced',
  clearFilter:          'pane.search.clearFilter',
  showWorkspaceSwitcher:'pane.ws.showSwitcher',
  newWorkspace:         'pane.ws.new',
  toggleHidden:         'pane.ws.toggleHidden',
  switchPane:           'pane.ctrl.switchPane',
  copyToOtherPane:      'pane.ctrl.copyToOther',
  moveToOtherPane:      'pane.ctrl.moveToOther',
  focusAddressBar:      'pane.ctrl.focusAddressBar',
  diskUsage:            'pane.tools.diskUsage',
} as const;

// ─── Binding helpers ──────────────────────────────────────────────────────────

type Mod = 'ctrl' | 'shift' | 'alt' | 'meta';

function kb(
  id: string,
  commandId: string,
  key: string,
  description: string,
  opts?: {
    modifiers?: Mod[];
    when?: string;
    stopPropagation?: boolean;
    category?: string;
  },
): InputBindingDefinition {
  return {
    id,
    commandId,
    contextId: 'global',
    source: 'default',
    description,
    category: opts?.category,
    when: opts?.when ?? '!typing && !quickAccess',
    preventDefault: true,
    stopPropagation: opts?.stopPropagation !== false,
    trigger: {
      device: 'keyboard',
      chord: {
        steps: [{ key, modifiers: opts?.modifiers?.length ? opts.modifiers : undefined }],
      },
    },
  };
}

function chord(
  id: string,
  commandId: string,
  s1: { key: string; modifiers?: Mod[] },
  s2: { key: string; modifiers?: Mod[] },
  description: string,
  category: string,
): InputBindingDefinition {
  return {
    id,
    commandId,
    contextId: 'global',
    source: 'default',
    description,
    category,
    when: '!typing && !quickAccess',
    preventDefault: true,
    stopPropagation: true,
    trigger: {
      device: 'keyboard',
      chord: {
        steps: [
          { key: s1.key, modifiers: s1.modifiers?.length ? s1.modifiers : undefined },
          { key: s2.key, modifiers: s2.modifiers?.length ? s2.modifiers : undefined },
        ],
      },
    },
  };
}

const NAV  = 'Navigation';
const TABS = 'Tab Management';
const FILES = 'File Operations';
const SRCH = 'Search & Filter';
const WS   = 'Workspaces';
const PANE = 'Pane Control';

// ─── Binding catalog ──────────────────────────────────────────────────────────

export const BROWSER_BINDINGS: InputBindingDefinition[] = [

  // ── Navigation ─────────────────────────────────────────────────────────────
  kb('pane.nav.openGlobalSearch.ctrlK', BC.openGlobalSearch, 'k', 'Open global search', { modifiers: ['ctrl'], category: NAV }),
  chord('pane.nav.openGlobalSearch.gs', BC.openGlobalSearch, { key: 'g' }, { key: 's' }, 'Open global search (G then S)', NAV),
  kb('pane.nav.showPathHistory.ctrlH', BC.showPathHistory, 'h', 'Show path history', { modifiers: ['ctrl'], category: NAV }),
  chord('pane.nav.showPathHistory.gh', BC.showPathHistory, { key: 'g' }, { key: 'h' }, 'Show path history (G then H)', NAV),
  kb('pane.nav.quickPathJump.ctrlP', BC.quickPathJump, 'p', 'Quick path jump', { modifiers: ['ctrl'], category: NAV }),
  kb('pane.nav.navigateUp.backspace', BC.navigateUp, 'backspace', 'Go to parent directory', { category: NAV }),
  kb('pane.nav.refresh.ctrlR', BC.refresh, 'r', 'Refresh current pane', { modifiers: ['ctrl'], category: NAV }),
  kb('pane.nav.showKeyboardHelp.ctrlSlash', BC.showKeyboardHelp, '/', 'Show keyboard shortcuts', { modifiers: ['ctrl'], category: NAV }),

  // ── Cursor movement ─────────────────────────────────────────────────────────
  kb('pane.cursor.up',       BC.cursorUp,        'arrowup',    'Move cursor up',              { category: NAV }),
  kb('pane.cursor.down',     BC.cursorDown,      'arrowdown',  'Move cursor down',            { category: NAV }),
  kb('pane.cursor.left',     BC.cursorLeft,      'arrowleft',  'Move cursor left',            { category: NAV }),
  kb('pane.cursor.right',    BC.cursorRight,     'arrowright', 'Move cursor right',           { category: NAV }),
  kb('pane.cursor.home',     BC.cursorHome,      'home',       'Move cursor to first item',   { category: NAV }),
  kb('pane.cursor.end',      BC.cursorEnd,       'end',        'Move cursor to last item',    { category: NAV }),
  kb('pane.cursor.pageUp',   BC.cursorPageUp,    'pageup',     'Move cursor up by page',      { category: NAV }),
  kb('pane.cursor.pageDown', BC.cursorPageDown,  'pagedown',   'Move cursor down by page',    { category: NAV }),

  // ── Tabs ─────────────────────────────────────────────────────────────────────
  kb('pane.tabs.new.ctrlT',          BC.newTab,       't',   'Open new tab',              { modifiers: ['ctrl'],          category: TABS }),
  kb('pane.tabs.close.ctrlW',        BC.closeTab,     'w',   'Close current tab',         { modifiers: ['ctrl'],          category: TABS }),
  kb('pane.tabs.next.ctrlTab',       BC.nextTab,      'tab', 'Switch to next tab',         { modifiers: ['ctrl'],          category: TABS }),
  kb('pane.tabs.prev.ctrlShiftTab',  BC.prevTab,      'tab', 'Switch to previous tab',    { modifiers: ['ctrl', 'shift'], category: TABS }),
  kb('pane.tabs.pin.ctrlShiftP',     BC.togglePinTab, 'p',   'Pin/unpin current tab',     { modifiers: ['ctrl', 'shift'], category: TABS }),
  ...[1, 2, 3, 4, 5, 6, 7, 8, 9].map(i =>
    kb(`pane.tabs.jumpTo${i}.ctrl${i}`, BC.jumpToTab(i), i.toString(), `Jump to tab ${i}`, { modifiers: ['ctrl'], category: TABS }),
  ),

  // ── File operations ───────────────────────────────────────────────────────────
  kb('pane.files.open.enter',          BC.openItem,       'enter',    'Open file or folder',                          { category: FILES }),
  kb('pane.files.quickView.f3',        BC.quickView,      'f3',       'Quick view selected item (F3)',                 { category: FILES }),
  kb('pane.files.copy.ctrlC',          BC.copy,           'c',        'Copy selected items',                          { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.cut.ctrlX',           BC.cut,            'x',        'Cut selected items',                           { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.paste.ctrlV',         BC.paste,          'v',        'Paste items',                                  { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.delete.delete',       BC.deleteSelected, 'delete',   'Delete selected items',                        { category: FILES }),
  kb('pane.files.delete.f8',           BC.deleteSelected, 'f8',       'Delete selected items (F8)',                   { category: FILES }),
  kb('pane.files.selectAll.ctrlA',     BC.selectAll,      'a',        'Select all items (Ctrl+A)',                    { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.deselectAll.ctrlD',   BC.deselectAll,    'd',        'Deselect all items (Ctrl+D)',                  { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.inlineDirectorySize.space', BC.inlineDirectorySize, 'space', 'Calculate current folder size inline, then mark item and move down', { category: FILES }),
  kb('pane.files.toggleCurrent.insert', BC.toggleCurrent, 'insert',   'Mark current directory or toggle file and move down', { category: FILES }),
  kb('pane.files.extendUp.shiftUp',    BC.extendSelUp,    'arrowup',  'Extend selection upward (Shift+ArrowUp)',      { modifiers: ['shift'], category: FILES }),
  kb('pane.files.extendDown.shiftDown',BC.extendSelDown,  'arrowdown','Extend selection downward (Shift+ArrowDown)',  { modifiers: ['shift'], category: FILES }),
  kb('pane.files.selectByMask.altS',   BC.selectByMask,   's',        'Select by mask (Alt+S)',                       { modifiers: ['alt'], category: FILES }),
  kb('pane.files.unselectByMask.altU', BC.unselectByMask, 'u',        'Unselect by mask (Alt+U)',                     { modifiers: ['alt'], category: FILES }),
  kb('pane.files.invertSel.altI',      BC.invertSelection,'i',        'Invert selection (Alt+I)',                     { modifiers: ['alt'], category: FILES }),
  kb('pane.files.selectSameExt.altE',  BC.selectSameExt,  'e',        'Select files with same extension (Alt+E)',     { modifiers: ['alt'], category: FILES }),
  kb('pane.files.newFolder.ctrlShiftN',BC.newFolder,      'n',        'Create new folder (Ctrl+Shift+N)',             { modifiers: ['ctrl', 'shift'], category: FILES }),
  kb('pane.files.newFolder.f7',        BC.newFolder,      'f7',       'Create folder (F7)',                           { category: FILES }),
  kb('pane.files.rename.f2',           BC.rename,         'f2',       'Rename selected item (F2)',                    { category: FILES }),
  kb('pane.files.rename.shiftF6',      BC.rename,         'f6',       'Rename selected item (Shift+F6)',              { modifiers: ['shift'], category: FILES }),
  kb('pane.files.copyToOther.f5',      BC.copyToOtherPane,'f5',       'Copy to other pane (F5)',                      { category: FILES }),
  kb('pane.files.moveToOther.f6',      BC.moveToOtherPane,'f6',       'Move to other pane (F6)',                      { category: FILES }),

  // ── Search & Filter ────────────────────────────────────────────────────────
  kb('pane.search.focusFilter.ctrlF',   BC.focusFilter,   'f', 'Focus filter input',      { modifiers: ['ctrl'],          category: SRCH }),
  kb('pane.search.advanced.ctrlShiftF', BC.advancedFilter,'f', 'Advanced filter options', { modifiers: ['ctrl', 'shift'], category: SRCH }),
  kb('pane.search.clearFilter.escape',  BC.clearFilter,   'escape', 'Clear filter',       { when: 'filterInput',          category: SRCH }),

  // ── Workspaces ────────────────────────────────────────────────────────────────
  kb('pane.ws.showSwitcher.ctrlShiftW', BC.showWorkspaceSwitcher, 'w', 'Open workspace switcher',  { modifiers: ['ctrl', 'shift'],       category: WS }),
  kb('pane.ws.new.ctrlShiftAltN',       BC.newWorkspace,          'n', 'Create new workspace',     { modifiers: ['ctrl', 'shift', 'alt'], category: WS }),
  kb('pane.ws.toggleHidden.ctrlShiftH', BC.toggleHidden,          'h', 'Show/hide hidden files',   { modifiers: ['ctrl', 'shift'],       category: WS }),

  // ── Pane control ──────────────────────────────────────────────────────────────
  kb('pane.ctrl.switch.tab',         BC.switchPane,     'tab', 'Switch between panes',           { category: PANE }),
  kb('pane.ctrl.copyToOther.ctrlShiftC', BC.copyToOtherPane, 'c', 'Copy selected to other pane',
    { modifiers: ['ctrl', 'shift'], when: '!typing && !quickAccess && activePane', category: PANE }),
  kb('pane.ctrl.moveToOther.ctrlShiftM', BC.moveToOtherPane, 'm', 'Move selected to other pane',
    { modifiers: ['ctrl', 'shift'], when: '!typing && !quickAccess && activePane', category: PANE }),
  kb('pane.ctrl.focusAddress.ctrlL', BC.focusAddressBar, 'l', 'Focus address bar (Ctrl+L)',
    { modifiers: ['ctrl'], stopPropagation: false, category: PANE }),
  kb('pane.ctrl.focusAddress.altD',  BC.focusAddressBar, 'd', 'Focus address bar (Alt+D)',
    { modifiers: ['alt'],  stopPropagation: false, category: PANE }),

  // ── Tools ─────────────────────────────────────────────────────────────────
  kb('pane.tools.diskUsage.ctrlShiftD', BC.diskUsage, 'd', 'Analyze disk usage',
    { modifiers: ['ctrl', 'shift'], category: FILES }),
];

// ─── Handler interface ────────────────────────────────────────────────────────

export interface FileBrowserCommandHandlers {
  openGlobalSearch:     () => void;
  showPathHistory:      () => void;
  quickPathJump:        () => void;
  navigateUp:           () => void;
  refresh:              () => void;
  showKeyboardHelp:     () => void;
  cursorUp:             () => void;
  cursorDown:           () => void;
  cursorLeft:           () => void;
  cursorRight:          () => void;
  cursorHome:           () => void;
  cursorEnd:            () => void;
  cursorPageUp:         () => void;
  cursorPageDown:       () => void;
  newTab:               () => void;
  closeTab:             () => void;
  nextTab:              () => void;
  prevTab:              () => void;
  jumpToTab:            (index: number) => void;
  togglePinTab:         () => void;
  openItem:             () => void;
  quickView:            () => void;
  copy:                 () => void;
  cut:                  () => void;
  paste:                () => void;
  deleteSelected:       () => void;
  selectAll:            () => void;
  deselectAll:          () => void;
  newFolder:            () => void;
  rename:               () => void;
  toggleCurrent:        () => void;
  inlineDirectorySize:  () => void;
  extendSelUp:          () => void;
  extendSelDown:        () => void;
  selectByMask:         () => void;
  unselectByMask:       () => void;
  invertSelection:      () => void;
  selectSameExt:        () => void;
  focusFilter:          () => void;
  advancedFilter:       () => void;
  clearFilter:          () => void;
  showWorkspaceSwitcher:() => void;
  newWorkspace:         () => void;
  toggleHidden:         () => void;
  switchPane:           () => void;
  copyToOtherPane:      () => void;
  moveToOtherPane:      () => void;
  focusAddressBar:      () => void;
  diskUsage:            () => void;
}

// ─── Registration ─────────────────────────────────────────────────────────────

/**
 * Registers all file-browser keybindings and command handlers with the input
 * engine.  Returns a cleanup function that unregisters everything — call it
 * in ngOnDestroy.
 */
export function registerFileBrowserCommands(
  engine: InputEngineService,
  handlers: FileBrowserCommandHandlers,
  ownerId: string,
): () => void {
  for (const binding of BROWSER_BINDINGS) {
    engine.registerBinding(binding);
  }

  engine.registerCommand(BC.openGlobalSearch,      handlers.openGlobalSearch,      ownerId);
  engine.registerCommand(BC.showPathHistory,        handlers.showPathHistory,        ownerId);
  engine.registerCommand(BC.quickPathJump,          handlers.quickPathJump,          ownerId);
  engine.registerCommand(BC.navigateUp,             handlers.navigateUp,             ownerId);
  engine.registerCommand(BC.refresh,                handlers.refresh,                ownerId);
  engine.registerCommand(BC.showKeyboardHelp,       handlers.showKeyboardHelp,       ownerId);
  engine.registerCommand(BC.cursorUp,               handlers.cursorUp,               ownerId);
  engine.registerCommand(BC.cursorDown,             handlers.cursorDown,             ownerId);
  engine.registerCommand(BC.cursorLeft,             handlers.cursorLeft,             ownerId);
  engine.registerCommand(BC.cursorRight,            handlers.cursorRight,            ownerId);
  engine.registerCommand(BC.cursorHome,             handlers.cursorHome,             ownerId);
  engine.registerCommand(BC.cursorEnd,              handlers.cursorEnd,              ownerId);
  engine.registerCommand(BC.cursorPageUp,           handlers.cursorPageUp,           ownerId);
  engine.registerCommand(BC.cursorPageDown,         handlers.cursorPageDown,         ownerId);
  engine.registerCommand(BC.newTab,                 handlers.newTab,                 ownerId);
  engine.registerCommand(BC.closeTab,               handlers.closeTab,               ownerId);
  engine.registerCommand(BC.nextTab,                handlers.nextTab,                ownerId);
  engine.registerCommand(BC.prevTab,                handlers.prevTab,                ownerId);
  for (let i = 1; i <= 9; i++) {
    const idx = i;
    engine.registerCommand(BC.jumpToTab(i), () => handlers.jumpToTab(idx), ownerId);
  }
  engine.registerCommand(BC.togglePinTab,           handlers.togglePinTab,           ownerId);
  engine.registerCommand(BC.openItem,               handlers.openItem,               ownerId);
  engine.registerCommand(BC.quickView,              handlers.quickView,              ownerId);
  engine.registerCommand(BC.copy,                   handlers.copy,                   ownerId);
  engine.registerCommand(BC.cut,                    handlers.cut,                    ownerId);
  engine.registerCommand(BC.paste,                  handlers.paste,                  ownerId);
  engine.registerCommand(BC.deleteSelected,         handlers.deleteSelected,         ownerId);
  engine.registerCommand(BC.selectAll,              handlers.selectAll,              ownerId);
  engine.registerCommand(BC.deselectAll,            handlers.deselectAll,            ownerId);
  engine.registerCommand(BC.newFolder,              handlers.newFolder,              ownerId);
  engine.registerCommand(BC.rename,                 handlers.rename,                 ownerId);
  engine.registerCommand(BC.toggleCurrent,          handlers.toggleCurrent,          ownerId);
  engine.registerCommand(BC.inlineDirectorySize,    handlers.inlineDirectorySize,    ownerId);
  engine.registerCommand(BC.extendSelUp,            handlers.extendSelUp,            ownerId);
  engine.registerCommand(BC.extendSelDown,          handlers.extendSelDown,          ownerId);
  engine.registerCommand(BC.selectByMask,           handlers.selectByMask,           ownerId);
  engine.registerCommand(BC.unselectByMask,         handlers.unselectByMask,         ownerId);
  engine.registerCommand(BC.invertSelection,        handlers.invertSelection,        ownerId);
  engine.registerCommand(BC.selectSameExt,          handlers.selectSameExt,          ownerId);
  engine.registerCommand(BC.focusFilter,            handlers.focusFilter,            ownerId);
  engine.registerCommand(BC.advancedFilter,         handlers.advancedFilter,         ownerId);
  engine.registerCommand(BC.clearFilter,            handlers.clearFilter,            ownerId);
  engine.registerCommand(BC.showWorkspaceSwitcher,  handlers.showWorkspaceSwitcher,  ownerId);
  engine.registerCommand(BC.newWorkspace,           handlers.newWorkspace,           ownerId);
  engine.registerCommand(BC.toggleHidden,           handlers.toggleHidden,           ownerId);
  engine.registerCommand(BC.switchPane,             handlers.switchPane,             ownerId);
  engine.registerCommand(BC.copyToOtherPane,        handlers.copyToOtherPane,        ownerId);
  engine.registerCommand(BC.moveToOtherPane,        handlers.moveToOtherPane,        ownerId);
  engine.registerCommand(BC.focusAddressBar,        handlers.focusAddressBar,        ownerId);
  engine.registerCommand(BC.diskUsage,              handlers.diskUsage,              ownerId);

  return () => {
    for (const binding of BROWSER_BINDINGS) {
      engine.unregisterBinding(binding.id);
    }
    const allCommands = [
      BC.openGlobalSearch, BC.showPathHistory, BC.quickPathJump, BC.navigateUp,
      BC.refresh, BC.showKeyboardHelp, BC.cursorUp, BC.cursorDown, BC.cursorLeft,
      BC.cursorRight, BC.cursorHome, BC.cursorEnd, BC.cursorPageUp, BC.cursorPageDown,
      BC.newTab, BC.closeTab, BC.nextTab, BC.prevTab, BC.togglePinTab,
      BC.openItem, BC.quickView, BC.copy, BC.cut, BC.paste, BC.deleteSelected,
      BC.selectAll, BC.deselectAll, BC.newFolder, BC.rename, BC.toggleCurrent,
      BC.inlineDirectorySize,
      BC.extendSelUp, BC.extendSelDown, BC.selectByMask, BC.unselectByMask,
      BC.invertSelection, BC.selectSameExt, BC.focusFilter, BC.advancedFilter,
      BC.clearFilter, BC.showWorkspaceSwitcher, BC.newWorkspace, BC.toggleHidden,
      BC.switchPane, BC.copyToOtherPane, BC.moveToOtherPane, BC.focusAddressBar,
      BC.diskUsage,
    ];
    for (const cmd of allCommands) {
      engine.unregisterCommand(cmd, ownerId);
    }
    for (let i = 1; i <= 9; i++) {
      engine.unregisterCommand(BC.jumpToTab(i), ownerId);
    }
  };
}
