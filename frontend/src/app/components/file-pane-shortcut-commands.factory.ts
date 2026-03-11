import type { FilePaneFocusController, FilePaneFocusHost } from './file-pane-focus.controller';
import type { FilePaneStateController, FilePaneStateHost } from './file-pane-state.controller';
import type { FilePaneTabsController, FilePaneTabsHost } from './file-pane-tabs.controller';

export interface FilePaneShortcutCommands {
  selection: {
    selectAll(): void;
    deselectAll(): void;
    toggleCurrentAndMoveDown(): void;
    extendSelection(delta: -1 | 1): void;
    selectByMask(): void;
    unselectByMask(): void;
    invertSelection(): void;
    selectSameExtension(): void;
    markCurrentItem(): void;
    moveCursorUp(): void;
    moveCursorDown(): void;
    moveCursorLeft(): void;
    moveCursorRight(): void;
    moveCursorHome(): void;
    moveCursorEnd(): void;
    moveCursorPageUp(): void;
    moveCursorPageDown(): void;
  };
  tabs: {
    closeCurrentTab(): void;
    nextTab(): void;
    previousTab(): void;
    jumpToTab(index: number): void;
    togglePinCurrentTab(): void;
  };
  focus: {
    focusAddressBar(): void;
    focusFind(): void;
    focusFilter(): void;
    clearFilter(): void;
  };
}

export interface FilePaneShortcutCommandsSource {
  stateController: FilePaneStateController;
  stateHost: FilePaneStateHost;
  tabsController: FilePaneTabsController;
  tabsHost: FilePaneTabsHost;
  focusController: FilePaneFocusController;
  focusHost: FilePaneFocusHost;
}

export class FilePaneShortcutCommandsFactory {
  create(source: FilePaneShortcutCommandsSource): FilePaneShortcutCommands {
    return {
      selection: {
        selectAll: () => source.stateController.onSelectAll(source.stateHost),
        deselectAll: () => source.stateController.onDeselectAll(source.stateHost),
        toggleCurrentAndMoveDown: () => source.stateController.onToggleCurrentAndMoveDown(source.stateHost),
        extendSelection: (delta: -1 | 1) => source.stateController.onExtendSelection(source.stateHost, delta),
        selectByMask: () => source.stateController.onSelectByMaskWithPrompt(source.stateHost),
        unselectByMask: () => source.stateController.onUnselectByMaskWithPrompt(source.stateHost),
        invertSelection: () => source.stateController.onInvertSelection(source.stateHost),
        selectSameExtension: () => source.stateController.onSelectSameExtension(source.stateHost),
        markCurrentItem: () => source.stateController.onMarkCurrentItem(source.stateHost),
        moveCursorUp: () => source.stateController.onMoveCursorUp(source.stateHost),
        moveCursorDown: () => source.stateController.onMoveCursorDown(source.stateHost),
        moveCursorLeft: () => source.stateController.onMoveCursorLeft(source.stateHost),
        moveCursorRight: () => source.stateController.onMoveCursorRight(source.stateHost),
        moveCursorHome: () => source.stateController.onMoveCursorHome(source.stateHost),
        moveCursorEnd: () => source.stateController.onMoveCursorEnd(source.stateHost),
        moveCursorPageUp: () => source.stateController.onMoveCursorPageUp(source.stateHost),
        moveCursorPageDown: () => source.stateController.onMoveCursorPageDown(source.stateHost),
      },
      tabs: {
        closeCurrentTab: () => source.tabsController.closeCurrentTab(source.tabsHost),
        nextTab: () => source.tabsController.nextTab(source.tabsHost),
        previousTab: () => source.tabsController.previousTab(source.tabsHost),
        jumpToTab: (index: number) => source.tabsController.jumpToTab(source.tabsHost, index),
        togglePinCurrentTab: () => source.tabsController.togglePinCurrentTab(source.tabsHost),
      },
      focus: {
        focusAddressBar: () => source.focusController.focusAddressBar(source.focusHost),
        focusFind: () => source.focusController.focusFind(source.focusHost),
        focusFilter: () => source.focusController.focusFilter(source.focusHost),
        clearFilter: () => source.focusController.clearFilter(source.focusHost),
      },
    };
  }
}