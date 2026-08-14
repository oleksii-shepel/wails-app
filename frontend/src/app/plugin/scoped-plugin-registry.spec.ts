import { describe, expect, it, vi } from 'vitest';
import { ScopedPluginRegistry } from './scoped-plugin-registry';
import type { PluginRegistryAPI } from './plugin-registry-api';

function host(): PluginRegistryAPI {
  return {
    registerInput: vi.fn(), unregisterInput: vi.fn(),
    registerCommand: vi.fn(), unregisterCommand: vi.fn(),
    registerViewer: vi.fn(), unregisterViewer: vi.fn(),
    registerSurface: vi.fn(), unregisterSurface: vi.fn(),
    registerToolbarAction: vi.fn(), unregisterToolbarAction: vi.fn(),
    registerContextMenuItem: vi.fn(), unregisterContextMenuItem: vi.fn(),
    registerSettingsPanel: vi.fn(), unregisterSettingsPanel: vi.fn(),
    registerFileCategory: vi.fn(), unregisterFileCategory: vi.fn(),
  };
}

describe('ScopedPluginRegistry', () => {
  it('automatically unwinds registered contributions', () => {
    const target = host();
    const scope = new ScopedPluginRegistry('demo', target, new Set(['commands', 'toolbar']));
    scope.registerCommand({ id: 'plugin.demo.run', title: 'Run', execute: () => undefined });
    scope.registerToolbarAction({ id: 'run', commandId: 'plugin.demo.run', score: () => 1 });

    scope.dispose();

    expect(target.unregisterToolbarAction).toHaveBeenCalledWith('run');
    expect(target.unregisterCommand).toHaveBeenCalledWith('plugin.demo.run');
  });

  it('blocks undeclared contribution permissions', () => {
    const scope = new ScopedPluginRegistry('demo', host(), new Set(['commands']));
    expect(() => scope.registerToolbarAction({ id: 'run', score: () => 1 })).toThrow('toolbar');
  });

  it('forces input ownership to the active plugin', () => {
    const target = host();
    const scope = new ScopedPluginRegistry('demo', target, new Set(['commands', 'keybindings']));
    scope.registerInput({ pluginId: 'somebody-else', commands: [], bindings: [] });
    expect(target.registerInput).toHaveBeenCalledWith(expect.objectContaining({ pluginId: 'demo' }));
  });
});
