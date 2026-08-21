import { readFileSync } from 'node:fs';

const directory = readFileSync(
  new URL('./directory-pane.component.ts', import.meta.url),
  'utf8',
);
const network = readFileSync(
  new URL('./network-pane.component.ts', import.meta.url),
  'utf8',
);

function block(source: string, start: string, end: string): string {
  const from = source.indexOf(start);
  if (from < 0) throw new Error(`Missing ${start}`);
  const to = source.indexOf(end, from);
  if (to < 0) throw new Error(`Missing ${end}`);
  return source.slice(from, to + end.length);
}

describe('Network pane Directory clone', () => {
  it('keeps Directory styles byte-for-byte identical', () => {
    expect(block(network, '  styles: [`', '  `]'))
      .toBe(block(directory, '  styles: [`', '  `]'));
  });

  it('keeps Directory Details DOM classes', () => {
    for (const token of [
      'class="details-header"',
      'class="list"',
      'class="entry"',
      'class="icon"',
      'class="name"',
      'class="kind"',
      'class="size"',
      'class="modified"',
      'class="pane-footer"',
    ]) {
      expect(network).toContain(token);
    }
  });

  it('adapts the Directory cells to Network semantics', () => {
    expect(network).toContain("networkField(entry, 'kind')");
    expect(network).toContain("networkField(entry, 'status')");
    expect(network).toContain('networkEndpoints(entry)');
    expect(network).toContain('systemFields?: Record<string, unknown>');
  });

  it('is user-facing Details only', () => {
    expect(network).toContain("viewMode: DirectoryView = 'details';");
    expect(network).not.toContain("setViewMode('list')");
    expect(network).not.toContain("setViewMode('grid')");
    expect(network).not.toContain("setViewMode('tree')");
  });

  it('activates Network entries through the active panel source', () => {
    const method = network.slice(
      network.indexOf('openEntry(entry: FileInfo): void'),
      network.indexOf('onEntryDragStart(', network.indexOf('openEntry(entry: FileInfo): void')),
    );
    expect(method).toContain('activePanelSource.source.activateEntry?.(');
    expect(method).not.toContain('openViewer');
  });
});
