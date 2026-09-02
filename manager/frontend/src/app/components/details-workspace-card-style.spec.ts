import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./tool-workspace-host.component.ts', import.meta.url),
  'utf8',
);

describe('Details workspace card styling', () => {
  it('uses a gray workspace canvas with 5px breathing room', () => {
    expect(source).toContain('.split.details-workspace-split {');
    expect(source).toContain('gap: 5px;');
    expect(source).toContain('padding: 5px;');
    expect(source).toContain('background: var(--vsc-panel-background, var(--vsc-side-bar-background));');
  });

  it('renders Details surfaces as rounded cards', () => {
    expect(source).toContain('.split.details-workspace-split .surface {');
    expect(source).toContain('border-radius: 6px;');
    expect(source).toContain('background: var(--vsc-editor-background);');
  });

  it('keeps workspace toolbar buttons borderless', () => {
    expect(source).toContain('.surface-header button,');
    expect(source).toContain('border: 0;');
    expect(source).toContain('background: transparent;');
  });
});
