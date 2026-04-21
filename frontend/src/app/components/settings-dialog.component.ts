import { CommonModule } from '@angular/common';
import { Component, ElementRef, ViewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil, type Stream } from '@epikodelabs/streamix';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';
import { moveFocusWithin } from '../directives/focus-trap.utils';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { InputHistoryService } from '../services/input-history.service';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import {
  SettingsService,
  SettingsTab,
  type ProjectSettings,
} from '../services/settings.service';

interface SettingsDialogRequest {
  resolve: (value: boolean) => void;
  initialTab?: SettingsTab;
}

@Component({
  selector: 'app-settings-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective],
  styleUrls: ['./settings-dialog.component.scss'],
  template: `
    <ng-container *ngIf="request$ | async">
      <div class="settings-overlay" (click)="onCancel()">
        <div
          #dialogPanel
          class="settings-panel"
          [appTrapFocus]="true"
          role="dialog"
          aria-modal="true"
          aria-label="Project Settings"
          tabindex="-1"
          (click)="$event.stopPropagation()"
        >
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
                type="button"
                class="btn btn-secondary btn-small"
                (click)="onResetSection()"
                [disabled]="hasChanges"
                title="Reset current section to defaults"
              >
                Reset Section
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-small"
                (click)="onResetAll()"
                [disabled]="hasChanges"
                title="Reset all settings to defaults"
              >
                Reset All
              </button>
              <button type="button" class="dialog-close-btn" aria-label="Close" (click)="onCancel()" title="Close">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                  <path d="M8 7.293l3.536-3.536.707.707L8.707 8l3.536 3.536-.707.707L8 8.707l-3.536 3.536-.707-.707L7.293 8 3.757 4.464l.707-.707L8 7.293z"/>
                </svg>
              </button>
            </div>
          </header>

          <div class="settings-body">
            <aside class="settings-sidebar">
              <nav class="settings-nav">
                <button
                  *ngFor="let tab of tabs"
                  type="button"
                  class="nav-item"
                  [class.active]="activeTab === tab.id"
                  [attr.aria-selected]="activeTab === tab.id"
                  (click)="setActiveTab(tab.id)"
                >
                  <span class="nav-icon" [innerHTML]="tab.icon"></span>
                  <span class="nav-label">{{ tab.label }}</span>
                </button>
              </nav>
            </aside>

            <main class="settings-content">
              <!-- General Settings -->
              <section *ngIf="activeTab === 'general'" class="settings-section">
                <h3 class="section-title">General</h3>
                
                <div class="settings-group">
                  <h4 class="group-title">Appearance</h4>
                  <label class="field">
                    <span>Theme</span>
                    <select [(ngModel)]="draft.general.theme" (ngModelChange)="onSettingChanged()">
                      <option value="dark">Dark</option>
                      <option value="light">Light</option>
                      <option value="system">System Preference</option>
                    </select>
                  </label>
                  <label class="field">
                    <span>Language</span>
                    <select [(ngModel)]="draft.general.language" (ngModelChange)="onSettingChanged()">
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
                  </label>
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
                      [(ngModel)]="draft.general.startupFolder"
                      (ngModelChange)="onSettingChanged()"
                      placeholder="/path/to/folder"
                      list="sfd-history-startup-folder"
                      autocomplete="off"
                    />
                    <datalist id="sfd-history-startup-folder">
                      <option *ngFor="let v of history('settings.startupFolder')" [value]="v"></option>
                    </datalist>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Updates & Privacy</h4>
                  <label class="field">
                    <span>Update Channel</span>
                    <select [(ngModel)]="draft.general.updateChannel" (ngModelChange)="onSettingChanged()">
                      <option value="stable">Stable</option>
                      <option value="beta">Beta</option>
                      <option value="dev">Development</option>
                    </select>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.general.analyticsEnabled" (ngModelChange)="onSettingChanged()" />
                    <span>Enable anonymous analytics</span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.general.crashReportsEnabled" (ngModelChange)="onSettingChanged()" />
                    <span>Send crash reports automatically</span>
                  </label>
                </div>
              </section>

              <!-- File Settings -->
              <section *ngIf="activeTab === 'files'" class="settings-section">
                <h3 class="section-title">Files</h3>

                <div class="settings-group">
                  <h4 class="group-title">Display</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.showHiddenFiles" (ngModelChange)="onSettingChanged()" />
                    <span>Show hidden files</span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.showFileExtensions" (ngModelChange)="onSettingChanged()" />
                    <span>Show file extensions</span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.calculateFolderSizes" (ngModelChange)="onSettingChanged()" />
                    <span>Calculate folder sizes (may impact performance)</span>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">File Operations</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.confirmBeforeDelete" (ngModelChange)="onSettingChanged()" />
                    <span>Confirm before deleting files</span>
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
                  <label class="field">
                    <span>Default Open Action</span>
                    <select [(ngModel)]="draft.files.defaultOpenAction" (ngModelChange)="onSettingChanged()">
                      <option value="preview">Preview</option>
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
                    <span>Auto-refresh Interval (ms)</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.files.autoRefreshInterval"
                      (ngModelChange)="onSettingChanged()"
                      min="1000"
                      max="60000"
                      step="1000"
                    />
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.cacheThumbnailPreviews" (ngModelChange)="onSettingChanged()" />
                    <span>Cache thumbnail previews</span>
                  </label>
                  <label class="field">
                    <span>Thumbnail Cache Size (MB)</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.files.thumbnailCacheSize"
                      (ngModelChange)="onSettingChanged()"
                      min="10"
                      max="1000"
                      step="10"
                    />
                  </label>
                </div>
              </section>

              <!-- Editor Settings -->
              <section *ngIf="activeTab === 'editor'" class="settings-section">
                <h3 class="section-title">Editor</h3>

                <div class="settings-group">
                  <h4 class="group-title">Font</h4>
                  <label class="field">
                    <span>Font Family</span>
                    <input
                      type="text"
                      [(ngModel)]="draft.editor.fontFamily"
                      (ngModelChange)="onSettingChanged()"
                      placeholder="Consolas, 'Courier New', monospace"
                      list="sfd-history-font-family"
                      autocomplete="off"
                    />
                    <datalist id="sfd-history-font-family">
                      <option *ngFor="let v of history('settings.fontFamily')" [value]="v"></option>
                    </datalist>
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
                    <span>Word Wrap</span>
                    <select [(ngModel)]="draft.editor.wordWrap" (ngModelChange)="onSettingChanged()">
                      <option value="off">Off</option>
                      <option value="on">On</option>
                      <option value="wordWrapColumn">At Column</option>
                      <option value="bounded">Bounded</option>
                    </select>
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
                    <span>Auto Save</span>
                    <select [(ngModel)]="draft.editor.autoSave" (ngModelChange)="onSettingChanged()">
                      <option value="off">Off</option>
                      <option value="afterDelay">After Delay</option>
                      <option value="onFocusChange">On Focus Change</option>
                      <option value="onWindowChange">On Window Change</option>
                    </select>
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
              </section>

              <!-- View Settings -->
              <section *ngIf="activeTab === 'view'" class="settings-section">
                <h3 class="section-title">View</h3>

                <div class="settings-group">
                  <h4 class="group-title">Layout</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.compactMode" (ngModelChange)="onSettingChanged()" />
                    <span>Compact mode (reduced padding and font size)</span>
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
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.sidebarVisible" (ngModelChange)="onSettingChanged()" />
                    <span>Show sidebar by default</span>
                  </label>
                  <label class="field" *ngIf="draft.view.sidebarVisible">
                    <span>Sidebar Width (px)</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.view.sidebarWidth"
                      (ngModelChange)="onSettingChanged()"
                      min="150"
                      max="500"
                    />
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.view.previewPaneVisible" (ngModelChange)="onSettingChanged()" />
                    <span>Show preview pane by default</span>
                  </label>
                  <label class="field" *ngIf="draft.view.previewPaneVisible">
                    <span>Preview Pane Width (px)</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.view.previewPaneWidth"
                      (ngModelChange)="onSettingChanged()"
                      min="150"
                      max="800"
                    />
                  </label>
                </div>
              </section>

              <!-- Advanced Settings -->
              <section *ngIf="activeTab === 'advanced'" class="settings-section">
                <h3 class="section-title">Advanced</h3>

                <div class="settings-group">
                  <h4 class="group-title">Performance</h4>
                  <label class="field">
                    <span>GPU Acceleration</span>
                    <select [(ngModel)]="draft.advanced.gpuAcceleration" (ngModelChange)="onSettingChanged()">
                      <option value="auto">Automatic</option>
                      <option value="on">Enabled</option>
                      <option value="off">Disabled</option>
                    </select>
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
                          [(ngModel)]="draft.advanced.proxyHost"
                          (ngModelChange)="onSettingChanged()"
                          placeholder="proxy.example.com"
                          list="sfd-history-proxy-host"
                          autocomplete="off"
                        />
                        <datalist id="sfd-history-proxy-host">
                          <option *ngFor="let v of history('settings.proxyHost')" [value]="v"></option>
                        </datalist>
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
                          [(ngModel)]="draft.advanced.proxyUsername"
                          (ngModelChange)="onSettingChanged()"
                          list="sfd-history-proxy-user"
                          autocomplete="off"
                        />
                        <datalist id="sfd-history-proxy-user">
                          <option *ngFor="let v of history('settings.proxyUsername')" [value]="v"></option>
                        </datalist>
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
              <button type="button" class="btn btn-primary" [disabled]="!hasChanges" (click)="onSave()">
                Save Changes<kbd class="btn-kbd">Ctrl+S</kbd>
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

  readonly request$: Stream<SettingsDialogRequest | null> = this.settingsService.getActiveRequest$();

  activeTab: SettingsTab = 'general';
  hasChanges = false;
  draft: ProjectSettings = this.settingsService.getSettings();

  readonly tabs: { id: SettingsTab; label: string; icon: string }[] = [
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
  ];

  private originalSettings: ProjectSettings = this.settingsService.getSettings();
  private releaseDialogContext: (() => void) | null = null;
  private readonly destroy$ = createSubject<void>();

  constructor(
    private readonly settingsService: SettingsService,
    private readonly inputContexts: InputContextService,
    private readonly inputEngine: InputEngineService,
    private readonly overlayFreeze: NativeOverlayFreezeService,
    private readonly inputHistory: InputHistoryService,
  ) {}

  ngOnInit(): void {
    this.inputEngine.registerCommand('dialog.settings.save', () => {
      if (this.hasChanges) {
        void this.onSave();
      }
    });
    this.inputEngine.registerCommand('dialog.settings.close', () => this.onCancel());
    this.inputEngine.registerCommand('dialog.settings.focusNext', () => this.moveDialogFocus(1));
    this.inputEngine.registerCommand('dialog.settings.focusPrev', () => this.moveDialogFocus(-1));

    this.settingsService
      .getActiveRequest$()
      .pipe(takeUntil(this.destroy$))
      .subscribe((request) => {
        if (request) {
          this.originalSettings = this.settingsService.getSettings();
          this.draft = JSON.parse(JSON.stringify(this.originalSettings));
          this.activeTab = request.initialTab ?? 'general';
          this.hasChanges = false;
          this.releaseDialogContext ??= this.inputContexts.acquire('dialog.settings');
          this.overlayFreeze.acquire('dialog.settings');
        } else {
          this.releaseDialogContext?.();
          this.releaseDialogContext = null;
          this.overlayFreeze.release('dialog.settings');
        }
      });
  }

  ngOnDestroy(): void {
    this.overlayFreeze.release('dialog.settings');
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.inputEngine.unregisterCommand('dialog.settings.save');
    this.inputEngine.unregisterCommand('dialog.settings.close');
    this.inputEngine.unregisterCommand('dialog.settings.focusNext');
    this.inputEngine.unregisterCommand('dialog.settings.focusPrev');
    this.destroy$.next();
    this.destroy$.complete();
  }

  setActiveTab(tab: SettingsTab): void {
    this.activeTab = tab;
  }

  getTabLabel(): string {
    return this.tabs.find((t) => t.id === this.activeTab)?.label ?? '';
  }

  /** Returns stored autocomplete suggestions for a given history key. */
  history(key: string): string[] {
    return this.inputHistory.get(key);
  }

  onSettingChanged(): void {
    this.hasChanges = !this.isEqual(this.draft, this.originalSettings);
  }

  onCancel(): void {
    if (this.hasChanges) {
      // Optionally show a confirmation dialog here
    }
    this.settingsService.closeSettings(false);
  }

  onSave(): void {
    this.settingsService.updateSettings(this.getChanges());
    this.saveSettingsHistory();
    this.originalSettings = JSON.parse(JSON.stringify(this.draft));
    this.hasChanges = false;
    this.settingsService.closeSettings(true);
  }

  private saveSettingsHistory(): void {
    if (this.draft.general.startupFolder?.trim()) {
      this.inputHistory.push('settings.startupFolder', this.draft.general.startupFolder.trim());
    }
    if (this.draft.editor.fontFamily?.trim()) {
      this.inputHistory.push('settings.fontFamily', this.draft.editor.fontFamily.trim());
    }
    if (this.draft.advanced.proxyHost?.trim()) {
      this.inputHistory.push('settings.proxyHost', this.draft.advanced.proxyHost.trim());
    }
    if (this.draft.advanced.proxyUsername?.trim()) {
      this.inputHistory.push('settings.proxyUsername', this.draft.advanced.proxyUsername.trim());
    }
  }

  onResetSection(): void {
    this.settingsService.resetSection(this.activeTab);
    this.draft = JSON.parse(JSON.stringify(this.settingsService.getSettings()));
    this.onSettingChanged();
  }

  onResetAll(): void {
    this.settingsService.resetToDefaults();
    this.draft = JSON.parse(JSON.stringify(this.settingsService.getSettings()));
    this.onSettingChanged();
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
}