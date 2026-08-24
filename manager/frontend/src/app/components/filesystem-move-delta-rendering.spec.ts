import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('filesystem move delta rendering', () => {
  const dual = read('dual-pane.component.ts');
  const directory = read('directory-pane.component.ts');

  it('updates source and destination rows without refreshing either pane', () => {
    const start = dual.indexOf("private async transferToOther(operation: 'copy' | 'move')");
    const end = dual.indexOf('private isRecycleBinPath(', start);
    const method = dual.slice(start, end);

    expect(method).toContain('destination.selection?.reconcileAddedEntries?.(');
    expect(method).toContain('source.selection?.reconcileRemovedPaths?.(');
    expect(method).not.toContain('await destination.navigation?.refresh();');
    expect(method).not.toContain('await source.navigation?.refresh();');
  });

  it('keeps the cursor near the removed row', () => {
    expect(directory).toContain('preferredIndex');
    expect(directory).toContain('this.applyOptimisticDeletedPaths(paths, preferredIndex)');
  });

  it('adds destination entries without replacing the whole entries array', () => {
    expect(directory).toContain('this.entries.push(incoming)');
    expect(directory).toContain('this.invalidateProjection()');
  });
});
