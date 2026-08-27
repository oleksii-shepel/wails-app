import { readFileSync } from 'node:fs';

describe('inactive pane selection is preserved visually', () => {
  const styles = readFileSync(new URL('../../../styles.scss', import.meta.url), 'utf8');

  it('paints selected entries with the inactive-selection token', () => {
    expect(styles).toContain('app-pane-list .file-entry.selected');
    expect(styles).toContain('app-pane-grid .grid-item.selected');
    expect(styles).toContain('app-pane-tree .tree-row.selected');
    expect(styles).toContain('app-pane-details .details-row.selected');
    expect(styles).toContain('--vsc-list-inactive-selection');
  });

  it('does not make inactive selected rows transparent', () => {
    const selectionBlockStart = styles.indexOf(
      '.pane:not(.active-pane) app-pane-list .file-entry.selected'
    );
    const cursorBlockStart = styles.indexOf('/* Cursor/focus belongs exclusively');
    const block = styles.slice(selectionBlockStart, cursorBlockStart);
    expect(block).not.toContain('background: transparent');
  });

  it('still suppresses inactive cursor/focus decoration', () => {
    expect(styles).toContain('app-pane-list .file-entry.cursor');
    expect(styles).toContain('app-pane-grid .grid-item.cursor');
    expect(styles).toContain('app-pane-tree .tree-row.focused-node');
    expect(styles).toContain('app-pane-details .details-row.cursor');
    expect(styles).toContain('box-shadow: none !important;');
    expect(styles).toContain('outline: none !important;');
  });
});
