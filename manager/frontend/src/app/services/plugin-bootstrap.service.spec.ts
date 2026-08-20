import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { PluginBootstrapService } from './plugin-bootstrap.service';
import type { PluginLifecycleCoordinatorService } from './plugin-lifecycle-coordinator.service';
import type { PluginInstallationStateService } from './plugin-installation-state.service';

function coordinator(overrides: Partial<PluginLifecycleCoordinatorService> = {}): PluginLifecycleCoordinatorService {
  return {
    sync: fn(async () => []),
    activateStartup: fn(async () => []),
    shutdown: fn(async () => []),
    ...overrides,
  } as unknown as PluginLifecycleCoordinatorService;
}

describe('PluginBootstrapService', () => {
  function installation(overrides: Partial<PluginInstallationStateService> = {}): PluginInstallationStateService {
    return {
      ensureStartupHostPrivilege: fn(async () => false),
      ...overrides,
    } as unknown as PluginInstallationStateService;
  }

  it('initializes only once and activates startup plugins after reconciliation', async () => {
    const lifecycle = coordinator();
    const service = new PluginBootstrapService(lifecycle, installation());

    const [a, b] = await Promise.all([service.initialize(), service.initialize()]);

    expect(a.status).toBe('ready');
    expect(b.status).toBe('ready');
    expect(lifecycle.sync).toHaveBeenCalledTimes(1);
    expect(lifecycle.activateStartup).toHaveBeenCalledTimes(1);
  });

  it('degrades without throwing when one startup plugin fails', async () => {
    const lifecycle = coordinator({
      activateStartup: fn(async () => [
        { pluginId: 'good', activated: true },
        { pluginId: 'bad', activated: false, error: 'boom' },
      ]),
    });
    const service = new PluginBootstrapService(lifecycle, installation());

    const snapshot = await service.initialize();

    expect(snapshot.status).toBe('degraded');
    expect(snapshot.errors).toEqual(['bad: boom']);
  });

  it('keeps the host usable when discovery itself fails', async () => {
    const lifecycle = coordinator({ sync: fn(async () => { throw new Error('discovery failed'); }) });
    const service = new PluginBootstrapService(lifecycle, installation());

    const result = await service.initialize();
    expect(result).toEqual(jasmine.objectContaining({ status: 'degraded' }));
    expect(service.snapshot.errors).toEqual(['discovery failed']);
  });

  it('records shutdown errors but continues teardown', async () => {
    const lifecycle = coordinator({
      shutdown: fn(async () => [
        { pluginId: 'bad', error: 'deactivate failed' },
        { pluginId: 'good' },
      ]),
    });
    const service = new PluginBootstrapService(lifecycle, installation());
    await service.initialize();

    await service.shutdown();

    expect(service.snapshot.status).toBe('stopped');
    expect(service.snapshot.errors).toContain('bad: deactivate failed');
  });
  it('starts backend command discovery before package reconciliation and stops it at shutdown', async () => {
    const order: string[] = [];
    const lifecycle = coordinator({
      sync: fn(async () => { order.push('sync'); return []; }),
      shutdown: fn(async () => { order.push('shutdown'); return []; }),
    });
    const backend = {
      start: fn(async (selection: () => string[]) => { order.push(`backend:${selection().join(',')}`); }),
      stop: fn(() => { order.push('backend-stop'); }),
    };
    const service = new PluginBootstrapService(lifecycle, installation(), backend as any);

    await service.initialize(() => ['C:/work/a.txt']);
    await service.shutdown();

    expect(order.slice(0, 2)).toEqual(['backend:C:/work/a.txt', 'sync']);
    expect(order.slice(-2)).toEqual(['shutdown', 'backend-stop']);
  });

  it('relaunches elevated before plugin bootstrap when startup policy requires it', async () => {
    const lifecycle = coordinator({
      sync: fn(async () => { throw new Error('sync should not run after relaunch'); }),
    });
    const state = installation({
      ensureStartupHostPrivilege: fn(async () => true),
    });
    const backend = {
      start: fn(async () => undefined),
      stop: fn(() => undefined),
    };
    const service = new PluginBootstrapService(lifecycle, state, backend as any);

    const snapshot = await service.initialize();

    expect(snapshot.status).toBe('initializing');
    expect(state.ensureStartupHostPrivilege).toHaveBeenCalled();
    expect(backend.start).not.toHaveBeenCalled();
    expect(lifecycle.sync).not.toHaveBeenCalled();
    expect(lifecycle.activateStartup).not.toHaveBeenCalled();
  });

});
