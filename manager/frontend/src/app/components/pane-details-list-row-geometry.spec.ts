import { readFileSync } from 'node:fs';

describe('Details matches Directory List geometry', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('uses the actual 30px Directory List row height', () => {
    expect(source).toContain('private readonly DEFAULT_ROW_HEIGHT = 30;');
    expect(source).toContain('rowHeight = 30;');
    expect(source).toContain('height: 30px;');
  });

  it('keeps body grid rows flush so columns do not drift horizontally', () => {
    expect(source).toMatch(/\.details-row\s*\{[^}]*padding:\s*0;/s);
  });

  it('retains the previous 28px header height', () => {
    expect(source).toContain('&.header-row {\n        height: 28px;');
  });

  it('keeps one shared grid definition for header and body', () => {
    expect(source.match(/\[style\.grid-template-columns\]="columnGridTemplate"/g)?.length).toBe(2);
  });
});
