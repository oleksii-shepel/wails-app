import { describe, expect, it, vi } from 'vitest';
import { PluginLifecycleCoordinatorService } from './plugin-lifecycle-coordinator.service';
import type { ManagedPluginPackage, PluginPackageState } from './plugin-installation-state.service';
import type { PluginRuntimeSnapshot } from './plugin-platform.service';

function state(overrides: Partial<PluginPackageState> = {}): PluginPackageState {
  return {
    pluginId: 'example',
    enabled: true,
    trusted: false,
    grantedPermissions: ['commands'],
    quarantined: false,
    lastActivationStatus: 'never',
    consecutiveActivationFailures: 0,
    updatedAt: 1,
    ...overrides,
  };
}

function managed(overrides: Partial<ManagedPluginPackage> = {}): ManagedPluginPackage {
  const base: ManagedPluginPackage = {
    discovered: {
      manifest: {
        schemaVersion: 1,
        id: 'example',
        name: 'Example',
        version: '1.0.0',
        permissions: ['commands'],
        activationEvents: ['onStartup'],
      },
      path: '/plugins/example',
      manifestPath: '/plugins/example/plugin.json',
      valid: true,
      errors: [],
      warnings: [],
    },
    state: state(),
    requestedPermissions: ['commands'],
    effectiveGrantedPermissions: ['commands'],
    missingPermissions: [],
    readyToActivate: true,
  };
  return { ...base, ...overrides };
}

function createPlatform(hasLoader = false) {
  const runtimes = new Map<string, PluginRuntimeSnapshot>();
  return {
    list: vi.fn(() => Array.from(runtimes.values())),
    get: vi.fn((id: string) => runtimes.get(id)),
    isInstalled: vi.fn((id: string) => runtimes.has(id)),
    hasLoader: vi.fn(() => hasLoader),
    installFromSource: vi.fn(async (source: any, options: any) => {
      runtimes.set('example', {
        manifest: managed().discovered.manifest,
        state: 'installed',
        grantedPermissions: [...options.grantedPermissions],
        missingPermissions: [],
        source,
      });
    }),
    activate: vi.fn(async (id: string) => {
      const runtime = runtimes.get(id);
      if (runtime) runtimes.set(id, { ...runtime, state: 'active' });
    }),
    deactivate: vi.fn(async () => undefined),
    uninstall: vi.fn(async (id: string) => { runtimes.delete(id); }),
  };
}

describe('PluginLifecycleCoordinatorService', () => {
  it('keeps policy-ready packages inert when no safe loader exists', async () => {
    const pkg = managed();
    const installation = { refresh: vi.fn(async () => [pkg]) } as any;
    const platform = createPlatform(false);
    const coordinator = new PluginLifecycleCoordinatorService(installation, platform as any);

    const [snapshot] = await coordinator.sync();
    expect(snapshot.status).toBe('loader-unavailable');
    expect(platform.installFromSource).not.toHaveBeenCalled();
  });

  it('installs eligible packages without activating them during sync', async () => {
    const pkg = managed();
    const installation = { refresh: vi.fn(async () => [pkg]) } as any;
    const platform = createPlatform(true);
    const coordinator = new PluginLifecycleCoordinatorService(installation, platform as any);

    const [snapshot] = await coordinator.sync();
    expect(snapshot.status).toBe('installed');
    expect(platform.installFromSource).toHaveBeenCalledOnce();
    expect(platform.activate).not.toHaveBeenCalled();
  });

  it('removes managed runtimes when persisted policy disables the plugin', async () => {
    const enabledPkg = managed();
    const disabledPkg = managed({
      state: state({ enabled: false }),
      readyToActivate: false,
    });
    let current = [enabledPkg];
    const installation = { refresh: vi.fn(async () => current) } as any;
    const platform = createPlatform(true);
    const coordinator = new PluginLifecycleCoordinatorService(installation, platform as any);

    await coordinator.sync();
    current = [disabledPkg];
    const [snapshot] = await coordinator.sync();
    expect(snapshot.status).toBe('disabled');
    expect(platform.uninstall).toHaveBeenCalledWith('example');
  });

  it('records successful activation health', async () => {
    const pkg = managed();
    const installation = {
      refresh: vi.fn(async () => [pkg]),
      recordActivation: vi.fn(async () => state({ lastActivationStatus: 'success' })),
    } as any;
    const platform = createPlatform(true);
    const coordinator = new PluginLifecycleCoordinatorService(installation, platform as any);

    await coordinator.activate('example');
    expect(platform.activate).toHaveBeenCalledWith('example');
    expect(installation.recordActivation).toHaveBeenCalledWith('example', true);
  });
});
