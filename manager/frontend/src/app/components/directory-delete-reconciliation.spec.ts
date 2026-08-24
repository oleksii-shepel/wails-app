import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./directory-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Directory delete reconciliation', () => {
  it('does not eagerly reload the whole directory after delete', () => {
    const start = source.indexOf(
      'async deleteSelected(forcePermanent = false)',
    );
    const end = source.indexOf(
      'async splitSelectedFile()',
      start,
    );
    const method = source.slice(start, end);

    expect(method).not.toContain(
      'await this.refresh();\n      this.focusContent();',
    );
    expect(method).toContain('this.applyOptimisticDeletedPaths(');
  });

  it('removes only deletes that can be identified as successful', () => {
    expect(source).toContain('result.skipped === 0');
    expect(source).toContain(
      '!failedPaths.some(failed => this.samePath(path, failed))',
    );
  });

  it('keeps cursor near the deleted row instead of resetting to the top', () => {
    expect(source).toContain('cursorIndexBeforeDelete');
    expect(source).toContain('preferredCursorIndex');
    expect(source).toContain('surviving.length - 1');
  });

  it('leaves disk reconciliation to the filesystem watcher', () => {
    expect(source).toContain(
      'The filesystem watcher is\n      // the authoritative reconciliation path',
    );
  });
});
