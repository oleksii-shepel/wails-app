import { readFileSync } from 'node:fs';

const styles = readFileSync(new URL('../../../styles.scss', import.meta.url), 'utf8');

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('standard dialog close control', () => {
  it('uses the larger shared close glyph and a neutral pressed state', () => {
    expect(styles).toContain('width: 32px;');
    expect(styles).toContain('height: 32px;');
    expect(styles).toContain('font-size: 24px;');
    expect(styles).toContain('.dialog-close-btn:active');
    expect(styles).toContain('background: var(--vsc-list-hover-background');
    expect(styles).toContain('box-shadow: none !important;');
  });

  it('keeps tool dialog close controls on the shared class', () => {
    for (const file of [
      'operation-center.component.ts',
      'operation-journal.component.ts',
      'tools-center.component.ts',
      'pane-compare-dialog.component.ts',
      'pane-recursive-compare-dialog.component.ts',
      'pane-sync-review.component.ts',
      'trash-dialog.component.ts',
    ]) {
      expect(read(file)).toContain('class="dialog-close-btn"');
    }
  });
});
