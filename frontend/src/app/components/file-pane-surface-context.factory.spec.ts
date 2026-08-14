import { FilePaneSurfaceContextFactory, type FilePaneSurfaceContextSource } from './file-pane-surface-context.factory';
import type { TabInfo } from '../shared/protocol-enhanced';

function tab(surfaceId: string, kind: TabInfo['kind'] = 'dir'): TabInfo {
  return {
    id: `tab-${surfaceId}`,
    title: surfaceId,
    path: '',
    kind,
    surfaceId,
    isActive: true,
    isPinned: false,
    createdAt: 1,
  };
}

function source(active: TabInfo): FilePaneSurfaceContextSource {
  return {
    paneId: 'left',
    currentTabId: active.id,
    tabs: [active],
    dirVm: {} as any,
    sizeManagerVm: {} as any,
  };
}

describe('FilePaneSurfaceContextFactory', () => {
  const factory = new FilePaneSurfaceContextFactory();

  it('creates a minimal extensible context for a foreign surface', () => {
    const ctx = factory.getSurfaceContextForTab(source(tab('console')), tab('console'));

    expect(ctx.surfaceId).toBe('console');
    expect(ctx.paneId).toBe('left');
    expect((ctx as any).dirVm).toBeUndefined();
    expect((ctx as any).viewer).toBeUndefined();
  });

  it('keeps built-in dir context strongly populated', () => {
    const active = tab('dir');
    const host = source(active);
    const ctx = factory.getSurfaceContextForTab(host, active) as any;

    expect(ctx.surfaceId).toBe('dir');
    expect(ctx.kind).toBe('dir');
    expect(ctx.dirVm).toBe(host.dirVm);
  });

  it('uses surfaceId rather than legacy kind as the primary identity', () => {
    const active = tab('console', 'viewer');
    const ctx = factory.getSurfaceContextForTab(source(active), active);

    expect(ctx.surfaceId).toBe('console');
    expect((ctx as any).viewer).toBeUndefined();
  });
});
