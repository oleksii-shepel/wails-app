import { readFileSync } from 'node:fs';

describe('Registry toolbar', () => {
  const pane = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');

  it('shows Registry-specific actions instead of filesystem operations', () => {
    expect(pane).toContain("*ngIf=\"isRegistrySurface; else directoryActionBar\"");
    expect(pane).toContain('New key');
    expect(pane).toContain('New value');
    expect(pane).toContain('Edit value');
    expect(pane).toContain('Keys');
    expect(pane).toContain('Refresh');
  });

  it('dispatches built-in Registry commands', () => {
    expect(pane).toContain("runRegistryCommand('builtin.registry.newKey')");
    expect(pane).toContain("runRegistryCommand('builtin.registry.newValue')");
    expect(pane).toContain("runRegistryCommand('builtin.registry.editValue')");
    expect(pane).toContain("runRegistryCommand('builtin.registry.delete')");
  });
});
