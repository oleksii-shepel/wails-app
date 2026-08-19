import { readFileSync } from 'node:fs';

describe('Grid thumbnail pipeline integration', () => {
  const grid = readFileSync(
    new URL('./file-grid.component.ts', import.meta.url),
    'utf8',
  );
  const entry = readFileSync(
    new URL('./file-entry.component.ts', import.meta.url),
    'utf8',
  );
  const thumbnails = readFileSync(
    new URL('../services/thumbnail-pipeline.service.ts', import.meta.url),
    'utf8',
  );

  it('keeps grid virtualization row-aligned through VirtualCollection', () => {
    expect(grid).toContain('const firstVisibleRow');
    expect(grid).toContain('visibleStart = firstVisibleRow * columns');
    expect(grid).toContain('const renderWindow = this.virtualCollection.window');
  });

  it('prioritizes visible then overscan then idle thumbnails', () => {
    const visible = grid.indexOf("'visible'");
    const overscan = grid.indexOf("'overscan'");
    const idle = grid.indexOf("'idle'");
    expect(visible).toBeGreaterThan(-1);
    expect(overscan).toBeGreaterThan(visible);
    expect(idle).toBeGreaterThan(overscan);
  });

  it('updates thumbnail priority on scroll even when the retained DOM window stays fixed', () => {
    const scrollStart = grid.indexOf('private onScroll');
    const scrollEnd = grid.indexOf('private resetScroll', scrollStart);
    const body = grid.slice(scrollStart, scrollEnd);
    expect(body).toContain('this.refreshThumbnailPriorities()');
  });

  it('does not fetch thumbnails inside FileEntryComponent', () => {
    expect(entry).toContain('@Input() thumbnailSrc');
    expect(entry).not.toContain('getImageThumbnailURL');
    expect(entry).not.toContain('fetch(');
  });

  it('cancels stale thumbnail work and bounds decoded images', () => {
    expect(thumbnails).toContain('beginGeneration(scope: string)');
    expect(thumbnails).toContain('controller.abort()');
    expect(thumbnails).toContain('maxDecodedEntries = 192');
    expect(thumbnails).toContain('maxDecodedBytes = 96 * 1024 * 1024');
    expect(thumbnails).toContain('URL.revokeObjectURL');
  });
});