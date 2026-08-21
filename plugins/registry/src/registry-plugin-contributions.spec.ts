import { readFileSync } from 'node:fs';

const plugin = readFileSync(
  new URL('./registry.plugin.ts', import.meta.url),
  'utf8',
);
const publicApi = readFileSync(
  new URL('./public-api.ts', import.meta.url),
  'utf8',
);

describe('Registry contribution ownership', () => {
  it('does not export or register a plugin-owned pane presentation', () => {
    expect(publicApi).not.toContain("./registry-pane.component");
    expect(plugin).not.toContain('RegistryPanelPresentationContribution');
    expect(plugin).not.toContain('registerPanelPresentation');
  });

  it('owns Registry toolbar actions', () => {
    for (const label of ['New key', 'New value', 'Edit', 'Delete', 'Refresh']) {
      expect(plugin).toContain(`label: '${label}'`);
    }
  });

  it('owns Registry background and entry context actions', () => {
    for (const label of ['New Key', 'New Value', 'Edit', 'Delete', 'Refresh']) {
      expect(plugin).toContain(`label: '${label}'`);
    }
    expect(plugin).toContain('context.targetPath === context.currentDirectory');
    expect(plugin).toContain('context.targetPath !== context.currentDirectory');
  });

  it('keeps the package descriptor version aligned with package.json', () => {
    expect(plugin).toContain("version: '1.3.2'");
  });
});
