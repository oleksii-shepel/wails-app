import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('pane cursor border-radius parity', () => {
  it('keeps the original 3px entry radius while horizontal cursor expansion is enabled', () => {
    for (const name of [
      'directory-pane.component.ts',
      'bin-pane.component.ts',
      'registry-pane.component.ts',
      'autorun-pane.component.ts',
      'network-pane.component.ts',
      'environment-pane.component.ts',
    ]) {
      const source = read(name);
      expect(source).toContain('border-radius: 3px;');
      const details = source.lastIndexOf('.list.view-details .entry {');
      if (details >= 0) {
        const block = source.slice(details, source.indexOf('}', details) + 1);
        expect(block).not.toContain('border-radius: 0');
      }
    }
  });

  it('keeps grouped Bin parent and child cursors rounded', () => {
    const source = read('bin-pane.component.ts');
    for (const selector of [
      '.list.view-details.bin-grouped .entry.bin-group-header {',
      '.list.view-details.bin-grouped .entry.bin-revision-row {',
    ]) {
      const start = source.lastIndexOf(selector);
      expect(start).toBeGreaterThanOrEqual(0);
      const block = source.slice(start, source.indexOf('}', start) + 1);
      expect(block).toContain('border-radius: 3px;');
    }
  });

  it('keeps System Table cursor rows rounded too', () => {
    expect(read('system-table-pane.component.ts')).toContain('border-radius:3px');
  });
});
