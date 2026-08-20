import { readFileSync } from 'node:fs';

const manager = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

describe('DualPaneComponent pane port finalization', () => {
  it('routes pane collaboration through narrow ports', () => {
    expect(manager).toContain('.navigation?.navigateLinkedLocation(');
    expect(manager).toContain('.selection?.selectComparisonItem(');
    expect(manager).toContain('.tabCommands?.reloadWorkspaceTabs(');
    expect(manager).toContain('.viewerCommands?.openSurfaceTab(');
    expect(manager).toContain('.commands?.transferArchiveSelectionTo(');
    expect(manager).toContain('.presentation?.applyMirroredPresentation(');
    expect(manager).toContain('.viewport?.syncViewportToPeer(');
  });

  it('does not call the legacy flat pane behavior surface', () => {
    const forbidden = [
      '.navigateLinkedLocation(',
      '.canNavigateToLocation(',
      '.currentLocationAvailable(',
      '.jumpNavigationHistory(',
      '.selectComparisonItem(',
      '.selectPeerByName(',
      '.selectPaths(',
      '.reloadWorkspaceTabs(',
      '.exportActiveTabId(',
      '.exportTabCount(',
      '.openSurfaceTab(',
      '.openQuickView(',
      '.openActiveViewerExternally(',
      '.extractSelectedTo(',
      '.transferArchiveSelectionTo(',
      '.panePresentation(',
      '.viewportSnapshot(',
      '.syncViewportToPeer(',
      '.applyMirroredPresentation(',
    ];

    for (const token of forbidden) {
      const unported = manager
        .split('\n')
        .filter(line => line.includes(token))
        .filter(line => !line.includes('?.') && !line.includes('.navigation?.') && !line.includes('.selection?.') && !line.includes('.tabCommands?.') && !line.includes('.viewerCommands?.') && !line.includes('.commands?.') && !line.includes('.presentation?.') && !line.includes('.viewport?.'));
      expect(unported).withContext(token).toEqual([]);
    }
  });
});
