import { InputEngineService } from './input-engine.service';

describe('InputEngineService modifier normalization', () => {
  it('matches a binding regardless of declared modifier order', async () => {
    const engine = new InputEngineService();

    engine.registerContext({
      id: 'test',
      kind: 'surface',
      priority: 100,
      parentId: 'global',
      description: 'test',
    });
    engine.activateContext('test');

    let called = false;
    engine.registerCommand('test.command', () => {
      called = true;
    });

    engine.registerBinding({
      id: 'test.ctrlShiftAltN',
      commandId: 'test.command',
      contextId: 'test',
      source: 'default',
      description: 'Three modifier shortcut',
      category: 'Test',
      trigger: {
        device: 'keyboard',
        chord: {
          steps: [{
            key: 'n',
            modifiers: ['ctrl', 'shift', 'alt'],
          }],
        },
      },
    });

    const handled = await engine.dispatchKeyboardEvent({
      key: 'n',
      code: 'KeyN',
      ctrlKey: true,
      altKey: true,
      shiftKey: true,
    });

    expect(handled).toBeTrue();
    expect(called).toBeTrue();
  });

  it('also normalizes two-modifier bindings such as Ctrl+Alt', async () => {
    const engine = new InputEngineService();

    engine.registerContext({
      id: 'test',
      kind: 'surface',
      priority: 100,
      parentId: 'global',
      description: 'test',
    });
    engine.activateContext('test');

    let called = false;
    engine.registerCommand('test.command', () => {
      called = true;
    });

    engine.registerBinding({
      id: 'test.ctrlAltP',
      commandId: 'test.command',
      contextId: 'test',
      source: 'default',
      description: 'Two modifier shortcut',
      category: 'Test',
      trigger: {
        device: 'keyboard',
        chord: {
          steps: [{
            key: 'p',
            modifiers: ['ctrl', 'alt'],
          }],
        },
      },
    });

    const handled = await engine.dispatchKeyboardEvent({
      key: 'p',
      code: 'KeyP',
      ctrlKey: true,
      altKey: true,
    });

    expect(handled).toBeTrue();
    expect(called).toBeTrue();
  });
});
