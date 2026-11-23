import { Injector, runInInjectionContext } from '@angular/core';
import { CustomizationRegistryService } from './customization-registry.service';
import { ThemeService } from './theme.service';

describe('ThemeService', () => {
  const builtinPresetIds = [
    'midnight-default',
    'nordic-slate',
    'obsidian-emerald',
    'graphite-amber',
    'warm-paper',
  ];

  let originalMatchMedia: typeof window.matchMedia;
  let prefersLight = false;
  let mediaListeners: Array<() => void> = [];
  let savedTheme: string | null = null;
  let savedPreset: string | null = null;

  beforeEach(() => {
    savedTheme = localStorage.getItem('app-theme');
    savedPreset = localStorage.getItem('app-theme-preset');
    localStorage.removeItem('app-theme');
    localStorage.removeItem('app-theme-preset');
    document.documentElement.classList.remove('vscode-light', 'vscode-dark', 'compact-mode');
    delete document.documentElement.dataset['themePreset'];
    delete document.documentElement.dataset['themeId'];
    delete document.documentElement.dataset['themeKind'];
    delete document.documentElement.dataset['themeMode'];

    prefersLight = false;
    mediaListeners = [];
    originalMatchMedia = window.matchMedia;
    window.matchMedia = ((query: string) => ({
      matches: query.includes('light') ? prefersLight : !prefersLight,
      media: query,
      onchange: null,
      addEventListener: (_event: string, listener: () => void) => {
        mediaListeners.push(listener);
      },
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => true,
    })) as unknown as typeof window.matchMedia;
  });

  afterEach(() => {
    window.matchMedia = originalMatchMedia;
    document.documentElement.classList.remove('vscode-light', 'vscode-dark', 'compact-mode');
    delete document.documentElement.dataset['themePreset'];
    delete document.documentElement.dataset['themeId'];
    delete document.documentElement.dataset['themeKind'];
    delete document.documentElement.dataset['themeMode'];
    if (savedTheme === null) localStorage.removeItem('app-theme');
    else localStorage.setItem('app-theme', savedTheme);
    if (savedPreset === null) localStorage.removeItem('app-theme-preset');
    else localStorage.setItem('app-theme-preset', savedPreset);
  });

  function createService(): { service: ThemeService; customizations: CustomizationRegistryService } {
    const customizations = new CustomizationRegistryService();
    const injector = Injector.create({
      providers: [
        { provide: CustomizationRegistryService, useValue: customizations },
      ],
    });
    const service = runInInjectionContext(injector, () => new ThemeService());
    return { service, customizations };
  }

  it('registers all built-in presets as adaptive across dark, light, and system modes', () => {
    const { customizations } = createService();

    expect(customizations.themes$.value.map((preset) => preset.id)).toEqual(builtinPresetIds);
    for (const preset of customizations.themes$.value) {
      expect(preset.kind).toBe('adaptive');
    }
    expect(customizations.getTheme('warm-paper')?.label).toBe('Warm Paper');
  });

  it('preserves dark, light, and system modes across every built-in preset', () => {
    const { service } = createService();
    const root = document.documentElement;

    for (const presetId of builtinPresetIds) {
      service.setThemePreference('dark');
      service.setThemePreset(presetId);
      expect(root.classList.contains('vscode-dark')).toBe(true);
      expect(root.classList.contains('vscode-light')).toBe(false);
      expect(root.dataset['themePreset']).toBe(presetId);
      expect(root.dataset['themeMode']).toBe('dark');

      service.setThemePreference('light');
      expect(root.classList.contains('vscode-light')).toBe(true);
      expect(root.classList.contains('vscode-dark')).toBe(false);
      expect(root.dataset['themePreset']).toBe(presetId);
      expect(root.dataset['themeMode']).toBe('light');

      prefersLight = false;
      service.setThemePreference('system');
      expect(root.classList.contains('vscode-dark')).toBe(true);
      expect(root.classList.contains('vscode-light')).toBe(false);
      expect(root.dataset['themeMode']).toBe('dark');

      prefersLight = true;
      for (const listener of mediaListeners) listener();
      expect(root.classList.contains('vscode-light')).toBe(true);
      expect(root.classList.contains('vscode-dark')).toBe(false);
      expect(root.dataset['themeMode']).toBe('light');
    }
  });

  it('re-applies mode-specific custom theme variables when mode changes', () => {
    const { service, customizations } = createService();
    const root = document.documentElement;

    customizations.registerTheme({
      id: 'custom-adaptive',
      label: 'Custom Adaptive',
      kind: 'adaptive',
      variables: {},
      modes: {
        dark: { '--vsc-editor-background': '#12161c' },
        light: { '--vsc-editor-background': '#f6f8fa' },
      },
    }, 'plugin');

    service.setThemePreset('custom-adaptive');
    service.setThemePreference('dark');
    expect(root.style.getPropertyValue('--vsc-editor-background')).toBe('#12161c');

    service.setThemePreference('light');
    expect(root.style.getPropertyValue('--vsc-editor-background')).toBe('#f6f8fa');

    prefersLight = false;
    service.setThemePreference('system');
    expect(root.style.getPropertyValue('--vsc-editor-background')).toBe('#12161c');
  });
});
