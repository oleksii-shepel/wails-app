import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { PluginPlatformService } from './plugin-platform.service';
import type { PluginRegistryAPI } from '../plugin/plugin-registry-api';
import type { PluginPackage } from '../plugin/plugin-package';

function registry(): PluginRegistryAPI {
  return {
    registerInput: fn(), unregisterInput: fn(),
    registerCommand: fn(), unregisterCommand: fn(),
    registerViewer: fn(), unregisterViewer: fn(),
    registerSurface: fn(), unregisterSurface: fn(),
    registerToolbarAction: fn(), unregisterToolbarAction: fn(),
    registerContextMenuItem: fn(), unregisterContextMenuItem: fn(),
    registerFileCategory: fn(), unregisterFileCategory: fn(),
  };
}

function pkg(activate = fn()): PluginPackage {
  return {
    manifest: {
      schemaVersion: 1,
      id: 'demo.tools',
      name: 'Demo Tools',
      version: '1.0.0',
      apiVersion: '^1.0.0',
      permissions: ['commands'],
    },
    activate,
  };
}

describe('PluginPlatformService', () => {
  it('installs and activates trusted packages', async () => {
    const activate = fn();
    const platform = new PluginPlatformService(registry() as any);
    await platform.install(pkg(activate), { trusted: true, activate: true });

    expect(activate).toHaveBeenCalledTimes(1);
    expect(platform.get('demo.tools')?.state).toBe('active');
  });

  it('refuses activation until requested permissions are granted', async () => {
    const platform = new PluginPlatformService(registry() as any);
    await platform.install(pkg());
    await expectAsync(platform.activate('demo.tools')).toBeRejectedWithError(/missing permissions/);
  });

  it('disposes all scoped contributions if activation fails', async () => {
    const target = registry();
    const platform = new PluginPlatformService(target as any);
    const broken = pkg(({ api }: any) => {
      api.registerCommand({ id: 'plugin.demo.run', title: 'Run', execute: () => undefined });
      throw new Error('boom');
    });
    await platform.install(broken, { trusted: true });

    await expectAsync(platform.activate('demo.tools')).toBeRejectedWithError(/boom/);
    expect(target.unregisterCommand).toHaveBeenCalledWith('plugin.demo.run');
    expect(platform.get('demo.tools')?.state).toBe('failed');
  });

  it('activates packages by declared activation event', async () => {
    const activate = fn();
    const p = pkg(activate);
    p.manifest = { ...p.manifest, activationEvents: ['onCommand:hash.sha256'] };
    const platform = new PluginPlatformService(registry() as any);
    await platform.install(p, { trusted: true });

    expect(await platform.activateByEvent('onStartup')).toEqual([]);
    expect(await platform.activateByEvent('onCommand:hash.sha256')).toEqual(['demo.tools']);
  });
});