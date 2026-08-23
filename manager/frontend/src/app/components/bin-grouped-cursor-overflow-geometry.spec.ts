import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin grouped cursor overflow geometry', () => {
  it('preserves the standard parent row inset after expand/collapse', () => {
    const selector = '.list.view-details.bin-grouped .list-content-track > .entry.bin-group-header,';
    const start = source.indexOf(selector);
    const end = source.indexOf('}', start);
    const block = source.slice(start, end + 1);

    expect(start).toBeGreaterThanOrEqual(0);
    expect(block).toContain('margin-left: 4px;');
    expect(block).toContain('margin-right: 4px;');
    expect(block).toContain('border-radius: 3px;');
  });

  it('adds hierarchy indentation without dropping the right inset or radius', () => {
    const selector = '.list.view-details.bin-grouped .list-content-track > .entry.bin-revision-row {';
    const start = source.indexOf(selector);
    const end = source.indexOf('}', start);
    const block = source.slice(start, end + 1);

    expect(start).toBeGreaterThanOrEqual(0);
    expect(block).toContain('margin-left: 26px;');
    expect(block).toContain('margin-right: 4px;');
    expect(block).toContain('border-radius: 3px;');
  });

  it('keeps cursor and selection radius unchanged', () => {
    expect(source).toContain('.entry.bin-group-header.cursor,');
    expect(source).toContain('.entry.bin-revision-row.cursor,');
    expect(source).toContain('border-radius: 3px;');
  });
});
