import { readFileSync } from 'node:fs';

describe('Registry single-view presentation', () => {
  const pane = readFileSync(
    new URL('./registry-pane.component.ts', import.meta.url),
    'utf8',
  );
  const directory = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('uses one Name Type Data view with no Size column', () => {
    expect(pane).toContain('Name');
    expect(pane).toContain('Type');
    expect(pane).toContain('Data');
    expect(pane).not.toContain('>Size<');
    expect(pane).not.toContain('RegistryPaneViewMode');
    expect(pane).not.toContain('@Input() viewMode');
  });

  it('does not expose the generic view switcher on Registry surfaces', () => {
    expect(directory).toContain(
      '<div class="view-switcher" *ngIf="!isRegistrySurface"',
    );
    expect(directory).not.toContain('[viewMode]="registryViewMode"');
  });

  it('uses folder and key emoji according to Registry semantics', () => {
    expect(pane).toContain("this.typeLabel(entry) === 'Key' ? '📁' : '🔑'");
  });
});
