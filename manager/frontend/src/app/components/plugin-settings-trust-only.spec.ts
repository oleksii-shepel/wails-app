import { readFileSync } from 'node:fs';

describe('Plugin settings trust-only UX', () => {
  const source = readFileSync(
    new URL('./plugin-settings-panel.component.ts', import.meta.url),
    'utf8',
  );

  it('has no separate Enabled control', () => {
    expect(source).not.toContain('<span>Enabled</span>');
    expect(source).not.toContain('toggleEnabled(');
    expect(source).not.toContain('enabledValue(');
    expect(source).not.toContain('Saving enabled setting timed out');
  });

  it('has no manual Activate button', () => {
    expect(source).not.toContain('>Activate</button>');
    expect(source).not.toContain('async activate(snapshot');
  });

  it('keeps trust as the admission control', () => {
    expect(source).toContain('Trust This Version');
    expect(source).toContain('Revoke Trust');
    expect(source).toContain('and enables it.');
  });

  it('keeps Deactivate for an already running plugin', () => {
    expect(source).toContain('>Deactivate</button>');
    expect(source).toContain("snapshot.status === 'active'");
  });
});
