import {
  FILE_MANAGER_BINDINGS,
  FILE_MANAGER_COMMANDS,
} from './file-manager-command-registry';

describe('file manager command registry', () => {
  it('keeps command IDs stable and pane-scoped', () => {
    expect(FILE_MANAGER_COMMANDS.navigateUp).toBe('pane.nav.navigateUp');
    expect(FILE_MANAGER_COMMANDS.copyToOtherPane).toBe('pane.ctrl.copyToOther');
    expect(FILE_MANAGER_COMMANDS.openOperations).toBe('app.operations.open');
  });

  it('keeps binding ids unique', () => {
    const ids = FILE_MANAGER_BINDINGS.map(binding => binding.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('keeps every default binding attached to a known command id', () => {
    const commandIds = new Set<string>();
    for (const value of Object.values(FILE_MANAGER_COMMANDS)) {
      if (typeof value === 'string') commandIds.add(value);
    }
    for (let index = 1; index <= 9; index += 1) {
      commandIds.add(FILE_MANAGER_COMMANDS.jumpToTab(index));
    }

    for (const binding of FILE_MANAGER_BINDINGS) {
      expect(commandIds.has(binding.commandId))
        .withContext(binding.id)
        .toBe(true);
    }
  });
});
