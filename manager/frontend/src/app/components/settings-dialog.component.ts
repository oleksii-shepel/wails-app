import { writeClipboardText } from '../utils/clipboard-text';
import { CommonModule } from '@angular/common';
import { Component, ElementRef, NgZone, ViewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil, type Stream } from '@epikodelabs/streamix';
import { InputHistoryDirective } from '../directives/input-history.directive';
import { moveFocusWithin } from '../utils/focus-navigation.utils';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import {
  SettingsService,
  SettingsTab,
  type ProjectSettings,
} from '../services/settings.service';
import { FILE_MANAGER_BINDINGS } from './command-registry';
import { KeybindingStorageService } from '../services/keybinding-storage.service';
import type { InputBindingDefinition, KeyStroke, InputTrigger } from '../services/input-system.types';
import { PluginSettingsPanelComponent } from './plugin-settings-panel.component';
import { ApplicationOverlayHostService } from '../services/application-overlay-host.service';
import { CustomizationRegistryService, type ThemeContribution } from '../services/customization-registry.service';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';

interface SettingsDialogRequest {
  resolve: (value: boolean) => void;
  initialTab?: SettingsTab;
}

const SETTINGS_DIALOG_CONTEXT = 'dialog.settings';
const SETTINGS_DIALOG_OWNER = 'dialog.settings';
const SETTINGS_DIALOG_BINDINGS: readonly InputBindingDefinition[] = [
  {
    id: 'dialog.settings.close.escape',
    commandId: 'dialog.settings.close',
    contextId: SETTINGS_DIALOG_CONTEXT,
    source: 'default',
    description: 'Close settings dialog',
    category: 'Navigation',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'escape' }] } },
  },
  {
    id: 'dialog.settings.close.escape.typing',
    commandId: 'dialog.settings.close',
    contextId: SETTINGS_DIALOG_CONTEXT,
    source: 'default',
    description: 'Close settings dialog while typing',
    category: 'Navigation',
    when: 'typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'escape' }] } },
  },
];

@Component({
  selector: 'app-settings-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, InputHistoryDirective, PluginSettingsPanelComponent],
  styleUrls: ['./settings-dialog.component.scss'],
  template: `
    <ng-container *ngIf="request$ | async">
      <div class="settings-overlay palette-dialog-backdrop dialog-group" [class.settings-overlay--closing]="isClosing" [class.dialog-group--closing]="isClosing" (click)="onCancel()" (animationend)="onAnimationEnd($event)">
        <div
          #dialogPanel
          class="settings-panel palette-dialog-surface dialog-surface"
          role="dialog"
          aria-modal="true"
          aria-label="Project Settings"
          tabindex="-1"
          (click)="$event.stopPropagation()"
         (keydown.escape)="onEscapeKey($event)">
          <header class="settings-header">
            <div class="settings-header-title">
              <h2>Settings</h2>
              <span class="settings-breadcrumb">
                <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M6.22 3.22a.75.75 0 0 1 1.06 0l4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.75.75 0 0 1-1.06-1.06L9.94 8 6.22 4.28a.75.75 0 0 1 0-1.06z"/></svg>
                {{ getTabLabel() }}
              </span>
            </div>
            <div class="settings-header-actions">
              <button
                *ngIf="activeTab !== 'plugins'"
                type="button"
                class="btn btn-secondary btn-small"
                (click)="onResetSection()"
                aria-label="Reset current section to defaults" title="Reset current section to defaults"
              >
                Reset Section
              </button>
              <button
                *ngIf="activeTab !== 'plugins'"
                type="button"
                class="btn btn-secondary btn-small"
                (click)="onResetAll()"
                aria-label="Reset all settings to defaults" title="Reset all settings to defaults"
              >
                Reset All
              </button>
              <button type="button" class="dialog-close" aria-label="Close dialog" (click)="onCancel()" title="Close (Esc)"> × </button>
            </div>
          </header>

          <div class="settings-body">
            <aside class="settings-sidebar">
              <nav class="settings-nav">
                <ng-container *ngFor="let tab of tabs">
                  <div *ngIf="tab.divider" class="nav-divider"></div>
                  <button
                    type="button"
                    class="nav-item"
                    [class.active]="activeTab === tab.id"
                    [attr.aria-selected]="activeTab === tab.id"
                    (click)="setActiveTab(tab.id)"
                  >
                    <span class="nav-icon" [innerHTML]="tab.icon"></span>
                    <span class="nav-label">{{ tab.label }}</span>
                  </button>
                </ng-container>
              </nav>
            </aside>

            <main class="settings-content">
              <!-- General Settings -->
              <section *ngIf="activeTab === 'general'" class="settings-section">
                <h3 class="section-title">General</h3>
                <p class="section-desc">Configure application appearance, startup behavior, and privacy preferences.</p>
                
                <div class="settings-group">
                  <h4 class="group-title">Appearance</h4>
                  <div class="field-row">
                      <label class="field">
                        <span id="settings-theme-label">Theme Mode</span>
                        <select aria-labelledby="settings-theme-label" aria-describedby="settings-theme-help" [(ngModel)]="draft.general.theme" (ngModelChange)="onSettingChanged()">
                          <option value="dark">Dark</option>
                          <option value="light">Light</option>
                          <option value="system">System Preference</option>
                        </select>
                        <small id="settings-theme-help">Base dark/light mode. "System Preference" follows your OS setting.</small>
                      </label>
                      <label class="field">
                        <span id="settings-theme-preset-label">Theme Preset</span>
                        <select aria-labelledby="settings-theme-preset-label" aria-describedby="settings-theme-preset-help" [(ngModel)]="draft.general.themePreset" (ngModelChange)="onSettingChanged()">
                          @for (preset of themePresets; track preset.id) {
                            <option [value]="preset.id">{{ preset.label }}</option>
                          }
                        </select>
                        <small id="settings-theme-preset-help">Extensible palette preset (built-in and plugin-contributed themes).</small>
                      </label>
                    <label class="field">
                      <span id="settings-ui-zoom-label">UI Zoom</span>
                      <select aria-labelledby="settings-ui-zoom-label" aria-describedby="settings-ui-zoom-help" [(ngModel)]="draft.general.uiZoom" (ngModelChange)="onSettingChanged()">
                        <option [ngValue]="75">75%</option>
                        <option [ngValue]="90">90%</option>
                        <option [ngValue]="100">100% (Default)</option>
                        <option [ngValue]="110">110%</option>
                        <option [ngValue]="125">125%</option>
                        <option [ngValue]="150">150%</option>
                      </select>
                      <small id="settings-ui-zoom-help">Scales all UI elements. Requires a restart to fully apply.</small>
                    </label>
                  </div>
                  <label class="field color-field">
                    <span>Accent Color</span>
                    <div class="color-input-row">
                      <input
                        type="color"
                        [(ngModel)]="draft.general.accentColor"
                        (ngModelChange)="onSettingChanged()"
                        class="color-swatch"
                      />
                      <input
                        type="text"
                        [(ngModel)]="draft.general.accentColor"
                        (ngModelChange)="onSettingChanged()"
                        placeholder="var(--vsc-accent-blue)"
                        class="color-text"
                      />
                      <button
                        type="button"
                        class="btn btn-secondary btn-small"
                        (click)="draft.general.accentColor = ''; onSettingChanged()"
                        aria-label="Reset to default accent color" title="Reset to default accent color"
                      >Reset</button>
                    </div>
                    <small>Overrides the default blue highlight color for active items, selections, and focus rings. Leave empty to use the default.</small>
                  </label>
                  <label class="field">
                    <span id="settings-language-label">Language</span>
                    <select aria-labelledby="settings-language-label" aria-describedby="settings-language-help" [(ngModel)]="draft.general.language" (ngModelChange)="onSettingChanged()">
                      <option value="en">English</option>
                      <option value="es">Spanish</option>
                      <option value="fr">French</option>
                      <option value="de">German</option>
                      <option value="it">Italian</option>
                      <option value="pt">Portuguese</option>
                      <option value="ru">Russian</option>
                      <option value="zh">Chinese</option>
                      <option value="ja">Japanese</option>
                      <option value="ko">Korean</option>
                    </select>
                    <small id="settings-language-help">Sets the interface display language. Requires a restart to fully apply.</small>
                  </label>
                  <div class="field-row">
                    <label class="field">
                      <span id="settings-date-format-label">Date Format</span>
                      <select aria-labelledby="settings-date-format-label" aria-describedby="settings-date-format-help" [(ngModel)]="draft.general.dateFormat" (ngModelChange)="onSettingChanged()">
                        <option value="relative">Relative (2 hours ago)</option>
                        <option value="short">Short (Apr 21, 2026)</option>
                        <option value="long">Long (April 21, 2026 10:30 AM)</option>
                        <option value="iso">ISO 8601 (2026-04-21T10:30)</option>
                      </select>
                      <small id="settings-date-format-help">How timestamps appear in file lists and the details pane.</small>
                    </label>
                    <label class="field">
                      <span id="settings-size-unit-label">Size Unit</span>
                      <select aria-labelledby="settings-size-unit-label" aria-describedby="settings-size-unit-help" [(ngModel)]="draft.general.sizeUnit" (ngModelChange)="onSettingChanged()">
                        <option value="auto">Automatic</option>
                        <option value="bytes">Bytes</option>
                        <option value="kib">KiB / KB</option>
                        <option value="mib">MiB / MB</option>
                        <option value="gib">GiB / GB</option>
                      </select>
                      <small id="settings-size-unit-help">Unit used when displaying file sizes across all panes.</small>
                    </label>
                  </div>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Window Behavior</h4>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-confirm-on-close-label" aria-describedby="settings-confirm-on-close-help" type="checkbox" [(ngModel)]="draft.general.confirmOnClose" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-confirm-on-close-label">Confirm before closing</span><small id="settings-confirm-on-close-help">Shows a confirmation dialog before the application window closes.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-reopen-last-workspace-label" aria-describedby="settings-reopen-last-workspace-help" type="checkbox" [(ngModel)]="draft.general.reopenLastWorkspace" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-reopen-last-workspace-label">Reopen last workspace on launch</span><small id="settings-reopen-last-workspace-help">Automatically activates the most recently used workspace when the app starts.</small></span>
                  </label>
                  <p class="section-note">Background tray mode is not available in the current desktop shell, so closing the window either confirms and exits or exits immediately.</p>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Startup</h4>
                  <label class="field">
                    <span>Startup Behavior</span>
                    <select [(ngModel)]="draft.general.startupBehavior" (ngModelChange)="onSettingChanged()">
                      <option value="welcome">Show Welcome Page</option>
                      <option value="lastSession">Restore Last Session</option>
                      <option value="specificFolder">Open Specific Folder</option>
                    </select>
                  </label>
                  <label class="field" *ngIf="draft.general.startupBehavior === 'specificFolder'">
                    <span>Startup Folder</span>
                    <input
                      type="text"
                      name="settings-startup-folder"
                      [(ngModel)]="draft.general.startupFolder"
                      (ngModelChange)="onSettingChanged()"
                      placeholder="/path/to/folder"
                      [appInputHistory]="'settings.startupFolder'"
                    />
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Updates & Privacy</h4>
                  <label class="field">
                    <span id="settings-update-channel-label">Update Channel</span>
                    <select aria-labelledby="settings-update-channel-label" aria-describedby="settings-update-channel-help" [(ngModel)]="draft.general.updateChannel" (ngModelChange)="onSettingChanged()">
                      <option value="stable">Stable</option>
                      <option value="beta">Beta</option>
                      <option value="dev">Development</option>
                    </select>
                    <small id="settings-update-channel-help">Stable is recommended for most users. Beta and Dev channels receive updates earlier but may be less stable.</small>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-check-updates-on-startup-label" aria-describedby="settings-check-updates-on-startup-help" type="checkbox" [(ngModel)]="draft.general.checkUpdatesOnStartup" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-check-updates-on-startup-label">Check for updates on startup</span><small id="settings-check-updates-on-startup-help">Automatically checks for a newer version each time the application launches.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-analytics-enabled-label" aria-describedby="settings-analytics-enabled-help" type="checkbox" [(ngModel)]="draft.general.analyticsEnabled" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-analytics-enabled-label">Enable anonymous analytics</span><small id="settings-analytics-enabled-help">Sends non-identifiable usage data to help improve the product.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-crash-reports-enabled-label" aria-describedby="settings-crash-reports-enabled-help" type="checkbox" [(ngModel)]="draft.general.crashReportsEnabled" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-crash-reports-enabled-label">Send crash reports automatically</span><small id="settings-crash-reports-enabled-help">Uploads anonymized crash diagnostics to help identify and fix bugs.</small></span>
                  </label>
                </div>
              </section>

              <!-- File Settings -->
              <section *ngIf="activeTab === 'files'" class="settings-section">
                <h3 class="section-title">Files</h3>
                <p class="section-desc">Control how files are displayed, handled, and cached in the browser panes.</p>

                <div class="settings-group">
                  <h4 class="group-title">Display</h4>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-show-hidden-files-label" aria-describedby="settings-show-hidden-files-help" type="checkbox" [(ngModel)]="draft.files.showHiddenFiles" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-show-hidden-files-label">Show hidden files</span><small id="settings-show-hidden-files-help">Displays files and folders whose names start with a dot or have the hidden attribute set.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-show-file-extensions-label" aria-describedby="settings-show-file-extensions-help" type="checkbox" [(ngModel)]="draft.files.showFileExtensions" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-show-file-extensions-label">Show file extensions</span><small id="settings-show-file-extensions-help">Appends the extension (.txt, .jpg …) to every filename in the list.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-sort-folders-first-label" aria-describedby="settings-sort-folders-first-help" type="checkbox" [(ngModel)]="draft.files.sortFoldersFirst" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-sort-folders-first-label">Sort folders before files</span><small id="settings-sort-folders-first-help">Keeps directories grouped at the top regardless of the active sort column.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-single-click-to-open-label" aria-describedby="settings-single-click-to-open-help" type="checkbox" [(ngModel)]="draft.files.singleClickToOpen" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-single-click-to-open-label">Single-click to open</span><small id="settings-single-click-to-open-help">Opens files and navigates into folders with a single click instead of a double-click.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-calculate-folder-sizes-label" aria-describedby="settings-calculate-folder-sizes-help" type="checkbox" [(ngModel)]="draft.files.calculateFolderSizes" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-calculate-folder-sizes-label">Calculate folder sizes</span><small id="settings-calculate-folder-sizes-help">Recursively sums directory contents to show sizes. May impact performance on large trees.</small></span>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">File Operations</h4>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-confirm-before-delete-label" aria-describedby="settings-confirm-before-delete-help" type="checkbox" [(ngModel)]="draft.files.confirmBeforeDelete" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-confirm-before-delete-label">Confirm before deleting files</span><small id="settings-confirm-before-delete-help">Shows a confirmation prompt before any delete operation.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-delete-to-trash-label" aria-describedby="settings-delete-to-trash-help" type="checkbox" [(ngModel)]="draft.files.deleteToTrash" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-delete-to-trash-label">Send to Recycle Bin</span><small id="settings-delete-to-trash-help">Moves deleted files to the system trash instead of permanently erasing them.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.confirmBeforeMove" (ngModelChange)="onSettingChanged()" />
                    <span>Confirm before moving files</span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.confirmBeforeCopy" (ngModelChange)="onSettingChanged()" />
                    <span>Confirm before copying files</span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.confirmDragAndDrop" (ngModelChange)="onSettingChanged()" />
                    <span>Confirm drag and drop operations</span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-copy-on-drop-label" aria-describedby="settings-copy-on-drop-help" type="checkbox" [(ngModel)]="draft.files.copyOnDrop" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-copy-on-drop-label">Copy on drag and drop by default</span><small id="settings-copy-on-drop-help">When dropping without a modifier key, copies instead of moving the files.</small></span>
                  </label>
                  <label class="field">
                    <span>Default Open Action</span>
                    <select [(ngModel)]="draft.files.defaultOpenAction" (ngModelChange)="onSettingChanged()">
                      <option value="edit">Edit</option>
                      <option value="external">Open in External Application</option>
                    </select>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">File Watching & Cache</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.enableFileWatcher" (ngModelChange)="onSettingChanged()" />
                    <span>Enable file system watcher</span>
                  </label>
                  <label class="field">
                    <span id="settings-auto-refresh-interval-label">Auto-refresh Interval (ms)</span>
                    <input aria-labelledby="settings-auto-refresh-interval-label" aria-describedby="settings-auto-refresh-interval-help"
                      type="number"
                      [(ngModel)]="draft.files.autoRefreshInterval"
                      (ngModelChange)="onSettingChanged()"
                      min="1000"
                      max="60000"
                      step="1000"
                    />
                    <small id="settings-auto-refresh-interval-help">How often to poll the filesystem for external changes. Lower values use more CPU.</small>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.cacheThumbnailPreviews" (ngModelChange)="onSettingChanged()" />
                    <span>Cache thumbnail previews</span>
                  </label>
                  <label class="field">
                    <span id="settings-thumbnail-cache-size-label">Thumbnail Cache Size (MB)</span>
                    <input aria-labelledby="settings-thumbnail-cache-size-label" aria-describedby="settings-thumbnail-cache-size-help"
                      type="number"
                      [(ngModel)]="draft.files.thumbnailCacheSize"
                      (ngModelChange)="onSettingChanged()"
                      min="10"
                      max="1000"
                      step="10"
                    />
                    <small id="settings-thumbnail-cache-size-help">Maximum disk space reserved for cached image and video thumbnails.</small>
                  </label>
                </div>
              </section>

              <!-- Editor Settings -->
              <section *ngIf="activeTab === 'editor'" class="settings-section">
                <h3 class="section-title">Editor</h3>
                <p class="section-desc">Customize the text editor font, indentation style, formatting rules, and autosave behavior.</p>

                <div class="settings-group">
                  <h4 class="group-title">Font</h4>
                  <label class="field">
                    <span id="settings-font-family-label">Font Family</span>
                    <input aria-labelledby="settings-font-family-label" aria-describedby="settings-font-family-help"
                      type="text"
                      name="settings-font-family"
                      [(ngModel)]="draft.editor.fontFamily"
                      (ngModelChange)="onSettingChanged()"
                      placeholder="Consolas, 'Courier New', monospace"
                      [appInputHistory]="'settings.fontFamily'"
                    />
                    <small id="settings-font-family-help">Comma-separated list of font names. The first available font on your system is used.</small>
                  </label>
                  <div class="field-row">
                    <label class="field">
                      <span>Font Size (px)</span>
                      <input
                        type="number"
                        [(ngModel)]="draft.editor.fontSize"
                        (ngModelChange)="onSettingChanged()"
                        min="8"
                        max="32"
                      />
                    </label>
                    <label class="field">
                      <span>Line Height</span>
                      <input
                        type="number"
                        [(ngModel)]="draft.editor.lineHeight"
                        (ngModelChange)="onSettingChanged()"
                        min="1"
                        max="3"
                        step="0.1"
                      />
                    </label>
                  </div>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Editing</h4>
                  <label class="field">
                    <span id="settings-word-wrap-label">Word Wrap</span>
                    <select aria-labelledby="settings-word-wrap-label" aria-describedby="settings-word-wrap-help" [(ngModel)]="draft.editor.wordWrap" (ngModelChange)="onSettingChanged()">
                      <option value="off">Off</option>
                      <option value="on">On</option>
                      <option value="wordWrapColumn">At Column</option>
                      <option value="bounded">Bounded</option>
                    </select>
                    <small id="settings-word-wrap-help">How long lines are wrapped in the editor. "At Column" and "Bounded" wrap at a fixed character width.</small>
                  </label>
                  <label class="field" *ngIf="draft.editor.wordWrap === 'wordWrapColumn' || draft.editor.wordWrap === 'bounded'">
                    <span>Word Wrap Column</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.editor.wordWrapColumn"
                      (ngModelChange)="onSettingChanged()"
                      min="20"
                      max="200"
                    />
                  </label>
                  <div class="field-row">
                    <label class="field">
                      <span>Tab Size</span>
                      <input
                        type="number"
                        [(ngModel)]="draft.editor.tabSize"
                        (ngModelChange)="onSettingChanged()"
                        min="1"
                        max="8"
                      />
                    </label>
                    <label class="toggle-field inline">
                      <input type="checkbox" [(ngModel)]="draft.editor.insertSpaces" (ngModelChange)="onSettingChanged()" />
                      <span>Insert spaces</span>
                    </label>
                  </div>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.detectIndentation" (ngModelChange)="onSettingChanged()" />
                    <span>Detect indentation</span>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Formatting</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.trimTrailingWhitespace" (ngModelChange)="onSettingChanged()" />
                    <span>Trim trailing whitespace</span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.insertFinalNewline" (ngModelChange)="onSettingChanged()" />
                    <span>Insert final newline</span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.trimFinalNewlines" (ngModelChange)="onSettingChanged()" />
                    <span>Trim final newlines</span>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Auto Save</h4>
                  <label class="field">
                    <span id="settings-auto-save-label">Auto Save</span>
                    <select aria-labelledby="settings-auto-save-label" aria-describedby="settings-auto-save-help" [(ngModel)]="draft.editor.autoSave" (ngModelChange)="onSettingChanged()">
                      <option value="off">Off</option>
                      <option value="afterDelay">After Delay</option>
                      <option value="onFocusChange">On Focus Change</option>
                      <option value="onWindowChange">On Window Change</option>
                    </select>
                    <small id="settings-auto-save-help">Automatically saves the open file without requiring an explicit save action.</small>
                  </label>
                  <label class="field" *ngIf="draft.editor.autoSave === 'afterDelay'">
                    <span>Auto Save Delay (ms)</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.editor.autoSaveDelay"
                      (ngModelChange)="onSettingChanged()"
                      min="100"
                      max="10000"
                      step="100"
                    />
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Display</h4>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-show-line-numbers-label" aria-describedby="settings-show-line-numbers-help" type="checkbox" [(ngModel)]="draft.editor.showLineNumbers" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-show-line-numbers-label">Show line numbers</span><small id="settings-show-line-numbers-help">Renders line numbers in the left gutter of the editor.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-code-folding-label" aria-describedby="settings-code-folding-help" type="checkbox" [(ngModel)]="draft.editor.codeFolding" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-code-folding-label">Enable code folding</span><small id="settings-code-folding-help">Shows fold controls in the gutter to collapse and expand sections of code.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input aria-labelledby="settings-highlight-current-line-label" aria-describedby="settings-highlight-current-line-help" type="checkbox" [(ngModel)]="draft.editor.highlightCurrentLine" (ngModelChange)="onSettingChanged()" />
                    <span><span id="settings-highlight-current-line-label">Highlight current line</span><small id="settings-highlight-current-line-help">Applies a subtle background tint to the line containing the cursor.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.occurrencesHighlight" (ngModelChange)="onSettingChanged()" />
                    <span>Highlight all occurrences<small>When a word is selected, highlights all other matching occurrences in the file.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.minimap" (ngModelChange)="onSettingChanged()" />
                    <span>Show minimap<small>Displays a scaled-down overview of the file in the right scroll margin.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.smoothScrolling" (ngModelChange)="onSettingChanged()" />
                    <span>Smooth scrolling<small>Animates scroll events for a more fluid experience in the editor.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.bracketPairColorization" (ngModelChange)="onSettingChanged()" />
                    <span>Bracket pair colorization<small>Colors matching bracket pairs with distinct hues to aid visual grouping.</small></span>
                  </label>
                  <div class="field-row">
                    <label class="field">
                      <span>Cursor Style</span>
                      <select [(ngModel)]="draft.editor.cursorStyle" (ngModelChange)="onSettingChanged()">
                        <option value="line">Line ( | )</option>
                        <option value="block">Block ( █ )</option>
                        <option value="underline">Underline ( _ )</option>
                      </select>
                    </label>
                    <label class="field">
                      <span>Match Brackets</span>
                      <select [(ngModel)]="draft.editor.matchBrackets" (ngModelChange)="onSettingChanged()">
                        <option value="never">Never</option>
                        <option value="near">Near cursor</option>
                        <option value="always">Always</option>
                      </select>
                      <small>Controls when matching bracket pairs are highlighted.</small>
                    </label>
                  </div>
                  <label class="field">
                    <span>Render Whitespace</span>
                    <select [(ngModel)]="draft.editor.renderWhitespace" (ngModelChange)="onSettingChanged()">
                      <option value="none">None</option>
                      <option value="boundary">Boundary</option>
                      <option value="selection">Selection only</option>
                      <option value="all">All</option>
                    </select>
                    <small>Controls when space and tab characters are shown as visible glyphs.</small>
                  </label>
                </div>
              </section>

              <!-- View Settings -->
              <section *ngIf="activeTab === 'view'" class="settings-section">
                <h3 class="section-title">View</h3>
                <p class="section-desc">Adjust panel visibility, file list display mode, sorting, and default pane widths.</p>

                <div class="settings-group">
                  <h4 class="group-title">Layout</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.compactMode" (ngModelChange)="onSettingChanged()" />
                    <span>Compact mode<small>Reduces padding and font size throughout the UI for a denser layout.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.animationsEnabled" (ngModelChange)="onSettingChanged()" />
                    <span>Enable animations<small>Smooth transitions for panel openings, dialog entries, and list updates.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.showTabBar" (ngModelChange)="onSettingChanged()" />
                    <span>Show tab bar<small>Displays the tab strip above the file pane for switching between open locations.</small></span>
                  </label>
                  <label class="field" *ngIf="draft.view.showTabBar">
                    <span>Tab Position</span>
                    <select [(ngModel)]="draft.view.tabPosition" (ngModelChange)="onSettingChanged()">
                      <option value="top">Top</option>
                      <option value="bottom">Bottom</option>
                    </select>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.showStatusBar" (ngModelChange)="onSettingChanged()" />
                    <span>Show status bar</span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.showBreadcrumbs" (ngModelChange)="onSettingChanged()" />
                    <span>Show breadcrumbs</span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.showPathBar" (ngModelChange)="onSettingChanged()" />
                    <span>Show path bar</span>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">File List</h4>
                  <label class="field">
                    <span>Icon Size</span>
                    <select [(ngModel)]="draft.view.iconSize" (ngModelChange)="onSettingChanged()">
                      <option value="small">Small</option>
                      <option value="medium">Medium</option>
                      <option value="large">Large</option>
                    </select>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.thumbnailsEnabled" (ngModelChange)="onSettingChanged()" />
                    <span>Show image thumbnails<small>Renders previews for image files. May slow down folder loading for large directories.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.foldersFirst" (ngModelChange)="onSettingChanged()" />
                    <span>Sort folders before files<small>Keeps directories at the top of the file list regardless of the active sort column.</small></span>
                  </label>
                  <label class="field">
                    <span>View Mode</span>
                    <select [(ngModel)]="draft.view.listViewMode" (ngModelChange)="onSettingChanged()">
                      <option value="details">Details</option>
                      <option value="icons">Icons</option>
                      <option value="tiles">Tiles</option>
                    </select>
                  </label>
                  <div class="field-row">
                    <label class="field">
                      <span>Sort By</span>
                      <select [(ngModel)]="draft.view.sortBy" (ngModelChange)="onSettingChanged()">
                        <option value="name">Name</option>
                        <option value="size">Size</option>
                        <option value="modified">Modified Date</option>
                        <option value="created">Created Date</option>
                        <option value="type">Type</option>
                      </select>
                    </label>
                    <label class="field">
                      <span>Sort Order</span>
                      <select [(ngModel)]="draft.view.sortOrder" (ngModelChange)="onSettingChanged()">
                        <option value="asc">Ascending</option>
                        <option value="desc">Descending</option>
                      </select>
                    </label>
                  </div>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Navigation</h4>
                  <label class="field">
                    <span>Path Display Format</span>
                    <select [(ngModel)]="draft.view.pathDisplayFormat" (ngModelChange)="onSettingChanged()">
                      <option value="full">Full Path</option>
                      <option value="short">Short Name</option>
                      <option value="smart">Smart (context-aware)</option>
                    </select>
                  </label>
                  <p class="section-note">This build uses inline pane surfaces and viewer tabs. Separate sidebar and detached preview-pane layout options are not available.</p>
                </div>
              </section>

              <!-- Advanced Settings -->
              <section *ngIf="activeTab === 'advanced'" class="settings-section">
                <h3 class="section-title">Advanced</h3>
                <p class="section-desc">Tune performance limits, diagnostic logging verbosity, and network proxy settings. Changes here may affect stability.</p>

                <div class="settings-group">
                  <h4 class="group-title">Performance</h4>
                  <label class="field">
                    <span>GPU Acceleration</span>
                    <select [(ngModel)]="draft.advanced.gpuAcceleration" (ngModelChange)="onSettingChanged()">
                      <option value="auto">Automatic</option>
                      <option value="on">Enabled</option>
                      <option value="off">Disabled</option>
                    </select>
                    <small>Hardware-accelerated rendering. Disable if you encounter graphical artifacts or driver crashes.</small>
                  </label>
                  <label class="field">
                    <span>Memory Cache Size (MB)</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.advanced.memoryCacheSize"
                      (ngModelChange)="onSettingChanged()"
                      min="64"
                      max="4096"
                      step="64"
                    />
                    <small>In-memory buffer for file metadata and preview data. Higher values improve responsiveness at the cost of RAM.</small>
                  </label>
                  <label class="field">
                    <span>Max Concurrent Operations</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.advanced.maxConcurrentOperations"
                      (ngModelChange)="onSettingChanged()"
                      min="1"
                      max="16"
                    />
                    <small>Number of file copy, move, and delete tasks that can run in parallel.</small>
                  </label>
                  <label class="field">
                    <span>Network Timeout (ms)</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.advanced.networkTimeout"
                      (ngModelChange)="onSettingChanged()"
                      min="5000"
                      max="120000"
                      step="5000"
                    />
                    <small>Maximum time to wait for a remote network call before treating it as failed.</small>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Logging</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.advanced.enableLogging" (ngModelChange)="onSettingChanged()" />
                    <span>Enable logging</span>
                  </label>
                  <label class="field" *ngIf="draft.advanced.enableLogging">
                    <span>Log Level</span>
                    <select [(ngModel)]="draft.advanced.logLevel" (ngModelChange)="onSettingChanged()">
                      <option value="error">Error</option>
                      <option value="warn">Warning</option>
                      <option value="info">Info</option>
                      <option value="debug">Debug</option>
                      <option value="trace">Trace</option>
                    </select>
                  </label>
                  <label class="toggle-field" *ngIf="draft.advanced.enableLogging">
                    <input type="checkbox" [(ngModel)]="draft.advanced.logToFile" (ngModelChange)="onSettingChanged()" />
                    <span>Log to file</span>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Proxy</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.advanced.proxyEnabled" (ngModelChange)="onSettingChanged()" />
                    <span>Enable proxy</span>
                  </label>
                  <ng-container *ngIf="draft.advanced.proxyEnabled">
                    <div class="field-row">
                      <label class="field">
                        <span>Host</span>
                        <input
                          type="text"
                          name="settings-proxy-host"
                          [(ngModel)]="draft.advanced.proxyHost"
                          (ngModelChange)="onSettingChanged()"
                          placeholder="proxy.example.com"
                          [appInputHistory]="'settings.proxyHost'"
                        />
                      </label>
                      <label class="field">
                        <span>Port</span>
                        <input
                          type="number"
                          [(ngModel)]="draft.advanced.proxyPort"
                          (ngModelChange)="onSettingChanged()"
                          min="1"
                          max="65535"
                        />
                      </label>
                    </div>
                    <label class="toggle-field">
                      <input type="checkbox" [(ngModel)]="draft.advanced.proxyAuthEnabled" (ngModelChange)="onSettingChanged()" />
                      <span>Authentication required</span>
                    </label>
                    <ng-container *ngIf="draft.advanced.proxyAuthEnabled">
                      <label class="field">
                        <span>Username</span>
                        <input
                          type="text"
                          name="settings-proxy-user"
                          [(ngModel)]="draft.advanced.proxyUsername"
                          (ngModelChange)="onSettingChanged()"
                          [appInputHistory]="'settings.proxyUsername'"
                        />
                      </label>
                      <label class="field">
                        <span>Password</span>
                        <input
                          type="password"
                          [(ngModel)]="draft.advanced.proxyPassword"
                          (ngModelChange)="onSettingChanged()"
                        />
                      </label>
                    </ng-container>
                  </ng-container>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Experimental</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.advanced.experimentalFeatures" (ngModelChange)="onSettingChanged()" />
                    <span>Enable experimental features</span>
                  </label>
                  <p class="field-hint" *ngIf="draft.advanced.experimentalFeatures">
                    Warning: Experimental features may be unstable and change without notice.
                  </p>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Behavior</h4>
                  <label class="field">
                    <span>Telemetry Level</span>
                    <select [(ngModel)]="draft.advanced.telemetryLevel" (ngModelChange)="onSettingChanged()">
                      <option value="none">None — no data sent</option>
                      <option value="error">Errors only</option>
                      <option value="crash">Crashes &amp; errors</option>
                      <option value="all">All — usage + crashes</option>
                    </select>
                    <small>Granular control over what diagnostic data is transmitted. Overrides the individual analytics and crash-report toggles in General.</small>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.advanced.useNativeFilePicker" (ngModelChange)="onSettingChanged()" />
                    <span>Use native file picker<small>Opens the OS file chooser dialog for save/open prompts instead of the built-in browser.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.advanced.hardReloadOnExternalChange" (ngModelChange)="onSettingChanged()" />
                    <span>Hard reload on external change<small>Fully refreshes the pane when files are modified externally instead of merging changes incrementally.</small></span>
                  </label>
                </div>
              </section>

              <!-- Plugins -->
              <section *ngIf="activeTab === 'plugins'" class="settings-section plugins-section">
                <app-plugin-settings-panel
                  (dirtyChange)="onPluginDirtyChange($event)">
                </app-plugin-settings-panel>
              </section>

              <!-- Shortcuts -->
              <section *ngIf="activeTab === 'shortcuts'" class="settings-section" (keydown)="onShortcutsKeydown($event)">
                <h3 class="section-title">Keyboard Shortcuts</h3>
                <p class="section-desc">Click any shortcut to customize it. Press Escape to cancel recording.</p>

                <div class="shortcuts-toolbar">
                  <input
                    type="text"
                    [(ngModel)]="shortcutSearch"
                    (ngModelChange)="onShortcutSearchChange()"
                    placeholder="Search shortcuts..."
                    class="shortcut-search"
                  />
                  <button type="button" class="btn btn-secondary btn-small" (click)="resetAllKeybindings()">Reset All</button>
                </div>

                <div *ngIf="recordingBindingId" class="recording-banner">
                  <span>
                    Recording new shortcut for <strong>{{ recordingDescription }}</strong>
                    <span *ngIf="recordingSteps.length > 0" class="recording-steps">
                      <ng-container *ngFor="let step of recordingSteps; let last = last">
                        <kbd class="key-badge">{{ formatStep(step) }}</kbd>
                        <span *ngIf="!last" class="chord-then">then</span>
                      </ng-container>
                    </span>
                  </span>
                  <div class="recording-actions">
                    <button *ngIf="recordingSteps.length > 0" type="button" class="btn btn-small btn-primary" (click)="finishRecording()">Finish</button>
                    <button type="button" class="btn btn-small" (click)="cancelRecording()">Cancel</button>
                  </div>
                </div>

                <ng-container *ngFor="let group of shortcutGroupsDynamic">
                  <div class="settings-group shortcuts-group">
                    <h4 class="group-title">{{ group.category }}</h4>
                    <div class="shortcuts-table">
                      <div
                        class="shortcut-row"
                        *ngFor="let item of group.items"
                        [class.recording]="recordingBindingId === item.id"
                        [class.disabled]="item.enabled === false"
                      >
                        <span class="shortcut-desc">{{ item.description }}</span>
                        <div class="shortcut-actions">
                          <button
                            type="button"
                            class="shortcut-key-btn"
                            (click)="startRecording(item.id)"
                            [disabled]="!!recordingBindingId && recordingBindingId !== item.id"
                          >
                            <ng-container *ngIf="recordingBindingId !== item.id">
                              <span *ngFor="let chord of item.chords; let last = last" class="shortcut-chord">
                                <kbd class="key-badge" *ngFor="let k of chord">{{ k }}</kbd>
                              </span>
                              <span *ngIf="item.enabled === false" class="shortcut-disabled-tag">Disabled</span>
                            </ng-container>
                            <span *ngIf="recordingBindingId === item.id" class="recording-hint">
                              <ng-container *ngIf="recordingSteps.length === 0">Press keys...</ng-container>
                              <ng-container *ngIf="recordingSteps.length > 0">Next step...</ng-container>
                            </span>
                          </button>
                          <button
                            type="button"
                            class="btn btn-small btn-icon"
                            (click)="toggleBindingEnabled(item.id)" aria-label="Enable shortcut" [attr.aria-pressed]="item.enabled !== false"
                            [title]="item.enabled === false ? 'Enable' : 'Disable'"
                          >
                            <svg *ngIf="item.enabled !== false" width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M10.5 8a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0z"/><path d="M0 8s3-5.5 8-5.5S16 8 16 8s-3 5.5-8 5.5S0 8 0 8zm8 3.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z"/></svg>
                            <svg *ngIf="item.enabled === false" width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M13.359 11.238C15.06 9.72 16 8 16 8s-3-5.5-8-5.5a7.028 7.028 0 0 0-2.79.588l.77.771A5.944 5.944 0 0 1 8 3.5c2.12 0 3.879 1.168 5.168 2.457A13.134 13.134 0 0 1 14.828 8c-.058.087-.122.183-.195.288-.335.48-.83 1.12-1.465 1.755-.165.165-.337.328-.517.486l.708.709z"/><path d="M11.297 9.176a3.5 3.5 0 0 0-4.474-4.474l.823.823a2.5 2.5 0 0 1 2.829 2.829l.822.822zm-2.943 1.299.822.822a3.5 3.5 0 0 1-4.474-4.474l.823.823a2.5 2.5 0 0 0 2.829 2.829z"/><path d="M3.35 5.47c-.18.16-.353.322-.518.487A13.134 13.134 0 0 0 1.172 8l.195.288c.335.48.83 1.12 1.465 1.755C4.121 11.332 5.881 12.5 8 12.5c.716 0 1.39-.133 2.02-.36l.77.772A7.028 7.028 0 0 1 8 13.5C3 13.5 0 8 0 8s.939-1.721 2.641-3.238l.708.709z"/><path d="M10.084 6.906a5.002 5.002 0 0 1 1.753 3.134l.708.709a6.002 6.002 0 0 0-2.163-3.325l-.298.482zM8.293 5.293 4.525 9.06l-.707-.707L7.586 4.586l.707.707z"/></svg>
                          </button>
                          <button
                            type="button"
                            class="btn btn-small btn-icon"
                            (click)="resetKeybinding(item.id)"
                            aria-label="Reset shortcut to default" title="Reset to default"
                          >
                            <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path fill-rule="evenodd" d="M8 3a5 5 0 1 0 4.546 2.914.5.5 0 0 1 .908-.417A6 6 0 1 1 8 2v1z"/><path d="M8 4.466V.534a.25.25 0 0 1 .41-.192l2.36 1.966c.12.1.12.284 0 .384L8.41 4.658A.25.25 0 0 1 8 4.466z"/></svg>
                          </button>
                        </div>
                        <div *ngIf="hasConflict(item.id)" class="shortcut-conflict">{{ getConflictMessage(item.id) }}</div>
                      </div>
                    </div>
                  </div>
                </ng-container>

                <div *ngIf="shortcutGroupsDynamic.length === 0" class="shortcuts-no-results">
                  No shortcuts found for "{{ shortcutSearch }}"
                </div>
              </section>

              <!-- About -->
              <section *ngIf="activeTab === 'about'" class="settings-section about-section">
                <h3 class="section-title">About</h3>
                <p class="section-desc">Reassembly Resource Commander overview, runtime details, and open-source notices.</p>

                <div class="settings-group">
                  <h4 class="group-title">Application</h4>
                  <div class="about-hero-banner" *ngIf="!aboutImageFailed">
                    <img
                      src="/assets/aistudio/photo_2026-09-05_17-23-23_cbdbac3a.jpg"
                      alt="Reassembly Resource Commander"
                      referrerpolicy="no-referrer"
                      (error)="aboutImageFailed = true" />
                  </div>
                  <div class="about-app-card">
                    <div class="about-app-icon" aria-hidden="true">
                      <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z"/>
                      </svg>
                    </div>
                    <div class="about-app-info">
                      <span class="about-app-name">Reassembly — Resource Commander</span>
                      <span class="about-app-version">Dual-pane file manager &amp; archive workbench</span>
                      <span class="about-app-build">Angular 22 · TypeScript · Node.js Web Runtime</span>
                    </div>
                  </div>
                  <div class="about-action-row">
                    <button type="button" class="btn btn-secondary" (click)="openFullAboutScreen()">Open About Screen</button>
                    <button type="button" class="btn btn-secondary" (click)="copyAboutSystemInfo()">
                      {{ aboutInfoCopied ? 'Copied to Clipboard' : 'Copy System Info' }}
                    </button>
                  </div>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Platform</h4>
                  <div class="about-kv-list">
                    <div class="about-kv-row">
                      <span class="about-kv-label">Workspace</span>
                      <span class="about-kv-value">Dual-Pane Commander</span>
                    </div>
                    <div class="about-kv-row">
                      <span class="about-kv-label">Frontend</span>
                      <span class="about-kv-value">Angular 22 (Zoneless)</span>
                    </div>
                    <div class="about-kv-row">
                      <span class="about-kv-label">Runtime</span>
                      <span class="about-kv-value">Node.js · Express API</span>
                    </div>
                    <div class="about-kv-row">
                      <span class="about-kv-label">Archive Engine</span>
                      <span class="about-kv-value">ZIP · TAR · Nested Inspection</span>
                    </div>
                  </div>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Open Source</h4>
                  <p class="about-oss-note">Reassembly builds on open-source libraries used under their respective licenses.</p>
                  <div class="about-oss-grid">
                    <div class="about-oss-item">
                      <span class="oss-name">Angular</span>
                      <span class="oss-license">MIT</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">CodeMirror 6</span>
                      <span class="oss-license">MIT</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">Streamix</span>
                      <span class="oss-license">MIT</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">Express</span>
                      <span class="oss-license">MIT</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">Node.js</span>
                      <span class="oss-license">MIT</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">Reassembly Core</span>
                      <span class="oss-license">Apache 2.0</span>
                    </div>
                  </div>
                </div>
              </section>
            </main>
          </div>

          <footer class="settings-footer">
            <div class="settings-status">
              <span *ngIf="hasChanges" class="unsaved-indicator">Unsaved changes</span>
              <span *ngIf="!hasChanges" class="saved-indicator">All changes saved</span>
            </div>
            <div class="settings-actions">
              <button type="button" class="btn btn-secondary" (click)="onCancel()">Cancel</button>
              <button type="button" class="btn btn-primary" [disabled]="!hasChanges || saving" (click)="onSave()">
                {{ saving ? 'Saving…' : 'Save Changes' }}<kbd class="btn-kbd">Ctrl+S</kbd>
              </button>
            </div>
          </footer>
        </div>
      </div>
    </ng-container>
  `,
})
export class SettingsDialogComponent {
  @ViewChild('dialogPanel') private dialogPanel?: ElementRef<HTMLElement>;
  @ViewChild(PluginSettingsPanelComponent) private pluginSettingsPanel?: PluginSettingsPanelComponent;

  readonly request$: Stream<SettingsDialogRequest | null> = this.settingsService.getMountedRequest$();
  themePresets: ThemeContribution[] = this.customizations.themes$.value;

  activeTab: SettingsTab = 'general';
  hasChanges = false;
  saving = false;
  draft: ProjectSettings = this.settingsService.getSettings();

  // Keybinding customization state
  shortcutSearch = '';
  recordingBindingId: string | null = null;
  recordingDescription = '';
  recordingSteps: KeyStroke[] = [];
  private recordingTimeout: number | null = null;
  private conflicts = new Map<string, string[]>();
  aboutImageFailed = false;
  aboutInfoCopied = false;

  readonly tabs: { id: SettingsTab; label: string; icon: string; divider?: boolean }[] = [
    {
      id: 'general',
      label: 'General',
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 11a3 3 0 1 1 0-6 3 3 0 0 1 0 6zm0 1a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM8 0a.5.5 0 0 1 .5.5v2a.5.5 0 0 1-1 0v-2A.5.5 0 0 1 8 0zm0 13a.5.5 0 0 1 .5.5v2a.5.5 0 0 1-1 0v-2A.5.5 0 0 1 8 13zm8-5a.5.5 0 0 1-.5.5h-2a.5.5 0 0 1 0-1h2a.5.5 0 0 1 .5.5zM3 8a.5.5 0 0 1-.5.5h-2a.5.5 0 0 1 0-1h2A.5.5 0 0 1 3 8zm10.657-5.657a.5.5 0 0 1 0 .707l-1.414 1.415a.5.5 0 1 1-.707-.708l1.414-1.414a.5.5 0 0 1 .707 0zm-9.193 9.193a.5.5 0 0 1 0 .707L3.05 13.657a.5.5 0 0 1-.707-.707l1.414-1.414a.5.5 0 0 1 .707 0zm9.193 2.121a.5.5 0 0 1-.707 0l-1.414-1.414a.5.5 0 0 1 .707-.707l1.414 1.414a.5.5 0 0 1 0 .707zM4.464 4.465a.5.5 0 0 1-.707 0L2.343 3.05a.5.5 0 1 1 .707-.707l1.414 1.414a.5.5 0 0 1 0 .708z"/></svg>',
    },
    {
      id: 'files',
      label: 'Files',
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M6 2a1 1 0 0 0-1 1v1H4a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2h-1V3a1 1 0 0 0-1-1H6zm0 1h4v1H6V3zm-2 3h8v7H4V6z"/></svg>',
    },
    {
      id: 'editor',
      label: 'Editor',
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M15.502 1.94a.5.5 0 0 1 0 .706L14.459 3.69l-2-2L13.502.646a.5.5 0 0 1 .707 0l1.293 1.293zm-1.75 2.456-2-2L4.939 9.21a.5.5 0 0 0-.121.196l-.805 2.414a.25.25 0 0 0 .316.316l2.414-.805a.5.5 0 0 0 .196-.12l6.813-6.814z"/><path fill-rule="evenodd" d="M1 13.5A1.5 1.5 0 0 0 2.5 15h11a1.5 1.5 0 0 0 1.5-1.5v-6a.5.5 0 0 0-1 0v6a.5.5 0 0 1-.5.5h-11a.5.5 0 0 1-.5-.5v-11a.5.5 0 0 1 .5-.5H9a.5.5 0 0 0 0-1H2.5A1.5 1.5 0 0 0 1 2.5v11z"/></svg>',
    },
    {
      id: 'view',
      label: 'View',
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M4 1.5a.5.5 0 0 1 .5-.5h7a.5.5 0 0 1 .5.5v2a.5.5 0 0 1-.5.5h-7a.5.5 0 0 1-.5-.5v-2zm0 5a.5.5 0 0 1 .5-.5h7a.5.5 0 0 1 .5.5v2a.5.5 0 0 1-.5.5h-7a.5.5 0 0 1-.5-.5v-2zm0 5a.5.5 0 0 1 .5-.5h7a.5.5 0 0 1 .5.5v2a.5.5 0 0 1-.5.5h-7a.5.5 0 0 1-.5-.5v-2zM1 2a1 1 0 0 1 1-1h1a1 1 0 0 1 1 1v1a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V2zm0 5a1 1 0 0 1 1-1h1a1 1 0 0 1 1 1v1a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V7zm0 5a1 1 0 0 1 1-1h1a1 1 0 0 1 1 1v1a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1v-1z"/></svg>',
    },
    {
      id: 'advanced',
      label: 'Advanced',
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 4.754a3.246 3.246 0 1 0 0 6.492 3.246 3.246 0 0 0 0-6.492zM5.754 8a2.246 2.246 0 1 1 4.492 0 2.246 2.246 0 0 1-4.492 0z"/><path d="M9.796 1.343c-.527-1.79-3.065-1.79-3.592 0l-.094.319a.873.873 0 0 1-1.255.52l-.292-.16c-1.64-.892-3.433.902-2.54 2.541l.159.292a.873.873 0 0 1-.52 1.255l-.319.094c-1.79.527-1.79 3.065 0 3.592l.319.094a.873.873 0 0 1 .52 1.255l-.16.292c-.892 1.64.901 3.434 2.541 2.54l.292-.159a.873.873 0 0 1 1.255.52l.094.319c.527 1.79 3.065 1.79 3.592 0l.094-.319a.873.873 0 0 1 1.255-.52l.292.16c1.639.893 3.432-.902 2.54-2.541l-.159-.292a.873.873 0 0 1 .52-1.255l.319-.094c1.79-.527 1.79-3.065 0-3.592l-.319-.094a.873.873 0 0 1-.52-1.255l.16-.292c.893-1.64-.902-3.433-2.541-2.54l-.292.159a.873.873 0 0 1-1.255-.52l-.094-.319z"/></svg>',
    },
    {
      id: 'plugins',
      label: 'Plugins',
      divider: true,
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M6.5 1a1.5 1.5 0 0 0-3 0v1H2a2 2 0 0 0-2 2v2.5h1.5a1.5 1.5 0 0 1 0 3H0V12a2 2 0 0 0 2 2h2.5v-1.5a1.5 1.5 0 0 1 3 0V14H10a2 2 0 0 0 2-2V9.5h1.5a1.5 1.5 0 0 0 0-3H12V4a2 2 0 0 0-2-2H6.5V1z"/></svg>',
    },
    {
      id: 'shortcuts',
      label: 'Shortcuts',
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M14 5a1 1 0 0 1 1 1v5a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h12zm-12-1a2 2 0 0 0-2 2v5a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2H2z"/><path d="M3.5 7.5a.5.5 0 0 1 .5-.5h1a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5H4a.5.5 0 0 1-.5-.5v-1zm2.5 0a.5.5 0 0 1 .5-.5h1a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5H6.5a.5.5 0 0 1-.5-.5v-1zm2.5 0a.5.5 0 0 1 .5-.5h1a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5H9a.5.5 0 0 1-.5-.5v-1zm2.5 0a.5.5 0 0 1 .5-.5h2a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5h-2a.5.5 0 0 1-.5-.5v-1z"/></svg>',
    },
    {
      id: 'about',
      label: 'About',
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 15A7 7 0 1 1 8 1a7 7 0 0 1 0 14zm0 1A8 8 0 1 0 8 0a8 8 0 0 0 0 16z"/><path d="m8.93 6.588-2.29.287-.082.38.45.083c.294.07.352.176.288.469l-.738 3.468c-.194.897.105 1.319.808 1.319.545 0 1.178-.252 1.465-.598l.088-.416c-.2.176-.492.246-.686.246-.275 0-.375-.193-.304-.533L8.93 6.588zM9 4.5a1 1 0 1 1-2 0 1 1 0 0 1 2 0z"/></svg>',
    },
  ];

  private originalSettings: ProjectSettings = this.settingsService.getSettings();
  private pluginHasChanges = false;
  private isOpen = false;
  private releaseDialogContext: (() => void) | null = null;
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;
  private readonly destroy$ = createSubject<void>();
  private readonly captureEscapeListener = (event: KeyboardEvent) => {
    if (!this.isOpen || event.key !== 'Escape') {
      return;
    }
    if (this.modalStack && !this.modalStack.isTopmost('settings-dialog')) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation?.();
    this.ngZone.run(() => {
      this.onEscapeKey();
    });
  };

  constructor(
    private readonly settingsService: SettingsService,
    private readonly inputContexts: InputContextService,
    private readonly inputEngine: InputEngineService,
    private readonly keybindingStorage: KeybindingStorageService,
    private readonly overlays: ApplicationOverlayHostService,
    private readonly customizations: CustomizationRegistryService,
    private readonly ngZone: NgZone,
  ) {}

  ngOnInit(): void {
    window.addEventListener('keydown', this.captureEscapeListener, { capture: true });
    this.inputEngine.registerContext({
      id: SETTINGS_DIALOG_CONTEXT,
      kind: 'overlay',
      priority: 420,
      exclusive: true,
      parentId: 'global',
      description: 'Project settings dialog',
    });
    this.inputEngine.registerCommand('dialog.settings.save', () => {
      if (this.hasChanges) {
        void this.onSave();
      }
    }, SETTINGS_DIALOG_OWNER);
    this.inputEngine.registerCommand('dialog.settings.close', () => this.onEscapeKey(), SETTINGS_DIALOG_OWNER);
    this.inputEngine.registerCommand('dialog.settings.focusNext', () => this.moveDialogFocus(1), SETTINGS_DIALOG_OWNER);
    this.inputEngine.registerCommand('dialog.settings.focusPrev', () => this.moveDialogFocus(-1), SETTINGS_DIALOG_OWNER);
    for (const binding of SETTINGS_DIALOG_BINDINGS) {
      this.inputEngine.registerBinding(binding, SETTINGS_DIALOG_OWNER);
    }

    this.customizations.themes$
      .pipe(takeUntil(this.destroy$))
      .subscribe((presets) => {
        this.themePresets = presets;
      });

    this.settingsService
      .getActiveRequest$()
      .pipe(takeUntil(this.destroy$))
      .subscribe((request) => {
        if (request) {
          if (this.modalStack && this.hostElement && !this.unregisterModal) {
            this.unregisterModal = this.modalStack.register({
              id: 'settings-dialog',
              contextId: SETTINGS_DIALOG_CONTEXT,
              element: this.hostElement.nativeElement,
              dismiss: () => this.onEscapeKey(),
              dismissible: () => this.isOpen && !this.isClosing,
            });
          }
        } else {
          this.unregisterModal?.();
          this.unregisterModal = null;
        }
      });

    this.settingsService
      .getMountedRequest$()
      .pipe(takeUntil(this.destroy$))
      .subscribe((request) => {
        if (request) {
          this.originalSettings = this.settingsService.getSettings();
          this.draft = JSON.parse(JSON.stringify(this.originalSettings));
          this.activeTab = request.initialTab ?? 'general';
          this.pluginHasChanges = false;
          this.hasChanges = false;
          this.saving = false;
          this.isOpen = true;
          this.releaseDialogContext ??= this.inputContexts.acquire(SETTINGS_DIALOG_CONTEXT);
        } else {
          this.isOpen = false;
          this.releaseDialogContext?.();
          this.releaseDialogContext = null;
        }
      });
  }

  ngOnDestroy(): void {
    window.removeEventListener('keydown', this.captureEscapeListener, { capture: true });
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    for (const binding of SETTINGS_DIALOG_BINDINGS) {
      this.inputEngine.unregisterBinding(binding.id, SETTINGS_DIALOG_OWNER);
    }
    this.inputEngine.unregisterCommand('dialog.settings.save', SETTINGS_DIALOG_OWNER);
    this.inputEngine.unregisterCommand('dialog.settings.close', SETTINGS_DIALOG_OWNER);
    this.inputEngine.unregisterCommand('dialog.settings.focusNext', SETTINGS_DIALOG_OWNER);
    this.inputEngine.unregisterCommand('dialog.settings.focusPrev', SETTINGS_DIALOG_OWNER);
    this.destroy$.next();
    this.destroy$.complete();
  }

  setActiveTab(tab: SettingsTab): void {
    this.activeTab = tab;
  }

  getTabLabel(): string {
    return this.tabs.find((t) => t.id === this.activeTab)?.label ?? '';
  }

  formatStep(step: KeyStroke): string {
    const mods = step.modifiers ?? [];
    const parts: string[] = [];
    if (mods.includes('ctrl'))  parts.push('Ctrl');
    if (mods.includes('shift')) parts.push('Shift');
    if (mods.includes('alt'))   parts.push('Alt');
    if (mods.includes('meta'))  parts.push('Meta');
    const keyLabels: Record<string, string> = {
      arrowup: '↑', arrowdown: '↓', arrowleft: '←', arrowright: '→',
      enter: 'Enter', escape: 'Esc', backspace: 'Backspace', delete: 'Del',
      tab: 'Tab', space: 'Space', pageup: 'PgUp', pagedown: 'PgDn',
      home: 'Home', end: 'End', insert: 'Ins',
      f1: 'F1', f2: 'F2', f3: 'F3', f4: 'F4', f5: 'F5', f6: 'F6',
      f7: 'F7', f8: 'F8', f9: 'F9', f10: 'F10', f11: 'F11', f12: 'F12',
    };
    const key = step.key.toLowerCase();
    parts.push(keyLabels[key] ?? step.key.toUpperCase());
    return parts.join('+');
  }

  private formatMouseGesture(binding: InputBindingDefinition): string[][] {
    if (binding.trigger.device !== 'mouse') return [['?']];
    const g = binding.trigger.gesture;
    const mods = (g.modifiers ?? []).map(m => {
      switch (m) { case 'ctrl': return 'Ctrl'; case 'shift': return 'Shift'; case 'alt': return 'Alt'; case 'meta': return 'Meta'; }
      return m;
    });
    let action = '';
    switch (g.type) {
      case 'click': action = 'Click'; break;
      case 'doubleClick': action = 'Double-click'; break;
      case 'mousedown': action = 'Mouse down'; break;
      case 'mouseup': action = 'Mouse up'; break;
      case 'wheel': action = g.direction === 'up' ? 'Wheel up' : g.direction === 'down' ? 'Wheel down' : `Wheel ${g.direction}`; break;
      case 'drag': action = 'Drag'; break;
      default: action = g.type;
    }
    if (g.button && g.button !== 'left') {
      action += ` (${g.button})`;
    }
    const label = [...mods, action].join('+');
    return [[label]];
  }

  get shortcutGroupsDynamic(): Array<{
    category: string;
    items: Array<{ id: string; description: string; chords: string[][]; enabled: boolean }>;
  }> {
    const snapshot = this.keybindingStorage.mergeWithDefaults(this.inputEngine.getBindingSnapshot());
    const catMap = new Map<string, Map<string, { id: string; description: string; chords: string[][]; enabled: boolean }>>();

    for (const b of snapshot) {
      if (b.trigger.device !== 'keyboard' && b.trigger.device !== 'mouse') continue;
      const cat = b.category ?? 'General';
      if (!catMap.has(cat)) catMap.set(cat, new Map());
      const group = catMap.get(cat)!;

      const chords = b.trigger.device === 'keyboard'
        ? b.trigger.chord.steps.map((s) => {
            const label = this.formatStep(s);
            return [label];
          })
        : this.formatMouseGesture(b);

      const existing = group.get(b.commandId);
      if (existing) {
        // merge alternative chord as separate display entry only if description differs
        if (existing.description !== b.description) {
          group.set(`${b.commandId}:${b.id}`, { id: b.id, description: b.description, chords, enabled: b.enabled !== false });
        }
      } else {
        group.set(b.commandId, { id: b.id, description: b.description, chords, enabled: b.enabled !== false });
      }
    }

    let entries = Array.from(catMap.entries());
    if (this.shortcutSearch.trim()) {
      const search = this.shortcutSearch.toLowerCase().trim();
      entries = entries.map(([cat, groupMap]) => {
        const filtered = new Map(Array.from(groupMap.entries()).filter(([, item]) => {
          return item.description.toLowerCase().includes(search)
            || item.chords.some(chord => chord.some(k => k.toLowerCase().includes(search)));
        }));
        return [cat, filtered] as [string, Map<string, { id: string; description: string; chords: string[][]; enabled: boolean }>];
      }).filter(([, groupMap]) => groupMap.size > 0);
    }

    const categoryOrder = ['Navigation', 'Tab Management', 'File Operations', 'Search & Filter', 'Workspaces', 'Pane Control', 'Tools'];
    const result: Array<{ category: string; items: Array<{ id: string; description: string; chords: string[][]; enabled: boolean }> }> = [];
    for (const cat of categoryOrder) {
      const groupMap = entries.find(([c]) => c === cat)?.[1];
      if (groupMap && groupMap.size > 0) {
        result.push({ category: cat, items: [...groupMap.values()] });
      }
    }
    for (const [cat, groupMap] of entries) {
      if (!categoryOrder.includes(cat) && groupMap.size > 0) {
        result.push({ category: cat, items: [...groupMap.values()] });
      }
    }
    return result;
  }

  onShortcutSearchChange(): void {
    // Triggered by ngModelChange - getter recomputes automatically
  }

  startRecording(bindingId: string): void {
    if (this.recordingBindingId === bindingId) {
      this.cancelRecording();
      return;
    }
    this.recordingBindingId = bindingId;
    this.recordingSteps = [];
    const binding = this.inputEngine.getBinding(bindingId);
    this.recordingDescription = binding?.description ?? bindingId;
  }

  cancelRecording(): void {
    this.recordingBindingId = null;
    this.recordingDescription = '';
    this.recordingSteps = [];
    if (this.recordingTimeout !== null) {
      window.clearTimeout(this.recordingTimeout);
      this.recordingTimeout = null;
    }
  }

  finishRecording(): void {
    if (this.recordingSteps.length === 0) {
      this.cancelRecording();
      return;
    }
    this.commitRecordingSteps(this.recordingSteps);
  }

  private onRecordingStep(stroke: KeyStroke): void {
    const bindingId = this.recordingBindingId;
    if (!bindingId) return;

    this.recordingSteps.push(stroke);

    // Clear existing timeout
    if (this.recordingTimeout !== null) {
      window.clearTimeout(this.recordingTimeout);
      this.recordingTimeout = null;
    }

    // Start a timeout: if user presses another key within CHORD_STEP_TIMEOUT_MS, it becomes next step
    const CHORD_STEP_TIMEOUT_MS = 1500;
    this.recordingTimeout = window.setTimeout(() => {
      this.finishRecording();
    }, CHORD_STEP_TIMEOUT_MS);
  }

  private commitRecordingSteps(steps: KeyStroke[]): void {
    const bindingId = this.recordingBindingId;
    if (!bindingId || steps.length === 0) return;

    const existing = this.inputEngine.getBinding(bindingId);
    if (!existing) {
      this.cancelRecording();
      return;
    }

    const newTrigger: InputTrigger = {
      device: 'keyboard',
      chord: { steps: [...steps] },
    };

    // Check conflicts: exact same chord sequence
    this.conflicts.clear();
    const snapshot = this.inputEngine.getBindingSnapshot();
    const conflicts: string[] = [];
    for (const b of snapshot) {
      if (b.id === bindingId) continue;
      if (b.trigger.device !== 'keyboard') continue;
      if (b.enabled === false) continue;
      if (this.sameChord(b.trigger.chord.steps, steps)) {
        conflicts.push(b.description || b.commandId);
      }
    }
    if (conflicts.length > 0) {
      this.conflicts.set(bindingId, conflicts);
    }

    this.inputEngine.updateBindingTrigger(bindingId, newTrigger);
    this.keybindingStorage.setOverride({ id: bindingId, trigger: newTrigger });
    this.cancelRecording();
  }

  private sameChord(a: KeyStroke[], b: KeyStroke[]): boolean {
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (a[i].key !== b[i].key || !this.sameModifiers(a[i].modifiers, b[i].modifiers)) {
        return false;
      }
    }
    return true;
  }

  private sameModifiers(a?: Array<'ctrl'|'shift'|'alt'|'meta'>, b?: Array<'ctrl'|'shift'|'alt'|'meta'>): boolean {
    const ma = (a ?? []).sort().join('+');
    const mb = (b ?? []).sort().join('+');
    return ma === mb;
  }

  hasConflict(bindingId: string): boolean {
    return this.conflicts.has(bindingId);
  }

  getConflictMessage(bindingId: string): string {
    const list = this.conflicts.get(bindingId);
    if (!list || list.length === 0) return '';
    return `Conflicts with: ${list.join(', ')}`;
  }

  onShortcutsKeydown(event: KeyboardEvent): void {
    if (!this.recordingBindingId) return;

    // Don't capture when typing in inputs
    const target = event.target as HTMLElement | null;
    if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();

    if (event.key === 'Escape') {
      this.cancelRecording();
      return;
    }

    if (event.key === 'Enter' && this.recordingSteps.length > 0) {
      this.finishRecording();
      return;
    }

    // Ignore pure modifier keys
    if (['Control', 'Shift', 'Alt', 'Meta'].includes(event.key)) return;

    const modifiers: KeyStroke['modifiers'] = [];
    if (event.ctrlKey || event.metaKey) modifiers.push(event.ctrlKey ? 'ctrl' : 'meta');
    if (event.shiftKey) modifiers.push('shift');
    if (event.altKey) modifiers.push('alt');

    const key = this.normalizeRecordingKey(event.key, event.code);
    if (!key) return;

    this.onRecordingStep({ key, modifiers: modifiers.length > 0 ? modifiers.sort() : undefined });
  }

  private normalizeRecordingKey(key: string, code?: string): string | null {
    const codeMap: Record<string, string> = {
      NumpadAdd: 'numpadadd',
      NumpadSubtract: 'numpadsubtract',
      NumpadMultiply: 'numpadmultiply',
    };
    if (code && codeMap[code]) return codeMap[code];

    const keyMap: Record<string, string> = {
      ' ': 'space',
      Escape: 'escape',
      Enter: 'enter',
      Tab: 'tab',
      Insert: 'insert',
      Backspace: 'backspace',
      Delete: 'delete',
      Home: 'home',
      End: 'end',
      PageUp: 'pageup',
      PageDown: 'pagedown',
      ArrowUp: 'arrowup',
      ArrowDown: 'arrowdown',
      ArrowLeft: 'arrowleft',
      ArrowRight: 'arrowright',
    };
    return keyMap[key] ?? key.toLowerCase();
  }

  toggleBindingEnabled(bindingId: string): void {
    const binding = this.inputEngine.getBinding(bindingId);
    if (!binding) return;
    const next = binding.enabled === false;
    this.inputEngine.setBindingEnabled(bindingId, next);
    this.keybindingStorage.setOverride({ id: bindingId, enabled: next });
  }

  resetKeybinding(bindingId: string): void {
    const def = this.keybindingStorage.getDefaultBinding(bindingId);
    if (def) {
      this.inputEngine.updateBindingTrigger(bindingId, def.trigger);
      this.inputEngine.setBindingEnabled(bindingId, true);
    }
    this.keybindingStorage.removeOverride(bindingId);
    this.conflicts.delete(bindingId);
  }

  resetAllKeybindings(): void {
    for (const override of this.keybindingStorage.getOverrides()) {
      const def = this.keybindingStorage.getDefaultBinding(override.id);
      if (def) {
        this.inputEngine.updateBindingTrigger(override.id, def.trigger);
        this.inputEngine.setBindingEnabled(override.id, true);
      }
    }
    this.keybindingStorage.resetAll();
    this.conflicts.clear();
  }

  onSettingChanged(): void {
    this.recomputeHasChanges();
  }

  onPluginDirtyChange(dirty: boolean): void {
    this.pluginHasChanges = dirty;
    this.recomputeHasChanges();
  }

  onEscapeKey(event?: Event): void {
    event?.preventDefault();
    event?.stopPropagation();
    if (this.recordingBindingId) {
      this.cancelRecording();
      return;
    }
    this.onCancel();
  }

  onCancel(): void {
    if (this.saving) return;

    this.isOpen = false;
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.pluginSettingsPanel?.discard();
    this.pluginHasChanges = false;
    this.recomputeHasChanges();
    this.settingsService.closeSettings(false);
  }

  async onSave(): Promise<void> {
    if (!this.hasChanges || this.saving) return;

    this.saving = true;
    try {
      if (this.pluginHasChanges) {
        const saved = await this.pluginSettingsPanel?.save();
        if (saved === false) {
          return;
        }
        this.pluginHasChanges = false;
      }

      const changes = this.getChanges();
      if (Object.keys(changes).length > 0) {
        this.settingsService.updateSettings(changes);
      }

      this.originalSettings = JSON.parse(JSON.stringify(this.draft));
      this.recomputeHasChanges();
      this.isOpen = false;
      this.unregisterModal?.();
      this.unregisterModal = null;
      this.settingsService.closeSettings(true);
    } finally {
      this.saving = false;
    }
  }

  onResetSection(): void {
    const settingsKeys: Array<keyof ProjectSettings> = ['general', 'files', 'editor', 'view', 'advanced'];
    const key = settingsKeys.find((k) => k === this.activeTab);
    if (key) {
      this.settingsService.resetSection(key);
    }
    this.draft = JSON.parse(JSON.stringify(this.settingsService.getSettings()));
    this.onSettingChanged();
  }

  onResetAll(): void {
    this.settingsService.resetToDefaults();
    this.draft = JSON.parse(JSON.stringify(this.settingsService.getSettings()));
    this.onSettingChanged();
  }

  private recomputeHasChanges(): void {
    const projectHasChanges = !this.isEqual(this.draft, this.originalSettings);
    this.hasChanges = projectHasChanges || this.pluginHasChanges;
  }

  private getChanges(): Partial<ProjectSettings> {
    const changes: Partial<ProjectSettings> = {};
    
    if (!this.isEqual(this.draft.general, this.originalSettings.general)) {
      changes.general = this.draft.general;
    }
    if (!this.isEqual(this.draft.files, this.originalSettings.files)) {
      changes.files = this.draft.files;
    }
    if (!this.isEqual(this.draft.editor, this.originalSettings.editor)) {
      changes.editor = this.draft.editor;
    }
    if (!this.isEqual(this.draft.view, this.originalSettings.view)) {
      changes.view = this.draft.view;
    }
    if (!this.isEqual(this.draft.advanced, this.originalSettings.advanced)) {
      changes.advanced = this.draft.advanced;
    }
    
    return changes;
  }

  private isEqual(a: unknown, b: unknown): boolean {
    return JSON.stringify(a) === JSON.stringify(b);
  }

  private moveDialogFocus(delta: number): void {
    if (!this.dialogPanel?.nativeElement) {
      return;
    }
    moveFocusWithin(this.dialogPanel.nativeElement, delta);
  }

  get isClosing(): boolean {
    return !!this.settingsService.getClosingRequest();
  }

  openFullAboutScreen(): void {
    this.onCancel();
    queueMicrotask(() => this.overlays.openAbout());
  }

  async copyAboutSystemInfo(): Promise<void> {
    const text = [
      'Application: Reassembly — Resource Commander',
      'Frontend: Angular 22 (Zoneless Dual-Pane)',
      'Runtime: Node.js Web Workspace',
      `Host: ${window.location.host}`,
    ].join('\n');
    try {
      await writeClipboardText(text);
      this.aboutInfoCopied = true;
      setTimeout(() => {
        this.aboutInfoCopied = false;
      }, 2000);
    } catch {
      // Ignore clipboard failure in restricted contexts
    }
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.settingsService.finishCloseSettings();
  }
}