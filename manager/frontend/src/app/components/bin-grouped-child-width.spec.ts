import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin grouped child row width', () => {
  it('uses the parent available width rather than intrinsic max-content width', () => {
    const selector = '.list.view-details.bin-grouped .entry.bin-revision-row {';
    const start = source.indexOf(selector);
    const end = source.indexOf('}', start);
    const block = source.slice(start, end + 1);

    expect(block).toContain('width: calc(100% - 40px);');
    expect(block).toContain('min-width: calc(100% - 40px);');
    expect(block).toContain('margin-left: 22px;');
    expect(block).toContain('margin-right: 18px;');
    expect(block).not.toContain('max-content');
  });
});
