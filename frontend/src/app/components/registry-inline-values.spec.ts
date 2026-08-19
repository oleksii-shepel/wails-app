import { readFileSync } from 'node:fs';

describe('Registry inline values', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('../services/plugin-manager.service.ts', import.meta.url),
    'utf8',
  );
  const api = readFileSync(
    new URL('../plugin/plugin-registry-api.ts', import.meta.url),
    'utf8',
  );

  it('lets providers opt columns into compact inline presentation', () => {
    expect(api).toContain('readonly inline?: boolean');
    expect(pane).toContain('get inlinePanelColumns()');
    expect(pane).toContain('.filter(column => column.inline === true)');
  });

  it('renders inline provider fields inside the list name cell', () => {
    expect(pane).toContain('class="inline-provider-fields"');
    expect(pane).toContain('panelColumnValue(column, entry) as inlineValue');
    expect(pane).toContain('inline-provider-value');
  });

  it('accepts the backend Registry metadata casing', () => {
    // vfs/registry exposes RegistryKind as "Key" / "Value".
    expect(manager).toContain("String(entry.registryKind ?? '').toLowerCase() === 'value'");
  });

  it('shows Registry Type and Data inline only for value rows', () => {
    expect(manager).toContain("id: 'builtin.registry.type'");
    expect(manager).toContain("id: 'builtin.registry.data'");
    expect(manager.match(/inline: true/g)?.length).toBeGreaterThanOrEqual(2);
    expect(manager).toContain("String(entry.registryKind ?? '').toLowerCase() === 'value'");
  });
});
