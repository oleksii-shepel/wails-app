import { readFileSync } from 'node:fs';

describe('global application scrollbar style', () => {
  const styles = readFileSync(new URL('../../../styles.scss', import.meta.url), 'utf8');
  const theme = readFileSync(new URL('../../../vscode-theme.scss', import.meta.url), 'utf8');

  it('uses one global rectangular scrollbar contract', () => {
    expect(styles).toContain('*::-webkit-scrollbar-thumb {');
    expect(styles).toContain('border: 0;');
    expect(styles).toContain('border-radius: 0;');
    expect(styles).toContain('background-clip: border-box;');
    expect(styles).not.toContain('border-radius: 999px;');
    expect(styles).not.toContain('border: 3px solid transparent;');
  });

  it('uses transparent track and corner globally', () => {
    expect(styles).toContain('background: var(--vsc-scrollbar-track);');
    expect(styles).toContain('scrollbar-color: var(--vsc-scrollbar-thumb) var(--vsc-scrollbar-track);');
  });

  it('uses the same scrollbar palette in dark and light themes', () => {
    expect(theme.match(/--vsc-scrollbar-size: 12px;/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-track: transparent;/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-thumb: rgba\(121, 121, 121, 0\.42\);/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-thumb-hover: rgba\(121, 121, 121, 0\.58\);/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-thumb-active: rgba\(121, 121, 121, 0\.72\);/g)?.length).toBe(2);
  });

  it('does not keep pane-only scrollbar skinning', () => {
    expect(styles).not.toContain('Pane view native scrollbars');
  });
});
