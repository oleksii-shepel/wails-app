import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('Details workspace row actions', () => {
  const host = read('tool-workspace-host.component.ts');
  const dual = read('dual-pane.component.ts');
  const paneDetails = read('pane-details.component.ts');
  const service = readFileSync(
    new URL('../services/builtin-workspace-surface-actions.service.ts', import.meta.url),
    'utf8',
  );

  it('binds F2 to an inline rename command in the workspace context', () => {
    expect(host).toContain("rename: 'reassembly.workspace.details.rename'");
    expect(host).toContain("id: 'reassembly.workspace.details.rename.f2'");
    expect(host).toContain("steps: [{ key: 'f2' }]");
    expect(host).toContain("when: '!typing'");
    expect(host).toContain('this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.rename');
  });

  it('exposes the row menu and field edits through keyboard bindings', () => {
    expect(host).toContain("rowMenu: 'reassembly.workspace.details.rowMenu'");
    expect(host).toContain("attributes: 'reassembly.workspace.details.attributes'");
    expect(host).toContain("modifiedTime: 'reassembly.workspace.details.modifiedTime'");
    expect(host).toContain("id: 'reassembly.workspace.details.rowMenu.ctrlPeriod'");
    expect(host).toContain("key: '.', modifiers: ['ctrl']");
    expect(host).toContain("id: 'reassembly.workspace.details.attributes.ctrlShiftA'");
    expect(host).toContain("id: 'reassembly.workspace.details.modifiedTime.ctrlM'");
    expect(host).toContain('openDetailsWorkspaceRowMenu(): boolean');
    expect(host).toContain("editDetailsWorkspaceCursorField(action: 'attributes' | 'modified-time'): boolean");
    expect(host).toContain('new MouseEvent(\'contextmenu\'');
    expect(host).toContain('if (!path || path === PARENT_ENTRY_PATH) return false;');
    expect(dual).toContain("'Ctrl+M', canMutate");
    expect(dual).toContain("'Ctrl+Shift+A', canMutate");
  });

  it('delegates inline rename to the shared pane view host', () => {
    expect(host).toContain('startDetailsWorkspaceRename(path?: string): boolean');
    expect(host).toContain('host.startRenamePath(path)');
    expect(host).toContain('host.startRenameSelected()');
    expect(host).toContain('revealDetailsWorkspacePath(path: string): void');
    expect(host).toContain('refreshDetailsWorkspaceTreeNode(path: string): void');
  });

  it('focuses and selects the inline rename input once editing starts', () => {
    expect(paneDetails).toContain("querySelector<HTMLInputElement>('.details-rename-input')");
    expect(paneDetails).toContain('input?.focus();');
    expect(paneDetails).toContain('input?.select();');
  });

  it('routes the inspector rename output through a dedicated entry action', () => {
    expect(host).toContain('(rename)="onDetailsRename(surface, $event)"');
    expect(host).toContain("this.emitNamedAction(surface, 'rename-entry', event);");
    expect(dual).toContain("event.action === 'rename-entry'");
    expect(dual).toContain('renameDetailsWorkspaceEntry');
    expect(dual).toContain('exactDestination: true');
    expect(dual).toContain("this.patchLiveSurface(detailsSurfaceId, { selectedPath: destination, error: '' });");
    expect(dual).toContain('this.toolWorkspaceHost?.refreshDetailsWorkspaceTreeNode(parent);');
    expect(dual).toContain('this.toolWorkspaceHost?.revealDetailsWorkspacePath(destination);');
  });

  it('guards inline rename against the parent row, archives, and re-entrancy', () => {
    expect(dual).toContain("if (!entry?.path || entry.path === '__parent__') return;");
    expect(dual).toContain("if (surface.state?.['inArchive'] === true) return;");
    expect(dual).toContain('private detailsWorkspaceRenameBusy = false;');
    expect(dual).toContain('if (surface.state?.[\'busy\'] === true || this.detailsWorkspaceRenameBusy) return;');
  });

  it('opens a context menu on rows and tree nodes that targets the clicked entry', () => {
    expect(dual).toContain('(entryContextMenu)="openDetailsWorkspaceEntryMenu($event)"');
    expect(host).toContain('@Output() entryContextMenu = new EventEmitter<{');
    expect(host).toContain('(contextMenu)="onDetailsEntryContextMenu(surface, $event)"');
    expect(host).toContain('(contextMenu)="onTreeEntryContextMenu(surface, $event)"');
    expect(dual).toContain('openDetailsWorkspaceEntryMenu(request: {');
    expect(dual).toContain('reassembly.context.detailsWorkspace.');
    expect(dual).toContain('this.contextMenuActions.set(token, run)');
    expect(dual).toContain("title: isParent ? 'Parent folder' : (entry.name || 'Item')");
    expect(dual).toContain('const canMutate = !isParent && !inArchive && !busy;');
  });

  it('runs menu mutations against the clicked path through the action registry', () => {
    expect(dual).toContain("this.toolWorkspaceHost?.startDetailsWorkspaceRename(entry.path);");
    expect(dual).toContain("payload: { path: entry.path }");
    expect(service).toContain('private targetPath(context: WorkspaceSurfaceActionContext): string');
    expect(service).toContain("id: 'builtin.workspace.details.copyPath'");
    expect(service).toContain("action: 'copy-path'");
  });

  it('formats the inspector Attributes column like the Permissions column', () => {
    expect(host).toContain("import { formatAttributeFlags } from '../utils/attribute-flags'");
    expect(host).toContain('formatter: value => formatAttributeFlags(value)');
    expect(paneDetails).toContain("import { formatAttributeFlags } from '../utils/attribute-flags'");
    expect(paneDetails).toContain('formatter: (value) => formatAttributeFlags(value)');
  });

  it('keeps toolbar actions scoped to the inspected folder after retargeting', () => {
    expect(service).toContain('const renamedInspectedFolder = path === String(surface.state?.[\'path\'] ?? \'\');');
    expect(service).toContain('if (renamedInspectedFolder) {');
  });
});
