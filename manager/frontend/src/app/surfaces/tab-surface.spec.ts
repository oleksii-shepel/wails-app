import { tabLocation, tabSurfaceId } from './tab-surface';

describe('tab surface helpers', () => {
  it('prefers stable surfaceId over legacy kind', () => {
    expect(tabSurfaceId({ surfaceId: 'console', kind: 'viewer' })).toBe('console');
  });

  it('prefers canonical location over legacy path', () => {
    expect(tabLocation({
      location: { location: 'search://?q=todo', scheme: 'search', kind: 'query' },
      path: 'filesystem://c/legacy',
    })).toBe('search://?q=todo');
  });

  it('falls back to legacy path for older workspaces', () => {
    expect(tabLocation({ path: 'filesystem://c/work', location: undefined })).toBe('filesystem://c/work');
  });
});