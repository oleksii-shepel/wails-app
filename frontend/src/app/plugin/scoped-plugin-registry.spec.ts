import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { ScopedPluginRegistry } from './scoped-plugin-registry';
import type { PluginRegistryAPI } from './plugin-registry-api';

function host(): PluginRegistryAPI {
  return {
    registerInput: fn(), unregisterInput: fn(),
    registerCommand: fn(), unregisterCommand: fn(),
    registerViewer: fn(), unregisterViewer: fn(),
    registerSurface: fn(), unregisterSurface: fn(),
    registerToolbarAction: fn(), unregisterToolbarAction: fn(),
    registerContextMenuItem: fn(), unregisterContextMenuItem: fn(),
    registerFileCategory: fn(), unregisterFileCategory: fn(),
    registerRelatedLocation: fn(), unregisterRelatedLocation: fn(),
    createResultsCollection: fn(() => 'results://plugin-collection'), removeResultsCollection: fn(),
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

  it('owns related-location contributions and requires explicit permission', () => {
    const target = host();
    const scope = new ScopedPluginRegistry('demo', target, new Set(['relatedLocations']));
    scope.registerRelatedLocation({ id: 'demo.related', score: () => 42 });

    scope.dispose();

    expect(target.registerRelatedLocation).toHaveBeenCalledWith(expect.objectContaining({ id: 'demo.related' }));
    expect(target.unregisterRelatedLocation).toHaveBeenCalledWith('demo.related');
  });

  it('blocks related-location contributions without permission', () => {
    const scope = new ScopedPluginRegistry('demo', host(), new Set([]));
    expect(() => scope.registerRelatedLocation({ id: 'demo.related', score: () => 1 })).toThrow('relatedLocations');
  });


  it('creates plugin-owned result collections and removes them with the scope', () => {
    const target = host();
    const scope = new ScopedPluginRegistry('demo', target, new Set(['collections']));

    const path = scope.createResultsCollection({
      id: 'reports',
      title: 'Reports',
      entries: [],
    });

    expect(path).toBe('results://plugin-collection');
    expect(target.createResultsCollection).toHaveBeenCalledWith(jasmine.objectContaining({ id: 'reports', ownerPluginId: 'demo' }));

    scope.dispose();
    expect(target.removeResultsCollection).toHaveBeenCalledWith('results://plugin-collection');
  });

  it('blocks result collections without collections permission', () => {
    const scope = new ScopedPluginRegistry('demo', host(), new Set([]));
    expect(() => scope.createResultsCollection({ id: 'x', title: 'X', entries: [] })).toThrow('collections');
  });

  it('forces input ownership to the active plugin', () => {
    const target = host();
    const scope = new ScopedPluginRegistry('demo', target, new Set(['commands', 'keybindings']));
    scope.registerInput({ pluginId: 'somebody-else', commands: [], bindings: [] });
    expect(target.registerInput).toHaveBeenCalledWith(expect.objectContaining({ pluginId: 'demo' }));
  });
});