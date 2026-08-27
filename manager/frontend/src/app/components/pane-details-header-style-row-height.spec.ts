import { readFileSync } from 'node:fs';

describe('Details header styling and row height', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('keeps the shared grid alignment fix', () => {
    expect(source.match(/\[style\.grid-template-columns\]="columnGridTemplate"/g)?.length).toBe(2);
  });

  it('restores the previous header spacing without clipping', () => {
    expect(source).toContain('margin-left: var(--vsc-padding-lg);');
    expect(source).toContain('margin-right: var(--vsc-padding-lg);');
    expect(source).toContain('font-size: 10px;');
    expect(source).toContain('line-height: normal;');
  });

  it('uses the roomier List-like 44px row geometry', () => {
    expect(source).toContain('private readonly DEFAULT_ROW_HEIGHT = 44;');
    expect(source).toContain('rowHeight = 44;');
    expect(source).toContain('height: 44px;');
  });
});
