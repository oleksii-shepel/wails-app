import { readFileSync } from 'node:fs';

describe('pane background selection UX', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const fileEntry = readFileSync(
    new URL('./file-entry.component.ts', import.meta.url),
    'utf8',
  );

  it('does not clear selection or cursor when empty pane space is clicked', () => {
    const start = pane.indexOf('onBackgroundClick(event: MouseEvent)');
    const end = pane.indexOf('\n  }', start) + 4;
    const method = pane.slice(start, end);

    expect(method).not.toContain('this.selectedPaths = new Set()');
    expect(method).not.toContain('this.cursorPath = null');
    expect(method).not.toContain('this.anchorPath = null');
    expect(method).toContain("this.lastAction = 'pane focused'");
  });

  it('only recovers a cursor for an inactive pane with no existing selection state', () => {
    expect(pane).toContain('if (!this.cursorPath && this.selectedPaths.size === 0)');
    expect(pane).toContain('this.recoverVisibleCursor()');
  });
  it('declares and captures inactive-pane recovery state', () => {
    expect(pane).toContain('private restoreCursorOnBackgroundClick = false;');
    expect(pane).toContain('onPaneMouseDown(event: MouseEvent)');
    expect(pane).toContain('this.restoreCursorOnBackgroundClick = !this.active;');
  });

  it('keeps shared file entries transparent until hover or selection state applies', () => {
    expect(fileEntry).toContain('.file-entry {');
    expect(fileEntry).toContain('background: transparent;');
    expect(fileEntry).toContain('&:hover {');
    expect(fileEntry).toContain('&.selected {');
  });

});
