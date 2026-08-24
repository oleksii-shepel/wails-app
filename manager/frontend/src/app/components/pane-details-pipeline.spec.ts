import { readFileSync } from 'node:fs';

describe('Details view pipeline integration', () => {
  const details = readFileSync(
    new URL('./pane-details.component.ts', import.meta.url),
    'utf8',
  );
  const container = readFileSync(
    new URL('./file-view-container.component.ts', import.meta.url),
    'utf8',
  );

  it('does not rebuild row VMs during column resize mousemove', () => {
    const moveStart = details.indexOf('private onColumnResizeMove');
    const moveEnd = details.indexOf('private onColumnResizeEnd', moveStart);
    const body = details.slice(moveStart, moveEnd);

    expect(body).toContain('setColumnWidthCss');
    expect(body).not.toContain('rebuildVisibleRowVms');
    expect(body).not.toContain('markForCheck');
  });

  it('routes header sorting outward instead of locally sorting the full entries array', () => {
    const start = container.indexOf('onSortChange(sort:');
    const end = container.indexOf('// ── Event passthrough', start);
    const body = container.slice(start, end);

    expect(body).toContain('this.sortChange.emit(sort)');
    expect(body).not.toContain('sortController.sortEntries');
  });

  it('uses compiled column VMs and formatted cell cache', () => {
    expect(details).toContain('formattedValueCache');
    expect(details).toContain('activeColumnVms');
    expect(details).toContain('columnConfiguration');
  });
});
