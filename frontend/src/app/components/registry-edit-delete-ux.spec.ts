import { readFileSync } from 'node:fs';

describe('Registry Edit/Delete UX', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const dual = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('uses generic Edit and Delete actions in the Registry toolbar', () => {
    expect(pane).toContain("runRegistryCommand('builtin.registry.edit')");
    expect(pane).toContain('>\n            Edit\n');
    expect(pane).toContain("runRegistryCommand('builtin.registry.delete')");
  });

  it('opens Registry values in Edit instead of Quick View', () => {
    expect(pane).toContain("if (kind === 'value')");
    expect(pane).toContain("this.runRegistryCommand('builtin.registry.edit')");
    expect(pane).toContain('open registry key:');
  });

  it('passes selected target and refresh callback to Registry commands', () => {
    expect(dual).toContain("targetPath: target?.path ?? pane.path");
    expect(dual).toContain("refresh: () => pane.refresh()");
  });
});
