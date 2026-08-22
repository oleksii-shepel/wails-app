import { readFileSync } from 'node:fs';

describe('pane background selection UX', () => {
  const read = (name: string) =>
    readFileSync(new URL(name, import.meta.url), 'utf8');

  const directory = read('./directory-pane.component.ts');
  const registry = read('./registry-pane.component.ts');
  const autorun = read('./autorun-pane.component.ts');
  const network = read('./network-pane.component.ts');
  const environment = read('./environment-pane.component.ts');
  const paneHost = read('./pane-host.component.ts');
  const fileEntry = read('./file-entry.component.ts');

  function backgroundMethod(source: string): string {
    const start = source.indexOf('onBackgroundClick(event: MouseEvent)');
    const end = source.indexOf('\n  }', start) + 4;
    return source.slice(start, end);
  }

  for (const [name, source] of [
    ['directory', directory],
    ['registry', registry],
    ['autorun', autorun],
    ['network', network],
    ['environment', environment],
  ] as const) {
    it(`${name} pane activates on empty space without losing cursor/selection`, () => {
      const method = backgroundMethod(source);

      expect(method).not.toContain('this.selectedPaths = new Set()');
      expect(method).not.toContain('this.cursorPath = null');
      expect(method).not.toContain('this.anchorPath = null');
      expect(method).not.toContain('this.cursorChange.emit(null)');
      expect(method).not.toContain('selection cleared');
      expect(method).toContain("this.lastAction = 'pane focused'");
      expect(method).toContain('this.activateRequested.emit()');
      expect(method).toContain('focus({ preventScroll: true })');
    });
  }

  it('provider/system-table background activation preserves provider selection', () => {
    const start = paneHost.indexOf('onProviderBackgroundClick(event: MouseEvent)');
    const end = paneHost.indexOf('\n  }', start) + 4;
    const method = paneHost.slice(start, end);

    expect(method).not.toContain('clearSelection');
    expect(method).toContain('this.activateRequested.emit()');
    expect(method).toContain('focus({ preventScroll: true })');
  });

  it('only recovers a cursor for an inactive pane with no existing selection state', () => {
    expect(directory).toContain('if (!this.cursorPath && this.selectedPaths.size === 0)');
    expect(directory).toContain('this.recoverVisibleCursor()');
  });

  it('declares and captures inactive-pane recovery state', () => {
    expect(directory).toContain('private restoreCursorOnBackgroundClick = false;');
    expect(directory).toContain('onPaneMouseDown(event: MouseEvent)');
    expect(directory).toContain('this.restoreCursorOnBackgroundClick = !this.active;');
  });

  it('keeps shared file entries transparent until hover or selection state applies', () => {
    expect(fileEntry).toContain('.file-entry {');
    expect(fileEntry).toContain('background: transparent;');
    expect(fileEntry).toContain('&:hover {');
    expect(fileEntry).toContain('&.selected {');
  });
});
