import { inject, Injectable } from '@angular/core';
import { createBehaviorSubject, type Stream } from '@epikodelabs/streamix';
import { GetPreferences, SavePreferences } from 'wailsjs/go/app/App';
import { QueuedDialogGate } from './dialog-gate';
import { ThemeService } from './theme.service';

export type SettingsTab = 'general' | 'files' | 'editor' | 'view' | 'advanced' | 'plugins' | 'shortcuts' | 'about';

export interface GeneralSettings {
  theme: 'dark' | 'light' | 'system';
  themePreset?: string;
  language: string;
  startupBehavior: 'welcome' | 'lastSession' | 'specificFolder';
  startupFolder: string;
  updateChannel: 'stable' | 'beta' | 'dev';
  analyticsEnabled: boolean;
  crashReportsEnabled: boolean;
  checkUpdatesOnStartup: boolean;
  dateFormat: 'relative' | 'short' | 'long' | 'iso';
  sizeUnit: 'auto' | 'bytes' | 'kib' | 'mib' | 'gib';
  accentColor: string;
  uiZoom: number;
  confirmOnClose: boolean;
  reopenLastWorkspace: boolean;
}

export interface FileSettings {
  showHiddenFiles: boolean;
  showFileExtensions: boolean;
  confirmBeforeDelete: boolean;
  confirmBeforeMove: boolean;
  confirmBeforeCopy: boolean;
  confirmDragAndDrop: boolean;
  defaultOpenAction: 'edit' | 'external';
  calculateFolderSizes: boolean;
  enableFileWatcher: boolean;
  autoRefreshInterval: number;
  cacheThumbnailPreviews: boolean;
  thumbnailCacheSize: number;
  deleteToTrash: boolean;
  singleClickToOpen: boolean;
  sortFoldersFirst: boolean;
  copyOnDrop: boolean;
}

export interface EditorSettings {
  fontFamily: string;
  fontSize: number;
  lineHeight: number;
  wordWrap: 'off' | 'on' | 'wordWrapColumn' | 'bounded';
  wordWrapColumn: number;
  tabSize: number;
  insertSpaces: boolean;
  detectIndentation: boolean;
  trimTrailingWhitespace: boolean;
  insertFinalNewline: boolean;
  trimFinalNewlines: boolean;
  autoSave: 'off' | 'afterDelay' | 'onFocusChange' | 'onWindowChange';
  autoSaveDelay: number;
  minimap: boolean;
  renderWhitespace: 'none' | 'boundary' | 'selection' | 'all';
  cursorStyle: 'line' | 'block' | 'underline';
  smoothScrolling: boolean;
  bracketPairColorization: boolean;
  showLineNumbers: boolean;
  codeFolding: boolean;
  highlightCurrentLine: boolean;
  occurrencesHighlight: boolean;
  matchBrackets: 'never' | 'near' | 'always';
}

export interface ViewSettings {
  compactMode: boolean;
  iconSize: 'small' | 'medium' | 'large';
  listViewMode: 'details' | 'icons' | 'tiles';
  sortBy: 'name' | 'size' | 'modified' | 'created' | 'type';
  sortOrder: 'asc' | 'desc';
  showStatusBar: boolean;
  showBreadcrumbs: boolean;
  showPathBar: boolean;
  pathDisplayFormat: 'full' | 'short' | 'smart';
  animationsEnabled: boolean;
  showTabBar: boolean;
  thumbnailsEnabled: boolean;
  foldersFirst: boolean;
  tabPosition: 'top' | 'bottom';
}

export interface AdvancedSettings {
  enableLogging: boolean;
  logLevel: 'error' | 'warn' | 'info' | 'debug' | 'trace';
  logToFile: boolean;
  experimentalFeatures: boolean;
  gpuAcceleration: 'on' | 'off' | 'auto';
  memoryCacheSize: number;
  maxConcurrentOperations: number;
  networkTimeout: number;
  proxyEnabled: boolean;
  proxyHost: string;
  proxyPort: number;
  proxyAuthEnabled: boolean;
  proxyUsername: string;
  proxyPassword: string;
  useNativeFilePicker: boolean;
  hardReloadOnExternalChange: boolean;
  telemetryLevel: 'none' | 'error' | 'crash' | 'all';
}

export interface ProjectSettings {
  general: GeneralSettings;
  files: FileSettings;
  editor: EditorSettings;
  view: ViewSettings;
  advanced: AdvancedSettings;
}

const DEFAULT_SETTINGS: ProjectSettings = {
  general: {
    theme: 'system',
    themePreset: 'midnight-default',
    language: 'en',
    startupBehavior: 'lastSession',
    startupFolder: '',
    updateChannel: 'stable',
    analyticsEnabled: false,
    crashReportsEnabled: true,
    checkUpdatesOnStartup: true,
    dateFormat: 'relative',
    sizeUnit: 'auto',
    accentColor: '',
    uiZoom: 100,
    confirmOnClose: true,
    reopenLastWorkspace: true,
  },
  files: {
    showHiddenFiles: false,
    showFileExtensions: true,
    confirmBeforeDelete: true,
    confirmBeforeMove: true,
    confirmBeforeCopy: false,
    confirmDragAndDrop: true,
    defaultOpenAction: 'external',
    calculateFolderSizes: false,
    enableFileWatcher: true,
    autoRefreshInterval: 5000,
    cacheThumbnailPreviews: true,
    thumbnailCacheSize: 100,
    deleteToTrash: true,
    singleClickToOpen: false,
    sortFoldersFirst: true,
    copyOnDrop: false,
  },
  editor: {
    fontFamily: 'Consolas, "Courier New", monospace',
    fontSize: 14,
    lineHeight: 1.5,
    wordWrap: 'on',
    wordWrapColumn: 80,
    tabSize: 4,
    insertSpaces: true,
    detectIndentation: true,
    trimTrailingWhitespace: false,
    insertFinalNewline: false,
    trimFinalNewlines: false,
    autoSave: 'off',
    autoSaveDelay: 1000,
    minimap: false,
    renderWhitespace: 'boundary',
    cursorStyle: 'line',
    smoothScrolling: true,
    bracketPairColorization: true,
    showLineNumbers: true,
    codeFolding: true,
    highlightCurrentLine: true,
    occurrencesHighlight: true,
    matchBrackets: 'always',
  },
  view: {
    compactMode: false,
    iconSize: 'medium',
    listViewMode: 'details',
    sortBy: 'name',
    sortOrder: 'asc',
    showStatusBar: true,
    showBreadcrumbs: true,
    showPathBar: true,
    pathDisplayFormat: 'smart',
    animationsEnabled: true,
    showTabBar: true,
    thumbnailsEnabled: true,
    foldersFirst: true,
    tabPosition: 'top',
  },
  advanced: {
    enableLogging: false,
    logLevel: 'info',
    logToFile: false,
    experimentalFeatures: false,
    gpuAcceleration: 'auto',
    memoryCacheSize: 512,
    maxConcurrentOperations: 4,
    networkTimeout: 30000,
    proxyEnabled: false,
    proxyHost: '',
    proxyPort: 8080,
    proxyAuthEnabled: false,
    proxyUsername: '',
    proxyPassword: '',
    useNativeFilePicker: false,
    hardReloadOnExternalChange: false,
    telemetryLevel: 'crash',
  },
};

const STORAGE_KEY = 'project-settings-v1';

interface SettingsDialogRequest {
  resolve(value: boolean): void;
  initialTab?: SettingsTab;
}

interface BackendUserPreferences {
  theme?: string;
  themePreset?: string;
  accentColor?: string;
  uiZoom?: number;
  compactMode?: boolean;
  animationsEnabled?: boolean;
  confirmOnClose?: boolean;
  reopenLastWorkspace?: boolean;
  [key: string]: unknown;
}

const APPEARANCE_CACHE_KEY = 'app-appearance-v1';

interface CachedAppearance {
  theme?: GeneralSettings['theme'];
  themePreset?: string;
  accentColor?: string;
  uiZoom?: number;
  compactMode?: boolean;
  animationsEnabled?: boolean;
}

function readAppearanceCache(): CachedAppearance {
  try {
    const raw = localStorage.getItem(APPEARANCE_CACHE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as CachedAppearance;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function writeAppearanceCache(value: CachedAppearance): void {
  try {
    localStorage.setItem(APPEARANCE_CACHE_KEY, JSON.stringify(value));
  } catch {
    // Ignore storage errors
  }
}

@Injectable({ providedIn: 'root' })
export class SettingsService {
  private readonly themeService = inject(ThemeService, { optional: true });
  private settings$ = createBehaviorSubject<ProjectSettings>(this.loadSettings());
  private readonly dialogGate = new QueuedDialogGate<SettingsDialogRequest, boolean>();
  private backendPreferences: BackendUserPreferences | null = null;

  constructor() {
    this.applyAppearance(this.settings$.value);
    void this.loadBackendPreferences();
  }

  readonly settings: Stream<ProjectSettings> = this.settings$;
  readonly currentSettings = this.settings$.value;

  getSettings$(): Stream<ProjectSettings> {
    return this.settings$;
  }

  getSettings(): ProjectSettings {
    return this.settings$.value;
  }

  updateSettings(settings: Partial<ProjectSettings>): void {
    const current = this.settings$.value;
    const merged: ProjectSettings = {
      general: { ...current.general, ...settings.general },
      files: { ...current.files, ...settings.files },
      editor: { ...current.editor, ...settings.editor },
      view: { ...current.view, ...settings.view },
      advanced: { ...current.advanced, ...settings.advanced },
    };
    this.settings$.next(merged);
    this.applyAppearance(merged);
    this.saveSettings(merged);
  }

  private applyAppearance(settings: ProjectSettings): void {
    this.themeService?.setThemePreference(settings.general.theme);
    this.themeService?.setThemePreset(settings.general.themePreset || 'midnight-default');
    this.themeService?.setCompact(settings.view.compactMode);
  }

  updateGeneralSettings(settings: Partial<GeneralSettings>): void {
    this.updateSettings({ general: { ...this.settings$.value.general, ...settings } });
  }

  updateFileSettings(settings: Partial<FileSettings>): void {
    this.updateSettings({ files: { ...this.settings$.value.files, ...settings } });
  }

  updateEditorSettings(settings: Partial<EditorSettings>): void {
    this.updateSettings({ editor: { ...this.settings$.value.editor, ...settings } });
  }

  updateViewSettings(settings: Partial<ViewSettings>): void {
    this.updateSettings({ view: { ...this.settings$.value.view, ...settings } });
  }

  updateAdvancedSettings(settings: Partial<AdvancedSettings>): void {
    this.updateSettings({ advanced: { ...this.settings$.value.advanced, ...settings } });
  }

  resetToDefaults(): void {
    this.settings$.next(DEFAULT_SETTINGS);
    this.applyAppearance(DEFAULT_SETTINGS);
    this.saveSettings(DEFAULT_SETTINGS);
  }

  resetSection(section: keyof ProjectSettings): void {
    const current = this.settings$.value;
    const updated = { ...current, [section]: DEFAULT_SETTINGS[section] };
    this.settings$.next(updated);
    this.applyAppearance(updated);
    this.saveSettings(updated);
  }

  openSettings(initialTab?: SettingsTab): Promise<boolean> {
    return new Promise<boolean>((resolve) => {
      this.dialogGate.enqueue({
        resolve,
        initialTab,
      });
    });
  }

  closeSettings(confirmed: boolean): void {
    this.dialogGate.beginClose(confirmed);
  }

  getActiveRequest$(): Stream<SettingsDialogRequest | null> {
    return this.dialogGate.activeRequest$;
  }

  getMountedRequest$(): Stream<SettingsDialogRequest | null> {
    return this.dialogGate.mountedRequest$;
  }

  getClosingRequest$(): Stream<SettingsDialogRequest | null> {
    return this.dialogGate.closingRequest$;
  }

  getClosingRequest(): SettingsDialogRequest | null {
    return this.dialogGate.closingRequest$.value;
  }

  finishCloseSettings(): void {
    this.dialogGate.finishClose();
  }

  private loadSettings(): ProjectSettings {
    let result = DEFAULT_SETTINGS;
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed = JSON.parse(stored) as Partial<ProjectSettings>;
        result = this.mergeWithDefaults(parsed);
      }
    } catch {
      // Ignore parse errors
    }

    writeAppearanceCache({
      theme: result.general.theme,
      themePreset: result.general.themePreset,
      accentColor: result.general.accentColor,
      uiZoom: result.general.uiZoom,
      compactMode: result.view.compactMode,
      animationsEnabled: result.view.animationsEnabled,
    });
    return result;
  }

  private saveSettings(settings: ProjectSettings): void {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
    } catch {
      // Ignore storage errors
    }

    void this.syncBackendPreferences(settings);
  }

  private async loadBackendPreferences(): Promise<void> {
    try {
      const preferences = await GetPreferences() as unknown as BackendUserPreferences;
      this.backendPreferences = preferences;

      const current = this.settings$.value;
      const themeValue = this.coerceTheme(preferences.theme, current.general.theme);
      const themePreset = typeof preferences.themePreset === 'string' && preferences.themePreset.trim()
        ? preferences.themePreset.trim()
        : (current.general.themePreset || 'midnight-default');
      const accentColor = typeof preferences.accentColor === 'string'
        ? preferences.accentColor
        : current.general.accentColor;
      const uiZoom = typeof preferences.uiZoom === 'number' && preferences.uiZoom > 0
        ? preferences.uiZoom
        : current.general.uiZoom;
      const compactMode = typeof preferences.compactMode === 'boolean'
        ? preferences.compactMode
        : current.view.compactMode;
      const animationsEnabled = typeof preferences.animationsEnabled === 'boolean'
        ? preferences.animationsEnabled
        : current.view.animationsEnabled;

      const merged: ProjectSettings = {
        ...current,
        general: {
          ...current.general,
          theme: themeValue,
          themePreset,
          accentColor,
          uiZoom,
          confirmOnClose: typeof preferences.confirmOnClose === 'boolean'
            ? preferences.confirmOnClose
            : current.general.confirmOnClose,
          reopenLastWorkspace: typeof preferences.reopenLastWorkspace === 'boolean'
            ? preferences.reopenLastWorkspace
            : current.general.reopenLastWorkspace,
        },
        view: {
          ...current.view,
          compactMode,
          animationsEnabled,
        },
      };

      this.settings$.next(merged);
      this.applyAppearance(merged);
      writeAppearanceCache({
        theme: merged.general.theme,
        themePreset: merged.general.themePreset,
        accentColor: merged.general.accentColor,
        uiZoom: merged.general.uiZoom,
        compactMode: merged.view.compactMode,
        animationsEnabled: merged.view.animationsEnabled,
      });
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(merged));
      } catch {
        // Ignore storage errors
      }
    } catch {
      // Ignore backend availability errors during bootstrap.
    }
  }

  private async syncBackendPreferences(settings: ProjectSettings): Promise<void> {
    writeAppearanceCache({
      theme: settings.general.theme,
      themePreset: settings.general.themePreset,
      accentColor: settings.general.accentColor,
      uiZoom: settings.general.uiZoom,
      compactMode: settings.view.compactMode,
      animationsEnabled: settings.view.animationsEnabled,
    });

    try {
      const current = this.backendPreferences ?? await GetPreferences() as unknown as BackendUserPreferences;
      this.backendPreferences = current;

      await SavePreferences({
        ...current,
        theme: settings.general.theme,
        themePreset: settings.general.themePreset || 'midnight-default',
        accentColor: settings.general.accentColor,
        uiZoom: settings.general.uiZoom,
        compactMode: settings.view.compactMode,
        animationsEnabled: settings.view.animationsEnabled,
        confirmOnClose: settings.general.confirmOnClose,
        reopenLastWorkspace: settings.general.reopenLastWorkspace,
      } as any);
    } catch {
      // Ignore backend sync errors during early startup or tests.
    }
  }

  private coerceTheme(value: unknown, fallback: GeneralSettings['theme']): GeneralSettings['theme'] {
    return value === 'dark' || value === 'light' || value === 'system' ? value : fallback;
  }

  private mergeWithDefaults(stored: Partial<ProjectSettings>): ProjectSettings {
    const files = { ...DEFAULT_SETTINGS.files, ...stored.files };
    if (files.defaultOpenAction !== 'edit' && files.defaultOpenAction !== 'external') {
      files.defaultOpenAction = 'external';
    }

    return {
      general: { ...DEFAULT_SETTINGS.general, ...stored.general },
      files,
      editor: { ...DEFAULT_SETTINGS.editor, ...stored.editor },
      view: { ...DEFAULT_SETTINGS.view, ...stored.view },
      advanced: { ...DEFAULT_SETTINGS.advanced, ...stored.advanced },
    };
  }
}
