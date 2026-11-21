import {
  AfterViewInit,
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Input,
  OnChanges,
  OnDestroy,
  SimpleChanges,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import type { PanelColumnContribution } from '../plugin/plugin-registry-api';
import { FileInfo, FileType } from '../shared/protocol';
import type { DirectoryView } from './pane-component.contract';
import { FileEntryInlineComponent } from './file-entry-inline.component';
import { PaneDetailsResizeHandleDirective } from './pane-details-resize-handle.directive';
import type { VirtualWindow } from '../services/virtual-window';

export interface DirectoryListSurfaceHost {
  onScroll(event: Event): void;
  onBackgroundClick(event: MouseEvent): void;
  onBackgroundContextMenu(event: MouseEvent): void;
  onBackgroundDragOver(event: DragEvent): void;
  onBackgroundDragLeave(event: DragEvent): void;
  onBackgroundDrop(event: DragEvent): void;
  onSelect(entry: FileInfo, event: MouseEvent): void;
  onOpen(entry: FileInfo): void;
  onContextMenu(entry: FileInfo, event: MouseEvent): void;
  onDragStart(entry: FileInfo, event: DragEvent): void;
  onDragEnd(): void;
  onDragOver(entry: FileInfo, event: DragEvent): void;
  onDragLeave(entry: FileInfo): void;
  onDrop(entry: FileInfo, event: DragEvent): void;
  onRenameInput(event: Event): void;
  onRenameKeyDown(entry: FileInfo, event: KeyboardEvent): void;
  onRenameBlur(entry: FileInfo): void;
}

@Component({
  selector: 'app-directory-list-surface',
  standalone: true,
  imports: [
    CommonModule,
    FileEntryInlineComponent,
    PaneDetailsResizeHandleDirective,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="list"
      data-pane-details-root
      [class.view-list]="viewMode === 'list'"
      [class.view-grid]="viewMode === 'grid'"
      [class.view-details]="viewMode === 'details'"
      [class.active-pane]="active"
      [attr.aria-busy]="loading"
      [class.background-drop-target]="backgroundDropActive">

      <div class="details-header-viewport" *ngIf="viewMode === 'details'">
        <div
          #detailsHeader
          class="details-header"
          [style.grid-template-columns]="detailsGridTemplate">
          <span></span>
          <span>Name<i [paneDetailsResizeHandle]="'name'" [paneDetailsMinWidth]="120"></i></span>
          <span>Type<i [paneDetailsResizeHandle]="'type'" [paneDetailsMinWidth]="72"></i></span>
          <span>Size<i [paneDetailsResizeHandle]="'size'" [paneDetailsMinWidth]="64"></i></span>
          <span>Modified<i [paneDetailsResizeHandle]="'modified'" [paneDetailsMinWidth]="110"></i></span>
          <span *ngFor="let column of pluginPanelColumns" [title]="column.label">
            {{ column.label }}<i [paneDetailsResizeHandle]="'plugin-' + column.id" [paneDetailsMinWidth]="72"></i>
          </span>
        </div>
      </div>

      <div
        #listViewport
        class="list-viewport"
        data-pane-content
        tabindex="-1"
        role="listbox"
        [attr.aria-label]="'Files in ' + (path || 'location')"
        (scroll)="onViewportScroll($event)"
        (click)="host.onBackgroundClick($event)"
        (contextmenu)="host.onBackgroundContextMenu($event)"
        (dragover)="host.onBackgroundDragOver($event)"
        (dragleave)="host.onBackgroundDragLeave($event)"
        (drop)="host.onBackgroundDrop($event)">

        <div
          *ngIf="listVirtualWindow.enabled && listVirtualWindow.beforePx > 0"
          class="virtual-spacer list-virtual-spacer"
          [style.height.px]="listVirtualWindow.beforePx"
          aria-hidden="true"></div>

        <div
          *ngFor="let entry of renderedEntries; let virtualIndex=index; trackBy: trackEntry"
          class="entry"
          tabindex="-1"
          role="option"
          [attr.aria-posinset]="renderedEntryStart + virtualIndex + 1"
          [attr.aria-setsize]="visibleEntryCount"
          draggable="true"
          [attr.data-file-entry]="entry.path"
          [attr.aria-selected]="selectedPaths.has(entry.path)"
          [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
          [attr.aria-label]="entryAriaLabel(entry)"
          [class.cursor]="entry.path === cursorPath"
          [class.selected]="selectedPaths.has(entry.path)"
          [class.compare-only]="comparisonStatus(entry.path)?.startsWith('only-')"
          [class.compare-newer]="comparisonStatus(entry.path)?.endsWith('-newer')"
          [class.compare-different]="isComparisonDifferent(entry.path)"
          [class.drop-target]="dragOverPath === entry.path"
          [class.drop-invalid]="dragOverPath === entry.path && dropIntent === 'invalid'"
          [class.drop-copy]="dragOverPath === entry.path && dropIntent === 'copy'"
          [class.drop-move]="dragOverPath === entry.path && dropIntent === 'move'"
          [style.grid-template-columns]="viewMode === 'details' ? detailsGridTemplate : null"
          (click)="host.onSelect(entry, $event)"
          (dblclick)="host.onOpen(entry)"
          (contextmenu)="host.onContextMenu(entry, $event)"
          (dragstart)="host.onDragStart(entry, $event)"
          (dragend)="host.onDragEnd()"
          (dragover)="host.onDragOver(entry, $event)"
          (dragleave)="host.onDragLeave(entry)"
          (drop)="host.onDrop(entry, $event)">

          <span class="icon" aria-hidden="true" [style.color]="entryDisplayColor(entry)">
            {{ entryDisplayIcon(entry) }}
          </span>
          <ng-container *ngIf="renamingPath !== entry.path">
            <app-file-entry-inline
              *ngIf="!perfSimpleMode"
              class="name entry-name"
              [entry]="entry"
              [showIcon]="false">
            </app-file-entry-inline>
            <span *ngIf="perfSimpleMode" class="name entry-name perf-name">{{ entry.name }}</span>
          </ng-container>
          <span
            class="rename-editor"
            *ngIf="renamingPath === entry.path"
            (click)="$event.stopPropagation()"
            (dblclick)="$event.stopPropagation()">
            <input
              class="rename-input"
              [class.rename-input--invalid]="!!renameError"
              aria-label="New name"
              [value]="renameValue"
              [disabled]="renameBusy"
              [attr.data-rename-input]="entry.path"
              [attr.aria-invalid]="renameError ? 'true' : null"
              [attr.aria-describedby]="renameError ? 'rename-error-' + paneId : null"
              (input)="host.onRenameInput($event)"
              (keydown)="host.onRenameKeyDown(entry, $event)"
              (blur)="host.onRenameBlur(entry)" />
            <span *ngIf="renameError" class="rename-error" [id]="'rename-error-' + paneId" role="alert">
              {{ renameError }}
            </span>
          </span>

          <div class="kind" [style.color]="entryDisplayColor(entry)">
            {{ entryDisplayKind(entry) }}
          </div>
          <div class="size">{{ entry.type === FileType.DIRECTORY ? '' : formatSize(entry.size) }}</div>
          <div class="modified">{{ formatModified(entry.modified) }}</div>
          <div class="plugin-column" *ngFor="let column of pluginPanelColumns">
            {{ panelColumnValue(column, entry) }}
          </div>
        </div>

        <div
          *ngIf="listVirtualWindow.enabled && listVirtualWindow.afterPx > 0"
          class="virtual-spacer list-virtual-spacer"
          [style.height.px]="listVirtualWindow.afterPx"
          aria-hidden="true"></div>

        <div class="empty" *ngIf="!loading && visibleEntryCount === 0" role="status">
          <strong>{{ filterText ? 'No matching items.' : 'This directory is empty.' }}</strong>
          <span *ngIf="filterText">Clear the filter to see all items.</span>
          <span *ngIf="!filterText && canCreate && !isInArchive">Use F7 to create a folder or Command Palette for New File.</span>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      flex: 1 1 auto;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    .entry .name,
    .list.view-list .entry .name,
    .list.view-details .entry .name,
    .list.view-grid .entry .name {
      display: block;
      width: 100%;
      justify-self: stretch;
      align-self: center;
      min-width: 0;
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .perf-name {
      pointer-events: none;
    }

    .list {
      display: flex;
      flex-direction: column;
      flex: 1 1 auto;
      min-height: 0;
      height: 100%;
      overflow: hidden;
      background: var(--vsc-editor-background);
    }

    .list-viewport {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      padding: 0;
      overscroll-behavior: none;
      scroll-behavior: auto;
      touch-action: pan-y;
    }

    .details-header-viewport {
      flex: 0 0 auto;
      z-index: 3;
      background: var(--vsc-editor-background);
    }

    .details-header,
    .list.view-list .entry,
    .list.view-details .entry {
      display: grid;
      align-items: center;
      column-gap: 10px;
      min-width: max-content;
      box-sizing: border-box;
    }

    .details-header {
      padding: 6px 10px;
      padding-right: calc(10px + var(--details-scrollbar-gutter, 0px));
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      color: var(--vsc-foreground-dim);
      font-size: 11px;
      font-weight: 600;
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
    }

    .details-header > span:not(:first-child),
    .list.view-list .entry > span:not(:first-child),
    .list.view-details .entry > span:not(:first-child) {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .list.view-list .entry > app-file-entry-inline,
    .list.view-details .entry > app-file-entry-inline,
    .list.view-grid .entry > app-file-entry-inline {
      min-width: 0;
    }

    .list.view-list .entry,
    .list.view-details .entry,
    .list.view-list .list-virtual-spacer,
    .list.view-details .list-virtual-spacer,
    .list.view-details .details-header-viewport {
      padding-left: 10px;
      padding-right: 10px;
      box-sizing: border-box;
    }

    .list.view-list .entry {
      grid-template-columns: 22px minmax(220px, 1fr) 0 0 0;
    }

    .list.view-details .entry {
      padding-top: 6px;
      padding-bottom: 6px;
    }

    .list.view-grid {
      display: flex;
    }

    .list.view-grid .list-viewport {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(108px, 1fr));
      gap: 5px;
      align-content: start;
      padding: 7px;
    }

    .list.view-grid .entry {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: flex-start;
      gap: 8px;
      min-width: 0;
      min-height: 109px;
      padding: 10px 8px;
      text-align: center;
    }

    .list.view-grid .entry .icon { font-size: 30px; }
    .list.view-grid .entry .name {
      white-space: normal;
      text-align: center;
      line-height: 1.3;
    }
    .list.view-grid .entry .kind,
    .list.view-grid .entry .size,
    .list.view-grid .entry .modified,
    .list.view-grid .entry .plugin-column { display: none; }

    .virtual-spacer {
      display: block;
      width: 100%;
      pointer-events: none;
      flex: 0 0 auto;
    }

    .entry {
      position: relative;
      border-radius: 4px;
      color: var(--vsc-foreground);
      cursor: default;
      user-select: none;
      transition: none;
    }

    .entry::before {
      content: '';
      position: absolute;
      left: 0;
      top: 2px;
      bottom: 2px;
      width: 2px;
      border-radius: 2px;
      background: transparent;
      pointer-events: none;
    }

    .entry:hover {
      background: var(--vsc-list-hover-background);
    }

    .entry.cursor {
      background: var(--vsc-list-hover-background);
      box-shadow: none;
      outline: none;
    }

    .entry.cursor::before {
      background: var(--vsc-accent-blue);
    }

    .entry.selected {
      background: var(--vsc-list-active-background);
      color: var(--vsc-list-active-foreground, var(--vsc-foreground));
    }

    .entry.drop-target {
      outline: 1px dashed var(--vsc-focus-border);
      outline-offset: -1px;
    }

    .entry.drop-copy { background: color-mix(in srgb, var(--vsc-accent-blue) 16%, transparent); }
    .entry.drop-move { background: color-mix(in srgb, var(--vsc-focus-border) 16%, transparent); }
    .entry.drop-invalid { background: color-mix(in srgb, var(--vsc-error) 12%, transparent); }

    .list.background-drop-target {
      outline: 2px dashed var(--vsc-focus-border);
      outline-offset: -2px;
    }

    .icon,
    .kind,
    .size,
    .modified,
    .plugin-column {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .rename-editor {
      display: inline-flex;
      align-items: center;
      min-width: 0;
      width: 100%;
    }

    .rename-input {
      width: 100%;
      min-width: 0;
      box-sizing: border-box;
      font: inherit;
      color: var(--vsc-input-foreground);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-input-border);
      border-radius: 4px;
      padding: 4px 6px;
      outline: none;
    }

    .rename-input--invalid {
      border-color: var(--vsc-error);
    }

    .rename-error {
      margin-left: 8px;
      color: var(--vsc-error);
      font-size: 11px;
      white-space: nowrap;
    }

    .empty {
      display: flex;
      flex-direction: column;
      gap: 6px;
      padding: 18px 10px;
      color: var(--vsc-foreground-dim);
    }
  `],
})
export class DirectoryListSurfaceComponent implements AfterViewInit, OnChanges, OnDestroy {
  @ViewChild('listViewport') private viewportRef?: ElementRef<HTMLElement>;
  @ViewChild('detailsHeader') private detailsHeaderRef?: ElementRef<HTMLElement>;
  private viewportResizeObserver?: ResizeObserver;
  readonly FileType = FileType;
  readonly perfSimpleMode = typeof window !== 'undefined' && window.localStorage.getItem('afm-simple-list-surface') === '1';

  @Input({ required: true }) paneId!: 'left' | 'right';
  @Input({ required: true }) path = '';
  @Input({ required: true }) viewMode!: DirectoryView;
  @Input() active = false;
  @Input() loading = false;
  @Input() backgroundDropActive = false;
  @Input() detailsGridTemplate = '';
  @Input() pluginPanelColumns: readonly PanelColumnContribution[] = [];
  @Input() listVirtualWindow!: VirtualWindow;
  @Input() renderedEntries: readonly FileInfo[] = [];
  @Input() renderedEntryStart = 0;
  @Input() visibleEntryCount = 0;
  @Input() selectedPaths: ReadonlySet<string> = new Set();
  @Input() cursorPath: string | null = null;
  @Input() dragOverPath: string | null = null;
  @Input() dropIntent: 'copy' | 'move' | 'invalid' | null = null;
  @Input() renamingPath: string | null = null;
  @Input() renameValue = '';
  @Input() renameError: string | null = null;
  @Input() renameBusy = false;
  @Input() filterText = '';
  @Input() canCreate = false;
  @Input() isInArchive = false;
  @Input({ required: true }) host!: DirectoryListSurfaceHost;

  @Input({ required: true }) entryAriaLabel!: (entry: FileInfo) => string;
  @Input({ required: true }) entryDisplayColor!: (entry: FileInfo) => string | null;
  @Input({ required: true }) entryDisplayIcon!: (entry: FileInfo) => string;
  @Input({ required: true }) entryDisplayKind!: (entry: FileInfo) => string;
  @Input({ required: true }) comparisonStatus!: (path: string) => string | null;
  @Input({ required: true }) isComparisonDifferent!: (path: string) => boolean;
  @Input({ required: true }) formatSize!: (value: number) => string;
  @Input({ required: true }) formatModified!: (value: unknown) => string;
  @Input({ required: true }) panelColumnValue!: (column: PanelColumnContribution, entry: FileInfo) => string;
  @Input({ required: true }) trackEntry!: (_index: number, entry: FileInfo) => string;

  get viewportElement(): HTMLElement | null {
    return this.viewportRef?.nativeElement ?? null;
  }

  ngAfterViewInit(): void {
    this.syncDetailsHeaderViewport();
    this.attachViewportResizeObserver();
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['viewMode'] || changes['detailsGridTemplate'] || changes['pluginPanelColumns']) {
      queueMicrotask(() => this.syncDetailsHeaderViewport());
    }
  }

  ngOnDestroy(): void {
    this.viewportResizeObserver?.disconnect();
    this.viewportResizeObserver = undefined;
  }

  onViewportScroll(event: Event): void {
    this.host.onScroll(event);

    const header = this.detailsHeaderRef?.nativeElement;
    const viewport = event.currentTarget instanceof HTMLElement ? event.currentTarget : null;
    if (header && viewport) {
      header.style.transform = viewport.scrollLeft > 0 ? `translateX(${-viewport.scrollLeft}px)` : '';
    }
    this.syncDetailsHeaderViewport(viewport);
  }

  private attachViewportResizeObserver(): void {
    if (typeof ResizeObserver === 'undefined') {
      return;
    }

    const viewport = this.viewportRef?.nativeElement;
    if (!viewport) {
      return;
    }

    this.viewportResizeObserver?.disconnect();
    this.viewportResizeObserver = new ResizeObserver(() => this.syncDetailsHeaderViewport(viewport));
    this.viewportResizeObserver.observe(viewport);
  }

  private syncDetailsHeaderViewport(viewport = this.viewportRef?.nativeElement): void {
    const header = this.detailsHeaderRef?.nativeElement;
    if (!header || !viewport || this.viewMode !== 'details') {
      return;
    }

    const scrollbarGutter = Math.max(0, viewport.offsetWidth - viewport.clientWidth);
    header.style.transform = viewport.scrollLeft > 0 ? `translateX(${-viewport.scrollLeft}px)` : '';
    header.style.setProperty('--details-scrollbar-gutter', `${scrollbarGutter}px`);
  }
}


