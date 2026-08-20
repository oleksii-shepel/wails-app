import { readFileSync } from 'node:fs';

describe('sibling pane architecture', () => {
  const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');
  const renderers = readFileSync(new URL('./pane-renderer-registry.service.ts', import.meta.url), 'utf8');
  const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');

  it('keeps DirectoryPane as the directory sibling and selects other renderers in PaneHost', () => {
    expect(host).toContain('DirectoryPaneComponent');
    expect(host).toContain('RegistryPaneComponent');
    expect(host).toContain('AutorunPaneComponent');
    expect(host).toContain('SystemTablePaneComponent');
    expect(host).toContain('PaneRendererRegistryService');
  });

  it('routes system resources through one SystemTable sibling', () => {
    expect(renderers).toContain("'environment'");
    expect(renderers).toContain("'processes'");
    expect(renderers).toContain("'services'");
    expect(renderers).toContain("'tasks'");
    expect(renderers).toContain("'devices'");
    expect(renderers).toContain("'network'");
    expect(renderers).toContain("'shell-extensions'");
  });

  it('does not turn DirectoryPane into a Registry implementation', () => {
    expect(directory).not.toContain('RegistryPaneComponent');
  });
});
