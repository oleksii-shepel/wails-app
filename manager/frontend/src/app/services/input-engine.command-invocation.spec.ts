import { NgZone } from '@angular/core';
import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { CommandRegistryService } from './command-registry.service';
import { InputEngineService } from './input-engine.service';
import type { CommandInvocation } from './input-system.types';

function createEngine(): InputEngineService {
  const zone = new NgZone({ enableLongStackTrace: false });
  return new InputEngineService(zone, new CommandRegistryService(zone));
}

describe('InputEngineService command invocation', () => {
  it('passes structured invocation data to command handlers', () => {
    const engine = createEngine();
    const execute = fn();
    engine.registerCommandDescriptor({ id: 'test.open', title: 'Open', execute });

    const invocation: CommandInvocation = {
      source: 'console',
      args: { location: 's3://bucket/report.csv' },
      context: { paneId: 'left' },
    };

    expect(engine.dispatchCommandById('test.open', invocation)).toBe(true);
    expect(execute).toHaveBeenCalledWith(invocation);
  });

  it('lets availability inspect invocation arguments', () => {
    const engine = createEngine();
    const execute = fn();
    engine.registerCommandDescriptor({
      id: 'test.hash',
      title: 'Hash',
      execute,
      isEnabled: invocation => Array.isArray(invocation?.args?.['paths']),
    });

    expect(engine.isCommandEnabled('test.hash')).toBe(false);
    expect(engine.isCommandEnabled('test.hash', {
      source: 'console',
      args: { paths: ['C:/work/a.txt'] },
    })).toBe(true);

    expect(engine.dispatchCommandById('test.hash', {
      source: 'console',
      args: { paths: ['C:/work/a.txt'] },
    })).toBe(true);
    expect(execute).toHaveBeenCalledTimes(1);
  });

  it('annotates binding-driven dispatch with its source and binding id', async () => {
    const engine = createEngine();
    const execute = fn();
    engine.registerCommandDescriptor({ id: 'test.refresh', title: 'Refresh', execute });
    engine.registerBinding({
      id: 'test.refresh.f9',
      commandId: 'test.refresh',
      contextId: 'global',
      source: 'default',
      description: 'Refresh',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'f9' }] } },
    });

    await engine.dispatchKeyStroke({ key: 'f9' });

    expect(execute).toHaveBeenCalledWith({
      source: 'keyboard',
      context: { bindingId: 'test.refresh.f9' },
    });
  });

  it('dispatches synthetic keyboard events with shortcut context', async () => {
    const engine = createEngine();
    const execute = fn();
    engine.registerCommandDescriptor({ id: 'test.palette', title: 'Palette', execute });
    engine.registerBinding({
      id: 'test.palette.ctrlShiftP',
      commandId: 'test.palette',
      contextId: 'global',
      source: 'default',
      description: 'Open palette',
      when: '!typing && activePane',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'p', modifiers: ['ctrl', 'shift'] }] } },
    });

    await engine.dispatchKeyboardEvent({
      key: 'P',
      code: 'KeyP',
      ctrlKey: true,
      shiftKey: true,
    }, {
      context: {
        hasActivePane: true,
      },
    });

    expect(execute).toHaveBeenCalledWith({
      source: 'keyboard',
      context: { bindingId: 'test.palette.ctrlShiftP' },
    });
  });

  it('recovers WebView2 Unidentified navigation accelerators from DOM code', async () => {
    const engine = createEngine();
    const previous = fn();
    const next = fn();
    engine.registerCommandDescriptor({ id: 'workspace.previous', title: 'Previous Workspace', execute: previous });
    engine.registerCommandDescriptor({ id: 'workspace.next', title: 'Next Workspace', execute: next });
    engine.registerBinding({
      id: 'workspace.previous.ctrlAltLeft',
      commandId: 'workspace.previous',
      contextId: 'global',
      source: 'default',
      description: 'Previous workspace',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowleft', modifiers: ['alt', 'ctrl'] }] } },
    });
    engine.registerBinding({
      id: 'workspace.next.ctrlAltRight',
      commandId: 'workspace.next',
      contextId: 'global',
      source: 'default',
      description: 'Next workspace',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowright', modifiers: ['alt', 'ctrl'] }] } },
    });

    expect(await engine.dispatchKeyboardEvent({
      key: 'Unidentified',
      code: 'ArrowLeft',
      ctrlKey: true,
      altKey: true,
    })).toBe(true);
    expect(await engine.dispatchKeyboardEvent({
      key: 'Unidentified',
      code: 'ArrowRight',
      ctrlKey: true,
      altKey: true,
    })).toBe(true);

    expect(previous).toHaveBeenCalledWith({
      source: 'keyboard',
      context: { bindingId: 'workspace.previous.ctrlAltLeft' },
    });
    expect(next).toHaveBeenCalledWith({
      source: 'keyboard',
      context: { bindingId: 'workspace.next.ctrlAltRight' },
    });
  });

  it('keeps the latest binding owner active when an older owner unregisters the same binding id', async () => {
    const engine = createEngine();
    const oldExecute = fn();
    const newExecute = fn();

    engine.registerCommandDescriptor({ id: 'test.refresh', title: 'Refresh', execute: oldExecute }, 'owner-a');
    engine.registerBinding({
      id: 'test.refresh.f9',
      commandId: 'test.refresh',
      contextId: 'global',
      source: 'default',
      description: 'Refresh',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'f9' }] } },
    }, 'owner-a');

    engine.registerCommandDescriptor({ id: 'test.refresh', title: 'Refresh', execute: newExecute }, 'owner-b');
    engine.registerBinding({
      id: 'test.refresh.f9',
      commandId: 'test.refresh',
      contextId: 'global',
      source: 'default',
      description: 'Refresh',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'f9' }] } },
    }, 'owner-b');

    engine.unregisterCommandDescriptor('test.refresh', 'owner-a');
    engine.unregisterBinding('test.refresh.f9', 'owner-a');

    await engine.dispatchKeyStroke({ key: 'f9' });

    expect(oldExecute).not.toHaveBeenCalled();
    expect(newExecute).toHaveBeenCalledTimes(1);
  });
});