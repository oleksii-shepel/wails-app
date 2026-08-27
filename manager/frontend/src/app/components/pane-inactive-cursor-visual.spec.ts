import { readFileSync } from 'node:fs';

describe('inactive pane cursor visual contract', () => {
  const styles = readFileSync(new URL('../../../styles.scss', import.meta.url), 'utf8');

  it('does not paint selection/cursor in inactive List/Grid/Tree/Details views', () => {
    for (const selector of [
      'app-pane-list .file-entry.selected',
      'app-pane-list .file-entry.cursor',
      'app-pane-grid .grid-item.selected',
      'app-pane-grid .grid-item.cursor',
      'app-pane-tree .tree-row.selected',
      'app-pane-tree .tree-row.cursor',
      'app-pane-tree .tree-row.focused-node',
      'app-pane-details .details-row.selected',
      'app-pane-details .details-row.cursor',
    ]) {
      expect(styles).withContext(selector)
        .toContain(`.pane:not(.active-pane) ${selector}`);
    }
  });

  it('keeps inactive state visual fully transparent', () => {
    expect(styles).toContain('background: transparent !important;');
    expect(styles).toContain('box-shadow: none !important;');
    expect(styles).toContain('outline: none !important;');
  });
});
