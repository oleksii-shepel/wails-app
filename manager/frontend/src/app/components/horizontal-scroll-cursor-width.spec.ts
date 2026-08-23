import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('horizontal-scroll-aware cursor width', () => {
  it('uses intrinsic-width Details rows in all Directory-family panes', () => {
    for (const name of [
      'directory-pane.component.ts',
      'registry-pane.component.ts',
      'autorun-pane.component.ts',
      'network-pane.component.ts',
      'environment-pane.component.ts',
    ]) {
      const source = read(name);
      const start = source.indexOf('.list.view-details .entry {');
      const block = source.slice(start, source.indexOf('}', start) + 1);
      expect(block).toContain('width: max-content');
      expect(block).toContain('min-width: 100%');
    }
  });

  it('allows FileDetails rows to grow to the full column width', () => {
    const source = read('file-details.component.ts');
    expect(source).toContain('width: max-content;');
    expect(source).toContain('min-width: 100%;');
    expect(source).toContain('.details-window.virtual-window');
  });

  it('gives SystemTable rows scroll-aware width', () => {
    const source = read('system-table-pane.component.ts');
    expect(source).toContain('.row {display:grid; width:max-content;min-width:100%;');
  });
});
