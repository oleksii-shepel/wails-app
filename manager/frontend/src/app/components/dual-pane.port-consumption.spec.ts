import { readFileSync } from 'node:fs';

describe('DualPaneComponent capability consumption', () => {
  const source = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

  it('routes pane actions through narrow capability ports', () => {
    expect(source).toContain('navigation?.refresh(');
    expect(source).toContain('selection?.selectPaths(');
    expect(source).toContain('commands?.deleteSelected(');
    expect(source).toContain('tabCommands?.reloadWorkspaceTabs(');
    expect(source).toContain('viewerCommands?.openSurfaceTab(');
  });

  it('does not use the old flat forwarding surface for migrated capabilities', () => {
    const forbidden = [
      '.refresh(',
      '.navigateTo(',
      '.selectPaths(',
      '.deleteSelected(',
      '.reloadWorkspaceTabs(',
      '.openSurfaceTab(',
    ];

    for (const call of forbidden) {
      expect(source).not.toMatch(new RegExp(`(?:activePane|otherPane|leftPane|rightPane)\\?*\\.${call.slice(1).replace(/[.*+?^${}()|[\\]\\]/g, '\\$&')}`));
    }
  });
});
