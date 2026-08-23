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
  exclusive = false,
): InputContextDefinition => ({
  id,
  kind,
  priority,
  description,
  parentId,
  ...(exclusive ? { exclusive: true } : {}),
});

const keyBinding = (
  id: string,
  contextId: string,
  commandId: string,
  key: string,
  description: string,
  modifiers?: Array<'ctrl' | 'shift' | 'alt' | 'meta'>,
  when?: string,
): InputBindingDefinition => ({
  id,
  contextId,
  commandId,
  source: 'default',
  description,
  when,
  trigger: {
    device: 'keyboard',
    chord: {
      steps: [{ key, modifiers }],
    },
  },
});


export const PATH_HISTORY_INPUT_COMMANDS = {
  close: 'reassembly.pathHistory.close',
  previous: 'reassembly.pathHistory.previous',
  next: 'reassembly.pathHistory.next',
  open: 'reassembly.pathHistory.open',
  remove: 'reassembly.pathHistory.remove',
  clear: 'reassembly.pathHistory.clear',
  previousMode: 'reassembly.pathHistory.previousMode',
  nextMode: 'reassembly.pathHistory.nextMode',
  copyJson: 'reassembly.pathHistory.copyJson',
} as const;

export const QUICK_ACCESS_INPUT_COMMANDS = {
  close: 'reassembly.quickAccess.close',
  previous: 'reassembly.quickAccess.previous',
  next: 'reassembly.quickAccess.next',
  previousSection: 'reassembly.quickAccess.previousSection',
  nextSection: 'reassembly.quickAccess.nextSection',
  open: 'reassembly.quickAccess.open',
  removePin: 'reassembly.quickAccess.removePin',
  first: 'reassembly.quickAccess.first',
  last: 'reassembly.quickAccess.last',
  openSlot1: 'reassembly.quickAccess.openSlot1',
  openSlot2: 'reassembly.quickAccess.openSlot2',
  openSlot3: 'reassembly.quickAccess.openSlot3',
  openSlot4: 'reassembly.quickAccess.openSlot4',
  openSlot5: 'reassembly.quickAccess.openSlot5',
  openSlot6: 'reassembly.quickAccess.openSlot6',
  openSlot7: 'reassembly.quickAccess.openSlot7',
  openSlot8: 'reassembly.quickAccess.openSlot8',
  openSlot9: 'reassembly.quickAccess.openSlot9',
} as const;

export const PATH_HISTORY_INPUT: DialogBindingCatalogEntry = {
  context: dialogContext(
    'dialog.pathHistory',
    'dialog',
    390,
    'Path history',
    'global',
    true,
  ),
  bindings: [
    keyBinding('reassembly.pathHistory.escape', 'dialog.pathHistory', PATH_HISTORY_INPUT_COMMANDS.close, 'escape', 'Close path history'),
    keyBinding('reassembly.pathHistory.up', 'dialog.pathHistory', PATH_HISTORY_INPUT_COMMANDS.previous, 'arrowup', 'Previous history entry'),
    keyBinding('reassembly.pathHistory.down', 'dialog.pathHistory', PATH_HISTORY_INPUT_COMMANDS.next, 'arrowdown', 'Next history entry'),
    keyBinding('reassembly.pathHistory.enter', 'dialog.pathHistory', PATH_HISTORY_INPUT_COMMANDS.open, 'enter', 'Open history entry'),
    keyBinding('reassembly.pathHistory.delete', 'dialog.pathHistory', PATH_HISTORY_INPUT_COMMANDS.remove, 'delete', 'Remove history entry', undefined, '!typing'),
    keyBinding('reassembly.pathHistory.ctrlDelete', 'dialog.pathHistory', PATH_HISTORY_INPUT_COMMANDS.clear, 'delete', 'Clear history', ['ctrl']),
    keyBinding('reassembly.pathHistory.previousMode', 'dialog.pathHistory', PATH_HISTORY_INPUT_COMMANDS.previousMode, 'pageup', 'Previous history view', ['ctrl']),
    keyBinding('reassembly.pathHistory.nextMode', 'dialog.pathHistory', PATH_HISTORY_INPUT_COMMANDS.nextMode, 'pagedown', 'Next history view', ['ctrl']),
    keyBinding('reassembly.pathHistory.ctrlC', 'dialog.pathHistory', PATH_HISTORY_INPUT_COMMANDS.copyJson, 'c', 'Copy history JSON', ['ctrl'], '!typing'),
  ].map(binding => ({ ...binding, category: 'Navigation' })),
};

export const QUICK_ACCESS_INPUT: DialogBindingCatalogEntry = {
  context: dialogContext(
    'overlay.quickAccess',
    'overlay',
    380,
    'Quick Access',
    'global',
    true,
  ),
  bindings: [
    keyBinding('reassembly.quickAccess.escape', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.close, 'escape', 'Close Quick Access'),
    keyBinding('reassembly.quickAccess.up', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.previous, 'arrowup', 'Previous item'),
    keyBinding('reassembly.quickAccess.down', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.next, 'arrowdown', 'Next item'),
    keyBinding('reassembly.quickAccess.previousSection', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.previousSection, 'pageup', 'Previous section', ['ctrl']),
    keyBinding('reassembly.quickAccess.nextSection', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.nextSection, 'pagedown', 'Next section', ['ctrl']),
    keyBinding('reassembly.quickAccess.enter', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.open, 'enter', 'Open item'),
    keyBinding('reassembly.quickAccess.delete', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.removePin, 'delete', 'Remove pin', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.home', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.first, 'home', 'First Quick Access item', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.end', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.last, 'end', 'Last Quick Access item', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.slot1', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.openSlot1, '1', 'Open Quick Access slot 1', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.slot2', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.openSlot2, '2', 'Open Quick Access slot 2', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.slot3', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.openSlot3, '3', 'Open Quick Access slot 3', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.slot4', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.openSlot4, '4', 'Open Quick Access slot 4', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.slot5', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.openSlot5, '5', 'Open Quick Access slot 5', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.slot6', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.openSlot6, '6', 'Open Quick Access slot 6', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.slot7', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.openSlot7, '7', 'Open Quick Access slot 7', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.slot8', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.openSlot8, '8', 'Open Quick Access slot 8', undefined, '!typing'),
    keyBinding('reassembly.quickAccess.slot9', 'overlay.quickAccess', QUICK_ACCESS_INPUT_COMMANDS.openSlot9, '9', 'Open Quick Access slot 9', undefined, '!typing'),
  ].map(binding => ({ ...binding, category: 'Navigation' })),
};

export const BUILTIN_INPUT_CATALOG: DialogBindingCatalogEntry[] = [
  {
    context: dialogContext('dialog.archiveMoveWarning', 'dialog', 990, 'Archive move warning dialog'),
    bindings: [
      keyBinding('dialog.archiveMoveWarning.confirm.enter', 'dialog.archiveMoveWarning', 'dialog.archiveMoveWarning.confirm', 'enter', 'Confirm archive move'),
      keyBinding('dialog.archiveMoveWarning.cancel.escape', 'dialog.archiveMoveWarning', 'dialog.archiveMoveWarning.cancel', 'escape', 'Cancel archive move'),
      keyBinding('dialog.archiveMoveWarning.focus.next', 'dialog.archiveMoveWarning', 'dialog.archiveMoveWarning.focusNext', 'tab', 'Move focus to next action'),
      keyBinding('dialog.archiveMoveWarning.focus.prev', 'dialog.archiveMoveWarning', 'dialog.archiveMoveWarning.focusPrev', 'tab', 'Move focus to previous action', ['shift']),
      keyBinding('dialog.archiveMoveWarning.focus.next.arrowRight', 'dialog.archiveMoveWarning', 'dialog.archiveMoveWarning.focusNextArrow', 'arrowright', 'Move focus to next action with arrows'),
      keyBinding('dialog.archiveMoveWarning.focus.prev.arrowLeft', 'dialog.archiveMoveWarning', 'dialog.archiveMoveWarning.focusPrevArrow', 'arrowleft', 'Move focus to previous action with arrows'),
    ],
  },
  {
    context: dialogContext('dialog.commandPalette', 'dialog', 975, 'Command palette'),
    bindings: [
      keyBinding('dialog.commandPalette.close.escape', 'dialog.commandPalette', 'dialog.commandPalette.close', 'escape', 'Close command palette'),
      keyBinding('dialog.commandPalette.next.arrowDown', 'dialog.commandPalette', 'dialog.commandPalette.next', 'arrowdown', 'Select next command'),
      keyBinding('dialog.commandPalette.prev.arrowUp', 'dialog.commandPalette', 'dialog.commandPalette.prev', 'arrowup', 'Select previous command'),
    ],
  },
  {
    context: dialogContext('dialog.keyboardHelp', 'dialog', 970, 'Keyboard help dialog'),
    bindings: [
      keyBinding('dialog.keyboardHelp.close.escape', 'dialog.keyboardHelp', 'dialog.keyboardHelp.close', 'escape', 'Close keyboard help'),
      keyBinding('dialog.keyboardHelp.focus.ctrlF', 'dialog.keyboardHelp', 'dialog.keyboardHelp.focusSearch', 'f', 'Focus shortcut search', ['ctrl']),
      keyBinding('dialog.keyboardHelp.export.ctrlS', 'dialog.keyboardHelp', 'dialog.keyboardHelp.exportBindings', 's', 'Export bindings', ['ctrl']),
      keyBinding('dialog.keyboardHelp.focus.next.arrowRight', 'dialog.keyboardHelp', 'dialog.keyboardHelp.focusNextArrow', 'arrowright', 'Move focus to next help control'),
      keyBinding('dialog.keyboardHelp.focus.prev.arrowLeft', 'dialog.keyboardHelp', 'dialog.keyboardHelp.focusPrevArrow', 'arrowleft', 'Move focus to previous help control'),
    ],
  },
  PATH_HISTORY_INPUT,
  {
    context: dialogContext('dialog.remoteConnect', 'dialog', 955, 'S3-compatible remote connect dialog'),
    bindings: [
      keyBinding('dialog.remoteConnect.submit.enter', 'dialog.remoteConnect', 'dialog.remoteConnect.submit', 'enter', 'Submit remote connect dialog'),
      keyBinding('dialog.remoteConnect.close.escape', 'dialog.remoteConnect', 'dialog.remoteConnect.close', 'escape', 'Close remote connect dialog'),
      keyBinding('dialog.remoteConnect.focus.next', 'dialog.remoteConnect', 'dialog.remoteConnect.focusNext', 'tab', 'Move focus to next control'),
      keyBinding('dialog.remoteConnect.focus.prev', 'dialog.remoteConnect', 'dialog.remoteConnect.focusPrev', 'tab', 'Move focus to previous control', ['shift']),
    ],
  },
  {
    context: dialogContext('dialog.settings', 'dialog', 950, 'Project settings dialog'),
    bindings: [
      keyBinding('dialog.settings.save.ctrlEnter', 'dialog.settings', 'dialog.settings.save', 'enter', 'Save settings changes', ['ctrl']),
      keyBinding('dialog.settings.close.escape', 'dialog.settings', 'dialog.settings.close', 'escape', 'Close settings dialog'),
      keyBinding('dialog.settings.focus.next', 'dialog.settings', 'dialog.settings.focusNext', 'tab', 'Move focus to next control'),
      keyBinding('dialog.settings.focus.prev', 'dialog.settings', 'dialog.settings.focusPrev', 'tab', 'Move focus to previous control', ['shift']),
    ],
  },
  {
    context: dialogContext('dialog.batchRename', 'dialog', 945, 'Batch rename dialog'),
    bindings: [
      keyBinding('dialog.batchRename.apply.ctrlEnter', 'dialog.batchRename', 'dialog.batchRename.apply', 'enter', 'Apply batch rename', ['ctrl']),
      keyBinding('dialog.batchRename.close.escape', 'dialog.batchRename', 'dialog.batchRename.close', 'escape', 'Close batch rename dialog'),
      keyBinding('dialog.batchRename.focus.next', 'dialog.batchRename', 'dialog.batchRename.focusNext', 'tab', 'Move focus to next control'),
      keyBinding('dialog.batchRename.focus.prev', 'dialog.batchRename', 'dialog.batchRename.focusPrev', 'tab', 'Move focus to previous control', ['shift']),
    ],
  },
  {
    context: dialogContext('dialog.trash', 'dialog', 940, 'Trash dialog'),
    bindings: [
      keyBinding('dialog.trash.close.escape', 'dialog.trash', 'dialog.trash.close', 'escape', 'Close Trash dialog'),
      keyBinding('dialog.trash.focus.next', 'dialog.trash', 'dialog.trash.focusNext', 'tab', 'Move focus to next control'),
      keyBinding('dialog.trash.focus.prev', 'dialog.trash', 'dialog.trash.focusPrev', 'tab', 'Move focus to previous control', ['shift']),
      keyBinding('dialog.trash.open.enter', 'dialog.trash', 'dialog.trash.openSelection', 'enter', 'Open selected Trash item', undefined, '!typing'),
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
    context: dialogContext('widget.addressBar.edit', 'widget', 810, 'Address bar edit field'),
    bindings: [
      keyBinding('widget.addressBar.edit.commit.enter', 'widget.addressBar.edit', 'widget.addressBar.edit.commit', 'enter', 'Commit address bar path', undefined, 'typing'),
      keyBinding('widget.addressBar.edit.cancel.escape', 'widget.addressBar.edit', 'widget.addressBar.edit.cancel', 'escape', 'Cancel address bar edit', undefined, 'typing'),
      keyBinding('widget.addressBar.edit.complete.ctrlSpace', 'widget.addressBar.edit', 'widget.addressBar.edit.showSuggestions', 'space', 'Show address suggestions', ['ctrl'], 'typing'),
      keyBinding('widget.addressBar.edit.prev.arrowUp', 'widget.addressBar.edit', 'widget.addressBar.edit.prevItem', 'arrowup', 'Select previous address suggestion', undefined, 'typing'),
      keyBinding('widget.addressBar.edit.next.arrowDown', 'widget.addressBar.edit', 'widget.addressBar.edit.nextItem', 'arrowdown', 'Select next address suggestion', undefined, 'typing'),
    ],
  },
  QUICK_ACCESS_INPUT,
  {
    context: dialogContext('overlay.addressBar.drives', 'overlay', 890, 'Address bar drive selector'),
    bindings: [
      keyBinding('overlay.addressBar.drives.close.escape', 'overlay.addressBar.drives', 'overlay.addressBar.drives.close', 'escape', 'Close drive menu'),
      keyBinding('overlay.addressBar.drives.prev.arrowUp', 'overlay.addressBar.drives', 'overlay.addressBar.drives.prevItem', 'arrowup', 'Select previous drive'),
      keyBinding('overlay.addressBar.drives.next.arrowDown', 'overlay.addressBar.drives', 'overlay.addressBar.drives.nextItem', 'arrowdown', 'Select next drive'),
      keyBinding('overlay.addressBar.drives.prev.arrowLeft', 'overlay.addressBar.drives', 'overlay.addressBar.drives.prevColumn', 'arrowleft', 'Select previous drive tile'),
      keyBinding('overlay.addressBar.drives.next.arrowRight', 'overlay.addressBar.drives', 'overlay.addressBar.drives.nextColumn', 'arrowright', 'Select next drive tile'),
      keyBinding('overlay.addressBar.drives.open.enter', 'overlay.addressBar.drives', 'overlay.addressBar.drives.openSelected', 'enter', 'Open selected drive'),
    ],
  },
  {
    context: dialogContext('overlay.addressBar.siblings', 'overlay', 880, 'Address bar sibling selector'),
    bindings: [
      keyBinding('overlay.addressBar.siblings.close.escape', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.close', 'escape', 'Close sibling menu'),
      keyBinding('overlay.addressBar.siblings.prev.arrowUp', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.prevItem', 'arrowup', 'Select previous sibling'),
      keyBinding('overlay.addressBar.siblings.next.arrowDown', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.nextItem', 'arrowdown', 'Select next sibling'),
      keyBinding('overlay.addressBar.siblings.prev.arrowLeft', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.prevItem', 'arrowleft', 'Select previous sibling'),
      keyBinding('overlay.addressBar.siblings.next.arrowRight', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.nextItem', 'arrowright', 'Select next sibling'),
      keyBinding('overlay.addressBar.siblings.open.enter', 'overlay.addressBar.siblings', 'overlay.addressBar.siblings.openSelected', 'enter', 'Open selected sibling'),
    ],
  },
  {
    context: dialogContext('overlay.workspaceSelector', 'overlay', 870, 'Workspace selector menu'),
    bindings: [
      keyBinding('overlay.workspaceSelector.close.escape', 'overlay.workspaceSelector', 'overlay.workspaceSelector.close', 'escape', 'Close workspace menu'),
      keyBinding('overlay.workspaceSelector.prev.arrowUp', 'overlay.workspaceSelector', 'overlay.workspaceSelector.prevItem', 'arrowup', 'Select previous workspace'),
      keyBinding('overlay.workspaceSelector.next.arrowDown', 'overlay.workspaceSelector', 'overlay.workspaceSelector.nextItem', 'arrowdown', 'Select next workspace'),
      keyBinding('overlay.workspaceSelector.prev.arrowLeft', 'overlay.workspaceSelector', 'overlay.workspaceSelector.prevItem', 'arrowleft', 'Select previous workspace'),
      keyBinding('overlay.workspaceSelector.next.arrowRight', 'overlay.workspaceSelector', 'overlay.workspaceSelector.nextItem', 'arrowright', 'Select next workspace'),
      keyBinding('overlay.workspaceSelector.open.enter', 'overlay.workspaceSelector', 'overlay.workspaceSelector.activateSelected', 'enter', 'Activate selected workspace'),
    ],
  },
  {
    context: dialogContext('overlay.filter.category', 'overlay', 860, 'Filter category dropdown'),
    bindings: [
      keyBinding('overlay.filter.category.close.escape', 'overlay.filter.category', 'overlay.filter.category.close', 'escape', 'Close category dropdown'),
      keyBinding('overlay.filter.category.prev.arrowUp', 'overlay.filter.category', 'overlay.filter.category.prevItem', 'arrowup', 'Select previous category'),
      keyBinding('overlay.filter.category.next.arrowDown', 'overlay.filter.category', 'overlay.filter.category.nextItem', 'arrowdown', 'Select next category'),
      keyBinding('overlay.filter.category.prev.arrowLeft', 'overlay.filter.category', 'overlay.filter.category.prevItem', 'arrowleft', 'Select previous category'),
      keyBinding('overlay.filter.category.next.arrowRight', 'overlay.filter.category', 'overlay.filter.category.nextItem', 'arrowright', 'Select next category'),
      keyBinding('overlay.filter.category.open.enter', 'overlay.filter.category', 'overlay.filter.category.select', 'enter', 'Apply selected category'),
    ],
  },
  {
    context: dialogContext('overlay.filter.presets', 'overlay', 850, 'Filter presets dropdown'),
    bindings: [
      keyBinding('overlay.filter.presets.close.escape', 'overlay.filter.presets', 'overlay.filter.presets.close', 'escape', 'Close presets dropdown'),
      keyBinding('overlay.filter.presets.prev.arrowUp', 'overlay.filter.presets', 'overlay.filter.presets.prevItem', 'arrowup', 'Select previous preset'),
      keyBinding('overlay.filter.presets.next.arrowDown', 'overlay.filter.presets', 'overlay.filter.presets.nextItem', 'arrowdown', 'Select next preset'),
      keyBinding('overlay.filter.presets.prev.arrowLeft', 'overlay.filter.presets', 'overlay.filter.presets.prevItem', 'arrowleft', 'Select previous preset'),
      keyBinding('overlay.filter.presets.next.arrowRight', 'overlay.filter.presets', 'overlay.filter.presets.nextItem', 'arrowright', 'Select next preset'),
      keyBinding('overlay.filter.presets.open.enter', 'overlay.filter.presets', 'overlay.filter.presets.select', 'enter', 'Apply selected preset'),
    ],
  },
];
/** @deprecated Use BUILTIN_INPUT_CATALOG. */
export const DIALOG_INPUT_CATALOG = BUILTIN_INPUT_CATALOG;

