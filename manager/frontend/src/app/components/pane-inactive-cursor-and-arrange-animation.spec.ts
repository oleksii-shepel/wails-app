import { readFileSync } from 'node:fs';

describe('Arrange animation and inactive pane cursor', () => {
  const read = (name: string) =>
    readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

  it('disables animation only for Arrange', () => {
    const arrange = read('pane-arrange-controls.component.ts');
    const dropdown = read('dropdown-container.component.ts');

    expect(arrange).toContain('[animate]="false"');
    expect(dropdown).toContain('@Input() animate = true;');
    expect(dropdown).toContain('dropdown-menu-wrapper--no-animation');
    expect(dropdown).toContain('animation: none;');
  });

  it('does not render List cursor in an inactive pane', () => {
    const source = read('pane-list.component.ts');
    expect(source).toContain('[cursor]="paneActive && row.cursor"');
    expect(source).toContain('[cursor]="paneActive && parentEntry.path === cursorPath"');
  });

  it('does not render Grid cursor in an inactive pane', () => {
    expect(read('pane-grid.component.ts'))
      .toContain('[class.cursor]="paneActive && isCursor(entry)"');
  });

  it('does not render Tree focused cursor in an inactive pane', () => {
    expect(read('pane-tree.component.ts'))
      .toContain('[class.focused-node]="paneActive && isFocusedNode(node)"');
  });

  it('does not render Details cursor in an inactive pane', () => {
    const source = read('pane-details.component.ts');
    expect(source).toContain('[class.cursor]="paneActive && row.cursor"');
    expect(source).not.toContain(':host-context(.pane:not(.active-pane)) .details-row.cursor');
  });
});
