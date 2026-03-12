import { Injectable } from '@angular/core';
import { KeyboardService } from './keyboard.service';

/**
 * Service to register all default keyboard shortcuts
 * Call registerAllShortcuts() during app initialization
 */
@Injectable({
  providedIn: 'root'
})
export class ShortcutRegistryService {
  constructor(private keyboardService: KeyboardService) {}

  registerAllShortcuts(callbacks: ShortcutCallbacks): void {
    this.registerNavigationShortcuts(callbacks);
    this.registerTabShortcuts(callbacks);
    this.registerFileOperationShortcuts(callbacks);
    this.registerSearchShortcuts(callbacks);
    this.registerWorkspaceShortcuts(callbacks);
    this.registerPaneShortcuts(callbacks);
  }

  private registerNavigationShortcuts(callbacks: ShortcutCallbacks): void {
    this.keyboardService.register({ id: 'global-search', key: 'k', modifiers: ['ctrl'], action: callbacks.openGlobalSearch, context: 'global', category: 'navigation', description: 'Open global search' });
    this.keyboardService.register({ id: 'global-search-chord', key: 'g', sequence: [{ key: 'g' }, { key: 's' }], action: callbacks.openGlobalSearch, context: 'global', category: 'navigation', description: 'Open global search (G then S)' });
    this.keyboardService.register({ id: 'path-history', key: 'h', modifiers: ['ctrl'], action: callbacks.showPathHistory, context: 'global', category: 'navigation', description: 'Show path history' });
    this.keyboardService.register({ id: 'path-history-chord', key: 'g', sequence: [{ key: 'g' }, { key: 'h' }], action: callbacks.showPathHistory, context: 'global', category: 'navigation', description: 'Show path history (G then H)' });
    this.keyboardService.register({ id: 'quick-jump', key: 'p', modifiers: ['ctrl'], action: callbacks.quickPathJump, context: 'global', category: 'navigation', description: 'Quick path jump' });
    this.keyboardService.register({ id: 'navigate-up', key: 'backspace', modifiers: [], action: callbacks.navigateUp, context: 'global', category: 'navigation', description: 'Go to parent directory' });
    this.keyboardService.register({ id: 'refresh', key: 'r', modifiers: ['ctrl'], action: callbacks.refresh, context: 'global', category: 'navigation', description: 'Refresh current pane' });
    this.keyboardService.register({ id: 'show-help', key: '/', modifiers: ['ctrl'], action: callbacks.showKeyboardHelp, context: 'global', category: 'navigation', description: 'Show keyboard shortcuts' });
    this.keyboardService.register({ id: 'cursor-up', key: 'arrowup', modifiers: [], action: callbacks.moveCursorUp, context: 'global', category: 'navigation', description: 'Move cursor up (ArrowUp)' });
    this.keyboardService.register({ id: 'cursor-down', key: 'arrowdown', modifiers: [], action: callbacks.moveCursorDown, context: 'global', category: 'navigation', description: 'Move cursor down (ArrowDown)' });
    this.keyboardService.register({ id: 'cursor-left', key: 'arrowleft', modifiers: [], action: callbacks.moveCursorLeft, context: 'global', category: 'navigation', description: 'Move cursor left / prev item (ArrowLeft)' });
    this.keyboardService.register({ id: 'cursor-right', key: 'arrowright', modifiers: [], action: callbacks.moveCursorRight, context: 'global', category: 'navigation', description: 'Move cursor right / next item (ArrowRight)' });
    this.keyboardService.register({ id: 'cursor-home', key: 'home', modifiers: [], action: callbacks.moveCursorHome, context: 'global', category: 'navigation', description: 'Move cursor to first item (Home)' });
    this.keyboardService.register({ id: 'cursor-end', key: 'end', modifiers: [], action: callbacks.moveCursorEnd, context: 'global', category: 'navigation', description: 'Move cursor to last item (End)' });
    this.keyboardService.register({ id: 'cursor-page-up', key: 'pageup', modifiers: [], action: callbacks.moveCursorPageUp, context: 'global', category: 'navigation', description: 'Move cursor up by page (PageUp)' });
    this.keyboardService.register({ id: 'cursor-page-down', key: 'pagedown', modifiers: [], action: callbacks.moveCursorPageDown, context: 'global', category: 'navigation', description: 'Move cursor down by page (PageDown)' });
  }

  private registerTabShortcuts(callbacks: ShortcutCallbacks): void {
    this.keyboardService.register({ id: 'new-tab', key: 't', modifiers: ['ctrl'], action: callbacks.newTab, context: 'global', category: 'tabs', description: 'Open new tab' });
    this.keyboardService.register({ id: 'close-tab', key: 'w', modifiers: ['ctrl'], action: callbacks.closeTab, context: 'global', category: 'tabs', description: 'Close current tab' });
    this.keyboardService.register({ id: 'next-tab', key: 'tab', modifiers: ['ctrl'], action: callbacks.nextTab, context: 'global', category: 'tabs', description: 'Switch to next tab' });
    this.keyboardService.register({ id: 'prev-tab', key: 'tab', modifiers: ['ctrl', 'shift'], action: callbacks.previousTab, context: 'global', category: 'tabs', description: 'Switch to previous tab' });
    for (let i = 1; i <= 9; i++) {
      this.keyboardService.register({ id: `jump-tab-${i}`, key: i.toString(), modifiers: ['ctrl'], action: () => callbacks.jumpToTab(i), context: 'global', category: 'tabs', description: `Jump to tab ${i}` });
    }
    this.keyboardService.register({ id: 'pin-tab', key: 'p', modifiers: ['ctrl', 'shift'], action: callbacks.togglePinTab, context: 'global', category: 'tabs', description: 'Pin/unpin current tab' });
  }

  private registerFileOperationShortcuts(callbacks: ShortcutCallbacks): void {
    // Open / navigate
    this.keyboardService.register({ id: 'open-item', key: 'enter', modifiers: [], action: callbacks.openItem, context: 'global', category: 'files', description: 'Open file or folder' });
    this.keyboardService.register({ id: 'quick-view', key: 'f3', modifiers: [], action: callbacks.quickView, context: 'global', category: 'files', description: 'Quick view selected item (F3)' });

    // Clipboard
    this.keyboardService.register({ id: 'copy', key: 'c', modifiers: ['ctrl'], action: callbacks.copySelected, context: 'global', category: 'files', description: 'Copy selected items' });
    this.keyboardService.register({ id: 'cut', key: 'x', modifiers: ['ctrl'], action: callbacks.cutSelected, context: 'global', category: 'files', description: 'Cut selected items' });
    this.keyboardService.register({ id: 'paste', key: 'v', modifiers: ['ctrl'], action: callbacks.paste, context: 'global', category: 'files', description: 'Paste items' });

    // Delete
    this.keyboardService.register({ id: 'delete', key: 'delete', modifiers: [], action: callbacks.deleteSelected, context: 'global', category: 'files', description: 'Delete selected items' });
    this.keyboardService.register({ id: 'tc-delete', key: 'f8', modifiers: [], action: callbacks.deleteSelected, context: 'global', category: 'files', description: 'Delete selected items (F8)' });

    // Select / deselect all  (Ctrl+A / Ctrl+D)
    this.keyboardService.register({ id: 'select-all', key: 'a', modifiers: ['ctrl'], action: callbacks.selectAll, context: 'global', category: 'files', description: 'Select all items (Ctrl+A)' });
    this.keyboardService.register({ id: 'deselect-all', key: 'd', modifiers: ['ctrl'], action: callbacks.deselectAll, context: 'global', category: 'files', description: 'Deselect all items (Ctrl+D)' });

    // Toggle current item and advance — Space (replaces Insert / Num+)
    this.keyboardService.register({ id: 'toggle-current', key: ' ', modifiers: [], action: callbacks.toggleCurrentAndMoveDown, context: 'global', category: 'files', description: 'Toggle mark on current item and move down (Space)' });

    // Cursor-mark with movement — Shift+Arrow (TC: marks item and moves)
    this.keyboardService.register({ id: 'extend-selection-up', key: 'arrowup', modifiers: ['shift'], action: callbacks.extendSelectionUp, context: 'global', category: 'files', description: 'Extend selection upward (Shift+ArrowUp)' });
    this.keyboardService.register({ id: 'extend-selection-down', key: 'arrowdown', modifiers: ['shift'], action: callbacks.extendSelectionDown, context: 'global', category: 'files', description: 'Extend selection downward (Shift+ArrowDown)' });

    // Mask / invert selection  (Alt+S / Alt+D / Alt+I)
    this.keyboardService.register({ id: 'select-by-mask', key: 's', modifiers: ['alt'], action: callbacks.selectByMask, context: 'global', category: 'files', description: 'Select by mask (Alt+S)' });
    this.keyboardService.register({ id: 'unselect-by-mask', key: 'd', modifiers: ['alt'], action: callbacks.unselectByMask, context: 'global', category: 'files', description: 'Unselect by mask (Alt+D)' });
    this.keyboardService.register({ id: 'invert-selection', key: 'i', modifiers: ['alt'], action: callbacks.invertSelection, context: 'global', category: 'files', description: 'Invert selection (Alt+I)' });
    this.keyboardService.register({ id: 'select-same-extension', key: 'e', modifiers: ['alt'], action: callbacks.selectSameExtension, context: 'global', category: 'files', description: 'Select files with same extension (Alt+E)' });

    // Folder / rename
    this.keyboardService.register({ id: 'new-folder', key: 'n', modifiers: ['ctrl', 'shift'], action: callbacks.newFolder, context: 'global', category: 'files', description: 'Create new folder' });
    this.keyboardService.register({ id: 'tc-new-folder', key: 'f7', modifiers: [], action: callbacks.newFolder, context: 'global', category: 'files', description: 'Create folder (F7)' });
    this.keyboardService.register({ id: 'rename', key: 'f2', modifiers: [], action: callbacks.renameSelected, context: 'global', category: 'files', description: 'Rename selected item (F2)' });
    this.keyboardService.register({ id: 'tc-rename', key: 'f6', modifiers: ['shift'], action: callbacks.renameSelected, context: 'global', category: 'files', description: 'Rename selected item (Shift+F6)' });

    // TC pane operations
    this.keyboardService.register({ id: 'tc-copy', key: 'f5', modifiers: [], action: callbacks.copyToOtherPane, context: 'global', category: 'files', description: 'Copy to other pane (F5)' });
    this.keyboardService.register({ id: 'tc-move', key: 'f6', modifiers: [], action: callbacks.moveToOtherPane, context: 'global', category: 'files', description: 'Move to other pane (F6)' });
  }

  private registerSearchShortcuts(callbacks: ShortcutCallbacks): void {
    this.keyboardService.register({ id: 'focus-filter', key: 'f', modifiers: ['ctrl'], action: callbacks.focusFilter, context: 'global', category: 'search', description: 'Focus filter input' });
    this.keyboardService.register({ id: 'advanced-filter', key: 'f', modifiers: ['ctrl', 'shift'], action: callbacks.advancedFilter, context: 'global', category: 'search', description: 'Advanced filter options' });
    this.keyboardService.register({ id: 'clear-filter', key: 'escape', modifiers: [], action: callbacks.clearFilter, context: 'filter', category: 'search', description: 'Clear filter' });
  }

  private registerWorkspaceShortcuts(callbacks: ShortcutCallbacks): void {
    this.keyboardService.register({ id: 'workspace-switcher', key: 'w', modifiers: ['ctrl', 'shift'], action: callbacks.showWorkspaceSwitcher, context: 'global', category: 'workspace', description: 'Open workspace switcher' });
    this.keyboardService.register({ id: 'new-workspace', key: 'n', modifiers: ['ctrl', 'shift', 'alt'], action: callbacks.newWorkspace, context: 'global', category: 'workspace', description: 'Create new workspace' });
    this.keyboardService.register({ id: 'toggle-hidden', key: 'h', modifiers: ['ctrl', 'shift'], action: callbacks.toggleHiddenFiles, context: 'global', category: 'workspace', description: 'Show/hide hidden files' });
  }

  private registerPaneShortcuts(callbacks: ShortcutCallbacks): void {
    this.keyboardService.register({ id: 'switch-pane', key: 'tab', modifiers: [], action: callbacks.switchPane, context: 'global', category: 'pane', description: 'Switch between panes' });
    this.keyboardService.register({ id: 'copy-to-pane', key: 'c', modifiers: ['ctrl', 'shift'], action: callbacks.copyToOtherPane, context: 'pane', category: 'pane', description: 'Copy selected to other pane' });
    this.keyboardService.register({ id: 'move-to-pane', key: 'm', modifiers: ['ctrl', 'shift'], action: callbacks.moveToOtherPane, context: 'pane', category: 'pane', description: 'Move selected to other pane' });
    this.keyboardService.register({ id: 'focus-address', key: 'l', modifiers: ['ctrl'], action: callbacks.focusAddressBar, context: 'global', category: 'pane', description: 'Focus address bar (Ctrl+L)', allowBubble: true });
    this.keyboardService.register({ id: 'focus-address-alt-d', key: 'd', modifiers: ['alt'], action: callbacks.focusAddressBar, context: 'global', category: 'pane', description: 'Focus address bar (Alt+D)', allowBubble: true });
  }
}

export interface ShortcutCallbacks {
  // Navigation
  openGlobalSearch: () => void;
  showPathHistory: () => void;
  quickPathJump: () => void;
  navigateUp: () => void;
  refresh: () => void;
  showKeyboardHelp: () => void;

  // Tabs
  newTab: () => void;
  closeTab: () => void;
  nextTab: () => void;
  previousTab: () => void;
  jumpToTab: (index: number) => void;
  togglePinTab: () => void;

  // File operations
  quickView: () => void;
  openItem: () => void;
  deleteSelected: () => void;
  copySelected: () => void;
  cutSelected: () => void;
  paste: () => void;
  selectAll: () => void;
  deselectAll: () => void;
  newFolder: () => void;
  renameSelected: () => void;
  toggleCurrentAndMoveDown: () => void;
  extendSelectionUp: () => void;
  extendSelectionDown: () => void;
  selectByMask: () => void;
  unselectByMask: () => void;
  invertSelection: () => void;
  selectSameExtension: () => void;

  // Search
  focusFilter: () => void;
  advancedFilter: () => void;
  clearFilter: () => void;

  // Workspace
  showWorkspaceSwitcher: () => void;
  newWorkspace: () => void;
  toggleHiddenFiles: () => void;

  // Pane
  switchPane: () => void;
  copyToOtherPane: () => void;
  moveToOtherPane: () => void;
  focusAddressBar: () => void;

  // Cursor movement
  moveCursorUp: () => void;
  moveCursorDown: () => void;
  moveCursorLeft: () => void;
  moveCursorRight: () => void;
  moveCursorHome: () => void;
  moveCursorEnd: () => void;
  moveCursorPageUp: () => void;
  moveCursorPageDown: () => void;
  markCurrentItem: () => void;
}