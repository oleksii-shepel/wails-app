import { fn } from '../testing/jasmine-test-helpers';
import { PluginLifecycleCoordinatorService } from './plugin-lifecycle-coordinator.service';
import type {
  ManagedPluginPackage,
  PluginPackageState,
} from './plugin-installation-state.service';
import type { PluginRuntimeSnapshot } from './plugin-platform.service';

function state(
  overrides: Partial<PluginPackageState> = {},
): PluginPackageState {
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

function managed(
  overrides: Partial<ManagedPluginPackage> = {},
): ManagedPluginPackage {
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
    list: fn(() => Array.from(runtimes.values())),
    get: fn((id: string) => runtimes.get(id)),
    isInstalled: fn((id: string) => runtimes.has(id)),
    hasLoader: fn(() => hasLoader),
    installFromSource: fn(async (source: any, options: any) => {
      runtimes.set('example', {
        manifest: managed().discovered.manifest,
        state: 'installed',
        grantedPermissions: [...options.grantedPermissions],
        missingPermissions: [],
        source,
      });
    }),
    activate: fn(async (id: string) => {
      const runtime = runtimes.get(id);
      if (runtime) {
        runtimes.set(id, { ...runtime, state: 'active' });
      }
    }),
    deactivate: fn(async () => undefined),
    uninstall: fn(async (id: string) => {
      runtimes.delete(id);
    }),
  };
}

describe('PluginLifecycleCoordinatorService setTrusted', () => {
  it('updates trust without forcing explicit enablement', async () => {
    const trustedState = state({
      enabled: false,
      trusted: true,
      trustedVersion: '1.0.0',
    });
    const pkg = managed({
      state: trustedState,
      readyToActivate: false,
    });
    const installation = {
      setTrusted: fn(async () => trustedState),
      refresh: fn(async () => [pkg]),
    } as any;
    const platform = createPlatform(false);
    const coordinator = new PluginLifecycleCoordinatorService(
      installation,
      platform as any,
    );

    const result = await coordinator.setTrusted('example', true);

    expect(installation.setTrusted).toHaveBeenCalledWith('example', true);
    expect(result.enabled).toBe(false);
  });

  it('does not try to toggle enablement when trust is revoked', async () => {
    const untrustedState = state({
      enabled: false,
      trusted: false,
    });
    const pkg = managed({
      state: untrustedState,
      effectiveGrantedPermissions: [],
      missingPermissions: ['commands'],
      readyToActivate: false,
    });
    const installation = {
      setTrusted: fn(async () => untrustedState),
      refresh: fn(async () => [pkg]),
    } as any;
    const platform = createPlatform(false);
    const coordinator = new PluginLifecycleCoordinatorService(
      installation,
      platform as any,
    );

    await coordinator.setTrusted('example', false);

    expect(installation.setTrusted).toHaveBeenCalledWith('example', false);
  });
});
