import { ChangeDetectionStrategy, Component, ElementRef, EventEmitter, Input, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';
import { PaneDetailsResizeHandleDirective } from './pane-details-resize-handle.directive';

type BinFileInfo = FileInfo & {
  binGroupHeader?: boolean;
  binGroupKey?: string;
  binRevisionCount?: number;
  binOriginalPath?: string;
  binDeletedAt?: string;
  binRevisionLabel?: string;
  binOriginalDirectory?: string;
};

/**
 * Recycle Bin's grouped presentation is intentionally a custom pane view.
 * It models an original item as a non-actionable group row with concrete
 * deleted revisions beneath it; that hierarchy is not a generic file tree.
 */
@Component({
  selector: 'app-bin-grouped-view',
  standalone: true,
  imports: [CommonModule, PaneDetailsResizeHandleDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      #viewport
      class="bin-grouped-view"
      data-pane-content
      data-pane-details-root
      tabindex="-1"
      role="listbox"
      aria-label="Recycle Bin grouped by original item"
      [attr.aria-busy]="loading"
      (click)="backgroundClick.emit($event)"
      (contextmenu)="backgroundContextMenu.emit($event)"
      (scroll)="scroll.emit($event)">
      <div class="details-header" aria-hidden="true">
        <span></span>
        <span>Name<i [paneDetailsResizeHandle]="'bin-name'" [paneDetailsMinWidth]="140"></i></span>
        <span>Path<i [paneDetailsResizeHandle]="'bin-location'" [paneDetailsMinWidth]="120"></i></span>
        <span>Deleted<i [paneDetailsResizeHandle]="'bin-deleted'" [paneDetailsMinWidth]="100"></i></span>
        <span>Size<i [paneDetailsResizeHandle]="'bin-size'" [paneDetailsMinWidth]="64"></i></span>
        <span>Revision<i [paneDetailsResizeHandle]="'bin-revision'" [paneDetailsMinWidth]="64"></i></span>
      </div>

      <div *ngIf="beforePx > 0" class="virtual-spacer" [style.height.px]="beforePx" aria-hidden="true"></div>

      <div
        *ngFor="let entry of entries; trackBy: trackEntry"
        class="entry"
        tabindex="-1"
        role="option"
        [attr.data-file-entry]="entry.path"
        [attr.aria-selected]="isGroup(entry) ? false : selectedPaths.has(entry.path)"
        [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
        [class.cursor]="entry.path === cursorPath"
        [class.selected]="!isGroup(entry) && selectedPaths.has(entry.path)"
        [class.group]="isGroup(entry)"
        [class.revision]="!isGroup(entry)"
        (click)="entryClick.emit({ entry: entry, event: $event }); $event.stopPropagation()"
        (dblclick)="entryDoubleClick.emit({ entry: entry, event: $event }); $event.stopPropagation()"
        (contextmenu)="entryContextMenu.emit({ entry: entry, event: $event }); $event.stopPropagation()">
        <span class="leading">
          <button
            *ngIf="isGroup(entry); else revisionIndent"
            type="button"
            class="disclosure"
            tabindex="-1"
            [attr.aria-label]="(isExpanded(entry) ? 'Collapse ' : 'Expand ') + entry.name + ' revisions'"
            [attr.aria-expanded]="isExpanded(entry)"
            (click)="toggleGroup.emit({ entry: entry, event: $event }); $event.stopPropagation()">
            {{ isExpanded(entry) ? '▾' : '▸' }}
          </button>
          <ng-template #revisionIndent><span class="revision-indent"></span></ng-template>
          <span class="icon" aria-hidden="true">{{ entry.type === FileType.DIRECTORY ? '📁' : '📄' }}</span>
        </span>
        <span class="name" [title]="entry.name">{{ entry.name }}</span>
        <span class="path" [title]="originalPath(entry)">{{ originalDirectory(entry) }}</span>
        <span class="deleted" [title]="deletedAt(entry)">{{ deletedAt(entry) }}</span>
        <span class="size">{{ entry.type === FileType.DIRECTORY || isGroup(entry) ? '' : formatSize(entry.size) }}</span>
        <span class="revision-label">{{ isGroup(entry) ? revisionCount(entry) + ' revision' + (revisionCount(entry) === 1 ? '' : 's') : revisionLabel(entry) }}</span>
      </div>

      <div *ngIf="afterPx > 0" class="virtual-spacer" [style.height.px]="afterPx" aria-hidden="true"></div>
      <div class="empty" *ngIf="!loading && entries.length === 0" role="status">Recycle Bin is empty.</div>
    </div>
  `,
  styles: [`
    :host { display:flex; flex:1 1 0%; min-width:0; min-height:0; overflow:hidden; }
    .bin-grouped-view { flex:1 1 0%; min-width:0; min-height:0; overflow:auto; background:var(--vsc-editor-background); color:var(--vsc-foreground); }
    .details-header, .entry { display:grid; grid-template-columns:26px var(--details-column-bin-name-width, minmax(180px,1.5fr)) var(--details-column-bin-location-width, minmax(140px,1fr)) var(--details-column-bin-deleted-width, minmax(130px,.8fr)) var(--details-column-bin-size-width, minmax(72px,.45fr)) var(--details-column-bin-revision-width, minmax(82px,.5fr)); align-items:center; min-width:760px; }
    .details-header { position:sticky; top:0; z-index:2; height:28px; border-bottom:1px solid var(--vsc-border); background:var(--vsc-sidebar-background); color:var(--vsc-description-foreground); font-size:12px; }
    .details-header > span, .entry > span { min-width:0; padding:0 6px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .entry { position:relative; min-height:26px; border-radius:3px; font-size:13px; }
    .entry:hover { background:var(--vsc-list-hover-background); }
    .entry.selected { background:var(--vsc-list-active-selection-background); color:var(--vsc-list-active-selection-foreground); }
    .entry.cursor::before { content:''; position:absolute; left:0; top:3px; bottom:3px; width:2px; border-radius:2px; background:var(--vsc-focus-border); }
    .entry.cursor { background:var(--vsc-list-hover-background); }
    .entry.group { font-weight:600; background:var(--vsc-sidebar-background); }
    .entry.revision .name { padding-left:14px; }
    .leading { display:flex; align-items:center; justify-content:flex-start; gap:2px; padding-left:2px !important; }
    .disclosure { width:16px; height:20px; padding:0; border:0; background:transparent; color:inherit; cursor:pointer; }
    .disclosure:hover, .disclosure:active { background:var(--vsc-toolbar-hover); }
    .revision-indent { width:16px; flex:0 0 16px; }
    .icon { padding:0 !important; overflow:visible !important; }
    .path, .deleted, .size, .revision-label { color:var(--vsc-description-foreground); }
    .virtual-spacer { width:1px; pointer-events:none; }
    .empty { padding:18px; color:var(--vsc-description-foreground); }
  `],
})
export class BinGroupedViewComponent {
  readonly FileType = FileType;
  @ViewChild('viewport') viewport?: ElementRef<HTMLElement>;

  get viewportElement(): HTMLElement | null {
    return this.viewport?.nativeElement ?? null;
  }

  setScrollTop(value: number): void {
    if (this.viewport?.nativeElement) {
      this.viewport.nativeElement.scrollTop = Math.max(0, value);
    }
  }
  @Input() entries: FileInfo[] = [];
  @Input() selectedPaths: ReadonlySet<string> = new Set<string>();
  @Input() cursorPath: string | null = null;
  @Input() loading = false;
  @Input() beforePx = 0;
  @Input() afterPx = 0;
  @Input() expandedGroups: ReadonlySet<string> = new Set<string>();

  @Output() scroll = new EventEmitter<Event>();
  @Output() backgroundClick = new EventEmitter<MouseEvent>();
  @Output() backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() entryClick = new EventEmitter<{ entry: FileInfo; event: MouseEvent }>();
  @Output() entryDoubleClick = new EventEmitter<{ entry: FileInfo; event: MouseEvent }>();
  @Output() entryContextMenu = new EventEmitter<{ entry: FileInfo; event: MouseEvent }>();
  @Output() toggleGroup = new EventEmitter<{ entry: FileInfo; event: Event }>();

  trackEntry = (_: number, entry: FileInfo) => (entry as BinFileInfo).binGroupKey && this.isGroup(entry) ? `group:${(entry as BinFileInfo).binGroupKey}` : entry.path;
  isGroup(entry: FileInfo): boolean { return !!(entry as BinFileInfo).binGroupHeader; }
  groupKey(entry: FileInfo): string { return (entry as BinFileInfo).binGroupKey || entry.path; }
  isExpanded(entry: FileInfo): boolean { return this.expandedGroups.has(this.groupKey(entry)); }
  revisionCount(entry: FileInfo): number { return (entry as BinFileInfo).binRevisionCount || 0; }
  revisionLabel(entry: FileInfo): string { return (entry as BinFileInfo).binRevisionLabel || ''; }
  originalPath(entry: FileInfo): string { return (entry as BinFileInfo).binOriginalPath || ''; }
  originalDirectory(entry: FileInfo): string {
    const explicit = (entry as BinFileInfo).binOriginalDirectory;
    if (explicit) return explicit;
    const path = this.originalPath(entry).replace(/\\/g, '/');
    const i = path.lastIndexOf('/');
    return i > 0 ? path.slice(0, i) : path;
  }
  deletedAt(entry: FileInfo): string { return (entry as BinFileInfo).binDeletedAt || ''; }
  formatSize(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes < 0) return '';
    if (bytes < 1024) return `${bytes} B`;
    const units = ['KiB', 'MiB', 'GiB', 'TiB']; let value = bytes / 1024; let unit = units[0];
    for (let i = 1; i < units.length && value >= 1024; i++) { value /= 1024; unit = units[i]; }
    return `${value.toFixed(value >= 10 ? 1 : 2)} ${unit}`;
  }
}