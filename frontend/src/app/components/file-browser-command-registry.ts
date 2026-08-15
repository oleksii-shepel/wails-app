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

import type { InputBindingDefinition, CommandDefinition, CommandDescriptor, InputCommandHandler } from '../services/input-system.types';
import type { InputEngineService } from '../services/input-engine.service';

// ─── Command IDs ──────────────────────────────────────────────────────────────

export const BC = {
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
} as const;

const NAV  = 'Navigation';
const TABS = 'Tab Management';
const FILES = 'File Operations';
const SRCH = 'Search & Filter';
const WS   = 'Workspaces';
const PANE = 'Pane Control';
const TOOLS = 'Tools';

// ─── Command definitions ──────────────────────────────────────────────────────

const CMD_DEF = (
  id: string,
  title: string,
  category: string,
  description?: string,
  name?: string,
  aliases?: string[],
): CommandDefinition => ({
  id, title, category, description, name, aliases,
});

const BROWSER_COMMAND_DEFINITIONS: CommandDefinition[] = [
  CMD_DEF(BC.openCommandPalette,   'Open Command Palette',      NAV,  'Open the command palette', 'command-palette', ['commands']),
  CMD_DEF(BC.openGlobalSearch,     'Open Global Search',        NAV,  'Search files across all indexed locations', 'search', ['find']),
  CMD_DEF(BC.showPathHistory,      'Show Path History',         NAV,  'Browse recently visited paths', 'history', undefined),
  CMD_DEF(BC.quickPathJump,        'Quick Path Jump',           NAV,  'Jump to a path by typing', 'jump', ['go']),
  CMD_DEF(BC.navigateUp,           'Navigate Up',               NAV,  'Go to parent directory', 'up', ['parent']),
  CMD_DEF(BC.refresh,              'Refresh',                   NAV,  'Refresh current pane', 'refresh', ['reload']),
  CMD_DEF(BC.showKeyboardHelp,     'Show Keyboard Help',        NAV,  'Show keyboard shortcuts reference', 'shortcuts', ['keys']),
  CMD_DEF(BC.openSettings,         'Open Settings',             NAV,  'Open application settings', 'settings', undefined),
  CMD_DEF(BC.openOperations,       'Open Operations Center',    TOOLS, 'Show active and recent file operations', 'operations', ['activity', 'jobs']),
  CMD_DEF(BC.openSelectionBasket,  'Open Selection Basket',     TOOLS, 'Review the cross-surface working set', 'basket', ['collection', 'working-set']),
  CMD_DEF(BC.addToSelectionBasket, 'Add to Selection Basket',   FILES, 'Add selected or explicitly supplied paths to the working set', 'basket.add', ['collect', 'stash-selection']),
  CMD_DEF(BC.clearSelectionBasket, 'Clear Selection Basket',    TOOLS, 'Remove every item from the working set', 'basket.clear', ['clear-basket']),
  CMD_DEF(BC.cursorUp,             'Move Cursor Up',            NAV,  'Move cursor up one item', 'cursor.up', undefined),
  CMD_DEF(BC.cursorDown,           'Move Cursor Down',          NAV,  'Move cursor down one item', 'cursor.down', undefined),
  CMD_DEF(BC.cursorLeft,           'Move Cursor Left',          NAV,  'Move cursor left', 'cursor.left', undefined),
  CMD_DEF(BC.cursorRight,          'Move Cursor Right',         NAV,  'Move cursor right', 'cursor.right', undefined),
  CMD_DEF(BC.cursorHome,           'Move Cursor to First',      NAV,  'Move cursor to first item', 'cursor.first', undefined),
  CMD_DEF(BC.cursorEnd,            'Move Cursor to Last',       NAV,  'Move cursor to last item', 'cursor.last', undefined),
  CMD_DEF(BC.cursorPageUp,         'Page Up',                   NAV,  'Move cursor up by page', 'cursor.page-up', undefined),
  CMD_DEF(BC.cursorPageDown,       'Page Down',                 NAV,  'Move cursor down by page', 'cursor.page-down', undefined),
  CMD_DEF(BC.newTab,               'New Tab',                   TABS, 'Open a new tab', 'tab.new', undefined),
  CMD_DEF(BC.closeTab,             'Close Tab',                 TABS, 'Close current tab', 'tab.close', undefined),
  CMD_DEF(BC.nextTab,              'Next Tab',                  TABS, 'Switch to next tab', 'tab.next', undefined),
  CMD_DEF(BC.prevTab,              'Previous Tab',              TABS, 'Switch to previous tab', 'tab.prev', undefined),
  CMD_DEF(BC.togglePinTab,         'Pin/Unpin Tab',             TABS, 'Pin or unpin current tab', 'tab.pin', undefined),
  CMD_DEF(BC.openItem,             'Open Item',                 FILES, 'Open file or folder', 'open', undefined),
  CMD_DEF(BC.quickView,            'Quick View',                FILES, 'Open quick view', 'quick-view', ['preview']),
  CMD_DEF(BC.copyCursorPath,       'Copy Cursor Path',          FILES, 'Copy path of item under cursor', 'copy-path', undefined),
  CMD_DEF(BC.copy,                 'Copy',                      FILES, 'Copy selected items', 'copy', undefined),
  CMD_DEF(BC.cut,                  'Cut',                       FILES, 'Cut selected items', 'cut', undefined),
  CMD_DEF(BC.paste,                'Paste',                     FILES, 'Paste items', 'paste', undefined),
  CMD_DEF(BC.deleteSelected,       'Delete',                    FILES, 'Delete selected items', 'delete', ['remove']),
  CMD_DEF(BC.selectAll,            'Select All',                FILES, 'Select all items', 'selection.all', undefined),
  CMD_DEF(BC.deselectAll,          'Deselect All',              FILES, 'Deselect all items', 'selection.none', undefined),
  CMD_DEF(BC.newFolder,            'New Folder',                FILES, 'Create new folder', 'mkdir', ['new-folder']),
  CMD_DEF(BC.rename,               'Rename',                    FILES, 'Rename selected item', 'rename', undefined),
  CMD_DEF(BC.batchRename,          'Batch Rename',              FILES, 'Batch rename selected items', 'rename.batch', ['batch-rename']),
  CMD_DEF(BC.toggleCurrent,        'Toggle Selection',          FILES, 'Toggle selection of current item', 'selection.toggle', undefined),
  CMD_DEF(BC.inlineDirectorySize,  'Calculate Folder Size',     FILES, 'Calculate current folder size inline', 'size.calculate', ['folder-size']),
  CMD_DEF(BC.extendSelUp,          'Extend Selection Up',       FILES, 'Extend selection upward', 'selection.extend-up', undefined),
  CMD_DEF(BC.extendSelDown,        'Extend Selection Down',     FILES, 'Extend selection downward', 'selection.extend-down', undefined),
  CMD_DEF(BC.selectByMask,         'Select by Mask',            FILES, 'Select files matching mask', 'selection.mask', undefined),
  CMD_DEF(BC.unselectByMask,       'Unselect by Mask',          FILES, 'Unselect files matching mask', 'selection.unmask', undefined),
  CMD_DEF(BC.invertSelection,      'Invert Selection',          FILES, 'Invert current selection', 'selection.invert', undefined),
  CMD_DEF(BC.selectSameExt,        'Select Same Extension',     FILES, 'Select files with same extension', 'selection.same-extension', undefined),
  CMD_DEF(BC.focusFilter,          'Focus Filter',              SRCH, 'Focus filter input', 'filter.focus', undefined),
  CMD_DEF(BC.advancedFilter,       'Advanced Filter',           SRCH, 'Open advanced filter options', 'filter.advanced', undefined),
  CMD_DEF(BC.clearFilter,          'Clear Filter',              SRCH, 'Clear active filter', 'filter.clear', undefined),
  CMD_DEF(BC.showWorkspaceSwitcher,'Show Workspace Switcher',   WS,   'Open workspace switcher', 'workspace.switcher', undefined),
  CMD_DEF(BC.newWorkspace,         'New Workspace',             WS,   'Create new workspace', 'workspace.new', undefined),
  CMD_DEF(BC.toggleHidden,         'Toggle Hidden Files',       WS,   'Show or hide hidden files', 'workspace.toggle-hidden', ['hidden']),
  CMD_DEF(BC.switchPane,           'Switch Pane',               PANE, 'Switch between panes', 'pane.switch', undefined),
  CMD_DEF(BC.copyToOtherPane,      'Copy to Other Pane',        PANE, 'Copy selected to other pane', 'pane.copy-to-other', ['copy-to-other']),
  CMD_DEF(BC.moveToOtherPane,      'Move to Other Pane',        PANE, 'Move selected to other pane', 'pane.move-to-other', ['move-to-other']),
  CMD_DEF(BC.focusAddressBar,      'Focus Address Bar',         PANE, 'Focus address bar', 'pane.address', ['address']),
  CMD_DEF(BC.comparePanes,          'Compare Panes',               PANE, 'Compare the two folder panes', 'pane.compare', ['compare']),
  CMD_DEF(BC.recursiveComparePanes, 'Recursive Pane Compare',     PANE, 'Compare complete directory trees', 'pane.compare.recursive', ['compare-tree']),
  CMD_DEF(BC.reviewSyncPlan,         'Review Sync Plan',             PANE, 'Review and approve pane synchronization decisions', 'pane.compare.review', ['sync-review']),
  CMD_DEF(BC.selectLeftDifferences, 'Select Left Differences',     PANE, 'Select differing items in the left pane', 'pane.compare.select-left', undefined),
  CMD_DEF(BC.selectRightDifferences,'Select Right Differences',    PANE, 'Select differing items in the right pane', 'pane.compare.select-right', undefined),
  CMD_DEF(BC.copyMissingLeftToRight,'Copy Missing Left to Right',  PANE, 'Copy items that exist only on the left pane', 'pane.compare.missing-right', undefined),
  CMD_DEF(BC.copyMissingRightToLeft,'Copy Missing Right to Left',  PANE, 'Copy items that exist only on the right pane', 'pane.compare.missing-left', undefined),
  CMD_DEF(BC.diskUsage,            'Disk Usage',                TOOLS, 'Analyze disk usage', 'disk-usage', ['du']),
  CMD_DEF(BC.findDuplicates,       'Find Duplicate Files',      TOOLS, 'Find content-identical files recursively', 'duplicates', ['find-duplicates']),
  CMD_DEF(BC.openWithExternal,     'Open with External App',    FILES, 'Open selected file with external application', 'open-external', undefined),
];

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

// ─── Binding catalog ──────────────────────────────────────────────────────────

export const BROWSER_BINDINGS: InputBindingDefinition[] = [

  // ── Navigation ─────────────────────────────────────────────────────────────
  kb('pane.nav.openCommandPalette.ctrlShiftP', BC.openCommandPalette, 'p', 'Open command palette', { modifiers: ['ctrl', 'shift'], when: '!quickAccess', category: NAV }),
  kb('pane.nav.openCommandPalette.f1', BC.openCommandPalette, 'f1', 'Open command palette (F1)', { when: '!quickAccess', category: NAV }),
  kb('pane.nav.openGlobalSearch.ctrlK', BC.openGlobalSearch, 'k', 'Open global search', { modifiers: ['ctrl'], category: NAV }),
  chord('pane.nav.openGlobalSearch.gs', BC.openGlobalSearch, { key: 'g' }, { key: 's' }, 'Open global search (G then S)', NAV),
  kb('pane.nav.showPathHistory.ctrlH', BC.showPathHistory, 'h', 'Show path history', { modifiers: ['ctrl'], category: NAV }),
  chord('pane.nav.showPathHistory.gh', BC.showPathHistory, { key: 'g' }, { key: 'h' }, 'Show path history (G then H)', NAV),
  kb('pane.nav.quickPathJump.ctrlP', BC.quickPathJump, 'p', 'Quick path jump', { modifiers: ['ctrl'], category: NAV }),
  kb('pane.nav.navigateUp.backspace', BC.navigateUp, 'backspace', 'Go to parent directory', { category: NAV }),
  kb('pane.nav.refresh.ctrlR', BC.refresh, 'r', 'Refresh current pane', { modifiers: ['ctrl'], category: NAV }),
  kb('pane.nav.showKeyboardHelp.ctrlSlash', BC.showKeyboardHelp, '/', 'Show keyboard shortcuts', { modifiers: ['ctrl'], category: NAV }),
  kb('pane.nav.openSettings.ctrlShiftS', BC.openSettings, 's', 'Open settings', { modifiers: ['ctrl', 'shift'], category: NAV }),

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
  kb('pane.tabs.pin.ctrlAltP',       BC.togglePinTab, 'p',   'Pin/unpin current tab',     { modifiers: ['ctrl', 'alt'], category: TABS }),
  ...[1, 2, 3, 4, 5, 6, 7, 8, 9].map(i =>
    kb(`pane.tabs.jumpTo${i}.ctrl${i}`, BC.jumpToTab(i), i.toString(), `Jump to tab ${i}`, { modifiers: ['ctrl'], category: TABS }),
  ),

  // ── File operations ───────────────────────────────────────────────────────────
  kb('pane.files.open.enter',          BC.openItem,       'enter',    'Open file or folder',                          { category: FILES }),
  kb('pane.files.quickView.f3',        BC.quickView,      'f3',       'Quick view selected item (F3)',                 { category: FILES }),
  kb('pane.files.copyCursorPath.ctrlShiftEnter', BC.copyCursorPath, 'enter', 'Copy path of item under cursor',        { modifiers: ['ctrl', 'shift'], category: FILES }),
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

  // ── Additional File Operations ────────────────────────────────────────────
  kb('pane.files.batchRename.ctrlShiftR', BC.batchRename, 'r', 'Batch rename selected items', { modifiers: ['ctrl', 'shift'], category: FILES }),
  kb('pane.files.openWithExternal.ctrlShiftO', BC.openWithExternal, 'o', 'Open with external application', { modifiers: ['ctrl', 'shift'], category: FILES }),
];

// ─── Handler interface ────────────────────────────────────────────────────────

export interface FileBrowserCommandHandlers {
  openCommandPalette:   () => void;
  openGlobalSearch:     () => void;
  showPathHistory:      () => void;
  quickPathJump:        () => void;
  navigateUp:           () => void;
  refresh:              () => void;
  showKeyboardHelp:     () => void;
  openSettings:         () => void;
  openOperations:       () => void;
  openSelectionBasket:  () => void;
  addToSelectionBasket: InputCommandHandler;
  clearSelectionBasket: () => void;
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
  copyCursorPath:       () => void;
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
  comparePanes:          () => void;
  recursiveComparePanes: () => void;
  reviewSyncPlan:         () => void;
  selectLeftDifferences: () => void;
  selectRightDifferences:() => void;
  copyMissingLeftToRight:() => void;
  copyMissingRightToLeft:() => void;
  diskUsage:            () => void;
  findDuplicates:       () => void;
  batchRename:          () => void;
  openWithExternal:     () => void;
}

// ─── Registration ─────────────────────────────────────────────────────────────

/**
 * Registers all file-browser keybindings and command handlers with the input
 * engine.  Returns a cleanup function that unregisters everything — call it
 * in ngOnDestroy.
 */
export function createFileBrowserCommandDescriptors(
  handlers: FileBrowserCommandHandlers,
  isEnabled?: (commandId: string) => boolean,
): CommandDescriptor[] {
  const directHandlers = new Map<string, InputCommandHandler>([
    [BC.openCommandPalette, handlers.openCommandPalette],
    [BC.openGlobalSearch, handlers.openGlobalSearch],
    [BC.showPathHistory, handlers.showPathHistory],
    [BC.quickPathJump, handlers.quickPathJump],
    [BC.navigateUp, handlers.navigateUp],
    [BC.refresh, handlers.refresh],
    [BC.showKeyboardHelp, handlers.showKeyboardHelp],
    [BC.openSettings, handlers.openSettings],
    [BC.openOperations, handlers.openOperations],
    [BC.openSelectionBasket, handlers.openSelectionBasket],
    [BC.addToSelectionBasket, handlers.addToSelectionBasket],
    [BC.clearSelectionBasket, handlers.clearSelectionBasket],
    [BC.cursorUp, handlers.cursorUp],
    [BC.cursorDown, handlers.cursorDown],
    [BC.cursorLeft, handlers.cursorLeft],
    [BC.cursorRight, handlers.cursorRight],
    [BC.cursorHome, handlers.cursorHome],
    [BC.cursorEnd, handlers.cursorEnd],
    [BC.cursorPageUp, handlers.cursorPageUp],
    [BC.cursorPageDown, handlers.cursorPageDown],
    [BC.newTab, handlers.newTab],
    [BC.closeTab, handlers.closeTab],
    [BC.nextTab, handlers.nextTab],
    [BC.prevTab, handlers.prevTab],
    [BC.togglePinTab, handlers.togglePinTab],
    [BC.openItem, handlers.openItem],
    [BC.quickView, handlers.quickView],
    [BC.copyCursorPath, handlers.copyCursorPath],
    [BC.copy, handlers.copy],
    [BC.cut, handlers.cut],
    [BC.paste, handlers.paste],
    [BC.deleteSelected, handlers.deleteSelected],
    [BC.selectAll, handlers.selectAll],
    [BC.deselectAll, handlers.deselectAll],
    [BC.newFolder, handlers.newFolder],
    [BC.rename, handlers.rename],
    [BC.toggleCurrent, handlers.toggleCurrent],
    [BC.inlineDirectorySize, handlers.inlineDirectorySize],
    [BC.extendSelUp, handlers.extendSelUp],
    [BC.extendSelDown, handlers.extendSelDown],
    [BC.selectByMask, handlers.selectByMask],
    [BC.unselectByMask, handlers.unselectByMask],
    [BC.invertSelection, handlers.invertSelection],
    [BC.selectSameExt, handlers.selectSameExt],
    [BC.focusFilter, handlers.focusFilter],
    [BC.advancedFilter, handlers.advancedFilter],
    [BC.clearFilter, handlers.clearFilter],
    [BC.showWorkspaceSwitcher, handlers.showWorkspaceSwitcher],
    [BC.newWorkspace, handlers.newWorkspace],
    [BC.toggleHidden, handlers.toggleHidden],
    [BC.switchPane, handlers.switchPane],
    [BC.copyToOtherPane, handlers.copyToOtherPane],
    [BC.moveToOtherPane, handlers.moveToOtherPane],
    [BC.focusAddressBar, handlers.focusAddressBar],
    [BC.comparePanes, handlers.comparePanes],
    [BC.recursiveComparePanes, handlers.recursiveComparePanes],
    [BC.reviewSyncPlan, handlers.reviewSyncPlan],
    [BC.selectLeftDifferences, handlers.selectLeftDifferences],
    [BC.selectRightDifferences, handlers.selectRightDifferences],
    [BC.copyMissingLeftToRight, handlers.copyMissingLeftToRight],
    [BC.copyMissingRightToLeft, handlers.copyMissingRightToLeft],
    [BC.diskUsage, handlers.diskUsage],
    [BC.findDuplicates, handlers.findDuplicates],
    [BC.batchRename, handlers.batchRename],
    [BC.openWithExternal, handlers.openWithExternal],
  ]);

  for (let i = 1; i <= 9; i++) {
    const index = i;
    directHandlers.set(BC.jumpToTab(i), () => handlers.jumpToTab(index));
  }

  const descriptors = BROWSER_COMMAND_DEFINITIONS.map(definition => {
    const execute = directHandlers.get(definition.id);
    if (!execute) {
      throw new Error(`Missing file-browser command handler: ${definition.id}`);
    }
    return {
      ...definition,
      execute,
      isEnabled: isEnabled ? () => isEnabled(definition.id) : undefined,
    };
  });

  for (let i = 1; i <= 9; i++) {
    const id = BC.jumpToTab(i);
    const execute = directHandlers.get(id)!;
    descriptors.push({
      id,
      title: `Jump to Tab ${i}`,
      category: TABS,
      description: `Switch to tab ${i}`,
      name: `tab.${i}`,
      execute,
      isEnabled: isEnabled ? () => isEnabled(id) : undefined,
    });
  }

  return descriptors;
}

export function registerFileBrowserCommands(
  engine: InputEngineService,
  handlers: FileBrowserCommandHandlers,
  ownerId: string,
  isEnabled?: (commandId: string) => boolean,
): () => void {
  const descriptors = createFileBrowserCommandDescriptors(handlers, isEnabled);

  for (const descriptor of descriptors) {
    engine.registerCommandDescriptor(descriptor, ownerId);
  }

  for (const binding of BROWSER_BINDINGS) {
    engine.registerBinding(binding);
  }

  return () => {
    for (const binding of BROWSER_BINDINGS) {
      engine.unregisterBinding(binding.id);
    }
    for (const descriptor of descriptors) {
      engine.unregisterCommandDescriptor(descriptor.id, ownerId);
    }
  };
}
