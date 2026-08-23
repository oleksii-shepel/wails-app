import { readFileSync } from 'node:fs';

describe('Registry pane scheme activation regression', () => {
  const source = readFileSync(
    new URL('./registry-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('activates scheme plugins inside the generic load path', () => {
    expect(source).toContain('await this.activatePluginsForLocation(requestedPath);');
  });

  it('does not bypass Registry plugin activation in the generic scheme path', () => {
    expect(source).toContain("if (scheme === 'filesystem') return;");
    expect(source).not.toContain("if (scheme === 'filesystem' || scheme === 'registry') return;");
  });
});