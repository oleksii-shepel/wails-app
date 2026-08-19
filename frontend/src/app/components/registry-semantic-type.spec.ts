import { readFileSync } from 'node:fs';

describe('Registry semantic Type presentation', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('shows the synthetic parent row as Key on registry surfaces', () => {
    expect(pane).toContain("entry.path === '__parent__' && this.panelColumnContext.sourceKind === 'registry'");
    expect(pane).toContain("return 'Key'");
  });

  it('shows registry keys as Key instead of Folder', () => {
    expect(pane).toContain("if (registryKind === 'key')");
    expect(pane).toContain("return 'Key'");
  });

  it('shows the real REG_* type for registry values instead of File', () => {
    expect(pane).toContain("if (registryKind === 'value')");
    expect(pane).toContain("return entry.registryValueType?.trim() || 'Value'");
  });

  it('keeps filesystem type presentation unchanged', () => {
    expect(pane).toContain("if (entry.type === FileType.DIRECTORY) return 'Folder'");
    expect(pane).toContain("return this.entryCategory(entry)?.category || 'File'");
  });
});
