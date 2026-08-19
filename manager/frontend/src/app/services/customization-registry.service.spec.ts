import { CustomizationRegistryService } from './customization-registry.service';

describe('CustomizationRegistryService', () => {
  it('supports owned overrides and restores the previous contribution', () => {
    const registry = new CustomizationRegistryService();
    registry.registerTheme({ id: 'workbench', label: 'Workbench', kind: 'dark', variables: { '--surface': '#111' } }, 'core');
    registry.registerTheme({ id: 'workbench', label: 'Plugin Workbench', kind: 'dark', variables: { '--surface': '#222' } }, 'plugin');

    expect(registry.getTheme('workbench')?.label).toBe('Plugin Workbench');
    registry.unregisterOwner('plugin');
    expect(registry.getTheme('workbench')?.label).toBe('Workbench');
  });

  it('removes all contribution kinds owned by a plugin', () => {
    const registry = new CustomizationRegistryService();
    registry.registerTheme({ id: 'theme', label: 'Theme', kind: 'light', variables: {} }, 'plugin');
    registry.registerColumnProfile({ id: 'columns', label: 'Columns', columnIds: ['name'] }, 'plugin');
    registry.registerLayoutPreset({
      id: 'layout', label: 'Layout', createLayout: workspaceId => ({
        kind: 'surface', surface: { id: `${workspaceId}:one`, surfaceId: 'directory-pane', title: 'Files' },
      }),
    }, 'plugin');
    registry.registerWorkspaceTemplate({
      id: 'template', label: 'Template', create: ({ workspaceId }) => ({
        workspaceId, title: 'Template', kind: 'files', lifetime: 'persistent',
        layout: { kind: 'surface', surface: { id: `${workspaceId}:one`, surfaceId: 'directory-pane', title: 'Files' } },
      }),
    }, 'plugin');

    registry.unregisterOwner('plugin');
    expect(registry.themes$.value).toEqual([]);
    expect(registry.columnProfiles$.value).toEqual([]);
    expect(registry.layoutPresets$.value).toEqual([]);
    expect(registry.workspaceTemplates$.value).toEqual([]);
  });
});