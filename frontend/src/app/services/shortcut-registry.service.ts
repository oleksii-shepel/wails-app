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

  /**
   * Register all default shortcuts
   * Call this in app initialization
   */
  registerAllShortcuts(callbacks: ShortcutCallbacks): void {
    this.registerNavigationShortcuts(callbacks);
    this.registerTabShortcuts(callbacks);
    this.registerFileOperationShortcuts(callbacks);
    this.registerSearchShortcuts(callbacks);
    this.registerWorkspaceShortcuts(callbacks);
    this.registerPaneShortcuts(callbacks);
  }

  /**
   * Navigation shortcuts
   */
  private registerNavigationShortcuts(callbacks: ShortcutCallbacks): void {
    // Global search
    this.keyboardService.register({
      id: 'global-search',
      key: 'k',
      modifiers: ['ctrl'],
      action: callbacks.openGlobalSearch,
      context: 'global',
      category: 'navigation',
      description: 'Open global search',
    });

    // Path history
    this.keyboardService.register({
      id: 'path-history',
      key: 'h',
      modifiers: ['ctrl'],
      action: callbacks.showPathHistory,
      context: 'global',
      category: 'navigation',
      description: 'Show path history',
    });

    // Quick path jump
    this.keyboardService.register({
      id: 'quick-jump',
      key: 'p',
      modifiers: ['ctrl'],
      action: callbacks.quickPathJump,
      context: 'global',
      category: 'navigation',
      description: 'Quick path jump',
    });

    // Navigate up (parent directory)
    this.keyboardService.register({
      id: 'navigate-up',
      key: 'backspace',
      modifiers: [],
      action: callbacks.navigateUp,
      context: 'global',
      category: 'navigation',
      description: 'Go to parent directory',
    });

    // Refresh current pane
    this.keyboardService.register({
      id: 'refresh',
      key: 'r',
      modifiers: ['ctrl'],
      action: callbacks.refresh,
      context: 'global',
      category: 'navigation',
      description: 'Refresh current pane',
    });

    // Show keyboard help
    this.keyboardService.register({
      id: 'show-help',
      key: '/',
      modifiers: ['ctrl'],
      action: callbacks.showKeyboardHelp,
      context: 'global',
      category: 'navigation',
      description: 'Show keyboard shortcuts',
    });
  }

  /**
   * Tab management shortcuts
   */
  private registerTabShortcuts(callbacks: ShortcutCallbacks): void {
    // New tab
    this.keyboardService.register({
      id: 'new-tab',
      key: 't',
      modifiers: ['ctrl'],
      action: callbacks.newTab,
      context: 'global',
      category: 'tabs',
      description: 'Open new tab',
    });

    // Close tab
    this.keyboardService.register({
      id: 'close-tab',
      key: 'w',
      modifiers: ['ctrl'],
      action: callbacks.closeTab,
      context: 'global',
      category: 'tabs',
      description: 'Close current tab',
    });

    // Next tab
    this.keyboardService.register({
      id: 'next-tab',
      key: 'tab',
      modifiers: ['ctrl'],
      action: callbacks.nextTab,
      context: 'global',
      category: 'tabs',
      description: 'Switch to next tab',
    });

    // Previous tab
    this.keyboardService.register({
      id: 'prev-tab',
      key: 'tab',
      modifiers: ['ctrl', 'shift'],
      action: callbacks.previousTab,
      context: 'global',
      category: 'tabs',
      description: 'Switch to previous tab',
    });

    // Jump to tab 1-9
    for (let i = 1; i <= 9; i++) {
      this.keyboardService.register({
        id: `jump-tab-${i}`,
        key: i.toString(),
        modifiers: ['ctrl'],
        action: () => callbacks.jumpToTab(i),
        context: 'global',
        category: 'tabs',
        description: `Jump to tab ${i}`,
      });
    }

    // Pin/unpin tab
    this.keyboardService.register({
      id: 'pin-tab',
      key: 'p',
      modifiers: ['ctrl', 'shift'],
      action: callbacks.togglePinTab,
      context: 'global',
      category: 'tabs',
      description: 'Pin/unpin current tab',
    });
  }

  /**
   * File operation shortcuts
   */
  private registerFileOperationShortcuts(callbacks: ShortcutCallbacks): void {
    // Quick view (preview)
    this.keyboardService.register({
      id: 'quick-view',
      key: 'f3',
      modifiers: [],
      action: callbacks.quickView,
      context: 'global',
      category: 'files',
      description: 'Quick view selected item (F3)',
    });

    // Open file/folder
    this.keyboardService.register({
      id: 'open-item',
      key: 'enter',
      modifiers: [],
      action: callbacks.openItem,
      context: 'global',
      category: 'files',
      description: 'Open file or folder',
    });

    // Delete selected
    this.keyboardService.register({
      id: 'delete',
      key: 'delete',
      modifiers: [],
      action: callbacks.deleteSelected,
      context: 'global',
      category: 'files',
      description: 'Delete selected items',
    });

    // Copy
    this.keyboardService.register({
      id: 'copy',
      key: 'c',
      modifiers: ['ctrl'],
      action: callbacks.copySelected,
      context: 'global',
      category: 'files',
      description: 'Copy selected items',
    });

    // Cut
    this.keyboardService.register({
      id: 'cut',
      key: 'x',
      modifiers: ['ctrl'],
      action: callbacks.cutSelected,
      context: 'global',
      category: 'files',
      description: 'Cut selected items',
    });

    // Paste
    this.keyboardService.register({
      id: 'paste',
      key: 'v',
      modifiers: ['ctrl'],
      action: callbacks.paste,
      context: 'global',
      category: 'files',
      description: 'Paste items',
    });

    // Select all
    this.keyboardService.register({
      id: 'select-all',
      key: 'a',
      modifiers: ['ctrl'],
      action: callbacks.selectAll,
      context: 'global',
      category: 'files',
      description: 'Select all items',
    });

    this.keyboardService.register({
      id: 'select-all-ctrl-num-plus',
      key: 'numpadadd',
      modifiers: ['ctrl'],
      action: callbacks.selectAll,
      context: 'global',
      category: 'files',
      description: 'Select all items (Ctrl+Num +)',
    });

    this.keyboardService.register({
      id: 'select-all-shift-num-plus',
      key: 'numpadadd',
      modifiers: ['shift'],
      action: callbacks.selectAll,
      context: 'global',
      category: 'files',
      description: 'Select all items (Shift+Num +)',
    });

    // Deselect all
    this.keyboardService.register({
      id: 'deselect-all',
      key: 'd',
      modifiers: ['ctrl'],
      action: callbacks.deselectAll,
      context: 'global',
      category: 'files',
      description: 'Deselect all items',
    });

    this.keyboardService.register({
      id: 'deselect-all-ctrl-num-minus',
      key: 'numpadsubtract',
      modifiers: ['ctrl'],
      action: callbacks.deselectAll,
      context: 'global',
      category: 'files',
      description: 'Deselect all items (Ctrl+Num -)',
    });

    this.keyboardService.register({
      id: 'deselect-all-shift-num-minus',
      key: 'numpadsubtract',
      modifiers: ['shift'],
      action: callbacks.deselectAll,
      context: 'global',
      category: 'files',
      description: 'Deselect all items (Shift+Num -)',
    });

    this.keyboardService.register({
      id: 'toggle-current-insert',
      key: 'insert',
      modifiers: [],
      action: callbacks.toggleCurrentAndMoveDown,
      context: 'global',
      category: 'files',
      description: 'Toggle current item and move down (Insert)',
    });

    this.keyboardService.register({
      id: 'select-current-space',
      key: 'space',
      modifiers: [],
      action: callbacks.selectCurrentAndMoveDown,
      context: 'global',
      category: 'files',
      description: 'Select current item and move down (Space)',
    });

    this.keyboardService.register({
      id: 'extend-selection-up',
      key: 'arrowup',
      modifiers: ['shift'],
      action: callbacks.extendSelectionUp,
      context: 'global',
      category: 'files',
      description: 'Extend selection upward (Shift+ArrowUp)',
    });

    this.keyboardService.register({
      id: 'extend-selection-down',
      key: 'arrowdown',
      modifiers: ['shift'],
      action: callbacks.extendSelectionDown,
      context: 'global',
      category: 'files',
      description: 'Extend selection downward (Shift+ArrowDown)',
    });

    this.keyboardService.register({
      id: 'select-by-mask',
      key: 'numpadadd',
      modifiers: [],
      action: callbacks.selectByMask,
      context: 'global',
      category: 'files',
      description: 'Select by mask (Num +)',
    });

    this.keyboardService.register({
      id: 'unselect-by-mask',
      key: 'numpadsubtract',
      modifiers: [],
      action: callbacks.unselectByMask,
      context: 'global',
      category: 'files',
      description: 'Unselect by mask (Num -)',
    });

    this.keyboardService.register({
      id: 'invert-selection',
      key: 'numpadmultiply',
      modifiers: [],
      action: callbacks.invertSelection,
      context: 'global',
      category: 'files',
      description: 'Invert selection (Num *)',
    });

    this.keyboardService.register({
      id: 'select-same-extension',
      key: 'numpadadd',
      modifiers: ['alt'],
      action: callbacks.selectSameExtension,
      context: 'global',
      category: 'files',
      description: 'Select files with same extension (Alt+Num +)',
    });

    // New folder
    this.keyboardService.register({
      id: 'new-folder',
      key: 'n',
      modifiers: ['ctrl', 'shift'],
      action: callbacks.newFolder,
      context: 'global',
      category: 'files',
      description: 'Create new folder',
    });

    // Rename
    this.keyboardService.register({
      id: 'rename',
      key: 'f2',
      modifiers: [],
      action: callbacks.renameSelected,
      context: 'global',
      category: 'files',
      description: 'Rename selected item',
    });

    // Total Commander-style function keys
    this.keyboardService.register({
      id: 'tc-copy',
      key: 'f5',
      modifiers: [],
      action: callbacks.copyToOtherPane,
      context: 'global',
      category: 'files',
      description: 'Copy to other pane (F5)',
    });

    this.keyboardService.register({
      id: 'tc-move',
      key: 'f6',
      modifiers: [],
      action: callbacks.moveToOtherPane,
      context: 'global',
      category: 'files',
      description: 'Move to other pane (F6)',
    });

    this.keyboardService.register({
      id: 'tc-rename',
      key: 'f6',
      modifiers: ['shift'],
      action: callbacks.renameSelected,
      context: 'global',
      category: 'files',
      description: 'Rename selected item (Shift+F6)',
    });

    this.keyboardService.register({
      id: 'tc-new-folder',
      key: 'f7',
      modifiers: [],
      action: callbacks.newFolder,
      context: 'global',
      category: 'files',
      description: 'Create folder (F7)',
    });

    this.keyboardService.register({
      id: 'tc-delete',
      key: 'f8',
      modifiers: [],
      action: callbacks.deleteSelected,
      context: 'global',
      category: 'files',
      description: 'Delete selected items (F8)',
    });
  }

  /**
   * Search and filter shortcuts
   */
  private registerSearchShortcuts(callbacks: ShortcutCallbacks): void {
    // Focus filter
    this.keyboardService.register({
      id: 'focus-filter',
      key: 'f',
      modifiers: ['ctrl'],
      action: callbacks.focusFilter,
      context: 'global',
      category: 'search',
      description: 'Focus filter input',
    });

    // Advanced filter
    this.keyboardService.register({
      id: 'advanced-filter',
      key: 'f',
      modifiers: ['ctrl', 'shift'],
      action: callbacks.advancedFilter,
      context: 'global',
      category: 'search',
      description: 'Advanced filter options',
    });

    // Clear filter
    this.keyboardService.register({
      id: 'clear-filter',
      key: 'escape',
      modifiers: [],
      action: callbacks.clearFilter,
      context: 'filter',
      category: 'search',
      description: 'Clear filter',
    });
  }

  /**
   * Workspace shortcuts
   */
  private registerWorkspaceShortcuts(callbacks: ShortcutCallbacks): void {
    // Workspace switcher
    this.keyboardService.register({
      id: 'workspace-switcher',
      key: 'w',
      modifiers: ['ctrl', 'shift'],
      action: callbacks.showWorkspaceSwitcher,
      context: 'global',
      category: 'workspace',
      description: 'Open workspace switcher',
    });

    // New workspace
    this.keyboardService.register({
      id: 'new-workspace',
      key: 'n',
      modifiers: ['ctrl', 'shift', 'alt'],
      action: callbacks.newWorkspace,
      context: 'global',
      category: 'workspace',
      description: 'Create new workspace',
    });

    // Toggle hidden files
    this.keyboardService.register({
      id: 'toggle-hidden',
      key: 'h',
      modifiers: ['ctrl', 'shift'],
      action: callbacks.toggleHiddenFiles,
      context: 'global',
      category: 'workspace',
      description: 'Show/hide hidden files',
    });
  }

  /**
   * Pane control shortcuts
   */
  private registerPaneShortcuts(callbacks: ShortcutCallbacks): void {
    // Switch pane
    this.keyboardService.register({
      id: 'switch-pane',
      key: 'tab',
      modifiers: [],
      action: callbacks.switchPane,
      context: 'global',
      category: 'pane',
      description: 'Switch between panes',
    });

    // Copy to other pane
    this.keyboardService.register({
      id: 'copy-to-pane',
      key: 'c',
      modifiers: ['ctrl', 'shift'],
      action: callbacks.copyToOtherPane,
      context: 'pane',
      category: 'pane',
      description: 'Copy selected to other pane',
    });

    // Move to other pane
    this.keyboardService.register({
      id: 'move-to-pane',
      key: 'm',
      modifiers: ['ctrl', 'shift'],
      action: callbacks.moveToOtherPane,
      context: 'pane',
      category: 'pane',
      description: 'Move selected to other pane',
    });

    // Focus address bar — Ctrl+L
    // allowBubble: true so the event still reaches the address bar component's
    // own (keydown) handler, which calls enterEditMode() directly on the right instance.
    // Without this the global listener swallows the event via stopPropagation() and
    // the component never sees it.
    this.keyboardService.register({
      id: 'focus-address',
      key: 'l',
      modifiers: ['ctrl'],
      action: callbacks.focusAddressBar,
      context: 'global',
      category: 'pane',
      description: 'Focus address bar (Ctrl+L)',
      allowBubble: true,
    });

    // Focus address bar — Alt+D (Windows Explorer convention)
    // Same allowBubble requirement as Ctrl+L above.
    this.keyboardService.register({
      id: 'focus-address-alt-d',
      key: 'd',
      modifiers: ['alt'],
      action: callbacks.focusAddressBar,
      context: 'global',
      category: 'pane',
      description: 'Focus address bar (Alt+D)',
      allowBubble: true,
    });
  }
}

/**
 * Interface for shortcut callbacks
 * Implement this in your main component
 */
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
  selectCurrentAndMoveDown: () => void;
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
}