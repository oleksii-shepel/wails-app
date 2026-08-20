import { readFileSync } from 'node:fs';

describe('pane host narrowing', () => {
  const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');
  const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

  it('does not expose the legacy lastAction seam to the shell', () => {
    expect(host).not.toContain('get lastAction()');
    expect(host).not.toContain('set lastAction(');
    expect(dual).not.toContain('.lastAction');
  });

  it('keeps PaneHost as a capability adapter instead of a full forwarding facade', () => {
    expect(host).toContain('get navigation(): PaneNavigation | undefined');
    expect(host).toContain('get selection(): PaneSelection | undefined');
    expect(host).toContain('get commands(): PaneCommands | undefined');
    expect(host).toContain('get tabCommands(): PaneTabs | undefined');
    expect(host).toContain('get viewerCommands(): PaneViewerCommands | undefined');

    expect(host).not.toContain('copySelected(): void');
    expect(host).not.toContain('deleteSelected(permanent?');
    expect(host).not.toContain('openSurfaceTab(surfaceId: string');
    expect(host).not.toContain('reloadWorkspaceTabs(preferredTabId?');
  });

  it('keeps dual-pane behavior on narrow capabilities', () => {
    expect(dual).toContain('source.selection?.cursorEntry');
    expect(dual).toContain('pane.viewerCommands?.openViewer(entry)');
    expect(dual).toContain('pane.commands?.deleteSelected()');
    expect(dual).not.toContain('sourcePane.recoverVisibleCursor()');
    expect(dual).not.toContain('pane.openViewer(entry)');
  });
});
