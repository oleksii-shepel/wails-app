import { readFileSync } from 'node:fs';

describe('PaneViewHost native overlay lifecycle', () => {
  const source = readFileSync(new URL('./pane-view-host.component.ts', import.meta.url), 'utf8');

  it('does not keep a native overlay mounted behind normal file views', () => {
    expect(source).toContain('*ngIf="cachedShowBrowserPane"');
    expect(source).toContain('[active]="surfaceActive && cachedShowBrowserPane"');
    expect(source).not.toContain('[class.view-browser--hidden]="!cachedShowBrowserPane"');
  });

  it('still mounts the browser overlay for actual web paths', () => {
    expect(source).toContain('<app-native-overlay');
    expect(source).toContain('[url]="cachedBrowserPath"');
  });
});
