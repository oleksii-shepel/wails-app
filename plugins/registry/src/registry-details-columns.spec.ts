import { readFileSync } from 'node:fs';

describe('Registry details columns', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('../services/plugin-manager.service.ts', import.meta.url),
    'utf8',
  );

  it('uses Name, Type, Size and Data for Registry details', () => {
    expect(pane).toContain("return this.panelColumnContext.sourceKind === 'registry'");
    expect(pane).toContain("column.id === 'builtin.registry.data'");
    expect(pane).toContain('*ngIf="!isRegistrySurface">Modified');
    expect(pane).toContain("['28px', 'minmax(180px, 1fr)', '120px', '82px', ...extra]");
  });

  it('does not contribute redundant Registry Kind or Type columns', () => {
    expect(manager).not.toContain("id: 'builtin.registry.kind'");
    expect(manager).not.toContain("id: 'builtin.registry.type'");
    expect(manager).toContain("id: 'builtin.registry.data'");
  });

  it('keeps actual REG_* metadata in the standard Type column', () => {
    expect(pane).toContain("return entry.registryValueType?.trim() || 'Value'");
    expect(pane).toContain("return 'Key'");
  });
});