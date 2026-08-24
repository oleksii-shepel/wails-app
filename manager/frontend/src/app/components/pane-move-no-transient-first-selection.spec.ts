import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('natural move selection timing', () => {
  const directory = read('directory-pane.component.ts');
  const dual = read('dual-pane.component.ts');

  it('reconciles moved source rows before the watcher refresh', () => {
    expect(dual).toContain('source.selection?.reconcileRemovedPaths?.(');
    expect(dual).toContain('affected.map(item => item.source)');
    expect(directory).toContain('reconcileRemovedPaths(paths: readonly string[])');
  });

  it('never seeds entry zero during a preserveCursor stream', () => {
    expect(directory).toContain('if (preserveCursor) {');
    expect(directory).toContain('if (!preserved) return;');
    expect(directory).toContain('this.streamPreserveCursor');
  });

  it('does not clear cursor/selection at the beginning of preserveCursor refresh', () => {
    expect(directory).toContain('if (!restoreSession && !preserveCursor) {');
    expect(directory).toContain('if (!preserveCursor) {\n      this.entries = [];');
  });

  it('does not use first visible entry as preserveCursor fallback', () => {
    expect(directory).toContain(
      'options.preserveCursor\n          ? this.recoverCursorNearCurrentPosition()',
    );
  });
  it('keeps local row reconciliation as an optional pane-selection capability', () => {
    const selection = readFileSync(
      new URL('../../../../../packages/core/frontend/src/contracts/panes/selection.ts', import.meta.url),
      'utf8',
    );
    expect(selection).toContain(
      'reconcileRemovedPaths?(paths: readonly string[]): void;',
    );
  });

});
