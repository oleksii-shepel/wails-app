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
import { BROWSER_BINDINGS } from './file-browser-command-registry';
import type { InputBindingDefinition, KeyStroke } from '../services/input-system.types';

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
                title="Reset current section to defaults"
              >
                Reset Section
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-small"
                (click)="onResetAll()"
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
                      <span>Theme</span>
                      <select [(ngModel)]="draft.general.theme" (ngModelChange)="onSettingChanged()">
                        <option value="dark">Dark</option>
                        <option value="light">Light</option>
                        <option value="system">System Preference</option>
                      </select>
                      <small>Controls the color scheme. "System Preference" follows your OS setting.</small>
                    </label>
                    <label class="field">
                      <span>UI Zoom</span>
                      <select [(ngModel)]="draft.general.uiZoom" (ngModelChange)="onSettingChanged()">
                        <option [ngValue]="75">75%</option>
                        <option [ngValue]="90">90%</option>
                        <option [ngValue]="100">100% (Default)</option>
                        <option [ngValue]="110">110%</option>
                        <option [ngValue]="125">125%</option>
                        <option [ngValue]="150">150%</option>
                      </select>
                      <small>Scales all UI elements. Requires a restart to fully apply.</small>
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
                        placeholder="#0078d4"
                        class="color-text"
                      />
                      <button
                        type="button"
                        class="btn btn-secondary btn-small"
                        (click)="draft.general.accentColor = ''; onSettingChanged()"
                        title="Reset to default accent color"
                      >Reset</button>
                    </div>
                    <small>Overrides the default blue highlight color for active items, selections, and focus rings. Leave empty to use the default.</small>
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
                    <small>Sets the interface display language. Requires a restart to fully apply.</small>
                  </label>
                  <div class="field-row">
                    <label class="field">
                      <span>Date Format</span>
                      <select [(ngModel)]="draft.general.dateFormat" (ngModelChange)="onSettingChanged()">
                        <option value="relative">Relative (2 hours ago)</option>
                        <option value="short">Short (Apr 21, 2026)</option>
                        <option value="long">Long (April 21, 2026 10:30 AM)</option>
                        <option value="iso">ISO 8601 (2026-04-21T10:30)</option>
                      </select>
                      <small>How timestamps appear in file lists and the details pane.</small>
                    </label>
                    <label class="field">
                      <span>Size Unit</span>
                      <select [(ngModel)]="draft.general.sizeUnit" (ngModelChange)="onSettingChanged()">
                        <option value="auto">Automatic</option>
                        <option value="bytes">Bytes</option>
                        <option value="kib">KiB / KB</option>
                        <option value="mib">MiB / MB</option>
                        <option value="gib">GiB / GB</option>
                      </select>
                      <small>Unit used when displaying file sizes across all panes.</small>
                    </label>
                  </div>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Window Behavior</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.general.confirmOnClose" (ngModelChange)="onSettingChanged()" />
                    <span>Confirm before closing<small>Shows a confirmation dialog before the application window closes.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.general.reopenLastWorkspace" (ngModelChange)="onSettingChanged()" />
                    <span>Reopen last workspace on launch<small>Automatically activates the most recently used workspace when the app starts.</small></span>
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
                    <small>Stable is recommended for most users. Beta and Dev channels receive updates earlier but may be less stable.</small>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.general.checkUpdatesOnStartup" (ngModelChange)="onSettingChanged()" />
                    <span>Check for updates on startup<small>Automatically checks for a newer version each time the application launches.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.general.analyticsEnabled" (ngModelChange)="onSettingChanged()" />
                    <span>Enable anonymous analytics<small>Sends non-identifiable usage data to help improve the product.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.general.crashReportsEnabled" (ngModelChange)="onSettingChanged()" />
                    <span>Send crash reports automatically<small>Uploads anonymized crash diagnostics to help identify and fix bugs.</small></span>
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
                    <input type="checkbox" [(ngModel)]="draft.files.showHiddenFiles" (ngModelChange)="onSettingChanged()" />
                    <span>Show hidden files<small>Displays files and folders whose names start with a dot or have the hidden attribute set.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.showFileExtensions" (ngModelChange)="onSettingChanged()" />
                    <span>Show file extensions<small>Appends the extension (.txt, .jpg …) to every filename in the list.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.sortFoldersFirst" (ngModelChange)="onSettingChanged()" />
                    <span>Sort folders before files<small>Keeps directories grouped at the top regardless of the active sort column.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.singleClickToOpen" (ngModelChange)="onSettingChanged()" />
                    <span>Single-click to open<small>Opens files and navigates into folders with a single click instead of a double-click.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.calculateFolderSizes" (ngModelChange)="onSettingChanged()" />
                    <span>Calculate folder sizes<small>Recursively sums directory contents to show sizes. May impact performance on large trees.</small></span>
                  </label>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">File Operations</h4>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.confirmBeforeDelete" (ngModelChange)="onSettingChanged()" />
                    <span>Confirm before deleting files<small>Shows a confirmation prompt before any delete operation.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.files.deleteToTrash" (ngModelChange)="onSettingChanged()" />
                    <span>Send to Recycle Bin<small>Moves deleted files to the system trash instead of permanently erasing them.</small></span>
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
                    <input type="checkbox" [(ngModel)]="draft.files.copyOnDrop" (ngModelChange)="onSettingChanged()" />
                    <span>Copy on drag and drop by default<small>When dropping without a modifier key, copies instead of moving the files.</small></span>
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
                    <span>Auto-refresh Interval (ms)</span>
                    <input
                      type="number"
                      [(ngModel)]="draft.files.autoRefreshInterval"
                      (ngModelChange)="onSettingChanged()"
                      min="1000"
                      max="60000"
                      step="1000"
                    />
                    <small>How often to poll the filesystem for external changes. Lower values use more CPU.</small>
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
                    <small>Maximum disk space reserved for cached image and video thumbnails.</small>
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
                    <small>Comma-separated list of font names. The first available font on your system is used.</small>
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
                    <small>How long lines are wrapped in the editor. "At Column" and "Bounded" wrap at a fixed character width.</small>
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
                    <small>Automatically saves the open file without requiring an explicit save action.</small>
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
                    <input type="checkbox" [(ngModel)]="draft.editor.showLineNumbers" (ngModelChange)="onSettingChanged()" />
                    <span>Show line numbers<small>Renders line numbers in the left gutter of the editor.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.codeFolding" (ngModelChange)="onSettingChanged()" />
                    <span>Enable code folding<small>Shows fold controls in the gutter to collapse and expand sections of code.</small></span>
                  </label>
                  <label class="toggle-field">
                    <input type="checkbox" [(ngModel)]="draft.editor.highlightCurrentLine" (ngModelChange)="onSettingChanged()" />
                    <span>Highlight current line<small>Applies a subtle background tint to the line containing the cursor.</small></span>
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

              <!-- Shortcuts -->
              <section *ngIf="activeTab === 'shortcuts'" class="settings-section">
                <h3 class="section-title">Keyboard Shortcuts</h3>
                <p class="section-desc">Reference list of all keyboard shortcuts. Shortcuts are active when no text field is focused.</p>

                <ng-container *ngFor="let group of shortcutGroups">
                  <div class="settings-group shortcuts-group">
                    <h4 class="group-title">{{ group.category }}</h4>
                    <div class="shortcuts-table">
                      <div class="shortcut-row" *ngFor="let item of group.items">
                        <span class="shortcut-desc">{{ item.description }}</span>
                        <span class="shortcut-keys">
                          <ng-container *ngFor="let chord of item.chords; let last = last">
                            <span class="shortcut-chord">
                              <kbd class="key-badge" *ngFor="let k of chord">{{ k }}</kbd>
                            </span>
                            <span class="chord-then" *ngIf="!last">then</span>
                          </ng-container>
                        </span>
                      </div>
                    </div>
                  </div>
                </ng-container>
              </section>

              <!-- About -->
              <section *ngIf="activeTab === 'about'" class="settings-section about-section">
                <h3 class="section-title">About</h3>
                <p class="section-desc">Application version and license information.</p>

                <div class="settings-group">
                  <h4 class="group-title">Application</h4>
                  <div class="about-app-card">
                    <div class="about-app-icon">
                      <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z"/>
                      </svg>
                    </div>
                    <div class="about-app-info">
                      <span class="about-app-name">File Manager</span>
                      <span class="about-app-version">Version 1.0.0</span>
                      <span class="about-app-build">Built with Wails · Go · Angular</span>
                    </div>
                  </div>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Platform</h4>
                  <div class="about-kv-list">
                    <div class="about-kv-row">
                      <span class="about-kv-label">Runtime</span>
                      <span class="about-kv-value">Wails v2</span>
                    </div>
                    <div class="about-kv-row">
                      <span class="about-kv-label">WebView</span>
                      <span class="about-kv-value">Microsoft WebView2</span>
                    </div>
                    <div class="about-kv-row">
                      <span class="about-kv-label">Frontend</span>
                      <span class="about-kv-value">Angular 17</span>
                    </div>
                    <div class="about-kv-row">
                      <span class="about-kv-label">Backend</span>
                      <span class="about-kv-value">Go 1.22</span>
                    </div>
                  </div>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Updates</h4>
                  <div class="about-update-row">
                    <div class="about-update-status">
                      <span class="about-update-badge up-to-date">Up to date</span>
                      <span class="about-update-desc">You are running the latest version.</span>
                    </div>
                    <button type="button" class="btn btn-secondary">Check for Updates</button>
                  </div>
                </div>

                <div class="settings-group">
                  <h4 class="group-title">Open Source</h4>
                  <p class="about-oss-note">File Manager is built on top of open-source software. The following libraries and tools are used under their respective licenses.</p>
                  <div class="about-oss-grid">
                    <div class="about-oss-item">
                      <span class="oss-name">Wails</span>
                      <span class="oss-license">MIT</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">Angular</span>
                      <span class="oss-license">MIT</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">go-webview2</span>
                      <span class="oss-license">MIT</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">astiav / FFmpeg</span>
                      <span class="oss-license">LGPL 2.1</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">MuPDF</span>
                      <span class="oss-license">AGPL 3.0</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">ImageMagick</span>
                      <span class="oss-license">ImageMagick</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">LibreOffice</span>
                      <span class="oss-license">MPL 2.0</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">libvips</span>
                      <span class="oss-license">LGPL 2.1</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">DjVuLibre</span>
                      <span class="oss-license">GPL 2.0</span>
                    </div>
                    <div class="about-oss-item">
                      <span class="oss-name">Calibre</span>
                      <span class="oss-license">GPL 3.0</span>
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
      id: 'shortcuts',
      label: 'Shortcuts',
      divider: true,
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M14 5a1 1 0 0 1 1 1v5a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h12zm-12-1a2 2 0 0 0-2 2v5a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2H2z"/><path d="M3.5 7.5a.5.5 0 0 1 .5-.5h1a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5H4a.5.5 0 0 1-.5-.5v-1zm2.5 0a.5.5 0 0 1 .5-.5h1a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5H6.5a.5.5 0 0 1-.5-.5v-1zm2.5 0a.5.5 0 0 1 .5-.5h1a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5H9a.5.5 0 0 1-.5-.5v-1zm2.5 0a.5.5 0 0 1 .5-.5h2a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5h-2a.5.5 0 0 1-.5-.5v-1z"/></svg>',
    },
    {
      id: 'about',
      label: 'About',
      icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 15A7 7 0 1 1 8 1a7 7 0 0 1 0 14zm0 1A8 8 0 1 0 8 0a8 8 0 0 0 0 16z"/><path d="m8.93 6.588-2.29.287-.082.38.45.083c.294.07.352.176.288.469l-.738 3.468c-.194.897.105 1.319.808 1.319.545 0 1.178-.252 1.465-.598l.088-.416c-.2.176-.492.246-.686.246-.275 0-.375-.193-.304-.533L8.93 6.588zM9 4.5a1 1 0 1 1-2 0 1 1 0 0 1 2 0z"/></svg>',
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

  private formatStep(step: KeyStroke): string {
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

  /** Shortcut groups derived from BROWSER_BINDINGS, deduplicated by commandId+category. */
  get shortcutGroups(): Array<{ category: string; items: Array<{ description: string; chords: string[][] }> }> {
    const catMap = new Map<string, Map<string, { description: string; chords: string[][] }>>();

    for (const b of BROWSER_BINDINGS) {
      if (b.trigger.device !== 'keyboard') continue;
      const cat = b.category ?? 'General';
      if (!catMap.has(cat)) catMap.set(cat, new Map());
      const group = catMap.get(cat)!;

      const chords = b.trigger.chord.steps.map((s) => {
        const label = this.formatStep(s);
        return [label];
      });

      const key = `${b.commandId}:${cat}`;
      const existing = group.get(b.commandId);
      if (existing) {
        // merge alternative chord as separate display entry only if description differs
        if (existing.description !== b.description) {
          group.set(key, { description: b.description, chords });
        }
      } else {
        group.set(b.commandId, { description: b.description, chords });
      }
    }

    const categoryOrder = ['Navigation', 'Tab Management', 'File Operations', 'Search & Filter', 'Workspaces', 'Pane Control'];
    const result: Array<{ category: string; items: Array<{ description: string; chords: string[][] }> }> = [];
    for (const cat of categoryOrder) {
      const group = catMap.get(cat);
      if (group) {
        result.push({ category: cat, items: [...group.values()] });
        catMap.delete(cat);
      }
    }
    for (const [cat, group] of catMap) {
      result.push({ category: cat, items: [...group.values()] });
    }
    return result;
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