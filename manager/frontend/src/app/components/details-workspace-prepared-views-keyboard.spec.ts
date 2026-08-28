import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

const host = read('tool-workspace-host.component.ts');
const dual = read('dual-pane.component.ts');
const viewHost = read('pane-view-host.component.ts');
const tree = read('pane-tree.component.ts');

describe('Details workspace focused inspection UX', () => {
  it('offers exactly the three workspace-specific prepared views', () => {
    expect(host).toContain("type DetailsWorkspaceViewPresetId = 'security' | 'timestamps' | 'filesystem';");
    expect(host).toContain("{ id: 'security', label: 'Security' }");
    expect(host).toContain("{ id: 'timestamps', label: 'Timestamps' }");
    expect(host).toContain("{ id: 'filesystem', label: 'Filesystem' }");
    expect(host).not.toContain("{ id: 'brief', label: 'Brief' }");
    expect(host).not.toContain("{ id: 'standard', label: 'Standard' }");
    expect(host).not.toContain("{ id: 'full', label: 'Full' }");
  });

  it('keeps the specialized column sets exact and rematerializes columns', () => {
    expect(host).toContain("security: ['name', 'owner', 'permissions', 'attributes']");
    expect(host).toContain("timestamps: ['name', 'created', 'modified', 'accessed']");
    expect(host).toContain("filesystem: ['name', 'type', 'attributes', 'isHidden', 'isSymlink']");
    expect(host).toContain('.map(column => ({ ...column }))');
  });

  it('owns workspace navigation instead of borrowing normal pane commands', () => {
    expect(host).toContain("nextPane: 'reassembly.workspace.details.nextPane'");
    expect(host).toContain("previousPane: 'reassembly.workspace.details.previousPane'");
    expect(host).toContain("cursorUp: 'reassembly.workspace.details.cursorUp'");
    expect(host).toContain("cursorDown: 'reassembly.workspace.details.cursorDown'");
    expect(host).toContain("open: 'reassembly.workspace.details.open'");
    expect(host).toContain("modifiers: ['shift']");
    expect(host).toContain("key: 'arrowup'");
    expect(host).toContain("key: 'arrowdown'");
    expect(host).toContain("key: 'pageup'");
    expect(host).toContain("key: 'pagedown'");
  });

  it('activates the Details input context only for the Details split', () => {
    expect(host).toContain('syncDetailsWorkspaceInputContext()');
    expect(host).toContain('this.isDetailsWorkspaceSplit(this.presentation.layout)');
    expect(host).toContain('this.input.deactivateContext(DETAILS_WORKSPACE_INPUT_CONTEXT);');
  });

  it('cycles Tree and Inspector with Tab while preserving explicit pane state', () => {
    expect(host).toContain("const order: readonly DetailsWorkspacePane[] = ['tree', 'details'];");
    expect(host).toContain('this.focusDetailsWorkspacePane(event.shiftKey ? -1 : 1);');
    expect(host).toContain("window.addEventListener('keydown', this.detailsWorkspaceTabFallback, { capture: true });");
    expect(host).toContain("if (event.key !== 'Tab' || event.defaultPrevented) return;");
    expect(host).toContain('data-details-workspace-pane="tree"');
    expect(host).toContain('data-details-workspace-pane="details"');
    expect(host).toContain('focusTarget.focus();');
  });

  it('uses Up/Down/Home/End/Page keys inside the active region', () => {
    expect(host).toContain("this.detailsWorkspacePane === 'tree'");
    expect(host).toContain('navigateDetailsWorkspaceTree(move)');
    expect(host).toContain('navigateDetailsWorkspaceList(move)');
    expect(host).toContain("case 'first': return 0;");
    expect(host).toContain("case 'last': return length - 1;");
    expect(host).toContain("case 'page-up'");
    expect(host).toContain("case 'page-down'");
  });

  it('moves the Tree cursor without opening containment until Enter', () => {
    expect(tree).toContain('getFocusedNodePath(): string | null');
    expect(viewHost).toContain('getTreeFocusedPath(): string | null');
    expect(host).toContain('host.focusTreeNode(next.path);');
    expect(host).toContain('this.select(surface, next);');
    expect(host).not.toContain("host.focusTreeNode(next.path);\n    this.emitNamedAction(surface, 'select-folder', next);");
    expect(host).toContain("this.emitNamedAction(surface, 'select-folder', entry);");
  });

  it('navigates Details rows in the exact rendered sort order', () => {
    expect(viewHost).toContain('getSortedEntries(): FileInfo[]');
    expect(host).toContain('host?.getSortedEntries() ?? this.fileEntries(surface)');
  });

  it('opens from a file with that file selected in the containing-folder inspector', () => {
    expect(dual).toContain("const initialSelectedPath = entry.type === FileType.DIRECTORY ? '' : entry.path;");
    expect(dual).toContain('selectedPath: initialSelectedPath');
    expect(host).toContain("const statePath = this.text(surface, 'selectedPath');");
  });

  it('keeps the folder tree anchored while the inspector moves through descendants', () => {
    expect(dual).toContain("const rootPath = String(treeSurface?.state?.['rootPath'] ?? '');");
    expect(dual).toContain("if (!rootPath || info.path === rootPath)");
    expect(dual).toContain("treePatch['entries'] = entries.map(entry => ({ ...entry }));");
  });

  it('uses latest-wins semantics for rapid Tree navigation', () => {
    expect(dual).toContain('private detailsWorkspaceRefreshSequence = 0;');
    expect(dual).toContain('const refreshSequence = ++this.detailsWorkspaceRefreshSequence;');
    expect(dual).toContain('if (refreshSequence !== this.detailsWorkspaceRefreshSequence) return;');
    expect(dual).toContain("this.patchLiveSurface(detailsSurface.id, { path, selectedPath: '', error: '' });");
  });
  it('promotes a valid Details row cursor when focus moves from the tree', () => {
    expect(host).toContain("if (paneId === 'details') {");
    expect(host).toContain("const cursor = current ?? entries[0];");
    expect(host).toContain('this.select(surface, cursor);');
    expect(host).toContain('this.cdr.detectChanges();');
    expect(host).toContain('row?.scrollIntoView');
  });

  it('keeps Tree and Details cursors independent until real navigation occurs', () => {
    expect(host).toContain('[entries]="detailsWorkspaceEntries(surface)"');
    expect(host).toContain("name: '..'");
    expect(host).not.toContain('treePath === parentPath');
    expect(dual).toContain('navigateDetailsWorkspacePath(event.surface, targetPath)');
    expect(dual).toContain('pathWithinWorkspaceRoot(path, rootPath)');
  });


  it('keeps the synthetic parent row selectable in Details', () => {
    expect(host).toContain("surface.surfaceId === 'details-workspace'");
    expect(host).toContain('this.detailsWorkspaceEntries(surface)');
    expect(host).toContain('const validPaths = new Set(selectableEntries.map(entry => entry.path));');
  });

  it('keeps the parent row until the selected Windows drive root', () => {
    expect(host).toContain('isDetailsWorkspaceDriveRoot(currentPath)');
    expect(host).toContain('/^[a-z]:$/i.test(normalized)');
    expect(host).toContain('/^filesystem:\/\/[a-z]$/i.test(normalized)');
    expect(dual).toContain("filter(entry => entry.path !== '__parent__')");
    expect(dual).toContain("this.parentLocation(String(event.surface.state?.['path'] ?? ''))");
  });

  it('keeps Details workspace hidden-folder visibility aligned with the manager setting', () => {
    expect(dual).toContain('[showHidden]="showHidden"');
    expect(host).toContain('@Input() showHidden = false;');
    expect(host).toContain('[showHidden]="showHidden"');
    expect(dual).toContain('this.api.listDirectory(path, this.showHidden)');
    expect(dual).toContain("this.findWorkspaceSurfaceByKind('details-workspace')");
  });

  it('keeps directory links eligible for the folders-only tree pipeline', () => {
    expect(host).toContain('[treeContentMode]="\'folders\'"');
  });

  it('handles Backspace as scoped parent navigation in the Details side', () => {
    expect(host).toContain("id: 'reassembly.workspace.details.parent'");
    expect(host).toContain("steps: [{ key: 'backspace' }]");
    expect(host).toContain('(keydown.backspace)="onDetailsWorkspaceBackspace($event)"');
    expect(host).toContain('navigateDetailsWorkspaceParent()');
    expect(host).toContain('isDetailsWorkspaceDriveRoot(currentPath)');
    expect(host).toContain("this.emitNamedAction(detailsSurface, 'open'");
  });

});
