import { readFileSync } from 'node:fs';

const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');
const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
const autorun = readFileSync(new URL('./autorun-pane.component.ts', import.meta.url), 'utf8');
const systemTable = readFileSync(new URL('./system-table-pane.component.ts', import.meta.url), 'utf8');

describe('visible pane column gutters', () => {
  it('uses a deliberately visible 12px shared gutter', () => {
    expect(host).toMatch(/--pane-column-gutter\s*:\s*12px/);
  });

  it('applies the gutter to all Directory row-style views', () => {
    expect(directory).toContain('var(--pane-column-gutter, 12px)');
    expect(directory).toContain('.list.view-details .entry');
    expect(directory).toContain('.list.view-list .entry');
    expect(directory).toContain('.tree-row');
  });

  it('applies the gutter to Autorun and System Table views', () => {
    expect(autorun).toContain('var(--pane-column-gutter, 12px)');
    expect(systemTable).toContain('var(--pane-column-gutter, 12px)');
  });
});