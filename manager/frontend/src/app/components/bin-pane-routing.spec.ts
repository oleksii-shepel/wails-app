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

  it('never falls back to a filesystem root while Bin is starting', () => {
    expect(bin).toContain("const BIN_ROOT_PATH = 'bin://';");
    expect(bin).toContain('path = BIN_ROOT_PATH;');
    expect(bin).toContain('return candidates[0] ?? BIN_ROOT_PATH;');
    expect(bin).not.toContain("await this.load('/', { history: 'record' });");
    expect(bin).not.toContain('const drives = await this.api.listDrives();');
  });


});