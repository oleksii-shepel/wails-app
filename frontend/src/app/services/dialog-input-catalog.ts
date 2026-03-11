import {
  type DialogBindingCatalogEntry,
  type InputBindingDefinition,
  type InputContextDefinition,
} from './input-system.types';

const dialogContext = (
  id: string,
  kind: InputContextDefinition['kind'],
  priority: number,
  description: string,
  parentId?: string,
): InputContextDefinition => ({ id, kind, priority, description, parentId });

const keyBinding = (
  id: string,
  contextId: string,
  commandId: string,
  key: string,
  description: string,
  modifiers?: Array<'ctrl' | 'shift' | 'alt' | 'meta'>,
): InputBindingDefinition => ({
  id,
  contextId,
  commandId,
  source: 'default',
  description,
  trigger: {
    device: 'keyboard',
    chord: {
      steps: [{ key, modifiers }],
    },
  },
});

export const DIALOG_INPUT_CATALOG: DialogBindingCatalogEntry[] = [
  {
    context: dialogContext('dialog.confirm', 'dialog', 1000, 'Confirmation and alert dialog'),
    bindings: [
      keyBinding('dialog.confirm.accept.enter', 'dialog.confirm', 'dialog.confirm.accept', 'enter', 'Accept confirm dialog'),
      keyBinding('dialog.confirm.cancel.escape', 'dialog.confirm', 'dialog.confirm.cancel', 'escape', 'Cancel confirm dialog'),
      keyBinding('dialog.confirm.focus.next', 'dialog.confirm', 'dialog.confirm.focusNext', 'tab', 'Move focus to next action'),
      keyBinding('dialog.confirm.focus.prev', 'dialog.confirm', 'dialog.confirm.focusPrev', 'tab', 'Move focus to previous action', ['shift']),
    ],
  },
  {
    context: dialogContext('dialog.archiveMoveWarning', 'dialog', 990, 'Archive move warning dialog'),
    bindings: [
      keyBinding('dialog.archiveMoveWarning.confirm.enter', 'dialog.archiveMoveWarning', 'dialog.archiveMoveWarning.confirm', 'enter', 'Confirm archive move'),
      keyBinding('dialog.archiveMoveWarning.cancel.escape', 'dialog.archiveMoveWarning', 'dialog.archiveMoveWarning.cancel', 'escape', 'Cancel archive move'),
    ],
  },
  {
    context: dialogContext('dialog.globalSearch', 'dialog', 980, 'Global search dialog'),
    bindings: [
      keyBinding('dialog.globalSearch.close.escape', 'dialog.globalSearch', 'dialog.globalSearch.close', 'escape', 'Close global search'),
      keyBinding('dialog.globalSearch.prev.arrowUp', 'dialog.globalSearch', 'dialog.globalSearch.prevResult', 'arrowup', 'Select previous result'),
      keyBinding('dialog.globalSearch.next.arrowDown', 'dialog.globalSearch', 'dialog.globalSearch.nextResult', 'arrowdown', 'Select next result'),
      keyBinding('dialog.globalSearch.open.enter', 'dialog.globalSearch', 'dialog.globalSearch.openSelected', 'enter', 'Open selected result'),
      keyBinding('dialog.globalSearch.settings.ctrlComma', 'dialog.globalSearch', 'dialog.globalSearch.openSettings', ',', 'Open search settings', ['ctrl']),
    ],
  },
  {
    context: dialogContext('dialog.globalSearch.settings', 'dialog', 985, 'Global search settings dialog', 'dialog.globalSearch'),
    bindings: [
      keyBinding('dialog.globalSearch.settings.close.escape', 'dialog.globalSearch.settings', 'dialog.globalSearch.settings.close', 'escape', 'Close search settings'),
      keyBinding('dialog.globalSearch.settings.apply.enter', 'dialog.globalSearch.settings', 'dialog.globalSearch.settings.apply', 'enter', 'Apply settings'),
    ],
  },
  {
    context: dialogContext('dialog.keyboardHelp', 'dialog', 970, 'Keyboard help dialog'),
    bindings: [
      keyBinding('dialog.keyboardHelp.close.escape', 'dialog.keyboardHelp', 'dialog.keyboardHelp.close', 'escape', 'Close keyboard help'),
      keyBinding('dialog.keyboardHelp.focus.ctrlF', 'dialog.keyboardHelp', 'dialog.keyboardHelp.focusSearch', 'f', 'Focus shortcut search', ['ctrl']),
      keyBinding('dialog.keyboardHelp.export.ctrlS', 'dialog.keyboardHelp', 'dialog.keyboardHelp.exportBindings', 's', 'Export bindings', ['ctrl']),
    ],
  },
  {
    context: dialogContext('dialog.pathHistory', 'dialog', 960, 'Path history viewer'),
    bindings: [
      keyBinding('dialog.pathHistory.close.escape', 'dialog.pathHistory', 'dialog.pathHistory.close', 'escape', 'Close path history'),
      keyBinding('dialog.pathHistory.focus.ctrlF', 'dialog.pathHistory', 'dialog.pathHistory.focusFilter', 'f', 'Focus path filter', ['ctrl']),
      keyBinding('dialog.pathHistory.prev.arrowUp', 'dialog.pathHistory', 'dialog.pathHistory.prevItem', 'arrowup', 'Select previous path'),
      keyBinding('dialog.pathHistory.next.arrowDown', 'dialog.pathHistory', 'dialog.pathHistory.nextItem', 'arrowdown', 'Select next path'),
      keyBinding('dialog.pathHistory.open.enter', 'dialog.pathHistory', 'dialog.pathHistory.openSelected', 'enter', 'Open selected path'),
      keyBinding('dialog.pathHistory.remove.delete', 'dialog.pathHistory', 'dialog.pathHistory.removeSelected', 'delete', 'Remove selected path'),
    ],
  },
  {
    context: dialogContext('widget.filterInline', 'widget', 800, 'Inline viewer filter widget'),
    bindings: [
      keyBinding('widget.filterInline.close.escape', 'widget.filterInline', 'widget.filterInline.close', 'escape', 'Close filter widget'),
      keyBinding('widget.filterInline.next.enter', 'widget.filterInline', 'widget.filterInline.nextMatch', 'enter', 'Jump to next match'),
      keyBinding('widget.filterInline.prev.shiftEnter', 'widget.filterInline', 'widget.filterInline.prevMatch', 'enter', 'Jump to previous match', ['shift']),
      keyBinding('widget.filterInline.next.f3', 'widget.filterInline', 'widget.filterInline.nextMatch', 'f3', 'Jump to next match'),
      keyBinding('widget.filterInline.prev.shiftF3', 'widget.filterInline', 'widget.filterInline.prevMatch', 'f3', 'Jump to previous match', ['shift']),
    ],
  },
  {
    context: dialogContext('overlay.quickAccess', 'overlay', 900, 'Address bar quick access overlay'),
    bindings: [
      keyBinding('overlay.quickAccess.close.escape', 'overlay.quickAccess', 'overlay.quickAccess.close', 'escape', 'Close quick access'),
      keyBinding('overlay.quickAccess.nextSection.tab', 'overlay.quickAccess', 'overlay.quickAccess.nextSection', 'tab', 'Move to next section'),
      keyBinding('overlay.quickAccess.prevSection.shiftTab', 'overlay.quickAccess', 'overlay.quickAccess.prevSection', 'tab', 'Move to previous section', ['shift']),
      keyBinding('overlay.quickAccess.prev.arrowUp', 'overlay.quickAccess', 'overlay.quickAccess.prevItem', 'arrowup', 'Select previous item'),
      keyBinding('overlay.quickAccess.next.arrowDown', 'overlay.quickAccess', 'overlay.quickAccess.nextItem', 'arrowdown', 'Select next item'),
      keyBinding('overlay.quickAccess.open.enter', 'overlay.quickAccess', 'overlay.quickAccess.openSelected', 'enter', 'Open selected item'),
    ],
  },
  {
    context: dialogContext('overlay.addressBar.drives', 'overlay', 890, 'Address bar drive selector'),
    bindings: [
      keyBinding('overlay.addressBar.drives.close.escape', 'overlay.addressBar.drives', 'overlay.addressBar.drives.close', 'escape', 'Close drive menu'),
      keyBinding('overlay.addressBar.drives.prev.arrowUp', 'overlay.addressBar.drives', 'overlay.addressBar.drives.prevItem', 'arrowup', 'Select previous drive'),
      keyBinding('overlay.addressBar.drives.next.arrowDown', 'overlay.addressBar.drives', 'overlay.addressBar.drives.nextItem', 'arrowdown', 'Select next drive'),
      keyBinding('overlay.addressBar.drives.open.enter', 'overlay.addressBar.drives', 'overlay.addressBar.drives.openSelected', 'enter', 'Open selected drive'),
    ],
  },
  {
    context: dialogContext('overlay.addressBar.siblings', 'overlay', 880, 'Address bar sibling selector'),
    bindings: [
      keyBinding('overlay.addressBar.siblings.close.escape', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.close', 'escape', 'Close sibling menu'),
      keyBinding('overlay.addressBar.siblings.prev.arrowUp', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.prevItem', 'arrowup', 'Select previous sibling'),
      keyBinding('overlay.addressBar.siblings.next.arrowDown', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.nextItem', 'arrowdown', 'Select next sibling'),
      keyBinding('overlay.addressBar.siblings.open.enter', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.openSelected', 'enter', 'Open selected sibling'),
    ],
  },
  {
    context: dialogContext('overlay.workspaceSelector', 'overlay', 870, 'Workspace selector menu'),
    bindings: [
      keyBinding('overlay.workspaceSelector.close.escape', 'overlay.workspaceSelector', 'overlay.workspaceSelector.close', 'escape', 'Close workspace menu'),
      keyBinding('overlay.workspaceSelector.prev.arrowUp', 'overlay.workspaceSelector', 'overlay.workspaceSelector.prevItem', 'arrowup', 'Select previous workspace'),
      keyBinding('overlay.workspaceSelector.next.arrowDown', 'overlay.workspaceSelector', 'overlay.workspaceSelector.nextItem', 'arrowdown', 'Select next workspace'),
      keyBinding('overlay.workspaceSelector.open.enter', 'overlay.workspaceSelector', 'overlay.workspaceSelector.activateSelected', 'enter', 'Activate selected workspace'),
    ],
  },
  {
    context: dialogContext('overlay.filter.category', 'overlay', 860, 'Filter category dropdown'),
    bindings: [
      keyBinding('overlay.filter.category.close.escape', 'overlay.filter.category', 'overlay.filter.category.close', 'escape', 'Close category dropdown'),
      keyBinding('overlay.filter.category.prev.arrowUp', 'overlay.filter.category', 'overlay.filter.category.prevItem', 'arrowup', 'Select previous category'),
      keyBinding('overlay.filter.category.next.arrowDown', 'overlay.filter.category', 'overlay.filter.category.nextItem', 'arrowdown', 'Select next category'),
      keyBinding('overlay.filter.category.open.enter', 'overlay.filter.category', 'overlay.filter.category.select', 'enter', 'Apply selected category'),
    ],
  },
  {
    context: dialogContext('overlay.filter.presets', 'overlay', 850, 'Filter presets dropdown'),
    bindings: [
      keyBinding('overlay.filter.presets.close.escape', 'overlay.filter.presets', 'overlay.filter.presets.close', 'escape', 'Close presets dropdown'),
      keyBinding('overlay.filter.presets.prev.arrowUp', 'overlay.filter.presets', 'overlay.filter.presets.prevItem', 'arrowup', 'Select previous preset'),
      keyBinding('overlay.filter.presets.next.arrowDown', 'overlay.filter.presets', 'overlay.filter.presets.nextItem', 'arrowdown', 'Select next preset'),
      keyBinding('overlay.filter.presets.open.enter', 'overlay.filter.presets', 'overlay.filter.presets.select', 'enter', 'Apply selected preset'),
    ],
  },
];