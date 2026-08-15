import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { BackendPluginCommandBridgeService, backendPluginCommandId, backendPluginCommandName, parsePluginHotkey } from './backend-plugin-command-bridge.service';

class FakeInputEngine {
  contributions: any[] = [];
  removed: string[] = [];
  registerPluginInput(value: any): void { this.contributions.push(value); }
  unregisterPluginInput(pluginId: string): void { this.removed.push(pluginId); }
}

describe('BackendPluginCommandBridgeService', () => {
  it('parses backend hotkeys into normal input strokes', () => {
    expect(parsePluginHotkey('Ctrl+Alt+H')).toEqual({ key: 'h', modifiers: ['ctrl', 'alt'] });
    expect(parsePluginHotkey('Ctrl K')).toBeNull();
  });

  it('registers backend commands as plugin-owned input contributions', async () => {
    const api = {
      listBackendPlugins: fn().mockResolvedValue([{
        id: 'hash', name: 'Hash Tools', version: '1.0.0',
        commands: [{ id: 'sha256', label: 'SHA-256', hotkey: 'Ctrl+Alt+H', appliesTo: ['file'] }],
      }]),
      invokeBackendPluginCommand: fn().mockResolvedValue(undefined),
    };
    const input = new FakeInputEngine();
    const bridge = new BackendPluginCommandBridgeService(api as any, input as any);

    await bridge.start(() => ['C:/work/a.txt']);

    expect(input.contributions).toHaveLength(1);
    const contribution = input.contributions[0];
    expect(contribution.pluginId).toBe('backend-plugin:hash');
    expect(contribution.commands[0].id).toBe(backendPluginCommandId('hash', 'sha256'));
    expect(contribution.commands[0].name).toBe(backendPluginCommandName('hash', 'sha256'));
    expect(contribution.commands[0].isEnabled()).toBe(true);
    expect(contribution.bindings[0].source).toBe('plugin');

    await contribution.commands[0].execute();
    expect(api.invokeBackendPluginCommand).toHaveBeenCalledWith('hash', 'sha256', ['C:/work/a.txt']);

    await contribution.commands[0].execute({
      source: 'console',
      args: { paths: ['C:/explicit/b.txt'] },
    });
    expect(api.invokeBackendPluginCommand.calls.mostRecent().args).toEqual(['hash', 'sha256', ['C:/explicit/b.txt']]);
  });

  it('disables path-scoped commands without a selection and cleans up on stop', async () => {
    const api = {
      listBackendPlugins: fn().mockResolvedValue([{
        id: 'images', name: 'Images', version: '1',
        commands: [{ id: 'convert', label: 'Convert', appliesTo: ['.png'] }],
      }]),
      invokeBackendPluginCommand: fn(),
    };
    const input = new FakeInputEngine();
    const bridge = new BackendPluginCommandBridgeService(api as any, input as any);

    await bridge.start(() => []);
    expect(input.contributions[0].commands[0].isEnabled()).toBe(false);

    bridge.stop();
    expect(input.removed).toEqual(['backend-plugin:images']);
  });
});

