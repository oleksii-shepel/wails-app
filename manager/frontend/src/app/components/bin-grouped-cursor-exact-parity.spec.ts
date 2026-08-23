import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin grouped cursor exact paint parity', () => {
  it('changes only grouped cursor paint, not geometry', () => {
    const selector = '.list.view-details.bin-grouped .entry.bin-group-header.cursor {';
    const start = source.indexOf(selector);
    const end = source.indexOf('}', start);
    const block = source.slice(start, end + 1);

    expect(start).toBeGreaterThanOrEqual(0);
    expect(block).not.toContain('margin');
    expect(block).not.toContain('padding');
    expect(block).not.toContain('min-width');
    expect(block).not.toContain('width:');
    expect(block).not.toContain('height');
  });
});
