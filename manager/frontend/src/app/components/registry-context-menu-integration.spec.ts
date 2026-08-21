import { readFileSync } from 'node:fs';

const dual = readFileSync(
  new URL('./dual-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Registry context-menu integration', () => {
  it('bypasses the generic filesystem menu for Registry entries and background', () => {
    expect(dual).toContain("if (/^registry:\/\//i.test(pane?.contextState.path ?? ''))");
    expect(dual).toContain("title: entry.name || 'Registry entry'");
    expect(dual).toContain("title: pane?.contextState.path || 'Registry'");
  });

  it('preserves the exact plugin invocation context selected when the menu opens', () => {
    expect(dual).toContain('private contextMenuActions = new Map');
    expect(dual).toContain('this.contextMenuActions.set(menuCommandId, () => command.execute())');
    expect(dual).toContain('const contextualAction = this.contextMenuActions.get(commandId)');
  });

  it('does not redispatch Registry plugin menu items with a reduced context', () => {
    expect(dual).toContain('reassembly.context.plugin.');
    expect(dual).toContain("source: 'context-menu'");
  });
});
