import { readFileSync } from 'node:fs';

describe('Details uses legacy DirectoryPane header styling', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('restores the compact legacy header appearance', () => {
    expect(source).toContain('min-height: 25px;');
    expect(source).toContain('font-size: 10px;');
    expect(source).toContain('var(--vsc-description-foreground, var(--vsc-foreground-dim))');
    expect(source).toContain('var(--vsc-tabbar-background, var(--vsc-panel-background))');
  });

  it('retains shared dynamic column geometry', () => {
    expect(source.match(/\[style\.grid-template-columns\]="columnGridTemplate"/g)?.length).toBe(2);
  });

  it('retains the 4px List-style horizontal row inset', () => {
    expect(source).toContain('margin-left: 4px;');
    expect(source).toContain('margin-right: 4px;');
  });
});
