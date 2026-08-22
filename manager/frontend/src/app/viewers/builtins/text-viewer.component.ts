import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  Input,
  NgZone,
  OnChanges,
  OnDestroy,
  Renderer2,
  ViewChild,
} from '@angular/core';
import { Compartment, EditorState } from '@codemirror/state';
import {
  bracketMatching,
  foldGutter,
  foldKeymap,
  LanguageDescription,
  syntaxHighlighting,
} from '@codemirror/language';
import { languages } from '@codemirror/language-data';
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars,
  keymap,
  lineNumbers,
} from '@codemirror/view';
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
import { gotoLine, highlightSelectionMatches, openSearchPanel, searchKeymap } from '@codemirror/search';
import { classHighlighter } from '@lezer/highlight';
import { VscodeScrollbarController } from '../../directives/vscode-scrollbar.directive';
import { ApiService } from '../../services/api.service';
import { EditorSettings, SettingsService } from '../../services/settings.service';
import { Theme, ThemeService } from '../../services/theme.service';
import { FileInfo, FileProbe, FileType } from '../../shared/protocol';
import { ViewerArchiveContext } from '../viewer-archive-context';
import { VIEWER_PRODUCTION_CHROME_STYLES } from './builtin-viewer.shared';

type TextViewerKind = 'empty' | 'loading' | 'editor' | 'directory' | 'unsupported' | 'error';

function createFoldMarker(open: boolean): HTMLElement {
  const marker = document.createElement('span');
  marker.className = `cm-fold-marker ${open ? 'is-open' : 'is-closed'}`;
  marker.setAttribute('aria-hidden', 'true');
  marker.innerHTML = open
    ? '<svg viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M4 6L8 10L12 6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>'
    : '<svg viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M6 4L10 8L6 12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>';
  return marker;
}

function createCodePreviewTheme(isDark: boolean, settings: Pick<EditorSettings, 'fontFamily' | 'fontSize' | 'lineHeight'>) {
  return EditorView.theme({
  '&': {
    height: '100%',
    backgroundColor: 'var(--vsc-code-background)',
    color: 'var(--vsc-foreground)',
    fontSize: `${settings.fontSize}px`,
  },
  '.cm-editor': {
    backgroundColor: 'var(--vsc-code-background)',
  },
  '.cm-scroller': {
    overflow: 'auto',
    color: 'var(--vsc-foreground)',
    fontFamily: settings.fontFamily || "var(--vsc-font-family-mono, ui-monospace, 'Cascadia Code', 'SFMono-Regular', Consolas, monospace)",
    lineHeight: String(settings.lineHeight),
  },
  '.cm-content': {
    padding: '10px 0 24px',
    minHeight: '100%',
    caretColor: 'transparent',
    color: 'var(--vsc-foreground)',
  },
  '.cm-line': {
    padding: '0 16px',
    color: 'inherit',
  },
  '.cm-gutters': {
    backgroundColor: 'var(--vsc-code-gutter-background)',
    color: 'var(--vsc-code-gutter-foreground)',
    borderRight: '1px solid var(--vsc-code-border)',
  },
  '.cm-gutterElement': {
    padding: '0 10px 0 8px',
  },
  '.cm-lineNumbers .cm-gutterElement': {
    minWidth: '28px',
  },
  '.cm-foldGutter .cm-gutterElement': {
    padding: '0 4px',
  },
  '.cm-fold-marker': {
    display: 'inline-flex',
    width: '14px',
    height: '14px',
    alignItems: 'center',
    justifyContent: 'center',
    color: 'var(--vsc-code-gutter-foreground)',
    opacity: '0.85',
  },
  '.cm-fold-marker svg': {
    width: '12px',
    height: '12px',
    display: 'block',
  },
  '.cm-foldGutter .cm-gutterElement:hover .cm-fold-marker': {
    color: 'var(--vsc-foreground)',
    opacity: '1',
  },
  '.cm-activeLine': {
    backgroundColor: 'var(--vsc-code-active-line)',
  },
  '.cm-activeLineGutter': {
    color: 'var(--vsc-foreground)',
    backgroundColor: 'var(--vsc-code-active-line-gutter)',
  },
  '.cm-foldPlaceholder': {
    backgroundColor: 'color-mix(in srgb, var(--vsc-code-active-line-gutter) 80%, transparent)',
    border: '1px solid var(--vsc-code-border)',
    color: 'var(--vsc-foreground-dim)',
  },
  '.cm-panels': {
    backgroundColor: 'var(--vsc-code-panel-background)',
    color: 'var(--vsc-foreground)',
    borderBottom: '1px solid var(--vsc-code-border)',
    boxShadow: '0 10px 24px var(--vsc-shadow-soft)',
  },
  '.cm-panels-top': {
    borderBottom: '1px solid var(--vsc-code-border)',
  },
  '.cm-search': {
    padding: '8px 10px',
    gap: '8px',
    alignItems: 'center',
    flexWrap: 'wrap',
  },
  '.cm-search input, .cm-search button, .cm-gotoLine input, .cm-gotoLine button': {
    font: 'inherit',
    fontSize: '12px',
    color: 'var(--vsc-input-foreground)',
    backgroundColor: 'var(--vsc-input-background)',
    border: '1px solid var(--vsc-input-border)',
    borderRadius: '6px',
  },
  '.cm-search input:focus, .cm-gotoLine input:focus': {
    outline: '1px solid var(--vsc-input-focus-border)',
    borderColor: 'var(--vsc-input-focus-border)',
  },
  '.cm-search input, .cm-gotoLine input': {
    minWidth: '180px',
    padding: '5px 8px',
  },
  '.cm-search button, .cm-gotoLine button': {
    padding: '5px 9px',
    cursor: 'pointer',
  },
  '.cm-search button:hover, .cm-gotoLine button:hover': {
    backgroundColor: 'var(--vsc-list-hover-background)',
  },
  '.cm-search label': {
    fontSize: '11px',
    opacity: '0.85',
  },
  '.cm-searchMatch': {
    backgroundColor: 'var(--vsc-code-search-match)',
    outline: '1px solid color-mix(in srgb, var(--vsc-code-search-match) 85%, transparent)',
  },
  '.cm-searchMatch.cm-searchMatch-selected': {
    backgroundColor: 'var(--vsc-code-search-match-selected)',
    outline: '1px solid color-mix(in srgb, var(--vsc-code-search-match-selected) 92%, transparent)',
  },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection': {
    backgroundColor: 'var(--vsc-list-active-background)',
  },
  '.cm-matchingBracket, .cm-nonmatchingBracket': {
    backgroundColor: 'var(--vsc-code-bracket-match)',
    outline: '1px solid color-mix(in srgb, var(--vsc-code-bracket-match) 92%, transparent)',
  },
  // Syntax token colours — driven by classHighlighter .tok-* classes
  '.tok-keyword': { color: 'var(--vsc-code-keyword)' },
  '.tok-string': { color: 'var(--vsc-code-string)' },
  '.tok-string2': { color: 'var(--vsc-code-regexp)' },
  '.tok-number, .tok-literal, .tok-bool, .tok-atom': { color: 'var(--vsc-code-number)' },
  '.tok-comment': { color: 'var(--vsc-code-comment)', fontStyle: 'italic' },
  '.tok-variableName': { color: 'var(--vsc-code-variable)' },
  '.tok-variableName.tok-definition': { color: 'var(--vsc-code-function)' },
  '.tok-variableName2': { color: 'var(--vsc-code-function)' },
  '.tok-typeName, .tok-className, .tok-namespace, .tok-macroName': { color: 'var(--vsc-code-type)' },
  '.tok-propertyName, .tok-labelName': { color: 'var(--vsc-code-property)' },
  '.tok-operator, .tok-punctuation': { color: 'var(--vsc-code-operator)' },
  '.tok-meta, .tok-link, .tok-url': { color: 'var(--vsc-code-meta)' },
  '.tok-heading': { color: 'var(--vsc-code-heading)', fontWeight: '600' },
  '.tok-invalid': { color: 'var(--vsc-code-invalid)', textDecoration: 'wavy underline' },
  '.cm-cursor, .cm-dropCursor': {
    display: 'none',
  },
  '.cm-focused': {
    outline: 'none',
  },
  }, { dark: isDark });
}

@Component({
  selector: 'app-text-viewer',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <div class="viewer-header">
        <div class="header-main">
          <span class="file-icon">&lt;/&gt;</span>
          <div class="header-text">
            <div class="title" [title]="displayName">{{ displayName || 'Code Preview' }}</div>
            <div class="path" [title]="displayPath">{{ displayPath || 'Select a file to preview' }}</div>
          </div>
        </div>
        <div class="header-meta" *ngIf="kind === 'editor' || kind === 'directory'">
          <span class="badge archive-badge" *ngIf="archiveContext" title="Loaded from archive">
            &#x1F4E6; archive
          </span>
          <span class="badge type-badge">{{ typeBadge }}</span>
          <span class="badge size-badge" *ngIf="info?.size">{{ formatSize(info.size) }}</span>
          <span class="badge line-badge" *ngIf="lineCount !== null">{{ lineCount }} lines</span>
        </div>
      </div>

      <div class="viewer-toolbar" *ngIf="kind === 'editor'">
        <button class="tool-btn" type="button" (click)="toggleWordWrap()" [class.active]="wordWrap">
          Wrap
        </button>
        <button class="tool-btn" type="button" (click)="openSearch()">
          Search
        </button>
        <button class="tool-btn" type="button" (click)="openGoToLine()">
          Go to line
        </button>
        <span class="toolbar-divider"></span>
        <button class="tool-btn" type="button" (click)="copyPath()" [class.active]="copiedPath">
          {{ copiedPath ? 'Copied' : 'Copy path' }}
        </button>
        <button class="tool-btn" type="button" *ngIf="!archiveContext && targetPath" (click)="openExternally()">
          Open externally
        </button>
      </div>

      <div class="viewer-body">
        <div *ngIf="kind === 'loading'" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Loading preview…</div>
        </div>

        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon">&#x1F50D;</div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <div *ngIf="kind === 'directory'" class="state-container">
          <div class="state-icon">&#x1F4C1;</div>
          <div class="state-label">Directory — no preview available</div>
        </div>

        <div *ngIf="kind === 'unsupported'" class="state-container state-error">
          <div class="state-icon">&#x26A0;</div>
          <div class="state-label">Text preview is unavailable</div>
          <div class="state-sub">The file could not be decoded as UTF-8 text.</div>
        </div>

        <div *ngIf="kind === 'error'" class="state-container state-error">
          <div class="state-icon">&#x274C;</div>
          <div class="state-label">Failed to load preview</div>
          <div class="state-sub">The source could not be rendered in the code preview.</div>
          <div class="error-detail">{{ error }}</div>
        </div>

        <div *ngIf="kind === 'editor'" class="editor-shell">
          <div #editorHost class="editor-host"></div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }
    .viewer-surface {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-code-background, var(--vsc-editor-background, var(--vsc-editor-background)));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }
    .viewer-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      min-height: 42px;
      padding: 5px 10px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 92%, black);
      flex-shrink: 0;
      min-width: 0;
    }
    .header-main {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      flex: 1;
    }
    .file-icon {
      font-size: 16px;
      flex-shrink: 0;
      line-height: 1;
    }
    .header-text {
      min-width: 0;
      flex: 1;
    }
    .title {
      font-size: 12px;
      font-weight: 600;
      letter-spacing: 0;
      opacity: 0.9;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .path {
      font-family: var(--vsc-font-family-mono, ui-monospace, 'Courier New', monospace);
      font-size: 10px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      opacity: 1;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      margin-top: 1px;
    }
    .header-meta {
      display: flex;
      align-items: center;
      gap: 4px;
      flex-shrink: 0;
    }
    .badge {
      font-size: 9px;
      font-weight: 600;
      padding: 1px 5px;
      border-radius: 3px;
      white-space: nowrap;
      letter-spacing: 0.3px;
    }
    .archive-badge {
      background: color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-accent-orange)) 20%, transparent);
      color: var(--vsc-accent-orange, var(--vsc-warning));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-accent-orange)) 40%, transparent);
    }
    .type-badge {
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 40%, transparent);
    }
    .size-badge,
    .line-badge {
      background: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 8%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 50%, transparent);
    }
    .viewer-toolbar {
      display: flex;
      align-items: center;
      gap: 2px;
      min-height: 30px;
      padding: 2px 6px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 96%, black);
      flex-shrink: 0;
    }
    .tool-btn {
      background: transparent;
      border: 1px solid transparent;
      color: var(--vsc-foreground, var(--vsc-foreground));
      font-size: 11px;
      padding: 2px 7px;
      border-radius: 3px;
      cursor: pointer;
      line-height: 1.4;
      min-width: 24px;
      transition: background 0.1s, border-color 0.1s;
    }
    .tool-btn:hover {
      background: var(--vsc-list-hover-background, var(--vsc-neutral-overlay-subtle));
      border-color: var(--vsc-border, var(--vsc-border));
    }
    .tool-btn.active {
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
      border-color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 50%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
    }
    .toolbar-divider {
      width: 1px;
      height: 16px;
      background: var(--vsc-border, var(--vsc-border));
      margin: 0 4px;
    }
    .viewer-body {
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      display: flex;
      overflow: hidden;
    }
    .editor-shell {
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      display: flex;
    }
    .editor-host {
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }
    .state-container {
      display: flex;
      flex: 1 1 auto;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      width: 100%;
      height: 100%;
      min-height: 160px;
      gap: 8px;
      padding: 24px;
      text-align: center;
    }
    .state-icon {
      font-size: 36px;
      line-height: 1;
      opacity: 0.55;
    }
    .state-label {
      font-size: 13px;
      opacity: 0.65;
    }
    .state-sub {
      font-size: 11px;
      opacity: 0.4;
    }
    .state-error {
      color: inherit;
    }
    .state-error .state-icon {
      opacity: 1;
    }
    .state-error .state-label {
      color: var(--vsc-error);
      opacity: 1;
    }
    .error-detail {
      font-size: 11px;
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      color: color-mix(in srgb, var(--vsc-error-bright) 70%, var(--vsc-foreground, var(--vsc-foreground)));
      background: var(--vsc-error-surface);
      border: 1px solid var(--vsc-error-border);
      border-radius: 4px;
      padding: 8px 12px;
      max-width: 480px;
      white-space: pre-wrap;
      word-break: break-word;
    }
    .spinner {
      width: 26px;
      height: 26px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, var(--vsc-accent-blue));
      border-radius: 50%;
      animation: spin 0.65s linear infinite;
    }
    @keyframes spin {
      to { transform: rotate(360deg); }
    }
  `, VIEWER_PRODUCTION_CHROME_STYLES],
})
export class TextViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;

  @ViewChild('editorHost')
  set editorHostRef(value: ElementRef<HTMLDivElement> | undefined) {
    this.editorHost = value ?? null;
    if (this.editorHost && this.pendingDocument !== null) {
      void this.mountEditor(this.pendingDocument, this.pendingFilename, this.pendingLoadId);
    }
  }

  kind: TextViewerKind = 'empty';
  info: FileInfo | null = null;
  error = '';
  lineCount: number | null = null;
  wordWrap = false;
  copiedPath = false;

  private editorHost: ElementRef<HTMLDivElement> | null = null;
  private editorView: EditorView | null = null;
  private editorScrollbar: VscodeScrollbarController | null = null;
  private pendingDocument: string | null = null;
  private pendingFilename = 'untitled.txt';
  private pendingLoadId = 0;
  private destroyed = false;
  private loadId = 0;
  private copyResetTimer: number | null = null;

  private readonly languageCompartment = new Compartment();
  private readonly wrapCompartment = new Compartment();
  private readonly lineNumbersCompartment = new Compartment();
  private readonly foldGutterCompartment = new Compartment();
  private readonly activeLineCompartment = new Compartment();
  private readonly activeLineGutterCompartment = new Compartment();
  private readonly selectionMatchesCompartment = new Compartment();
  private readonly bracketMatchingCompartment = new Compartment();
  private readonly themeCompartment = new Compartment();
  private readonly themeSubscription: { unsubscribe(): void };
  private readonly settingsSubscription: { unsubscribe(): void };
  private currentTheme: Theme;
  private editorSettings: EditorSettings;

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
    private themeService: ThemeService,
    private settingsService: SettingsService,
    private renderer: Renderer2,
    private ngZone: NgZone,
  ) {
    this.editorSettings = this.settingsService.getSettings().editor;
    this.wordWrap = this.isWordWrapEnabled(this.editorSettings.wordWrap);
    this.currentTheme = this.themeService.getTheme$().value;
    this.themeSubscription = this.themeService.getTheme$().subscribe((theme: Theme) => {
      this.currentTheme = theme;
      if (this.editorView) {
        this.editorView.dispatch({
          effects: this.themeCompartment.reconfigure(this.resolveThemeExtension(theme)),
        });
      }
    });
    this.settingsSubscription = this.settingsService.getSettings$().subscribe((settings) => {
      this.applyEditorSettings(settings.editor);
    });
  }

  get displayName(): string {
    if (this.info?.name) return this.info.name;
    const source = this.archiveContext?.entryPath ?? this.targetPath;
    return source ? this.baseName(source) : '';
  }

  get displayPath(): string {
    if (this.archiveContext) {
      return `${this.archiveContext.fsPath} :: ${this.archiveContext.entryPath}`;
    }
    return this.targetPath;
  }

  get fileExt(): string {
    return this.extractExt(this.archiveContext?.entryPath ?? this.targetPath);
  }

  get typeBadge(): string {
    if (this.fileExt) return this.fileExt.toUpperCase();
    return 'TEXT';
  }

  ngOnChanges(): void {
    void this.reload();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.themeSubscription.unsubscribe();
    this.settingsSubscription.unsubscribe();
    if (this.copyResetTimer !== null) {
      window.clearTimeout(this.copyResetTimer);
      this.copyResetTimer = null;
    }
    this.editorView?.destroy();
    this.editorView = null;
    this.editorScrollbar?.destroy();
    this.editorScrollbar = null;
  }

  toggleWordWrap(): void {
    this.settingsService.updateEditorSettings({ wordWrap: this.wordWrap ? 'off' : 'on' });
  }

  async openExternally(): Promise<void> {
    if (!this.targetPath || this.archiveContext) return;
    await this.api.openFile(this.targetPath);
  }

  openSearch(): void {
    if (!this.editorView) return;
    this.editorView.focus();
    openSearchPanel(this.editorView);
  }

  openGoToLine(): void {
    if (!this.editorView) return;
    this.editorView.focus();
    gotoLine(this.editorView);
  }

  async copyPath(): Promise<void> {
    const path = this.displayPath?.trim();
    if (!path) return;

    await navigator.clipboard.writeText(path).catch(() => {});
    this.copiedPath = true;
    if (this.copyResetTimer !== null) {
      window.clearTimeout(this.copyResetTimer);
    }
    this.copyResetTimer = window.setTimeout(() => {
      this.copiedPath = false;
      this.copyResetTimer = null;
      this.cdr.markForCheck();
    }, 1200);
    this.cdr.markForCheck();
  }

  formatSize(bytes: number): string {
    if (!bytes || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, i);
    return `${i === 0 ? value : value.toFixed(1)} ${units[i]}`;
  }

  private async reload(): Promise<void> {
    const path = (this.targetPath ?? '').trim();
    const currentLoad = ++this.loadId;

    this.error = '';
    this.info = null;
    this.lineCount = null;
    this.pendingDocument = null;

    if (!path && !this.archiveContext) {
      this.kind = 'empty';
      this.editorScrollbar?.destroy();
      this.editorScrollbar = null;
      this.editorView?.destroy();
      this.editorView = null;
      this.cdr.markForCheck();
      return;
    }

    this.kind = 'loading';
    this.cdr.markForCheck();

    try {
      const result = this.archiveContext
        ? await this.loadFromArchive(this.archiveContext)
        : await this.loadFromDisk(path);

      if (this.destroyed || currentLoad !== this.loadId) return;

      if (result.kind === 'directory') {
        this.kind = 'directory';
        this.editorScrollbar?.destroy();
        this.editorScrollbar = null;
        this.editorView?.destroy();
        this.editorView = null;
        this.cdr.markForCheck();
        return;
      }

      this.lineCount = result.text.split(/\r?\n/).length;
      this.pendingDocument = result.text;
      this.pendingFilename = result.filename;
      this.pendingLoadId = currentLoad;
      this.kind = 'editor';
      this.cdr.markForCheck();

      if (this.editorHost) {
        await this.mountEditor(result.text, result.filename, currentLoad);
      }
    } catch (error) {
      if (this.destroyed || currentLoad !== this.loadId) return;
      this.kind = 'error';
      this.error = error instanceof Error ? error.message : String(error);
      this.editorScrollbar?.destroy();
      this.editorScrollbar = null;
      this.editorView?.destroy();
      this.editorView = null;
      this.cdr.markForCheck();
    }
  }

  private async loadFromDisk(path: string): Promise<{ kind: 'editor' | 'directory'; text: string; filename: string }> {
    const info = await this.api.getFileInfo(path);
    this.info = info;

    if (info.type === FileType.DIRECTORY) {
      return { kind: 'directory', text: '', filename: info.name };
    }

    const encoding = this.normalizeEncoding(this.probe?.encoding ?? '');
    const streamURL = await this.api.getStreamURL(path);
    const file = await this.api.fetchTextFromURL(streamURL, encoding);

    return {
      kind: 'editor',
      text: file.content,
      filename: info.name || this.baseName(path),
    };
  }

  private async loadFromArchive(ctx: ViewerArchiveContext): Promise<{ kind: 'editor'; text: string; filename: string }> {
    const encoding = this.normalizeEncoding(this.probe?.encoding ?? '');
    const streamURL = await this.api.getArchiveStreamURL(ctx.fsPath, ctx.nestedChain, ctx.entryPath);
    const response = await this.api.fetchTextFromURL(streamURL, encoding);

    const entryName = this.baseName(ctx.entryPath);
    this.info = {
      name: entryName,
      path: ctx.entryPath,
      type: FileType.FILE,
      size: response.size,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
    };

    return {
      kind: 'editor',
      text: response.content,
      filename: entryName,
    };
  }

  private async mountEditor(text: string, filename: string, loadId: number): Promise<void> {
    if (!this.editorHost || this.destroyed || loadId !== this.loadId) return;

    const host = this.editorHost.nativeElement;
    const languageExtension = await this.resolveLanguage(filename);
    if (!this.editorHost || this.destroyed || loadId !== this.loadId) return;

    const extensions = [
      this.lineNumbersCompartment.of(this.editorSettings.showLineNumbers ? lineNumbers() : []),
      this.activeLineGutterCompartment.of(this.editorSettings.highlightCurrentLine ? highlightActiveLineGutter() : []),
      highlightSpecialChars(),
      history(),
      this.foldGutterCompartment.of(this.editorSettings.codeFolding ? foldGutter({ markerDOM: createFoldMarker }) : []),
      drawSelection(),
      syntaxHighlighting(classHighlighter),
      this.bracketMatchingCompartment.of(this.editorSettings.matchBrackets === 'never' ? [] : bracketMatching()),
      this.activeLineCompartment.of(this.editorSettings.highlightCurrentLine ? highlightActiveLine() : []),
      this.selectionMatchesCompartment.of(this.editorSettings.occurrencesHighlight ? highlightSelectionMatches() : []),
      this.themeCompartment.of(this.resolveThemeExtension(this.currentTheme)),
      EditorState.readOnly.of(true),
      EditorView.editable.of(false),
      keymap.of([...defaultKeymap, ...historyKeymap, ...foldKeymap, ...searchKeymap]),
      this.wrapCompartment.of(this.isWordWrapEnabled(this.editorSettings.wordWrap) ? EditorView.lineWrapping : []),
      this.languageCompartment.of(languageExtension ?? []),
    ];

    if (!this.editorView) {
      this.editorView = new EditorView({
        state: EditorState.create({ doc: text, extensions }),
        parent: host,
      });
      this.attachEditorScrollbar();
      return;
    }

    this.editorView.setState(EditorState.create({ doc: text, extensions }));
    this.attachEditorScrollbar();
  }

  private attachEditorScrollbar(): void {
    if (!this.editorView) return;

    const editorRoot = this.editorView.dom;
    const scrollHost = editorRoot.querySelector<HTMLElement>('.cm-scroller');
    if (!scrollHost) return;

    this.editorScrollbar?.destroy();
    this.editorScrollbar = new VscodeScrollbarController(scrollHost, this.renderer, this.ngZone, editorRoot);
    this.editorScrollbar.init();
  }

  private async resolveLanguage(filename: string): Promise<any[] | null> {
    const match = LanguageDescription.matchFilename(languages, filename);
    if (!match) return null;

    try {
      const support = await match.load();
      return [support];
    } catch {
      return null;
    }
  }

  private normalizeEncoding(raw: string): string {
    const encoding = (raw ?? '').trim().toLowerCase();
    if (!encoding || encoding === 'utf-8') return '';
    if (encoding === 'utf16' || encoding === 'utf-16') return 'utf16le';
    return encoding;
  }

  private resolveThemeExtension(theme: Theme) {
    return createCodePreviewTheme(theme === 'dark', this.editorSettings);
  }

  private applyEditorSettings(settings: EditorSettings): void {
    this.editorSettings = settings;
    this.wordWrap = this.isWordWrapEnabled(settings.wordWrap);

    if (this.editorView) {
      this.editorView.dispatch({
        effects: [
          this.lineNumbersCompartment.reconfigure(settings.showLineNumbers ? lineNumbers() : []),
          this.foldGutterCompartment.reconfigure(settings.codeFolding ? foldGutter({ markerDOM: createFoldMarker }) : []),
          this.activeLineCompartment.reconfigure(settings.highlightCurrentLine ? highlightActiveLine() : []),
          this.activeLineGutterCompartment.reconfigure(settings.highlightCurrentLine ? highlightActiveLineGutter() : []),
          this.selectionMatchesCompartment.reconfigure(settings.occurrencesHighlight ? highlightSelectionMatches() : []),
          this.bracketMatchingCompartment.reconfigure(settings.matchBrackets === 'never' ? [] : bracketMatching()),
          this.wrapCompartment.reconfigure(this.wordWrap ? EditorView.lineWrapping : []),
          this.themeCompartment.reconfigure(this.resolveThemeExtension(this.currentTheme)),
        ],
      });
    }

    this.cdr.markForCheck();
  }

  private isWordWrapEnabled(wordWrap: EditorSettings['wordWrap']): boolean {
    return wordWrap !== 'off';
  }

  private baseName(path: string): string {
    return path.replace(/\\/g, '/').split('/').pop() || path;
  }

  private extractExt(path: string): string {
    const normalized = path.replace(/\\/g, '/');
    const base = normalized.split('/').pop() || normalized;
    if (base.toLowerCase() === 'dockerfile') return 'dockerfile';
    if (base.startsWith('.')) return base.slice(1).toLowerCase();
    const index = base.lastIndexOf('.');
    return index >= 0 ? base.slice(index + 1).toLowerCase() : '';
  }
}
