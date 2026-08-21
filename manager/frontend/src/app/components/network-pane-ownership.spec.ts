import { readFileSync } from 'node:fs';

const renderers = readFileSync(
  new URL('./pane-renderer-registry.service.ts', import.meta.url),
  'utf8',
);
const host = readFileSync(
  new URL('./pane-host.component.ts', import.meta.url),
  'utf8',
);
const network = readFileSync(
  new URL('./network-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Manager-owned Network pane', () => {
  it('has a first-class Manager renderer identity', () => {
    expect(renderers).toContain("| 'network'");
    expect(renderers).toContain("id: 'builtin.network-pane'");
    expect(renderers).toContain("kind: 'network'");
    expect(renderers).toContain("scheme === 'network'");
  });

  it('does not route network through the generic system-table renderer', () => {
    const schemesStart = renderers.indexOf('SYSTEM_TABLE_SCHEMES');
    const constructorStart = renderers.indexOf('@Injectable', schemesStart);
    const schemes = renderers.slice(schemesStart, constructorStart);
    expect(schemes).not.toContain("'network'");
  });

  it('is rendered directly by PaneHost from Manager code', () => {
    expect(host).toContain(
      "import { NetworkPaneComponent } from './network-pane.component';",
    );
    expect(host).toContain("*ngSwitchCase=\"'network'\"");
    expect(host).toContain('<app-network-pane');
  });

  it('keeps the existing Manager network columns as the initial presentation', () => {
    expect(network).toContain("kind: 'network'");
    for (const field of ['kind', 'status', 'local', 'remote']) {
      expect(network).toContain(`field: '${field}'`);
    }
  });

  it('keeps context-menu and selection events on the shared pane chain', () => {
    expect(network).toContain('backgroundContextMenu.emit($event)');
    expect(network).toContain('entryContextMenu.emit({ entry: entry, event: $event })');
    expect(network).toContain('entrySelect.emit({ entry: entry, event: $event })');
  });
});
