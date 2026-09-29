import { readFileSync } from 'node:fs';

describe('Details cursor matches List view', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('uses rounded body rows', () => {
    expect(source).toContain('border-radius: 3px;');
  });

  it('uses the List hover fill for an unselected cursor', () => {
    expect(source).toContain('var(--vsc-list-hover-background, var(--vsc-list-hover-background)) 75%');
  });

  it('uses the same active-pane focus edge and selected emphasis', () => {
    expect(source).toContain('inset 0 0 0 1px color-mix(');
    expect(source).toContain('var(--vsc-list-active-background, var(--vsc-list-selected-emphasis)) 82%');
  });
});
