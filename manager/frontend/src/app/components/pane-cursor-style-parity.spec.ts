import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('pane cursor style parity', () => {
  const detailsCursor = [
    'inset 0 0 0 1px color-mix(',
    'var(--vsc-accent-blue, var(--vsc-focus-border)) 42%',
    'var(--vsc-list-active-background, var(--vsc-list-selected-emphasis)) 82%',
  ];

  it('uses the Details cursor treatment in all shared views', () => {
    for (const name of ['file-entry.component.ts', 'pane-grid.component.ts', 'pane-tree.component.ts']) {
      const source = read(name);
      for (const token of detailsCursor) expect(source).toContain(token);
    }
  });

  it('uses the Details cursor treatment in all dedicated pane views', () => {
    for (const name of [
      'directory-pane.component.ts',
      'registry-pane.component.ts',
      'autorun-pane.component.ts',
      'network-pane.component.ts',
      'environment-pane.component.ts',
      'bin-pane.component.ts',
      'system-table-pane.component.ts',
    ]) {
      const source = read(name);
      for (const token of detailsCursor) expect(source).toContain(token);
    }
  });
});
