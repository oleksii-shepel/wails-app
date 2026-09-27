import { readFileSync } from 'node:fs';

describe('pane virtual scroll hot path', () => {
  const list = readFileSync(new URL('./pane-list.component.ts', import.meta.url), 'utf8');
  const details = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');
  const grid = readFileSync(new URL('./pane-grid.component.ts', import.meta.url), 'utf8');
  const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');

  it('never repopulates VirtualCollection with a full directory from a scroll-frame update', () => {
    expect(details).not.toContain('populate(0, this.entries, this.entries.length)');
    expect(grid).not.toContain('populate(0, this.entries, this.entries.length)');
    expect(list).not.toContain('populate(0, this.cachedVirtualEntries, itemCount)');

    expect(details).toContain('this.entries.slice(window.start, window.end)');
    expect(grid).toContain('this.entries.slice(window.start, window.end)');
    expect(list).toContain('this.cachedVirtualEntries.slice(window.start, window.end)');
  });

  it('keeps rendered windows bounded without making VirtualCollection a second Grid source of truth', () => {
    expect(details).toContain('Array.from(this.virtualCollection.renderedItems())');
    expect(list).toContain('Array.from(this.virtualCollection.renderedItems())');
    expect(grid).toContain('this.entries.slice(this.virtualStart, this.virtualEnd)');
  });

  it('does not eagerly build full-directory cursor indexes in list/details/grid', () => {
    expect(details).not.toContain('pathToEntryIndex = new Map');
    expect(grid).not.toContain('pathToEntryIndex = new Map');
    expect(list).not.toContain('pathToVirtualIndex = new Map');
  });

  it('does not replace directory tab state or mark the whole pane dirty per shared-view scroll frame', () => {
    const handler = directory.slice(
      directory.indexOf('onSharedDetailsViewportChange(): void'),
      directory.indexOf('onContentScroll(event: Event): void'),
    );
    expect(handler).toContain('active.scrollTop = Math.max(0, scrollTop)');
    expect(handler).not.toContain('this.tabs = this.tabs.map');
    expect(handler).not.toContain('this.cdr.markForCheck()');
  });
});
