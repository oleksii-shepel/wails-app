import { readFileSync } from 'node:fs';

describe('Arrange dropdown in contained pane slots', () => {
  const dropdown = readFileSync(new URL('./dropdown-container.component.ts', import.meta.url), 'utf8');
  const arrange = readFileSync(new URL('./pane-arrange-controls.component.ts', import.meta.url), 'utf8');

  it('keeps Arrange non-modal', () => {
    expect(arrange).toContain('[backdrop]="false"');
  });

  it('converts viewport placement to the actual fixed containing block', () => {
    expect(dropdown).toContain('private fixedContainingBlock(element: HTMLElement)');
    expect(dropdown).toContain('viewportLeft - (containingRect?.left ?? 0)');
    expect(dropdown).toContain('viewportTop - (containingRect?.top ?? 0)');
  });

  it('recognizes CSS containment as a fixed-position containing block', () => {
    expect(dropdown).toContain("contain.includes('layout')");
    expect(dropdown).toContain("contain.includes('paint')");
  });
});
