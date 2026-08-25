/**
 * Shared command IDs and default keyboard bindings for the file-manager UI.
 *
 * This module intentionally contains no shell-specific command handlers.
 * Command execution belongs to the active shell/pane composition; this file is
 * only the stable catalog shared by panes, settings, and keybinding storage.
 */

import type { InputBindingDefinition } from '../services/input-system.types';

export const FILE_MANAGER_COMMANDS = {
  openCommandPalette:   'pane.nav.openCommandPalette',
  openGlobalSearch:     'pane.nav.openGlobalSearch',
  showPathHistory:      'pane.nav.showPathHistory',
  quickPathJump:        'pane.nav.quickPathJump',
  navigateUp:           'pane.nav.navigateUp',
  refresh:              'pane.nav.refresh',
  showKeyboardHelp:     'pane.nav.showKeyboardHelp',
  openSettings:         'pane.nav.openSettings',
  openOperations:       'app.operations.open',
  openSelectionBasket:  'app.basket.open',
  addToSelectionBasket: 'app.basket.add',
  clearSelectionBasket: 'app.basket.clear',

  // User-openable dialogs / centers exposed through the application menu.
  operationHistory:     'reassembly.edit.operationHistory',
  showClosedTabs:       'reassembly.manager.showClosedTabs',
  openQuickAccess:      'reassembly.navigation.quickAccess',
  showPinnedLocations:  'reassembly.navigation.pinnedLocations',
  openRemoteConnections:'reassembly.navigation.remoteConnections',
  remoteConnect:        'reassembly.navigation.remoteConnect',
  openTrash:            'reassembly.navigation.trash',
  openInputCenter:      'reassembly.help.shortcuts',
  openPluginSettings:   'reassembly.settings.plugins',
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
  copyCursorPath:       'pane.files.copyCursorPath',
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
  comparePanes:          'pane.compare.toggle',
  recursiveComparePanes: 'pane.compare.recursive',
  reviewSyncPlan:         'pane.compare.review',
  selectLeftDifferences: 'pane.compare.selectLeftDifferences',
  selectRightDifferences:'pane.compare.selectRightDifferences',
  copyMissingLeftToRight:'pane.compare.copyMissingLeftToRight',
  copyMissingRightToLeft:'pane.compare.copyMissingRightToLeft',
  diskUsage:            'pane.tools.diskUsage',
  findDuplicates:       'pane.tools.findDuplicates',
  batchRename:          'pane.files.batchRename',
  openWithExternal:     'pane.files.openWithExternal',
  createArchive:        'pane.files.createArchive',
} as const;

const NAV  = 'Navigation';
const TABS = 'Tab Management';
const FILES = 'File Operations';
const SRCH = 'Search & Filter';
const WS   = 'Workspaces';
const PANE = 'Pane Control';
const TOOLS = 'Tools';

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

// ─── Binding catalog ──────────────────────────────────────────────────────────

export const FILE_MANAGER_BINDINGS: InputBindingDefinition[] = [

  // ── Navigation ─────────────────────────────────────────────────────────────
  kb('pane.nav.openCommandPalette.ctrlShiftP', FILE_MANAGER_COMMANDS.openCommandPalette, 'p', 'Open command palette', { modifiers: ['ctrl', 'shift'], category: NAV }),
  kb('pane.nav.openGlobalSearch.ctrlK', FILE_MANAGER_COMMANDS.openGlobalSearch, 'k', 'Open global search', { modifiers: ['ctrl'], category: NAV }),
  chord('pane.nav.openGlobalSearch.gs', FILE_MANAGER_COMMANDS.openGlobalSearch, { key: 'g' }, { key: 's' }, 'Open global search (G then S)', NAV),
  kb('pane.nav.showPathHistory.ctrlH', FILE_MANAGER_COMMANDS.showPathHistory, 'h', 'Show path history', { modifiers: ['ctrl'], category: NAV }),
  chord('pane.nav.showPathHistory.gh', FILE_MANAGER_COMMANDS.showPathHistory, { key: 'g' }, { key: 'h' }, 'Show path history (G then H)', NAV),
  kb('pane.nav.quickPathJump.ctrlP', FILE_MANAGER_COMMANDS.quickPathJump, 'p', 'Quick path jump', { modifiers: ['ctrl'], category: NAV }),
  kb('pane.nav.navigateUp.backspace', FILE_MANAGER_COMMANDS.navigateUp, 'backspace', 'Go to parent directory', { category: NAV }),
  kb('pane.nav.refresh.ctrlR', FILE_MANAGER_COMMANDS.refresh, 'r', 'Refresh current pane', { modifiers: ['ctrl'], category: NAV }),
  kb('pane.nav.showKeyboardHelp.ctrlSlash', FILE_MANAGER_COMMANDS.showKeyboardHelp, '/', 'Show keyboard shortcuts', { modifiers: ['ctrl'], category: NAV }),
  kb('pane.nav.openSettings.ctrlShiftS', FILE_MANAGER_COMMANDS.openSettings, 's', 'Open settings', { modifiers: ['ctrl', 'shift'], category: NAV }),

  // ── Cursor movement ─────────────────────────────────────────────────────────
  kb('pane.cursor.up',       FILE_MANAGER_COMMANDS.cursorUp,        'arrowup',    'Move cursor up',              { category: NAV }),
  kb('pane.cursor.down',     FILE_MANAGER_COMMANDS.cursorDown,      'arrowdown',  'Move cursor down',            { category: NAV }),
  kb('pane.cursor.left',     FILE_MANAGER_COMMANDS.cursorLeft,      'arrowleft',  'Move cursor left',            { category: NAV }),
  kb('pane.cursor.right',    FILE_MANAGER_COMMANDS.cursorRight,     'arrowright', 'Move cursor right',           { category: NAV }),
  kb('pane.cursor.home',     FILE_MANAGER_COMMANDS.cursorHome,      'home',       'Move cursor to first item',   { category: NAV }),
  kb('pane.cursor.end',      FILE_MANAGER_COMMANDS.cursorEnd,       'end',        'Move cursor to last item',    { category: NAV }),
  kb('pane.cursor.pageUp',   FILE_MANAGER_COMMANDS.cursorPageUp,    'pageup',     'Move cursor up by page',      { category: NAV }),
  kb('pane.cursor.pageDown', FILE_MANAGER_COMMANDS.cursorPageDown,  'pagedown',   'Move cursor down by page',    { category: NAV }),

  // ── Tabs ─────────────────────────────────────────────────────────────────────
  kb('pane.tabs.new.ctrlT',          FILE_MANAGER_COMMANDS.newTab,       't',   'Open new tab',              { modifiers: ['ctrl'],          category: TABS }),
  kb('pane.tabs.close.ctrlW',        FILE_MANAGER_COMMANDS.closeTab,     'w',   'Close current tab',         { modifiers: ['ctrl'],          category: TABS }),
  kb('pane.tabs.next.ctrlTab',       FILE_MANAGER_COMMANDS.nextTab,      'tab', 'Switch to next tab',         { modifiers: ['ctrl'],          category: TABS }),
  kb('pane.tabs.prev.ctrlShiftTab',  FILE_MANAGER_COMMANDS.prevTab,      'tab', 'Switch to previous tab',    { modifiers: ['ctrl', 'shift'], category: TABS }),
  kb('pane.tabs.pin.ctrlAltP',       FILE_MANAGER_COMMANDS.togglePinTab, 'p',   'Pin/unpin current tab',     { modifiers: ['ctrl', 'alt'], category: TABS }),
  ...[1, 2, 3, 4, 5, 6, 7, 8, 9].map(i =>
    kb(`pane.tabs.jumpTo${i}.ctrl${i}`, FILE_MANAGER_COMMANDS.jumpToTab(i), i.toString(), `Jump to tab ${i}`, { modifiers: ['ctrl'], category: TABS }),
  ),

  // ── File operations ───────────────────────────────────────────────────────────
  kb('pane.files.open.enter',          FILE_MANAGER_COMMANDS.openItem,       'enter',    'Open file or folder',                          { category: FILES }),
  kb('pane.files.quickView.f3',        FILE_MANAGER_COMMANDS.quickView,      'f3',       'Quick view selected item (F3)',                 { category: FILES }),
  kb('pane.files.copyCursorPath.ctrlShiftEnter', FILE_MANAGER_COMMANDS.copyCursorPath, 'enter', 'Copy path of item under cursor',        { modifiers: ['ctrl', 'shift'], category: FILES }),
  kb('pane.files.copy.ctrlC',          FILE_MANAGER_COMMANDS.copy,           'c',        'Copy selected items',                          { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.cut.ctrlX',           FILE_MANAGER_COMMANDS.cut,            'x',        'Cut selected items',                           { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.paste.ctrlV',         FILE_MANAGER_COMMANDS.paste,          'v',        'Paste items',                                  { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.delete.delete',       FILE_MANAGER_COMMANDS.deleteSelected, 'delete',   'Delete selected items',                        { category: FILES }),
  kb('pane.files.delete.f8',           FILE_MANAGER_COMMANDS.deleteSelected, 'f8',       'Delete selected items (F8)',                   { category: FILES }),
  kb('pane.files.selectAll.ctrlA',     FILE_MANAGER_COMMANDS.selectAll,      'a',        'Select all items (Ctrl+A)',                    { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.deselectAll.ctrlD',   FILE_MANAGER_COMMANDS.deselectAll,    'd',        'Deselect all items (Ctrl+D)',                  { modifiers: ['ctrl'], category: FILES }),
  kb('pane.files.inlineDirectorySize.space', FILE_MANAGER_COMMANDS.inlineDirectorySize, 'space', 'Calculate current folder size inline, then mark item and move down', { category: FILES }),
  kb('pane.files.toggleCurrent.insert', FILE_MANAGER_COMMANDS.toggleCurrent, 'insert',   'Mark current directory or toggle file and move down', { category: FILES }),
  kb('pane.files.extendUp.shiftUp',    FILE_MANAGER_COMMANDS.extendSelUp,    'arrowup',  'Extend selection upward (Shift+ArrowUp)',      { modifiers: ['shift'], category: FILES }),
  kb('pane.files.extendDown.shiftDown',FILE_MANAGER_COMMANDS.extendSelDown,  'arrowdown','Extend selection downward (Shift+ArrowDown)',  { modifiers: ['shift'], category: FILES }),
  kb('pane.files.selectByMask.altS',   FILE_MANAGER_COMMANDS.selectByMask,   's',        'Select by mask (Alt+S)',                       { modifiers: ['alt'], category: FILES }),
  kb('pane.files.unselectByMask.altU', FILE_MANAGER_COMMANDS.unselectByMask, 'u',        'Unselect by mask (Alt+U)',                     { modifiers: ['alt'], category: FILES }),
  kb('pane.files.invertSel.altI',      FILE_MANAGER_COMMANDS.invertSelection,'i',        'Invert selection (Alt+I)',                     { modifiers: ['alt'], category: FILES }),
  kb('pane.files.selectSameExt.altE',  FILE_MANAGER_COMMANDS.selectSameExt,  'e',        'Select files with same extension (Alt+E)',     { modifiers: ['alt'], category: FILES }),
  kb('pane.files.newFolder.ctrlShiftN',FILE_MANAGER_COMMANDS.newFolder,      'n',        'Create new folder (Ctrl+Shift+N)',             { modifiers: ['ctrl', 'shift'], category: FILES }),
  kb('pane.files.newFolder.f7',        FILE_MANAGER_COMMANDS.newFolder,      'f7',       'Create folder (F7)',                           { category: FILES }),
  kb('pane.files.rename.f2',           FILE_MANAGER_COMMANDS.rename,         'f2',       'Rename selected item (F2)',                    { category: FILES }),
  kb('pane.files.rename.shiftF6',      FILE_MANAGER_COMMANDS.rename,         'f6',       'Rename selected item (Shift+F6)',              { modifiers: ['shift'], category: FILES }),
  kb('pane.files.copyToOther.f5',      FILE_MANAGER_COMMANDS.copyToOtherPane,'f5',       'Copy to other pane (F5)',                      { category: FILES }),
  kb('pane.files.moveToOther.f6',      FILE_MANAGER_COMMANDS.moveToOtherPane,'f6',       'Move to other pane (F6)',                      { category: FILES }),

  // ── Search & Filter ────────────────────────────────────────────────────────
  kb('pane.search.focusFilter.ctrlF',   FILE_MANAGER_COMMANDS.focusFilter,   'f', 'Focus filter input',      { modifiers: ['ctrl'],          category: SRCH }),
  kb('pane.search.advanced.ctrlShiftF', FILE_MANAGER_COMMANDS.advancedFilter,'f', 'Advanced filter options', { modifiers: ['ctrl', 'shift'], category: SRCH }),
  kb('pane.search.clearFilter.escape',  FILE_MANAGER_COMMANDS.clearFilter,   'escape', 'Clear filter',       { when: 'filterInput',          category: SRCH }),

  // ── Workspaces ────────────────────────────────────────────────────────────────
  kb('pane.ws.showSwitcher.ctrlShiftW', FILE_MANAGER_COMMANDS.showWorkspaceSwitcher, 'w', 'Open workspace switcher',  { modifiers: ['ctrl', 'shift'],       category: WS }),
  kb('pane.ws.new.ctrlShiftAltN',       FILE_MANAGER_COMMANDS.newWorkspace,          'n', 'Create new workspace',     { modifiers: ['ctrl', 'shift', 'alt'], category: WS }),
  kb('pane.ws.toggleHidden.ctrlShiftH', FILE_MANAGER_COMMANDS.toggleHidden,          'h', 'Show/hide hidden files',   { modifiers: ['ctrl', 'shift'],       category: WS }),

  // ── Pane control ──────────────────────────────────────────────────────────────
  kb('pane.ctrl.switch.tab',         FILE_MANAGER_COMMANDS.switchPane,     'tab', 'Switch between panes',           { category: PANE }),
  kb('pane.ctrl.copyToOther.ctrlShiftC', FILE_MANAGER_COMMANDS.copyToOtherPane, 'c', 'Copy selected to other pane',
    { modifiers: ['ctrl', 'shift'], when: '!typing && !quickAccess && activePane', category: PANE }),
  kb('pane.ctrl.moveToOther.ctrlShiftM', FILE_MANAGER_COMMANDS.moveToOtherPane, 'm', 'Move selected to other pane',
    { modifiers: ['ctrl', 'shift'], when: '!typing && !quickAccess && activePane', category: PANE }),
  kb('pane.ctrl.focusAddress.ctrlL', FILE_MANAGER_COMMANDS.focusAddressBar, 'l', 'Focus address bar (Ctrl+L)',
    { modifiers: ['ctrl'], stopPropagation: false, category: PANE }),
  kb('pane.ctrl.focusAddress.altD',  FILE_MANAGER_COMMANDS.focusAddressBar, 'd', 'Focus address bar (Alt+D)',
    { modifiers: ['alt'],  stopPropagation: false, category: PANE }),

  // ── Tools ─────────────────────────────────────────────────────────────────
  kb('pane.tools.diskUsage.ctrlShiftD', FILE_MANAGER_COMMANDS.diskUsage, 'd', 'Analyze disk usage',
    { modifiers: ['ctrl', 'shift'], category: FILES }),

  // ── Additional File Operations ────────────────────────────────────────────
  kb('pane.files.batchRename.ctrlShiftR', FILE_MANAGER_COMMANDS.batchRename, 'r', 'Batch rename selected items', { modifiers: ['ctrl', 'shift'], category: FILES }),
  kb('pane.files.openWithExternal.ctrlShiftO', FILE_MANAGER_COMMANDS.openWithExternal, 'o', 'Open with external application', { modifiers: ['ctrl', 'shift'], category: FILES }),
];
