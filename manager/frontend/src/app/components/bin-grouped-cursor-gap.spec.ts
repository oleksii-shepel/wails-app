import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin grouped cursor and name/location layout', () => {
  it('keeps group cursor visual state separate from selection semantics', () => {
    expect(source).toContain(
      '[class.bin-group-cursor]="isBinGroupHeader(entry) && entry.path === cursorPath"',
    );
    expect(source).toContain(
      '[class.selected]="!isBinGroupHeader(entry) && selectedPaths.has(entry.path)"',
    );
    expect(source).toContain(
      '[attr.aria-selected]="isBinGroupHeader(entry) ? false : selectedPaths.has(entry.path)"',
    );
  });

  it('uses Directory active-selection tokens for a group cursor', () => {
    expect(source).toContain('.pane.active-pane .entry.bin-group-cursor {');
    expect(source).toContain(
      'var(--vsc-list-active-background, var(--vsc-list-selected-emphasis)) 82%',
    );
    expect(source).toContain(
      'inset 3px 0 0 var(--vsc-accent-blue, var(--vsc-focus-border))',
    );
  });

  it('separates filename and location with an explicit flex gap', () => {
    expect(source).toContain('.bin-group-header .bin-name {');
    expect(source).toContain('gap: 12px;');
    expect(source).toContain('.bin-group-header .bin-group-directory {');
  });
});
