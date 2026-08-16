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
import { WorkspaceService } from '../services/workspace.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { getSourceCapabilities } from '../services/source-capabilities';
import { FileInfo, FileType } from '../shared/protocol';
import type { ViewerFileListItem } from '../shared/protocol-enhanced';
import { ViewerHostSurfaceComponent } from '../viewers/viewer-host-surface.component';
import { ReassemblyAddressBarComponent } from './reassembly-address-bar.component';
import { ReassemblyFilterComponent } from './reassembly-filter.component';
import { ReassemblyNavigationController } from './reassembly-navigation.controller';

type ReassemblyDirectoryView = 'list' | 'grid' | 'details' | 'tree';

interface ReassemblyTreeRow {
  entry: FileInfo;
  depth: number;
  expanded: boolean;
  loading: boolean;
  expandable: boolean;
}

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
  imports: [
    CommonModule,
    ReassemblyAddressBarComponent,
    ReassemblyFilterComponent,
    ViewerHostSurfaceComponent,
  ],
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
          class="nav-button pin"
          [class.pinned]="currentPathPinned"
          [disabled]="!isDirectoryTab || !path"
          (click)="togglePinRequested.emit()"
          [attr.aria-label]="currentPathPinned ? 'Unpin current location' : 'Pin current location'"
          [title]="currentPathPinned ? 'Unpin current location' : 'Pin current location'">
          {{ currentPathPinned ? '★' : '☆' }}
        </button>

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
        <span *ngIf="showHidden" class="hidden-state">hidden visible</span>
        <span *ngIf="clipboard" class="clipboard-state">
          {{ clipboard.operation === 'copy' ? 'Copied' : 'Cut' }}: {{ clipboard.paths.length }}
        </span>
        <span class="hint">Ctrl+L location</span>
      </div>

      <div class="action-bar">
        <div class="view-switcher" role="group" aria-label="Directory view">
          <button type="button" [class.active]="viewMode === 'list'" [attr.aria-pressed]="viewMode === 'list'" (click)="setViewMode('list')">List</button>
          <button type="button" [class.active]="viewMode === 'grid'" [attr.aria-pressed]="viewMode === 'grid'" (click)="setViewMode('grid')">Grid</button>
          <button type="button" [class.active]="viewMode === 'details'" [attr.aria-pressed]="viewMode === 'details'" (click)="setViewMode('details')">Details</button>
          <button type="button" [class.active]="viewMode === 'tree'" [attr.aria-pressed]="viewMode === 'tree'" (click)="setViewMode('tree')">Tree</button>
        </div>
        <span class="action-separator"></span>
        <button type="button" [disabled]="loading || !canCreate" (click)="newFolder()" title="New folder (F7)">New folder</button>
        <button type="button" [disabled]="loading || !canRename" (click)="renameSelected()" title="Rename (F2)">Rename</button>
        <span class="action-separator"></span>
        <button type="button" [disabled]="loading || !hasSelection" (click)="copySelected()" title="Copy (Ctrl+C)">Copy</button>
        <button type="button" [disabled]="loading || !canCut" (click)="cutSelected()" title="Cut (Ctrl+X)">Cut</button>
        <button type="button" [disabled]="loading || !canPaste" (click)="paste()" title="Paste (Ctrl+V)">Paste</button>
        <span class="action-separator"></span>
        <button type="button" class="danger" [disabled]="loading || !canDelete" (click)="deleteSelected()" title="Delete (Delete / F8)">Delete</button>
      </div>

      <app-reassembly-filter
        #filterWidget
        [paneId]="paneId"
        [visible]="filterVisible"
        [value]="filterText"
        [matchCount]="visibleEntries.length"
        [totalCount]="entries.length"
        (valueChange)="setFilter($event)"
        (close)="closeFilter()"
        (next)="nextFilterMatch()"
        (previous)="previousFilterMatch()">
      </app-reassembly-filter>

      <div class="error" *ngIf="error">
        <strong>Directory could not be loaded.</strong>
        <span>{{ error }}</span>
        <button type="button" (click)="refresh()">Retry</button>
      </div>

      <div
        class="tree-view"
        *ngIf="!error && viewMode === 'tree'"
        role="tree"
        [attr.aria-busy]="loading">
        <button
          *ngFor="let row of treeRows; trackBy: trackTreeRow"
          type="button"
          class="tree-row"
          role="treeitem"
          [class.selected]="selectedPaths.has(row.entry.path)"
          [class.cursor]="cursorPath === row.entry.path"
          [attr.aria-level]="row.depth + 1"
          [attr.aria-expanded]="row.expandable ? row.expanded : null"
          [style.padding-left.px]="8 + row.depth * 18"
          (click)="selectFromPointer(row.entry, $event)"
          (dblclick)="onTreeDoubleClick(row)"
          (contextmenu)="onEntryContextMenu(row.entry, $event)"
          (dragstart)="onEntryDragStart(row.entry, $event)"
          (dragend)="onDragEnd()"
          draggable="true">
          <span
            class="tree-expander"
            [class.placeholder]="!row.expandable"
            (click)="toggleTreeRow(row, $event)">
            {{ row.loading ? '…' : (row.expandable ? (row.expanded ? '▾' : '▸') : '') }}
          </span>
          <span class="icon">{{ entryIcon(row.entry) }}</span>
          <span class="name">{{ row.entry.name }}</span>
          <span class="tree-meta" *ngIf="row.entry.type !== FileType.DIRECTORY">
            {{ formatSize(row.entry.size) }}
          </span>
        </button>

        <div class="empty" *ngIf="!loading && treeRows.length === 0">
          This directory is empty.
        </div>
      </div>

      <div class="details-header" *ngIf="!error && viewMode === 'details'">
        <span></span><span>Name</span><span>Type</span><span>Size</span><span>Modified</span>
      </div>

      <div
        class="list"
        *ngIf="!error"
        role="listbox"
        [class.view-list]="viewMode === 'list'"
        [class.view-grid]="viewMode === 'grid'"
        [class.view-details]="viewMode === 'details'"
        [attr.aria-busy]="loading"
        [class.background-drop-target]="backgroundDropActive"
        (contextmenu)="onBackgroundContextMenu($event)"
        (dragover)="onBackgroundDragOver($event)"
        (dragleave)="onBackgroundDragLeave($event)"
        (drop)="onBackgroundDrop($event)">

        <button
          *ngFor="let entry of visibleEntries; trackBy: trackEntry"
          type="button"
          class="entry"
          role="option"
          draggable="true"
          [attr.data-file-entry]="entry.path"
          [attr.aria-selected]="selectedPaths.has(entry.path)"
          [class.cursor]="entry.path === cursorPath"
          [class.selected]="selectedPaths.has(entry.path)"
          (click)="selectFromPointer(entry, $event)"
          (dblclick)="openEntry(entry)"
          (contextmenu)="onEntryContextMenu(entry, $event)"
          (dragstart)="onEntryDragStart(entry, $event)"
          (dragend)="onDragEnd()"
          (dragover)="onEntryDragOver(entry, $event)"
          (dragleave)="onEntryDragLeave(entry)"
          (drop)="onEntryDrop(entry, $event)"
          [class.drop-target]="dragOverPath === entry.path">

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
          <span class="modified">{{ formatModified(entry.modified) }}</span>
        </button>

        <div class="empty" *ngIf="!loading && visibleEntries.length === 0">
          {{ filterText ? 'No matching items.' : 'This directory is empty.' }}
        </div>
      </div>

      <footer class="pane-footer">
        <span class="footer-item" *ngIf="cursorEntry as cursor" [title]="cursor.path">
          <strong>{{ cursor.name }}</strong>
        </span>

        <span class="footer-item" *ngIf="selectedPaths.size">
          {{ selectedPaths.size }} selected
          <ng-container *ngIf="selectedTotalBytes > 0">
            · {{ formatSize(selectedTotalBytes) }}
          </ng-container>
        </span>

        <span class="footer-item" *ngIf="filterText">
          {{ visibleEntries.length }} / {{ entries.length }} visible
        </span>

        <span class="footer-spacer"></span>

        <span class="footer-item capability" [class.disabled-capability]="!sourceCapabilities.create">
          write
        </span>
        <span class="footer-item capability" [class.disabled-capability]="!sourceCapabilities.delete">
          delete
        </span>
      </footer>
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
      grid-template-columns: auto minmax(0, 1fr) 32px 34px;
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

    .nav-button.pin {
      font-size: 16px;
    }

    .nav-button.pin.pinned {
      color: var(--vsc-accent-blue, #4aa3ff);
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

    .clipboard-state,
    .hidden-state {
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

    .view-switcher {
      display: flex;
      gap: 2px;
    }

    .view-switcher button.active {
      background: var(--vsc-list-active-background, #094771);
      color: var(--vsc-list-active-foreground, #ffffff);
    }

    .tree-view {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      padding: 4px 0;
      background: var(--vsc-editor-background, #1e1e1e);
    }

    .tree-row {
      display: grid;
      grid-template-columns: 18px 24px minmax(0, 1fr) auto;
      gap: 4px;
      align-items: center;
      width: 100%;
      min-height: 27px;
      padding-top: 2px;
      padding-right: 8px;
      padding-bottom: 2px;
      border: 0;
      background: transparent;
      color: inherit;
      text-align: left;
      cursor: default;
      font: inherit;
    }

    .tree-row:hover {
      background: var(--vsc-list-hover-background, #2a2d2e);
    }

    .tree-row.selected {
      background: var(--vsc-list-active-background, #094771);
      color: var(--vsc-list-active-foreground, #ffffff);
    }

    .tree-row.cursor {
      outline: 1px solid color-mix(
        in srgb,
        var(--vsc-focus-border, #007fd4) 60%,
        transparent
      );
      outline-offset: -1px;
    }

    .tree-expander {
      display: grid;
      place-items: center;
      width: 18px;
      height: 22px;
      color: var(--vsc-description-foreground, #969696);
      cursor: pointer;
      user-select: none;
    }

    .tree-expander.placeholder {
      cursor: default;
    }

    .tree-meta {
      color: var(--vsc-description-foreground, #969696);
      font-size: 9px;
    }

    .details-header {
      display: grid;
      grid-template-columns: 28px minmax(120px, 1fr) 90px 82px 150px;
      gap: 8px;
      align-items: center;
      min-height: 25px;
      padding: 0 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, #2b2b2b);
      background: var(--vsc-tabbar-background, #252526);
      color: var(--vsc-description-foreground, #969696);
      font-size: 10px;
    }

    .list.view-list .modified {
      display: none;
    }

    .list.view-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(108px, 1fr));
      align-content: start;
      gap: 5px;
      padding: 7px;
    }

    .list.view-grid .entry {
      display: grid;
      grid-template-columns: 1fr;
      grid-template-rows: 42px auto auto auto;
      justify-items: center;
      min-height: 104px;
      padding: 8px 6px;
      text-align: center;
    }

    .list.view-grid .entry .icon { font-size: 30px; }
    .list.view-grid .entry .name {
      width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .list.view-grid .entry .kind,
    .list.view-grid .entry .size,
    .list.view-grid .entry .modified {
      color: var(--vsc-description-foreground, #969696);
      font-size: 9px;
    }

    .list.view-grid .entry .modified { display: none; }

    .list.view-details .entry {
      grid-template-columns: 28px minmax(120px, 1fr) 90px 82px 150px;
    }

    .list.view-details .entry .modified {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, #969696);
      font-size: 10px;
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

    .entry.drop-target {
      outline: 1px solid var(--vsc-accent-blue, #007acc);
      outline-offset: -1px;
      background: color-mix(
        in srgb,
        var(--vsc-accent-blue, #007acc) 18%,
        var(--vsc-list-hover-background, #2a2d2e)
      );
    }

    .list.background-drop-target {
      box-shadow: inset 0 0 0 2px color-mix(
        in srgb,
        var(--vsc-accent-blue, #007acc) 72%,
        transparent
      );
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

    .pane-footer {
      display: flex;
      align-items: center;
      gap: 10px;
      min-height: 27px;
      padding: 0 8px;
      border-top: 1px solid var(--vsc-border-subtle, #2b2b2b);
      background: var(--vsc-tabbar-background, #252526);
      color: var(--vsc-description-foreground, #969696);
      font-size: 10px;
      white-space: nowrap;
      overflow: hidden;
    }

    .footer-item {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .footer-item strong {
      color: var(--vsc-foreground, #cccccc);
      font-weight: 500;
    }

    .footer-spacer {
      flex: 1 1 auto;
      min-width: 8px;
    }

    .footer-item.capability {
      flex: 0 0 auto;
      color: var(--vsc-accent-blue, #4aa3ff);
      text-transform: uppercase;
      letter-spacing: .04em;
      font-size: 9px;
    }

    .footer-item.disabled-capability {
      opacity: .28;
      color: var(--vsc-description-foreground, #969696);
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
        grid-template-columns: auto minmax(0, 1fr) 30px 32px;
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
  @ViewChild('filterWidget') filterWidget?: ReassemblyFilterComponent;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() active = false;
  @Input() showHidden = false;
  @Input() currentPathPinned = false;
  @Output() activateRequested = new EventEmitter<void>();
  @Output() togglePinRequested = new EventEmitter<void>();
  @Output() entryContextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() internalDrop = new EventEmitter<{
    event: DragEvent;
    targetPath: string;
    targetEntry: FileInfo | null;
  }>();

  path = '';
  entries: FileInfo[] = [];
  cursorPath: string | null = null;
  selectedPaths = new Set<string>();
  loading = false;
  error: string | null = null;

  filterVisible = false;
  filterText = '';

  viewMode: ReassemblyDirectoryView = 'list';

  readonly treeExpandedPaths = new Set<string>();
  readonly treeChildren = new Map<string, FileInfo[]>();
  readonly treeLoadingPaths = new Set<string>();
  lastAction = 'startup';
  clipboard: { operation: 'copy' | 'cut'; paths: string[] } | null = null;

  tabs: ReassemblyTab[] = [];
  activeTabId: string | null = null;
  private tabCounter = 0;
  private workspaceTabsReady = false;

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

  dragOverPath: string | null = null;
  backgroundDropActive = false;

  private anchorPath: string | null = null;
  private loadGeneration = 0;
  private readonly navigation = new ReassemblyNavigationController();

  constructor(
    private readonly api: ApiService,
    private readonly directorySource: DirectorySourceService,
    private readonly pathHistory: PathHistoryService,
    private readonly workspaceService: WorkspaceService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get cursorEntry(): FileInfo | null {
    if (!this.cursorPath) return null;
    return this.entries.find(entry => entry.path === this.cursorPath) ?? null;
  }

  get treeRows(): ReassemblyTreeRow[] {
    const rows: ReassemblyTreeRow[] = [];

    const append = (entries: FileInfo[], depth: number): void => {
      for (const entry of entries) {
        const expandable = entry.type === FileType.DIRECTORY;
        const expanded = expandable && this.treeExpandedPaths.has(entry.path);
        rows.push({
          entry,
          depth,
          expanded,
          loading: this.treeLoadingPaths.has(entry.path),
          expandable,
        });

        if (expanded) {
          append(this.treeChildren.get(entry.path) ?? [], depth + 1);
        }
      }
    };

    append(this.visibleEntries, 0);
    return rows;
  }

  get selectedEntries(): FileInfo[] {
    if (this.selectedPaths.size === 0) return [];
    return this.entries.filter(entry => this.selectedPaths.has(entry.path));
  }

  get selectedTotalBytes(): number {
    return this.selectedEntries.reduce((total, entry) => {
      if (entry.type === FileType.DIRECTORY) return total;
      const size = Number(entry.size);
      return total + (Number.isFinite(size) && size > 0 ? size : 0);
    }, 0);
  }

  get visibleEntries(): FileInfo[] {
    const query = this.filterText.trim().toLocaleLowerCase();
    if (!query) return this.entries;

    return this.entries.filter(entry => {
      const name = (entry.name ?? '').toLocaleLowerCase();
      const path = (entry.path ?? '').toLocaleLowerCase();
      return name.includes(query) || path.includes(query);
    });
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

  async restoreWorkspaceState(): Promise<void> {
    this.tabs = [];
    this.activeTabId = null;
    this.filterText = '';
    this.filterVisible = false;
    this.selectedPaths = new Set();
    this.cursorPath = null;
    this.anchorPath = null;
    this.workspaceTabsReady = false;
    this.filterWidget?.deactivateContext();

    const restored = await this.tryRestoreWorkspaceTabs();
    if (!restored) {
      await this.initialize();
    }

    this.cdr.markForCheck();
  }

  newTab(): void {
    const path = this.path || '/';
    const persisted = this.workspaceService.addTab(this.paneId, path, true);

    if (persisted) {
      this.tabs = this.mergePersistedDirectoryTabs();
      this.activeTabId = persisted.id;
      this.lastAction = `new tab: ${persisted.title || this.tabTitle(path)}`;
      this.cdr.markForCheck();
      return;
    }

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
      this.workspaceService.switchTab(this.paneId, tab.id);
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

    const closing = this.tabs[index];
    const wasActive = this.activeTabId === tabId;

    if (closing.kind === 'dir') {
      this.workspaceService.closeTab(this.paneId, tabId);
    }

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
      this.workspaceService.switchTab(this.paneId, next.id);
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

  setViewMode(mode: ReassemblyDirectoryView): void {
    if (this.viewMode === mode) return;

    if (mode === 'tree') {
      this.treeExpandedPaths.clear();
      this.treeChildren.clear();
      this.treeLoadingPaths.clear();
    }
    this.viewMode = mode;
    this.lastAction = `view: ${mode}`;
    try {
      localStorage.setItem(`reassembly.view.${this.paneId}`, mode);
    } catch {}
    queueMicrotask(() => this.ensureCursorVisible());
    this.cdr.markForCheck();
  }

  formatModified(value: unknown): string {
    const numeric = Number(value);
    if (!Number.isFinite(numeric) || numeric <= 0) return '';
    const millis = numeric < 10_000_000_000 ? numeric * 1000 : numeric;
    const date = new Date(millis);
    if (Number.isNaN(date.getTime())) return '';
    return date.toLocaleString([], {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit',
    });
  }

  trackTreeRow(_index: number, row: ReassemblyTreeRow): string {
    return row.entry.path;
  }

  async toggleTreeRow(row: ReassemblyTreeRow, event?: Event): Promise<void> {
    event?.preventDefault();
    event?.stopPropagation();

    if (!row.expandable) return;

    if (this.treeExpandedPaths.has(row.entry.path)) {
      this.treeExpandedPaths.delete(row.entry.path);
      this.lastAction = `collapse: ${row.entry.name}`;
      this.cdr.markForCheck();
      return;
    }

    this.treeExpandedPaths.add(row.entry.path);
    this.lastAction = `expand: ${row.entry.name}`;
    this.cdr.markForCheck();

    if (this.treeChildren.has(row.entry.path)) return;

    this.treeLoadingPaths.add(row.entry.path);
    this.cdr.markForCheck();

    try {
      const listing = await this.directorySource.listDirectory(
        row.entry.path,
        this.showHidden,
      );
      const children = [...(listing.entries ?? [])].sort((a, b) => {
        if (a.type === FileType.DIRECTORY && b.type !== FileType.DIRECTORY) return -1;
        if (a.type !== FileType.DIRECTORY && b.type === FileType.DIRECTORY) return 1;
        return (a.name ?? '').localeCompare(b.name ?? '', undefined, {
          numeric: true,
          sensitivity: 'base',
        });
      });
      this.treeChildren.set(row.entry.path, children);
    } catch (error) {
      this.treeExpandedPaths.delete(row.entry.path);
      this.lastAction = `tree load failed: ${
        error instanceof Error ? error.message : String(error)
      }`;
    } finally {
      this.treeLoadingPaths.delete(row.entry.path);
      this.cdr.markForCheck();
    }
  }

  async onTreeDoubleClick(row: ReassemblyTreeRow): Promise<void> {
    if (row.entry.type === FileType.DIRECTORY) {
      await this.toggleTreeRow(row);
      return;
    }
    this.openEntry(row.entry);
  }

  async treeLeft(): Promise<void> {
    if (this.viewMode !== 'tree') return;
    const rows = this.treeRows;
    const index = rows.findIndex(row => row.entry.path === this.cursorPath);
    if (index < 0) return;
    const row = rows[index];

    if (row.expandable && row.expanded) {
      this.treeExpandedPaths.delete(row.entry.path);
      this.cdr.markForCheck();
      return;
    }

    if (row.depth === 0) return;

    for (let i = index - 1; i >= 0; i -= 1) {
      if (rows[i].depth === row.depth - 1) {
        this.setSingleSelection(rows[i].entry);
        this.ensureCursorVisible();
        this.cdr.markForCheck();
        return;
      }
    }
  }

  async treeRight(): Promise<void> {
    if (this.viewMode !== 'tree') return;
    const rows = this.treeRows;
    const index = rows.findIndex(row => row.entry.path === this.cursorPath);
    if (index < 0) return;
    const row = rows[index];

    if (!row.expandable) return;

    if (!row.expanded) {
      await this.toggleTreeRow(row);
      return;
    }

    const child = rows[index + 1];
    if (child && child.depth === row.depth + 1) {
      this.setSingleSelection(child.entry);
      this.ensureCursorVisible();
      this.cdr.markForCheck();
    }
  }

  focusFilter(): void {
    if (!this.isDirectoryTab) return;
    this.filterVisible = true;
    this.lastAction = 'filter';
    this.cdr.markForCheck();
    this.filterWidget?.focus(true);
    queueMicrotask(() => this.filterWidget?.focus(true));
  }

  setFilter(value: string): void {
    this.filterText = value ?? '';
    this.reconcileSelectionWithVisibleEntries();
    this.lastAction = this.filterText ? `filter: ${this.filterText}` : 'filter cleared';
    this.cdr.markForCheck();
  }

  closeFilter(): void {
    this.filterText = '';
    this.filterVisible = false;
    this.filterWidget?.deactivateContext();
    this.reconcileSelectionWithVisibleEntries();
    this.lastAction = 'filter closed';
    this.cdr.markForCheck();
  }

  nextFilterMatch(): void {
    this.moveFilterCursor(1);
  }

  previousFilterMatch(): void {
    this.moveFilterCursor(-1);
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

  onEntryDragStart(entry: FileInfo, event: DragEvent): void {
    if (!this.isDirectoryTab || !event.dataTransfer) return;

    if (!this.selectedPaths.has(entry.path)) {
      this.setSingleSelection(entry);
      this.cdr.markForCheck();
    }

    const selected = this.entries
      .filter(candidate => this.selectedPaths.has(candidate.path))
      .map(candidate => ({
        path: candidate.path,
        name: candidate.name,
        type: candidate.type,
      }));

    event.dataTransfer.effectAllowed = 'copyMove';
    event.dataTransfer.setData('application/json', JSON.stringify({
      sourcePaneId: this.paneId,
      entries: selected,
    }));
    event.dataTransfer.setData('text/plain', selected.map(item => item.path).join('\n'));

    this.lastAction = `drag ${selected.length}`;
    this.cdr.markForCheck();
  }

  onDragEnd(): void {
    this.dragOverPath = null;
    this.backgroundDropActive = false;
    this.cdr.markForCheck();
  }

  onEntryDragOver(entry: FileInfo, event: DragEvent): void {
    if (entry.type !== FileType.DIRECTORY || !this.hasInternalDrag(event)) return;

    event.preventDefault();
    event.stopPropagation();

    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = event.shiftKey ? 'move' : 'copy';
    }

    this.backgroundDropActive = false;
    this.dragOverPath = entry.path;
    this.cdr.markForCheck();
  }

  onEntryDragLeave(entry: FileInfo): void {
    if (this.dragOverPath !== entry.path) return;
    this.dragOverPath = null;
    this.cdr.markForCheck();
  }

  onEntryDrop(entry: FileInfo, event: DragEvent): void {
    if (entry.type !== FileType.DIRECTORY || !this.hasInternalDrag(event)) return;

    event.preventDefault();
    event.stopPropagation();

    this.dragOverPath = null;
    this.backgroundDropActive = false;
    this.internalDrop.emit({
      event,
      targetPath: entry.path,
      targetEntry: entry,
    });
    this.cdr.markForCheck();
  }

  onBackgroundDragOver(event: DragEvent): void {
    if (!this.hasInternalDrag(event)) return;

    event.preventDefault();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = event.shiftKey ? 'move' : 'copy';
    }

    this.backgroundDropActive = true;
    this.cdr.markForCheck();
  }

  onBackgroundDragLeave(event: DragEvent): void {
    const current = event.currentTarget as HTMLElement | null;
    const related = event.relatedTarget as Node | null;
    if (current && related && current.contains(related)) return;

    this.backgroundDropActive = false;
    this.cdr.markForCheck();
  }

  onBackgroundDrop(event: DragEvent): void {
    if (!this.hasInternalDrag(event)) return;

    event.preventDefault();
    this.backgroundDropActive = false;
    this.dragOverPath = null;
    this.internalDrop.emit({
      event,
      targetPath: this.path,
      targetEntry: null,
    });
    this.cdr.markForCheck();
  }

  private hasInternalDrag(event: DragEvent): boolean {
    const types = Array.from(event.dataTransfer?.types ?? []);
    return types.includes('application/json');
  }

  onEntryContextMenu(entry: FileInfo, event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();

    if (!this.selectedPaths.has(entry.path)) {
      this.setSingleSelection(entry);
      this.lastAction = `context: ${entry.name}`;
      this.cdr.markForCheck();
    }

    this.entryContextMenu.emit({ event, entry });
  }

  onBackgroundContextMenu(event: MouseEvent): void {
    if (event.defaultPrevented) return;
    event.preventDefault();
    this.backgroundContextMenu.emit(event);
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
    const selectable = this.viewMode === 'tree'
      ? this.treeRows.map(row => row.entry)
      : this.visibleEntries;
    this.selectedPaths = new Set(selectable.map(entry => entry.path));
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

    const entry = this.selectedEntries[0];
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

    const entries = this.selectedEntries;
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

  entryIcon(entry: FileInfo): string {
    if (entry.type === FileType.DIRECTORY) return '📁';
    if (entry.type === FileType.ARCHIVE) return '🗜️';
    return '📄';
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
    this.restoreViewPreference();
    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    const restored = await this.tryRestoreWorkspaceTabs();
    if (restored) {
      return;
    }

    try {
      const drives = await this.api.listDrives();
      const initialPath = drives[0]?.path || '/';
      const loaded = await this.load(initialPath, { history: 'record' });
      if (loaded && this.tabs.length === 0) {
        const persisted = this.workspaceService.addTab(this.paneId, this.path, true);
        const tab = persisted
          ? this.fromPersistedTab(persisted)
          : this.createDirectoryTab(this.path);
        this.tabs = [tab];
        this.activeTabId = tab.id;
        this.workspaceTabsReady = !!persisted;
        this.cdr.markForCheck();
      }
    } catch {
      try {
        const loaded = await this.load('/', { history: 'record' });
        if (loaded && this.tabs.length === 0) {
          const persisted = this.workspaceService.addTab(this.paneId, this.path, true);
          const tab = persisted
            ? this.fromPersistedTab(persisted)
            : this.createDirectoryTab(this.path);
          this.tabs = [tab];
          this.activeTabId = tab.id;
          this.workspaceTabsReady = !!persisted;
          this.cdr.markForCheck();
        }
      } catch {
        // load() records the visible error state.
      }
    }
  }

  private restoreViewPreference(): void {
    try {
      const saved = localStorage.getItem(`reassembly.view.${this.paneId}`);
      if (
        saved === 'list' ||
        saved === 'grid' ||
        saved === 'details' ||
        saved === 'tree'
      ) {
        this.viewMode = saved;
      }
    } catch {}
  }

  private async tryRestoreWorkspaceTabs(): Promise<boolean> {
    // WorkspaceService initializes asynchronously from Wails. Give it a short
    // bootstrap window; if it is not ready, normal filesystem fallback remains.
    for (let attempt = 0; attempt < 20; attempt += 1) {
      const persisted = this.workspaceService.getTabs(this.paneId);
      const active = this.workspaceService.getActiveTab(this.paneId);

      if (persisted.length > 0) {
        const dirTabs = persisted
          .filter(tab => (tab.surfaceId ?? tab.kind ?? 'dir') === 'dir')
          .map(tab => this.fromPersistedTab(tab));

        if (dirTabs.length > 0) {
          this.tabs = dirTabs;
          const activeDir = active
            ? dirTabs.find(tab => tab.id === active.id)
            : null;
          const target = activeDir ?? dirTabs[0];

          this.activeTabId = target.id;
          this.workspaceTabsReady = true;
          this.tabCounter = Math.max(this.tabCounter, dirTabs.length);
          this.cdr.markForCheck();

          const loaded = await this.load(target.path, {
            history: 'record',
            syncTab: false,
          });

          if (loaded) {
            this.workspaceService.switchTab(this.paneId, target.id);
            return true;
          }
        }

        return false;
      }

      await new Promise(resolve => setTimeout(resolve, 50));
    }

    return false;
  }

  private fromPersistedTab(tab: any): ReassemblyTab {
    const path =
      typeof tab.location === 'string'
        ? tab.location
        : tab.location?.location || tab.path || '/';

    return {
      id: tab.id,
      kind: 'dir',
      path,
      title: tab.title || this.tabTitle(path),
    };
  }

  private mergePersistedDirectoryTabs(): ReassemblyTab[] {
    const persisted = this.workspaceService.getTabs(this.paneId)
      .filter(tab => (tab.surfaceId ?? tab.kind ?? 'dir') === 'dir')
      .map(tab => this.fromPersistedTab(tab));

    const viewers = this.tabs.filter(tab => tab.kind === 'viewer');
    return [...persisted, ...viewers];
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
      const listing = await this.directorySource.listDirectory(
        requestedPath,
        this.showHidden,
      );
      if (generation !== this.loadGeneration) return false;

      const nextPath = listing.path || requestedPath;
      const pathChanged = this.path && this.path !== nextPath;
      this.path = nextPath;

      if (pathChanged) {
        this.filterText = '';
        this.filterVisible = false;
        this.filterWidget?.deactivateContext();
      }

      if (pathChanged) {
        this.treeExpandedPaths.clear();
        this.treeChildren.clear();
        this.treeLoadingPaths.clear();
      }

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

      if (this.activeTabId && this.activeTab?.kind === 'dir') {
        this.workspaceService.updateTabPath(
          this.paneId,
          this.activeTabId,
          this.path,
        );
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
    const visible = this.viewMode === 'tree'
      ? this.treeRows.map(row => row.entry)
      : this.visibleEntries;
    if (!this.isDirectoryTab || visible.length === 0) return;

    const currentIndex = this.cursorPath
      ? visible.findIndex(entry => entry.path === this.cursorPath)
      : -1;

    const nextIndex = currentIndex < 0
      ? 0
      : Math.max(0, Math.min(visible.length - 1, currentIndex + delta));

    const entry = visible[nextIndex];
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
    const visible = this.visibleEntries;
    const start = visible.findIndex(entry => entry.path === anchorPath);
    const end = visible.findIndex(entry => entry.path === targetPath);

    if (start < 0 || end < 0) {
      const target = visible.find(entry => entry.path === targetPath);
      if (target) this.setSingleSelection(target);
      return;
    }

    const from = Math.min(start, end);
    const to = Math.max(start, end);
    this.selectedPaths = new Set(
      visible.slice(from, to + 1).map(entry => entry.path),
    );
  }

  private reconcileSelectionWithVisibleEntries(): void {
    const visible = this.visibleEntries;
    if (visible.length === 0) {
      this.cursorPath = null;
      this.selectedPaths = new Set();
      this.anchorPath = null;
      return;
    }

    const cursorStillVisible =
      !!this.cursorPath && visible.some(entry => entry.path === this.cursorPath);

    if (!cursorStillVisible) {
      this.setSingleSelection(visible[0]);
      return;
    }

    const visiblePaths = new Set(visible.map(entry => entry.path));
    const nextSelected = new Set(
      Array.from(this.selectedPaths).filter(path => visiblePaths.has(path)),
    );

    if (nextSelected.size === 0 && this.cursorPath) {
      nextSelected.add(this.cursorPath);
    }

    this.selectedPaths = nextSelected;
  }

  private moveFilterCursor(delta: -1 | 1): void {
    const visible = this.visibleEntries;
    if (visible.length === 0) return;

    const currentIndex = this.cursorPath
      ? visible.findIndex(entry => entry.path === this.cursorPath)
      : -1;

    const nextIndex = currentIndex < 0
      ? (delta > 0 ? 0 : visible.length - 1)
      : (currentIndex + delta + visible.length) % visible.length;

    this.setSingleSelection(visible[nextIndex]);
    this.lastAction = delta > 0 ? 'filter next' : 'filter previous';
    this.ensureCursorVisible();
    this.cdr.markForCheck();
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
