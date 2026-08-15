import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { adaptPluginToolbarActions, buildBuiltinPaneToolbarActions } from './pane-toolbar-action';
import type { ActionAvailability } from '../services/action-availability';
import type { PluginCommandPresentationHost } from './plugin-command-presentation';

const availability: ActionAvailability = {
  createFolder: false,
  deleteSelected: false,
  copySelected: true,
  cutSelected: false,
  paste: false,
  rename: false,
  copyToOther: true,
  moveToOther: false,
};

describe('pane toolbar actions', () => {
  it('derives built-in enabled state from ActionAvailability', () => {
    const actions = buildBuiltinPaneToolbarActions(availability, {
      createFolder: fn(),
      deleteSelected: fn(),
      copyToOther: fn(),
      moveToOther: fn(),
      refresh: fn(),
    });

    expect(actions.map(({ id, enabled }) => ({ id, enabled }))).toEqual([
      { id: 'new-folder', enabled: false },
      { id: 'delete', enabled: false },
      { id: 'copy-to-other', enabled: true },
      { id: 'move-to-other', enabled: false },
      { id: 'refresh', enabled: true },
    ]);
  });

  it('adapts plugin contributions without moving plugin logic into the action bar', async () => {
    const execute = fn();
    const context = { selectedPaths: ['filesystem://c/a.txt'], currentDirectory: 'filesystem://c' };
    const actions = adaptPluginToolbarActions([{
      id: 'hash',
      label: 'Hash',
      icon: '#',
      shortcut: 'Ctrl+H',
      score: () => 10,
      execute,
    }], context, {
      getCommandState: () => undefined,
      dispatchCommandById: () => false,
    } satisfies PluginCommandPresentationHost);

    expect(actions[0]).toMatchObject({
      id: 'plugin:hash',
      label: 'Hash',
      icon: 'plugin',
      iconText: '#',
      shortcut: 'Ctrl+H',
      enabled: true,
      source: 'plugin',
    });
    await actions[0].execute();
    expect(execute).toHaveBeenCalledWith(context);
  });
});
