import { NgZone } from '@angular/core';
import { fn } from '../testing/jasmine-test-helpers';
import { CommandRegistryService } from './command-registry.service';
import { InputEngineService } from './input-engine.service';

function createEngine(): InputEngineService {
  const zone = new NgZone({ enableLongStackTrace: false });
  return new InputEngineService(zone, new CommandRegistryService(zone));
}

function registerNativeShortcut(
  engine: InputEngineService,
  id: string,
  key: string,
  modifiers: Array<'ctrl' | 'shift' | 'alt' | 'meta'> = [],
) {
  const execute = fn();
  engine.registerCommandDescriptor({ id, title: id, execute });
  engine.registerBinding({
    id: `${id}.binding`,
    commandId: id,
    contextId: 'global',
    source: 'default',
    description: id,
    trigger: { device: 'keyboard', chord: { steps: [{ key, modifiers: modifiers.length ? modifiers : undefined }] } },
  });
  return execute;
}

describe('InputEngineService native keyboard boundary', () => {
  it('dispatches Ctrl+/ exactly once from the real window keydown listener', () => {
    const engine = createEngine();
    const execute = registerNativeShortcut(engine, 'test.native.help', '/', ['ctrl']);

    const event = new KeyboardEvent('keydown', {
      key: '.',
      code: 'Slash',
      ctrlKey: true,
      bubbles: true,
      cancelable: true,
    });

    window.dispatchEvent(event);
    expect(execute).toHaveBeenCalledTimes(1);
    expect(event.defaultPrevented).toBeTrue();
  });

  it('dispatches Ctrl+Shift+P exactly once from the real window keydown listener', () => {
    const engine = createEngine();
    const execute = registerNativeShortcut(engine, 'test.native.palette', 'p', ['ctrl', 'shift']);

    const event = new KeyboardEvent('keydown', {
      key: 'P',
      code: 'KeyP',
      ctrlKey: true,
      shiftKey: true,
      bubbles: true,
      cancelable: true,
    });

    window.dispatchEvent(event);
    expect(execute).toHaveBeenCalledTimes(1);
  });

  it('dispatches F1 exactly once from the real window keydown listener', () => {
    const engine = createEngine();
    const execute = registerNativeShortcut(engine, 'test.native.inputCenter', 'f1');

    const event = new KeyboardEvent('keydown', {
      key: 'F1',
      code: 'F1',
      bubbles: true,
      cancelable: true,
    });

    window.dispatchEvent(event);
    expect(execute).toHaveBeenCalledTimes(1);
  });
});
