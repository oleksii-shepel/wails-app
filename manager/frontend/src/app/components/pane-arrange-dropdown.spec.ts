import { readFileSync } from 'node:fs';

describe('Pane Arrange dropdown', () => {
  const arrange = readFileSync(new URL('./pane-arrange-controls.component.ts', import.meta.url), 'utf8');
  const dropdown = readFileSync(new URL('./dropdown-container.component.ts', import.meta.url), 'utf8');

  it('opens Arrange as a non-modal context-style dropdown', () => {
    expect(arrange).toContain('[(isOpen)]="menuOpen" [backdrop]="false"');
    expect(arrange).toContain('var(--vsc-menu-background, var(--vsc-input-background))');
    expect(arrange).toContain('var(--vsc-menu-border, var(--vsc-border))');
  });

  it('makes the generic backdrop optional without removing outside-click support', () => {
    expect(dropdown).toContain('@Input() backdrop = true;');
    expect(dropdown).toContain('*ngIf="isOpen && backdrop"');
    expect(dropdown).toContain('[appDropdown]="isOpen"');
  });
});
