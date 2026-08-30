import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('Details workspace batch targets', () => {
  const host = read('tool-workspace-host.component.ts');
  const dual = read('dual-pane.component.ts');
  const actions = read('../services/builtin-workspace-surface-actions.service.ts');

  it('keeps local selection and the Selection Basket as explicit targets', () => {
    expect(host).toContain("type DetailsWorkspaceTarget = 'selection' | 'basket';");
    expect(host).toContain('Selection ({{ detailsWorkspaceSelectionCount(surface) }})');
    expect(host).toContain('Basket ({{ selectionBasketPaths.length }})');
    expect(host).toContain("detailsWorkspaceTarget: DetailsWorkspaceTarget = 'selection';");
    expect(host).toContain("if (value === 'basket' && this.selectionBasketPaths.length === 0) return;");
  });

  it('adds the current Details multi-selection to the existing basket', () => {
    expect(host).toContain('@Output() addSelectionToBasket = new EventEmitter<string[]>();');
    expect(host).toContain('this.addSelectionToBasket.emit(paths);');
    expect(dual).toContain('(addSelectionToBasket)="addDetailsWorkspaceSelectionToBasket($event)"');
    expect(dual).toContain("this.selectionBasketService.add(paths, 'details-workspace');");
  });

  it('emits backward-compatible path plus the complete batch path list', () => {
    expect(host).toContain('path: paths[0],');
    expect(host).toContain('paths,');
    expect(host).toContain('target: this.detailsWorkspaceTarget,');
    expect(actions).toContain('private targetPaths(context: WorkspaceSurfaceActionContext): string[]');
    expect(actions).toContain("return this.targetPaths(context)[0] ?? '';");
  });

  it('applies modified time to every selected target', () => {
    expect(actions).toContain('Promise.all(paths.map(path => this.api.setModifiedTime(path, timestamp)))');
    expect(actions).toContain('`Modified Time — ${paths.length} items`');
  });

  it('uses patch semantics for Windows batch attributes', () => {
    expect(actions).toContain('+readonly, -readonly');
    expect(actions).toContain("patches.get('hidden') ?? !!current.hidden");
    expect(actions).toContain("patches.get('archive') ?? !!current.archive");
    expect(actions).toContain('Unspecified attributes stay unchanged.');
  });

  it('applies one requested Unix mode across the batch', () => {
    expect(actions).toContain("const defaultMode = modes.every(mode => mode === modes[0]) ? modes[0] : '';");
    expect(actions).toContain('Promise.all(paths.map(path => this.api.setPathAttributes(path, { unixMode })))');
  });
});
