import { readFileSync } from 'node:fs';

describe('Details header typography', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('uses the small UI font for Details header cells only', () => {
    expect(source).toContain('.details-header .reassembly-pane-table-cell');
    expect(source).toContain('font-size: var(--vsc-font-size-small, 11px);');
  });
});
