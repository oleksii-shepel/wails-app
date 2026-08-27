import { readFileSync } from 'node:fs';

describe('custom VS Code scrollbar removal', () => {
  it('uses native/theme scrollbar CSS instead of appVscodeScrollbar overlays', () => {
    const styles = readFileSync(new URL('../../../styles.scss', import.meta.url), 'utf8');
    expect(styles).not.toContain('.vsc-scroll-host');
    expect(styles).not.toContain('.vsc-scroll-anchor');
    expect(styles).not.toContain('.vsc-scrollbar {');
    expect(styles).toContain('*::-webkit-scrollbar');
  });
});
