import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('dialog close-note standardization', () => {
  it('uses the Command Palette backdrop contract for Keyboard Shortcuts', () => {
    const html = read('keyboard-help.component.html');
    const scss = read('keyboard-help.component.scss');
    expect(html).toContain('shortcut-reference-backdrop backdrop dialog-group');
    expect(html).toContain('shortcut-reference dialog-surface');
    expect(html).toContain('class="dialog-close"');
    expect(scss).toContain('place-items: start center');
    expect(scss).toContain('padding-top: min(8vh, 72px)');
    expect(scss).toContain('background: var(--vsc-scrim-soft)');
    expect(scss).not.toContain('backdrop-filter');
  });

  it('does not advertise Escape as dialog chrome text', () => {
    for (const name of [
      'keyboard-help.component.html',
      'command-palette.component.ts',
      'global-search.component.ts',
      'input-center.component.ts',
      'path-history.component.ts',
      'path-history-viewer.component.ts',
      'operation-journal.component.ts',
      'remote-connect-dialog.component.ts',
      'remote-connections.component.ts',
      'closed-tabs.component.ts',
      'tools-center.component.ts',
      'workspace-switcher.component.ts',
    ]) {
      const source = read(name);
      expect(source).withContext(name).not.toMatch(/Esc(?:ape)?\s+(?:to\s+)?close|Close \(Esc\)/i);
    }
  });
});