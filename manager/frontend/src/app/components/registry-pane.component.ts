import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  DoCheck,
  ElementRef,
  EventEmitter,
  Input,
  Output,
  ViewChild,
} from '@angular/core';

import { WorkspaceService } from '../services/workspace.service';
import { PanelSourceRegistryService } from '../services/panel-source-registry.service';
import { PluginLifecycleCoordinatorService } from '../services/plugin-lifecycle-coordinator.service';
import { getSourceCapabilities, type SourceCapabilities } from '../services/source-capabilities';
import type { PaneComparisonStatus } from '../services/pane-comparison.service';
import { FileInfo, FileType } from '../shared/protocol';
import type { PresentedCommand } from './presented-command';
import type { PaneComponent } from './pane-component.contract';
import type { PanePresentation } from './directory-pane.component';

type RegistryViewMode = 'list' | 'details';
type RegistryEntryKind = 'parent' | 'hive' | 'key' | 'value';

const ROOT = 'registry://';
const PARENT_ENTRY_PATH = '__parent__';

@Component({
  selector: 'app-registry-pane',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      #paneRoot
      class="registry-pane"
      tabindex="-1"
      role="region"
      [class.active-pane]="active"
      [attr.data-pane-active]="active ? 'true' : null"
      [attr.aria-label]="paneId === 'left' ? 'Left registry pane' : 'Right registry pane'"
      [attr.aria-busy]="loading"
      (focusin)="activateRequested.emit()"
      (mousedown)="activateRequested.emit()">

      <header class="pane-header">
        <div class="nav-row">
          <button type="button" class="nav-button" (click)="goBack()" [disabled]="!canGoBack()"><</button>
          <button type="button" class="nav-button" (click)="goForward()" [disabled]="!canGoForward()">></button>
          <button type="button" class="nav-button" (click)="navigateUp()" [disabled]="isRootView">Up</button>
          <div class="location-bar">
            <span class="location-label">{{ path || rootPath }}</span>
          </div>
          <button type="button" class="nav-button" (click)="refresh()">Refresh</button>
        </div>

        <div class="status-row">
          <span>{{ visibleEntryCount }} items</span>
          <span *ngIf="selectedPaths.size">{{ selectedPaths.size }} selected</span>
        </div>

        <div class="toolbar-row">
          <button
            type="button"
            class="mode-button"
            [class.active]="viewMode === 'list'"
            (click)="setViewMode('list')">
            List
          </button>
          <button
            type="button"
            class="mode-button"
            [class.active]="viewMode === 'details'"
            (click)="setViewMode('details')">
            Details
          </button>
        </div>
      </header>

      <ng-container *ngIf="!error; else errorState">
        <div class="registry-surface-header" *ngIf="isRootView; else keyHeader">
          <div class="registry-surface-title">Registry Hives</div>
          <div class="registry-surface-subtitle">
            System root hives available in this session
          </div>
        </div>

        <ng-template #keyHeader>
          <div class="registry-surface-header">
            <div class="registry-surface-title">Registry Key</div>
            <div class="registry-surface-subtitle">
              {{ keyEntries.length }} subkeys and {{ valueEntries.length }} values
            </div>
          </div>
        </ng-template>

        <div
          class="registry-details-header"
          *ngIf="viewMode === 'details' && !isRootView">
          <span></span>
          <span>Name</span>
          <span>Kind</span>
          <span>Type</span>
          <span>Data</span>
        </div>

        <div
          #content
          class="registry-list"
          role="listbox"
          [class.view-list]="viewMode === 'list'"
          [class.view-details]="viewMode === 'details'"
          (scroll)="onScroll()"
          (click)="onBackgroundClick($event)"
          (contextmenu)="backgroundContextMenu.emit($event)">

          <ng-container *ngIf="isRootView; else groupedKeyView">
            <div class="registry-section registry-section-root">
              <div
                *ngFor="let entry of entries; trackBy: trackEntry"
                class="registry-entry registry-entry-root"
                [class.selected]="selectedPaths.has(entry.path)"
                [class.cursor]="entry.path === cursorPath"
                [attr.data-file-entry]="entry.path"
                (click)="selectEntry(entry, $event)"
                (dblclick)="openEntry(entry)"
                (contextmenu)="emitEntryContextMenu(entry, $event)">
                <span class="icon">{{ icon(entry) }}</span>
                <span class="name-cell">
                  <span class="name-line">
                    <span class="name">{{ entry.name }}</span>
                    <span class="badge">{{ badgeLabel(entry) }}</span>
                  </span>
                  <span class="meta">{{ metaLabel(entry) }}</span>
                </span>
                <span class="data root-data">{{ dataLabel(entry) }}</span>
              </div>
            </div>
          </ng-container>

          <ng-template #groupedKeyView>
            <div class="registry-value-inspector" *ngIf="focusedValueEntry as entry">
              <div class="registry-value-inspector-header">
                <span class="registry-value-inspector-title">Focused Value</span>
                <span class="registry-value-inspector-type">{{ entry.registryValueType?.trim() || 'REG_SZ' }}</span>
              </div>
              <div class="registry-value-inspector-name">{{ entry.name }}</div>
              <div class="registry-value-inspector-meta">{{ metaLabel(entry) }}</div>
              <div class="registry-value-inspector-data">{{ dataLabel(entry) }}</div>
            </div>

            <div class="registry-section" *ngIf="parentEntry as entry">
              <div class="registry-section-label">Navigation</div>
              <ng-container [ngTemplateOutlet]="entryRow" [ngTemplateOutletContext]="{ $implicit: entry }"></ng-container>
            </div>

            <div class="registry-section" *ngIf="keyEntries.length">
              <div class="registry-section-label">Subkeys</div>
              <ng-container *ngFor="let entry of keyEntries; trackBy: trackEntry">
                <ng-container [ngTemplateOutlet]="entryRow" [ngTemplateOutletContext]="{ $implicit: entry }"></ng-container>
              </ng-container>
            </div>

            <div class="registry-section" *ngIf="valueEntries.length">
              <div class="registry-section-label">Values</div>
              <ng-container *ngFor="let entry of valueEntries; trackBy: trackEntry">
                <ng-container [ngTemplateOutlet]="entryRow" [ngTemplateOutletContext]="{ $implicit: entry }"></ng-container>
              </ng-container>
            </div>
          </ng-template>

          <ng-template #entryRow let-entry>
            <div
              class="registry-entry"
              [class.selected]="selectedPaths.has(entry.path)"
              [class.cursor]="entry.path === cursorPath"
              [attr.data-file-entry]="entry.path"
              (click)="selectEntry(entry, $event)"
              (dblclick)="openEntry(entry)"
              (contextmenu)="emitEntryContextMenu(entry, $event)">
              <span class="icon">{{ icon(entry) }}</span>
              <span class="name-cell">
                <span class="name-line">
                  <span class="name">{{ entry.name }}</span>
                  <span class="badge">{{ badgeLabel(entry) }}</span>
                </span>
                <span class="meta">{{ metaLabel(entry) }}</span>
              </span>
              <span class="kind">{{ kindLabel(entry) }}</span>
              <span class="type">{{ typeLabel(entry) }}</span>
              <span class="data">{{ dataLabel(entry) }}</span>
            </div>
          </ng-template>

          <div class="empty" *ngIf="!loading && visibleEntryCount === 0">
            <strong>This registry key is empty.</strong>
          </div>
        </div>
      </ng-container>

      <ng-template #errorState>
        <div class="error-state">
          <strong>Directory could not be loaded.</strong>
          <span>{{ error }}</span>
          <div class="error-actions">
            <button type="button" class="nav-button" (click)="refresh()">Retry</button>
            <button type="button" class="nav-button" (click)="navigateUp()" [disabled]="isRootView">Parent folder</button>
          </div>
        </div>
      </ng-template>
    </section>
  `,
  styles: [`
    :host {
      display: flex;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }

    .registry-pane {
      display: flex;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      flex-direction: column;
      overflow: hidden;
      background: var(--vsc-editor-background);
      color: var(--vsc-foreground);
    }

    .pane-header {
      display: grid;
      flex: 0 0 auto;
      gap: 6px;
      padding: 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-editor-background);
    }

    .nav-row,
    .status-row,
    .toolbar-row {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
    }

    .status-row {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 12px;
    }

    .location-bar {
      display: flex;
      align-items: center;
      flex: 1 1 auto;
      min-width: 0;
      height: 32px;
      padding: 0 10px;
      border: 1px solid var(--vsc-border, #d0d7de);
      border-radius: 8px;
      background: var(--vsc-input-background, #fff);
    }

    .location-label {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font: 600 14px/1 Consolas, monospace;
    }

    .nav-button,
    .mode-button {
      border: 1px solid var(--vsc-border, #d0d7de);
      border-radius: 6px;
      background: var(--vsc-button-secondary-background, #f6f8fa);
      color: var(--vsc-button-secondary-foreground, var(--vsc-foreground));
      padding: 6px 10px;
      font: inherit;
    }

    .mode-button.active {
      background: var(--vsc-button-background, #0969da);
      color: var(--vsc-button-foreground, #fff);
    }

    .nav-button:disabled {
      opacity: .5;
    }

    .registry-surface-header {
      display: grid;
      gap: 2px;
      padding: 8px 10px 7px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background:
        linear-gradient(
          180deg,
          color-mix(in srgb, var(--vsc-tabbar-background, var(--vsc-panel-background)) 90%, white 10%),
          var(--vsc-tabbar-background, var(--vsc-panel-background))
        );
    }

    .registry-surface-title {
      font-size: 12px;
      font-weight: 600;
      color: var(--vsc-foreground);
    }

    .registry-surface-subtitle,
    .meta,
    .kind,
    .type,
    .data,
    .registry-section-label {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .registry-surface-subtitle,
    .meta,
    .kind,
    .type,
    .data {
      font-size: 10px;
    }

    .registry-details-header {
      display: grid;
      grid-template-columns: 28px minmax(180px, 1fr) 96px 120px minmax(220px, 1fr);
      align-items: center;
      min-height: 25px;
      padding: 0 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
      font-size: 10px;
      column-gap: 6px;
    }

    .registry-list {
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: auto;
      outline: none;
      scrollbar-width: thin;
    }

    .registry-section {
      display: grid;
      gap: 2px;
      padding: 6px 0 2px;
    }

    .registry-section-root {
      padding: 10px 10px 8px;
      gap: 8px;
    }

    .registry-section-label {
      padding: 0 10px 2px;
      font-size: 10px;
      font-weight: 700;
      letter-spacing: .08em;
      text-transform: uppercase;
    }

    .registry-entry {
      box-sizing: border-box;
      display: grid;
      grid-template-columns: 22px minmax(0, 1fr) 96px 120px minmax(220px, 1fr);
      align-items: center;
      gap: 6px;
      min-height: 34px;
      padding: 4px 8px;
      user-select: none;
      cursor: default;
    }

    .registry-entry-root {
      grid-template-columns: 22px minmax(0, 1fr) minmax(120px, 1fr);
      min-height: 52px;
      padding: 10px 12px;
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-foreground-dim)) 65%, transparent);
      border-radius: 10px;
      background:
        linear-gradient(
          135deg,
          color-mix(in srgb, var(--vsc-editor-background, #fff) 90%, var(--vsc-panel-background, #fff) 10%),
          color-mix(in srgb, var(--vsc-editor-background, #fff) 96%, var(--vsc-accent-blue, #06c) 4%)
        );
    }

    .registry-entry:hover {
      background: var(--vsc-list-hover-background);
    }

    .registry-entry.selected {
      background: var(--vsc-list-active-background);
      color: var(--vsc-list-active-foreground, var(--vsc-foreground));
    }

    .registry-entry.cursor {
      box-shadow: inset 0 0 0 1px var(--vsc-focus-border, var(--vsc-accent-blue));
    }

    .icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 18px;
      height: 18px;
      border-radius: 999px;
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-description-foreground)) 85%, transparent);
      font: 600 10px/1 var(--vsc-font-family-mono, ui-monospace, monospace);
    }

    .name-cell {
      display: grid;
      gap: 2px;
      min-width: 0;
    }

    .name-line {
      display: inline-flex;
      gap: 6px;
      align-items: center;
      min-width: 0;
    }

    .name,
    .meta,
    .badge,
    .kind,
    .type,
    .data {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .badge {
      flex: 0 0 auto;
      max-width: 88px;
      padding: 1px 6px;
      border-radius: 999px;
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 12%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-border));
      font-size: 9px;
      font-weight: 600;
      text-transform: uppercase;
    }

    .root-data {
      font-size: 10px;
    }

    .registry-value-inspector {
      display: grid;
      gap: 4px;
      margin: 10px 10px 4px;
      padding: 10px 12px;
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 45%, transparent);
      border-radius: 10px;
      background:
        linear-gradient(
          135deg,
          color-mix(in srgb, var(--vsc-editor-background, #fff) 94%, var(--vsc-accent-blue, #06c) 6%),
          color-mix(in srgb, var(--vsc-editor-background, #fff) 98%, var(--vsc-panel-background, #fff) 2%)
        );
    }

    .registry-value-inspector-header {
      display: flex;
      gap: 8px;
      align-items: center;
    }

    .registry-value-inspector-title,
    .registry-section-label {
      font-size: 10px;
      font-weight: 700;
      letter-spacing: .08em;
      text-transform: uppercase;
    }

    .registry-value-inspector-type {
      padding: 1px 6px;
      border-radius: 999px;
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 14%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-border));
      font-size: 9px;
      font-weight: 600;
    }

    .registry-value-inspector-name {
      font-size: 12px;
      font-weight: 600;
    }

    .registry-value-inspector-data {
      padding: 8px 10px;
      border-radius: 8px;
      background: color-mix(in srgb, var(--vsc-editor-background, #fff) 82%, black 18%);
      font: 12px/1.4 var(--vsc-font-family-mono, ui-monospace, monospace);
      white-space: pre-wrap;
      word-break: break-word;
    }

    .empty,
    .error-state {
      display: flex;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 6px;
      padding: 20px;
      text-align: center;
    }

    .error-actions {
      display: flex;
      gap: 8px;
    }
  `],
})
export class RegistryPaneComponent implements DoCheck, PaneComponent {
  @ViewChild('paneRoot') private paneRoot?: ElementRef<HTMLElement>;

  readonly rootPath = ROOT;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() active = false;
  @Input() showHidden = false;
  @Input() currentPathPinned = false;
  @Input() pluginToolbarActions: readonly PresentedCommand[] = [];
  @Input() comparisonStatuses: ReadonlyMap<string, PaneComparisonStatus> =
    new Map<string, PaneComparisonStatus>();

  @Output() readonly activateRequested = new EventEmitter<void>();
  @Output() readonly navigationTrailRequested = new EventEmitter<{
    direction: 'back' | 'forward';
    event: MouseEvent;
    entries: readonly string[];
  }>();
  @Output() readonly remoteRecoveryRequested = new EventEmitter<void>();
  @Output() readonly togglePinRequested = new EventEmitter<void>();
  @Output() readonly locationChange = new EventEmitter<string>();
  @Output() readonly cursorChange = new EventEmitter<FileInfo | null>();
  @Output() readonly presentationChange = new EventEmitter<PanePresentation>();
  @Output() readonly viewportChange = new EventEmitter<{ topName: string | null; ratio: number }>();
  @Output() readonly entryContextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() readonly backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() readonly internalDrop = new EventEmitter<{
    event: DragEvent;
    targetPath: string;
    targetEntry: FileInfo | null;
  }>();

  path = '';
  entries: FileInfo[] = [];
  readonly tabs: never[] = [];
  readonly activeViewerTab = null;
  readonly mountedViewer = null;
  readonly viewerArchiveContext = null;
  readonly currentArchiveFrame = null;
  readonly activeSurfaceId = 'registry';
  readonly pinnableSurfaceId = 'registry';
  viewMode: RegistryViewMode = 'list';
  selectedPaths: ReadonlySet<string> = new Set<string>();
  cursorPath: string | null = null;
  loading = false;
  error = '';

  private backTrail: string[] = [];
  private forwardTrail: string[] = [];
  private workspacePathSnapshot = '';
  private workspaceTabSnapshot = '';
  private loadToken = 0;

  constructor(
    private readonly workspaceService: WorkspaceService,
    private readonly panelSources: PanelSourceRegistryService,
    private readonly pluginLifecycle: PluginLifecycleCoordinatorService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngDoCheck(): void {
    const activeTab = this.workspaceService.getActiveTab(this.paneId);
    const nextPath = activeTab?.path ?? '';
    const nextTabId = activeTab?.id ?? '';
    if (nextPath === this.workspacePathSnapshot && nextTabId === this.workspaceTabSnapshot) return;

    this.workspacePathSnapshot = nextPath;
    this.workspaceTabSnapshot = nextTabId;

    if (!this.isRegistryPath(nextPath)) {
      this.reset();
      return;
    }

    if (nextPath !== this.path) {
      void this.loadRegistryPath(nextPath, { preserveSelection: false, history: 'none' });
    }
  }

  get activeTab() {
    return this.workspaceService.getActiveTab(this.paneId);
  }

  get visibleEntries(): readonly FileInfo[] {
    return this.entries;
  }

  get visibleEntryCount(): number {
    return this.entries.length;
  }

  get cursorEntry(): FileInfo | null {
    return this.cursorPath
      ? this.entries.find(entry => entry.path === this.cursorPath) ?? null
      : null;
  }

  get selectedEntries(): FileInfo[] {
    return this.entries.filter(entry => this.selectedPaths.has(entry.path));
  }

  get sourceCapabilities(): SourceCapabilities {
    return this.activePanelSource?.capabilities ?? getSourceCapabilities(this.path);
  }

  get activePanelSource(): any {
    return this.panelSources.resolve({ path: this.path, entries: this.entries });
  }

  get pinnableLocation(): string {
    return this.path;
  }

  get pinnableTitle(): string {
    return this.path;
  }

  get hasSelection(): boolean {
    return this.selectedPaths.size > 0;
  }

  get canCreate(): boolean {
    return !!this.path && this.sourceCapabilities.create;
  }

  get canRename(): boolean {
    return this.selectedPaths.size === 1 && this.sourceCapabilities.rename;
  }

  get canDelete(): boolean {
    return this.hasSelection && this.sourceCapabilities.delete;
  }

  get canCut(): boolean {
    return false;
  }

  get canPaste(): boolean {
    return false;
  }

  get isInArchive(): boolean {
    return false;
  }

  get isDirectoryTab(): boolean {
    return true;
  }

  get lastAction(): string {
    return `registry: ${this.path}`;
  }

  set lastAction(_value: string) {}

  get isRootView(): boolean {
    return this.normalizePath(this.path) === ROOT;
  }

  get parentEntry(): FileInfo | null {
    return this.entries.find(entry => this.entryKind(entry) === 'parent') ?? null;
  }

  get keyEntries(): readonly FileInfo[] {
    return this.entries.filter(entry => this.entryKind(entry) === 'key');
  }

  get valueEntries(): readonly FileInfo[] {
    return this.entries.filter(entry => this.entryKind(entry) === 'value');
  }

  get focusedValueEntry(): FileInfo | null {
    const byCursor = this.cursorPath
      ? this.entries.find(entry =>
          entry.path === this.cursorPath && this.entryKind(entry) === 'value',
        ) ?? null
      : null;
    if (byCursor) return byCursor;
    return this.entries.find(entry =>
      this.selectedPaths.has(entry.path) && this.entryKind(entry) === 'value',
    ) ?? null;
  }

  async refresh(): Promise<void> {
    return this.loadRegistryPath(this.path, { preserveSelection: true, history: 'none' });
  }

  focusContent(): void {
    this.paneRoot?.nativeElement.focus({ preventScroll: true });
  }

  focusLocation(): void {}
  focusFilter(): void {}

  canGoBack(): boolean {
    return this.backTrail.length > 0;
  }

  canGoForward(): boolean {
    return this.forwardTrail.length > 0;
  }

  goBack(): void {
    const target = this.backTrail.pop();
    if (!target) return;
    if (this.path) this.forwardTrail.push(this.path);
    void this.loadRegistryPath(target, { preserveSelection: false, history: 'none' });
  }

  goForward(): void {
    const target = this.forwardTrail.pop();
    if (!target) return;
    if (this.path) this.backTrail.push(this.path);
    void this.loadRegistryPath(target, { preserveSelection: false, history: 'none' });
  }

  navigateUp(): void {
    const parent = this.panelSources.getParentPath(this.path, this.entries);
    if (parent && parent !== this.path) {
      void this.loadRegistryPath(parent, { preserveSelection: false, history: 'record' });
    }
  }

  navigateTo(path: string): Promise<void> {
    if (!this.isRegistryPath(path)) return Promise.resolve();
    return this.loadRegistryPath(path, { preserveSelection: false, history: 'record' });
  }

  navigateLinkedLocation(path: string): Promise<boolean> {
    return this.navigateTo(path).then(() => true);
  }

  canNavigateToLocation(path: string): boolean {
    return this.isRegistryPath(path);
  }

  currentLocationAvailable(): Promise<boolean> {
    return Promise.resolve(!!this.path);
  }

  jumpNavigationHistory(path: string): Promise<boolean> {
    return this.navigateTo(path).then(() => true);
  }

  panePresentation(): PanePresentation {
    return {
      viewMode: this.viewMode,
      sortField: 'name',
      sortDirection: 'asc',
      directoriesFirst: true,
      filterText: '',
      filterVisible: false,
    };
  }

  viewportSnapshot(): { topName: string | null; ratio: number } {
    return {
      topName: this.entries[0]?.name ?? null,
      ratio: 0,
    };
  }

  syncViewportToPeer(_snapshot: { topName: string | null; ratio: number }): void {}

  applyMirroredPresentation(presentation: PanePresentation): void {
    if (presentation.viewMode === 'list' || presentation.viewMode === 'details') {
      this.viewMode = presentation.viewMode;
      this.presentationChange.emit(this.panePresentation());
      this.cdr.markForCheck();
    }
  }

  setViewMode(viewMode: RegistryViewMode): void {
    if (this.viewMode === viewMode) return;
    this.viewMode = viewMode;
    this.presentationChange.emit(this.panePresentation());
    this.cdr.markForCheck();
  }

  moveCursor(delta: number, extend = false): void {
    const entries = this.selectableEntries();
    if (entries.length === 0) return;
    const currentIndex = Math.max(0, entries.findIndex(entry => entry.path === this.cursorPath));
    const nextIndex = Math.max(0, Math.min(entries.length - 1, currentIndex + delta));
    this.cursorPath = entries[nextIndex]?.path ?? this.cursorPath;
    if (!extend && this.cursorPath) {
      this.selectedPaths = new Set([this.cursorPath]);
    }
    this.emitCursor();
  }

  moveCursorHome(extend = false): void { this.moveCursor(-Number.MAX_SAFE_INTEGER, extend); }
  moveCursorEnd(extend = false): void { this.moveCursor(Number.MAX_SAFE_INTEGER, extend); }
  moveCursorPage(direction: -1 | 1, extend = false): void { this.moveCursor(direction * 10, extend); }

  openEntry(entry: FileInfo): Promise<void> {
    if (entry.path === PARENT_ENTRY_PATH) {
      this.navigateUp();
      return Promise.resolve();
    }
    if (entry.type === FileType.DIRECTORY) {
      return this.loadRegistryPath(entry.path, { preserveSelection: false, history: 'record' });
    }
    this.cursorPath = entry.path;
    this.selectedPaths = new Set([entry.path]);
    this.emitCursor();
    return Promise.resolve();
  }

  openViewer(_entry: FileInfo, _viewerId?: string | null, _autoFollow?: boolean): any {
    return null;
  }

  openSelectedWithViewer(_viewerId: string): Promise<void> { return Promise.resolve(); }
  openQuickView(_entry: FileInfo): void {}
  openActiveViewerExternally(): Promise<void> { return Promise.resolve(); }
  returnToViewerSource(): Promise<void> { return Promise.resolve(); }
  closeActiveTab(): void {}
  newTab(): void {}
  nextTab(): void {}
  previousTab(): void {}
  jumpToTab(_index: number): void {}
  closeOtherTabs(): void {}
  closeTabsToRight(): void {}
  toggleActiveTabPin(): void {}
  exportActiveTabId(): string | null { return this.activeTab?.id ?? null; }
  exportTabCount(): number { return this.workspaceService.getTabs(this.paneId).length; }
  restoreWorkspaceState(): Promise<void> { return Promise.resolve(); }
  reloadWorkspaceTabs(_preferredTabId?: string): Promise<void> { return Promise.resolve(); }

  selectAll(): void {
    this.selectedPaths = new Set(this.selectableEntries().map(entry => entry.path));
    if (!this.cursorPath) this.cursorPath = this.selectableEntries()[0]?.path ?? null;
    this.emitCursor();
  }

  clearSelection(): void {
    this.selectedPaths = new Set<string>();
    this.cdr.markForCheck();
  }

  toggleCurrentSelection(): void {
    if (!this.cursorPath) return;
    const next = new Set(this.selectedPaths);
    if (next.has(this.cursorPath)) next.delete(this.cursorPath);
    else next.add(this.cursorPath);
    this.selectedPaths = next;
    this.cdr.markForCheck();
  }

  extendSelection(delta: number): void { this.moveCursor(delta, true); }
  invertSelection(): void {}
  selectSameExtension(): void {}
  selectSameType(): void {}
  selectFiles(): void {
    this.selectedPaths = new Set(this.valueEntries.map(entry => entry.path));
    this.emitCursor();
  }
  selectFolders(): void {
    this.selectedPaths = new Set(this.keyEntries.map(entry => entry.path));
    this.emitCursor();
  }
  unselectFiles(): void {}
  unselectFolders(): void {}
  selectByMaskPrompt(): Promise<void> { return Promise.resolve(); }
  unselectByMaskPrompt(): Promise<void> { return Promise.resolve(); }

  selectPaths(paths: readonly string[]): void {
    this.selectedPaths = new Set(paths.filter(path => this.entries.some(entry => entry.path === path)));
    this.cursorPath = paths[0] ?? this.cursorPath;
    this.emitCursor();
  }

  selectComparisonItem(path: string): void {
    this.selectPaths([path]);
  }

  recoverVisibleCursor(): void {
    if (this.entries.length === 0) {
      this.cursorPath = null;
      this.selectedPaths = new Set<string>();
      this.emitCursor();
      return;
    }
    const selectable = this.selectableEntries();
    const fallback = selectable[0] ?? this.entries[0];
    if (!this.cursorPath || !this.entries.some(entry => entry.path === this.cursorPath)) {
      this.cursorPath = fallback?.path ?? null;
    }
    if (this.cursorPath && this.selectedPaths.size === 0) {
      this.selectedPaths = new Set([this.cursorPath]);
    }
    this.emitCursor();
  }

  calculateSelectedSize(): Promise<void> { return Promise.resolve(); }
  copySelected(): void {}
  cutSelected(): void {}
  paste(): Promise<void> { return Promise.resolve(); }
  deleteSelected(_permanent?: boolean): Promise<void> { return Promise.resolve(); }
  renameSelected(): Promise<void> { return Promise.resolve(); }
  newFolder(): Promise<void> { return Promise.resolve(); }
  newFile(): Promise<void> { return Promise.resolve(); }
  newDocumentTemplate(_kind: string): Promise<void> { return Promise.resolve(); }
  moveSelectionToNewFolder(): Promise<void> { return Promise.resolve(); }
  duplicateSelected(): Promise<void> { return Promise.resolve(); }
  revealSelectedInFileManager(): Promise<void> { return Promise.resolve(); }
  touchSelectedModifiedTime(): Promise<void> { return Promise.resolve(); }
  editSelectedAttributes(): Promise<void> { return Promise.resolve(); }
  showSelectedProperties(): Promise<void> { return Promise.resolve(); }
  showSelectedChecksum(): Promise<void> { return Promise.resolve(); }
  copySelectedPaths(): Promise<void> { return Promise.resolve(); }
  copySelectedRelativePaths(): Promise<void> { return Promise.resolve(); }
  copySelectedFileUris(): Promise<void> { return Promise.resolve(); }
  copySelectedNames(): Promise<void> { return Promise.resolve(); }
  createLinkForSelected(_kind: 'symbolic' | 'hard'): Promise<void> { return Promise.resolve(); }
  createShortcutForSelected(): Promise<void> { return Promise.resolve(); }
  createZipFromSelection(): Promise<void> { return Promise.resolve(); }
  openResultsPanel(_title: string, _entries: readonly FileInfo[], _metadata?: unknown): Promise<void> { return Promise.resolve(); }
  openRecentFilesPanel(): Promise<void> { return Promise.resolve(); }
  openSurfaceTab(_surfaceId: string, _title?: string, _location?: string): Promise<void> { return Promise.resolve(); }
  extractSelectedTo(_destinationDir: string, _flattenPaths = false): Promise<boolean> { return Promise.resolve(false); }
  transferArchiveSelectionTo(_destinationDir: string, _operation: 'copy' | 'move'): Promise<null> { return Promise.resolve(null); }
  treeLeft(): void {}
  treeRight(): void {}
  selectPeerByName(name: string): boolean {
    const entry = this.entries.find(candidate => candidate.name === name);
    if (!entry) return false;
    this.selectPaths([entry.path]);
    return true;
  }

  trackEntry(_index: number, entry: FileInfo): string {
    return entry.path;
  }

  icon(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent': return '..';
      case 'hive': return 'H';
      case 'key': return 'K';
      case 'value': return '=';
    }
  }

  badgeLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent': return 'Up';
      case 'hive': return 'Hive';
      case 'key': return 'Key';
      case 'value': return entry.registryValueType?.trim() || 'Value';
    }
  }

  metaLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent': return 'Open parent key';
      case 'hive': return 'Top-level registry hive';
      case 'key': return 'Registry key';
      case 'value': {
        const type = entry.registryValueType?.trim() || 'REG_SZ';
        const bytes = entry.size > 0 ? `${Math.round(entry.size)} B` : '';
        return bytes ? `${type} value, ${bytes}` : `${type} value`;
      }
    }
  }

  kindLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent': return 'Parent';
      case 'hive': return 'Hive';
      case 'key': return 'Key';
      case 'value': return 'Value';
    }
  }

  typeLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent': return 'Navigate up';
      case 'hive': return 'Root hive';
      case 'key': return 'Subkey';
      case 'value': return entry.registryValueType?.trim() || 'REG_SZ';
    }
  }

  dataLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'hive': return 'Top-level registry hive';
      case 'value': {
        const preview = entry.registryPreview?.trim() ?? '';
        if (preview) return preview;
        return entry.size > 0 ? '(binary data)' : '(empty)';
      }
      default: return '';
    }
  }

  onScroll(): void {
    this.viewportChange.emit({
      topName: this.entries[0]?.name ?? null,
      ratio: 0,
    });
  }

  onBackgroundClick(event: MouseEvent): void {
    if (event.target === event.currentTarget) {
      this.selectedPaths = new Set<string>();
      this.cdr.markForCheck();
    }
  }

  selectEntry(entry: FileInfo, event: MouseEvent): void {
    const next = new Set(this.selectedPaths);
    if (event.ctrlKey || event.metaKey) {
      if (next.has(entry.path)) next.delete(entry.path);
      else next.add(entry.path);
    } else {
      next.clear();
      next.add(entry.path);
    }
    this.selectedPaths = next;
    this.cursorPath = entry.path;
    this.emitCursor();
  }

  emitEntryContextMenu(entry: FileInfo, event: MouseEvent): void {
    this.entryContextMenu.emit({ entry, event });
  }

  private reset(): void {
    if (!this.path && !this.entries.length && !this.error) return;
    this.path = '';
    this.entries = [];
    this.selectedPaths = new Set<string>();
    this.cursorPath = null;
    this.error = '';
    this.backTrail = [];
    this.forwardTrail = [];
    this.cdr.markForCheck();
  }

  private async loadRegistryPath(
    path: string,
    options: { preserveSelection: boolean; history: 'record' | 'none' },
  ): Promise<void> {
    const token = ++this.loadToken;
    const previousPath = this.path;
    const previousSelection = new Set(this.selectedPaths);

    this.loading = true;
    this.error = '';
    this.cdr.markForCheck();

    try {
      await this.activatePluginsForLocation(path);
      const resolved = this.panelSources.resolve({ path });
      if (resolved.source.kind !== 'registry') {
        throw new Error('No enabled plugin is registered for registry://.');
      }

      const listing = await this.panelSources.listDirectory(path, this.showHidden);
      if (token !== this.loadToken) return;

      this.path = listing.path;
      this.entries = this.withParent(listing.path, [...listing.entries]);
      this.selectedPaths = options.preserveSelection
        ? new Set([...previousSelection].filter(candidate => this.entries.some(entry => entry.path === candidate)))
        : new Set<string>();
      this.cursorPath =
        this.cursorPath && this.entries.some(entry => entry.path === this.cursorPath)
          ? this.cursorPath
          : this.selectableEntries()[0]?.path ?? this.entries[0]?.path ?? null;

      if (this.cursorPath && this.selectedPaths.size === 0) {
        this.selectedPaths = new Set([this.cursorPath]);
      }

      if (options.history === 'record' && previousPath && previousPath !== listing.path) {
        this.backTrail.push(previousPath);
        this.forwardTrail = [];
      }

      this.syncWorkspacePath(listing.path);
      this.locationChange.emit(listing.path);
      this.emitCursor();
      this.presentationChange.emit(this.panePresentation());
    } catch (error) {
      if (token !== this.loadToken) return;
      this.error = error instanceof Error ? error.message : String(error);
      this.entries = [];
      this.selectedPaths = new Set<string>();
      this.cursorPath = null;
      this.cursorChange.emit(null);
    } finally {
      if (token !== this.loadToken) return;
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  private withParent(path: string, entries: FileInfo[]): FileInfo[] {
    if (!path || /^registry:\/\/$/i.test(path)) return entries;
    if (entries.some(entry => entry.path === PARENT_ENTRY_PATH)) return entries;
    return [
      {
        name: '..',
        path: PARENT_ENTRY_PATH,
        type: FileType.DIRECTORY,
        size: 0,
        created: 0,
        modified: 0,
        accessed: 0,
        permissions: '',
        isHidden: false,
      },
      ...entries,
    ];
  }

  private selectableEntries(): FileInfo[] {
    return this.entries.filter(entry => entry.path !== PARENT_ENTRY_PATH);
  }

  private entryKind(entry: FileInfo): RegistryEntryKind {
    if (entry.path === PARENT_ENTRY_PATH) return 'parent';
    const kind = String(entry.registryKind ?? '').trim().toLowerCase();
    if (kind === 'value') return 'value';
    const path = this.normalizePath(entry.path);
    const relative = path.slice(ROOT.length).replace(/^\/+/, '');
    if (relative && !relative.includes('/')) return 'hive';
    if (kind === 'key' || entry.type === FileType.DIRECTORY) return 'key';
    return 'value';
  }

  private emitCursor(): void {
    this.cursorChange.emit(this.cursorEntry);
    this.cdr.markForCheck();
  }

  private normalizeLocationInput(path: string): string {
    let value = String(path ?? '').trim();
    if (!value) return value;
    if (/^\/[a-z][a-z0-9+.-]*:/i.test(value)) value = value.slice(1);
    return value;
  }

  private schemeOfLocation(path: string): string {
    const value = this.normalizeLocationInput(path);
    if (!value) return 'filesystem';
    const match = /^([a-z][a-z0-9+.-]*):/i.exec(value);
    return match?.[1]?.toLowerCase() ?? 'filesystem';
  }

  private canonicalizePluginLocation(path: string): string {
    const value = this.normalizeLocationInput(path);
    if (!value) return value;
    const scheme = this.schemeOfLocation(value);
    if (scheme === 'filesystem') return value;
    if (value.toLowerCase() === `${scheme}:`) return `${scheme}://`;
    return value;
  }

  private async activatePluginsForLocation(path: string): Promise<void> {
    const value = this.canonicalizePluginLocation(path);
    if (!value) return;
    const scheme = this.schemeOfLocation(value);
    if (scheme === 'filesystem') return;

    const results = await this.pluginLifecycle.activateByEvent(`onScheme:${scheme}`);
    const failed = results.find(result => !result.activated);
    if (failed) {
      throw new Error(failed.error || `Plugin ${failed.pluginId} failed to activate for ${scheme}://.`);
    }
  }

  private syncWorkspacePath(path: string): void {
    const activeTab = this.workspaceService.getActiveTab(this.paneId);
    if (!activeTab || activeTab.path === path) return;
    this.workspaceService.updateTabPath(this.paneId, activeTab.id, path);
    this.workspacePathSnapshot = path;
  }

  private isRegistryPath(path: string): boolean {
    return /^registry:\/\//i.test(String(path ?? ''));
  }

  private normalizePath(path: string): string {
    const value = String(path ?? '').trim().replace(/\\/g, '/');
    if (!/^registry:\/\//i.test(value)) return value;
    const rest = value.slice(value.indexOf('://') + 3).replace(/^\/+/, '').replace(/\/+/g, '/');
    return rest ? `registry://${rest}` : ROOT;
  }
}
