import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

const host = read('tool-workspace-host.component.ts');
const dual = read('dual-pane.component.ts');

describe('Details workspace specialized views and keyboard navigation', () => {
  it('offers exactly the three workspace-specific prepared views', () => {
    expect(host).toContain("type DetailsWorkspaceViewPresetId = 'security' | 'timestamps' | 'filesystem';");
    expect(host).toContain("{ id: 'security', label: 'Security' }");
    expect(host).toContain("{ id: 'timestamps', label: 'Timestamps' }");
    expect(host).toContain("{ id: 'filesystem', label: 'Filesystem' }");
    expect(host).not.toContain("{ id: 'brief', label: 'Brief' }");
    expect(host).not.toContain("{ id: 'standard', label: 'Standard' }");
    expect(host).not.toContain("{ id: 'full', label: 'Full' }");
  });

  it('keeps the specialized column sets exact', () => {
    expect(host).toContain("security: ['name', 'owner', 'permissions', 'attributes']");
    expect(host).toContain("timestamps: ['name', 'created', 'modified', 'accessed']");
    expect(host).toContain("filesystem: ['name', 'type', 'attributes', 'isHidden', 'isSymlink']");
  });

  it('materializes a new Details column array on every preset change', () => {
    expect(host).toContain('detailsWorkspaceColumns: PaneDetailsColumn[] = [];');
    expect(host).toContain('this.applyDetailsWorkspaceColumns();');
    expect(host).toContain('.map(column => ({ ...column }))');
    expect(host).toContain('this.cdr.markForCheck();');
  });

  it('owns Tab and Shift+Tab in a higher-priority workspace input context', () => {
    expect(host).toContain("const DETAILS_WORKSPACE_INPUT_CONTEXT = 'reassembly.workspace.details';");
    expect(host).toContain("priority: 180");
    expect(host).toContain("source: 'workspace'");
    expect(host).toContain("id: 'reassembly.workspace.details.switchPane.tab'");
    expect(host).toContain("id: 'reassembly.workspace.details.switchPane.shiftTab'");
    expect(host).toContain("modifiers: ['shift']");
    expect(host).toContain('this.input.activateContext(DETAILS_WORKSPACE_INPUT_CONTEXT);');
    expect(host).toContain('(keydown.tab)=\"onDetailsWorkspaceTab($event)\"');
  });

  it('routes the pane-switch command into the Details workspace first', () => {
    expect(dual).toContain('#toolWorkspaceHost');
    expect(dual).toContain("@ViewChild('toolWorkspaceHost') toolWorkspaceHost?: ToolWorkspaceHostComponent;");
    expect(dual).toContain('this.toolWorkspaceHost?.switchDetailsWorkspacePane()');
    expect(host).toContain('switchDetailsWorkspacePane(): boolean');
  });

  it('moves to the opposite Tree/Inspector side and focuses it', () => {
    expect(host).toContain("const next: DetailsWorkspacePane = current === 'tree' ? 'details' : 'tree';");
    expect(host).toContain('data-details-workspace-pane="tree"');
    expect(host).toContain('data-details-workspace-pane="details"');
    expect(host).toContain('focusTarget.focus();');
  });
});
