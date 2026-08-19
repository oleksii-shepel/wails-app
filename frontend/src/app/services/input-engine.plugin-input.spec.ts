import { NgZone } from '@angular/core';
import '@angular/compiler';
import { fn } from '../testing/jasmine-test-helpers';
import { CommandRegistryService } from './command-registry.service';
import { InputEngineService } from './input-engine.service';

function createEngine(): InputEngineService {
  const zone = new NgZone({ enableLongStackTrace: false });
  return new InputEngineService(zone, new CommandRegistryService(zone));
}

describe('InputEngineService plugin input ownership', () => {

  it('registers commands, contexts, and default bindings as one plugin unit', () => {
    const engine = createEngine();
    const execute = fn();

    engine.registerPluginInput({
      pluginId: 'archive-tools',
      contexts: [{
        id: 'plugin.archive-tools',
        kind: 'pane',
        priority: 25,
        description: 'Archive tools',
      }],
      commands: [{
        id: 'plugin.archive-tools.compress',
        title: 'Compress Selection',
        category: 'Plugins',
        execute,
      }],
      bindings: [{
        id: 'plugin.archive-tools.compress.ctrlAltC',
        commandId: 'plugin.archive-tools.compress',
        contextId: 'plugin.archive-tools',
        source: 'default',
        description: 'Compress Selection',
        trigger: {
          device: 'keyboard',
          chord: { steps: [{ key: 'c', modifiers: ['alt', 'ctrl'] }] },
        },
      }],
    });

    expect(engine.getCommandCatalog().find(x => x.id === 'plugin.archive-tools.compress')).toEqual(jasmine.objectContaining({
      title: 'Compress Selection',
      registered: true,
      enabled: true,
      preferredBinding: { source: 'plugin' },
    }));
    expect(engine.getContextSnapshot().some(x => x.id === 'plugin.archive-tools')).toBe(true);
    expect(engine.dispatchCommandById('plugin.archive-tools.compress')).toBe(true);
    expect(execute).toHaveBeenCalledTimes(1);
  });

  it('removes the complete plugin input unit with one unregister call', () => {
    const engine = createEngine();

    engine.registerPluginInput({
      pluginId: 'sample',
      contexts: [{ id: 'plugin.sample', kind: 'pane', priority: 20, description: 'Sample' }],
      commands: [{ id: 'plugin.sample.run', title: 'Run Sample', execute: () => undefined }],
      bindings: [{
        id: 'plugin.sample.run.f9',
        commandId: 'plugin.sample.run',
        contextId: 'plugin.sample',
        source: 'plugin',
        description: 'Run Sample',
        trigger: { device: 'keyboard', chord: { steps: [{ key: 'f9' }] } },
      }],
    });

    engine.unregisterPluginInput('sample');

    expect(engine.getCommandCatalog().some(x => x.id === 'plugin.sample.run')).toBe(false);
    expect(engine.getBinding('plugin.sample.run.f9')).toBeUndefined();
    expect(engine.getContextSnapshot().some(x => x.id === 'plugin.sample')).toBe(false);
    expect(engine.dispatchCommandById('plugin.sample.run')).toBe(false);
  });

  it('rolls back the whole unit when a plugin tries to replace host input', () => {
    const engine = createEngine();
    engine.registerCommandDescriptor({ id: 'host.command', title: 'Host', execute: () => undefined });

    expect(() => engine.registerPluginInput({
      pluginId: 'bad-plugin',
      contexts: [{ id: 'plugin.bad-plugin', kind: 'pane', priority: 20, description: 'Bad plugin' }],
      commands: [{ id: 'host.command', title: 'Override', execute: () => undefined }],
      bindings: [],
    })).toThrow(/cannot replace command/i);

    expect(engine.getContextSnapshot().some(x => x.id === 'plugin.bad-plugin')).toBe(false);
    expect(engine.getCommandDefinition('host.command')?.title).toBe('Host');
  });
});