import { readFileSync } from 'node:fs';

const host = readFileSync(
  new URL('./pane-host.component.ts', import.meta.url),
  'utf8',
);
const registry = readFileSync(
  new URL('./registry-pane.component.ts', import.meta.url),
  'utf8',
);
const dual = readFileSync(
  new URL('./dual-pane.component.ts', import.meta.url),
  'utf8',
);
const app = readFileSync(
  new URL('../app.component.ts', import.meta.url),
  'utf8',
);

describe('Registry rendered path integration', () => {
  it('renders the Manager Directory-clone Registry pane, never the plugin pane component', () => {
    expect(host).toContain("import { RegistryPaneComponent } from './registry-pane.component';");
    expect(host).not.toContain("import { RegistryPaneComponent } from '@reassembly/registry-plugin';");
    expect(host).toContain("rendererKind !== 'directory' && rendererKind !== 'registry'");
    expect(host).toContain("*ngSwitchCase=\"'registry'\"");
  });

  it('resolves Registry toolbar contributions inside PaneHost', () => {
    expect(host).toContain('get effectivePluginToolbarActions()');
    expect(host).toContain('this.pluginManager.resolveToolbarActions(context)');
    expect(host).toContain('adaptPluginToolbarActions(');
    expect(host).toContain('[pluginToolbarActions]="effectivePluginToolbarActions"');
  });

  it('shows Registry type and data inline in the rendered clone', () => {
    expect(registry).toContain(
      '<span></span><span>Name</span><span>Kind</span><span>Type</span><span>Value</span>',
    );
    expect(registry).toContain('{{ entryDisplayType(entry) }}');
    expect(registry).toContain('{{ entryDisplayValue(entry) }}');
    expect(registry).toContain("entry.registryPreview?.trim() || ''");
  });

  it('activates Registry values from click and double-click/open paths', () => {
    expect(registry).toContain('(click)="onRegistryEntryClick(entry, $event)"');
    expect(registry).toContain('(dblclick)="openEntry(entry)"');
    expect(registry).toContain('source.activateEntry?.(');
    expect(registry).toContain("this.registryEntryKind(entry) === 'Value'");
  });

  it('mounts the plugin-owned editor outlet at application root', () => {
    expect(app).toContain("RegistryPluginHostComponent");
    expect(app).toContain('<reassembly-registry-plugin-host>');
  });

  it('uses Registry-only entry/background context menus', () => {
    expect(dual).toContain("title: entry.name || 'Registry entry'");
    expect(dual).toContain("title: pane?.contextState.path || 'Registry'");
  });
});
