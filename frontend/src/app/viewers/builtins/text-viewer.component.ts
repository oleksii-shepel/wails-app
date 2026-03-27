import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  Input,
  OnChanges,
  OnDestroy,
  ViewChild,
} from '@angular/core';
import { basicSetup } from 'codemirror';
import { Compartment, EditorState } from '@codemirror/state';
import {
  defaultHighlightStyle,
  LanguageDescription,
  syntaxHighlighting,
} from '@codemirror/language';
import { languages } from '@codemirror/language-data';
import { EditorView, keymap } from '@codemirror/view';
import { defaultKeymap, historyKeymap } from '@codemirror/commands';
import { searchKeymap } from '@codemirror/search';
import { ApiService } from '../../services/api.service';
import { FileInfo, FileType } from '../../shared/protocol';
import { ViewerArchiveContext } from '../viewer-archive-context';

type TextViewerKind = 'empty' | 'loading' | 'editor' | 'directory' | 'unsupported' | 'error';

const codePreviewTheme = EditorView.theme({
  '&': {
    height: '100%',
    backgroundColor: 'var(--vsc-editor-background, #1e1e1e)',
    color: 'var(--vsc-foreground, #cccccc)',
    fontSize: '13px',
  },
  '.cm-scroller': {
    overflow: 'auto',
    fontFamily: "var(--vsc-font-family-mono, ui-monospace, 'Cascadia Code', 'SFMono-Regular', Consolas, monospace)",
    lineHeight: '1.6',
  },
  '.cm-content': {
    padding: '10px 0 24px',
    minHeight: '100%',
  },
  '.cm-line': {
    padding: '0 16px',
  },
  '.cm-gutters': {
    backgroundColor: 'color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 94%, #000)',
    color: 'color-mix(in srgb, var(--vsc-foreground, #cccccc) 55%, transparent)',
    borderRight: '1px solid var(--vsc-border, #333)',
  },
  '.cm-activeLine': {
    backgroundColor: 'color-mix(in srgb, var(--vsc-editor-selectionBackground, #264f78) 28%, transparent)',
  },
  '.cm-activeLineGutter': {
    backgroundColor: 'color-mix(in srgb, var(--vsc-editor-selectionBackground, #264f78) 20%, transparent)',
  },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection': {
    backgroundColor: 'color-mix(in srgb, var(--vsc-editor-selectionBackground, #264f78) 70%, transparent)',
  },
  '.cm-cursor, .cm-dropCursor': {
    display: 'none',
  },
  '.cm-focused': {
    outline: 'none',
  },
}, { dark: true });

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
          <span class="badge size-badge" *ngIf="info?.size">{{ formatSize(info?.size ?? 0) }}</span>
          <span class="badge line-badge" *ngIf="lineCount !== null">{{ lineCount }} lines</span>
        </div>
      </div>

      <div class="viewer-toolbar" *ngIf="kind === 'editor'">
        <button class="tool-btn" type="button" (click)="toggleWordWrap()" [class.active]="wordWrap">
          Wrap
        </button>
        <button class="tool-btn" type="button" *ngIf="!archiveContext && targetPath" (click)="openExternally()">
          Open externally
        </button>
      </div>

      <div class="viewer-body">
        <div *ngIf="kind === 'loading'" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Loading preview...</div>
        </div>

        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon">&#x1F50D;</div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <div *ngIf="kind === 'directory'" class="state-container">
          <div class="state-icon">&#x1F4C1;</div>
          <div class="state-label">Directory preview is unavailable</div>
        </div>

        <div *ngIf="kind === 'unsupported'" class="state-container state-error">
          <div class="state-icon">&#x26A0;</div>
          <div class="state-label">Text preview is unavailable</div>
          <div class="error-detail">The file could not be decoded as UTF-8 text.</div>
        </div>

        <div *ngIf="kind === 'error'" class="state-container state-error">
          <div class="state-icon">&#x274C;</div>
          <div class="state-label">Failed to load preview</div>
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
      background: var(--vsc-editor-background, #1e1e1e);
      color: var(--vsc-foreground, #cccccc);
    }
    .viewer-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border-bottom: 1px solid var(--vsc-border, #333);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 92%, #000);
    }
    .header-main {
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 0;
    }
    .file-icon {
      font-size: 18px;
      opacity: 0.9;
    }
    .header-text {
      min-width: 0;
    }
    .title {
      font-weight: 600;
      font-size: 13px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .path {
      font-size: 11px;
      opacity: 0.75;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .header-meta {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-wrap: wrap;
      justify-content: flex-end;
    }
    .badge {
      font-size: 10px;
      padding: 2px 6px;
      border: 1px solid var(--vsc-border, #444);
      border-radius: 999px;
      opacity: 0.9;
    }
    .archive-badge {
      border-color: color-mix(in srgb, var(--vsc-accent-orange, #f90) 60%, var(--vsc-border, #444));
    }
    .viewer-toolbar {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 6px 10px;
      border-bottom: 1px solid var(--vsc-border, #333);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 95%, #000);
    }
    .tool-btn {
      font: inherit;
      font-size: 12px;
      padding: 4px 8px;
      background: transparent;
      color: var(--vsc-foreground, #ccc);
      border: 1px solid var(--vsc-border, #444);
      border-radius: 6px;
      cursor: pointer;
    }
    .tool-btn.active {
      background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
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
      height: 100%;
      width: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 10px;
      color: var(--vsc-foreground, #ccc);
      opacity: 0.85;
      user-select: none;
      padding: 20px;
      text-align: center;
    }
    .state-icon {
      font-size: 22px;
    }
    .state-label {
      font-size: 13px;
      font-weight: 600;
    }
    .state-error {
      color: color-mix(in srgb, var(--vsc-accent-red, #f14c4c) 70%, var(--vsc-foreground, #ccc));
    }
    .error-detail {
      font-size: 12px;
      opacity: 0.9;
      max-width: 680px;
      white-space: pre-wrap;
    }
    .spinner {
      width: 22px;
      height: 22px;
      border-radius: 50%;
      border: 2px solid color-mix(in srgb, var(--vsc-foreground, #ccc) 20%, transparent);
      border-top-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 80%, transparent);
      animation: spin 0.9s linear infinite;
    }
    @keyframes spin {
      to { transform: rotate(360deg); }
    }
  `],
})
export class TextViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;

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

  private editorHost: ElementRef<HTMLDivElement> | null = null;
  private editorView: EditorView | null = null;
  private pendingDocument: string | null = null;
  private pendingFilename = 'untitled.txt';
  private pendingLoadId = 0;
  private destroyed = false;
  private loadId = 0;

  private readonly languageCompartment = new Compartment();
  private readonly wrapCompartment = new Compartment();

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

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
    this.editorView?.destroy();
    this.editorView = null;
  }

  toggleWordWrap(): void {
    this.wordWrap = !this.wordWrap;
    if (this.editorView) {
      this.editorView.dispatch({
        effects: this.wrapCompartment.reconfigure(this.wordWrap ? EditorView.lineWrapping : []),
      });
    }
    this.cdr.markForCheck();
  }

  async openExternally(): Promise<void> {
    if (!this.targetPath || this.archiveContext) return;
    await this.api.openFile(this.targetPath);
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

    const file = await this.api.readFile(path, 'utf8');
    if (!file || typeof file.content !== 'string') {
      throw new Error('The file could not be read as UTF-8 text.');
    }

    return {
      kind: 'editor',
      text: file.content,
      filename: info.name || this.baseName(path),
    };
  }

  private async loadFromArchive(ctx: ViewerArchiveContext): Promise<{ kind: 'editor'; text: string; filename: string }> {
    const response = await this.api.archiveRead(ctx.fsPath, ctx.nestedChain, ctx.entryPath, 'utf8');
    if (!response || typeof response.content !== 'string') {
      throw new Error('The archive entry could not be read as UTF-8 text.');
    }

    const entryName = this.baseName(ctx.entryPath);
    this.info = {
      name: entryName,
      path: ctx.entryPath,
      type: FileType.FILE,
      size: response.size ?? 0,
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
      basicSetup,
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      codePreviewTheme,
      EditorState.readOnly.of(true),
      EditorView.editable.of(false),
      keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap]),
      this.wrapCompartment.of(this.wordWrap ? EditorView.lineWrapping : []),
      this.languageCompartment.of(languageExtension ?? []),
    ];

    if (!this.editorView) {
      this.editorView = new EditorView({
        state: EditorState.create({ doc: text, extensions }),
        parent: host,
      });
      return;
    }

    this.editorView.setState(EditorState.create({ doc: text, extensions }));
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