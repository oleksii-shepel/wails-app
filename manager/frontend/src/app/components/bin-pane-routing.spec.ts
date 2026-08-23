import { readFileSync } from 'node:fs';

describe('Bin pane routing', () => {
  const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');
  const renderers = readFileSync(new URL('./pane-renderer-registry.service.ts', import.meta.url), 'utf8');
  const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
  const bin = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

  it('resolves bin scheme to BinPane', () => {
    expect(renderers).toContain("kind: 'bin'");
    expect(renderers).toContain("scheme === 'bin'");
    expect(host).toContain("*ngSwitchCase=\"'bin'\"");
    expect(host).toContain('<app-bin-pane');
  });

  it('keeps BinPane a literal DirectoryPane clone initially', () => {
    const normalized = bin
      .replace("selector: 'app-bin-pane'", "selector: 'app-directory-pane'")
      .replace('export class BinPaneComponent implements OnInit, OnDestroy {',
               'export class DirectoryPaneComponent implements OnInit, OnDestroy {');
    expect(normalized).toBe(directory);
  });
});
