import { describe, expect, it, vi } from 'vitest';
import { PluginBootstrapService } from './plugin-bootstrap.service';
import type { PluginLifecycleCoordinatorService } from './plugin-lifecycle-coordinator.service';

function coordinator(overrides: Partial<PluginLifecycleCoordinatorService> = {}): PluginLifecycleCoordinatorService {
  return {
    sync: vi.fn(async () => []),
    activateStartup: vi.fn(async () => []),
    shutdown: vi.fn(async () => []),
    ...overrides,
  } as unknown as PluginLifecycleCoordinatorService;
}

describe('PluginBootstrapService', () => {
  it('initializes only once and activates startup plugins after reconciliation', async () => {
    const lifecycle = coordinator();
    const service = new PluginBootstrapService(lifecycle);

    const [a, b] = await Promise.all([service.initialize(), service.initialize()]);

    expect(a.status).toBe('ready');
    expect(b.status).toBe('ready');
    expect(lifecycle.sync).toHaveBeenCalledTimes(1);
    expect(lifecycle.activateStartup).toHaveBeenCalledTimes(1);
  });

  it('degrades without throwing when one startup plugin fails', async () => {
    const lifecycle = coordinator({
      activateStartup: vi.fn(async () => [
        { pluginId: 'good', activated: true },
        { pluginId: 'bad', activated: false, error: 'boom' },
      ]),
    });
    const service = new PluginBootstrapService(lifecycle);

    const snapshot = await service.initialize();

    expect(snapshot.status).toBe('degraded');
    expect(snapshot.errors).toEqual(['bad: boom']);
  });

  it('keeps the host usable when discovery itself fails', async () => {
    const lifecycle = coordinator({ sync: vi.fn(async () => { throw new Error('discovery failed'); }) });
    const service = new PluginBootstrapService(lifecycle);

    await expect(service.initialize()).resolves.toMatchObject({ status: 'degraded' });
    expect(service.snapshot.errors).toEqual(['discovery failed']);
  });

  it('records shutdown errors but continues teardown', async () => {
    const lifecycle = coordinator({
      shutdown: vi.fn(async () => [
        { pluginId: 'bad', error: 'deactivate failed' },
        { pluginId: 'good' },
      ]),
    });
    const service = new PluginBootstrapService(lifecycle);
    await service.initialize();

    await service.shutdown();

    expect(service.snapshot.status).toBe('stopped');
    expect(service.snapshot.errors).toContain('bad: deactivate failed');
  });
  it('starts backend command discovery before package reconciliation and stops it at shutdown', async () => {
    const order: string[] = [];
    const lifecycle = coordinator({
      sync: vi.fn(async () => { order.push('sync'); return []; }),
      shutdown: vi.fn(async () => { order.push('shutdown'); return []; }),
    });
    const backend = {
      start: vi.fn(async (selection: () => string[]) => { order.push(`backend:${selection().join(',')}`); }),
      stop: vi.fn(() => { order.push('backend-stop'); }),
    };
    const service = new PluginBootstrapService(lifecycle, backend as any);

    await service.initialize(() => ['C:/work/a.txt']);
    await service.shutdown();

    expect(order.slice(0, 2)).toEqual(['backend:C:/work/a.txt', 'sync']);
    expect(order.slice(-2)).toEqual(['shutdown', 'backend-stop']);
  });

});