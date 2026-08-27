import { readFileSync } from 'node:fs';

describe('pane scrollbar visual contract', () => {
  const styles = readFileSync(new URL('../../../styles.scss', import.meta.url), 'utf8');
  const theme = readFileSync(new URL('../../../vscode-theme.scss', import.meta.url), 'utf8');

  it('uses a 12px rectangular translucent scrollbar thumb on a transparent track', () => {
    expect(styles).toContain('width: var(--vsc-scrollbar-size);');
    expect(styles).toContain('height: var(--vsc-scrollbar-size);');
    expect(styles).toContain('background: var(--vsc-scrollbar-track);');
    expect(styles).toContain('border-radius: 0;');
    expect(styles).toContain('border: 0;');
    expect(styles).toContain('background: var(--vsc-scrollbar-thumb);');
  });

  it('uses the requested scrollbar tokens', () => {
    expect(theme).toContain('--vsc-scrollbar-size: 12px;');
    expect(theme).toContain('--vsc-scrollbar-track: transparent;');
    expect(theme.match(/--vsc-scrollbar-thumb: rgba\(121, 121, 121, 0\.42\);/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-thumb-hover: rgba\(121, 121, 121, 0\.58\);/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-thumb-active: rgba\(121, 121, 121, 0\.72\);/g)?.length).toBe(2);
  });
});
