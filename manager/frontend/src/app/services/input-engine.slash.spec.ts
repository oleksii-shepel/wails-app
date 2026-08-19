import { NgZone } from '@angular/core';
import { fn } from '../testing/jasmine-test-helpers';
import { CommandRegistryService } from './command-registry.service';
import { InputEngineService } from './input-engine.service';

describe('InputEngineService Ctrl+/ normalization', () => {
  function createEngine(): InputEngineService {
    const zone = new NgZone({ enableLongStackTrace: false });
    return new InputEngineService(zone, new CommandRegistryService(zone));
  }

  it('matches the physical Slash key even when the layout reports another key value', async () => {
    const engine = createEngine();
    const execute = fn();
    engine.registerCommandDescriptor({ id: 'test.help', title: 'Help', execute });
    engine.registerBinding({
      id: 'test.help.ctrlSlash',
      commandId: 'test.help',
      contextId: 'global',
      source: 'default',
      description: 'Show help',
      trigger: { device: 'keyboard', chord: { steps: [{ key: '/', modifiers: ['ctrl'] }] } },
    });

    expect(await engine.dispatchKeyboardEvent({ key: '.', code: 'Slash', ctrlKey: true })).toBe(true);
    expect(execute).toHaveBeenCalledTimes(1);
  });

  it('accepts Ctrl+NumpadDivide as the same quick-reference shortcut', async () => {
    const engine = createEngine();
    const execute = fn();
    engine.registerCommandDescriptor({ id: 'test.help', title: 'Help', execute });
    engine.registerBinding({
      id: 'test.help.ctrlSlash',
      commandId: 'test.help',
      contextId: 'global',
      source: 'default',
      description: 'Show help',
      trigger: { device: 'keyboard', chord: { steps: [{ key: '/', modifiers: ['ctrl'] }] } },
    });

    expect(await engine.dispatchKeyboardEvent({ key: 'Divide', code: 'NumpadDivide', ctrlKey: true })).toBe(true);
    expect(execute).toHaveBeenCalledTimes(1);
  });
});