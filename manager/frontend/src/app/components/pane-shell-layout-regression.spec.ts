import { readFileSync } from 'node:fs';

const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');

describe('pane shell layout regression', () => {
  it('makes pane slots own the complete available height', () => {
    expect(dual).toMatch(/\.pane-slot\s*\{[^}]*display:\s*flex;/s);
    expect(dual).toMatch(/\.pane-slot\s*\{[^}]*height:\s*100%;/s);
    expect(dual).toMatch(/\.pane-slot\s*>\s*app-pane-host\s*\{[^}]*height:\s*100%;/s);
  });

  it('makes PaneHost and specialized panes consume their complete slot', () => {
    expect(host).toMatch(/:host\s*\{[^}]*height:\s*100%;/s);
    expect(host).toMatch(/\.specialized-pane\s*\{[^}]*height:\s*100%;/s);
  });

  it('does not retain the known pane-shell mojibake sequences', () => {
    for (const broken of [
      '\u0412\u00b7',
      '\u0432\u2039\u0407',
      '\u0432\u045a\u201c',
      '\u0432\u0402\u00a6',
    ]) {
      expect(dual).not.toContain(broken);
      expect(host).not.toContain(broken);
    }
  });
});