import { readFileSync } from 'node:fs';

describe('pane peer port convergence', () => {
  const port = readFileSync(new URL('./pane-peer.port.ts', import.meta.url), 'utf8');
  const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');
  const filePane = readFileSync(new URL('./file-pane.component.ts', import.meta.url), 'utf8');

  it('keeps peer cooperation narrower than the old pane contract', () => {
    expect(port).toContain('interface PanePeerPort');
    expect(port).toContain('peerNavigation');
    expect(port).toContain('peerSelection');
    expect(port).not.toContain('deleteSelected');
    expect(port).not.toContain('openViewer');
    expect(port).not.toContain('reloadWorkspaceTabs');
  });

  it('is implemented by both current and target pane implementations', () => {
    expect(host).toContain('implements PanePeerPort');
    expect(filePane).toContain('PanePeerPort');
    expect(host).toContain('get peerNavigation()');
    expect(filePane).toContain('get peerNavigation()');
  });

  it('gives FilePaneComponent real peer navigation and selection behavior', () => {
    expect(filePane).toContain('canNavigateToPeerLocation');
    expect(filePane).toContain('navigateToPeerLocation');
    expect(filePane).toContain('selectPeerByNameForPeer');
    expect(filePane).toContain('selectPeerPaths');
  });
});