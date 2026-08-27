import { readFileSync } from 'node:fs';

describe('pointer selection activation atomicity', () => {
  const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
  const details = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('activates pane after child click handlers update selection/cursor', () => {
    expect(dual).toContain(`(click)="activate('left')"`);
    expect(dual).toContain(`(click)="activate('right')"`);
    expect(dual).not.toContain(`(pointerdown)="activate('left')"`);
    expect(dual).not.toContain(`(pointerdown)="activate('right')"`);
  });

  it('moves Details cursor locally before emitting selection', () => {
    const start = details.indexOf('onRowClick(entry: FileInfo, event: MouseEvent)');
    const end = details.indexOf('onRowDoubleClick', start);
    const body = details.slice(start, end);

    expect(body).toContain('this.cursorPath = entry.path;');
    expect(body).toContain('this.rebuildVisibleRowVms();');
    expect(body.indexOf('this.cursorPath = entry.path;'))
      .toBeLessThan(body.indexOf('this.selectionChange.emit'));
  });
});
