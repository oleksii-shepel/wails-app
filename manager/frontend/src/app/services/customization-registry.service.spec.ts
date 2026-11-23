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

  it('applies mode-specific variables for dark and light modes without leaking single-mode overrides', () => {
    const registry = new CustomizationRegistryService();
    const target = document.createElement('div');

    registry.registerTheme({
      id: 'dual-mode',
      label: 'Dual Mode',
      kind: 'adaptive',
      variables: {},
      modes: {
        dark: { '--vsc-editor-background': '#101418' },
        light: { '--vsc-editor-background': '#f8fafc' },
      },
    }, 'plugin');

    expect(registry.applyTheme('dual-mode', target, 'dark')).toBe(true);
    expect(target.dataset['themePreset']).toBe('dual-mode');
    expect(target.dataset['themeMode']).toBe('dark');
    expect(target.style.getPropertyValue('--vsc-editor-background')).toBe('#101418');

    expect(registry.applyTheme('dual-mode', target, 'light')).toBe(true);
    expect(target.dataset['themeMode']).toBe('light');
    expect(target.style.getPropertyValue('--vsc-editor-background')).toBe('#f8fafc');

    registry.registerTheme({
      id: 'dark-only',
      label: 'Dark Only',
      kind: 'dark',
      variables: { '--vsc-editor-background': '#111111' },
    }, 'plugin');

    expect(registry.applyTheme('dark-only', target, 'light')).toBe(true);
    expect(target.dataset['themeMode']).toBe('light');
    expect(target.style.getPropertyValue('--vsc-editor-background')).toBe('');
  });
});