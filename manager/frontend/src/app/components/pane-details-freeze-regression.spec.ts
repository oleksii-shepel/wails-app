import { readFileSync } from 'node:fs';

describe('Details view responsiveness and header', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('uses compact header typography on the actual header cells', () => {
    expect(source).toMatch(/&\.header-cell\s*\{[^}]*font-size:\s*10px/s);
  });

  it('does not repopulate VirtualCollection with the full directory on viewport updates', () => {
    expect(source).not.toContain('populate(0, this.entries, this.entries.length)');
    expect(source).toContain('this.entries.slice(window.start, window.end)');
  });

  it('does not eagerly index every path when Details mounts', () => {
    expect(source).not.toContain('pathToEntryIndex = new Map');
    expect(source).toContain('private findEntryIndex(path: string): number');
  });

  it('cancels marquee capture if the WebView loses focus', () => {
    expect(source).toContain("window.addEventListener('blur', this.windowBlur)");
    expect(source).toContain("window.removeEventListener('blur', this.windowBlur)");
  });
});
