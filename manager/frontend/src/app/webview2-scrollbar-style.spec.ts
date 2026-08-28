import { readFileSync } from 'node:fs';

describe('WebView2 application scrollbar contract', () => {
  const styles = readFileSync(new URL('../../styles.scss', import.meta.url), 'utf8');

  it('does not opt into Chromium native thin scrollbar rendering', () => {
    expect(styles).not.toContain('scrollbar-width: thin');
    expect(styles).not.toContain('scrollbar-color: var(--vsc-scrollbar-thumb)');
  });

  it('uses a full rectangular translucent WebKit thumb', () => {
    expect(styles).toContain('*::-webkit-scrollbar-thumb {');
    expect(styles).toContain('border: 0;');
    expect(styles).toContain('border-radius: 0;');
    expect(styles).toContain('background-clip: border-box;');
    expect(styles).not.toContain('border-radius: 999px');
    expect(styles).not.toContain('border: 3px solid transparent');
  });

  it('uses a transparent themed track', () => {
    expect(styles).toContain('*::-webkit-scrollbar-track-piece');
    expect(styles).toContain('background: var(--vsc-scrollbar-track);');
  });

  it('explicitly removes WebView2 single-button arrow controls', () => {
    expect(styles).toContain('*::-webkit-scrollbar-button:single-button');
    expect(styles).toContain('*::-webkit-scrollbar-button:vertical:decrement');
    expect(styles).toContain('*::-webkit-scrollbar-button:vertical:increment');
    expect(styles).toContain('-webkit-appearance: none;');
    expect(styles).toContain('display: none;');
    expect(styles).toContain('width: 0;');
    expect(styles).toContain('height: 0;');
  });
});
