import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  Output,
  ViewChild,
} from '@angular/core';
import { FileInfo, FileType } from '../shared/protocol';

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
 * controller. This component owns only Registry semantics and presentation:
 * Key/REG_* types, inline value payloads and the Registry Details schema.
 */
@Component({
  selector: 'app-registry-pane',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="registry-details-header"
      [style.grid-template-columns]="detailsGridTemplate">
      <span></span>
      <span class="heading">
        Name
        <i class="resizer" (mousedown)="beginResize($event, 'name')"></i>
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
        *ngFor="let entry of entries; let virtualIndex = index; trackBy: trackEntry"
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
        [style.grid-template-columns]="detailsGridTemplate"
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
          <span class="name">{{ entry.name }}</span>
        </span>

        <span class="type">{{ typeLabel(entry) }}</span>
        <span class="data" [title]="payload(entry)">
          {{ payload(entry) }}
        </span>
      </div>

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
      min-width: 0;
      min-height: 0;
      flex-direction: column;
      overflow: hidden;
    }

    .registry-details-header,
    .registry-entry {
      column-gap: 6px;
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
      user-select: none;
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
      gap: 6px;
      height: 30px;
      min-height: 30px;
      padding: 0 8px;
      cursor: default;
      user-select: none;
      color: var(--vsc-foreground);
    }

    /* Keys and values deliberately use the same row height. */
    .registry-list.view-list .registry-entry {
      grid-template-columns: 22px minmax(0, 1fr) 110px 74px;
    }

    .registry-list.view-list .data {
      display: none;
    }

    .registry-entry:hover {
      background: var(--vsc-list-hover-background);
    }

    .registry-entry.selected {
      background: var(--vsc-list-active-background, var(--vsc-list-selected-emphasis));
      color: var(--vsc-list-active-foreground, var(--vsc-on-accent));
    }

    .registry-entry {
      border-radius: 3px;
    }

    .registry-entry.cursor {
      box-shadow: inset 2px 0 0 var(--vsc-accent-blue, var(--vsc-accent-blue));
      background: color-mix(
        in srgb,
        var(--vsc-list-hover-background, var(--vsc-list-hover-background)) 75%,
        transparent
      );
    }

    .icon {
      width: 18px;
      text-align: center;
      opacity: .9;
    }

    .name-cell {
      min-width: 0;
      overflow: hidden;
    }

    .name,
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

    .type,
    .data {
      font-size: 10px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .registry-entry.selected .type,
    .registry-entry.selected .data {
      color: inherit;
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
  `],
})
export class RegistryPaneComponent {
  @ViewChild('viewport', { static: true }) viewport?: ElementRef<HTMLElement>;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() path = '';
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
      this.track('name', 'minmax(220px, 1fr)'),
      this.track('type', '150px'),
      this.track('data', 'minmax(280px, 2fr)'),
    ].join(' ');
  }

  trackEntry(_index: number, entry: FileInfo): string {
    return entry.path;
  }

  typeLabel(entry: FileInfo): string {
    if (entry.path === '__parent__') return 'Key';

    const kind = String(entry.registryKind ?? '').toLowerCase();
    if (kind === 'key') return 'Key';
    if (kind === 'value') return entry.registryValueType?.trim() || 'Value';

    // Registry backends historically represented keys/values using the
    // filesystem FileType enum. Preserve Registry semantics even for old rows.
    return entry.type === FileType.DIRECTORY ? 'Key' : (entry.registryValueType?.trim() || 'Value');
  }

  payload(entry: FileInfo): string {
    if (entry.path === '__parent__') return '';
    const kind = String(entry.registryKind ?? '').toLowerCase();
    if (kind && kind !== 'value') return '';
    return entry.registryPreview?.trim() ?? '';
  }

  sizeLabel(entry: FileInfo): string {
    if (this.typeLabel(entry) === 'Key') return '';
    return this.formatSize(entry.size);
  }

  icon(entry: FileInfo): string {
    return this.typeLabel(entry) === 'Key' ? '📁' : '🔑';
  }

  ariaLabel(entry: FileInfo): string {
    const type = this.typeLabel(entry);
    const data = this.payload(entry);
    return data ? `${entry.name}, ${type}, ${data}` : `${entry.name}, ${type}`;
  }

  beginResize(event: MouseEvent, key: string): void {
    event.preventDefault();
    event.stopPropagation();

    const heading = (event.target as HTMLElement | null)?.closest('.heading') as HTMLElement | null;
    if (!heading) return;

    this.resizeKey = key;
    this.resizeStartX = event.clientX;
    this.resizeStartWidth = Math.max(this.minWidth, Math.round(heading.getBoundingClientRect().width));
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
      Math.max(this.minWidth, Math.round(this.resizeStartWidth + event.clientX - this.resizeStartX)),
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
        if (typeof width === 'number' && Number.isFinite(width) && width >= this.minWidth) {
          this.widths.set(key, Math.round(width));
        }
      }
    } catch {
      // Presentation persistence is best effort.
    }
  }

  private persistWidths(): void {
    try {
      localStorage.setItem(this.storageKey, JSON.stringify(Object.fromEntries(this.widths)));
    } catch {
      // Presentation persistence is best effort.
    }
  }

  private formatSize(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let value = bytes;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
      value /= 1024;
      unit += 1;
    }
    return `${unit === 0 ? Math.round(value) : value.toFixed(value >= 10 ? 1 : 2)} ${units[unit]}`;
  }
}
