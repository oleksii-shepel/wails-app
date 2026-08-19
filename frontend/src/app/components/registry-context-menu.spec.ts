import { readFileSync } from 'node:fs';

describe('Registry context menu policy', () => {
  const dual = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('../services/plugin-manager.service.ts', import.meta.url),
    'utf8',
  );

  it('uses Registry-specific entry menus', () => {
    expect(dual).toContain('this.openRegistryEntryMenu(event, entry, pane)');
    expect(dual).toContain("item.commandId === 'builtin.registry.edit'");
    expect(dual).toContain("item.commandId === 'builtin.registry.delete'");
  });

  it('does not expose filesystem actions in Registry background menus', () => {
    expect(dual).toContain('this.openRegistryBackgroundMenu(event, pane)');
    expect(dual).toContain("item.commandId === 'builtin.registry.newKey'");
    expect(dual).toContain("item.commandId === 'builtin.registry.newValue'");
  });

  it('keeps creation contributions background-only', () => {
    expect(manager).toContain('context.targetPath === context.currentDirectory');
    expect(manager).toContain("label: 'New Key'");
    expect(manager).toContain("label: 'New Value'");
    expect(manager).toContain("label: 'Delete'");
  });
});
