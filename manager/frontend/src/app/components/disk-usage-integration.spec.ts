import { readFileSync } from 'node:fs';

const dualPane = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const directoryPane = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');

describe('disk usage utility integration', () => {
  it('opens Disk Usage as the dedicated size-manager surface', () => {
    expect(dualPane).toContain("async openDiskUsage(): Promise<void>");
    expect(dualPane).toContain("pane.viewerCommands?.openSurfaceTab(");
    expect(dualPane).toContain("'size-manager'");
    expect(dualPane).toContain("descriptor(COMMAND.diskUsage, 'Disk Usage', 'Tools', () => void this.openDiskUsage()");
    expect(dualPane).not.toContain("descriptor(COMMAND.diskUsage, 'Disk Usage', 'Tools', () => this.openTools('disk')");
  });

  it('adapts the generic surface tab to the size-manager VM contract', () => {
    expect(directoryPane).toContain("if (tab.surfaceId !== 'size-manager') return baseContext;");
    expect(directoryPane).toContain('const sizeContext: SizeManagerSurfaceContext');
    expect(directoryPane).toContain('analysisPath: tab.path');
    expect(directoryPane).toContain('onAnalyse: path =>');
    expect(directoryPane).toContain('void this.closeTab(tab.id)');
  });
});
