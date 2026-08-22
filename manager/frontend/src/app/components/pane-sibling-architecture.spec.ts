import { readFileSync } from 'node:fs';

describe('sibling pane architecture', () => {
  const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');
  const renderers = readFileSync(new URL('./pane-renderer-registry.service.ts', import.meta.url), 'utf8');
  const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
  const registry = readFileSync(new URL('./registry-pane.component.ts', import.meta.url), 'utf8');
  const autorun = readFileSync(new URL('./autorun-pane.component.ts', import.meta.url), 'utf8');

  it('keeps Registry as a renderer identity but uses the same pane implementation shape as Directory', () => {
    expect(host).toContain("import { RegistryPaneComponent } from './registry-pane.component'");
    expect(host).toContain("*ngSwitchCase=\"'registry'\"");
    expect(host).toContain('<app-registry-pane');
    expect(host).not.toContain("from '@reassembly/registry-plugin';");
  });

  it('keeps RegistryPane as a literal DirectoryPane clone apart from selector and class name', () => {
    const normalizedRegistry = registry
      .replace("selector: 'app-registry-pane'", "selector: 'app-directory-pane'")
      .replace('export class RegistryPaneComponent implements OnInit, OnDestroy {',
               'export class DirectoryPaneComponent implements OnInit, OnDestroy {');
    expect(normalizedRegistry).toBe(directory);
  });

  it('keeps AutorunPane as a literal DirectoryPane clone apart from selector and class name', () => {
    const normalizedAutorun = autorun
      .replace("selector: 'app-autorun-pane'", "selector: 'app-directory-pane'")
      .replace('export class AutorunPaneComponent implements OnInit, OnDestroy {',
               'export class DirectoryPaneComponent implements OnInit, OnDestroy {');
    expect(normalizedAutorun).toBe(directory);
    expect(host).toContain("*ngSwitchCase=\"'autorun'\"");
    expect(host).toContain('<app-autorun-pane');
  });

  it('routes system resources through one SystemTable sibling', () => {
    for (const scheme of ['environment', 'processes', 'services', 'tasks', 'devices', 'network', 'shell-extensions']) {
      expect(renderers).toContain(`'${scheme}'`);
    }
  });
});