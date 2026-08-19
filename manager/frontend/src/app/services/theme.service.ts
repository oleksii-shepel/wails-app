import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

export type Theme = 'dark' | 'light';
export type ThemePreference = Theme | 'system';

@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly themePreference$ = createBehaviorSubject<ThemePreference>(this.readInitialThemePreference());
  private theme$ = createBehaviorSubject<Theme>(this.resolveTheme(this.themePreference$.value));
  private compact$ = createBehaviorSubject<boolean>(this.readInitialCompact());
  private readonly systemThemeMedia = window.matchMedia ? window.matchMedia('(prefers-color-scheme: light)') : null;

  constructor() {
    this.systemThemeMedia?.addEventListener?.('change', () => {
      if (this.themePreference$.value === 'system') {
        const resolved = this.resolveTheme('system');
        this.theme$.next(resolved);
        this.applyTheme(resolved);
      }
    });
    this.applyTheme(this.theme$.value);
    this.applyCompact(this.compact$.value);
  }

  setTheme(theme: Theme) {
    this.setThemePreference(theme);
  }

  setThemePreference(theme: ThemePreference) {
    const resolved = this.resolveTheme(theme);
    this.themePreference$.next(theme);
    this.theme$.next(resolved);
    this.applyTheme(resolved);
    try { localStorage.setItem('app-theme', theme); } catch {}
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