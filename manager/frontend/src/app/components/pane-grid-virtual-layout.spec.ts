import { readFileSync } from 'node:fs';

describe('pane grid row geometry', () => {
  const grid = readFileSync(new URL('./pane-grid.component.ts', import.meta.url), 'utf8');
  const fileEntry = readFileSync(new URL('./file-entry.component.ts', import.meta.url), 'utf8');

  it('uses one fixed row contract for ordinary and virtual Grid', () => {
    expect(grid).toContain('readonly rowHeight = 124');
    expect(grid).toContain(`[style.--grid-row-pitch]="rowHeight + 'px'"`);
    expect(grid).toContain('--grid-row-gap: var(--vsc-padding-md)');
    expect(grid).toContain('grid-auto-rows: calc(var(--grid-row-pitch, 124px) - var(--grid-row-gap))');
    expect(grid).toContain('--grid-row-gap: var(--vsc-padding-sm)');
  });

  it('makes the custom file-entry host contribute to Grid cell height', () => {
    expect(grid).toContain('.grid-info app-file-entry');
    expect(grid).toContain('display: block');
    expect(grid).toContain('.grid-info {');
    expect(grid).toContain('flex: 1 1 auto');
    expect(grid).toContain('flex-direction: column');
  });

  it('keeps each grid item bounded to its row instead of painting into the next one', () => {
    expect(grid).toContain('box-sizing: border-box');
    expect(grid).toContain('min-height: calc(var(--grid-row-pitch, 124px) - var(--grid-row-gap))');
    expect(grid).toContain('.grid-window.virtual-window .grid-item');
    expect(grid).toContain('height: 100%');
    expect(grid).toContain('overflow: hidden');
    expect(grid).not.toContain('height: fit-content');
  });

  it('does not special-case parent cursor chrome', () => {
    const parentBlock = fileEntry.slice(
      fileEntry.indexOf('&.parent-dir {'),
      fileEntry.indexOf('.file-info { flex: 1;'),
    );
    expect(parentBlock).not.toContain('border-left-color: transparent !important');
    expect(parentBlock).not.toContain('&.cursor');
    expect(parentBlock).not.toContain('opacity: 0.65;\n        border');
    expect(parentBlock).toContain('.file-info { opacity: 0.65; }');
  });
});
