import { readFileSync } from 'node:fs';

describe('Registry pane host contract', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('does not depend on a Registry view-mode API', () => {
    expect(pane).not.toContain('RegistryPaneViewMode');
    expect(pane).not.toContain('[viewMode]="registryViewMode"');
    expect(pane).not.toContain('get registryViewMode()');
  });

  it('exposes Registry toolbar commands as typed string events', () => {
    expect(pane).toContain(
      '@Output() registryCommandRequested = new EventEmitter<string>()',
    );
    expect(pane).toContain('this.registryCommandRequested.emit(commandId)');
  });

  it('keeps Registry toolbar free of filesystem sort controls', () => {
    expect(pane).not.toContain('aria-label="Sort Registry"');
    expect(pane).toContain("runRegistryCommand('builtin.registry.newKey')");
    expect(pane).toContain("runRegistryCommand('builtin.registry.newValue')");
    expect(pane).toContain("runRegistryCommand('builtin.registry.edit')");
    expect(pane).toContain("runRegistryCommand('builtin.registry.delete')");
  });
});
