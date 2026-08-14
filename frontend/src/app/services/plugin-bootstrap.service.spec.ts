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
});
