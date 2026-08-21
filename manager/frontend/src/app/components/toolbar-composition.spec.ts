import { readFileSync } from 'node:fs';

const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
const registry = readFileSync(new URL('./registry-pane.component.ts', import.meta.url), 'utf8');
const responsive = readFileSync(new URL('./responsive-toolbar.component.ts', import.meta.url), 'utf8');

describe('toolbar composition', () => {
  it('keeps Directory sort/arrange as the compact dropdown', () => {
    expect(directory).toContain('<app-pane-arrange-controls');
    expect(directory).toContain('[currentSort]="sortField"');
    expect(directory).toContain('<app-responsive-toolbar');
  });

  it('keeps Registry toolbar text-only', () => {
    expect(registry).toContain('[showIcons]="false"');
    expect(registry).toContain('[actions]="pluginToolbarActions"');
  });

  it('supports icon suppression generically', () => {
    expect(responsive).toContain('@Input() showIcons = true;');
    expect(responsive).toContain('showIcons && (action.iconText || action.icon)');
  });
});
