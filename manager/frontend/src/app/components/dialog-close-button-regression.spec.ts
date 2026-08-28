import { readFileSync } from 'node:fs';

const styles = readFileSync(new URL('../../styles.scss', import.meta.url), 'utf8');

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('standard dialog close control', () => {
  it('uses the larger shared close glyph and a neutral pressed state', () => {
    expect(styles).toContain('--dialog-close-size: 32px;');
    expect(styles).toContain('width: var(--dialog-close-size');
    expect(styles).toContain('height: var(--dialog-close-size');
    expect(styles).toContain('font: 400 24px/1');
    expect(styles).toContain('.dialog-close-btn:active');
    expect(styles).toContain('background: var(--vsc-list-hover-background');
    expect(styles).toContain('box-shadow: none !important;');
  });

  it('never renders the dialog close sign larger than the application sign', () => {
    const closeSize = Number(styles.match(/--dialog-close-size:\s*(\d+)px/)?.[1] ?? 0);
    const appSignSize = Number(styles.match(/--app-sign-size:\s*(\d+)px/)?.[1] ?? 0);
    expect(closeSize).toBeGreaterThan(0);
    expect(appSignSize).toBeGreaterThan(0);
    expect(closeSize).toBeLessThanOrEqual(appSignSize);
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
