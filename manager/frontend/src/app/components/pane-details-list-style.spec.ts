import { readFileSync } from 'node:fs';

describe('PaneDetails List visual parity', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('uses List body inset and background', () => {
    expect(source).toContain('padding: var(--vsc-padding-sm);');
    expect(source).toContain('background: var(--vsc-editor-background);');
  });

  it('uses List row geometry', () => {
    expect(source).toContain('min-height: 28px;');
    expect(source).toContain('border-left: 3px solid transparent;');
    expect(source).toContain('border-radius: var(--vsc-border-radius-sm);');
  });

  it('uses List hover selection and cursor accents', () => {
    expect(source).toContain('border-left-color: var(--vsc-border-bright);');
    expect(source).toContain('border-left-color: var(--vsc-accent-blue);');
    expect(source).toContain('background: var(--vsc-list-active-background);');
  });

  it('keeps virtualization geometry synchronized with row height', () => {
    expect(source).toContain('rowHeight = 28');
  });
});
