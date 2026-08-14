import { describe, expect, it } from 'vitest';
import { SurfaceRegistryService } from './surface-registry.service';

describe('SurfaceRegistryService capabilities', () => {
  it('exposes directory command capabilities', () => {
    const registry = new SurfaceRegistryService();
    const capabilities = registry.getCapabilities({
      tab: { id: 'dir', title: 'Dir', path: '/', kind: 'dir', surfaceId: 'dir', isActive: true, isPinned: false, createdAt: 1 },
    });

    expect(capabilities.has('navigate')).toBe(true);
    expect(capabilities.has('selection')).toBe(true);
    expect(capabilities.has('fileOperations')).toBe(true);
    expect(capabilities.has('preview')).toBe(true);
    expect(capabilities.has('viewerControls')).toBe(false);
  });

  it('lets a foreign surface define its own command capabilities', () => {
    const registry = new SurfaceRegistryService();
    registry.register({
      id: 'console',
      capabilities: [],
      score: () => 0,
      load: () => class ConsoleSurface {},
    });

    const capabilities = registry.getCapabilities({
      tab: { id: 'console', title: 'Console', path: '', kind: 'dir', surfaceId: 'console', isActive: true, isPinned: false, createdAt: 1 },
    });

    expect(capabilities.size).toBe(0);
  });

  it('prefers surfaceId capabilities over legacy kind', () => {
    const registry = new SurfaceRegistryService();
    registry.register({
      id: 'search-results',
      capabilities: ['selection', 'preview'],
      score: () => 0,
      load: () => class SearchResultsSurface {},
    });

    const capabilities = registry.getCapabilities({
      tab: { id: 'search', title: 'Search', path: 'search://', kind: 'viewer', surfaceId: 'search-results', isActive: true, isPinned: false, createdAt: 1 },
    });

    expect(capabilities.has('selection')).toBe(true);
    expect(capabilities.has('preview')).toBe(true);
    expect(capabilities.has('viewerControls')).toBe(false);
  });
});
