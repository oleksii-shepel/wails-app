import { readFileSync } from 'node:fs';

describe('caption vertical clipping', () => {
  const menu = readFileSync(new URL('./application-menu-bar.component.ts', import.meta.url), 'utf8');
  const details = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('does not use a 1.0 line-height for the application menu font', () => {
    expect(menu).toContain('font: 12px/1.35');
    expect(menu).toContain('display: inline-flex;');
    expect(menu).toContain('align-items: center;');
  });

  it('gives Details header captions enough vertical glyph space', () => {
    expect(details).toContain('font-size: 10px;');
    expect(details).toContain('line-height: 1.35;');
    expect(details).toContain('min-height: 14px;');
  });
});
