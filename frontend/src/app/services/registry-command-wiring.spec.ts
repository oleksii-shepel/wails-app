import { readFileSync } from 'node:fs';

describe('Registry command wiring', () => {
  const manager = readFileSync(
    new URL('./plugin-manager.service.ts', import.meta.url),
    'utf8',
  );

  it('registers Registry actions as actual commands', () => {
    for (const id of [
      'builtin.registry.newKey',
      'builtin.registry.newValue',
      'builtin.registry.editValue',
      'builtin.registry.delete',
    ]) {
      expect(manager).toContain(`id: commandIds.`);
      expect(manager).toContain(`'${id}'`);
    }
  });

  it('uses command ids from Registry context-menu contributions', () => {
    expect(manager).toContain('commandId: commandIds.newKey');
    expect(manager).toContain('commandId: commandIds.newValue');
    expect(manager).toContain('commandId: commandIds.editValue');
    expect(manager).toContain('commandId: commandIds.delete');
  });

  it('does not keep Registry mutation logic as context-menu-only execute handlers', () => {
    expect(manager).toContain('execute: invocation => createKey(commandContext(invocation))');
    expect(manager).toContain('execute: invocation => createValue(commandContext(invocation))');
    expect(manager).toContain('execute: invocation => editValue(commandContext(invocation))');
    expect(manager).toContain('execute: invocation => deleteEntry(commandContext(invocation))');
  });
});
