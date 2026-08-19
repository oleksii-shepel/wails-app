import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { ScopedPluginRegistry } from './scoped-plugin-registry';
import type { PluginRegistryAPI } from './plugin-registry-api';

function host(): PluginRegistryAPI {
  return {
    registerInput: fn(), unregisterInput: fn(),
    registerCommand: fn(), unregisterCommand: fn(),
    registerViewer: fn(), unregisterViewer: fn(),
    registerSurface: fn(), unregisterSurface: fn(),
    registerWorkspaceSurfaceAction: fn(), unregisterWorkspaceSurfaceAction: fn(),
    registerToolbarAction: fn(), unregisterToolbarAction: fn(),
    registerContextMenuItem: fn(), unregisterContextMenuItem: fn(),
    registerFileCategory: fn(), unregisterFileCategory: fn(),
    registerRelatedLocation: fn(), unregisterRelatedLocation: fn(),
    registerPanelSource: fn(), unregisterPanelSource: fn(),
    registerPanelColumn: fn(), unregisterPanelColumn: fn(),
    registerTheme: fn(), unregisterTheme: fn(),
    registerColumnProfile: fn(), unregisterColumnProfile: fn(),
    registerLayoutPreset: fn(), unregisterLayoutPreset: fn(),
    registerWorkspaceTemplate: fn(), unregisterWorkspaceTemplate: fn(),
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
    expect(() => scope.registerToolbarAction({ id: 'run', score: () => 1 })).toThrowError(/toolbar/);
  });

  it('owns related-location contributions and requires explicit permission', () => {
    const target = host();
    const scope = new ScopedPluginRegistry('demo', target, new Set(['relatedLocations']));
    scope.registerRelatedLocation({ id: 'demo.related', score: () => 42 });

    scope.dispose();

    expect(target.registerRelatedLocation).toHaveBeenCalledWith(jasmine.objectContaining({ id: 'demo.related' }));
    expect(target.unregisterRelatedLocation).toHaveBeenCalledWith('demo.related');
  });

  it('blocks related-location contributions without permission', () => {
    const scope = new ScopedPluginRegistry('demo', host(), new Set([]));
    expect(() => scope.registerRelatedLocation({ id: 'demo.related', score: () => 1 })).toThrowError(/relatedLocations/);
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
    expect(() => scope.createResultsCollection({ id: 'x', title: 'X', entries: [] })).toThrowError(/collections/);
  });


  it('owns panel sources and columns and cleans both up', () => {
    const target = host();
    const scope = new ScopedPluginRegistry('demo', target, new Set(['panelSources', 'columns']));
    const source = {
      id: 'demo.source', kind: 'virtual' as const, title: 'Demo',
      canHandle: () => true, capabilities: () => ({ read: true, create: false, delete: false, rename: false, trash: false }),
      list: async () => ({ location: { path: 'demo://', sourceId: 'demo.source', sourceKind: 'virtual' as const }, entries: [] }),
      getRootPath: () => 'demo://', getParentPath: () => 'demo://', joinChildPath: () => 'demo://', supportsStreaming: () => false,
    };
    const column = { id: 'demo.owner', label: 'Owner', score: () => 1, value: () => 'demo' };

    scope.registerPanelSource(source);
    scope.registerPanelColumn(column);
    scope.dispose();

    expect(target.unregisterPanelColumn).toHaveBeenCalledWith('demo.owner');
    expect(target.unregisterPanelSource).toHaveBeenCalledWith('demo.source');
  });

  it('requires explicit panel source and column permissions', () => {
    const scope = new ScopedPluginRegistry('demo', host(), new Set([]));
    expect(() => scope.registerPanelColumn({ id: 'x', label: 'X', score: () => 1, value: () => '' })).toThrowError(/columns/);
  });

  it('forces input ownership to the active plugin', () => {
    const target = host();
    const scope = new ScopedPluginRegistry('demo', target, new Set(['commands', 'keybindings']));
    scope.registerInput({ pluginId: 'somebody-else', commands: [], bindings: [] });
    expect(target.registerInput).toHaveBeenCalledWith(jasmine.objectContaining({ pluginId: 'demo' }));
  });
  it('owns customization contributions and unwinds them with the plugin scope', () => {
    const target = host();
    const scope = new ScopedPluginRegistry('demo', target, new Set(['surfaces', 'columns']));

    scope.registerTheme({ id: 'demo.theme', label: 'Demo', kind: 'dark', variables: {} });
    scope.registerColumnProfile({ id: 'demo.columns', label: 'Demo', columnIds: [] });
    scope.registerLayoutPreset({ id: 'demo.layout', label: 'Demo', createLayout: () => ({}) as any });
    scope.registerWorkspaceTemplate({ id: 'demo.workspace', label: 'Demo', create: () => ({}) as any });

    scope.dispose();

    expect(target.unregisterWorkspaceTemplate).toHaveBeenCalledWith('demo.workspace');
    expect(target.unregisterLayoutPreset).toHaveBeenCalledWith('demo.layout');
    expect(target.unregisterColumnProfile).toHaveBeenCalledWith('demo.columns');
    expect(target.unregisterTheme).toHaveBeenCalledWith('demo.theme');
  });

});