import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./panel-source-pane-session.ts', import.meta.url),
  'utf8',
);

describe('PanelSourcePaneSession Registry sibling contract', () => {
  it('publishes a canonical specialized location before async activation', () => {
    expect(source).toContain('this.path = canonicalPath');
    expect(source).toContain('Publish the location before asynchronous plugin activation');
  });

  it('only memoizes synchronization after a successful load', () => {
    expect(source).toContain('if (loaded)');
    expect(source).toContain('this.syncedPath = this.path');
  });

  it('treats activateEntry false as unhandled so directory-like provider rows navigate', () => {
    expect(source).toContain('false means "not handled"');
    expect(source).toContain('await this.navigateTo(entry.path)');
  });
});