import { ResultsPanelService } from './results-panel.service';

describe('ResultsPanelService refresh lifecycle', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it('updates entries in place while keeping the results path stable', async () => {
    const service = new ResultsPanelService();
    const panel = service.create('Live', [], { kind: 'plugin-collection' });

    service.registerRefresher(panel.path, async () => ({
      entries: [{ path: '/new.txt', name: 'new.txt' } as any],
    }));

    const refreshed = await service.refresh(panel.path);

    expect(refreshed?.path).toBe(panel.path);
    expect(refreshed?.entries[0].path).toBe('/new.txt');
  });

  it('removes runtime refreshers when the collection is removed', () => {
    const service = new ResultsPanelService();
    const panel = service.create('Live', []);
    service.registerRefresher(panel.path, async () => []);

    service.remove(panel.path);

    expect(service.canRefresh(panel.path)).toBeFalse();
  });
});