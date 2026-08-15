import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  Input,
  OnInit,
  Output,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { DirectorySourceService } from '../services/directory-source.service';
import { ApiService } from '../services/api.service';
import { PathHistoryService } from '../services/path-history.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { getSourceCapabilities } from '../services/source-capabilities';
import { FileInfo, FileType } from '../shared/protocol';
import type { ViewerFileListItem } from '../shared/protocol-enhanced';
import { ViewerHostSurfaceComponent } from '../viewers/viewer-host-surface.component';
import { ReassemblyAddressBarComponent } from './reassembly-address-bar.component';
import { ReassemblyNavigationController } from './reassembly-navigation.controller';

type ReassemblyTab =
  | {
      id: string;
      kind: 'dir';
      path: string;
      title: string;
    }
  | {
      id: string;
      kind: 'viewer';
      path: string;
      title: string;
      targetPath: string;
      fileList: ViewerFileListItem[];
    };

@Component({
  selector: 'app-reassembly-directory-pane',
  standalone: true,
  imports: [CommonModule, ReassemblyAddressBarComponent, ViewerHostSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      class="pane"
      [class.active-pane]="active"
      [attr.data-pane-active]="active ? 'true' : null"
      [attr.aria-label]="paneId === 'left' ? 'Left file pane' : 'Right file pane'"
      (mousedown)="activateRequested.emit()">

      <div class="tab-strip" role="tablist" [attr.aria-label]="paneId + ' pane tabs'">
        <button
          *ngFor="let tab of tabs; trackBy: trackTab"
          type="button"
          class="tab"
          role="tab"
          [attr.aria-selected]="tab.id === activeTabId"
          [class.active]="tab.id === activeTabId"
          [title]="tab.path"
          (click)="activateTab(tab.id)">
          <span class="tab-title">
            <span class="tab-kind" aria-hidden="true">{{ tab.kind === 'viewer' ? '◉' : '▣' }}</span>
            {{ tab.title }}
          </span>
          <span
            class="tab-close"
            role="button"
            aria-label="Close tab"
            (mousedown)="$event.preventDefault(); $event.stopPropagation()"
            (click)="closeTab(tab.id); $event.stopPropagation()">
            ×
          </span>
        </button>

        <button
          type="button"
          class="new-tab"
          title="New tab (Ctrl+T)"
          aria-label="New tab"
          (click)="newTab()">
          +
        </button>
      </div>

      <ng-container *ngIf="activeTab?.kind !== 'viewer'; else viewerSurface">
      <header class="navigation-bar">
        <div class="nav-cluster">
          <button
            type="button"
            class="nav-button"
            [disabled]="loading || !canBack"
            (click)="goBack()"
            aria-label="Back"
            title="Back (Alt+Left)">
            ←
          </button>

          <button
            type="button"
            class="nav-button"
            [disabled]="loading || !canForward"
            (click)="goForward()"
            aria-label="Forward"
            title="Forward (Alt+Right)">
            →
          </button>

          <button
            type="button"
            class="nav-button"
            [disabled]="loading || atRoot"
            (click)="navigateUp()"
            aria-label="Parent directory"
            title="Parent directory (Backspace)">
            ↑
          </button>
        </div>

        <app-reassembly-address-bar
          #addressBar
          [paneId]="paneId"
          [path]="path"
          (navigate)="navigateTo($event)">
        </app-reassembly-address-bar>

        <button
          type="button"
          class="nav-button refresh"
          [disabled]="loading"
          (click)="refresh()"
          aria-label="Refresh"
          title="Refresh (Ctrl+R)">
          ↻
        </button>
      </header>

      <div class="status-row">
        <span *ngIf="loading">Loading…</span>
        <span *ngIf="!loading && !error">{{ entries.length }} items</span>
        <span *ngIf="selectedPaths.size">{{ selectedPaths.size }} selected</span>
        <span *ngIf="clipboard" class="clipboard-state">
          {{ clipboard.operation === 'copy' ? 'Copied' : 'Cut' }}: {{ clipboard.paths.length }}
        </span>
        <span class="hint">Ctrl+L location</span>
      </div>

      <div class="action-bar">
        <button type="button" [disabled]="loading || !canCreate" (click)="newFolder()" title="New folder (F7)">New folder</button>
        <button type="button" [disabled]="loading || !canRename" (click)="renameSelected()" title="Rename (F2)">Rename</button>
        <span class="action-separator"></span>
        <button type="button" [disabled]="loading || !hasSelection" (click)="copySelected()" title="Copy (Ctrl+C)">Copy</button>
        <button type="button" [disabled]="loading || !canCut" (click)="cutSelected()" title="Cut (Ctrl+X)">Cut</button>
        <button type="button" [disabled]="loading || !canPaste" (click)="paste()" title="Paste (Ctrl+V)">Paste</button>
        <span class="action-separator"></span>
        <button type="button" class="danger" [disabled]="loading || !canDelete" (click)="deleteSelected()" title="Delete (Delete / F8)">Delete</button>
      </div>

      <div class="error" *ngIf="error">
        <strong>Directory could not be loaded.</strong>
        <span>{{ error }}</span>
        <button type="button" (click)="refresh()">Retry</button>
      </div>

      <div
        class="list"
        *ngIf="!error"
        role="listbox"
        [attr.aria-busy]="loading">

        <button
          *ngFor="let entry of entries; trackBy: trackEntry"
          type="button"
          class="entry"
          role="option"
          [attr.data-file-entry]="entry.path"
          [attr.aria-selected]="selectedPaths.has(entry.path)"
          [class.cursor]="entry.path === cursorPath"
          [class.selected]="selectedPaths.has(entry.path)"
          (click)="selectFromPointer(entry, $event)"
          (dblclick)="openEntry(entry)">

          <span class="icon" aria-hidden="true">
            {{ entry.type === FileType.DIRECTORY ? '📁' : entry.type === FileType.ARCHIVE ? '🗜️' : '📄' }}
          </span>

          <span class="name">{{ entry.name }}</span>

          <span class="kind">
            {{ entry.type === FileType.DIRECTORY ? 'Folder' : entry.type === FileType.ARCHIVE ? 'Archive' : 'File' }}
          </span>

          <span class="size">
            {{ entry.type === FileType.DIRECTORY ? '' : formatSize(entry.size) }}
          </span>
        </button>

        <div class="empty" *ngIf="!loading && entries.length === 0">
          This directory is empty.
        </div>
      </div>
      </ng-container>

      <ng-template #viewerSurface>
        <div class="viewer-shell" *ngIf="activeViewerTab as viewerTab">
          <div class="viewer-toolbar">
            <span class="viewer-label">Preview</span>
            <span class="viewer-path" [title]="viewerTab.targetPath">{{ viewerTab.targetPath }}</span>
            <button type="button" (click)="closeActiveTab()" title="Close viewer (Ctrl+W)">×</button>
          </div>

          <app-viewer-host-surface
            #viewerHost
            [targetPath]="viewerTab.targetPath"
            [fileList]="viewerTab.fileList"
            [archiveContext]="null"
            [viewerId]="null">
          </app-viewer-host-surface>
        </div>
      </ng-template>
    </section>
  `,
  styles: [`
    :host {
      display: block;
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
    }

    .pane {
      box-sizing: border-box;
      display: flex;
      flex-direction: column;
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background, #1e1e1e);
      color: var(--vsc-foreground, #cccccc);
      border: 1px solid transparent;
    }

    .pane.active-pane {
      border-color: color-mix(in srgb, var(--vsc-accent-blue, #007acc) 62%, transparent);
    }

    .tab-strip {
      display: flex;
      align-items: stretch;
      min-height: 31px;
      overflow-x: auto;
      overflow-y: hidden;
      border-bottom: 1px solid var(--vsc-border, #3c3c3c);
      background: var(--vsc-tabbar-background, #252526);
      scrollbar-width: thin;
    }

    .tab,
    .new-tab {
      flex: 0 0 auto;
      min-width: 0;
      height: 30px;
      border: 0;
      border-right: 1px solid var(--vsc-border-subtle, #2b2b2b);
      background: var(--vsc-tab-inactive-background, #2d2d2d);
      color: var(--vsc-description-foreground, #969696);
      cursor: pointer;
      font: inherit;
    }

    .tab {
      display: grid;
      grid-template-columns: minmax(48px, 1fr) 20px;
      align-items: center;
      width: min(180px, 34vw);
      padding: 0 4px 0 9px;
      text-align: left;
    }

    .tab.active {
      background: var(--vsc-editor-background, #1e1e1e);
      color: var(--vsc-foreground, #cccccc);
      box-shadow: inset 0 2px 0 var(--vsc-accent-blue, #007acc);
    }

    .tab:hover {
      background: var(--vsc-list-hover-background, #2a2d2e);
      color: var(--vsc-foreground, #cccccc);
    }

    .tab-title {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 11px;
    }

    .tab-close {
      display: grid;
      place-items: center;
      width: 18px;
      height: 18px;
      border-radius: 3px;
      opacity: .45;
      font-size: 15px;
      line-height: 1;
    }

    .tab:hover .tab-close,
    .tab.active .tab-close {
      opacity: .8;
    }

    .tab-close:hover {
      background: var(--vsc-list-hover-background, #3a3d41);
      opacity: 1;
    }

    .new-tab {
      width: 32px;
      background: transparent;
      font-size: 18px;
    }

    .new-tab:hover {
      background: var(--vsc-list-hover-background, #2a2d2e);
      color: var(--vsc-foreground, #cccccc);
    }

    .navigation-bar {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) 34px;
      align-items: center;
      gap: 7px;
      padding: 6px 8px;
      border-bottom: 1px solid var(--vsc-border, #3c3c3c);
      background: var(--vsc-tabbar-background, #252526);
    }

    .nav-cluster {
      display: flex;
      align-items: center;
      gap: 2px;
    }

    .nav-button {
      width: 32px;
      height: 30px;
      padding: 0;
      border: 1px solid transparent;
      border-radius: 4px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font-size: 15px;
    }

    .nav-button:hover:not(:disabled) {
      background: var(--vsc-list-hover-background, #2a2d2e);
    }

    .nav-button:disabled {
      opacity: .28;
      cursor: default;
    }

    .status-row {
      display: flex;
      gap: 14px;
      min-height: 24px;
      align-items: center;
      padding: 0 10px;
      border-bottom: 1px solid var(--vsc-border-subtle, #2b2b2b);
      color: var(--vsc-description-foreground, #969696);
      font-size: 11px;
    }

    .hint {
      margin-left: auto;
      opacity: .7;
    }

    .clipboard-state {
      color: var(--vsc-accent-blue, #4aa3ff);
    }

    .action-bar {
      display: flex;
      align-items: center;
      gap: 4px;
      min-height: 34px;
      padding: 3px 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, #2b2b2b);
      background: var(--vsc-tabbar-background, #252526);
    }

    .action-bar button {
      min-height: 26px;
      padding: 3px 9px;
      border: 1px solid transparent;
      border-radius: 4px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font: inherit;
      font-size: 11px;
    }

    .action-bar button:hover:not(:disabled) {
      background: var(--vsc-list-hover-background, #2a2d2e);
      border-color: var(--vsc-border, #3c3c3c);
    }

    .action-bar button:disabled {
      opacity: .32;
      cursor: default;
    }

    .action-bar .danger:not(:disabled) {
      color: var(--vsc-error-foreground, #f48771);
    }

    .action-separator {
      width: 1px;
      height: 18px;
      margin: 0 3px;
      background: var(--vsc-border, #3c3c3c);
    }

    .list {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      padding: 4px;
    }

    .entry {
      box-sizing: border-box;
      display: grid;
      grid-template-columns: 28px minmax(0, 1fr) 100px 100px;
      align-items: center;
      width: 100%;
      min-height: 30px;
      padding: 0 8px;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      text-align: left;
      cursor: default;
      font: inherit;
    }

    .entry:hover {
      background: var(--vsc-list-hover-background, #2a2d2e);
    }

    .entry.cursor {
      box-shadow: inset 2px 0 0 var(--vsc-accent-blue, #007acc);
      background: color-mix(
        in srgb,
        var(--vsc-list-hover-background, #2a2d2e) 75%,
        transparent
      );
    }

    .entry.selected {
      background: var(--vsc-list-active-background, #094771);
      color: var(--vsc-list-active-foreground, #ffffff);
    }

    .entry.cursor.selected {
      box-shadow: inset 2px 0 0 var(--vsc-focus-border, #75beff);
    }

    .icon {
      width: 20px;
      text-align: center;
    }

    .name {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .kind,
    .size {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, #969696);
      font-size: 11px;
    }

    .selected .kind,
    .selected .size {
      color: inherit;
      opacity: .8;
    }

    .size {
      text-align: right;
    }

    .empty,
    .error {
      display: grid;
      place-items: center;
      gap: 8px;
      padding: 40px;
      color: var(--vsc-description-foreground, #969696);
    }

    .error {
      flex: 1 1 auto;
    }

    .error button {
      padding: 5px 10px;
    }

    .tab-kind {
      margin-right: 4px;
      opacity: .65;
      font-size: 9px;
    }

    .viewer-shell {
      display: flex;
      flex: 1 1 auto;
      flex-direction: column;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
      background: var(--vsc-editor-background, #1e1e1e);
    }

    .viewer-toolbar {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) 28px;
      align-items: center;
      gap: 8px;
      min-height: 32px;
      padding: 0 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, #2b2b2b);
      background: var(--vsc-tabbar-background, #252526);
      font-size: 11px;
    }

    .viewer-label {
      color: var(--vsc-accent-blue, #4aa3ff);
      font-weight: 600;
    }

    .viewer-path {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, #969696);
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
    }

    .viewer-toolbar button {
      width: 24px;
      height: 24px;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font-size: 17px;
    }

    .viewer-toolbar button:hover {
      background: var(--vsc-list-hover-background, #2a2d2e);
    }

    app-viewer-host-surface {
      display: flex;
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }

    @media (max-width: 700px) {
      .navigation-bar {
        grid-template-columns: auto minmax(0, 1fr) 32px;
      }

      .entry {
        grid-template-columns: 28px minmax(0, 1fr) 72px;
      }

      .size {
        display: none;
      }

      .hint {
        display: none;
      }
    }
  `],
})
export class ReassemblyDirectoryPaneComponent implements OnInit {
  readonly FileType = FileType;

  @ViewChild('addressBar') addressBar?: ReassemblyAddressBarComponent;
  @ViewChild('viewerHost') viewerHost?: ViewerHostSurfaceComponent;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() active = false;
  @Output() activateRequested = new EventEmitter<void>();

  path = '';
  entries: FileInfo[] = [];
  cursorPath: string | null = null;
  selectedPaths = new Set<string>();
  loading = false;
  error: string | null = null;
  lastAction = 'startup';
  clipboard: { operation: 'copy' | 'cut'; paths: string[] } | null = null;

  tabs: ReassemblyTab[] = [];
  activeTabId: string | null = null;
  private tabCounter = 0;

  get activeTab(): ReassemblyTab | null {
    if (!this.activeTabId) return null;
    return this.tabs.find(tab => tab.id === this.activeTabId) ?? null;
  }

  get activeViewerTab(): Extract<ReassemblyTab, { kind: 'viewer' }> | null {
    const tab = this.activeTab;
    return tab?.kind === 'viewer' ? tab : null;
  }

  get mountedViewer(): any {
    return this.viewerHost?.mountedViewer ?? null;
  }

  get isDirectoryTab(): boolean {
    return this.activeTab?.kind !== 'viewer';
  }

  get hasSelection(): boolean {
    return this.selectedPaths.size > 0;
  }

  get sourceCapabilities() {
    return getSourceCapabilities(this.path);
  }

  get canCreate(): boolean {
    return this.isDirectoryTab && !!this.path && this.sourceCapabilities.create;
  }

  get canRename(): boolean {
    return this.isDirectoryTab && this.selectedPaths.size === 1 && this.sourceCapabilities.rename;
  }

  get canDelete(): boolean {
    return this.isDirectoryTab && this.hasSelection && this.sourceCapabilities.delete;
  }

  get canCut(): boolean {
    return this.isDirectoryTab && this.hasSelection && this.sourceCapabilities.read && this.sourceCapabilities.delete;
  }

  get canPaste(): boolean {
    return this.isDirectoryTab && !!this.clipboard?.paths.length && this.sourceCapabilities.create;
  }

  private anchorPath: string | null = null;
  private loadGeneration = 0;
  private readonly navigation = new ReassemblyNavigationController();

  constructor(
    private readonly api: ApiService,
    private readonly directorySource: DirectorySourceService,
    private readonly pathHistory: PathHistoryService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get cursorEntry(): FileInfo | null {
    if (!this.cursorPath) return null;
    return this.entries.find(entry => entry.path === this.cursorPath) ?? null;
  }

  get atRoot(): boolean {
    return !this.path || this.directorySource.isRootPath(this.path);
  }

  get canBack(): boolean {
    return this.navigation.canBack;
  }

  get canForward(): boolean {
    return this.navigation.canForward;
  }

  ngOnInit(): void {
    void this.initialize();
  }

  focusLocation(): void {
    if (!this.isDirectoryTab) return;
    this.addressBar?.beginEdit();
  }

  newTab(): void {
    const path = this.path || '/';
    const tab = this.createDirectoryTab(path);
    this.tabs = [...this.tabs, tab];
    this.activeTabId = tab.id;
    this.lastAction = `new tab: ${tab.title}`;
    this.cdr.markForCheck();
  }

  async activateTab(tabId: string): Promise<void> {
    if (tabId === this.activeTabId) return;
    const tab = this.tabs.find(candidate => candidate.id === tabId);
    if (!tab) return;

    this.activeTabId = tabId;
    this.lastAction = `tab: ${tab.title}`;
    this.cdr.markForCheck();

    if (tab.kind === 'dir') {
      await this.load(tab.path, { history: 'record', syncTab: false });
    }
  }

  async closeActiveTab(): Promise<void> {
    if (!this.activeTabId) return;
    await this.closeTab(this.activeTabId);
  }

  async closeTab(tabId: string): Promise<void> {
    const index = this.tabs.findIndex(tab => tab.id === tabId);
    if (index < 0) return;

    if (this.tabs.length === 1) {
      const only = this.tabs[0];
      if (only.kind === 'dir') {
        const path = this.path || only.path;
        this.tabs = [{ ...only, path, title: this.tabTitle(path) }];
        this.activeTabId = only.id;
      } else {
        const fallback = this.createDirectoryTab(this.path || '/');
        this.tabs = [fallback];
        this.activeTabId = fallback.id;
        await this.load(fallback.path, { history: 'record', syncTab: false });
      }
      this.lastAction = 'kept last tab';
      this.cdr.markForCheck();
      return;
    }

    const wasActive = this.activeTabId === tabId;
    this.tabs = this.tabs.filter(tab => tab.id !== tabId);

    if (!wasActive) {
      this.cdr.markForCheck();
      return;
    }

    const nextIndex = Math.min(index, this.tabs.length - 1);
    const next = this.tabs[nextIndex];
    this.activeTabId = next.id;
    this.lastAction = `close tab -> ${next.title}`;
    this.cdr.markForCheck();

    if (next.kind === 'dir') {
      await this.load(next.path, { history: 'record', syncTab: false });
    }
  }

  async nextTab(): Promise<void> {
    if (this.tabs.length < 2 || !this.activeTabId) return;
    const currentIndex = this.tabs.findIndex(tab => tab.id === this.activeTabId);
    const nextIndex = currentIndex < 0 ? 0 : (currentIndex + 1) % this.tabs.length;
    await this.activateTab(this.tabs[nextIndex].id);
  }

  trackTab(_index: number, tab: ReassemblyTab): string {
    return tab.id;
  }

  navigateTo(path: string): void {
    if (!this.isDirectoryTab) return;
    this.lastAction = `navigate: ${path}`;
    void this.load(path, { history: 'record' });
  }

  async refresh(): Promise<void> {
    if (!this.isDirectoryTab || !this.path) return;
    this.lastAction = 'refresh';
    await this.load(this.path, {
      preserveCursor: true,
      history: 'none',
    });
  }

  async navigateUp(): Promise<void> {
    if (!this.isDirectoryTab || !this.path || this.atRoot) return;
    const parent = this.directorySource.getParentPath(this.path);
    if (!parent || parent === this.path) return;

    this.lastAction = 'parent directory';
    await this.load(parent, { history: 'record' });
  }

  async goBack(): Promise<void> {
    if (!this.isDirectoryTab) return;
    const target = this.navigation.backTarget();
    if (!target) return;

    this.lastAction = 'back';
    const loaded = await this.load(target, { history: 'none' });
    if (loaded) {
      this.navigation.commitBack(target);
      this.cdr.markForCheck();
    }
  }

  async goForward(): Promise<void> {
    if (!this.isDirectoryTab) return;
    const target = this.navigation.forwardTarget();
    if (!target) return;

    this.lastAction = 'forward';
    const loaded = await this.load(target, { history: 'none' });
    if (loaded) {
      this.navigation.commitForward(target);
      this.cdr.markForCheck();
    }
  }

  openEntry(entry: FileInfo): void {
    this.setSingleSelection(entry);

    if (entry.type !== FileType.DIRECTORY) {
      this.openViewer(entry);
      return;
    }

    this.lastAction = `open directory: ${entry.name}`;
    void this.load(entry.path, { history: 'record' });
  }

  selectFromPointer(entry: FileInfo, event: MouseEvent): void {
    this.cursorPath = entry.path;

    if (event.shiftKey && this.anchorPath) {
      this.selectRange(this.anchorPath, entry.path);
    } else if (event.ctrlKey || event.metaKey) {
      const next = new Set(this.selectedPaths);
      if (next.has(entry.path)) {
        next.delete(entry.path);
      } else {
        next.add(entry.path);
      }
      this.selectedPaths = next;
      this.anchorPath = entry.path;
    } else {
      this.setSingleSelection(entry);
    }

    this.lastAction = `select: ${entry.name}`;
    this.cdr.markForCheck();
  }

  openViewer(entry: FileInfo): void {
    const existing = this.tabs.find(tab =>
      tab.kind === 'viewer' && this.samePath(tab.targetPath, entry.path)
    );
    if (existing) {
      this.activeTabId = existing.id;
      this.lastAction = `viewer: ${entry.name}`;
      this.cdr.markForCheck();
      return;
    }

    const fileList: ViewerFileListItem[] = this.entries
      .filter(candidate => candidate.type !== FileType.DIRECTORY)
      .map(candidate => ({ path: candidate.path, name: candidate.name }));

    const tab: ReassemblyTab = {
      id: `${this.paneId}-tab-${++this.tabCounter}`,
      kind: 'viewer',
      path: entry.path,
      title: entry.name,
      targetPath: entry.path,
      fileList,
    };

    this.tabs = [...this.tabs, tab];
    this.activeTabId = tab.id;
    this.lastAction = `viewer: ${entry.name}`;
    this.cdr.markForCheck();
  }

  selectAll(): void {
    if (!this.isDirectoryTab) return;
    this.selectedPaths = new Set(this.entries.map(entry => entry.path));
    this.lastAction = 'select all';
    this.cdr.markForCheck();
  }

  clearSelection(): void {
    this.selectedPaths = new Set();
    this.anchorPath = null;
    this.lastAction = 'clear selection';
    this.cdr.markForCheck();
  }

  copySelected(): void {
    if (!this.hasSelection) return;
    this.clipboard = {
      operation: 'copy',
      paths: Array.from(this.selectedPaths),
    };
    this.lastAction = `copy ${this.clipboard.paths.length}`;
    this.cdr.markForCheck();
  }

  cutSelected(): void {
    if (!this.canCut) return;
    this.clipboard = {
      operation: 'cut',
      paths: Array.from(this.selectedPaths),
    };
    this.lastAction = `cut ${this.clipboard.paths.length}`;
    this.cdr.markForCheck();
  }

  async newFolder(): Promise<void> {
    if (!this.canCreate) return;

    const name = await this.confirmDialog.prompt({
      title: 'New Folder',
      message: 'Enter a name for the new folder.',
      confirmText: 'Create',
      defaultValue: 'New Folder',
      historyKey: 'reassembly.newFolder',
    });
    const cleanName = this.cleanLeafName(name);
    if (!cleanName) return;

    try {
      const destination = await this.api.joinPaths([this.path, cleanName]);
      await this.api.createDirectory(destination, false);
      this.lastAction = `new folder: ${cleanName}`;
      await this.refresh();
      this.selectPath(destination);
    } catch (error) {
      await this.showOperationError('Create Folder', error);
    }
  }

  async renameSelected(): Promise<void> {
    if (!this.canRename) return;

    const entry = this.selectedEntries()[0];
    if (!entry) return;

    const nextName = await this.confirmDialog.prompt({
      title: 'Rename',
      message: `Rename "${entry.name}" to:`,
      confirmText: 'Rename',
      defaultValue: entry.name,
      historyKey: 'reassembly.rename',
    });
    const cleanName = this.cleanLeafName(nextName);
    if (!cleanName || cleanName === entry.name) return;

    try {
      const destination = await this.api.joinPaths([this.path, cleanName]);
      const result = await this.api.batchMove([{
        source: entry.path,
        destination,
        exactDestination: true,
      }], true);

      if (result.failed > 0) {
        throw new Error(`Rename failed (${result.failed} item${result.failed === 1 ? '' : 's'}).`);
      }

      this.lastAction = `rename: ${entry.name} -> ${cleanName}`;
      await this.refresh();
      this.selectPath(destination);
    } catch (error) {
      await this.showOperationError('Rename', error);
    }
  }

  async deleteSelected(): Promise<void> {
    if (!this.canDelete) return;

    const entries = this.selectedEntries();
    if (entries.length === 0) return;

    const useTrash = this.sourceCapabilities.trash;
    const confirmed = await this.confirmDialog.confirm({
      title: useTrash ? 'Move to Trash' : 'Delete',
      message: entries.length === 1
        ? `${useTrash ? 'Move' : 'Delete'} "${entries[0].name}"${useTrash ? ' to Trash' : ' permanently'}?`
        : `${useTrash ? 'Move' : 'Delete'} ${entries.length} selected items${useTrash ? ' to Trash' : ' permanently'}?`,
      confirmText: useTrash ? 'Move to Trash' : 'Delete',
      danger: true,
    });
    if (!confirmed) return;

    try {
      const result = await this.api.batchDelete(
        entries.map(entry => entry.path),
        true,
        useTrash,
      );

      this.lastAction = `delete: ${result.succeeded}/${result.total}`;
      await this.refresh();

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: 'Delete Completed with Errors',
          message: `${result.failed} item${result.failed === 1 ? '' : 's'} could not be deleted.`,
        });
      }
    } catch (error) {
      await this.showOperationError('Delete', error);
    }
  }

  async paste(): Promise<void> {
    const clipboard = this.clipboard;
    if (!clipboard || !this.canPaste) return;

    try {
      const items = await Promise.all(clipboard.paths.map(async source => ({
        source,
        destination: await this.api.joinPaths([this.path, this.leafName(source)]),
      })));

      const result = clipboard.operation === 'copy'
        ? await this.api.batchCopy(items, true)
        : await this.api.batchMove(items, true);

      this.lastAction = `${clipboard.operation === 'copy' ? 'paste copy' : 'paste move'}: ${result.succeeded}/${result.total}`;

      if (clipboard.operation === 'cut' && result.failed === 0) {
        this.clipboard = null;
      }

      await this.refresh();

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: 'Paste Completed with Errors',
          message: `${result.failed} item${result.failed === 1 ? '' : 's'} could not be pasted.`,
        });
      }
    } catch (error) {
      await this.showOperationError('Paste', error);
    }
  }

  private selectedEntries(): FileInfo[] {
    if (!this.selectedPaths.size) return [];
    return this.entries.filter(entry => this.selectedPaths.has(entry.path));
  }

  private selectPath(path: string): void {
    const entry = this.entries.find(candidate => candidate.path === path);
    if (!entry) return;
    this.setSingleSelection(entry);
    this.ensureCursorVisible();
    this.cdr.markForCheck();
  }

  private cleanLeafName(value: string | null): string {
    const clean = (value ?? '').trim();
    if (!clean || clean === '.' || clean === '..' || /[\/\\]/.test(clean)) {
      return '';
    }
    return clean;
  }

  private leafName(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/').replace(/\/+$/, '');
    const index = normalized.lastIndexOf('/');
    return index >= 0 ? normalized.slice(index + 1) : normalized;
  }

  private async showOperationError(title: string, error: unknown): Promise<void> {
    const message = error instanceof Error ? error.message : String(error);
    this.lastAction = `${title.toLowerCase()} failed`;
    this.cdr.markForCheck();
    await this.confirmDialog.alert({
      title: `${title} Error`,
      message,
    });
  }

  trackEntry(_index: number, entry: FileInfo): string {
    return entry.path;
  }

  formatSize(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return bytes === 0 ? '0 B' : '';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let value = bytes;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
      value /= 1024;
      unit += 1;
    }
    return unit === 0
      ? `${value} ${units[unit]}`
      : `${value.toFixed(value >= 10 ? 1 : 2)} ${units[unit]}`;
  }

  private async initialize(): Promise<void> {
    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      const drives = await this.api.listDrives();
      const initialPath = drives[0]?.path || '/';
      const loaded = await this.load(initialPath, { history: 'record' });
      if (loaded && this.tabs.length === 0) {
        const tab = this.createDirectoryTab(this.path);
        this.tabs = [tab];
        this.activeTabId = tab.id;
        this.cdr.markForCheck();
      }
    } catch {
      try {
        const loaded = await this.load('/', { history: 'record' });
        if (loaded && this.tabs.length === 0) {
          const tab = this.createDirectoryTab(this.path);
          this.tabs = [tab];
          this.activeTabId = tab.id;
          this.cdr.markForCheck();
        }
      } catch {
        // load() records the visible error state.
      }
    }
  }

  private async load(
    requestedPath: string,
    options: {
      preserveCursor?: boolean;
      history?: 'record' | 'none';
      syncTab?: boolean;
    } = {},
  ): Promise<boolean> {
    const generation = ++this.loadGeneration;
    const previousCursor = options.preserveCursor ? this.cursorPath : null;

    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      const listing = await this.directorySource.listDirectory(requestedPath, false);
      if (generation !== this.loadGeneration) return false;

      this.path = listing.path || requestedPath;
      this.entries = [...(listing.entries || [])].sort((left, right) => {
        const leftDirectory = left.type === FileType.DIRECTORY;
        const rightDirectory = right.type === FileType.DIRECTORY;
        if (leftDirectory !== rightDirectory) return leftDirectory ? -1 : 1;
        return left.name.localeCompare(right.name, undefined, {
          numeric: true,
          sensitivity: 'base',
        });
      });

      const preserved = previousCursor
        ? this.entries.find(entry => entry.path === previousCursor)
        : null;
      const nextCursor = preserved ?? this.entries[0] ?? null;

      this.cursorPath = nextCursor?.path ?? null;
      this.selectedPaths = nextCursor ? new Set([nextCursor.path]) : new Set();
      this.anchorPath = nextCursor?.path ?? null;
      this.error = null;

      if ((options.history ?? 'record') === 'record') {
        this.navigation.record(this.path);
      }

      if (options.syncTab !== false) {
        this.syncActiveTab(this.path);
      }

      this.pathHistory.addLocation(this.path, 'reassembly');

      return true;
    } catch (error) {
      if (generation !== this.loadGeneration) return false;

      this.entries = [];
      this.cursorPath = null;
      this.selectedPaths = new Set();
      this.anchorPath = null;
      this.error = error instanceof Error ? error.message : String(error);
      return false;
    } finally {
      if (generation === this.loadGeneration) {
        this.loading = false;
        this.cdr.markForCheck();
      }
    }
  }

  private createDirectoryTab(path: string): ReassemblyTab {
    const normalized = path || '/';
    return {
      id: `${this.paneId}-tab-${++this.tabCounter}`,
      kind: 'dir',
      path: normalized,
      title: this.tabTitle(normalized),
    };
  }

  private syncActiveTab(path: string): void {
    if (!this.activeTabId) {
      const tab = this.createDirectoryTab(path);
      this.tabs = [tab];
      this.activeTabId = tab.id;
      return;
    }

    const index = this.tabs.findIndex(tab => tab.id === this.activeTabId);
    if (index < 0) return;

    const current = this.tabs[index];
    if (current.kind !== 'dir') return;

    const updated: ReassemblyTab = {
      ...current,
      path,
      title: this.tabTitle(path),
    };

    this.tabs = this.tabs.map((tab, i) => i === index ? updated : tab);
  }

  private samePath(left: string, right: string): boolean {
    return left.trim().replace(/\\/g, '/').toLowerCase() ===
      right.trim().replace(/\\/g, '/').toLowerCase();
  }

  private tabTitle(path: string): string {
    const normalized = (path || '/').replace(/\\/g, '/').replace(/\/+$/, '');
    if (!normalized) return '/';

    const schemeRoot = /^([a-z][a-z0-9+.-]*):\/\/([^/]+)?$/i.exec(normalized);
    if (schemeRoot) {
      return schemeRoot[2] || schemeRoot[1];
    }

    const parts = normalized.split('/').filter(Boolean);
    return parts[parts.length - 1] || normalized;
  }

  moveCursor(delta: -1 | 1): void {
    if (!this.isDirectoryTab || this.entries.length === 0) return;

    const currentIndex = this.cursorPath
      ? this.entries.findIndex(entry => entry.path === this.cursorPath)
      : -1;

    const nextIndex = currentIndex < 0
      ? 0
      : Math.max(0, Math.min(this.entries.length - 1, currentIndex + delta));

    const entry = this.entries[nextIndex];
    this.setSingleSelection(entry);
    this.lastAction = delta < 0 ? 'cursor up' : 'cursor down';
    this.ensureCursorVisible();
    this.cdr.markForCheck();
  }

  private setSingleSelection(entry: FileInfo): void {
    this.cursorPath = entry.path;
    this.selectedPaths = new Set([entry.path]);
    this.anchorPath = entry.path;
  }

  private selectRange(anchorPath: string, targetPath: string): void {
    const start = this.entries.findIndex(entry => entry.path === anchorPath);
    const end = this.entries.findIndex(entry => entry.path === targetPath);

    if (start < 0 || end < 0) {
      const target = this.entries.find(entry => entry.path === targetPath);
      if (target) this.setSingleSelection(target);
      return;
    }

    const from = Math.min(start, end);
    const to = Math.max(start, end);
    this.selectedPaths = new Set(
      this.entries.slice(from, to + 1).map(entry => entry.path),
    );
  }

  private ensureCursorVisible(): void {
    if (!this.cursorPath || typeof document === 'undefined') return;

    const escaped = typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
      ? CSS.escape(this.cursorPath)
      : this.cursorPath.replace(/["\\]/g, '\\$&');

    queueMicrotask(() => {
      document
        .querySelector<HTMLElement>(`[data-file-entry="${escaped}"]`)
        ?.scrollIntoView({ block: 'nearest' });
    });
  }
}
