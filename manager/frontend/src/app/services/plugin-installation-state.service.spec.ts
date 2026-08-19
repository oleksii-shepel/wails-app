import type { DiscoveredPluginPackage } from './plugin-discovery.service';
import {
  buildManagedPluginPackage,
  normalizePluginPackageState,
  type PluginPackageState,
} from './plugin-installation-state.service';

const discovered: DiscoveredPluginPackage = {
  manifest: {
    schemaVersion: 1,
    id: 'example',
    name: 'Example',
    version: '1.0.0',
    permissions: ['commands', 'network'],
  },
  path: '/plugins/example',
  manifestPath: '/plugins/example/plugin.json',
  valid: true,
  errors: [],
  warnings: [],
};

function state(overrides: Partial<PluginPackageState> = {}): PluginPackageState {
  return {
    pluginId: 'example', enabled: false, trusted: false, trustedVersion: undefined, grantedPermissions: [],
    quarantined: false, lastActivationStatus: 'never', updatedAt: 1, ...overrides,
  };
}

describe('plugin installation state', () => {
  it('keeps a newly discovered plugin disabled until explicitly enabled and permitted', () => {
    const managed = buildManagedPluginPackage(discovered, state());
    expect(managed.readyToActivate).toBe(false);
    expect(managed.missingPermissions).toEqual(['commands', 'network']);
  });

  it('allows explicitly granted permissions without global trust', () => {
    const managed = buildManagedPluginPackage(discovered, state({
      enabled: true,
      grantedPermissions: ['commands', 'network'],
    }));
    expect(managed.readyToActivate).toBe(true);
  });

  it('treats trusted packages as having their requested permissions', () => {
    const managed = buildManagedPluginPackage(discovered, state({ enabled: true, trusted: true, trustedVersion: '1.0.0' }));
    expect(managed.effectiveGrantedPermissions).toEqual(['commands', 'network']);
    expect(managed.readyToActivate).toBe(true);
  });

  it('keeps quarantined packages blocked even when trusted and enabled', () => {
    const managed = buildManagedPluginPackage(discovered, state({
      enabled: true, trusted: true, trustedVersion: '1.0.0', quarantined: true,
    }));
    expect(managed.readyToActivate).toBe(false);
  });

  it('requires trust review when the trusted version does not match the discovered package', () => {
    const managed = buildManagedPluginPackage(discovered, state({
      enabled: true, trusted: true, trustedVersion: '0.9.0',
    }));
    expect(managed.readyToActivate).toBe(false);
    expect(managed.missingPermissions).toEqual(['commands', 'network']);
  });

  it('normalizes incomplete Wails state safely', () => {
    expect(normalizePluginPackageState({ pluginId: 'x', enabled: true })).toEqual(jasmine.objectContaining({
      pluginId: 'x', enabled: true, trusted: false, trustedVersion: undefined, grantedPermissions: [],
      quarantined: false, lastActivationStatus: 'never',
    }));
  });
});