import { CommonModule } from '@angular/common';
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
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
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
      *ngIf="viewMode === 'details' && !isRootView"
      [style.grid-template-columns]="detailsGridTemplate">
      <span></span>
      <span class="heading">
        Name
        <i class="resizer" (mousedown)="beginResize($event, 'name')"></i>
      </span>
      <span class="heading">
        Kind
        <i class="resizer" (mousedown)="beginResize($event, 'kind')"></i>
      </span>
      <span class="heading">
        Type
        <i class="resizer" (mousedown)="beginResize($event, 'type')"></i>
      </span>
      <span class="heading">Data</span>
    </div>

    <div
      #viewport
      class="registry-list"
      data-pane-content
      tabindex="-1"
      role="listbox"
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

      <ng-container *ngIf="isRootView; else groupedKeyView">
        <div class="registry-section registry-section-root">
          <div
            *ngFor="let entry of entries; let virtualIndex = index; trackBy: trackEntry"
            class="registry-entry registry-entry-root"
            tabindex="-1"
            role="option"
            draggable="true"
            [attr.data-file-entry]="entry.path"
            [attr.aria-posinset]="renderedStart + virtualIndex + 1"
            [attr.aria-setsize]="visibleEntryCount"
            [attr.aria-selected]="selectedPaths.has(entry.path)"
            [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
            [attr.aria-label]="ariaLabel(entry)"
            [class.cursor]="entry.path === cursorPath"
            [class.selected]="selectedPaths.has(entry.path)"
            [class.drop-target]="dragOverPath === entry.path"
            [class.drop-invalid]="dragOverPath === entry.path && dropIntent === 'invalid'"
            [class.drop-copy]="dragOverPath === entry.path && dropIntent === 'copy'"
            [class.drop-move]="dragOverPath === entry.path && dropIntent === 'move'"
            [class.kind-parent]="entryKind(entry) === 'parent'"
            [class.kind-hive]="entryKind(entry) === 'hive'"
            [style.grid-template-columns]="rootGridTemplate"
            (click)="entrySelect.emit({ entry: entry, event: $event })"
            (dblclick)="entryOpen.emit(entry)"
            (contextmenu)="entryContextMenu.emit({ entry: entry, event: $event })"
            (dragstart)="entryDragStart.emit({ entry: entry, event: $event })"
            (dragend)="entryDragEnd.emit()"
            (dragover)="entryDragOver.emit({ entry: entry, event: $event })"
            (dragleave)="entryDragLeave.emit(entry)"
            (drop)="entryDrop.emit({ entry: entry, event: $event })">
            <span class="icon" aria-hidden="true">{{ icon(entry) }}</span>
            <span class="name-cell">
              <span class="name-line">
                <span class="name">{{ entry.name }}</span>
                <span class="badge">{{ badgeLabel(entry) }}</span>
              </span>
              <span
                class="meta"
                *ngIf="metaLabel(entry) as value"
                [title]="value">{{ value }}</span>
            </span>
            <span class="data root-data" [title]="dataLabel(entry)">{{ dataLabel(entry) }}</span>
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
          <div class="registry-value-inspector-data" [title]="dataLabel(entry)">{{ dataLabel(entry) }}</div>
        </div>

        <div class="registry-section" *ngIf="parentEntry as entry">
          <div class="registry-section-label">Navigation</div>
          <ng-container
            [ngTemplateOutlet]="entryRow"
            [ngTemplateOutletContext]="{ $implicit: entry, virtualIndex: entryIndex(entry) }">
          </ng-container>
        </div>

        <div class="registry-section" *ngIf="keyEntries.length">
          <div class="registry-section-label">Subkeys</div>
          <ng-container *ngFor="let entry of keyEntries; trackBy: trackEntry">
            <ng-container
              [ngTemplateOutlet]="entryRow"
              [ngTemplateOutletContext]="{ $implicit: entry, virtualIndex: entryIndex(entry) }">
            </ng-container>
          </ng-container>
        </div>

        <div class="registry-section" *ngIf="valueEntries.length">
          <div class="registry-section-label">Values</div>
          <ng-container *ngFor="let entry of valueEntries; trackBy: trackEntry">
            <ng-container
              [ngTemplateOutlet]="entryRow"
              [ngTemplateOutletContext]="{ $implicit: entry, virtualIndex: entryIndex(entry) }">
            </ng-container>
          </ng-container>
        </div>
      </ng-template>

      <ng-template #entryRow let-entry let-virtualIndex="virtualIndex">
        <div
          class="registry-entry"
          tabindex="-1"
          role="option"
          draggable="true"
          [attr.data-file-entry]="entry.path"
          [attr.aria-posinset]="renderedStart + virtualIndex + 1"
          [attr.aria-setsize]="visibleEntryCount"
          [attr.aria-selected]="selectedPaths.has(entry.path)"
          [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
          [attr.aria-label]="ariaLabel(entry)"
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
          [style.grid-template-columns]="viewMode === 'details' ? detailsGridTemplate : null"
          (click)="entrySelect.emit({ entry: entry, event: $event })"
          (dblclick)="entryOpen.emit(entry)"
          (contextmenu)="entryContextMenu.emit({ entry: entry, event: $event })"
          (dragstart)="entryDragStart.emit({ entry: entry, event: $event })"
          (dragend)="entryDragEnd.emit()"
          (dragover)="entryDragOver.emit({ entry: entry, event: $event })"
          (dragleave)="entryDragLeave.emit(entry)"
          (drop)="entryDrop.emit({ entry: entry, event: $event })">

          <span class="icon" aria-hidden="true">{{ icon(entry) }}</span>

          <span class="name-cell">
            <span class="name-line">
              <span class="name">{{ entry.name }}</span>
              <span class="badge">{{ badgeLabel(entry) }}</span>
            </span>
            <span
              class="meta"
              *ngIf="metaLabel(entry) as value"
              [title]="value">{{ value }}</span>
          </span>

          <span class="kind">{{ kindLabel(entry) }}</span>
          <span class="type">{{ typeLabel(entry) }}</span>
          <span class="data" [title]="dataLabel(entry)">{{ dataLabel(entry) }}</span>
        </div>
      </ng-template>

      <div
        *ngIf="virtualEnabled && afterPx > 0"
        class="virtual-spacer"
        [style.height.px]="afterPx"
        aria-hidden="true"></div>

      <div class="empty" *ngIf="!loading && visibleEntryCount === 0" role="status">
        <strong>{{ filterText ? 'No matching registry entries.' : 'This registry key is empty.' }}</strong>
        <span *ngIf="filterText">Clear the filter to see all entries.</span>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex: 1 1 0%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      flex-direction: column;
      overflow: hidden;
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
      letter-spacing: .02em;
    }

    .registry-surface-subtitle {
      font-size: 10px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .registry-details-header {
      display: grid;
      align-items: center;
      min-height: 25px;
      padding: 0 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
      user-select: none;      column-gap: var(--pane-column-gap, 6px);
    }

    .heading {
      position: relative;
      min-width: 0;
      padding-inline: 3px;
    }

    .resizer {
      position: absolute;
      z-index: 3;
      top: 2px;
      right: -5px;
      bottom: 2px;
      width: 9px;
      cursor: col-resize;
    }

    .resizer::after {
      content: '';
      position: absolute;
      top: 3px;
      bottom: 3px;
      left: 4px;
      width: 1px;
      background: transparent;
    }

    .resizer:hover::after {
      background: var(--vsc-focus-border, var(--vsc-accent-blue));
    }

    .registry-list {
      flex: 1 1 0%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      overflow: auto;
      outline: none;
      scrollbar-width: thin;
    }

    .registry-entry {
      box-sizing: border-box;
      display: grid;
      align-items: center;

      min-height: 34px;
      padding: 4px 8px;
      background: transparent;
      cursor: default;
      user-select: none;
      color: var(--vsc-foreground);
    
      column-gap: var(--pane-column-gap, 6px);
}

    .registry-section {
      display: grid;
      gap: 2px;
      padding: 6px 0 2px;
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
      align-items: center;
      gap: 8px;
      min-width: 0;
    }

    .registry-value-inspector-title {
      font-size: 10px;
      font-weight: 700;
      letter-spacing: .08em;
      text-transform: uppercase;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
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
      color: var(--vsc-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .registry-value-inspector-meta {
      font-size: 10px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .registry-value-inspector-data {
      padding: 8px 10px;
      border-radius: 8px;
      background: color-mix(in srgb, var(--vsc-editor-background, #fff) 82%, black 18%);
      color: var(--vsc-foreground);
      font: 12px/1.4 var(--vsc-font-family-mono, ui-monospace, monospace);
      white-space: pre-wrap;
      word-break: break-word;
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
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .registry-list.view-list .registry-entry {
      grid-template-columns: 22px minmax(0, 1fr) 110px minmax(120px, 1fr);
    }

    .registry-list.view-list .type {
      display: none;
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
      text-align: center;
      opacity: .9;
      font: 600 10px/1 var(--vsc-font-family-mono, ui-monospace, monospace);
      letter-spacing: .04em;
      border-radius: 999px;
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-description-foreground)) 85%, transparent);
      background: color-mix(in srgb, var(--vsc-editor-background, #fff) 84%, var(--vsc-panel-background, #fff));
    }

    .name-cell {
      display: grid;
      min-width: 0;
      grid-template-rows: auto auto;
      align-content: center;
      gap: 2px;
      overflow: hidden;
    }

    .name-line {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      min-width: 0;
    }

    .name,
    .meta,
    .badge,
    .kind,
    .type,
    .data {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .name {
      font-size: 11px;
      line-height: 13px;
    }

    .badge {
      flex: 0 0 auto;
      max-width: 88px;
      padding: 1px 6px;
      border-radius: 999px;
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 12%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-border));
      font-size: 9px;
      line-height: 12px;
      font-weight: 600;
      letter-spacing: .03em;
      text-transform: uppercase;
    }

    .meta {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 9px;
      line-height: 11px;
    }

    .kind,
    .type,
    .data {
      font-size: 10px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .kind {
      font-weight: 500;
    }

    .registry-entry.kind-parent .badge,
    .registry-entry.kind-parent .icon {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .registry-entry.kind-hive .badge,
    .registry-entry.kind-hive .icon {
      color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 90%, white 10%);
    }

    .registry-entry.kind-key .badge,
    .registry-entry.kind-key .icon {
      color: color-mix(in srgb, var(--vsc-entry-directory, var(--vsc-accent-blue)) 85%, white 15%);
    }

    .registry-entry.kind-value .badge,
    .registry-entry.kind-value .icon {
      color: color-mix(in srgb, var(--vsc-foreground, #000) 75%, var(--vsc-accent-blue, #06c) 25%);
    }

    .registry-entry-root {
      min-height: 52px;
      padding: 10px 12px;
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-foreground-dim)) 65%, transparent);
      border-radius: 10px;
      background: transparent;
    }

    .registry-entry-root .icon {
      width: 22px;
      height: 22px;
      font-size: 11px;
    }

    .root-data {
      font-size: 10px;
    }

    .registry-entry.selected .badge,
    .registry-entry.selected .meta,
    .registry-entry.selected .kind,
    .registry-entry.selected .type,
    .registry-entry.selected .data {
      color: inherit;
      opacity: .88;
    }

    .virtual-spacer {
      width: 1px;
      pointer-events: none;
    }

    .empty {
      min-height: 120px;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 4px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
    }

    .background-drop-target,
    .drop-target {
      outline: 1px dashed var(--vsc-focus-border, var(--vsc-accent-blue));
      outline-offset: -1px;
    }

    .drop-invalid {
      opacity: .55;
    }

    /* visible pane column gutters */
    .registry-details-header,
    .registry-entry {
      column-gap: var(--pane-column-gutter, 12px);
    }

    .registry-details-header > span:not(:first-child),
    .registry-entry > span:not(:first-child) {
      min-width: 0;
      padding-inline: 2px;
    }

    /* Resizer sits in the middle of the gutter rather than on top of text. */
    .registry-details-header .resizer {
      right: calc(var(--pane-column-gutter, 12px) / -2 - 1px);
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
    return '28px minmax(160px, 1fr) minmax(140px, 240px)';
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
        return 'Top-level registry hive';
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
        return 'Top-level registry hive';
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
        return 'H';
      case 'key':
        return 'K';
      case 'value':
        return '=';
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
