import { readFileSync } from 'node:fs';

const styles = readFileSync(new URL('../../../styles.scss', import.meta.url), 'utf8');

describe('global button style contract', () => {
  it('does not turn every native button into a primary application action', () => {
    const baseStart = styles.indexOf('button {');
    const baseEnd = styles.indexOf('button:not(:disabled)', baseStart);
    const base = styles.slice(baseStart, baseEnd);

    expect(base).not.toContain('var(--vsc-button-background)');
    expect(base).not.toContain('var(--vsc-button-foreground)');
  });

  it('keeps primary styling opt-in', () => {
    expect(styles).toContain('.btn-primary {');
    expect(styles).toContain('background: var(--vsc-button-background);');
  });

  it('treats dialog close controls as neutral chrome', () => {
    expect(styles).toContain('span.dialog-close');
    expect(styles).toContain('appearance: none;');
    expect(styles).toContain('background: transparent;');
    expect(styles).toContain('transform: none;');
  });

  it('does not compact every native button globally', () => {
    expect(styles).not.toContain('.compact-mode button,');
  });
});