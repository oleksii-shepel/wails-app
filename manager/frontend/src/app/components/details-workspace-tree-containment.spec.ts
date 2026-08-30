import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('Details workspace folder tree and containment', () => {
  const dual = read('dual-pane.component.ts');
  const host = read('tool-workspace-host.component.ts');

  it('builds Details as a folder-tree / inspector split', () => {
    expect(dual).toContain("'details-tree'");
    expect(dual).toContain("'details-workspace'");
    expect(dual).toContain("splitLayout(\n          'horizontal'");
    expect(dual).toContain('          25,');
    expect(host).toContain('[class.details-workspace-split]="isDetailsWorkspaceSplit(layout)"');
    expect(host).toContain('grid-template-columns: minmax(220px, 25%) minmax(0, 1fr);');
  });

  it('selects folders in the tree and retargets the details surface', () => {
    expect(host).toContain('(selectionChange)="onTreeSelectionChange(surface, $event)"');
    expect(host).toContain("(navigate)=\"emitNamedAction(surface, 'select-folder', $event)\"");
    expect(dual).toContain('selectDetailsTreeFolder');
    expect(dual).toContain("this.patchLiveSurface(detailsSurface.id, { path, selectedPath: '', error: '' });");
  });

  it('reuses the standard workspace tree and details views', () => {
    expect(host).toContain('class="details-tree-surface tree-view"');
    expect(host).toContain('class="details-surface details-inspector list view-details"');
    expect(host).toContain("[currentView]=\"'tree'\"");
    expect(host).toContain("[treeContentMode]=\"'folders'\"");
    expect(host).toContain("[currentView]=\"'details'\"");
    expect(host).toContain('<app-pane-view-host');
  });

  it('shows selected-folder metadata and immediate containment in the large inspector', () => {
    expect(dual).toContain('containment: entries.map');
    expect(host).toContain('<app-pane-view-host');
    expect(host).toContain("[currentView]=\"'details'\"");
    expect(host).toContain("[entries]=\"detailsWorkspaceEntries(surface)\"");
    expect(host).toContain('selectedCursorPath(surface)');
    expect(host).toContain('selectedPathSet(surface)');
  });

  it('uses the normal pane cursor language and compact tree geometry', () => {
    expect(host).toContain('[selectedFiles]="treeSelectedPathSet(surface)"');
    expect(host).toContain('[cursorPath]="treeCursorPath(surface)"');
    expect(dual).toContain('currentPath: info.path');
  });
});
