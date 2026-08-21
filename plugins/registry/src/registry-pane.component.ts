import { CommonModule } from '@angular/common';
import {
  PaneTableBodyDirective,
  PaneTableCellDirective,
  PaneTableComponent,
  PaneTableDividerDirective,
  PaneTableEmptyDirective,
  PaneTableHeaderDirective,
  PaneTableHeaderRowDirective,
  PaneTableRowDirective,
} from '@reassembly/core/ui/panes';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnDestroy,
  Output,
  ViewChild,
} from '@angular/core';
import {
  RegistryFileInfo as FileInfo,
  RegistryFileType as FileType,
} from './registry-model';

export type RegistryPaneViewMode = 'list' | 'details';
type RegistryEntryKind = 'parent' | 'hive' | 'key' | 'value';
const ROOT_HIVE_NAMES = [
  'HKEY_CLASSES_ROOT',
  'HKEY_CURRENT_CONFIG',
  'HKEY_CURRENT_USER',
  'HKEY_LOCAL_MACHINE',
  'HKEY_USERS',
] as const;

const ROOT_HIVE_ENTRIES: readonly FileInfo[] = ROOT_HIVE_NAMES.map(name => ({
  name,
  path: 'registry://' + name,
  type: FileType.DIRECTORY,
  size: 0,
  created: 0,
  modified: 0,
  accessed: 0,
  permissions: '',
  isHidden: false,
  registryKind: 'Key',
}));

export interface RegistryEntryPointerEvent {
  entry: FileInfo;
  event: MouseEvent;
}

export interface RegistryEntryDragEvent {
  entry: FileInfo;
  event: DragEvent;
}

/**
 * Registry-specific collection presentation.
 *
 * Navigation, selection, virtualization and commands remain owned by the pane
 * controller. This component owns only registry semantics and presentation:
 * hives, subkeys, REG_* types and inline value payloads.
 */
@Component({
  selector: 'app-registry-pane',
  standalone: true,
  imports: [
    CommonModule,
    PaneTableComponent,
    PaneTableHeaderDirective,
    PaneTableHeaderRowDirective,
    PaneTableBodyDirective,
    PaneTableRowDirective,
    PaneTableCellDirective,
    PaneTableDividerDirective,
    PaneTableEmptyDirective,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <reassembly-pane-table class="registry-table" role="table">
      <div
        *ngIf="viewMode === 'details'"
        class="registry-details-header"
        reassemblyPaneTableHeader
        role="rowgroup">
        <div
          class="registry-entry registry-header-row"
          reassemblyPaneTableHeaderRow
          role="row"
          [style.grid-template-columns]="detailsGridTemplate">
          <span class="registry-cell icon-cell" reassemblyPaneTableCell></span>
          <span class="registry-cell heading" reassemblyPaneTableCell role="columnheader">
            Name
            <i class="resizer" reassemblyPaneTableDivider (mousedown)="beginResize($event, 'name')"></i>
          </span>
          <span class="registry-cell heading" reassemblyPaneTableCell role="columnheader">
            Kind
            <i class="resizer" reassemblyPaneTableDivider (mousedown)="beginResize($event, 'kind')"></i>
          </span>
          <span class="registry-cell heading" reassemblyPaneTableCell role="columnheader">
            Type
            <i class="resizer" reassemblyPaneTableDivider (mousedown)="beginResize($event, 'type')"></i>
          </span>
          <span class="registry-cell heading" reassemblyPaneTableCell role="columnheader">
            Data
          </span>
        </div>
      </div>

      <div
        #viewport
        class="registry-list"
        reassemblyPaneTableBody
        data-pane-content
        tabindex="-1"
        role="rowgroup"
        [attr.aria-label]="'Registry entries in ' + (path || 'location')"
        [class.view-list]="viewMode === 'list'"
        [class.view-details]="viewMode === 'details'"
        [class.background-drop-target]="backgroundDropActive"
        [attr.aria-busy]="loading"
        (scroll)="scrolled.emit($event)"
        (click)="backgroundClick.emit($event)"
        (contextmenu)="backgroundContextMenu.emit($event)"
        (dragover)="backgroundDragOver.emit($event)"
        (dragleave)="backgroundDragLeave.emit($event)"
        (drop)="backgroundDrop.emit($event)">

        <div
          *ngIf="virtualEnabled && beforePx > 0"
          class="virtual-spacer"
          [style.height.px]="beforePx"
          aria-hidden="true"></div>

        <div
          *ngFor="let entry of (isRootView ? rootEntries : entries); let virtualIndex = index; trackBy: trackEntry"
          class="registry-entry"
          reassemblyPaneTableRow
          tabindex="-1"
          role="row"
          draggable="true"
          [attr.data-file-entry]="entry.path"
          [attr.aria-posinset]="renderedStart + virtualIndex + 1"
          [attr.aria-setsize]="effectiveVisibleEntryCount"
          [attr.aria-selected]="selectedPaths.has(entry.path)"
          [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
          [attr.aria-label]="ariaLabel(entry)"
          [class.registry-entry-root]="isRootView"
          [class.cursor]="entry.path === cursorPath"
          [class.selected]="selectedPaths.has(entry.path)"
          [class.drop-target]="dragOverPath === entry.path"
          [class.drop-invalid]="dragOverPath === entry.path && dropIntent === 'invalid'"
          [class.drop-copy]="dragOverPath === entry.path && dropIntent === 'copy'"
          [class.drop-move]="dragOverPath === entry.path && dropIntent === 'move'"
          [class.kind-parent]="entryKind(entry) === 'parent'"
          [class.kind-hive]="entryKind(entry) === 'hive'"
          [class.kind-key]="entryKind(entry) === 'key'"
          [class.kind-value]="entryKind(entry) === 'value'"
          [style.grid-template-columns]="viewMode === 'details' ? detailsGridTemplate : rootGridTemplate"
          (click)="entrySelect.emit({ entry: entry, event: $event })"
          (dblclick)="entryOpen.emit(entry)"
          (contextmenu)="entryContextMenu.emit({ entry: entry, event: $event })"
          (dragstart)="entryDragStart.emit({ entry: entry, event: $event })"
          (dragend)="entryDragEnd.emit()"
          (dragover)="entryDragOver.emit({ entry: entry, event: $event })"
          (dragleave)="entryDragLeave.emit(entry)"
          (drop)="entryDrop.emit({ entry: entry, event: $event })">

          <span class="registry-cell icon" reassemblyPaneTableCell aria-hidden="true">{{ icon(entry) }}</span>
          <span class="registry-cell name" reassemblyPaneTableCell [title]="entry.name">{{ entry.name }}</span>
          <span class="registry-cell kind" reassemblyPaneTableCell>{{ kindLabel(entry) }}</span>
          <span class="registry-cell type" reassemblyPaneTableCell>{{ typeLabel(entry) }}</span>
          <span class="registry-cell data" reassemblyPaneTableCell [title]="dataLabel(entry)">{{ dataLabel(entry) }}</span>
        </div>

        <div
          *ngIf="virtualEnabled && afterPx > 0"
          class="virtual-spacer"
          [style.height.px]="afterPx"
          aria-hidden="true"></div>

        <div
          class="empty"
          reassemblyPaneTableEmpty
          *ngIf="!loading && effectiveVisibleEntryCount === 0"
          role="status">
          {{ filterText ? 'No matching registry entries.' : 'This registry key is empty.' }}
        </div>
      </div>
    </reassembly-pane-table>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      height: 100%;
      overflow: hidden;
      contain: layout style paint;
    }

    .registry-table {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background);
    }

    .registry-details-header {
      flex: 0 0 auto;
      background: var(--vsc-tabbar-background);
      border-bottom: 1px solid var(--vsc-border);
    }

    .registry-entry {
      box-sizing: border-box;
      display: grid;
      align-items: center;
      min-height: 24px;
      height: 24px;
      padding: 0 var(--vsc-padding-md);
      column-gap: var(--pane-column-gutter, 12px);
      background: transparent;
      contain: layout style;
      position: relative;
    }

    .registry-header-row {
      min-height: 28px;
      height: 28px;
      font-weight: 600;
      color: var(--vsc-foreground-dim);
      cursor: default;
    }

    .registry-list {
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: auto;
      outline: none;
      background: var(--vsc-editor-background);
      scrollbar-width: thin;
    }

    .registry-cell {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--vsc-font-size);
    }

    .heading {
      position: relative;
    }

    .resizer {
      position: absolute;
      top: 0;
      right: calc(var(--pane-column-gutter, 12px) / -2 - 3px);
      bottom: 0;
      width: 6px;
      cursor: col-resize;
      z-index: 3;
    }

    .icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 18px;
      height: 18px;
      padding: 0;
      color: var(--vsc-foreground-dim);
      font: 600 11px/1 var(--vsc-font-family-mono, ui-monospace, monospace);
    }

    .name {
      color: var(--vsc-foreground);
    }

    .kind,
    .type,
    .data {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .registry-entry.selected .kind,
    .registry-entry.selected .type,
    .registry-entry.selected .data,
    .registry-entry.selected .icon {
      color: inherit;
    }

    .registry-entry.kind-parent {
      opacity: .7;
    }

    .registry-entry.kind-hive .icon,
    .registry-entry.kind-key .icon {
      color: var(--vsc-entry-directory, var(--vsc-accent-blue));
    }

    .registry-list.view-list .kind,
    .registry-list.view-list .type,
    .registry-list.view-list .data {
      display: none;
    }

    .registry-entry-root {
      background: transparent;
    }

    .virtual-spacer {
      width: 1px;
      pointer-events: none;
    }

    .empty {
      min-height: 120px;
    }

    .background-drop-target,
    .drop-target {
      outline: 1px dashed var(--vsc-focus-border, var(--vsc-accent-blue));
      outline-offset: -1px;
    }

    .drop-invalid {
      opacity: .55;
    }
  `],
})
export class RegistryPaneComponent implements OnDestroy {
  @ViewChild('viewport', { static: true }) viewport?: ElementRef<HTMLElement>;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() path = '';
  @Input() viewMode: RegistryPaneViewMode = 'list';
  @Input() entries: readonly FileInfo[] = [];
  @Input() selectedPaths: ReadonlySet<string> = new Set<string>();
  @Input() cursorPath: string | null = null;
  @Input() visibleEntryCount = 0;
  @Input() renderedStart = 0;
  @Input() virtualEnabled = false;
  @Input() beforePx = 0;
  @Input() afterPx = 0;
  @Input() loading = false;
  @Input() filterText = '';
  @Input() dragOverPath: string | null = null;
  @Input() dropIntent: 'copy' | 'move' | 'invalid' | null = null;
  @Input() backgroundDropActive = false;

  @Output() scrolled = new EventEmitter<Event>();
  @Output() backgroundClick = new EventEmitter<MouseEvent>();
  @Output() backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() backgroundDragOver = new EventEmitter<DragEvent>();
  @Output() backgroundDragLeave = new EventEmitter<DragEvent>();
  @Output() backgroundDrop = new EventEmitter<DragEvent>();

  @Output() entrySelect = new EventEmitter<RegistryEntryPointerEvent>();
  @Output() entryOpen = new EventEmitter<FileInfo>();
  @Output() entryContextMenu = new EventEmitter<RegistryEntryPointerEvent>();
  @Output() entryDragStart = new EventEmitter<RegistryEntryDragEvent>();
  @Output() entryDragEnd = new EventEmitter<void>();
  @Output() entryDragOver = new EventEmitter<RegistryEntryDragEvent>();
  @Output() entryDragLeave = new EventEmitter<FileInfo>();
  @Output() entryDrop = new EventEmitter<RegistryEntryDragEvent>();

  private readonly widths = new Map<string, number>();
  private resizeKey: string | null = null;
  private resizeStartX = 0;
  private resizeStartWidth = 0;
  private moveHandler?: (event: MouseEvent) => void;
  private upHandler?: () => void;
  private readonly minWidth = 64;

  get viewportElement(): HTMLElement | undefined {
    return this.viewport?.nativeElement;
  }

  get detailsGridTemplate(): string {
    return [
      '28px',
      this.track('name', 'minmax(180px, 1fr)'),
      this.track('kind', '96px'),
      this.track('type', '120px'),
      this.track('data', 'minmax(220px, 1fr)'),
    ].join(' ');
  }

  get rootGridTemplate(): string {
    return '28px minmax(160px, 1fr)';
  }

  /**
   * The Registry root is structurally fixed. Keep it visible even during the
   * short interval between mounting the dedicated sibling and provider
   * activation/list completion.
   */
  get rootEntries(): readonly FileInfo[] {
    return this.isRootView && this.entries.length === 0
      ? ROOT_HIVE_ENTRIES
      : this.entries;
  }

  get effectiveVisibleEntryCount(): number {
    return this.isRootView ? this.rootEntries.length : this.visibleEntryCount;
  }

  get isRootView(): boolean {
    return this.normalizeRegistryPath(this.path) === 'registry://';
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

    const selected = this.entries.find(entry =>
      this.selectedPaths.has(entry.path) && this.entryKind(entry) === 'value',
    );
    return selected ?? null;
  }

  ngOnDestroy(): void {
    this.endResize();
  }

  trackEntry(_index: number, entry: FileInfo): string {
    return entry.path;
  }

  entryIndex(entry: FileInfo): number {
    const index = this.entries.findIndex(candidate => candidate.path === entry.path);
    return index < 0 ? 0 : index;
  }

  entryKind(entry: FileInfo): RegistryEntryKind {
    if (entry.path === '__parent__') return 'parent';

    const kind = String(entry.registryKind ?? '').trim().toLowerCase();
    if (kind === 'value') return 'value';
    if (this.isHive(entry)) return 'hive';
    if (kind === 'key') return 'key';

    return entry.type === FileType.DIRECTORY ? 'key' : 'value';
  }

  kindLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent':
        return 'Parent';
      case 'hive':
        return 'Hive';
      case 'key':
        return 'Key';
      case 'value':
        return 'Value';
    }
  }

  typeLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent':
        return 'Navigate up';
      case 'hive':
        return 'Root hive';
      case 'key':
        return 'Subkey';
      case 'value':
        return entry.registryValueType?.trim() || 'REG_SZ';
    }
  }

  metaLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent':
        return 'Open parent key';
      case 'hive':
        return '';
      case 'key':
        return 'Registry key';
      case 'value':
        return this.formatValueMeta(entry);
    }
  }

  dataLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent':
        return '';
      case 'hive':
        return '';
      case 'key':
        return '';
      case 'value':
        return this.formatValueData(entry);
    }
  }

  badgeLabel(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent':
        return 'Up';
      case 'hive':
        return 'Hive';
      case 'key':
        return 'Key';
      case 'value':
        return entry.registryValueType?.trim() || 'Value';
    }
  }

  icon(entry: FileInfo): string {
    switch (this.entryKind(entry)) {
      case 'parent':
        return '..';
      case 'hive':
        return '▣';
      case 'key':
        return '▸';
      case 'value':
        return '•';
    }
  }

  ariaLabel(entry: FileInfo): string {
    const parts = [
      entry.name,
      this.kindLabel(entry),
      this.typeLabel(entry),
      this.dataLabel(entry),
    ].filter(value => !!value);
    return parts.join(', ');
  }

  beginResize(event: MouseEvent, key: string): void {
    event.preventDefault();
    event.stopPropagation();

    const heading = (event.target as HTMLElement | null)?.closest('.heading') as HTMLElement | null;
    if (!heading) return;

    this.resizeKey = key;
    this.resizeStartX = event.clientX;
    this.resizeStartWidth = Math.max(
      this.minWidth,
      Math.round(heading.getBoundingClientRect().width),
    );
    this.widths.set(key, this.resizeStartWidth);

    this.moveHandler = move => this.resize(move);
    this.upHandler = () => this.endResize();
    window.addEventListener('mousemove', this.moveHandler);
    window.addEventListener('mouseup', this.upHandler);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  private resize(event: MouseEvent): void {
    if (!this.resizeKey) return;
    this.widths.set(
      this.resizeKey,
      Math.max(
        this.minWidth,
        Math.round(this.resizeStartWidth + event.clientX - this.resizeStartX),
      ),
    );
  }

  private endResize(): void {
    this.persistWidths();
    if (this.moveHandler) window.removeEventListener('mousemove', this.moveHandler);
    if (this.upHandler) window.removeEventListener('mouseup', this.upHandler);
    this.moveHandler = undefined;
    this.upHandler = undefined;
    this.resizeKey = null;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }

  private track(key: string, fallback: string): string {
    if (this.widths.size === 0) this.restoreWidths();
    const width = this.widths.get(key);
    return typeof width === 'number' ? `${width}px` : fallback;
  }

  private get storageKey(): string {
    return `reassembly.registryColumns:${this.paneId}`;
  }

  private restoreWidths(): void {
    try {
      const raw = localStorage.getItem(this.storageKey);
      if (!raw) return;
      const value = JSON.parse(raw) as Record<string, unknown>;
      for (const [key, width] of Object.entries(value)) {
        if (
          typeof width === 'number' &&
          Number.isFinite(width) &&
          width >= this.minWidth
        ) {
          this.widths.set(key, Math.round(width));
        }
      }
    } catch {
      // Presentation persistence is best effort.
    }
  }

  private persistWidths(): void {
    try {
      const widths: Record<string, number> = {};
      for (const [column, width] of this.widths) {
        widths[column] = width;
      }
      localStorage.setItem(this.storageKey, JSON.stringify(widths));
    } catch {
      // Presentation persistence is best effort.
    }
  }

  private isHive(entry: FileInfo): boolean {
    const path = this.normalizeRegistryPath(entry.path);
    if (!path.toLowerCase().startsWith('registry://')) return false;
    const relative = path.slice('registry://'.length).replace(/^\/+/, '');
    return !!relative && !relative.includes('/');
  }

  private normalizeRegistryPath(path: string): string {
    const value = String(path ?? '').trim().replace(/\\/g, '/');
    if (!/^registry:\/\//i.test(value)) return value;
    const rest = value.slice(value.indexOf('://') + 3).replace(/^\/+/, '').replace(/\/+/g, '/');
    return rest ? `registry://${rest}` : 'registry://';
  }

  private formatValueMeta(entry: FileInfo): string {
    const type = entry.registryValueType?.trim() || 'REG_SZ';
    const bytes = this.formatByteSize(entry.size);
    return bytes ? `${type} value, ${bytes}` : `${type} value`;
  }

  private formatValueData(entry: FileInfo): string {
    const preview = entry.registryPreview?.trim() ?? '';
    if (preview) return preview;
    if (entry.size > 0) return '(binary data)';
    return '(empty)';
  }

  private formatByteSize(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '';
    return `${Math.round(bytes)} B`;
  }
}
