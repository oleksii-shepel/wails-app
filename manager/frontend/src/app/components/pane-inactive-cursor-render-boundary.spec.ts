import { readFileSync } from 'node:fs';

describe('inactive pane cursor renderer boundary', () => {
  const read = (name: string) =>
    readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

  it('List never forwards cursor=true while inactive', () => {
    const source = read('pane-list.component.ts');
    expect(source).toContain('[cursor]="paneActive && row.cursor"');
    expect(source).toContain('[cursor]="paneActive && parentEntry.path === cursorPath"');
    expect(source).not.toContain('[cursor]="row.cursor"');
  });

  it('Grid never paints or forwards cursor while inactive', () => {
    const source = read('pane-grid.component.ts');
    expect(source).toContain('[class.cursor]="paneActive && isCursor(entry)"');
    expect(source).toContain('[cursor]="paneActive && isCursor(entry)"');
  });

  it('Tree never paints focus cursor while inactive', () => {
    const source = read('pane-tree.component.ts');
    expect(source).toContain('[class.focused]="paneActive && isFocused"');
    expect(source).toContain('[class.focused-node]="paneActive && isFocusedNode(node)"');
  });

  it('Details never paints cursor while inactive', () => {
    const source = read('pane-details.component.ts');
    expect(source).toContain('[class.cursor]="paneActive && row.cursor"');
    expect(source)
      .not.toContain(':host-context(.pane:not(.active-pane)) .details-row.cursor');
  });
});
