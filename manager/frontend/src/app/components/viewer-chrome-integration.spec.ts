import { readFileSync } from 'node:fs';

describe('single viewer chrome integration', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const host = readFileSync(
    new URL('../viewers/viewer-host-surface.component.ts', import.meta.url),
    'utf8',
  );

  it('does not render a second pane-level viewer toolbar', () => {
    const shell = pane.slice(
      pane.indexOf('<div class="viewer-shell"'),
      pane.indexOf('</ng-template>', pane.indexOf('<div class="viewer-shell"')),
    );
    expect(shell).not.toContain('<div class="viewer-toolbar">');
  });

  it('delegates source, external and close actions to the one host chrome', () => {
    expect(pane).toContain('(returnSource)="returnToViewerSource()"');
    expect(pane).toContain('(external)="openActiveViewerExternally()"');
    expect(pane).toContain('(close)="closeActiveTab()"');
  });

  it('suppresses duplicated built-in viewer file headers but preserves viewer-specific toolbars', () => {
    expect(host).toContain(':host ::ng-deep .viewer-content .viewer-header');
    expect(host).toContain('display: none !important');
    expect(host).toContain(':host ::ng-deep .viewer-content .viewer-toolbar');
  });

  it('shows filename, type and useful metadata in the host chrome', () => {
    expect(host).toContain('viewer-title');
    expect(host).toContain('probe?.mimeType');
    expect(host).toContain('probe?.encoding');
    expect(host).toContain('formatSize(fileInfo?.size || 0)');
  });
});