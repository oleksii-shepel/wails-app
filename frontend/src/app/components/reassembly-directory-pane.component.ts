import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnDestroy,
  OnInit,
  Output,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { DirectorySourceService } from '../services/directory-source.service';
import { PanelSourceRegistryService } from '../services/panel-source-registry.service';
import type { ResolvedPanelSource } from '../services/panel-source';
import { ApiService } from '../services/api.service';
import { PathHistoryService } from '../services/path-history.service';
import { WorkspaceService } from '../services/workspace.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { FileClipboardService } from '../services/file-clipboard.service';
import { ReassemblyOperationHistoryService } from '../services/reassembly-operation-history.service';
import { RecentFilesService } from '../services/recent-files.service';
import { PluginManagerService } from '../services/plugin-manager.service';
import type { PanelColumnContribution, PanelColumnContext } from '../plugin/plugin-registry-api';
import { PluginLifecycleCoordinatorService } from '../services/plugin-lifecycle-coordinator.service';
import {
  ArchiveService,
  type ArchiveFrame,
  buildArchiveDisplayPath,
  buildArchiveTabTitle,
  isArchive,
  parseArchivePath,
  sameArchiveLocation,
  normalizeArchiveInnerPath,
  normalizeArchiveLocation,
} from '../services/archive.service';
import type { ViewerArchiveContext } from '../viewers/viewer-archive-context';
import { RemoteConnectionsService } from '../services/remote-connections.service';
import { FileInfo, FileType } from '../shared/protocol';
import type { TabInfo, ViewerFileListItem } from '../shared/protocol-enhanced';
import { ViewerHostSurfaceComponent } from '../viewers/viewer-host-surface.component';
import { SurfaceOutletComponent } from '../surfaces/surface-outlet.component';
import type { SurfaceContext } from '../surfaces/surface-context';
import type { SurfaceCapability } from '../surfaces/surface-contribution';
import { SurfaceRegistryService } from '../surfaces/surface-registry.service';
import { ReassemblyAddressBarComponent } from './reassembly-address-bar.component';
import { ReassemblyFilterComponent } from './reassembly-filter.component';
import { ReassemblyNavigationController } from './reassembly-navigation.controller';
import type { PresentedCommand } from './presented-command';
import {
  createFilenameMaskMatcher,
  entryTypeLabel,
  fileExtension,
  projectEntries,
  sortEntries as projectSortedEntries,
} from './reassembly-entry-model';

type ReassemblyDirectoryView = 'list' | 'grid' | 'details' | 'tree';
type ReassemblySortField = 'name' | 'size' | 'modified' | 'type';
type ReassemblySortDirection = 'asc' | 'desc';

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
      viewMode: ReassemblyDirectoryView;
      sortField: ReassemblySortField;
      sortDirection: ReassemblySortDirection;
      directoriesFirst: boolean;
      expandedPaths: string[];
      filterText: string;
      filterVisible: boolean;
      selectedPaths: string[];
      cursorPath: string | null;
      anchorPath: string | null;
      scrollTop: number;
      entries: FileInfo[];
      navigationEntries: string[];
      navigationIndex: number;
      isClosable: boolean;
      isPinned: boolean;
    }
  | {
      id: string;
      kind: 'viewer';
      path: string;
      title: string;
      targetPath: string;
      fileList: ViewerFileListItem[];
      archiveContext: ViewerArchiveContext | null;
      viewerId: string | null;
      autoFollow: boolean;
      sourcePane: 'left' | 'right' | null;
      sourceTabId: string | null;
      sourceDir: string | null;
      isClosable: boolean;
      isPinned: boolean;
    }
  | {
      id: string;
      kind: 'surface';
      path: string;
      title: string;
      surfaceId: string;
      persisted: TabInfo;
      isClosable: boolean;
      isPinned: boolean;
    };

interface ReassemblyDirTabSession {
  schemaVersion: 3;
  viewMode: ReassemblyDirectoryView;
  sortField: ReassemblySortField;
  sortDirection: ReassemblySortDirection;
  directoriesFirst: boolean;
  expandedPaths: string[];
  filterText: string;
  filterVisible: boolean;
  selectedPaths: string[];
  cursorPath: string | null;
  anchorPath: string | null;
  scrollTop: number;
  entries: FileInfo[];
  navigationEntries: string[];
  navigationIndex: number;
}

const REASSEMBLY_DIR_TAB_SESSION_VERSION = 3;

@Component({
  selector: 'app-reassembly-directory-pane',
  standalone: true,
  imports: [
    CommonModule,
    ReassemblyAddressBarComponent,
    ReassemblyFilterComponent,
    ViewerHostSurfaceComponent,
    SurfaceOutletComponent,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      #paneRoot
      class="pane"
      tabindex="-1"
      role="region"
      [class.active-pane]="active"
      [attr.data-pane-active]="active ? 'true' : null"
      [attr.aria-label]="paneId === 'left' ? 'Left file pane' : 'Right file pane'"
      [attr.aria-busy]="loading"
      [attr.aria-describedby]="paneStatusId"
      (focusin)="activateRequested.emit()"
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
            <span class="tab-kind" aria-hidden="true">{{ tab.kind === 'viewer' ? '◉' : (tab.kind === 'surface' ? '◆' : '▣') }}</span>
            <span class="tab-pin-indicator" *ngIf="tab.isPinned" aria-label="Pinned tab">●</span>
            {{ tab.title }}
          </span>
          <span
            class="tab-close"
            role="button"
            [class.disabled]="tab.isPinned || !tab.isClosable"
            [attr.aria-label]="tab.isPinned ? 'Pinned tab' : (!tab.isClosable ? 'Permanent tab' : 'Close tab')"
            (mousedown)="$event.preventDefault(); $event.stopPropagation()"
            (click)="tab.isPinned ? toggleTabPin(tab.id) : (tab.isClosable ? closeTab(tab.id) : null); $event.stopPropagation()">
            {{ tab.isPinned ? '●' : (tab.isClosable ? '×' : '•') }}
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

      <ng-container *ngIf="activeTab?.kind === 'dir'; else nonDirectorySurface">
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

      <div
        class="status-row"
        [id]="paneStatusId"
        role="status"
        aria-live="polite"
        aria-atomic="true">
        <span *ngIf="loading">Loading {{ path || 'location' }}…</span>
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

        <div class="sort-controls" role="group" aria-label="Sort directory">
          <select
            [value]="sortField"
            (change)="setSortField($any($event.target).value)"
            aria-label="Sort field"
            title="Sort field">
            <option value="name">Name</option>
            <option value="size">Size</option>
            <option value="modified">Modified</option>
            <option value="type">Type</option>
          </select>

          <button
            type="button"
            (click)="toggleSortDirection()"
            [title]="sortDirection === 'asc' ? 'Ascending' : 'Descending'">
            {{ sortDirection === 'asc' ? '↑' : '↓' }}
          </button>

          <button
            type="button"
            [class.active]="directoriesFirst"
            [attr.aria-pressed]="directoriesFirst"
            (click)="toggleDirectoriesFirst()"
            title="Folders first">
            Folders
          </button>
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
        <ng-container *ngIf="pluginToolbarActions.length">
          <span class="action-separator"></span>
          <button
            *ngFor="let action of pluginToolbarActions"
            type="button"
            class="plugin-action"
            [disabled]="loading || !action.enabled"
            [title]="action.shortcut ? action.label + ' (' + action.shortcut + ')' : action.label"
            (click)="runPluginToolbarAction(action)">
            <span aria-hidden="true">◆</span>
            {{ action.label }}
          </button>
        </ng-container>
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

      <div class="error" *ngIf="error" role="alert" aria-live="assertive">
        <strong>Directory could not be loaded.</strong>
        <span>{{ error }}</span>
        <div class="error-actions">
          <button type="button" (click)="refresh()">Retry</button>
          <button type="button" (click)="navigateUp()" [disabled]="atRoot">Parent folder</button>
          <button *ngIf="isRemoteLocation" type="button" (click)="remoteRecoveryRequested.emit()">
            Remote connections
          </button>
        </div>
      </div>

      <div
        #treeViewport
        class="tree-view"
        *ngIf="!error && viewMode === 'tree'"
        tabindex="0"
        role="tree"
        [attr.aria-label]="'Tree view of ' + (path || 'location')"
        [attr.aria-busy]="loading"
        (scroll)="onContentScroll($event)">
        <button
          *ngFor="let row of treeRows; trackBy: trackTreeRow"
          type="button"
          class="tree-row"
          role="treeitem"
          [class.selected]="selectedPaths.has(row.entry.path)"
          [class.cursor]="cursorPath === row.entry.path"
          [attr.aria-level]="row.depth + 1"
          [attr.aria-expanded]="row.expandable ? row.expanded : null"
          [attr.aria-selected]="selectedPaths.has(row.entry.path)"
          [attr.aria-current]="cursorPath === row.entry.path ? 'true' : null"
          [attr.aria-label]="entryAriaLabel(row.entry)"
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

        <div class="empty" *ngIf="!loading && treeRows.length === 0" role="status">
          <strong>This directory is empty.</strong>
          <span *ngIf="canCreate && !isInArchive">Use F7 to create a folder or Command Palette for New File.</span>
        </div>
      </div>

      <div class="details-header" *ngIf="!error && viewMode === 'details'" [style.grid-template-columns]="detailsGridTemplate">
        <span></span><span>Name</span><span>Type</span><span>Size</span><span>Modified</span>
        <span *ngFor="let column of pluginPanelColumns" [title]="column.label">{{ column.label }}</span>
      </div>

      <div
        #listViewport
        class="list"
        *ngIf="!error && viewMode !== 'tree'"
        tabindex="0"
        role="listbox"
        [attr.aria-label]="'Files in ' + (path || 'location')"
        [class.view-list]="viewMode === 'list'"
        [class.view-grid]="viewMode === 'grid'"
        [class.view-details]="viewMode === 'details'"
        [attr.aria-busy]="loading"
        (scroll)="onContentScroll($event)"
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
          [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
          [attr.aria-label]="entryAriaLabel(entry)"
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
          [class.drop-target]="dragOverPath === entry.path"
          [style.grid-template-columns]="viewMode === 'details' ? detailsGridTemplate : null">

          <span class="icon" aria-hidden="true" [style.color]="entryCategory(entry)?.color || null">
            {{ entryDisplayIcon(entry) }}
          </span>

          <span class="name">{{ entry.name }}</span>

          <span class="kind" [style.color]="entryCategory(entry)?.color || null">
            {{ entryDisplayKind(entry) }}
          </span>

          <span class="size">
            {{ entry.type === FileType.DIRECTORY ? '' : formatSize(entry.size) }}
          </span>
          <span class="modified">{{ formatModified(entry.modified) }}</span>
          <span class="plugin-column" *ngFor="let column of pluginPanelColumns">{{ panelColumnValue(column, entry) }}</span>
        </button>

        <div class="empty" *ngIf="!loading && visibleEntries.length === 0" role="status">
          <strong>{{ filterText ? 'No matching items.' : 'This directory is empty.' }}</strong>
          <span *ngIf="filterText">Clear the filter to see all items.</span>
          <span *ngIf="!filterText && canCreate && !isInArchive">Use F7 to create a folder or Command Palette for New File.</span>
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

        <span
          class="footer-item source-kind"
          [title]="'Panel source: ' + activePanelSource.location.sourceId">
          {{ activePanelSource.source.title }}
        </span>

        <span
          class="footer-item capacity"
          *ngIf="driveTotalBytes > 0"
          [title]="driveCapacityTitle">
          Disk: {{ formatSize(driveFreeBytes) }} free
          / {{ formatSize(driveTotalBytes) }}
          <ng-container *ngIf="driveFileSystem">
            · {{ driveFileSystem }}
          </ng-container>
        </span>

        <span class="footer-item capability" [class.disabled-capability]="!sourceCapabilities.create">
          write
        </span>
        <span class="footer-item capability" [class.disabled-capability]="!sourceCapabilities.delete">
          delete
        </span>
      </footer>
      </ng-container>

      <ng-template #nonDirectorySurface>
        <ng-container *ngIf="activeTab?.kind === 'surface'; else viewerSurface">
          <app-surface-outlet
            #surfaceOutlet
            *ngIf="activeSurfaceContext as surfaceContext"
            class="plugin-surface"
            [ctx]="surfaceContext"
            [active]="active">
          </app-surface-outlet>
        </ng-container>
      </ng-template>

      <ng-template #viewerSurface>
        <div class="viewer-shell" *ngIf="activeViewerTab as viewerTab">
          <div class="viewer-toolbar">
            <span class="viewer-label">{{ viewerTab.autoFollow ? 'Preview' : 'Viewer' }}</span>
            <span class="viewer-kind">{{ viewerTab.viewerId || 'Auto' }}</span>
            <span class="viewer-path" [title]="viewerTab.archiveContext?.entryPath || viewerTab.targetPath">
              {{ viewerTab.archiveContext?.entryPath || viewerTab.targetPath }}
            </span>

            <button
              type="button"
              *ngIf="viewerTab.sourceTabId || viewerTab.sourceDir"
              (click)="returnToViewerSource()"
              title="Return to source folder">
              Folder
            </button>

            <span class="viewer-separator"></span>

            <button type="button" [disabled]="!viewerSupports('showPrevious')" (click)="viewerInvoke('showPrevious')" title="Previous item (Left)">‹</button>
            <button type="button" [disabled]="!viewerSupports('showNext')" (click)="viewerInvoke('showNext')" title="Next item (Right)">›</button>
            <button type="button" [disabled]="!viewerSupports('zoomOut')" (click)="viewerInvoke('zoomOut')" title="Zoom out (Ctrl+-)">−</button>
            <button type="button" [disabled]="!viewerSupports('zoomIn')" (click)="viewerInvoke('zoomIn')" title="Zoom in (Ctrl++)">+</button>
            <button type="button" [disabled]="!viewerSupports('resetZoom')" (click)="viewerInvoke('resetZoom')" title="Fit (Ctrl+0)">Fit</button>
            <button type="button" [disabled]="!viewerSupports('zoom100')" (click)="viewerInvoke('zoom100')" title="Actual size (Ctrl+1)">1:1</button>

            <span class="viewer-separator"></span>

            <button
              type="button"
              [disabled]="!!viewerTab.archiveContext"
              (click)="openActiveViewerExternally()"
              title="Open with external application">
              External
            </button>
            <button
              type="button"
              [disabled]="viewerTab.isPinned || !viewerTab.isClosable"
              (click)="closeActiveTab()"
              title="Close viewer (Ctrl+W)">
              ×
            </button>
          </div>

          <app-viewer-host-surface
            #viewerHost
            [targetPath]="viewerTab.targetPath"
            [fileList]="viewerTab.fileList"
            [archiveContext]="viewerTab.archiveContext"
            [viewerId]="viewerTab.viewerId">
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
    
      overscroll-behavior: none;}

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
    
      overscroll-behavior-x: none;
      scroll-behavior: auto;
      touch-action: pan-x;}

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

    .sort-controls {
      display: flex;
      align-items: center;
      gap: 2px;
    }

    .sort-controls select {
      height: 26px;
      max-width: 110px;
      padding: 0 5px;
      border: 1px solid var(--vsc-border, #454545);
      border-radius: 3px;
      background: var(--vsc-input-background, #181818);
      color: var(--vsc-foreground, #cccccc);
      font: inherit;
    }

    .sort-controls button.active {
      background: var(--vsc-list-active-background, #094771);
      color: var(--vsc-list-active-foreground, #ffffff);
    }

    .tree-view {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      padding: 4px 0;
      background: var(--vsc-editor-background, #1e1e1e);
    
      overscroll-behavior: none;
      scroll-behavior: auto;
      touch-action: pan-y;}

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
    
      overscroll-behavior: none;
      scroll-behavior: auto;
      touch-action: pan-y;}

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

    .footer-item.source-kind {
      flex: 0 0 auto;
      opacity: .68;
    }

    .footer-item.capacity {
      flex: 0 0 auto;
      max-width: 280px;
      color: var(--vsc-foreground, #cccccc);
      font-variant-numeric: tabular-nums;
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

    .viewer-kind {
      flex: 0 0 auto;
      padding: 1px 5px;
      border: 1px solid var(--vsc-border, #454545);
      border-radius: 3px;
      color: var(--vsc-description-foreground, #969696);
      font-size: 9px;
      text-transform: uppercase;
    }

    .viewer-separator {
      width: 1px;
      height: 18px;
      background: var(--vsc-border, #454545);
      flex: 0 0 auto;
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
    .list:focus-visible,.tree-view:focus-visible,.pane:focus-visible{
      outline:1px solid var(--vsc-focus-border,#007fd4);
      outline-offset:-1px;
    }
    .empty{display:grid;gap:4px;place-items:center;align-content:center;min-height:96px;text-align:center}
    .empty strong{font-weight:500;color:var(--vsc-foreground,#ccc)}
    .error-actions{display:flex;gap:6px;margin-top:7px}
  `],
})
export class ReassemblyDirectoryPaneComponent implements OnInit, OnDestroy {
  readonly FileType = FileType;

  @ViewChild('addressBar') addressBar?: ReassemblyAddressBarComponent;
  @ViewChild('viewerHost') viewerHost?: ViewerHostSurfaceComponent;
  @ViewChild('surfaceOutlet') surfaceOutlet?: SurfaceOutletComponent;
  @ViewChild('filterWidget') filterWidget?: ReassemblyFilterComponent;
  @ViewChild('paneRoot') paneRoot?: ElementRef<HTMLElement>;
  @ViewChild('listViewport') listViewport?: ElementRef<HTMLElement>;
  @ViewChild('treeViewport') treeViewport?: ElementRef<HTMLElement>;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() active = false;
  @Input() showHidden = false;
  @Input() currentPathPinned = false;
  @Input() pluginToolbarActions: readonly PresentedCommand[] = [];
  @Output() activateRequested = new EventEmitter<void>();
  @Output() remoteRecoveryRequested = new EventEmitter<void>();
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

  driveFreeBytes = 0;
  driveTotalBytes = 0;
  driveFileSystem = '';

  viewMode: ReassemblyDirectoryView = 'list';
  sortField: ReassemblySortField = 'name';
  sortDirection: ReassemblySortDirection = 'asc';
  directoriesFirst = true;

  readonly treeExpandedPaths = new Set<string>();
  readonly treeChildren = new Map<string, FileInfo[]>();
  readonly treeLoadingPaths = new Set<string>();
  lastAction = 'startup';

  tabs: ReassemblyTab[] = [];
  activeTabId: string | null = null;
  private tabCounter = 0;
  private workspaceTabsReady = false;
  private workspaceRestoreGeneration = 0;
  private interactionGeneration = 0;
  private scrollPersistTimer: ReturnType<typeof setTimeout> | null = null;

  get paneStatusId(): string {
    return `reassembly-${this.paneId}-status`;
  }

  focusContent(): void {
    if (this.activeTab?.kind === 'surface' && this.surfaceOutlet) {
      this.surfaceOutlet.focus();
      return;
    }

    const target =
      this.activeTab?.kind !== 'dir'
        ? this.paneRoot?.nativeElement
        : this.viewMode === 'tree'
          ? this.treeViewport?.nativeElement
          : this.listViewport?.nativeElement;

    target?.focus({ preventScroll: true });
  }

  entryAriaLabel(entry: FileInfo): string {
    const kind = entryTypeLabel(entry);

    const size =
      entry.type === FileType.DIRECTORY
        ? ''
        : `, ${this.formatSize(entry.size)}`;

    return `${entry.name}, ${kind}${size}`;
  }

  entryCategory(entry: FileInfo) {
    if (entry.type === FileType.DIRECTORY || entry.type === FileType.ARCHIVE) {
      return null;
    }

    return this.pluginManager.resolveFileCategory(
      entry.path,
      (entry as FileInfo & { mimeType?: string | null }).mimeType ?? null,
    );
  }

  entryDisplayIcon(entry: FileInfo): string {
    if (entry.type === FileType.DIRECTORY) return '📁';
    if (entry.type === FileType.ARCHIVE) return '🗜️';
    return this.entryCategory(entry)?.icon || '📄';
  }

  entryDisplayKind(entry: FileInfo): string {
    if (entry.type === FileType.DIRECTORY) return 'Folder';
    if (entry.type === FileType.ARCHIVE) return 'Archive';
    return this.entryCategory(entry)?.category || 'File';
  }

  get activeSurfaceId(): string {
    const tab = this.activeTab;
    if (!tab) return 'dir';
    if (tab.kind === 'surface') return tab.surfaceId;
    return tab.kind;
  }

  get activeSurfaceCapabilities(): ReadonlySet<SurfaceCapability> {
    const tab = this.activeSurfaceTab;
    const context = this.activeSurfaceContext;
    if (!tab || !context) return new Set<SurfaceCapability>();

    return this.surfaceRegistry.getCapabilities({
      tab: tab.persisted,
      context,
    });
  }

  surfaceSupports(capability: SurfaceCapability): boolean {
    if (this.isDirectoryTab) {
      return capability === 'navigate' ||
        capability === 'selection' ||
        capability === 'fileOperations' ||
        capability === 'preview';
    }

    if (this.activeTab?.kind === 'viewer') {
      return capability === 'viewerControls';
    }

    return this.activeSurfaceCapabilities.has(capability);
  }

  surfaceSupportsMethod(method: string): boolean {
    return this.activeTab?.kind === 'surface' &&
      !!this.surfaceOutlet?.supports(method);
  }

  surfaceInvoke(method: string, ...args: unknown[]): unknown {
    if (this.activeTab?.kind !== 'surface') return undefined;
    return this.surfaceOutlet?.invoke(method, ...args);
  }

  get pinnableLocation(): string {
    const tab = this.activeTab;
    if (!tab) return '';

    if (tab.kind === 'surface') {
      return tab.path;
    }

    if (tab.kind === 'dir') {
      return tab.path || this.path;
    }

    return '';
  }

  get pinnableSurfaceId(): string {
    const tab = this.activeTab;
    if (!tab) return 'dir';
    if (tab.kind === 'surface') return tab.surfaceId;
    return 'dir';
  }

  get pinnableTitle(): string {
    const tab = this.activeTab;
    return tab?.title?.trim() || this.pinnableLocation;
  }

  get driveCapacityTitle(): string {
    if (this.driveTotalBytes <= 0) return '';
    return [
      `${this.formatSize(this.driveFreeBytes)} free`,
      `${this.formatSize(this.driveTotalBytes)} total`,
      this.driveFileSystem,
    ].filter(Boolean).join(' · ');
  }

  private async refreshDriveCapacity(path: string): Promise<void> {
    if (
      !path ||
      this.isInArchive ||
      this.isRemoteLocation ||
      this.activePanelSource.source.kind === 'results' ||
      this.activePanelSource.source.kind === 'virtual'
    ) {
      this.driveFreeBytes = 0;
      this.driveTotalBytes = 0;
      this.driveFileSystem = '';
      return;
    }

    try {
      const capacity = await this.api.getPathCapacity(path);

      this.driveFreeBytes = Number(capacity.freeSpace) || 0;
      this.driveTotalBytes = Number(capacity.totalSpace) || 0;
      this.driveFileSystem = capacity.fileSystem || '';
    } catch {
      this.driveFreeBytes = 0;
      this.driveTotalBytes = 0;
      this.driveFileSystem = '';
    }
  }

  get activeTab(): ReassemblyTab | null {
    if (!this.activeTabId) return null;
    return this.tabs.find(tab => tab.id === this.activeTabId) ?? null;
  }

  get activeViewerTab(): Extract<ReassemblyTab, { kind: 'viewer' }> | null {
    const tab = this.activeTab;
    return tab?.kind === 'viewer' ? tab : null;
  }

  get activeSurfaceTab(): Extract<ReassemblyTab, { kind: 'surface' }> | null {
    const tab = this.activeTab;
    return tab?.kind === 'surface' ? tab : null;
  }

  get activeSurfaceContext(): SurfaceContext | null {
    const tab = this.activeSurfaceTab;
    if (!tab) return null;

    return {
      paneId: this.paneId,
      tab: tab.persisted,
      surfaceId: tab.surfaceId,
      kind: 'plugin',
      location: tab.path,
    };
  }

  get mountedViewer(): any {
    return this.viewerHost?.mountedViewer ?? null;
  }

  get viewerTargetPath(): string | null {
    return this.activeViewerTab?.targetPath ?? null;
  }

  get viewerArchiveContext(): ViewerArchiveContext | null {
    return this.activeViewerTab?.archiveContext ?? null;
  }

  viewerSupports(method: string): boolean {
    return typeof this.mountedViewer?.[method] === 'function';
  }

  viewerInvoke(method: string): void {
    const fn = this.mountedViewer?.[method];
    if (typeof fn === 'function') fn.call(this.mountedViewer);
  }

  get isDirectoryTab(): boolean {
    return this.activeTab?.kind === 'dir';
  }

  get hasSelection(): boolean {
    return this.selectedEntries.length > 0;
  }

  get visibleSelectedCount(): number {
    if (!this.isDirectoryTab) return 0;
    const visible = new Set(this.selectableEntries().map(entry => entry.path));
    let count = 0;
    for (const path of this.selectedPaths) {
      if (visible.has(path)) count += 1;
    }
    return count;
  }

  get hiddenSelectedCount(): number {
    return Math.max(0, this.selectedPaths.size - this.visibleSelectedCount);
  }

  get isInArchive(): boolean {
    return this.archiveStack.length > 0;
  }

  get currentArchiveFrame(): ArchiveFrame | null {
    return this.archiveStack[this.archiveStack.length - 1] ?? null;
  }

  get activePanelSource(): ResolvedPanelSource {
    return this.panelSources.resolve({
      path: this.path,
      entries: this.entries,
      inArchive: this.isInArchive,
      archiveTopLevel:
        (this.currentArchiveFrame?.nestedChain.length ?? 0) === 0,
    });
  }

  get panelColumnContext(): PanelColumnContext {
    const resolved = this.activePanelSource;
    return {
      currentDirectory: this.path,
      sourceId: resolved.location.sourceId,
      sourceKind: resolved.source.kind,
    };
  }

  get pluginPanelColumns(): PanelColumnContribution[] {
    if (this.viewMode !== 'details') return [];
    return this.pluginManager.resolvePanelColumns(this.panelColumnContext);
  }

  get detailsGridTemplate(): string {
    const extra = this.pluginPanelColumns.map(column => column.width?.trim() || '120px');
    return ['28px', 'minmax(120px, 1fr)', '90px', '82px', '150px', ...extra].join(' ');
  }

  panelColumnValue(column: PanelColumnContribution, entry: FileInfo): string {
    const value = column.value(entry, this.panelColumnContext);
    return value == null ? '' : String(value);
  }

  get sourceCapabilities() {
    return this.activePanelSource.capabilities;
  }

  get isRemoteLocation(): boolean {
    return this.activePanelSource.source.kind === 'remote';
  }

  get canCreate(): boolean {
    return this.isDirectoryTab &&
      !this.isInArchive &&
      !!this.path &&
      this.sourceCapabilities.create;
  }

  get canRename(): boolean {
    return this.isDirectoryTab &&
      !this.isInArchive &&
      this.selectedPaths.size === 1 &&
      this.sourceCapabilities.rename;
  }

  get canDelete(): boolean {
    return this.isDirectoryTab &&
      this.hasSelection &&
      (this.isInArchive
        ? this.archiveMutationAllowed
        : this.sourceCapabilities.delete);
  }

  get canCut(): boolean {
    return this.isDirectoryTab &&
      this.hasSelection &&
      (this.isInArchive
        ? this.archiveMutationAllowed
        : this.sourceCapabilities.read && this.sourceCapabilities.delete);
  }

  get archiveMutationAllowed(): boolean {
    return this.isInArchive &&
      (this.currentArchiveFrame?.nestedChain.length ?? 1) === 0;
  }

  get canPaste(): boolean {
    return this.isDirectoryTab && !!this.clipboard?.paths.length && this.sourceCapabilities.create;
  }

  readonly archiveStack: ArchiveFrame[] = [];
  private archiveExitPath = '';

  dragOverPath: string | null = null;
  backgroundDropActive = false;

  private anchorPath: string | null = null;
  private loadGeneration = 0;
  private readonly navigation = new ReassemblyNavigationController(
    path => this.directorySource.isPathInsideArchive(path)
      ? normalizeArchiveLocation(path)
      : path.trim(),
  );

  constructor(
    private readonly api: ApiService,
    private readonly directorySource: DirectorySourceService,
    private readonly panelSources: PanelSourceRegistryService,
    private readonly pathHistory: PathHistoryService,
    private readonly workspaceService: WorkspaceService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly remoteConnections: RemoteConnectionsService,
    private readonly archiveService: ArchiveService,
    private readonly fileClipboard: FileClipboardService,
    private readonly operationHistory: ReassemblyOperationHistoryService,
    private readonly recentFiles: RecentFilesService,
    private readonly pluginManager: PluginManagerService,
    private readonly pluginLifecycle: PluginLifecycleCoordinatorService,
    private readonly surfaceRegistry: SurfaceRegistryService,
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

  get clipboard() {
    return this.fileClipboard.payload;
  }

  get selectedEntries(): FileInfo[] {
    if (this.selectedPaths.size === 0) return [];
    return this.entries.filter(entry =>
      entry.path !== '__parent__' &&
      this.selectedPaths.has(entry.path),
    );
  }

  get selectedTotalBytes(): number {
    return this.selectedEntries.reduce((total, entry) => {
      if (entry.type === FileType.DIRECTORY) return total;
      const size = Number(entry.size);
      return total + (Number.isFinite(size) && size > 0 ? size : 0);
    }, 0);
  }

  get sortedEntries(): FileInfo[] {
    return projectSortedEntries(this.entries, {
      sortField: this.sortField,
      sortDirection: this.sortDirection,
      directoriesFirst: this.directoriesFirst,
    });
  }

  get visibleEntries(): FileInfo[] {
    return projectEntries(this.entries, {
      sortField: this.sortField,
      sortDirection: this.sortDirection,
      directoriesFirst: this.directoriesFirst,
      filterText: this.filterText,
    });
  }

  get atRoot(): boolean {
    if (this.isInArchive) return false;
    return !this.path || this.panelSources.isRootPath(
      this.path,
      this.entries,
    );
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

  ngOnDestroy(): void {
    if (this.scrollPersistTimer) {
      clearTimeout(this.scrollPersistTimer);
      this.scrollPersistTimer = null;
    }
    this.syncActiveDirSessionState();
  }

  focusLocation(): void {
    if (!this.isDirectoryTab) return;
    this.addressBar?.beginEdit();
  }

  async restoreWorkspaceState(): Promise<void> {
    const generation = ++this.workspaceRestoreGeneration;

    this.syncActiveDirSessionState();
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
    if (generation !== this.workspaceRestoreGeneration) return;

    if (!restored) {
      await this.initialize();
      if (generation !== this.workspaceRestoreGeneration) return;
    }

    this.cdr.markForCheck();
  }

  async openSurfaceTab(
    surfaceId: string,
    title?: string,
    location?: string,
  ): Promise<void> {
    const id = surfaceId.trim();
    if (!id || id === 'dir' || id === 'viewer') return;

    try {
      await this.pluginLifecycle.activateByEvent(`onSurface:${id}`);
    } catch {
      // Surface resolution below fails closed if activation did not register it.
    }

    this.syncActiveDirSessionState();

    const targetLocation =
      location?.trim() ||
      `surface://${encodeURIComponent(id)}`;

    const persisted = this.workspaceService.addSurfaceTab(
      this.paneId,
      targetLocation,
      id,
      {
        activate: true,
        closable: true,
        title: title?.trim() || id,
      },
    );
    if (!persisted) return;

    this.tabs = this.mergePersistedTabs();
    this.activeTabId = persisted.id;
    this.loading = false;
    this.error = null;
    this.clearArchiveRuntime();
    this.lastAction = `surface: ${title?.trim() || id}`;
    this.cdr.markForCheck();
  }

  async openRecentFilesPanel(limit = 50): Promise<void> {
    const entries = this.recentFiles.recent(limit);
    if (entries.length === 0) {
      await this.confirmDialog.alert({
        title: 'Recent Files',
        message: 'No recently opened files are available yet.',
      });
      return;
    }

    await this.openResultsPanel(
      'Recent Files',
      entries,
      { origin: 'recent-files', kind: 'recent-files' },
    );
  }

  async openResultsPanel(
    title: string,
    entries: readonly FileInfo[],
    metadata: {
      origin?: string;
      rootPath?: string;
      query?: string;
      kind?: string;
    } = {},
  ): Promise<void> {
    if (entries.length === 0) return;

    this.syncActiveDirSessionState();

    const path = this.panelSources.createResultsPanel(
      title,
      entries,
      metadata,
    );

    const persisted = this.workspaceService.addTab(
      this.paneId,
      path,
      true,
    );

    if (persisted) {
      this.workspaceService.updateTabPathAndTitle(
        this.paneId,
        persisted.id,
        path,
        title,
      );
      this.tabs = this.mergePersistedTabs();
      this.activeTabId = persisted.id;

      const active = this.activeTab;
      if (active?.kind === 'dir') {
        this.applyDirSessionBeforeLoad(active);
      }

      await this.load(path, {
        history: 'record',
        syncTab: false,
      });

      this.lastAction = `results: ${title}`;
      this.cdr.markForCheck();
      return;
    }

    const tab = {
      ...this.createDirectoryTab(path),
      title,
    };
    this.tabs = [...this.tabs, tab];
    this.activeTabId = tab.id;
    this.applyDirSessionBeforeLoad(tab);
    await this.load(path, {
      history: 'record',
      syncTab: false,
    });
    this.lastAction = `results: ${title}`;
    this.cdr.markForCheck();
  }

  newTab(): void {
    const path = this.path || '/';
    const persisted = this.workspaceService.addTab(this.paneId, path, true);

    if (persisted) {
      this.tabs = this.mergePersistedTabs();
      this.activeTabId = persisted.id;
      const active = this.activeTab;
      if (active?.kind === 'dir') {
        this.applyDirSessionBeforeLoad(active);
      }
      this.lastAction = `new tab: ${persisted.title || this.tabTitle(path)}`;
      this.cdr.markForCheck();
      return;
    }

    const tab = this.createDirectoryTab(path);
    this.tabs = [...this.tabs, tab];
    this.activeTabId = tab.id;
    this.applyDirSessionBeforeLoad(tab);
    this.lastAction = `new tab: ${tab.title}`;
    this.cdr.markForCheck();
  }

  async activateTab(tabId: string): Promise<void> {
    if (tabId === this.activeTabId) return;

    this.syncActiveDirSessionState();

    const tab = this.tabs.find(candidate => candidate.id === tabId);
    if (!tab) return;

    this.activeTabId = tabId;
    this.lastAction = `tab: ${tab.title}`;

    if (tab.kind === 'dir') {
      this.applyDirSessionBeforeLoad(tab);
      this.restoreArchiveStateForTab(tab.id, tab.path);
      this.workspaceService.switchTab(this.paneId, tab.id);

      const interactionGeneration = this.interactionGeneration;
      const loaded = await this.load(tab.path, {
        history: 'record',
        syncTab: false,
        restoreSession: true,
      });

      if (loaded) {
        await this.finalizeDirSessionAfterLoad(
          tab,
          interactionGeneration,
        );
      }
    } else {
      this.workspaceService.switchTab(this.paneId, tab.id);
      this.loading = false;
      this.error = null;
    }

    this.cdr.markForCheck();
  }

  async closeTab(tabId: string): Promise<void> {
    const index = this.tabs.findIndex(tab => tab.id === tabId);
    if (index < 0) return;

    const requested = this.tabs[index];
    if (requested.isPinned) {
      this.lastAction = `pinned tab kept: ${requested.title}`;
      this.cdr.markForCheck();
      return;
    }

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

    this.workspaceService.closeTab(this.paneId, tabId);

    if (closing.kind === 'dir') {
      try {
        localStorage.removeItem(this.dirSessionStorageKey(tabId));
      } catch {
        // Best-effort cleanup only.
      }

      this.workspaceService.clearArchiveState(
        this.paneId,
        tabId,
      );
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
      this.restoreArchiveStateForTab(next.id, next.path);
      this.workspaceService.switchTab(this.paneId, next.id);
      await this.load(next.path, { history: 'record', syncTab: false });
    } else {
      this.workspaceService.switchTab(this.paneId, next.id);
      this.loading = false;
      this.error = null;
    }
  }

  async returnToViewerSource(): Promise<void> {
    const viewer = this.activeViewerTab;
    if (!viewer) return;

    if (
      viewer.sourceTabId &&
      this.tabs.some(tab => tab.id === viewer.sourceTabId)
    ) {
      await this.activateTab(viewer.sourceTabId);
      return;
    }

    if (!viewer.sourceDir) return;

    const existing = this.tabs.find(
      tab => tab.kind === 'dir' && this.samePath(tab.path, viewer.sourceDir!),
    );
    if (existing) {
      await this.activateTab(existing.id);
      return;
    }

    const tab = this.createDirectoryTab(viewer.sourceDir);
    this.tabs = [...this.tabs, tab];
    this.activeTabId = tab.id;
    await this.load(tab.path, { history: 'record', syncTab: false });
  }

  async openActiveViewerExternally(): Promise<void> {
    const viewer = this.activeViewerTab;
    if (!viewer || viewer.archiveContext) return;

    try {
      await this.api.openFile(viewer.targetPath);
      this.lastAction = `external: ${viewer.targetPath}`;
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Open Externally Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async closeActiveTab(): Promise<void> {
    if (!this.activeTabId) return;
    await this.closeTab(this.activeTabId);
  }

  async nextTab(): Promise<void> {
    if (this.tabs.length < 2 || !this.activeTabId) return;
    const currentIndex = this.tabs.findIndex(tab => tab.id === this.activeTabId);
    const nextIndex = currentIndex < 0 ? 0 : (currentIndex + 1) % this.tabs.length;
    await this.activateTab(this.tabs[nextIndex].id);
  }

  async closeOtherTabs(): Promise<void> {
    const activeId = this.activeTabId;
    if (!activeId || this.tabs.length <= 1) return;

    const closable = this.tabs
      .filter(tab =>
        tab.id !== activeId &&
        tab.isClosable &&
        !tab.isPinned,
      )
      .map(tab => tab.id);

    for (const tabId of closable) {
      await this.closeTab(tabId);
    }

    this.lastAction =
      closable.length > 0
        ? `close other tabs: ${closable.length}`
        : 'no closable other tabs';
    this.cdr.markForCheck();
  }

  async closeTabsToRight(): Promise<void> {
    const activeId = this.activeTabId;
    if (!activeId) return;

    const index = this.tabs.findIndex(tab => tab.id === activeId);
    if (index < 0) return;

    const closable = this.tabs
      .slice(index + 1)
      .filter(tab => tab.isClosable && !tab.isPinned)
      .map(tab => tab.id);

    for (const tabId of closable) {
      await this.closeTab(tabId);
    }

    this.lastAction =
      closable.length > 0
        ? `close tabs to right: ${closable.length}`
        : 'no closable tabs to right';
    this.cdr.markForCheck();
  }

  async previousTab(): Promise<void> {
    if (this.tabs.length < 2 || !this.activeTabId) return;
    const currentIndex = this.tabs.findIndex(tab => tab.id === this.activeTabId);
    const previousIndex =
      currentIndex <= 0 ? this.tabs.length - 1 : currentIndex - 1;
    await this.activateTab(this.tabs[previousIndex].id);
  }

  async jumpToTab(index: number): Promise<void> {
    const target = this.tabs[index];
    if (!target) return;
    await this.activateTab(target.id);
  }

  toggleActiveTabPin(): void {
    if (!this.activeTabId) return;
    this.toggleTabPin(this.activeTabId);
  }

  toggleTabPin(tabId: string): void {
    const index = this.tabs.findIndex(tab => tab.id === tabId);
    if (index < 0) return;

    const current = this.tabs[index];
    const pinned = !current.isPinned;
    const updated: ReassemblyTab = { ...current, isPinned: pinned };

    this.tabs = this.tabs.map(tab => tab.id === tabId ? updated : tab);

    this.workspaceService.toggleTabPin(this.paneId, tabId);

    this.lastAction = `${pinned ? 'pin' : 'unpin'} tab: ${current.title}`;
    this.cdr.markForCheck();
  }

  /**
   * Reloads the pane's tab list from WorkspaceService after a workspace-level
   * tab move/reorder operation. This keeps the reassembly runtime deliberately
   * thin: WorkspaceService remains the source of truth for persistent tab order.
   */
  async reloadWorkspaceTabs(preferredTabId?: string): Promise<void> {
    this.syncActiveDirSessionState();

    const merged = this.mergePersistedTabs();
    if (merged.length === 0) {
      await this.restoreWorkspaceState();
      return;
    }

    this.tabs = merged;

    const requested =
      preferredTabId &&
      this.tabs.some(tab => tab.id === preferredTabId)
        ? preferredTabId
        : this.tabs.find(tab => tab.id === this.activeTabId)?.id ??
          this.tabs[0].id;

    this.activeTabId = requested;

    const active = this.activeTab;
    if (active?.kind === 'dir') {
      this.applyDirSessionBeforeLoad(active);

      const interactionGeneration = this.interactionGeneration;
      const loaded = await this.load(active.path, {
        history: 'record',
        syncTab: false,
        restoreSession: true,
      });

      if (loaded) {
        await this.finalizeDirSessionAfterLoad(
          active,
          interactionGeneration,
        );
      }
    } else if (active?.kind === 'viewer') {
      this.workspaceService.switchTab(this.paneId, active.id);
      this.loading = false;
      this.error = null;
    }

    this.cdr.markForCheck();
  }

  exportActiveTabId(): string | null {
    return this.activeTab?.id ?? null;
  }

  exportActiveDirectoryTabId(): string | null {
    const active = this.activeTab;
    return active?.kind === 'dir' ? active.id : null;
  }

  exportTabCount(): number {
    return this.tabs.length;
  }

  trackTab(_index: number, tab: ReassemblyTab): string {
    return tab.id;
  }

  setSortField(field: string): void {
    this.markUserInteraction();
    if (
      field !== 'name' &&
      field !== 'size' &&
      field !== 'modified' &&
      field !== 'type'
    ) {
      return;
    }

    this.sortField = field;
    this.syncActiveDirSessionState();
    this.lastAction = `sort: ${field}`;
    this.cdr.markForCheck();
  }

  toggleSortDirection(): void {
    this.markUserInteraction();
    this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
    this.syncActiveDirSessionState();
    this.lastAction = `sort: ${this.sortDirection}`;
    this.cdr.markForCheck();
  }

  toggleDirectoriesFirst(): void {
    this.markUserInteraction();
    this.directoriesFirst = !this.directoriesFirst;
    this.syncActiveDirSessionState();
    this.lastAction = this.directoriesFirst
      ? 'folders first'
      : 'mixed folders/files';
    this.cdr.markForCheck();
  }

  setViewMode(mode: ReassemblyDirectoryView): void {
    this.markUserInteraction();

    if (this.isInArchive && mode === 'tree') {
      this.lastAction = 'tree view unavailable inside archive';
      this.cdr.markForCheck();
      return;
    }

    if (this.viewMode === mode) return;

    if (mode === 'tree') {
      this.treeExpandedPaths.clear();
      this.treeChildren.clear();
      this.treeLoadingPaths.clear();
    }
    this.viewMode = mode;
    this.syncActiveDirSessionState();
    this.lastAction = `view: ${mode}`;
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

  async toggleTreeRow(
    row: ReassemblyTreeRow,
    event?: Event,
    userInitiated = true,
  ): Promise<void> {
    event?.preventDefault();
    event?.stopPropagation();
    if (userInitiated) this.markUserInteraction();

    if (!row.expandable) return;

    if (this.treeExpandedPaths.has(row.entry.path)) {
      this.treeExpandedPaths.delete(row.entry.path);
      this.syncActiveDirSessionState();
      this.lastAction = `collapse: ${row.entry.name}`;
      this.cdr.markForCheck();
      return;
    }

    this.treeExpandedPaths.add(row.entry.path);
    this.syncActiveDirSessionState();
    this.lastAction = `expand: ${row.entry.name}`;
    this.cdr.markForCheck();

    if (this.treeChildren.has(row.entry.path)) return;

    this.treeLoadingPaths.add(row.entry.path);
    this.cdr.markForCheck();

    try {
      const listing = await this.panelSources.listDirectory(
        row.entry.path,
        this.showHidden,
      );
      const children = this.sortEntries([...(listing.entries ?? [])]);
      this.treeChildren.set(row.entry.path, children);
    } catch (error) {
      this.treeExpandedPaths.delete(row.entry.path);
      this.syncActiveDirSessionState();
      this.lastAction = `tree load failed: ${
        error instanceof Error ? error.message : String(error)
      }`;
    } finally {
      this.treeLoadingPaths.delete(row.entry.path);
      this.cdr.markForCheck();
    }
  }

  async onTreeDoubleClick(row: ReassemblyTreeRow): Promise<void> {
    if (this.isInArchive) {
      this.openEntry(row.entry);
      return;
    }

    if (row.entry.type === FileType.DIRECTORY) {
      await this.toggleTreeRow(row);
      return;
    }

    this.openEntry(row.entry);
  }

  async treeLeft(): Promise<void> {
    if (this.viewMode !== 'tree') return;
    this.markUserInteraction();
    const rows = this.treeRows;
    const index = rows.findIndex(row => row.entry.path === this.cursorPath);
    if (index < 0) return;
    const row = rows[index];

    if (row.expandable && row.expanded) {
      this.treeExpandedPaths.delete(row.entry.path);
      this.syncActiveDirSessionState();
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
    this.markUserInteraction();
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
    this.markUserInteraction();
    this.filterText = value ?? '';
    this.reconcileSelectionWithVisibleEntries();
    this.lastAction = this.filterText ? `filter: ${this.filterText}` : 'filter cleared';
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
  }

  closeFilter(): void {
    this.markUserInteraction();
    this.filterText = '';
    this.filterVisible = false;
    this.filterWidget?.deactivateContext();
    this.reconcileSelectionWithVisibleEntries();
    this.lastAction = 'filter closed';
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
  }

  nextFilterMatch(): void {
    this.moveFilterCursor(1);
  }

  previousFilterMatch(): void {
    this.moveFilterCursor(-1);
  }

  private async activatePluginsForLocation(path: string): Promise<void> {
    const value = path.trim();
    if (!value) return;

    const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec(value);
    const scheme = match?.[1]?.toLowerCase() ?? 'filesystem';

    try {
      await this.pluginLifecycle.activateByEvent(`onScheme:${scheme}`);
    } catch {
      // Plugin lifecycle records activation failure/quarantine independently.
    }
  }

  navigateTo(path: string): void {
    this.syncActiveDirSessionState();
    if (!this.isDirectoryTab) return;
    this.lastAction = `navigate: ${path}`;

    void (async () => {
      await this.activatePluginsForLocation(path);
      await this.load(path, { history: 'record' });
    })();
  }

  async refresh(): Promise<void> {
    if (!this.isDirectoryTab || !this.path) return;
    this.lastAction = 'refresh';

    if (this.isInArchive) {
      await this.refreshArchive();
      return;
    }

    await this.load(this.path, {
      preserveCursor: true,
      history: 'none',
    });
  }

  async navigateUp(): Promise<void> {
    this.syncActiveDirSessionState();
    if (!this.isDirectoryTab || !this.path) return;

    if (this.isInArchive) {
      await this.navigateArchiveUp();
      return;
    }

    if (this.atRoot) return;

    const parent = this.panelSources.getParentPath(
      this.path,
      this.entries,
    );
    if (!parent || parent === this.path) return;

    this.lastAction = 'parent directory';
    await this.load(parent, { history: 'record' });
  }

  async goBack(): Promise<void> {
    this.syncActiveDirSessionState();
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
    this.syncActiveDirSessionState();
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

    if (entry.path === '__parent__') {
      void this.navigateUp();
      return;
    }

    if (this.isInArchive) {
      if (entry.type === FileType.DIRECTORY) {
        void this.navigateArchiveInto(entry.path);
        return;
      }

      if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
        void this.openNestedArchiveEntry(entry);
        return;
      }

      this.openViewer(entry);
      return;
    }

    if (entry.type === FileType.DIRECTORY) {
      this.lastAction = `open directory: ${entry.name}`;
      void this.load(entry.path, { history: 'record' });
      return;
    }

    if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
      this.lastAction = `open archive: ${entry.name}`;
      void this.load(entry.path, { history: 'record' });
      return;
    }

    this.openViewer(entry);
  }

  onEntryDragStart(entry: FileInfo, event: DragEvent): void {
    if (!this.isDirectoryTab || !event.dataTransfer) return;

    if (!this.selectedPaths.has(entry.path)) {
      this.setSingleSelection(entry);
      this.cdr.markForCheck();
    }

    if (entry.path === '__parent__') {
      event.preventDefault();
      return;
    }

    const selected = this.selectedEntries.map(candidate => ({
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
    if (
      entry.path === '__parent__' ||
      entry.type !== FileType.DIRECTORY ||
      !this.hasInternalDrag(event)
    ) return;

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
    if (
      entry.path === '__parent__' ||
      entry.type !== FileType.DIRECTORY ||
      !this.hasInternalDrag(event)
    ) return;

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

    if (entry.path === '__parent__') {
      this.cursorPath = entry.path;
      this.selectedPaths = new Set();
      this.anchorPath = null;
      this.lastAction = 'context: parent';
      this.syncActiveDirSessionState();
      this.cdr.markForCheck();
      this.entryContextMenu.emit({ event, entry });
      return;
    }

    if (!this.selectedPaths.has(entry.path)) {
      this.setSingleSelection(entry);
      this.lastAction = `context: ${entry.name}`;
      this.syncActiveDirSessionState();
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
    this.markUserInteraction();
    this.cursorPath = entry.path;

    if (entry.path === '__parent__') {
      this.selectedPaths = new Set();
      this.anchorPath = null;
      this.lastAction = 'select parent';
      this.syncActiveDirSessionState();
      this.cdr.markForCheck();
      return;
    }

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
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
  }

  openViewer(
    entry: FileInfo,
    viewerId: string | null = null,
  ): void {
    if (entry.type === FileType.DIRECTORY) return;
    this.recentFiles.record(entry);

    const existing = this.tabs.find(tab =>
      tab.kind === 'viewer' &&
      !tab.autoFollow &&
      tab.viewerId === viewerId &&
      this.samePath(
        tab.archiveContext?.entryPath ?? tab.targetPath,
        entry.path,
      )
    );

    if (existing) {
      void this.activateTab(existing.id);
      this.lastAction = `viewer: ${entry.name}`;
      return;
    }

    const fileList = this.buildViewerFileList();
    const archiveContext = this.viewerArchiveContextFor(entry);

    const persisted = this.workspaceService.addViewerTab(
      this.paneId,
      entry.path,
      {
        activate: true,
        title: entry.name,
        viewerId: viewerId ?? undefined,
        autoFollow: false,
        closable: true,
        sourcePane: this.paneId,
        sourceTabId: this.activeTabId ?? undefined,
        sourceDir: this.path,
        fileList: archiveContext ? [] : fileList,
        archiveContext: archiveContext ?? undefined,
      },
    );

    if (persisted) {
      this.tabs = this.mergePersistedTabs();
      this.activeTabId = persisted.id;
      this.workspaceTabsReady = true;
    } else {
      const tab: Extract<ReassemblyTab, { kind: 'viewer' }> = {
        id: `${this.paneId}-tab-${++this.tabCounter}`,
        kind: 'viewer',
        path: entry.path,
        title: entry.name,
        targetPath: entry.path,
        fileList: archiveContext ? [] : fileList,
        archiveContext,
        viewerId,
        autoFollow: false,
        sourcePane: this.paneId,
        sourceTabId: this.activeTabId,
        sourceDir: this.path,
        isClosable: true,
        isPinned: false,
      };

      this.tabs = [...this.tabs, tab];
      this.activeTabId = tab.id;
    }

    this.lastAction = `viewer: ${entry.name}`;
    this.cdr.markForCheck();
  }

  openQuickView(entry: FileInfo): void {
    if (entry.type === FileType.DIRECTORY) return;

    const archiveContext = this.viewerArchiveContextFor(entry);
    const fileList = archiveContext ? [] : this.buildViewerFileList();

    if (!archiveContext) {
      const preview = this.tabs.find(
        tab => tab.kind === 'viewer' && tab.autoFollow,
      );

      if (preview) {
        this.workspaceService.updateViewerTabTargetPath(
          this.paneId,
          preview.id,
          entry.path,
          {
            sourcePane: this.paneId,
            sourceTabId: this.activeTabId ?? '',
            sourceDir: this.path,
            fileList,
          },
        );

        this.tabs = this.mergePersistedTabs();
        this.activeTabId = preview.id;
        this.workspaceService.switchTab(this.paneId, preview.id);
        this.lastAction = `preview: ${entry.name}`;
        this.cdr.markForCheck();
        return;
      }
    }

    const persisted = this.workspaceService.addViewerTab(
      this.paneId,
      entry.path,
      {
        activate: true,
        title: archiveContext ? entry.name : 'Preview',
        autoFollow: !archiveContext,
        closable: true,
        sourcePane: this.paneId,
        sourceTabId: this.activeTabId ?? undefined,
        sourceDir: this.path,
        fileList,
        archiveContext: archiveContext ?? undefined,
      },
    );

    if (persisted) {
      this.tabs = this.mergePersistedTabs();
      this.activeTabId = persisted.id;
      this.workspaceTabsReady = true;
      this.lastAction = `preview: ${entry.name}`;
      this.cdr.markForCheck();
      return;
    }

    this.openViewer(entry);
  }

  openSelectedWithViewer(viewerId: string): void {
    const entry = this.cursorEntry;
    if (!entry || entry.type === FileType.DIRECTORY) return;
    this.openViewer(entry, viewerId);
  }

  private viewerArchiveContextFor(
    entry: FileInfo,
  ): ViewerArchiveContext | null {
    const frame = this.currentArchiveFrame;
    if (!frame) return null;

    return {
      fsPath: this.archiveStack[0]?.fsPath ?? frame.fsPath,
      nestedChain: [...(frame.nestedChain ?? [])],
      entryPath: normalizeArchiveInnerPath(entry.path),
    };
  }

  private buildViewerFileList(): ViewerFileListItem[] {
    return this.sortedEntries
      .filter(candidate =>
        candidate.type !== FileType.DIRECTORY,
      )
      .map(candidate => ({
        path: candidate.path,
        name: candidate.name,
      }));
  }

  toggleCurrentSelection(): void {
    if (!this.isDirectoryTab) return;

    const entry = this.cursorEntry;
    if (!entry) return;

    this.markUserInteraction();

    const next = new Set(this.selectedPaths);
    if (next.has(entry.path)) {
      next.delete(entry.path);
    } else {
      next.add(entry.path);
    }

    this.selectedPaths = next;
    this.anchorPath = entry.path;
    this.lastAction = `toggle selection: ${entry.name}`;
    this.syncActiveDirSessionState();

    // Commander-style marking advances to the next item when possible.
    this.moveCursorAfterSelectionToggle(1);
    this.cdr.markForCheck();
  }

  extendSelection(delta: -1 | 1): void {
    if (!this.isDirectoryTab) return;

    const entries = this.selectableEntries();
    if (entries.length === 0) return;

    this.markUserInteraction();

    let index = this.cursorPath
      ? entries.findIndex(entry => entry.path === this.cursorPath)
      : -1;

    if (index < 0) index = 0;

    if (!this.anchorPath) {
      this.anchorPath = entries[index]?.path ?? null;
    }

    const nextIndex = Math.max(
      0,
      Math.min(entries.length - 1, index + delta),
    );
    const next = entries[nextIndex];
    if (!next) return;

    this.cursorPath = next.path;
    if (this.anchorPath) {
      this.selectRange(this.anchorPath, next.path);
    }

    this.lastAction = delta < 0
      ? 'extend selection up'
      : 'extend selection down';

    this.syncActiveDirSessionState();
    this.ensureCursorVisible();
    this.cdr.markForCheck();
  }

  async selectByMaskPrompt(): Promise<void> {
    const mask = (await this.confirmDialog.prompt({
      title: 'Select by Mask',
      message: 'Enter a filename mask. Use * and ? wildcards.',
      confirmText: 'Select',
      historyKey: 'reassembly.selection.mask.select',
    }))?.trim();

    if (!mask) return;
    this.applySelectionMask(mask, true);
  }

  async unselectByMaskPrompt(): Promise<void> {
    const mask = (await this.confirmDialog.prompt({
      title: 'Unselect by Mask',
      message: 'Enter a filename mask. Use * and ? wildcards.',
      confirmText: 'Unselect',
      historyKey: 'reassembly.selection.mask.unselect',
    }))?.trim();

    if (!mask) return;
    this.applySelectionMask(mask, false);
  }

  invertSelection(): void {
    if (!this.isDirectoryTab) return;

    this.markUserInteraction();

    const entries = this.selectableEntries();
    const visiblePaths = new Set(entries.map(entry => entry.path));
    const next = new Set(
      Array.from(this.selectedPaths).filter(path => !visiblePaths.has(path)),
    );

    for (const entry of entries) {
      if (!this.selectedPaths.has(entry.path)) {
        next.add(entry.path);
      }
    }

    this.selectedPaths = next;

    const firstVisibleSelected = entries.find(entry => next.has(entry.path));
    if (firstVisibleSelected) {
      this.cursorPath = firstVisibleSelected.path;
      this.anchorPath = firstVisibleSelected.path;
    } else if (entries.length > 0) {
      this.cursorPath = entries[0].path;
      this.anchorPath = this.cursorPath;
    }

    this.lastAction = `invert visible selection: ${next.size}`;
    this.syncActiveDirSessionState();
    this.ensureCursorVisible();
    this.cdr.markForCheck();
  }

  selectSameExtension(): void {
    if (!this.isDirectoryTab) return;

    const current = this.cursorEntry;
    if (!current) return;

    const extension = this.extensionOf(current.name);
    if (!extension) return;

    this.markUserInteraction();

    const next = new Set(this.selectedPaths);
    for (const entry of this.selectableEntries()) {
      if (this.extensionOf(entry.name) === extension) {
        next.add(entry.path);
      }
    }

    this.selectedPaths = next;
    this.syncCursorFromSelection(this.selectableEntries());
    this.lastAction = `select *.${extension}: ${next.size}`;
    this.syncActiveDirSessionState();
    this.ensureCursorVisible();
    this.cdr.markForCheck();
  }

  selectFiles(): void {
    this.applyTypeSelection(
      entry => entry.type !== FileType.DIRECTORY,
      true,
      'select files',
    );
  }

  selectFolders(): void {
    this.applyTypeSelection(
      entry => entry.type === FileType.DIRECTORY,
      true,
      'select folders',
    );
  }

  unselectFiles(): void {
    this.applyTypeSelection(
      entry => entry.type !== FileType.DIRECTORY,
      false,
      'unselect files',
    );
  }

  unselectFolders(): void {
    this.applyTypeSelection(
      entry => entry.type === FileType.DIRECTORY,
      false,
      'unselect folders',
    );
  }

  selectSameType(): void {
    const current = this.cursorEntry;
    if (!current) return;

    this.applyTypeSelection(
      entry => entry.type === current.type,
      true,
      `select ${entryTypeLabel(current)}`,
    );
  }

  moveCursorHome(): void {
    this.moveCursorToIndex(0, 'cursor first');
  }

  moveCursorEnd(): void {
    const entries = this.selectableEntries();
    this.moveCursorToIndex(entries.length - 1, 'cursor last');
  }

  moveCursorPage(delta: -1 | 1): void {
    const entries = this.selectableEntries();
    if (entries.length === 0) return;

    const viewport =
      this.viewMode === 'tree'
        ? this.treeViewport?.nativeElement
        : this.listViewport?.nativeElement;

    const rowHeight = this.viewMode === 'grid' ? 110 : 28;
    const pageSize = Math.max(
      1,
      Math.floor((viewport?.clientHeight ?? 280) / rowHeight),
    );

    const currentIndex = this.cursorPath
      ? entries.findIndex(entry => entry.path === this.cursorPath)
      : 0;

    this.moveCursorToIndex(
      Math.max(0, currentIndex) + pageSize * delta,
      delta < 0 ? 'page up' : 'page down',
    );
  }

  private applyTypeSelection(
    predicate: (entry: FileInfo) => boolean,
    select: boolean,
    action: string,
  ): void {
    if (!this.isDirectoryTab) return;

    const entries = this.selectableEntries();
    const next = new Set(this.selectedPaths);

    for (const entry of entries) {
      if (!predicate(entry)) continue;
      if (select) {
        next.add(entry.path);
      } else {
        next.delete(entry.path);
      }
    }

    this.markUserInteraction();
    this.selectedPaths = next;

    if (!this.cursorPath || !entries.some(entry => entry.path === this.cursorPath)) {
      const firstVisibleSelected = entries.find(entry => next.has(entry.path));
      this.cursorPath = firstVisibleSelected?.path ?? entries[0]?.path ?? null;
    }

    this.anchorPath = this.cursorPath;
    this.lastAction = `${action}: ${next.size}`;
    this.syncActiveDirSessionState();
    this.ensureCursorVisible();
    this.cdr.markForCheck();
  }

  private applySelectionMask(mask: string, select: boolean): void {
    if (!this.isDirectoryTab) return;

    const matcher = this.createMaskMatcher(mask);
    const next = new Set(this.selectedPaths);

    for (const entry of this.selectableEntries()) {
      if (!matcher(entry.name ?? '')) continue;

      if (select) {
        next.add(entry.path);
      } else {
        next.delete(entry.path);
      }
    }

    this.markUserInteraction();
    this.selectedPaths = next;
    this.syncCursorFromSelection(this.selectableEntries());
    this.lastAction = `${select ? 'select' : 'unselect'} mask ${mask}: ${next.size}`;
    this.syncActiveDirSessionState();
    this.ensureCursorVisible();
    this.cdr.markForCheck();
  }

  private createMaskMatcher(mask: string): (name: string) => boolean {
    return createFilenameMaskMatcher(mask);
  }

  private extensionOf(name: string): string {
    return fileExtension(name);
  }

  private selectableEntries(): FileInfo[] {
    return this.viewMode === 'tree'
      ? this.treeRows.map(row => row.entry)
      : this.visibleEntries;
  }

  private syncCursorFromSelection(entries: FileInfo[]): void {
    if (entries.length === 0) {
      this.cursorPath = null;
      this.anchorPath = null;
      return;
    }

    const firstSelected = entries.find(entry =>
      this.selectedPaths.has(entry.path),
    );

    if (firstSelected) {
      this.cursorPath = firstSelected.path;
      this.anchorPath = this.anchorPath ?? firstSelected.path;
      return;
    }

    this.cursorPath = entries[0].path;
    this.anchorPath = this.cursorPath;
  }

  private moveCursorToIndex(index: number, action: string): void {
    if (!this.isDirectoryTab) return;

    const entries = this.selectableEntries();
    if (entries.length === 0) return;

    this.markUserInteraction();

    const clamped = Math.max(
      0,
      Math.min(entries.length - 1, index),
    );
    const entry = entries[clamped];

    this.cursorPath = entry.path;
    this.anchorPath = entry.path;

    if (this.selectedPaths.size <= 1) {
      this.selectedPaths = new Set([entry.path]);
    }

    this.lastAction = action;
    this.syncActiveDirSessionState();
    this.ensureCursorVisible();
    this.cdr.markForCheck();
  }

  private moveCursorAfterSelectionToggle(delta: -1 | 1): void {
    const entries = this.selectableEntries();
    if (entries.length === 0 || !this.cursorPath) return;

    const index = entries.findIndex(
      entry => entry.path === this.cursorPath,
    );
    if (index < 0) return;

    const nextIndex = index + delta;
    if (nextIndex < 0 || nextIndex >= entries.length) return;

    this.cursorPath = entries[nextIndex].path;
    this.ensureCursorVisible();
    this.syncActiveDirSessionState();
  }

  selectPaths(paths: readonly string[]): void {
    if (!this.isDirectoryTab) return;

    this.markUserInteraction();

    const wanted = new Set(paths);
    const selectable = this.viewMode === 'tree'
      ? this.treeRows.map(row => row.entry)
      : this.visibleEntries;

    const selected = selectable
      .filter(entry => wanted.has(entry.path))
      .map(entry => entry.path);

    this.selectedPaths = new Set(selected);
    this.cursorPath = selected[0] ?? null;
    this.anchorPath = this.cursorPath;
    this.lastAction = `selected ${selected.length} comparison item${selected.length === 1 ? '' : 's'}`;

    this.syncActiveDirSessionState();
    this.ensureCursorVisible();
    this.cdr.markForCheck();
  }



  selectAll(): void {
    if (!this.isDirectoryTab) return;
    this.markUserInteraction();
    const selectable = (
      this.viewMode === 'tree'
        ? this.treeRows.map(row => row.entry)
        : this.visibleEntries
    ).filter(entry => entry.path !== '__parent__');
    this.selectedPaths = new Set(selectable.map(entry => entry.path));
    this.lastAction = 'select all';
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
  }

  clearSelection(): void {
    this.markUserInteraction();
    this.selectedPaths = new Set();
    this.anchorPath = null;
    this.lastAction = 'clear selection';
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
  }

  runPluginToolbarAction(action: PresentedCommand): void {
    if (!action.enabled) return;
    void action.execute();
    this.lastAction = `plugin: ${action.label}`;
    this.cdr.markForCheck();
  }

  private async writeTextClipboard(text: string): Promise<boolean> {
    const value = text.trim();
    if (!value) return false;

    try {
      await navigator.clipboard.writeText(value);
      return true;
    } catch {
      const runtime = (window as unknown as {
        runtime?: {
          ClipboardSetText?: (text: string) => Promise<boolean>;
        };
      }).runtime;

      if (typeof runtime?.ClipboardSetText === 'function') {
        return await runtime.ClipboardSetText(value);
      }
      return false;
    }
  }

  async copySelectedPaths(): Promise<void> {
    const entries = this.selectedEntries.length > 0
      ? this.selectedEntries
      : this.cursorEntry
        ? [this.cursorEntry]
        : [];

    if (entries.length === 0) return;

    const copied = await this.writeTextClipboard(
      entries.map(entry => entry.path).join('\n'),
    );
    this.lastAction = copied
      ? `copy path: ${entries.length}`
      : 'copy path failed';
    this.cdr.markForCheck();
  }

  async copySelectedNames(): Promise<void> {
    const entries = this.selectedEntries.length > 0
      ? this.selectedEntries
      : this.cursorEntry
        ? [this.cursorEntry]
        : [];

    if (entries.length === 0) return;

    const copied = await this.writeTextClipboard(
      entries.map(entry => entry.name).join('\n'),
    );
    this.lastAction = copied
      ? `copy name: ${entries.length}`
      : 'copy name failed';
    this.cdr.markForCheck();
  }

  async duplicateSelected(): Promise<void> {
    if (
      !this.isDirectoryTab ||
      this.isInArchive ||
      !this.sourceCapabilities.create
    ) return;

    const entries = this.selectedEntries;
    if (entries.length === 0) return;

    try {
      const items = entries.map(entry => ({
        source: entry.path,
        destination: this.path,
        exactDestination: false,
      }));

      const result = await this.api.batchCopy(items, true);
      const affected = result.affectedPaths ?? [];

      if (
        result.failed === 0 &&
        result.skipped === 0 &&
        affected.length === result.succeeded &&
        affected.length > 0
      ) {
        this.operationHistory.record({
          label: `Duplicate ${affected.length} item${
            affected.length === 1 ? '' : 's'
          }`,
          undo: async () => {
            const deletion = await this.api.batchDelete(
              affected.map(item => item.destination),
              true,
              false,
            );
            if (deletion.failed > 0) {
              throw new Error('Undo duplicate failed.');
            }
          },
          redo: async () => {
            const redone = await this.api.batchCopy(
              affected.map(item => ({
                source: item.source,
                destination: item.destination,
                exactDestination: true,
              })),
              true,
            );
            if (redone.failed > 0 || redone.skipped > 0) {
              throw new Error('Redo duplicate failed.');
            }
          },
        });
      }

      this.lastAction = `duplicate: ${result.succeeded}/${result.total}`;
      await this.refresh();

      if (affected.length === 1) {
        this.selectPath(affected[0].destination);
      }
    } catch (error) {
      await this.showOperationError('Duplicate', error);
    }
  }

  async createLinkForSelected(
    kind: 'symbolic' | 'hard',
  ): Promise<void> {
    if (
      !this.isDirectoryTab ||
      this.isInArchive ||
      !this.sourceCapabilities.create
    ) return;

    const entry = this.cursorEntry ?? this.selectedEntries[0];
    if (!entry || entry.path === '__parent__') return;

    if (kind === 'hard' && entry.type === FileType.DIRECTORY) {
      await this.confirmDialog.alert({
        title: 'Hard Link',
        message: 'Hard links to directories are not supported.',
      });
      return;
    }

    const suffix = kind === 'symbolic' ? ' link' : ' hardlink';
    const requestedName = await this.confirmDialog.prompt({
      title:
        kind === 'symbolic'
          ? 'Create Symbolic Link'
          : 'Create Hard Link',
      message: `Link to "${entry.name}" as:`,
      confirmText: 'Create',
      defaultValue: `${entry.name}${suffix}`,
      historyKey: `reassembly.${kind}Link.name`,
    });

    const cleanName = await this.validateRequestedLeaf(
      requestedName,
      this.path,
    );
    if (!cleanName) return;

    try {
      const destination = await this.api.joinPaths([
        this.path,
        cleanName,
      ]);

      if (kind === 'symbolic') {
        await this.api.createSymbolicLink(entry.path, destination);
      } else {
        await this.api.createHardLink(entry.path, destination);
      }

      this.operationHistory.record({
        label: `${
          kind === 'symbolic' ? 'Create symbolic link' : 'Create hard link'
        } ${cleanName}`,
        undo: async () => {
          await this.api.deleteFile(destination, true, false);
        },
        redo: async () => {
          if (kind === 'symbolic') {
            await this.api.createSymbolicLink(entry.path, destination);
          } else {
            await this.api.createHardLink(entry.path, destination);
          }
        },
      });

      this.lastAction = `${kind} link: ${cleanName}`;
      await this.refresh();
      this.selectPath(destination);
    } catch (error) {
      await this.showOperationError(
        kind === 'symbolic' ? 'Create Symbolic Link' : 'Create Hard Link',
        error,
      );
    }
  }

  async copySelectedFileUris(): Promise<void> {
    if (!this.isDirectoryTab || this.isInArchive) return;

    const entries =
      this.selectedEntries.length > 0
        ? this.selectedEntries
        : this.cursorEntry
          ? [this.cursorEntry]
          : [];

    if (entries.length === 0) return;

    const toFileUri = (path: string): string => {
      const normalized = path.replace(/\\/g, '/');

      if (/^[a-zA-Z]:\//.test(normalized)) {
        return `file:///${encodeURI(normalized)}`;
      }

      if (normalized.startsWith('//')) {
        return `file:${encodeURI(normalized)}`;
      }

      return `file://${encodeURI(
        normalized.startsWith('/') ? normalized : `/${normalized}`,
      )}`;
    };

    const copied = await this.writeTextClipboard(
      entries.map(entry => toFileUri(entry.path)).join('\n'),
    );

    this.lastAction = copied
      ? `copy file URI: ${entries.length}`
      : 'copy file URI failed';
    this.cdr.markForCheck();
  }

  async copySelectedRelativePaths(): Promise<void> {
    const entries = this.selectedEntries.length > 0
      ? this.selectedEntries
      : this.cursorEntry
        ? [this.cursorEntry]
        : [];

    if (entries.length === 0) return;

    const base = this.path.replace(/\\/g, '/').replace(/\/+$/, '');
    const relative = entries.map(entry => {
      const full = entry.path.replace(/\\/g, '/');
      const prefix = `${base}/`;
      return full.toLowerCase().startsWith(prefix.toLowerCase())
        ? full.slice(prefix.length)
        : entry.name;
    });

    const copied = await this.writeTextClipboard(relative.join('\n'));
    this.lastAction = copied
      ? `copy relative path: ${entries.length}`
      : 'copy relative path failed';
    this.cdr.markForCheck();
  }

  async revealSelectedInFileManager(): Promise<void> {
    const entry = this.cursorEntry ?? this.selectedEntries[0];
    if (
      !entry ||
      this.isInArchive ||
      !this.isDirectoryTab
    ) return;

    try {
      await this.api.revealInFileManager(entry.path);
      this.lastAction = `reveal: ${entry.name}`;
      this.cdr.markForCheck();
    } catch (error) {
      await this.showOperationError('Reveal in File Manager', error);
    }
  }

  async touchSelectedModifiedTime(): Promise<void> {
    if (!this.isDirectoryTab || this.isInArchive) return;

    const entries = this.selectedEntries.length > 0
      ? this.selectedEntries
      : this.cursorEntry
        ? [this.cursorEntry]
        : [];

    if (entries.length === 0) return;

    const initial = new Date().toISOString().slice(0, 19);
    const value = await this.confirmDialog.prompt({
      title:
        entries.length === 1
          ? `Modified Time — ${entries[0].name}`
          : `Modified Time — ${entries.length} items`,
      message:
        'Enter local date/time as YYYY-MM-DDTHH:mm:ss, or "now".',
      confirmText: 'Apply',
      defaultValue: initial,
      historyKey: 'reassembly.modifiedTime',
    });
    if (value === null || value === undefined) return;

    const raw = String(value).trim();
    const timestamp =
      raw.toLowerCase() === 'now'
        ? Date.now()
        : new Date(raw).getTime();

    if (!Number.isFinite(timestamp)) {
      await this.confirmDialog.alert({
        title: 'Invalid Date / Time',
        message:
          'Use YYYY-MM-DDTHH:mm:ss (for example 2026-08-16T12:30:00) or "now".',
      });
      return;
    }

    try {
      const before = await Promise.all(
        entries.map(async entry => ({
          path: entry.path,
          modified: Number((await this.api.getFileInfo(entry.path)).modified),
        })),
      );

      for (const entry of entries) {
        await this.api.setModifiedTime(entry.path, timestamp);
      }

      this.operationHistory.record({
        label: `Set modified time for ${entries.length} item${
          entries.length === 1 ? '' : 's'
        }`,
        undo: async () => {
          for (const item of before) {
            const milliseconds =
              item.modified < 10_000_000_000
                ? item.modified * 1000
                : item.modified;
            await this.api.setModifiedTime(item.path, milliseconds);
          }
        },
        redo: async () => {
          for (const entry of entries) {
            await this.api.setModifiedTime(entry.path, timestamp);
          }
        },
      });

      this.lastAction = `modified time: ${entries.length}`;
      await this.refresh();
    } catch (error) {
      await this.showOperationError('Set Modified Time', error);
    }
  }

  copySelected(): void {
    const entries = this.selectedEntries;
    if (entries.length === 0) return;

    this.fileClipboard.set({
      operation: 'copy',
      paths: entries.map(entry => entry.path),
      sourcePaneId: this.paneId,
      archiveFrame: this.cloneCurrentArchiveFrame(),
    });

    this.lastAction = `copy ${entries.length}`;
    this.cdr.markForCheck();
  }

  cutSelected(): void {
    if (!this.canCut) return;

    const entries = this.selectedEntries;
    if (entries.length === 0) return;

    this.fileClipboard.set({
      operation: 'cut',
      paths: entries.map(entry => entry.path),
      sourcePaneId: this.paneId,
      archiveFrame: this.cloneCurrentArchiveFrame(),
    });

    this.lastAction = `cut ${entries.length}`;
    this.cdr.markForCheck();
  }

  async newDocumentTemplate(
    kind: 'text' | 'markdown' | 'json',
  ): Promise<void> {
    if (!this.canCreate || this.isInArchive) return;

    const templates = {
      text: {
        title: 'New Text Document',
        defaultName: 'New Text Document.txt',
        content: '',
      },
      markdown: {
        title: 'New Markdown Document',
        defaultName: 'README.md',
        content: '# Title\n\n',
      },
      json: {
        title: 'New JSON Document',
        defaultName: 'data.json',
        content: '{\n  \n}\n',
      },
    } as const;

    const template = templates[kind];
    const name = await this.confirmDialog.prompt({
      title: template.title,
      message: 'Enter a name for the new document.',
      confirmText: 'Create',
      defaultValue: template.defaultName,
      historyKey: `reassembly.newTemplate.${kind}`,
    });

    const cleanName = await this.validateRequestedLeaf(name, this.path);
    if (!cleanName) return;

    try {
      const destination = await this.api.joinPaths([this.path, cleanName]);
      await this.api.writeFile(destination, template.content, 'utf8');

      this.operationHistory.record({
        label: `Create ${template.title.toLowerCase()} ${cleanName}`,
        undo: async () => {
          await this.api.deleteFile(destination, true, false);
        },
        redo: async () => {
          await this.api.writeFile(destination, template.content, 'utf8');
        },
      });

      this.lastAction = `new ${kind}: ${cleanName}`;
      await this.refresh();
      this.selectPath(destination);
    } catch (error) {
      await this.showOperationError(template.title, error);
    }
  }

  async createShortcutForSelected(): Promise<void> {
    if (
      !this.isDirectoryTab ||
      this.isInArchive ||
      !this.sourceCapabilities.create
    ) return;

    const entry = this.cursorEntry ?? this.selectedEntries[0];
    if (!entry || entry.path === '__parent__') return;

    try {
      const osInfo = await this.api.getOSInfo();
      const windows = String(osInfo?.os).toLowerCase() === 'windows';

      const defaultName = windows
        ? `${entry.name} - Shortcut.lnk`
        : `${entry.name} shortcut`;

      let name = await this.confirmDialog.prompt({
        title: 'Create Shortcut',
        message: `Create a shortcut to "${entry.name}" as:`,
        confirmText: 'Create',
        defaultValue: defaultName,
        historyKey: 'reassembly.shortcut.name',
      });

      name = this.cleanLeafName(name);
      if (!name) {
        await this.confirmDialog.alert({
          title: 'Invalid Name',
          message: 'Choose a valid shortcut filename.',
        });
        return;
      }
      if (windows && !/\.lnk$/i.test(name)) name += '.lnk';
      if (!(await this.ensureLeafAvailable(this.path, name))) return;

      const destination = await this.api.joinPaths([this.path, name]);
      await this.api.createShortcut(entry.path, destination);

      this.operationHistory.record({
        label: `Create shortcut ${name}`,
        undo: async () => {
          await this.api.deleteFile(destination, true, false);
        },
        redo: async () => {
          await this.api.createShortcut(entry.path, destination);
        },
      });

      this.lastAction = `shortcut: ${name}`;
      await this.refresh();
      this.selectPath(destination);
    } catch (error) {
      await this.showOperationError('Create Shortcut', error);
    }
  }

  async newFile(): Promise<void> {
    if (!this.canCreate || this.isInArchive) return;

    const name = await this.confirmDialog.prompt({
      title: 'New File',
      message: 'Enter a name for the new empty file.',
      confirmText: 'Create',
      defaultValue: 'New File.txt',
      historyKey: 'reassembly.newFile',
    });
    const cleanName = await this.validateRequestedLeaf(name, this.path);
    if (!cleanName) return;

    try {
      const destination = await this.api.joinPaths([this.path, cleanName]);
      await this.api.writeFile(destination, '', 'utf8');
      this.operationHistory.record({
        label: `Create file ${cleanName}`,
        undo: async () => {
          await this.api.deleteFile(destination, true, false);
        },
        redo: async () => {
          await this.api.writeFile(destination, '', 'utf8');
        },
      });
      this.lastAction = `new file: ${cleanName}`;
      await this.refresh();
      this.selectPath(destination);
    } catch (error) {
      await this.showOperationError('Create File', error);
    }
  }

  async moveSelectionToNewFolder(): Promise<void> {
    if (
      !this.isDirectoryTab ||
      this.isInArchive ||
      !this.canCreate
    ) return;

    const entries = this.selectedEntries;
    if (entries.length === 0) return;

    const requested = await this.confirmDialog.prompt({
      title: 'Move to New Folder',
      message: `Create a folder and move ${entries.length} selected item${
        entries.length === 1 ? '' : 's'
      } into it:`,
      confirmText: 'Create & Move',
      defaultValue: 'New Folder',
      historyKey: 'reassembly.moveSelectionToNewFolder',
    });

    const folderName = await this.validateRequestedLeaf(
      requested,
      this.path,
    );
    if (!folderName) return;

    const sourcePaths = entries.map(entry => entry.path);

    try {
      const folderPath = await this.api.joinPaths([
        this.path,
        folderName,
      ]);

      await this.api.createDirectory(folderPath, false);

      const result = await this.api.batchMove(
        sourcePaths.map(source => ({
          source,
          destination: folderPath,
          exactDestination: false,
        })),
        true,
      );

      const affected = result.affectedPaths ?? [];

      if (
        result.failed > 0 ||
        result.skipped > 0 ||
        affected.length !== sourcePaths.length
      ) {
        throw new Error(
          `Move completed only partially (${result.succeeded}/${result.total}).`,
        );
      }

      this.operationHistory.record({
        label: `Move ${affected.length} item${
          affected.length === 1 ? '' : 's'
        } to ${folderName}`,
        undo: async () => {
          const back = await this.api.batchMove(
            affected.map(item => ({
              source: item.destination,
              destination: item.source,
              exactDestination: true,
            })),
            true,
          );
          if (back.failed > 0 || back.skipped > 0) {
            throw new Error('Undo move-to-folder failed.');
          }
          await this.api.deleteFile(folderPath, true, false);
        },
        redo: async () => {
          await this.api.createDirectory(folderPath, false);
          const forward = await this.api.batchMove(
            affected.map(item => ({
              source: item.source,
              destination: folderPath,
              exactDestination: false,
            })),
            true,
          );
          if (forward.failed > 0 || forward.skipped > 0) {
            throw new Error('Redo move-to-folder failed.');
          }
        },
      });

      this.lastAction = `move to folder: ${folderName}`;
      await this.refresh();
      this.selectPath(folderPath);
    } catch (error) {
      await this.showOperationError(
        'Move Selection to New Folder',
        error,
      );
    }
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
    const cleanName = await this.validateRequestedLeaf(name, this.path);
    if (!cleanName) return;

    try {
      const destination = await this.api.joinPaths([this.path, cleanName]);
      await this.api.createDirectory(destination, false);
      this.operationHistory.record({
        label: `Create folder ${cleanName}`,
        undo: async () => {
          await this.api.deleteFile(destination, true, false);
        },
        redo: async () => {
          await this.api.createDirectory(destination, false);
        },
      });
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

    const cleanName = await this.validateRequestedLeaf(
      nextName,
      this.path,
      entry.path,
    );
    if (!cleanName || cleanName === entry.name) return;

    try {
      const destination = await this.api.joinPaths([this.path, cleanName]);
      const source = entry.path;
      const caseOnlyRename =
        this.samePath(destination, source) &&
        destination !== source;

      if (caseOnlyRename) {
        const temporary = await this.api.joinPaths([
          this.path,
          `.rename-${Date.now()}-${Math.random().toString(16).slice(2)}.tmp`,
        ]);

        const first = await this.api.batchMove([{
          source,
          destination: temporary,
          exactDestination: true,
        }], true);

        if (first.failed > 0 || first.skipped > 0) {
          throw new Error('Case-only rename failed during temporary move.');
        }

        try {
          const second = await this.api.batchMove([{
            source: temporary,
            destination,
            exactDestination: true,
          }], true);

          if (second.failed > 0 || second.skipped > 0) {
            throw new Error('Case-only rename failed during final move.');
          }
        } catch (error) {
          await this.api.batchMove([{
            source: temporary,
            destination: source,
            exactDestination: true,
          }], true).catch(() => undefined);
          throw error;
        }
      } else {
        const result = await this.api.batchMove([{
          source,
          destination,
          exactDestination: true,
        }], true);

        if (result.failed > 0 || result.skipped > 0) {
          throw new Error(`Rename failed (${result.failed} item${result.failed === 1 ? '' : 's'}).`);
        }
      }

      this.operationHistory.record({
        label: `Rename ${entry.name} to ${cleanName}`,
        undo: async () => {
          const result = await this.api.batchMove([{
            source: destination,
            destination: source,
            exactDestination: true,
          }], true);
          if (result.failed > 0 || result.skipped > 0) {
            throw new Error('Undo rename failed.');
          }
        },
        redo: async () => {
          const result = await this.api.batchMove([{
            source,
            destination,
            exactDestination: true,
          }], true);
          if (result.failed > 0 || result.skipped > 0) {
            throw new Error('Redo rename failed.');
          }
        },
      });

      this.lastAction = `rename: ${entry.name} -> ${cleanName}`;
      await this.refresh();
      this.selectPath(destination);
    } catch (error) {
      await this.showOperationError('Rename', error);
    }
  }

  async deleteSelected(forcePermanent = false): Promise<void> {
    if (!this.canDelete) return;

    const entries = this.selectedEntries;
    if (entries.length === 0) return;

    if (this.isInArchive) {
      await this.deleteSelectedFromArchive(entries);
      return;
    }

    const useTrash = this.sourceCapabilities.trash && !forcePermanent;
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
      const sourcePaths = entries.map(entry => entry.path);
      const trashBefore = useTrash
        ? await this.api.listTrashEntries().catch(() => [])
        : [];

      const result = await this.api.batchDelete(
        sourcePaths,
        true,
        useTrash,
      );

      if (useTrash && result.succeeded > 0) {
        const beforeIds = new Set(trashBefore.map(item => item.id));
        const normalize = (value: string) =>
          value.replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
        const wanted = new Set(sourcePaths.map(normalize));

        let currentTrashEntries = (
          await this.api.listTrashEntries().catch(() => [])
        ).filter(item =>
          !beforeIds.has(item.id) &&
          wanted.has(normalize(item.originalPath)),
        );

        if (currentTrashEntries.length > 0) {
          this.operationHistory.record({
            label:
              entries.length === 1
                ? `Move ${entries[0].name} to Trash`
                : `Move ${entries.length} items to Trash`,
            undo: async () => {
              for (const item of currentTrashEntries) {
                if (!item.canRestore) {
                  throw new Error(`Trash item cannot be restored: ${item.name}`);
                }
                await this.api.restoreTrashEntry(item.id);
              }
            },
            redo: async () => {
              const before = await this.api.listTrashEntries().catch(() => []);
              const known = new Set(before.map(item => item.id));
              const redoResult = await this.api.batchDelete(sourcePaths, true, true);
              if (redoResult.failed > 0) {
                throw new Error('Redo move to Trash failed.');
              }
              currentTrashEntries = (
                await this.api.listTrashEntries().catch(() => [])
              ).filter(item =>
                !known.has(item.id) &&
                wanted.has(normalize(item.originalPath)),
              );
            },
          });
        }
      }

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

  async showSelectedChecksum(): Promise<void> {
    if (!this.isDirectoryTab || this.isInArchive) return;

    const entries = (
      this.selectedEntries.length > 0
        ? this.selectedEntries
        : this.cursorEntry
          ? [this.cursorEntry]
          : []
    ).filter(entry => entry.type !== FileType.DIRECTORY);

    if (entries.length === 0) return;

    try {
      const rows: string[] = [];

      for (const entry of entries) {
        const digest = await this.api.hashFile(entry.path);
        rows.push(`${digest}  ${entry.name}`);
      }

      const text = rows.join('\n');

      await this.confirmDialog.alert({
        title:
          entries.length === 1
            ? `SHA-256 — ${entries[0].name}`
            : `SHA-256 — ${entries.length} files`,
        message:
          entries.length === 1
            ? `${text}\n\n${entries[0].path}`
            : text,
      });

      this.lastAction = `checksum: ${entries.length} file${
        entries.length === 1 ? '' : 's'
      }`;
    } catch (error) {
      await this.showOperationError('Checksum', error);
    }
  }

  async calculateSelectedSize(): Promise<void> {
    if (!this.isDirectoryTab || this.isInArchive) return;

    const entries =
      this.selectedEntries.length > 0
        ? this.selectedEntries
        : this.cursorEntry
          ? [this.cursorEntry]
          : [];

    if (entries.length === 0) return;

    try {
      let total = 0;
      for (const entry of entries) {
        total += entry.type === FileType.DIRECTORY
          ? await this.api.dirSizeRecursive(entry.path)
          : Number(entry.size) || 0;
      }

      await this.confirmDialog.alert({
        title:
          entries.length === 1
            ? `Size — ${entries[0].name}`
            : `Selected Size — ${entries.length} items`,
        message: [
          `Size: ${this.formatSize(total)}`,
          `Bytes: ${total.toLocaleString()}`,
        ].join('\n'),
      });

      this.lastAction = `size: ${entries.length} item${
        entries.length === 1 ? '' : 's'
      }`;
    } catch (error) {
      await this.showOperationError('Calculate Size', error);
    }
  }

  async createZipFromSelection(): Promise<void> {
    if (!this.isDirectoryTab || this.isInArchive || !this.canCreate) return;

    const entries = this.selectedEntries;
    if (entries.length === 0) return;

    const first = entries[0];
    const baseName =
      entries.length === 1
        ? first.name.replace(/\.[^.]+$/, '') || first.name
        : 'Archive';

    const requestedName = await this.confirmDialog.prompt({
      title: 'Create ZIP Archive',
      message:
        entries.length === 1
          ? `Compress "${first.name}" to:`
          : `Compress ${entries.length} selected items to:`,
      confirmText: 'Create',
      defaultValue: `${baseName}.zip`,
      historyKey: 'reassembly.createZip',
    });

    let cleanName = this.cleanLeafName(requestedName);
    if (!cleanName) {
      await this.confirmDialog.alert({
        title: 'Invalid Name',
        message: 'Choose a valid ZIP filename.',
      });
      return;
    }
    if (!/\.zip$/i.test(cleanName)) cleanName += '.zip';
    if (!(await this.ensureLeafAvailable(this.path, cleanName))) return;

    try {
      const destination = await this.api.joinPaths([this.path, cleanName]);
      const sources = entries.map(entry => entry.path);

      await this.api.createZipArchive(sources, destination);
      this.operationHistory.record({
        label: `Create archive ${cleanName}`,
        undo: async () => {
          await this.api.deleteFile(destination, true, false);
        },
        redo: async () => {
          await this.api.createZipArchive(sources, destination);
        },
      });

      this.lastAction = `archive created: ${cleanName}`;
      await this.refresh();
      this.selectPath(destination);
    } catch (error) {
      await this.showOperationError('Create ZIP Archive', error);
    }
  }

  async editSelectedAttributes(): Promise<void> {
    const entry = this.cursorEntry ?? this.selectedEntries[0];
    if (!entry || this.isInArchive || !this.isDirectoryTab) return;

    try {
      const [osInfo, attributes] = await Promise.all([
        this.api.getOSInfo(),
        this.api.getPathAttributes(entry.path),
      ]);

      if (String(osInfo?.os).toLowerCase() === 'windows') {
        const current = [
          attributes.readOnly ? 'readonly' : '',
          attributes.hidden ? 'hidden' : '',
          attributes.system ? 'system' : '',
          attributes.archive ? 'archive' : '',
        ].filter(Boolean).join(',');

        const value = await this.confirmDialog.prompt({
          title: `Attributes — ${entry.name}`,
          message: 'Comma-separated attributes: readonly, hidden, system, archive',
          confirmText: 'Apply',
          defaultValue: current,
          historyKey: 'reassembly.attributes.windows',
        });
        if (value === null || value === undefined) return;

        const tokens = new Set(
          String(value).split(',').map(token => token.trim().toLowerCase()).filter(Boolean),
        );
        const valid = new Set(['readonly', 'hidden', 'system', 'archive']);
        const invalid = [...tokens].filter(token => !valid.has(token));

        if (invalid.length > 0) {
          await this.confirmDialog.alert({
            title: 'Invalid Attributes',
            message: `Unknown attribute${invalid.length === 1 ? '' : 's'}: ${invalid.join(', ')}`,
          });
          return;
        }

        await this.api.setPathAttributes(entry.path, {
          readOnly: tokens.has('readonly'),
          hidden: tokens.has('hidden'),
          system: tokens.has('system'),
          archive: tokens.has('archive'),
        });
      } else {
        const currentMode = (attributes.unixMode || 0).toString(8).padStart(4, '0');
        const value = await this.confirmDialog.prompt({
          title: `Permissions — ${entry.name}`,
          message: 'Unix permission mode (octal), for example 0644 or 0755:',
          confirmText: 'Apply',
          defaultValue: currentMode,
          historyKey: 'reassembly.attributes.unix',
        });
        if (value === null || value === undefined) return;

        const normalized = String(value).trim();
        if (!/^[0-7]{3,4}$/.test(normalized)) {
          await this.confirmDialog.alert({
            title: 'Invalid Permission Mode',
            message: 'Enter a 3- or 4-digit octal mode such as 644, 0644, or 0755.',
          });
          return;
        }

        await this.api.setPathAttributes(entry.path, {
          unixMode: Number.parseInt(normalized, 8),
        });
      }

      this.lastAction = `attributes: ${entry.name}`;
      await this.refresh();
      this.selectPath(entry.path);
    } catch (error) {
      await this.showOperationError('Attributes', error);
    }
  }

  async showSelectedProperties(): Promise<void> {
    const selected =
      this.selectedEntries.length > 0
        ? this.selectedEntries
        : this.cursorEntry
          ? [this.cursorEntry]
          : [];

    if (selected.length === 0) return;

    try {
      if (selected.length > 1) {
        const infos = this.isInArchive
          ? selected
          : await Promise.all(
              selected.map(entry => this.api.getFileInfo(entry.path)),
            );

        let totalSize = 0;
        let files = 0;
        let directories = 0;
        let archives = 0;
        let symlinks = 0;

        for (const info of infos) {
          if (info.type === FileType.DIRECTORY) {
            directories++;
            totalSize += this.isInArchive
              ? Number(info.size) || 0
              : await this.api.dirSizeRecursive(info.path);
          } else {
            totalSize += Number(info.size) || 0;
            if (info.type === FileType.ARCHIVE) archives++;
            else if (info.type === FileType.SYMLINK) symlinks++;
            else files++;
          }
        }

        await this.confirmDialog.alert({
          title: `Properties — ${selected.length} items`,
          message: [
            `Items: ${selected.length}`,
            `Files: ${files}`,
            `Directories: ${directories}`,
            `Archives: ${archives}`,
            `Symbolic links: ${symlinks}`,
            `Total size: ${this.formatSize(totalSize) || '—'}`,
            `Bytes: ${totalSize.toLocaleString()}`,
            this.isInArchive ? 'Location: inside archive' : `Folder: ${this.path}`,
          ].join('\n'),
        });
        return;
      }

      const entry = selected[0];
      const info = this.isInArchive
        ? entry
        : await this.api.getFileInfo(entry.path);

      const formatTimestamp = (value: unknown): string => {
        const timestamp = Number(value);
        return Number.isFinite(timestamp) && timestamp > 0
          ? new Date(
              timestamp * (timestamp < 10_000_000_000 ? 1000 : 1),
            ).toLocaleString()
          : '—';
      };

      const modifiedText = formatTimestamp(info.modified);
      const createdText = formatTimestamp(info.created);
      const accessedText = formatTimestamp(info.accessed);

      const displaySize =
        !this.isInArchive && info.type === FileType.DIRECTORY
          ? await this.api.dirSizeRecursive(info.path)
          : Number(info.size) || 0;

      const type =
        info.type === FileType.DIRECTORY
          ? 'Directory'
          : info.type === FileType.ARCHIVE
            ? 'Archive'
            : info.type === FileType.SYMLINK
              ? 'Symbolic link'
              : 'File';

      await this.confirmDialog.alert({
        title: `Properties — ${info.name}`,
        message: [
          `Name: ${info.name}`,
          `Type: ${type}`,
          `Path: ${info.path}`,
          `Size: ${this.formatSize(displaySize) || '—'}`,
          `Bytes: ${displaySize.toLocaleString()}`,
          `Modified: ${modifiedText}`,
          `Created: ${createdText}`,
          `Accessed: ${accessedText}`,
          `Permissions: ${info.permissions || '—'}`,
          info.isSymlink ? 'Symbolic link: yes' : '',
          this.isInArchive ? 'Location: inside archive' : '',
        ].filter(Boolean).join('\n'),
      });
    } catch (error) {
      await this.showOperationError('Properties', error);
    }
  }

  async extractSelectedTo(
    destinationDir: string,
    flattenPaths = false,
  ): Promise<boolean> {
    const frame = this.currentArchiveFrame;
    if (!frame || !destinationDir) return false;

    const selected = this.selectedEntries
      .map(entry => entry.path)
      .filter(path => path !== '__parent__');

    if (selected.length === 0) return false;

    try {
      const result = await this.archiveService.extractEntries(
        frame,
        selected,
        destinationDir,
        flattenPaths,
      );

      this.lastAction =
        `extract: ${result?.succeeded ?? selected.length}/${result?.total ?? selected.length}`;

      if ((result?.failed ?? 0) > 0) {
        await this.confirmDialog.alert({
          title: 'Extract Completed with Errors',
          message:
            `${result.failed} item${result.failed === 1 ? '' : 's'} could not be extracted.`,
        });
      }

      return true;
    } catch (error) {
      await this.showOperationError('Extract', error);
      return false;
    }
  }

  private cloneCurrentArchiveFrame(): ArchiveFrame | null {
    const frame = this.currentArchiveFrame;
    if (!frame) return null;
    return {
      ...frame,
      nestedChain: [...frame.nestedChain],
      listing: {
        ...frame.listing,
        entries: (frame.listing.entries ?? []).map(entry => ({ ...entry })),
      },
    };
  }

  async transferArchiveSelectionTo(
    destinationDir: string,
    operation: 'copy' | 'move',
  ): Promise<{
    total: number;
    succeeded: number;
    failed: number;
    sourceRemoved?: boolean;
    errors?: Array<{ source: string; error: string }>;
  } | null> {
    const frame = this.currentArchiveFrame;
    if (!frame || !destinationDir) return null;

    const selected = this.selectedEntries
      .map(entry => entry.path)
      .filter(path => path !== '__parent__');

    if (selected.length === 0) return null;

    try {
      const result = operation === 'copy'
        ? await this.archiveService.copy(
            frame,
            selected,
            destinationDir,
          )
        : await this.archiveService.move(
            frame,
            selected,
            destinationDir,
          );

      this.lastAction =
        `${operation === 'copy' ? 'extract/copy' : 'extract/move'}: ` +
        `${result.succeeded}/${result.total}`;

      if (operation === 'move' && result.sourceRemoved) {
        await this.refreshArchive();
      }

      return result;
    } catch (error) {
      await this.showOperationError(
        operation === 'copy' ? 'Archive Copy' : 'Archive Move',
        error,
      );
      return null;
    }
  }

  private async deleteSelectedFromArchive(
    entries: readonly FileInfo[],
  ): Promise<void> {
    const frame = this.currentArchiveFrame;
    if (!frame) return;

    if (frame.nestedChain.length > 0) {
      await this.confirmDialog.alert({
        title: 'Nested Archive Limitation',
        message:
          'Deleting entries from nested archives is not supported. ' +
          'Navigate back to the top-level archive first.',
      });
      return;
    }

    const paths = entries
      .map(entry => entry.path)
      .filter(path => path !== '__parent__');

    if (paths.length === 0) return;

    const confirmed = await this.confirmDialog.confirm({
      title: 'Delete from Archive',
      message:
        `Delete ${paths.length} item${paths.length === 1 ? '' : 's'} ` +
        'from this archive? The archive file will be rewritten and this cannot be undone.',
      confirmText: 'Delete',
      danger: true,
    });
    if (!confirmed) return;

    try {
      const result = await this.archiveService.delete(frame, paths);
      this.lastAction =
        `archive delete: ${result.succeeded}/${result.total}`;

      await this.refreshArchive();

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: 'Archive Delete Completed with Errors',
          message:
            `${result.failed} item${result.failed === 1 ? '' : 's'} could not be deleted.`,
        });
      }
    } catch (error) {
      await this.showOperationError('Archive Delete', error);
    }
  }

  private nestedArchiveChainForEntry(
    frame: ArchiveFrame,
    entry: FileInfo,
  ): string[] {
    const currentChain = [...(frame.nestedChain ?? [])];
    const entryContainingChain = (entry as FileInfo & {
      nestedChain?: string[];
    }).nestedChain;

    const containingChain =
      Array.isArray(entryContainingChain) &&
      entryContainingChain.length >= currentChain.length
        ? [...entryContainingChain]
        : currentChain;

    const nestedArchivePath = this.resolveArchiveEntryInnerPath(
      frame.innerPath,
      entry.path,
    );

    if (!nestedArchivePath) return containingChain;
    if (containingChain[containingChain.length - 1] === nestedArchivePath) {
      return containingChain;
    }

    return [...containingChain, nestedArchivePath];
  }

  private async openNestedArchiveEntry(
    entry: FileInfo,
  ): Promise<void> {
    const frame = this.currentArchiveFrame;
    if (!frame) return;

    const nestedChain = this.nestedArchiveChainForEntry(frame, entry);
    if (nestedChain.length <= frame.nestedChain.length) {
      this.error = `Unable to resolve nested archive: ${entry.name}`;
      this.cdr.markForCheck();
      return;
    }

    await this.openNestedArchive(nestedChain);
  }

  private async openNestedArchive(
    nestedChain: string[],
  ): Promise<void> {
    const frame = this.currentArchiveFrame;
    if (!frame) return;

    const requestedChain = nestedChain
      .map(segment => normalizeArchiveInnerPath(segment))
      .filter(Boolean);

    if (
      requestedChain.length === frame.nestedChain.length &&
      requestedChain.every(
        (segment, index) => segment === frame.nestedChain[index],
      )
    ) {
      return;
    }

    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      const nested = await this.directorySource.openNestedArchive(
        frame,
        requestedChain,
      );

      this.archiveStack.push(nested);
      await this.renderArchiveFrame(nested, { history: 'record' });
      this.lastAction = `nested archive: ${
        requestedChain[requestedChain.length - 1] ?? ''
      }`;
    } catch (error) {
      this.error =
        error instanceof Error ? error.message : String(error);
    } finally {
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  private resolveArchiveEntryInnerPath(
    currentInnerPath: string,
    entryPath: string,
  ): string {
    const current = normalizeArchiveInnerPath(currentInnerPath);
    const entry = normalizeArchiveInnerPath(entryPath);

    if (!current) return entry;
    if (!entry) return current;

    // Archive backends normally return paths relative to archive root
    // (`folder/child`). Some formats/providers may return only the child name.
    // Accept both without duplicating the current directory.
    if (entry === current || entry.startsWith(`${current}/`)) {
      return entry;
    }

    return normalizeArchiveInnerPath(`${current}/${entry}`);
  }

  private async navigateArchiveInto(
    entryPath: string,
  ): Promise<void> {
    const frame = this.currentArchiveFrame;
    if (!frame) return;

    const innerPath = this.resolveArchiveEntryInnerPath(
      frame.innerPath,
      entryPath,
    );

    this.lastAction = `archive folder: ${innerPath}`;

    await this.navigateWithinOpenArchive(
      innerPath,
      {
        history: 'record',
        preserveCursor: false,
      },
    );
  }

  private async navigateArchiveUp(): Promise<void> {
    const frame = this.currentArchiveFrame;
    if (!frame) return;

    if (frame.innerPath) {
      this.loading = true;
      this.error = null;
      this.cdr.markForCheck();

      try {
        const parent = await this.directorySource.navigateArchiveUp(frame);
        if (!parent) return;

        this.archiveStack[this.archiveStack.length - 1] = parent;
        await this.renderArchiveFrame(parent, { history: 'record' });
        this.lastAction = 'archive up';
      } catch (error) {
        this.error =
          error instanceof Error ? error.message : String(error);
      } finally {
        this.loading = false;
        this.cdr.markForCheck();
      }
      return;
    }

    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      if (this.archiveStack.length > 1) {
        this.archiveStack.pop();
        const previous = this.currentArchiveFrame;
        if (previous) {
          await this.renderArchiveFrame(previous, { history: 'record' });
        }
        this.lastAction = 'leave nested archive';
        return;
      }

      const archivePath = frame.fsPath;
      const exitPath =
        this.archiveExitPath ||
        this.directorySource.getParentPath(archivePath);

      this.clearArchiveRuntime();
      this.lastAction = 'leave archive';
      await this.load(exitPath, {
        history: 'record',
        preserveCursor: false,
      });

      this.selectPath(archivePath);
    } catch (error) {
      this.error =
        error instanceof Error ? error.message : String(error);
    } finally {
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  private async refreshArchive(): Promise<void> {
    const frame = this.currentArchiveFrame;
    if (!frame) return;

    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      const refreshed = await this.directorySource.refreshArchive(frame);
      this.archiveStack[this.archiveStack.length - 1] = refreshed;
      await this.renderArchiveFrame(refreshed, {
        history: 'none',
        preserveCursor: true,
      });
    } catch (error) {
      this.error =
        error instanceof Error ? error.message : String(error);
    } finally {
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  private clearArchiveRuntime(): void {
    this.archiveStack.splice(0, this.archiveStack.length);
    this.archiveExitPath = '';
  }

  private async renderArchiveFrame(
    frame: ArchiveFrame,
    options: {
      history?: 'record' | 'none';
      preserveCursor?: boolean;
    } = {},
  ): Promise<void> {
    const previousCursor =
      options.preserveCursor ? this.cursorPath : null;

    if (this.viewMode === 'tree') {
      this.viewMode = 'list';
    }

    const archivePath = buildArchiveDisplayPath(
      this.archiveStack[0]?.fsPath ?? frame.fsPath,
      this.archiveStack,
    );
    const archiveEntries = this.directorySource.archiveListingToFileInfos(
      frame.listing,
    );
    const listing = await this.panelSources.listContext({
      path: archivePath,
      entries: archiveEntries,
      inArchive: true,
      archiveTopLevel: frame.nestedChain.length === 0,
    }, this.showHidden);

    this.path = listing.location.path;
    this.entries = [...listing.entries];

    const preserved = previousCursor
      ? this.entries.find(entry => entry.path === previousCursor)
      : null;

    const nextCursor =
      preserved ??
      this.entries.find(entry => entry.path !== '__parent__') ??
      this.entries[0] ??
      null;

    this.cursorPath = nextCursor?.path ?? null;
    this.selectedPaths = nextCursor
      ? new Set([nextCursor.path])
      : new Set();
    this.anchorPath = this.cursorPath;
    this.error = null;

    if ((options.history ?? 'record') === 'record') {
      this.navigation.record(this.path);
    }

    this.syncActiveTab(this.path);

    if (this.activeTabId && this.activeTab?.kind === 'dir') {
      this.workspaceService.updateTabPathAndTitle(
        this.paneId,
        this.activeTabId,
        this.path,
        buildArchiveTabTitle(
          this.archiveStack[0]?.fsPath ?? frame.fsPath,
          this.archiveStack,
        ),
      );
    }

    this.pathHistory.addLocation(this.path, this.paneId);
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
  }

  private canReuseArchiveStack(
    fsPath: string,
    nestedChain: readonly string[],
  ): boolean {
    if (this.archiveStack.length === 0) return false;

    const root = this.archiveStack[0];
    const current = this.currentArchiveFrame;
    if (!root || !current) return false;

    return sameArchiveLocation(root.fsPath, fsPath) &&
      current.nestedChain.length === nestedChain.length &&
      current.nestedChain.every(
        (segment, index) => segment === nestedChain[index],
      );
  }

  private async navigateWithinOpenArchive(
    innerPath: string,
    options: {
      preserveCursor?: boolean;
      history?: 'record' | 'none';
      syncTab?: boolean;
      restoreSession?: boolean;
    },
  ): Promise<boolean> {
    const frame = this.currentArchiveFrame;
    if (!frame) return false;

    const generation = ++this.loadGeneration;
    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      const next = await this.directorySource.navigateArchive(
        frame,
        innerPath,
      );
      if (generation !== this.loadGeneration) return false;

      this.archiveStack[this.archiveStack.length - 1] = next;
      await this.renderArchiveFrame(next, {
        history: options.history,
        preserveCursor: options.preserveCursor,
      });
      return true;
    } catch (error) {
      if (generation !== this.loadGeneration) return false;
      this.error = error instanceof Error ? error.message : String(error);
      return false;
    } finally {
      if (generation === this.loadGeneration) {
        this.loading = false;
        this.cdr.markForCheck();
      }
    }
  }

  private async loadArchivePath(
    requestedPath: string,
    options: {
      preserveCursor?: boolean;
      history?: 'record' | 'none';
      syncTab?: boolean;
      restoreSession?: boolean;
    },
  ): Promise<boolean> {
    const parsed = parseArchivePath(requestedPath);

    // Back/forward, breadcrumbs and Ctrl+L often stay inside the same physical
    // archive. Reuse the already-open frame chain instead of reopening every
    // archive layer through the backend.
    if (this.canReuseArchiveStack(parsed.fsPath, parsed.nestedChain)) {
      return this.navigateWithinOpenArchive(
        parsed.innerPath,
        options,
      );
    }

    const generation = ++this.loadGeneration;

    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      const rootFrame = await this.directorySource.getArchiveFrame(
        parsed.fsPath,
      );
      if (generation !== this.loadGeneration) return false;

      this.clearArchiveRuntime();
      this.archiveExitPath =
        this.directorySource.getParentPath(parsed.fsPath);
      this.archiveStack.push(rootFrame);

      let frame = rootFrame;

      for (
        let index = 0;
        index < parsed.nestedChain.length;
        index += 1
      ) {
        frame = await this.directorySource.openNestedArchive(
          frame,
          parsed.nestedChain.slice(0, index + 1),
        );
        if (generation !== this.loadGeneration) return false;
        this.archiveStack.push(frame);
      }

      if (parsed.innerPath) {
        frame = await this.directorySource.navigateArchive(
          frame,
          parsed.innerPath,
        );
        if (generation !== this.loadGeneration) return false;
        this.archiveStack[this.archiveStack.length - 1] = frame;
      }

      await this.renderArchiveFrame(frame, {
        history: options.history,
        preserveCursor: options.preserveCursor,
      });

      return true;
    } catch (error) {
      if (generation !== this.loadGeneration) return false;
      this.error =
        error instanceof Error ? error.message : String(error);
      return false;
    } finally {
      if (generation === this.loadGeneration) {
        this.loading = false;
        this.cdr.markForCheck();
      }
    }
  }

  async paste(): Promise<void> {
    const clipboard = this.fileClipboard.payload;
    if (!clipboard || !this.canPaste) return;

    try {
      if (clipboard.archiveFrame) {
        const result = clipboard.operation === 'copy'
          ? await this.archiveService.copy(
              clipboard.archiveFrame,
              clipboard.paths,
              this.path,
            )
          : await this.archiveService.move(
              clipboard.archiveFrame,
              clipboard.paths,
              this.path,
            );

        this.lastAction =
          `paste archive ${clipboard.operation}: ` +
          `${result.succeeded}/${result.total}`;

        if (
          clipboard.operation === 'cut' &&
          result.failed === 0 &&
          result.sourceRemoved
        ) {
          this.fileClipboard.clear();
        }

        await this.refresh();

        if (result.failed > 0) {
          await this.confirmDialog.alert({
            title: 'Paste Completed with Errors',
            message:
              `${result.failed} archive entr${
                result.failed === 1 ? 'y' : 'ies'
              } could not be ${
                clipboard.operation === 'copy' ? 'extracted' : 'moved'
              }.`,
          });
        }
        return;
      }

      const items = await Promise.all(
        clipboard.paths.map(async source => ({
          source,
          destination: await this.api.joinPaths([
            this.path,
            this.leafName(source),
          ]),
        })),
      );

      const result =
        clipboard.operation === 'copy'
          ? await this.api.batchCopy(items, true)
          : await this.api.batchMove(items, true);

      this.lastAction =
        `${clipboard.operation === 'copy' ? 'paste copy' : 'paste move'}: ` +
        `${result.succeeded}/${result.total}`;

      if (clipboard.operation === 'cut' && result.failed === 0) {
        this.fileClipboard.clear();
      }

      await this.refresh();

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: 'Paste Completed with Errors',
          message:
            `${result.failed} item${
              result.failed === 1 ? '' : 's'
            } could not be pasted.`,
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

    if (
      !clean ||
      clean === '.' ||
      clean === '..' ||
      /[\/\\]/.test(clean) ||
      /[<>:"|?*\u0000-\u001f]/.test(clean) ||
      /[ .]$/.test(clean)
    ) {
      return '';
    }

    const base = clean.split('.')[0].toUpperCase();
    const reserved = new Set([
      'CON', 'PRN', 'AUX', 'NUL',
      'COM1', 'COM2', 'COM3', 'COM4', 'COM5',
      'COM6', 'COM7', 'COM8', 'COM9',
      'LPT1', 'LPT2', 'LPT3', 'LPT4', 'LPT5',
      'LPT6', 'LPT7', 'LPT8', 'LPT9',
    ]);

    return reserved.has(base) ? '' : clean;
  }

  private async ensureLeafAvailable(
    directory: string,
    leafName: string,
    currentPath?: string,
  ): Promise<boolean> {
    const destination = await this.api.joinPaths([directory, leafName]);

    try {
      const existing = await this.api.getFileInfo(destination);

      if (
        currentPath &&
        this.samePath(existing.path, currentPath)
      ) {
        return true;
      }

      await this.confirmDialog.alert({
        title: 'Name Already Exists',
        message: `An item named "${leafName}" already exists in this folder.`,
      });
      return false;
    } catch {
      return true;
    }
  }

  private async validateRequestedLeaf(
    value: string | null,
    directory: string,
    currentPath?: string,
  ): Promise<string> {
    const clean = this.cleanLeafName(value);

    if (!clean) {
      await this.confirmDialog.alert({
        title: 'Invalid Name',
        message: [
          'Choose a normal file or folder name.',
          'Path separators, control characters, < > : " | ? *, trailing spaces/dots,',
          'and Windows reserved device names such as CON, PRN, AUX, COM1 and LPT1 are not allowed.',
        ].join('\\n'),
      });
      return '';
    }

    return await this.ensureLeafAvailable(
      directory,
      clean,
      currentPath,
    )
      ? clean
      : '';
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
          ? this.fromPersistedDirTab(persisted)
          : this.createDirectoryTab(this.path);
        this.tabs = [tab];
        this.activeTabId = tab.id;
        if (tab.kind === 'dir') this.applyDirSessionBeforeLoad(tab);
        this.workspaceTabsReady = !!persisted;
        this.cdr.markForCheck();
      }
    } catch {
      try {
        const loaded = await this.load('/', { history: 'record' });
        if (loaded && this.tabs.length === 0) {
          const persisted = this.workspaceService.addTab(this.paneId, this.path, true);
          const tab = persisted
            ? this.fromPersistedDirTab(persisted)
            : this.createDirectoryTab(this.path);
          this.tabs = [tab];
          this.activeTabId = tab.id;
          if (tab.kind === 'dir') this.applyDirSessionBeforeLoad(tab);
          this.workspaceTabsReady = !!persisted;
          this.cdr.markForCheck();
        }
      } catch {
        // load() records the visible error state.
      }
    }
  }


  private compareEntries(left: FileInfo, right: FileInfo): number {
    switch (this.sortField) {
      case 'size':
        return Number(left.size ?? 0) - Number(right.size ?? 0);

      case 'modified':
        return Number(left.modified ?? 0) - Number(right.modified ?? 0);

      case 'type':
        return this.sortTypeKey(left).localeCompare(
          this.sortTypeKey(right),
          undefined,
          { numeric: true, sensitivity: 'base' },
        );

      case 'name':
      default:
        return (left.name ?? '').localeCompare(right.name ?? '', undefined, {
          numeric: true,
          sensitivity: 'base',
        });
    }
  }

  private sortTypeKey(entry: FileInfo): string {
    if (entry.type === FileType.DIRECTORY) return '0-directory';

    const name = entry.name ?? '';
    const dot = name.lastIndexOf('.');
    const extension =
      dot > 0 ? name.slice(dot + 1).toLocaleLowerCase() : '';

    return `1-${extension || 'file'}`;
  }

  private defaultDirTabSession(): ReassemblyDirTabSession {
    return {
      schemaVersion: REASSEMBLY_DIR_TAB_SESSION_VERSION,
      viewMode: 'list',
      sortField: 'name',
      sortDirection: 'asc',
      directoriesFirst: true,
      expandedPaths: [],
      filterText: '',
      filterVisible: false,
      selectedPaths: [],
      cursorPath: null,
      anchorPath: null,
      scrollTop: 0,
      entries: [],
      navigationEntries: [],
      navigationIndex: -1,
    };
  }

  private dirSessionStorageKey(tabId: string): string {
    return `reassembly.dirTab.${this.paneId}.${tabId}`;
  }

  private loadDirTabSession(tabId: string): ReassemblyDirTabSession {
    const fallback = this.defaultDirTabSession();

    try {
      const raw = localStorage.getItem(this.dirSessionStorageKey(tabId));
      if (!raw) return fallback;

      const parsed = JSON.parse(raw) as Partial<ReassemblyDirTabSession>;
      const version =
        typeof parsed.schemaVersion === 'number'
          ? parsed.schemaVersion
          : 1;

      // v1 was Phase 24's presentation-only shape. Missing fields naturally
      // migrate to the defaults below.
      void version;

      return {
        schemaVersion: REASSEMBLY_DIR_TAB_SESSION_VERSION,
        viewMode:
          parsed.viewMode === 'list' ||
          parsed.viewMode === 'grid' ||
          parsed.viewMode === 'details' ||
          parsed.viewMode === 'tree'
            ? parsed.viewMode
            : fallback.viewMode,
        sortField:
          parsed.sortField === 'name' ||
          parsed.sortField === 'size' ||
          parsed.sortField === 'modified' ||
          parsed.sortField === 'type'
            ? parsed.sortField
            : fallback.sortField,
        sortDirection:
          parsed.sortDirection === 'desc' ? 'desc' : 'asc',
        directoriesFirst:
          typeof parsed.directoriesFirst === 'boolean'
            ? parsed.directoriesFirst
            : fallback.directoriesFirst,
        expandedPaths:
          Array.isArray(parsed.expandedPaths)
            ? parsed.expandedPaths.filter(
                (path): path is string => typeof path === 'string',
              )
            : [],
        filterText:
          typeof parsed.filterText === 'string'
            ? parsed.filterText
            : '',
        filterVisible:
          !!parsed.filterVisible,
        selectedPaths:
          Array.isArray(parsed.selectedPaths)
            ? parsed.selectedPaths.filter(
                (path): path is string => typeof path === 'string',
              )
            : [],
        cursorPath:
          typeof parsed.cursorPath === 'string'
            ? parsed.cursorPath
            : null,
        anchorPath:
          typeof parsed.anchorPath === 'string'
            ? parsed.anchorPath
            : null,
        scrollTop:
          typeof parsed.scrollTop === 'number' &&
          Number.isFinite(parsed.scrollTop) &&
          parsed.scrollTop >= 0
            ? parsed.scrollTop
            : 0,
        entries:
          Array.isArray(parsed.entries)
            ? parsed.entries.map(entry => ({ ...entry }))
            : [],
        navigationEntries:
          Array.isArray(parsed.navigationEntries)
            ? parsed.navigationEntries.filter(
                (entry): entry is string =>
                  typeof entry === 'string' && !!entry.trim(),
              ).slice(-200)
            : [],
        navigationIndex:
          typeof parsed.navigationIndex === 'number'
            ? parsed.navigationIndex
            : -1,
      };
    } catch {
      return fallback;
    }
  }

  private persistDirTabSession(
    tab: Extract<ReassemblyTab, { kind: 'dir' }>,
  ): void {
    try {
      const session: ReassemblyDirTabSession = {
        schemaVersion: REASSEMBLY_DIR_TAB_SESSION_VERSION,
        viewMode: tab.viewMode,
        sortField: tab.sortField,
        sortDirection: tab.sortDirection,
        directoriesFirst: tab.directoriesFirst,
        expandedPaths: [...tab.expandedPaths],
        filterText: tab.filterText,
        filterVisible: tab.filterVisible,
        selectedPaths: [...tab.selectedPaths],
        cursorPath: tab.cursorPath,
        anchorPath: tab.anchorPath,
        scrollTop: Math.max(0, Math.round(tab.scrollTop)),
        entries: tab.entries.map(entry => ({ ...entry })),
        navigationEntries: [...tab.navigationEntries],
        navigationIndex: tab.navigationIndex,
      };

      localStorage.setItem(
        this.dirSessionStorageKey(tab.id),
        JSON.stringify(session),
      );
    } catch {
      // Per-tab session persistence is best-effort only.
    }
  }

  private currentScrollTop(): number {
    const viewport =
      this.viewMode === 'tree'
        ? this.treeViewport?.nativeElement
        : this.listViewport?.nativeElement;
    return Math.max(0, viewport?.scrollTop ?? 0);
  }

  private persistArchiveStateForTab(tabId: string): void {
    const frame = this.currentArchiveFrame;
    if (!frame || this.archiveStack.length === 0) return;

    this.workspaceService.saveArchiveState(
      this.paneId,
      tabId,
      {
        path: this.path,
        archiveStack: this.archiveStack,
        fsDirPath: this.archiveStack[0]?.fsPath ?? frame.fsPath,
        fsParentDir: this.archiveExitPath,
      },
    );
  }

  private restoreArchiveStateForTab(
    tabId: string,
    requestedPath: string,
  ): boolean {
    const state = this.workspaceService.getArchiveState(
      this.paneId,
      tabId,
    );
    if (!state || state.archiveStack.length === 0) {
      return false;
    }

    const parsed = parseArchivePath(requestedPath);
    const root = state.archiveStack[0];
    const current = state.archiveStack[state.archiveStack.length - 1];

    if (
      !root ||
      !current ||
      !sameArchiveLocation(root.fsPath, parsed.fsPath) ||
      current.nestedChain.length !== parsed.nestedChain.length ||
      !current.nestedChain.every(
        (segment, index) => segment === parsed.nestedChain[index],
      )
    ) {
      return false;
    }

    this.archiveStack.splice(
      0,
      this.archiveStack.length,
      ...state.archiveStack.map(frame => ({
        ...frame,
        nestedChain: [...frame.nestedChain],
        listing: {
          ...frame.listing,
          entries: (frame.listing?.entries ?? []).map(entry => ({
            ...entry,
          })),
        },
      })),
    );

    this.archiveExitPath =
      state.fsParentDir ||
      this.directorySource.getParentPath(parsed.fsPath);

    return true;
  }

  private syncActiveDirSessionState(
    persist = true,
  ): Extract<ReassemblyTab, { kind: 'dir' }> | null {
    const active = this.activeTab;
    if (!active || active.kind !== 'dir') return null;

    const updated: Extract<ReassemblyTab, { kind: 'dir' }> = {
      ...active,
      path: this.path || active.path,
      title: this.tabTitle(this.path || active.path),
      viewMode: this.viewMode,
      sortField: this.sortField,
      sortDirection: this.sortDirection,
      directoriesFirst: this.directoriesFirst,
      expandedPaths: Array.from(this.treeExpandedPaths),
      filterText: this.filterText,
      filterVisible: this.filterVisible,
      selectedPaths: Array.from(this.selectedPaths),
      cursorPath: this.cursorPath,
      anchorPath: this.anchorPath,
      scrollTop: this.currentScrollTop(),
      entries: this.entries.map(entry => ({ ...entry })),
      navigationEntries: [...this.navigation.snapshotState().entries],
      navigationIndex: this.navigation.snapshotState().index,
    };

    this.tabs = this.tabs.map(tab =>
      tab.id === updated.id ? updated : tab,
    );

    if (persist) {
      this.persistDirTabSession(updated);

      if (this.isInArchive) {
        this.persistArchiveStateForTab(updated.id);
      } else {
        this.workspaceService.clearArchiveState(
          this.paneId,
          updated.id,
        );
      }
    }

    this.syncAutoFollowPreview(updated);
    return updated;
  }

  private syncAutoFollowPreview(
    sourceTab: Extract<ReassemblyTab, { kind: 'dir' }>,
  ): void {
    if (this.isInArchive) return;

    const preview = this.tabs.find(
      (
        tab,
      ): tab is Extract<ReassemblyTab, { kind: 'viewer' }> =>
        tab.kind === 'viewer' && tab.autoFollow,
    );
    if (!preview) return;

    const target = this.entries.find(
      entry =>
        entry.path === this.cursorPath &&
        entry.type !== FileType.DIRECTORY,
    );
    if (!target || this.samePath(preview.targetPath, target.path)) {
      return;
    }

    const fileList = this.buildViewerFileList();

    this.workspaceService.updateViewerTabTargetPath(
      this.paneId,
      preview.id,
      target.path,
      {
        sourcePane: this.paneId,
        sourceTabId: sourceTab.id,
        sourceDir: this.path,
        fileList,
      },
    );

    const updated: Extract<ReassemblyTab, { kind: 'viewer' }> = {
      ...preview,
      path: target.path,
      title: 'Preview',
      targetPath: target.path,
      fileList,
      archiveContext: null,
      viewerId: preview.viewerId,
      autoFollow: true,
      sourcePane: this.paneId,
      sourceTabId: sourceTab.id,
      sourceDir: this.path,
    };

    this.tabs = this.tabs.map(tab =>
      tab.id === updated.id ? updated : tab,
    );
  }


  private applyDirSessionBeforeLoad(
    tab: Extract<ReassemblyTab, { kind: 'dir' }>,
  ): void {
    this.viewMode = tab.viewMode;
    this.sortField = tab.sortField;
    this.sortDirection = tab.sortDirection;
    this.directoriesFirst = tab.directoriesFirst;
    this.filterText = tab.filterText;
    this.filterVisible = tab.filterVisible;
    this.navigation.restore({
      entries: tab.navigationEntries,
      index: tab.navigationIndex,
    });

    // Cached entries make tab switching immediate while the source refreshes.
    this.entries = tab.entries.map(entry => ({ ...entry }));

    this.cursorPath = tab.cursorPath;
    this.selectedPaths = new Set(tab.selectedPaths);
    this.anchorPath = tab.anchorPath;

    this.treeExpandedPaths.clear();
    for (const path of tab.expandedPaths) {
      this.treeExpandedPaths.add(path);
    }
    this.treeChildren.clear();
    this.treeLoadingPaths.clear();

    this.reconcileRestoredSelection();
    this.cdr.markForCheck();
  }

  private async finalizeDirSessionAfterLoad(
    tab: Extract<ReassemblyTab, { kind: 'dir' }>,
    interactionGeneration: number,
  ): Promise<void> {
    // Do not replace user selection/filter if the user interacted while the
    // refresh was in flight.
    if (interactionGeneration === this.interactionGeneration) {
      this.filterText = tab.filterText;
      this.filterVisible = tab.filterVisible;
      this.cursorPath = tab.cursorPath;
      this.selectedPaths = new Set(tab.selectedPaths);
      this.anchorPath = tab.anchorPath;
      this.reconcileRestoredSelection();

      if (this.viewMode === 'tree') {
        await this.restoreExpandedTreePaths(tab.expandedPaths);
      }

      queueMicrotask(() => this.restoreScrollTop(tab.scrollTop));
    }

    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
  }

  private async restoreExpandedTreePaths(
    expandedPaths: readonly string[],
  ): Promise<void> {
    if (this.viewMode !== 'tree' || expandedPaths.length === 0) return;

    const desired = new Set(expandedPaths);
    this.treeExpandedPaths.clear();
    this.treeChildren.clear();
    this.treeLoadingPaths.clear();

    const expandLevel = async (entries: FileInfo[]): Promise<void> => {
      for (const entry of entries) {
        if (
          entry.type !== FileType.DIRECTORY ||
          !desired.has(entry.path)
        ) {
          continue;
        }

        const row: ReassemblyTreeRow = {
          entry,
          depth: 0,
          expanded: false,
          loading: false,
          expandable: true,
        };

        await this.toggleTreeRow(row, undefined, false);
        await expandLevel(this.treeChildren.get(entry.path) ?? []);
      }
    };

    await expandLevel(this.visibleEntries);
  }

  private reconcileRestoredSelection(): void {
    const available = new Set(
      (this.viewMode === 'tree'
        ? this.treeRows.map(row => row.entry)
        : this.visibleEntries
      )
        .filter(entry => entry.path !== '__parent__')
        .map(entry => entry.path),
    );

    this.selectedPaths = new Set(
      Array.from(this.selectedPaths).filter(path => available.has(path)),
    );

    if (this.cursorPath && !available.has(this.cursorPath)) {
      this.cursorPath = null;
    }

    if (this.anchorPath && !available.has(this.anchorPath)) {
      this.anchorPath = null;
    }

    if (!this.cursorPath) {
      const first =
        this.selectedPaths.values().next().value ??
        (this.viewMode === 'tree'
          ? this.treeRows[0]?.entry.path
          : this.visibleEntries[0]?.path) ??
        null;

      this.cursorPath = first;
    }

    if (
      this.cursorPath &&
      this.cursorPath !== '__parent__' &&
      this.selectedPaths.size === 0
    ) {
      this.selectedPaths.add(this.cursorPath);
    }

    if (!this.anchorPath) {
      this.anchorPath = this.cursorPath;
    }
  }

  private restoreScrollTop(scrollTop: number): void {
    const viewport =
      this.viewMode === 'tree'
        ? this.treeViewport?.nativeElement
        : this.listViewport?.nativeElement;
    if (!viewport) return;

    viewport.scrollTop = Math.max(0, scrollTop);
  }

  private markUserInteraction(): void {
    this.interactionGeneration += 1;
  }

  onContentScroll(event: Event): void {
    const target = event.currentTarget as HTMLElement | null;
    if (!target || !this.isDirectoryTab) return;

    const active = this.activeTab;
    if (!active || active.kind !== 'dir') return;

    const updated: Extract<ReassemblyTab, { kind: 'dir' }> = {
      ...active,
      scrollTop: Math.max(0, target.scrollTop),
    };

    this.tabs = this.tabs.map(tab =>
      tab.id === updated.id ? updated : tab,
    );

    if (this.scrollPersistTimer) {
      clearTimeout(this.scrollPersistTimer);
    }

    this.scrollPersistTimer = setTimeout(() => {
      this.scrollPersistTimer = null;
      this.syncActiveDirSessionState();
    }, 140);
  }

  private async tryRestoreWorkspaceTabs(): Promise<boolean> {
    for (let attempt = 0; attempt < 20; attempt += 1) {
      const persisted = this.workspaceService.getTabs(this.paneId);
      const active = this.workspaceService.getActiveTab(this.paneId);

      if (persisted.length > 0) {
        const restored = persisted
          .map(tab => this.fromPersistedWorkspaceTab(tab))
          .filter((tab): tab is ReassemblyTab => !!tab);

        if (restored.length === 0) return false;

        this.tabs = restored;
        const target =
          (active ? restored.find(tab => tab.id === active.id) : null) ??
          restored.find(tab => tab.kind === 'dir') ??
          restored[0];

        this.activeTabId = target.id;
        this.workspaceTabsReady = true;
        this.tabCounter = Math.max(this.tabCounter, restored.length);
        this.cdr.markForCheck();

        if (target.kind !== 'dir') {
          this.workspaceService.switchTab(this.paneId, target.id);
          this.loading = false;
          this.error = null;
          this.clearArchiveRuntime();
          this.lastAction =
            target.kind === 'viewer'
              ? `viewer restored: ${target.title}`
              : `surface restored: ${target.title}`;
          this.cdr.markForCheck();
          return true;
        }

        // From here on target is narrowed to a directory tab.
        this.applyDirSessionBeforeLoad(target);
        this.restoreArchiveStateForTab(target.id, target.path);

        const interactionGeneration = this.interactionGeneration;
        const loaded = await this.load(target.path, {
          history: 'record',
          syncTab: false,
          restoreSession: true,
        });

        if (loaded) {
          this.workspaceService.switchTab(this.paneId, target.id);
          await this.finalizeDirSessionAfterLoad(
            target,
            interactionGeneration,
          );
          return true;
        }

        if (target.entries.length > 0) {
          this.applyDirSessionBeforeLoad(target);
          this.error = `Location unavailable: ${target.path}`;
          this.lastAction = 'cached workspace tab';
          this.loading = false;
          this.cdr.markForCheck();
          return true;
        }

        return false;
      }

      await new Promise(resolve => setTimeout(resolve, 50));
    }

    return false;
  }

  private fromPersistedDirTab(tab: any): Extract<ReassemblyTab, { kind: 'dir' }> {
    const path =
      typeof tab.location === 'string'
        ? tab.location
        : tab.location?.location || tab.path || '/';

    const session = this.loadDirTabSession(tab.id);

    return {
      id: tab.id,
      kind: 'dir',
      path,
      title: tab.title || this.tabTitle(path),
      viewMode: session.viewMode,
      sortField: session.sortField,
      sortDirection: session.sortDirection,
      directoriesFirst: session.directoriesFirst,
      expandedPaths: session.expandedPaths,
      filterText: session.filterText,
      filterVisible: session.filterVisible,
      selectedPaths: session.selectedPaths,
      cursorPath: session.cursorPath,
      anchorPath: session.anchorPath,
      scrollTop: session.scrollTop,
      entries: session.entries,
      navigationEntries: session.navigationEntries,
      navigationIndex: session.navigationIndex,
      isClosable: tab.isClosable !== false,
      isPinned: !!tab.isPinned,
    };
  }

  private fromPersistedViewerTab(
    tab: any,
  ): Extract<ReassemblyTab, { kind: 'viewer' }> {
    const viewer = tab.viewer ?? {};
    const targetPath = viewer.targetPath || tab.path || '';

    return {
      id: tab.id,
      kind: 'viewer',
      path: targetPath,
      title: tab.title || targetPath || 'Preview',
      targetPath,
      fileList: Array.isArray(viewer.fileList)
        ? viewer.fileList.map((item: ViewerFileListItem) => ({ ...item }))
        : [],
      archiveContext: viewer.archiveContext
        ? {
            fsPath: viewer.archiveContext.fsPath,
            nestedChain: [...(viewer.archiveContext.nestedChain ?? [])],
            entryPath: normalizeArchiveInnerPath(
              viewer.archiveContext.entryPath,
            ),
          }
        : null,
      viewerId:
        typeof viewer.viewerId === 'string' && viewer.viewerId
          ? viewer.viewerId
          : null,
      autoFollow: !!viewer.autoFollow,
      sourcePane:
        viewer.sourcePane === 'left' || viewer.sourcePane === 'right'
          ? viewer.sourcePane
          : null,
      sourceTabId:
        typeof viewer.sourceTabId === 'string' ? viewer.sourceTabId : null,
      sourceDir:
        typeof viewer.sourceDir === 'string' ? viewer.sourceDir : null,
      isClosable: tab.isClosable !== false,
      isPinned: !!tab.isPinned,
    };
  }

  private fromPersistedWorkspaceTab(tab: any): ReassemblyTab | null {
    const surface = String(tab.surfaceId ?? tab.kind ?? 'dir').trim() || 'dir';
    if (surface === 'viewer') return this.fromPersistedViewerTab(tab);
    if (surface === 'dir') return this.fromPersistedDirTab(tab);

    return {
      id: String(tab.id),
      kind: 'surface',
      path: String(tab.location?.location ?? tab.path ?? ''),
      title: String(tab.title || surface),
      surfaceId: surface,
      persisted: {
        ...tab,
        surfaceId: surface,
      } as TabInfo,
      isClosable: tab.isClosable !== false,
      isPinned: !!tab.isPinned,
    };
  }

  private mergePersistedTabs(): ReassemblyTab[] {
    return this.workspaceService.getTabs(this.paneId)
      .map(tab => this.fromPersistedWorkspaceTab(tab))
      .filter((tab): tab is ReassemblyTab => !!tab);
  }

  private async load(
    requestedPath: string,
    options: {
      preserveCursor?: boolean;
      history?: 'record' | 'none';
      syncTab?: boolean;
      restoreSession?: boolean;
    } = {},
  ): Promise<boolean> {
    if (this.directorySource.isPathInsideArchive(requestedPath)) {
      return this.loadArchivePath(requestedPath, options);
    }

    if (this.isInArchive) {
      this.clearArchiveRuntime();
    }

    const generation = ++this.loadGeneration;
    const previousCursor = options.preserveCursor ? this.cursorPath : null;

    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      const listing = await this.panelSources.listDirectory(
        requestedPath,
        this.showHidden,
      );
      if (generation !== this.loadGeneration) return false;

      const nextPath = listing.path || requestedPath;
      const pathChanged = this.path && this.path !== nextPath;
      this.path = nextPath;

      if (pathChanged && !options.restoreSession) {
        this.filterText = '';
        this.filterVisible = false;
        this.filterWidget?.deactivateContext();

        this.treeExpandedPaths.clear();
        this.treeChildren.clear();
        this.treeLoadingPaths.clear();
      }

      this.entries = [...(listing.entries || [])];

      if (!options.restoreSession) {
        const preserved = previousCursor
          ? this.entries.find(entry => entry.path === previousCursor)
          : null;
        const nextCursor = preserved ?? this.sortedEntries[0] ?? null;

        this.cursorPath = nextCursor?.path ?? null;
        this.selectedPaths = nextCursor
          ? new Set([nextCursor.path])
          : new Set();
        this.anchorPath = nextCursor?.path ?? null;
      }

      this.error = null;

      if ((options.history ?? 'record') === 'record') {
        this.navigation.record(this.path);
      }

      if (options.syncTab !== false) {
        this.syncActiveTab(this.path);
      }

      if (this.activeTabId && this.activeTab?.kind === 'dir') {
        const panelTitle = this.panelSources.titleFor(this.path);
        if (panelTitle) {
          this.workspaceService.updateTabPathAndTitle(
            this.paneId,
            this.activeTabId,
            this.path,
            panelTitle,
          );
        } else {
          this.workspaceService.updateTabPath(
            this.paneId,
            this.activeTabId,
            this.path,
          );
        }
      }

      this.pathHistory.addLocation(this.path, this.paneId);
      void this.refreshDriveCapacity(this.path).then(() => {
        if (generation === this.loadGeneration) {
          this.cdr.markForCheck();
        }
      });

      if (!options.restoreSession) {
        this.syncActiveDirSessionState();
      }

      return true;
    } catch (error) {
      if (generation !== this.loadGeneration) return false;

      if (!options.restoreSession || this.entries.length === 0) {
        this.entries = [];
        this.cursorPath = null;
        this.selectedPaths = new Set();
        this.anchorPath = null;
      }
      this.error = error instanceof Error ? error.message : String(error);
      return false;
    } finally {
      if (generation === this.loadGeneration) {
        this.loading = false;
        this.cdr.markForCheck();
      }
    }
  }

  private createDirectoryTab(path: string): Extract<ReassemblyTab, { kind: 'dir' }> {
    const normalized = path || '/';
    return {
      id: `${this.paneId}-tab-${++this.tabCounter}`,
      kind: 'dir',
      path: normalized,
      title: this.tabTitle(normalized),
      viewMode: this.viewMode,
      sortField: this.sortField,
      sortDirection: this.sortDirection,
      directoriesFirst: this.directoriesFirst,
      expandedPaths: [],
      filterText: '',
      filterVisible: false,
      selectedPaths: [],
      cursorPath: null,
      anchorPath: null,
      scrollTop: 0,
      entries: [],
      navigationEntries: normalized ? [normalized] : [],
      navigationIndex: normalized ? 0 : -1,
      isClosable: true,
      isPinned: false,
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
      filterText: path === current.path ? current.filterText : '',
      filterVisible: path === current.path ? current.filterVisible : false,
      selectedPaths: path === current.path ? current.selectedPaths : [],
      cursorPath: path === current.path ? current.cursorPath : null,
      anchorPath: path === current.path ? current.anchorPath : null,
      scrollTop: path === current.path ? current.scrollTop : 0,
      expandedPaths: path === current.path ? current.expandedPaths : [],
      entries: path === current.path ? current.entries : [],
      navigationEntries: [...this.navigation.snapshotState().entries],
      navigationIndex: this.navigation.snapshotState().index,
    };

    this.tabs = this.tabs.map((tab, i) => i === index ? updated : tab);
    if (updated.kind === 'dir') {
      this.persistDirTabSession(updated);
    }
  }

  private sortEntries(entries: FileInfo[]): FileInfo[] {
    return projectSortedEntries(entries, {
      sortField: this.sortField,
      sortDirection: this.sortDirection,
      directoriesFirst: this.directoriesFirst,
    });
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
    this.markUserInteraction();
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
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
  }

  private setSingleSelection(entry: FileInfo): void {
    this.cursorPath = entry.path;
    this.selectedPaths = new Set([entry.path]);
    this.anchorPath = entry.path;
  }

  private selectRange(anchorPath: string, targetPath: string): void {
    const visible = this.selectableEntries();
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
      // A filter is a projection, not a selection mutation. Keep the selection
      // so clearing the filter restores the user's working set.
      this.cursorPath = null;
      this.anchorPath = null;
      return;
    }

    const cursorStillVisible =
      !!this.cursorPath &&
      visible.some(entry => entry.path === this.cursorPath);

    if (!cursorStillVisible) {
      const selectedVisible = visible.find(entry =>
        this.selectedPaths.has(entry.path),
      );
      this.cursorPath = selectedVisible?.path ?? visible[0].path;
      this.anchorPath = this.cursorPath;
    }

    // Intentionally do not filter selectedPaths here. Hidden selections remain
    // selected and are surfaced through hiddenSelectedCount.
  }

  private moveFilterCursor(delta: -1 | 1): void {
    this.markUserInteraction();
    const visible = this.visibleEntries;
    if (visible.length === 0) return;

    const currentIndex = this.cursorPath
      ? visible.findIndex(entry => entry.path === this.cursorPath)
      : -1;

    const nextIndex = currentIndex < 0
      ? (delta > 0 ? 0 : visible.length - 1)
      : (currentIndex + delta + visible.length) % visible.length;

    const entry = visible[nextIndex];
    this.cursorPath = entry.path;
    this.anchorPath = entry.path;

    // F3 navigation moves the cursor through matches. It must not collapse an
    // existing multi-selection merely because the filter owns keyboard focus.
    if (this.selectedPaths.size <= 1) {
      this.selectedPaths = new Set([entry.path]);
    }

    this.lastAction = delta > 0 ? 'filter next' : 'filter previous';
    this.ensureCursorVisible();
    this.syncActiveDirSessionState();
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