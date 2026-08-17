import { ResultsPanelService } from './results-panel.service';

describe('ResultsPanelService discovery persistence', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it('pins ordinary result panels for cross-session discovery', () => {
    const service = new ResultsPanelService();
    const panel = service.create('Reports', [], {
      origin: 'global-search',
      kind: 'search',
    });

    service.setPinned(panel.path, true);

    expect(service.pinned().map(item => item.path)).toContain(panel.path);
  });

  it('does not persist plugin-owned collection snapshots', () => {
    const service = new ResultsPanelService();
    const panel = service.createPluginCollection('demo','reports','Reports',[]);
    service.setPinned(panel.path, true);

    expect(localStorage.getItem('reassembly.resultsPanels.pinned.v1') ?? '')
      .not.toContain('pluginId');
  });
});
