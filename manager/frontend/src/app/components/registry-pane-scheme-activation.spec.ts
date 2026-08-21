import { readFileSync } from 'node:fs';

describe('Registry pane scheme activation regression', () => {
  const source = readFileSync(
    new URL('./registry-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('activates scheme plugins inside the generic load path', () => {
    expect(source).toContain('await this.activatePluginsForLocation(requestedPath);');
  });

  it('keeps registry on the built-in provider instead of reactivating the plugin scheme', () => {
    expect(source).toContain("if (scheme === 'filesystem' || scheme === 'registry') return;");
  });
});
