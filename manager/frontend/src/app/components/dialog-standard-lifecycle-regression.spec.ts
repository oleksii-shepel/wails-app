import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('standard dialog close lifecycle', () => {
  it('keeps legacy Tools animations away from standard dialog classes', () => {
    const theme = read('tools-dialog-remote-theme.scss');
    expect(theme).toContain('.backdrop:not(.dialog-group)');
    expect(theme).toContain('.dialog-panel:not(.dialog-surface)');
  });

  it('uses the shared close button for operation and comparison dialogs', () => {
    for (const file of [
      'operation-center.component.ts',
      'operation-journal.component.ts',
      'pane-compare-dialog.component.ts',
      'pane-recursive-compare-dialog.component.ts',
      'pane-sync-review.component.ts',
      'trash-dialog.component.ts',
      'tools-center.component.ts',
    ]) {
      expect(read(file)).withContext(file).toContain('dialog-close-btn');
    }
  });

  it('uses the standard closing class for gate-backed compare/review/trash dialogs', () => {
    for (const file of [
      'pane-compare-dialog.component.ts',
      'pane-recursive-compare-dialog.component.ts',
      'pane-sync-review.component.ts',
      'trash-dialog.component.ts',
    ]) {
      const source = read(file);
      expect(source).withContext(file).toContain('dialog-group--closing');
      expect(source).withContext(file).toContain('dialog-surface');
    }
  });

  it('retains disk usage and tools center until exit animation completes', () => {
    for (const file of ['disk-usage-dialog.component.ts', 'tools-center.component.ts']) {
      const source = read(file);
      expect(source).withContext(file).toContain('renderVisible');
      expect(source).withContext(file).toContain('isClosing');
      expect(source).withContext(file).toContain('onAnimationEnd(event: AnimationEvent)');
    }
  });

  it('does not blur target dialog backdrops', () => {
    for (const file of [
      'operation-center.component.ts',
      'operation-journal.component.ts',
      'disk-usage-dialog.component.ts',
      'tools-center.component.ts',
      'pane-compare-dialog.component.ts',
      'pane-recursive-compare-dialog.component.ts',
      'pane-sync-review.component.ts',
      'trash-dialog.component.ts',
    ]) {
      expect(read(file)).withContext(file).not.toMatch(/backdrop-filter:\s*blur/i);
    }
  });
});
