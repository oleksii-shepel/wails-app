import { describe, expect, it, vi } from 'vitest';
import { InputEngineService } from './input-engine.service';
import type { CommandInvocation } from './input-system.types';

describe('InputEngineService command invocation', () => {
  it('passes structured invocation data to command handlers', () => {
    const engine = new InputEngineService();
    const execute = vi.fn();
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
    const engine = new InputEngineService();
    const execute = vi.fn();
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
    const engine = new InputEngineService();
    const execute = vi.fn();
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
});
