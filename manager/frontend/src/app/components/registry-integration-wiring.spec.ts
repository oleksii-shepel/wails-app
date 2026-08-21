import { readFileSync } from 'node:fs';

const registry = readFileSync(new URL('./registry-pane.component.ts', import.meta.url), 'utf8');
const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const app = readFileSync(new URL('../app.component.ts', import.meta.url), 'utf8');

describe('Registry integration wiring', () => {
  it('feeds plugin toolbar actions into both pane hosts', () => {
    expect(dual).toContain('[pluginToolbarActions]="pluginToolbarActionsFor(leftPane)"');
    expect(dual).toContain('[pluginToolbarActions]="pluginToolbarActionsFor(rightPane)"');
  });

  it('mounts the Registry-owned editor host once at app root', () => {
    expect(app).toContain("import { RegistryPluginHostComponent } from '@reassembly/registry-plugin';");
    expect(app).toContain('RegistryPluginHostComponent,');
    expect(app).toContain('<reassembly-registry-plugin-host></reassembly-registry-plugin-host>');
  });

  it('uses the Registry panel-source activation path for value clicks and opens', () => {
    expect(registry).toContain('(click)="onRegistryEntryClick(entry, $event)"');
    expect(registry).toContain("if (this.registryEntryKind(entry) === 'Value')");
    expect(registry).toContain('source.activateEntry?.(');
  });

  it('preserves DWORD/QWORD display width', () => {
    expect(registry).toContain("type === 'REG_DWORD'");
    expect(registry).toContain("type === 'REG_QWORD'");
    expect(registry).toContain("padStart(width, '0')");
  });
});
