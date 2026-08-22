import { readFileSync } from 'node:fs';

const address = readFileSync(new URL('./address-bar.component.ts', import.meta.url), 'utf8');
const sources = readFileSync(new URL('../services/panel-source-registry.service.ts', import.meta.url), 'utf8');
const directories = readFileSync(new URL('../services/directory-source.service.ts', import.meta.url), 'utf8');

describe('address bar bin source integration', () => {
  it('offers bin:// as a friendly source root', () => {
    expect(address).toContain("{ path: 'bin://', displayName: 'Recycle Bin' }");
    expect(address).toContain('buildSourceRootSuggestions(query)');
  });

  it('routes bin:// through a dedicated PanelSource rather than filesystem fallback', () => {
    expect(sources).toContain("readonly id = 'bin'");
    expect(sources).toContain("return schemeOf(context.path) === 'bin'");
    expect(sources).toContain('new BinPanelSource(this.directories)');
  });

  it('lists recycle-bin entries through the trash API without treating them as native paths', () => {
    expect(directories).toContain('await this.apiService.listTrashEntries()');
    expect(directories).toContain('path: `bin://entry/${encodeURIComponent(entry.id)}`');
  });
});
