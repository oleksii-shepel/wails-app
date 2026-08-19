import { DiscoveryHistoryService } from './discovery-history.service';

describe('DiscoveryHistoryService', () => {
  beforeEach(() => localStorage.clear());

  it('records complete search specifications, not query text alone', () => {
    const service = new DiscoveryHistoryService();
    service.record({
      rootPath: 's3://bucket/project',
      query: '*.json',
      recursive: true,
      type: 'files',
      sort: 'name',
    }, 12);

    expect(service.history()[0]).toEqual(jasmine.objectContaining({
      rootPath: 's3://bucket/project',
      query: '*.json',
      recursive: true,
      type: 'files',
      sort: 'name',
      resultCount: 12,
    }));
  });

  it('saves reusable provider-aware searches', () => {
    const service = new DiscoveryHistoryService();
    const saved = service.save('JSON reports', {
      rootPath: 'gdrive://team/reports',
      query: '*.json',
      recursive: true,
      type: 'files',
      sort: 'path',
    });

    expect(service.saved()[0].id).toBe(saved.id);
    expect(service.saved()[0].rootPath).toBe('gdrive://team/reports');
  });
});