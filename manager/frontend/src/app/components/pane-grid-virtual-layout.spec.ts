import { readFileSync } from 'node:fs';

describe('pane grid virtual layout', () => {
  const grid = readFileSync(new URL('./pane-grid.component.ts', import.meta.url), 'utf8');
  const fileEntry = readFileSync(new URL('./file-entry.component.ts', import.meta.url), 'utf8');

  it('keeps the CSS row pitch aligned with the 124px virtual scroll model', () => {
    expect(grid).toContain('readonly rowHeight = 124');
    expect(grid).toContain(`[style.--grid-row-pitch]="rowHeight + 'px'"`);
    expect(grid).toContain('--grid-row-gap: var(--vsc-padding-md)');
    expect(grid).toContain('grid-auto-rows: calc(var(--grid-row-pitch) - var(--grid-row-gap))');
    expect(grid).toContain('.grid-window.virtual-window .grid-item');
    expect(grid).toContain('height: 100%');
    expect(grid).toContain('overflow: hidden');
  });

  it('keeps compact-mode row pitch consistent with the virtual scroll model', () => {
    expect(grid).toContain('--grid-row-gap: var(--vsc-padding-sm)');
    expect(grid).toContain('gap: var(--grid-row-gap)');
  });

  it('makes grid file entries participate in normal block sizing and bounds names', () => {
    expect(fileEntry).toContain(':host(.variant-grid)');
    expect(fileEntry).toContain('display: block');
    expect(fileEntry).toContain('width: 100%');
    expect(fileEntry).toContain('-webkit-line-clamp: 2');
  });
});
