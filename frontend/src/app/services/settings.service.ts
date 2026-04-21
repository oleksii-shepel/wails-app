import { Injectable } from '@angular/core';
import { createBehaviorSubject, type Stream } from '@epikodelabs/streamix';

export type SettingsTab = 'general' | 'files' | 'editor' | 'view' | 'advanced';

export interface GeneralSettings {
  theme: 'dark' | 'light' | 'system';
  language: string;
  startupBehavior: 'welcome' | 'lastSession' | 'specificFolder';
  startupFolder: string;
  updateChannel: 'stable' | 'beta' | 'dev';
  analyticsEnabled: boolean;
  crashReportsEnabled: boolean;
}

export interface FileSettings {
  showHiddenFiles: boolean;
  showFileExtensions: boolean;
  confirmBeforeDelete: boolean;
  confirmBeforeMove: boolean;
  confirmBeforeCopy: boolean;
  confirmDragAndDrop: boolean;
  defaultOpenAction: 'preview' | 'edit' | 'external';
  calculateFolderSizes: boolean;
  enableFileWatcher: boolean;
  autoRefreshInterval: number;
  cacheThumbnailPreviews: boolean;
  thumbnailCacheSize: number;
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
  sidebarVisible: boolean;
  sidebarWidth: number;
  previewPaneVisible: boolean;
  previewPaneWidth: number;
  pathDisplayFormat: 'full' | 'short' | 'smart';
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
    language: 'en',
    startupBehavior: 'lastSession',
    startupFolder: '',
    updateChannel: 'stable',
    analyticsEnabled: false,
    crashReportsEnabled: true,
  },
  files: {
    showHiddenFiles: false,
    showFileExtensions: true,
    confirmBeforeDelete: true,
    confirmBeforeMove: true,
    confirmBeforeCopy: false,
    confirmDragAndDrop: true,
    defaultOpenAction: 'preview',
    calculateFolderSizes: false,
    enableFileWatcher: true,
    autoRefreshInterval: 5000,
    cacheThumbnailPreviews: true,
    thumbnailCacheSize: 100,
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
    sidebarVisible: true,
    sidebarWidth: 250,
    previewPaneVisible: false,
    previewPaneWidth: 300,
    pathDisplayFormat: 'smart',
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
  },
};

const STORAGE_KEY = 'project-settings-v1';

interface SettingsDialogRequest {
  resolve: (value: boolean) => void;
  initialTab?: SettingsTab;
}

@Injectable({ providedIn: 'root' })
export class SettingsService {
  private settings$ = createBehaviorSubject<ProjectSettings>(this.loadSettings());
  private activeRequest$ = createBehaviorSubject<SettingsDialogRequest | null>(null);

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
    this.saveSettings(merged);
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
    this.saveSettings(DEFAULT_SETTINGS);
  }

  resetSection(section: keyof ProjectSettings): void {
    const current = this.settings$.value;
    const updated = { ...current, [section]: DEFAULT_SETTINGS[section] };
    this.settings$.next(updated);
    this.saveSettings(updated);
  }

  openSettings(initialTab?: SettingsTab): Promise<boolean> {
    return new Promise<boolean>((resolve) => {
      this.activeRequest$.next({
        resolve,
        initialTab,
      });
    });
  }

  closeSettings(confirmed: boolean): void {
    const request = this.activeRequest$.value;
    if (request) {
      request.resolve(confirmed);
      this.activeRequest$.next(null);
    }
  }

  getActiveRequest$(): Stream<SettingsDialogRequest | null> {
    return this.activeRequest$;
  }

  private loadSettings(): ProjectSettings {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed = JSON.parse(stored) as Partial<ProjectSettings>;
        return this.mergeWithDefaults(parsed);
      }
    } catch {
      // Ignore parse errors
    }
    return DEFAULT_SETTINGS;
  }

  private saveSettings(settings: ProjectSettings): void {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
    } catch {
      // Ignore storage errors
    }
  }

  private mergeWithDefaults(stored: Partial<ProjectSettings>): ProjectSettings {
    return {
      general: { ...DEFAULT_SETTINGS.general, ...stored.general },
      files: { ...DEFAULT_SETTINGS.files, ...stored.files },
      editor: { ...DEFAULT_SETTINGS.editor, ...stored.editor },
      view: { ...DEFAULT_SETTINGS.view, ...stored.view },
      advanced: { ...DEFAULT_SETTINGS.advanced, ...stored.advanced },
    };
  }
}