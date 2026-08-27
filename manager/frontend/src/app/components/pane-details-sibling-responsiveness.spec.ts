import { readFileSync } from 'node:fs';

describe('Details sibling-pane responsiveness', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('does not populate the virtual collection with the entire directory on scroll', () => {
    expect(source).not.toContain('populate(0, this.entries, this.entries.length)');
    expect(source).toContain('this.entries.slice(window.start, window.end)');
    expect(source).toContain('window.enabled ? window.start : 0');
  });

  it('does not eagerly build a full path index when Details mounts', () => {
    expect(source).not.toContain('pathToEntryIndex = new Map');
    expect(source).not.toContain('this.entries.forEach((entry, index)');
    expect(source).toContain('private findEntryIndex(path: string): number');
  });

  it('materializes visible rows from the resident virtual slice', () => {
    expect(source).toContain('Array.from(this.virtualCollection.renderedItems())');
  });
});
