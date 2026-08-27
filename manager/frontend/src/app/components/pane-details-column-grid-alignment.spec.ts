import { readFileSync } from 'node:fs';

describe('Details header/body column alignment', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('uses one grid template for header and body rows', () => {
    expect(source.match(/\[style\.grid-template-columns\]="columnGridTemplate"/g)?.length).toBe(2);
    expect(source).toContain('get columnGridTemplate(): string');
  });

  it('does not independently flex or size header/body cells', () => {
    expect(source).not.toContain('[style.flex]="col.flex"');
    expect(source).not.toContain('[style.flex]="cell.flex"');
    expect(source).not.toContain('[style.width]="col.widthVar"');
    expect(source).not.toContain('[style.width]="cell.widthVar"');
  });

  it('keeps compact header text vertically centered without clipping', () => {
    expect(source).toContain('font-size: 10px;');
    expect(source).toContain('line-height: 14px;');
    expect(source).toContain('min-height: 28px;');
    expect(source).toContain('align-items: center;');
  });
});
