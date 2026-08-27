import { readFileSync } from 'node:fs';

describe('inactive pane pointer activation ordering', () => {
  const paneFiles = [
    'directory-pane.component.ts',
    'bin-pane.component.ts',
    'registry-pane.component.ts',
    'autorun-pane.component.ts',
  ];

  for (const name of paneFiles) {
    it(`${name}: suppresses focus activation until pointer selection commits`, () => {
      const source = readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

      expect(source).toContain('(focusin)="onPaneFocusIn()"');
      expect(source).toContain('(mousedown)="onPanePointerDown()"');
      expect(source).toContain('if (this.pointerActivationPending) return;');
      expect(source).toContain('private activateAfterPointerSelection(): void');

      const start = source.indexOf('selectFromPointer(entry: FileInfo');
      const end = source.indexOf('\n  openViewer(', start);
      const method = source.slice(start, end);
      expect(method.indexOf('this.cursorPath = entry.path;')).toBeGreaterThan(-1);
      expect(method.indexOf('this.activateAfterPointerSelection();'))
        .toBeGreaterThan(method.indexOf('this.cursorPath = entry.path;'));
    });
  }

  it('keeps inactive selection state intact', () => {
    const source = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
    expect(source).toContain('[class.selected]="selectedPaths.has(entry.path)"');
  });
});
