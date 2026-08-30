import { KeyboardHelpComponent } from './keyboard-help.component';
import type {
  InputBindingDefinition,
  InputContextDefinition,
} from '../services/input-system.types';

function keyboardBinding(
  id: string,
  contextId: string,
  category: string,
  description: string,
  key: string,
): InputBindingDefinition {
  return {
    id,
    contextId,
    commandId: `${id}.command`,
    source: 'default',
    category,
    description,
    trigger: {
      device: 'keyboard',
      chord: {
        steps: [{ key }],
      },
    },
  };
}

describe('KeyboardHelpComponent', () => {
  function createComponent(options?: {
    activeContextIds?: string[];
    contexts?: InputContextDefinition[];
    bindings?: InputBindingDefinition[];
  }): KeyboardHelpComponent {
    const activeContextIds = new Set(options?.activeContextIds ?? []);
    const contexts = options?.contexts ?? [];
    const bindings = options?.bindings ?? [];

    return new KeyboardHelpComponent(
      { acquire: () => () => undefined } as any,
      {
        registerCommand: () => undefined,
        unregisterCommand: () => undefined,
        getActiveContextIds: () => activeContextIds,
        getContextSnapshot: () => contexts,
        getBindingSnapshot: () => bindings,
      } as any,
      {
        acquire: () => undefined,
        release: () => undefined,
      } as any,
      { detectChanges: () => undefined } as any,
    );
  }

  it('shows dialog shortcuts before preserved context and global bindings', () => {
    const component = createComponent({
      activeContextIds: ['global', 'overlay.quickAccess'],
      contexts: [
        { id: 'global', kind: 'global', priority: 0, description: 'Global' },
        { id: 'overlay.quickAccess', kind: 'overlay', priority: 380, description: 'Quick Access' },
        { id: 'dialog.keyboardHelp', kind: 'dialog', priority: 970, description: 'Keyboard help dialog' },
      ],
      bindings: [
        keyboardBinding('context.open', 'overlay.quickAccess', 'Navigation', 'Open item', 'enter'),
        keyboardBinding('help.close', 'dialog.keyboardHelp', 'Navigation', 'Close keyboard help', 'escape'),
        keyboardBinding('global.refresh', 'global', 'Navigation', 'Refresh current pane', 'r'),
      ],
    });

    component.show();

    expect(component.sections.map(section => section.title)).toEqual([
      'This Dialog',
      'Current Context: Quick Access',
      'Global',
    ]);
    expect(component.sections[1].categories[0].bindings.map(binding => binding.description)).toEqual(['Open item']);
  });

  it('filters all sections by search text', () => {
    const component = createComponent({
      activeContextIds: ['global', 'overlay.quickAccess'],
      contexts: [
        { id: 'global', kind: 'global', priority: 0, description: 'Global' },
        { id: 'overlay.quickAccess', kind: 'overlay', priority: 380, description: 'Quick Access' },
        { id: 'dialog.keyboardHelp', kind: 'dialog', priority: 970, description: 'Keyboard help dialog' },
      ],
      bindings: [
        keyboardBinding('context.open', 'overlay.quickAccess', 'Navigation', 'Open item', 'enter'),
        keyboardBinding('help.focus', 'dialog.keyboardHelp', 'Navigation', 'Focus shortcut search', 'f'),
        keyboardBinding('global.refresh', 'global', 'Navigation', 'Refresh current pane', 'r'),
      ],
    });

    component.show();
    component.searchText = 'refresh';
    component.onSearchChange();

    expect(component.filteredSections.map(section => section.title)).toEqual(['Global']);
    expect(component.hasSearchResults).toBeTrue();
  });

  it('can scope the dialog to the current workspace and hide global bindings', () => {
    const component = createComponent({
      activeContextIds: ['global', 'overlay.quickAccess'],
      contexts: [
        { id: 'global', kind: 'global', priority: 0, description: 'Global' },
        { id: 'overlay.quickAccess', kind: 'overlay', priority: 380, description: 'Quick Access' },
        { id: 'dialog.keyboardHelp', kind: 'dialog', priority: 970, description: 'Keyboard help dialog' },
      ],
      bindings: [
        keyboardBinding('context.open', 'overlay.quickAccess', 'Navigation', 'Open item', 'enter'),
        keyboardBinding('help.focus', 'dialog.keyboardHelp', 'Navigation', 'Focus shortcut search', 'f'),
        keyboardBinding('global.refresh', 'global', 'Navigation', 'Refresh current pane', 'r'),
      ],
    });

    component.show({ contextIds: ['overlay.quickAccess'], includeGlobal: false });

    expect(component.sections.map(section => section.title)).toEqual([
      'This Dialog',
      'Current Context: Quick Access',
    ]);
  });
});
