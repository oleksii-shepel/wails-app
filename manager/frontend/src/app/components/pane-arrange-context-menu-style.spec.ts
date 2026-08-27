import { readFileSync } from 'node:fs';

describe('Arrange context-menu styling', () => {
  const source = readFileSync(new URL('./pane-arrange-controls.component.ts', import.meta.url), 'utf8');

  it('shares the application context-menu classes', () => {
    expect(source).toContain('arrange-menu context-menu');
    expect(source).toContain('arrange-item context-menu-item');
    expect(source).toContain('arrange-separator context-menu-separator');
  });

  it('remains non-modal', () => {
    expect(source).toContain('[backdrop]="false"');
  });
});
