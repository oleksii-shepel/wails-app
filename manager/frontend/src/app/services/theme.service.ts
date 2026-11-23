import { inject, Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import { CustomizationRegistryService } from './customization-registry.service';

export type Theme = 'dark' | 'light';
export type ThemePreference = Theme | 'system';

const THEME_PRESET_KEY = 'app-theme-preset';
const DEFAULT_PRESET = 'midnight-default';

@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly customizations = inject(CustomizationRegistryService, { optional: true });
  private readonly themePreference$ = createBehaviorSubject<ThemePreference>(this.readInitialThemePreference());
  private readonly themePreset$ = createBehaviorSubject<string>(this.readInitialThemePreset());
  private theme$ = createBehaviorSubject<Theme>(this.resolveTheme(this.themePreference$.value));
  private compact$ = createBehaviorSubject<boolean>(this.readInitialCompact());
  private readonly systemThemeMedia = window.matchMedia ? window.matchMedia('(prefers-color-scheme: light)') : null;

  constructor() {
    this.registerBuiltinPresets();
    this.customizations?.themes$.subscribe(() => {
      if (this.customizations?.getTheme(this.themePreset$.value)) {
        this.applyPreset(this.themePreset$.value);
      }
    });
    this.systemThemeMedia?.addEventListener?.('change', () => {
      if (this.themePreference$.value === 'system') {
        const resolved = this.resolveTheme('system');
        this.theme$.next(resolved);
        this.applyTheme(resolved);
        this.applyPreset(this.themePreset$.value);
      }
    });
    this.applyTheme(this.theme$.value);
    this.applyPreset(this.themePreset$.value);
    this.applyCompact(this.compact$.value);
  }

  private registerBuiltinPresets(): void {
    if (!this.customizations) return;
    const builtins = [
      { id: 'midnight-default', label: 'Midnight Graphite (Default)', kind: 'adaptive' as const, variables: {} },
      { id: 'nordic-slate', label: 'Nordic Slate', kind: 'adaptive' as const, variables: {} },
      { id: 'obsidian-emerald', label: 'Obsidian Emerald', kind: 'adaptive' as const, variables: {} },
      { id: 'graphite-amber', label: 'Graphite Amber', kind: 'adaptive' as const, variables: {} },
      { id: 'warm-paper', label: 'Warm Paper', kind: 'adaptive' as const, variables: {} },
    ];
    for (const preset of builtins) {
      if (!this.customizations.getTheme(preset.id)) {
        this.customizations.registerTheme(preset, 'builtin');
      }
    }
  }

  setTheme(theme: Theme) {
    this.setThemePreference(theme);
  }

  setThemePreference(theme: ThemePreference) {
    const resolved = this.resolveTheme(theme);
    this.themePreference$.next(theme);
    this.theme$.next(resolved);
    this.applyTheme(resolved);
    this.applyPreset(this.themePreset$.value);
    try { localStorage.setItem('app-theme', theme); } catch {}
  }

  setThemePreset(presetId: string) {
    const clean = (presetId || DEFAULT_PRESET).trim() || DEFAULT_PRESET;
    this.themePreset$.next(clean);
    this.applyPreset(clean);
    try { localStorage.setItem(THEME_PRESET_KEY, clean); } catch {}
  }

  getThemePreset$() {
    return this.themePreset$;
  }

  toggleTheme() {
    this.setThemePreference(this.theme$.value === 'dark' ? 'light' : 'dark');
  }

  getTheme$() {
    return this.theme$;
  }

  getThemePreference$() {
    return this.themePreference$;
  }

  setCompact(compact: boolean) {
    this.compact$.next(compact);
    this.applyCompact(compact);
    try { localStorage.setItem('app-compact', compact ? '1' : '0'); } catch {}
  }

  toggleCompact() {
    this.setCompact(!this.compact$.value);
  }

  getCompact$() {
    return this.compact$;
  }

  private applyTheme(theme: Theme) {
    const root = document.documentElement;
    root.classList.toggle('vscode-light', theme === 'light');
    root.classList.toggle('vscode-dark', theme === 'dark');
  }

  private applyPreset(presetId: string) {
    const root = document.documentElement;
    const resolvedMode = this.theme$.value;
    const applied = this.customizations?.applyTheme(presetId, root, resolvedMode) ?? false;
    if (!applied) {
      root.dataset['themePreset'] = presetId;
      root.dataset['themeMode'] = resolvedMode;
    }
  }

  private applyCompact(compact: boolean) {
    const root = document.documentElement;
    root.classList.toggle('compact-mode', compact);
  }

  private readInitialThemePreference(): ThemePreference {
    try {
      const stored = localStorage.getItem('app-theme');
      if (stored === 'light' || stored === 'dark' || stored === 'system') return stored;
    } catch {}
    return 'system';
  }

  private readInitialThemePreset(): string {
    try {
      return localStorage.getItem(THEME_PRESET_KEY) || DEFAULT_PRESET;
    } catch {}
    return DEFAULT_PRESET;
  }

  private resolveTheme(theme: ThemePreference): Theme {
    if (theme === 'light' || theme === 'dark') return theme;
    if (window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches) {
      return 'light';
    }
    return 'dark';
  }

  private readInitialCompact(): boolean {
    try {
      return localStorage.getItem('app-compact') === '1';
    } catch {}
    return false;
  }
}