import { readFileSync } from 'node:fs';

describe('pane peer port convergence', () => {
  const port = readFileSync(new URL('./pane-peer.port.ts', import.meta.url), 'utf8');
  const filePane = readFileSync(new URL('./file-pane.component.ts', import.meta.url), 'utf8');

  it('keeps peer cooperation narrow', () => {
    expect(port).toContain('interface PanePeerPort');
    expect(port).toContain('peerNavigation');
    expect(port).toContain('peerSelection');
    expect(port).not.toContain('deleteSelected');
    expect(port).not.toContain('openViewer');
    expect(port).not.toContain('reloadWorkspaceTabs');
  });

  it('is implemented directly by the surviving pane', () => {
    expect(filePane).toContain('PanePeerPort');
    expect(filePane).toContain('get peerNavigation()');
    expect(filePane).toContain('get peerSelection()');
  });

  it('gives FilePaneComponent real peer navigation and selection behavior', () => {
    expect(filePane).toContain('canNavigateToPeerLocation');
    expect(filePane).toContain('navigateToPeerLocation');
    expect(filePane).toContain('selectPeerByNameForPeer');
    expect(filePane).toContain('selectPeerPaths');
  });
});
