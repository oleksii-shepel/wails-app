import { describe, expect, it, vi } from 'vitest';
import type { CommandState } from '../services/input-system.types';
import { presentPluginCommand, type PluginCommandPresentationHost } from './plugin-command-presentation';

function host(state?: CommandState): PluginCommandPresentationHost & { dispatch: ReturnType<typeof vi.fn> } {
  const dispatch = vi.fn(() => true);
  return {
    dispatch,
    getCommandState: () => state,
    dispatchCommandById: dispatch,
  };
}

describe('presentPluginCommand', () => {
  it('inherits title and availability from a registered command', async () => {
    const commandHost = host({
      id: 'plugin.demo.compress',
      title: 'Compress Selection',
      category: 'Plugins',
      registered: true,
      enabled: true,
    });

    const item = presentPluginCommand({
      id: 'compress-toolbar',
      commandId: 'plugin.demo.compress',
    }, {}, commandHost);

    expect(item.label).toBe('Compress Selection');
    expect(item.enabled).toBe(true);
    expect(item.commandId).toBe('plugin.demo.compress');

    await item.execute();
    expect(commandHost.dispatch).toHaveBeenCalledWith('plugin.demo.compress');
  });

  it('disables a contribution whose command is missing or disabled', async () => {
    const missingHost = host(undefined);
    const missing = presentPluginCommand({ id: 'missing', commandId: 'plugin.demo.missing' }, {}, missingHost);
    expect(missing.enabled).toBe(false);
    await missing.execute();
    expect(missingHost.dispatch).not.toHaveBeenCalled();

    const disabledHost = host({
      id: 'plugin.demo.disabled',
      title: 'Disabled Command',
      registered: true,
      enabled: false,
    });
    const disabled = presentPluginCommand({ id: 'disabled', commandId: 'plugin.demo.disabled' }, {}, disabledHost);
    expect(disabled.enabled).toBe(false);
  });

  it('keeps legacy callback contributions working', async () => {
    const execute = vi.fn();
    const item = presentPluginCommand({ id: 'legacy', label: 'Legacy Action', execute }, { selected: 1 }, host());

    expect(item.label).toBe('Legacy Action');
    expect(item.enabled).toBe(true);
    expect(item.commandId).toBeUndefined();

    await item.execute();
    expect(execute).toHaveBeenCalledWith({ selected: 1 });
  });
});
