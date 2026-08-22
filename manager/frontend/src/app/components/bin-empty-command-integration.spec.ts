import { readFileSync } from 'node:fs';

const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');

describe('Recycle Bin Empty Bin command integration', () => {
  it('registers Empty Bin as a canonical command backed by ApiService.emptyTrash', () => {
    expect(dual).toContain("emptyBin: 'reassembly.trash.emptyBin'");
    expect(dual).toContain("descriptor(\n      COMMAND.emptyBin");
    expect(dual).toContain('await this.api.emptyTrash();');
    expect(dual).toContain("confirmText: 'Empty Bin'");
  });

  it('targets the pane supplied by toolbar invocation context', () => {
    expect(dual).toContain("const paneId = invocation?.context?.['paneId'];");
    expect(dual).toContain('this.recycleBinHasEntries(this.paneForInvocation(invocation))');
  });

  it('publishes the canonical command on bin:// toolbars', () => {
    expect(host).toContain("const commandId = 'reassembly.trash.emptyBin';");
    expect(host).toContain("label: 'Empty Bin'");
    expect(host).toContain("source: 'toolbar' as const");
    expect(host).toContain('this.inputEngine.dispatchCommandById(commandId, invocation)');
  });

  it('offers Empty Bin on both item and background bin:// context menus', () => {
    const placements = dual.match(/this\.menuCommand\(COMMAND\.emptyBin, 'Empty Bin'\)/g) ?? [];
    expect(placements.length).toBeGreaterThanOrEqual(2);
  });

  it('refreshes all visible bin panes after emptying', () => {
    expect(dual).toContain(".filter((pane): pane is PaneHostComponent => this.isRecycleBinPane(pane))");
    expect(dual).toContain('await Promise.all(binPanes.map(pane => pane.navigation?.refresh()));');
  });
});
