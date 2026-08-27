import { readFileSync } from 'node:fs';

describe('Arrange context-menu visual styling', () => {
  const source = readFileSync(new URL('./pane-arrange-controls.component.ts', import.meta.url), 'utf8');

  it('owns an opaque context-menu surface inside its scoped component styles', () => {
    expect(source).toContain('background: var(--vsc-menu-background, var(--vsc-panel-background));');
    expect(source).toContain('border: 1px solid var(--vsc-menu-border, var(--vsc-border));');
    expect(source).toContain('box-shadow: var(--vsc-widget-shadow);');
  });

  it('remains non-modal and backdrop-free', () => {
    expect(source).toContain('[backdrop]="false"');
  });
});
