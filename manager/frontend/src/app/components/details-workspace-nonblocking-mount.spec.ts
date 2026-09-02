import { readFileSync } from 'node:fs';

const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const tool = readFileSync(new URL('./tool-workspace-host.component.ts', import.meta.url), 'utf8');
const pane = readFileSync(new URL('./pane-view-host.component.ts', import.meta.url), 'utf8');
const lifecycle = readFileSync(new URL('./file-view-lifecycle.controller.ts', import.meta.url), 'utf8');

describe('Details workspace non-blocking mount', () => {
  it('does not eagerly select/sync the Details tree root before the first refresh', () => {
    expect(dual).toContain("rootPath: folderPath,\n                currentPath: '',\n                selectedPath: '',");
    expect(tool).toContain("[currentPath]=\"treeEntries(surface).length > 0");
  });

  it('defers tree synchronization by withholding the initial path until entries exist', () => {
    expect(pane).toContain("@Input() currentPath = '';");
    expect(lifecycle).toContain("host.currentView === 'tree' && treeRelevantChange");
  });

  it('starts Details I/O after a frame instead of in the activation task', () => {
    expect(dual).toContain("requestAnimationFrame(() => setTimeout(beginRefresh, 0));");
    expect(dual).toContain("setTimeout(beginRefresh, 0);");
  });
});
