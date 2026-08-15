import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { bindCommandPresentation, type CommandPresentation } from './presented-command';

describe('PresentedCommand', () => {
  it('binds execution without changing renderer-neutral metadata', async () => {
    const execute = fn();
    const presentation: CommandPresentation<'delete'> = {
      id: 'delete',
      label: 'Delete',
      shortcut: 'Del',
      enabled: false,
      source: 'builtin',
      commandId: 'pane.files.delete',
    };

    const command = bindCommandPresentation(presentation, execute);

    expect(command).toMatchObject(presentation);
    await command.execute();
    expect(execute).toHaveBeenCalledTimes(1);
  });
});

