import { NgZone } from '@angular/core';
import { CommandRegistryService } from './command-registry.service';
import { InputEngineService } from './input-engine.service';

function createEngine(): InputEngineService {
  const zone = new NgZone({ enableLongStackTrace: false });
  return new InputEngineService(zone, new CommandRegistryService(zone));
}

describe('InputEngineService editable ownership', () => {
  it('does not let a lower-priority overlay consume a key while a dialog input owns focus', async () => {
    const engine = createEngine();

    engine.registerContext({
      id: 'overlay.workspace',
      kind: 'overlay',
      priority: 900,
      parentId: 'global',
      description: 'workspace',
    });
    engine.registerContext({
      id: 'dialog.prompt',
      kind: 'dialog',
      priority: 1000,
      parentId: 'global',
      description: 'prompt',
    });

    engine.activateContext('overlay.workspace');
    engine.activateContext('dialog.prompt');

    let backgroundCalled = false;
    engine.registerCommand('overlay.slot1', () => {
      backgroundCalled = true;
    });

    engine.registerBinding({
      id: 'overlay.slot1',
      commandId: 'overlay.slot1',
      contextId: 'overlay.workspace',
      source: 'default',
      description: 'Workspace slot',
      trigger: {
        device: 'keyboard',
        chord: { steps: [{ key: '1' }] },
      },
    });

    const handled = await engine.dispatchKeyStroke(
      { key: '1' },
      { context: { inInput: true, inModal: true } },
    );

    expect(handled).toBeFalse();
    expect(backgroundCalled).toBeFalse();
  });

  it('still lets the owning dialog handle Enter while typing', async () => {
    const engine = createEngine();

    engine.registerContext({
      id: 'dialog.prompt',
      kind: 'dialog',
      priority: 1000,
      parentId: 'global',
      description: 'prompt',
    });
    engine.activateContext('dialog.prompt');

    let accepted = false;
    engine.registerCommand('dialog.accept', () => {
      accepted = true;
    });
    engine.registerBinding({
      id: 'dialog.accept',
      commandId: 'dialog.accept',
      contextId: 'dialog.prompt',
      source: 'default',
      description: 'Accept',
      when: 'typing',
      trigger: {
        device: 'keyboard',
        chord: { steps: [{ key: 'enter' }] },
      },
    });

    const handled = await engine.dispatchKeyStroke(
      { key: 'enter' },
      { context: { inInput: true, inModal: true } },
    );

    expect(handled).toBeTrue();
    expect(accepted).toBeTrue();
  });

  it('does not let workspace bindings consume keys while a context menu owns focus', async () => {
    const engine = createEngine();

    engine.registerContext({
      id: 'overlay.workspace',
      kind: 'overlay',
      priority: 900,
      parentId: 'global',
      description: 'workspace',
    });
    engine.activateContext('overlay.workspace');

    let backgroundCalled = false;
    engine.registerCommand('overlay.cursorDown', () => {
      backgroundCalled = true;
    });
    engine.registerBinding({
      id: 'overlay.cursorDown',
      commandId: 'overlay.cursorDown',
      contextId: 'overlay.workspace',
      source: 'default',
      description: 'Workspace cursor down',
      trigger: {
        device: 'keyboard',
        chord: { steps: [{ key: 'arrowdown' }] },
      },
    });

    const handled = await engine.dispatchKeyStroke(
      { key: 'arrowdown' },
      { context: { inContextMenu: true } },
    );

    expect(handled).toBeFalse();
    expect(backgroundCalled).toBeFalse();
  });

  it('normalizes modifier order for multi-modifier shortcuts', async () => {
    const engine = createEngine();

    engine.registerContext({
      id: 'surface',
      kind: 'pane',
      priority: 100,
      parentId: 'global',
      description: 'surface',
    });
    engine.activateContext('surface');

    let called = false;
    engine.registerCommand('new.workspace', () => {
      called = true;
    });
    engine.registerBinding({
      id: 'new.workspace',
      commandId: 'new.workspace',
      contextId: 'surface',
      source: 'default',
      description: 'New workspace',
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
      shiftKey: true,
      altKey: true,
    });

    expect(handled).toBeTrue();
    expect(called).toBeTrue();
  });
});
