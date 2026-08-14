import { describe, expect, it, vi } from 'vitest';
import { PluginPlatformService } from './plugin-platform.service';
import type { PluginRegistryAPI } from '../plugin/plugin-registry-api';
import type { PluginPackage } from '../plugin/plugin-package';

function registry(): PluginRegistryAPI {
  return {
    registerInput: vi.fn(), unregisterInput: vi.fn(),
    registerCommand: vi.fn(), unregisterCommand: vi.fn(),
    registerViewer: vi.fn(), unregisterViewer: vi.fn(),
    registerSurface: vi.fn(), unregisterSurface: vi.fn(),
    registerToolbarAction: vi.fn(), unregisterToolbarAction: vi.fn(),
    registerContextMenuItem: vi.fn(), unregisterContextMenuItem: vi.fn(),
    registerFileCategory: vi.fn(), unregisterFileCategory: vi.fn(),
  };
}

function pkg(activate = vi.fn()): PluginPackage {
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
    const activate = vi.fn();
    const platform = new PluginPlatformService(registry() as any);
    await platform.install(pkg(activate), { trusted: true, activate: true });

    expect(activate).toHaveBeenCalledOnce();
    expect(platform.get('demo.tools')?.state).toBe('active');
  });

  it('refuses activation until requested permissions are granted', async () => {
    const platform = new PluginPlatformService(registry() as any);
    await platform.install(pkg());
    await expect(platform.activate('demo.tools')).rejects.toThrow('missing permissions');
  });

  it('disposes all scoped contributions if activation fails', async () => {
    const target = registry();
    const platform = new PluginPlatformService(target as any);
    const broken = pkg(({ api }: any) => {
      api.registerCommand({ id: 'plugin.demo.run', title: 'Run', execute: () => undefined });
      throw new Error('boom');
    });
    await platform.install(broken, { trusted: true });

    await expect(platform.activate('demo.tools')).rejects.toThrow('boom');
    expect(target.unregisterCommand).toHaveBeenCalledWith('plugin.demo.run');
    expect(platform.get('demo.tools')?.state).toBe('failed');
  });

  it('activates packages by declared activation event', async () => {
    const activate = vi.fn();
    const p = pkg(activate);
    p.manifest = { ...p.manifest, activationEvents: ['onCommand:hash.sha256'] };
    const platform = new PluginPlatformService(registry() as any);
    await platform.install(p, { trusted: true });

    expect(await platform.activateByEvent('onStartup')).toEqual([]);
    expect(await platform.activateByEvent('onCommand:hash.sha256')).toEqual(['demo.tools']);
  });
});