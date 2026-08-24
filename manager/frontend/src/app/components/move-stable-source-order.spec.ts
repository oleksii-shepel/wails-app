import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('move source-order stability', () => {
  const directory = read('directory-pane.component.ts');
  const dual = read('dual-pane.component.ts');

  it('snapshots current visual order before removing moved source rows', () => {
    expect(directory).toContain('this.mutationOrder = this.visibleEntries');
    expect(directory).toContain(".filter(entry => entry.path !== '__parent__')");
  });

  it('renders source-only mutations in their preserved visual order', () => {
    expect(directory).toContain('if (this.mutationOrder) {');
    expect(directory).toContain('.map(path => byPath.get(path))');
  });

  it('does not commit/sort the source-only pane after a move', () => {
    const start = dual.indexOf("private async transferToOther(operation: 'copy' | 'move')");
    const end = dual.indexOf('private isRecycleBinPath(', start);
    const method = dual.slice(start, end);

    expect(method).toContain('source.selection?.reconcileRemovedPaths?.(');
    expect(method).not.toContain('source.selection?.commitMutationProjection?.()');
  });

  it('commits destination projection exactly after batch deltas are applied', () => {
    expect(dual).toContain(
      'destination.selection?.reconcileAddedEntries?.(destinationEntries, false)',
    );
    expect(dual).toContain(
      'destination.selection?.commitMutationProjection?.()',
    );
  });

  it('suppresses the watcher refresh that merely confirms a known source removal', () => {
    expect(directory).toContain('this.suppressNextObservedRefresh = true;');
    expect(directory).toContain('if (this.suppressNextObservedRefresh)');
  });
});
