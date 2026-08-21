import { readFileSync } from 'node:fs';

const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');
const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
const autorun = readFileSync(new URL('./autorun-pane.component.ts', import.meta.url), 'utf8');
const systemTable = readFileSync(new URL('./system-table-pane.component.ts', import.meta.url), 'utf8');

describe('pane column spacing', () => {
  it('defines one pane-wide column gap token', () => {
    expect(host).toContain('--pane-column-gap: 6px');
  });

  it('uses the shared gap in Directory list/details/tree rows', () => {
    expect(directory).toMatch(/\.details-header\s*\{[^}]*column-gap:\s*var\(--pane-column-gap,\s*6px\)/s);
    expect(directory).toMatch(/\.entry\s*\{[^}]*column-gap:\s*var\(--pane-column-gap,\s*6px\)/s);
    expect(directory).toMatch(/\.tree-row\s*\{[^}]*column-gap:\s*var\(--pane-column-gap,\s*6px\)/s);
  });

  it('uses the shared gap in sibling table views', () => {
    expect(autorun).toContain('column-gap: var(--pane-column-gap, 6px)');
    expect(systemTable).toContain('column-gap: var(--pane-column-gap, 6px)');
  });
});