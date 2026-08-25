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
import { OperationHistoryService } from '../services/operation-history.service';
import { ActionFeedbackService } from '../services/action-feedback.service';
import { RecentFilesService } from '../services/recent-files.service';
import { ResultsPanelService } from '../services/results-panel.service';
import { SmartResultsService } from '../services/smart-results.service';
import { calculateVirtualWindow, type VirtualWindow } from '../services/virtual-window';
import { PluginManagerService } from '../services/plugin-manager.service';
import type {
  PanelColumnContribution,
  PanelColumnContext,
  PanelPresentationContribution,
  PanelPresentationModel,
} from '../plugin/plugin-registry-api';
import { PluginLifecycleCoordinatorService } from '../services/plugin-lifecycle-coordinator.service';
import { PanelPresentationOutletComponent } from './panel-presentation-outlet.component';
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
import { AddressBarComponent } from './address-bar.component';
import { FilterComponent } from './filter.component';
import { FileEntryInlineComponent } from './file-entry-inline.component';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { NavigationController } from './navigation.controller';
import type { DirectoryView, PanePresentation, SortDirection, SortField } from './pane-component.contract';
import type { PresentedCommand } from './presented-command';
import { ResponsiveToolbarComponent } from './responsive-toolbar.component';
import { PaneClipboardController, type PaneClipboardHost } from './pane-clipboard.controller';
import { PaneDetailsResizeHandleDirective } from './pane-details-resize-handle.directive';
import {
  collapseNestedDragEntries,
  decideInternalDrop,
  dragOperationFromModifiers,
} from './drag-drop-policy';
import {
  DirListStreamController,
  type DirStreamBatchEvent,
  type DirStreamDoneEvent,
  type DirStreamErrorEvent,
  type DirStreamMetaEvent,
} from '../surfaces/dir/dir-list-stream.controller';
import {
  createFilenameMaskMatcher,
  entryTypeLabel,
  fileExtension,
  projectEntries,
  sortEntries as projectSortedEntries,
} from './entry-model';
import {
  reconcileVisibleSelection,
  selectPathRange,
  togglePathSelection,
} from './selection-policy';

interface TreeRow {
  entry: FileInfo;
  depth: number;
  expanded: boolean;
  loading: boolean;
  expandable: boolean;
}

type Tab =
  | {
      id: string;
      kind: 'dir';
      path: string;
      title: string;
      viewMode: DirectoryView;
      sortField: SortField;
      sortDirection: SortDirection;
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

interface DirTabSession {
  schemaVersion: 4;
  viewMode: DirectoryView;
  sortField: SortField;
  sortDirection: SortDirection;
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

const REASSEMBLY_DIR_TAB_SESSION_VERSION = 4;

// Directory contents are runtime data, not durable UI state. Persisting or
// cloning a WinSxS-sized listing on every selection/scroll change defeats
// virtualization and can lock the renderer for seconds.
const MAX_RUNTIME_TAB_CACHE_ENTRIES = 800;
const MAX_PERSISTED_TAB_CACHE_ENTRIES = 256;
const MAX_LEGACY_SESSION_BYTES = 512 * 1024;

import type { PaneComparisonStatus } from '../services/pane-comparison.service';

@Component({
  selector: 'app-registry-pane',
  standalone: true,
  imports: [
    CommonModule,
    AddressBarComponent,
    PanelPresentationOutletComponent,
    FilterComponent,
    FileEntryInlineComponent,
    ViewerHostSurfaceComponent,
    SurfaceOutletComponent,
    ResponsiveToolbarComponent,
    PaneDetailsResizeHandleDirective,
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

      <div
        #tabStrip
        class="tab-strip"
        role="tablist"
        [attr.aria-label]="paneId + ' pane tabs'"
        (dblclick)="onTabStripDoubleClick($event)"
        (wheel)="onTabStripWheel($event)">
        <button
          *ngFor="let tab of tabs; trackBy: trackTab"
          type="button"
          class="tab"
          role="tab"
          [attr.data-tab-id]="tab.id"
          [attr.aria-selected]="tab.id === activeTabId"
          [class.active]="tab.id === activeTabId"
          [title]="tab.path"
          (click)="activateTab(tab.id)"
          (auxclick)="onTabAuxClick(tab, $event)">
          <span class="tab-title">
            <span class="tab-kind" aria-hidden="true">{{ tab.kind === 'viewer' ? '\u25C9' : (tab.kind === 'surface' ? '\u25C6' : '\u25A3') }}</span>
            <span class="tab-pin-indicator" *ngIf="tab.isPinned" aria-label="Pinned tab">&#x25CF;</span>
            {{ tab.title }}
          </span>
          <span
            class="tab-close"
            role="button"
            [class.disabled]="tab.isPinned || !tab.isClosable"
            [attr.aria-label]="tab.isPinned ? 'Pinned tab' : (!tab.isClosable ? 'Permanent tab' : 'Close tab')"
            (mousedown)="$event.preventDefault(); $event.stopPropagation()"
            (click)="tab.isPinned ? toggleTabPin(tab.id) : (tab.isClosable ? closeTab(tab.id) : null); $event.stopPropagation()">
            {{ tab.isPinned ? '\u25CF' : (tab.isClosable ? '\u00D7' : '\u2022') }}
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
            &#x2190;
          </button>

          <button
            type="button"
            class="nav-button"
            [disabled]="loading || !canForward"
            (click)="goForward()"
            aria-label="Forward"
            title="Forward (Alt+Right)">
            &#x2192;
          </button>

          <button
            type="button"
            class="nav-button"
            [disabled]="loading || atRoot"
            (click)="navigateUp()"
            aria-label="Parent directory"
            title="Parent directory (Backspace)">
            &#x2191;
          </button>
        </div>

        <app-address-bar
          #addressBar
          [paneId]="paneId"
          [path]="path"
          (navigate)="navigateTo($event)">
        </app-address-bar>

        <button
          type="button"
          class="nav-button pin"
          [class.pinned]="currentPathPinned"
          [disabled]="!isDirectoryTab || !path"
          (click)="togglePinRequested.emit()"
          [attr.aria-label]="currentPathPinned ? 'Unpin current location' : 'Pin current location'"
          [title]="currentPathPinned ? 'Unpin current location' : 'Pin current location'">
          {{ currentPathPinned ? '\u2605' : '\u2606' }}
        </button>

        <button
          type="button"
          class="nav-button refresh"
          [disabled]="loading"
          (click)="refresh()"
          aria-label="Refresh"
          title="Refresh (Ctrl+R)">
          &#x21BB;
        </button>
      </header>

      <div
        class="status-row"
        [id]="paneStatusId"
        role="status"
        aria-live="polite"
        aria-atomic="true">
        <span *ngIf="loading">Loading {{ path || 'location' }}&#x2026;</span>
        <span *ngIf="!loading && !error">{{ totalEntryCount }} items</span>
        <span *ngIf="selectedPaths.size">{{ selectedPaths.size }} selected</span>
        <span *ngIf="showHidden" class="hidden-state">hidden visible</span>
        <span *ngIf="clipboard" class="clipboard-state">
          {{ clipboard.operation === 'copy' ? 'Copied' : 'Cut' }}: {{ clipboard.paths.length }}
        </span>
        <span class="hint">Ctrl+L location</span>
      </div>

      <div class="results-collection-bar" *ngIf="activeResultsPanel as panel">
        <span class="collection-title" [title]="panel.path">{{ panel.title }}</span>
        <span class="collection-kind">{{ panel.metadata.kind || panel.metadata.origin || 'results' }}</span>
        <span class="collection-count">{{ panel.entries.length }} items</span>
        <span class="collection-spacer"></span>
        <button type="button"
          [disabled]="loading || !resultsRefreshable"
          title="Refresh this collection"
          (click)="refreshResultsCollection()">&#x21BB;</button>
        <button type="button"
          [class.active]="panel.pinned"
          [title]="panel.pinned ? 'Unpin collection' : 'Pin collection'"
          (click)="toggleResultsCollectionPin()">{{ panel.pinned ? '\u2605' : '\u2606' }}</button>
        <button type="button"
          title="Rename collection"
          (click)="renameResultsCollection()">Rename</button>
      </div>

      <div class="action-bar registry-action-bar">
        <app-responsive-toolbar
          [actions]="registryToolbarActions"
          [minVisible]="0"
          [showIcons]="false"
          align="left"
          ariaLabel="Registry actions"
          overflowLabel="More registry actions">
        </app-responsive-toolbar>
      </div>

      <app-filter
        #filterWidget
        [paneId]="paneId"
        [visible]="filterVisible"
        [value]="filterText"
        [matchCount]="visibleEntryCount"
        [totalCount]="totalEntryCount"
        (valueChange)="setFilter($event)"
        (close)="closeFilter()"
        (next)="nextFilterMatch()"
        (previous)="previousFilterMatch()">
      </app-filter>

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
        data-pane-content
        *ngIf="!error && viewMode === 'tree'"
        tabindex="-1"
        role="tree"
        [attr.aria-label]="'Registry details of ' + (path || 'location')"
        [attr.aria-busy]="loading"
        (click)="onBackgroundClick($event)"
        (contextmenu)="onBackgroundContextMenu($event)"        (scroll)="onContentScroll($event)">
        <div
          *ngIf="treeVirtualWindow.enabled && treeVirtualWindow.beforePx > 0"
          class="virtual-spacer tree-virtual-spacer"
          [style.height.px]="treeVirtualWindow.beforePx"
          aria-hidden="true"></div>

        <div
          *ngFor="let row of renderedTreeRows; trackBy: trackTreeRow"
          class="tree-row"
          tabindex="-1"
          [attr.data-file-entry]="row.entry.path"
          role="treeitem"
          [class.selected]="selectedPaths.has(row.entry.path)"
          [class.cursor]="cursorPath === row.entry.path"
          [class.compare-only]="comparisonStatus(row.entry.path)?.startsWith('only-')"
          [class.compare-newer]="comparisonStatus(row.entry.path)?.endsWith('-newer')"
          [class.compare-different]="isComparisonDifferent(row.entry.path)"
          [class.drop-target]="dragOverPath === row.entry.path"
          [class.drop-invalid]="dragOverPath === row.entry.path && dropIntent === 'invalid'"
          [class.drop-copy]="dragOverPath === row.entry.path && dropIntent === 'copy'"
          [class.drop-move]="dragOverPath === row.entry.path && dropIntent === 'move'"
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
            {{ row.loading ? '\u2026' : (row.expandable ? (row.expanded ? '\u25BE' : '\u25B8') : '') }}
          </span>
          <span class="icon">{{ entryIcon(row.entry) }}</span>
          <app-file-entry-inline
            *ngIf="renamingPath !== row.entry.path"
            class="name tree-name"
            [entry]="row.entry"
            [showIcon]="false">
          </app-file-entry-inline>
          <span
            class="rename-editor"
            *ngIf="renamingPath === row.entry.path"
            (click)="$event.stopPropagation()"
            (dblclick)="$event.stopPropagation()">
            <input
              class="rename-input"
              [class.rename-input--invalid]="!!renameError"
              [value]="renameValue"
              [disabled]="renameBusy"
              [attr.data-rename-input]="row.entry.path"
              [attr.aria-invalid]="renameError ? 'true' : null"
              [attr.aria-describedby]="renameError ? 'rename-error-' + paneId : null"
              (input)="onRenameInput($event)"
              (keydown)="onRenameKeyDown(row.entry, $event)"
              (blur)="onRenameBlur(row.entry)" />
            <span
              *ngIf="renameError"
              class="rename-error"
              [id]="'rename-error-' + paneId"
              role="alert">{{ renameError }}</span>
          </span>
          <span class="tree-meta" *ngIf="row.entry.type !== FileType.DIRECTORY && renamingPath !== row.entry.path">
            {{ formatSize(row.entry.size) }}
          </span>
        </div>

        <div
          *ngIf="treeVirtualWindow.enabled && treeVirtualWindow.afterPx > 0"
          class="virtual-spacer tree-virtual-spacer"
          [style.height.px]="treeVirtualWindow.afterPx"
          aria-hidden="true"></div>

        <div class="empty" *ngIf="!loading && treeRows.length === 0" role="status">
          <strong>This directory is empty.</strong>
          <span *ngIf="canCreate && !isInArchive">Use F7 to create a folder or Command Palette for New File.</span>
        </div>
      </div>

      <app-panel-presentation-outlet
        *ngIf="!error && activePanelPresentation as presentation"
        [presentation]="presentation"
        [model]="panelPresentationModel">
      </app-panel-presentation-outlet>


      <div
        #listViewport
        class="list"
        data-pane-content
        data-pane-details-root
        *ngIf="!activePanelPresentation && !error && viewMode !== 'tree'"
        tabindex="-1"
        role="listbox"
        [attr.aria-label]="'Registry entries in ' + (path || 'location')"
        [class.view-list]="viewMode === 'list'"
        [class.view-grid]="viewMode === 'grid'"
        [class.view-details]="viewMode === 'details'"
        [attr.aria-busy]="loading"
        (scroll)="onContentScroll($event)"
        [class.background-drop-target]="backgroundDropActive"
        (click)="onBackgroundClick($event)"        (contextmenu)="onBackgroundContextMenu($event)"
        (dragover)="onBackgroundDragOver($event)"
        (dragleave)="onBackgroundDragLeave($event)"
        (drop)="onBackgroundDrop($event)">

      <div class="details-header-viewport" *ngIf="!activePanelPresentation && !error && viewMode === 'details'">
        <div class="details-header" [style.grid-template-columns]="detailsGridTemplate">
        <span></span>
        <span>Name<i [paneDetailsResizeHandle]="'name'" [paneDetailsMinWidth]="120"></i></span>
        <span>Kind<i [paneDetailsResizeHandle]="'kind'" [paneDetailsMinWidth]="72"></i></span>
        <span>Type<i [paneDetailsResizeHandle]="'type'" [paneDetailsMinWidth]="72"></i></span>
        <span>Value<i [paneDetailsResizeHandle]="'value'" [paneDetailsMinWidth]="100"></i></span>
        <span *ngFor="let column of pluginPanelColumns" [title]="column.label">
          {{ column.label }}<i [paneDetailsResizeHandle]="'plugin-' + column.id" [paneDetailsMinWidth]="72"></i>
        </span>
      </div>
      </div>

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
          (click)="onRegistryEntryClick(entry, $event)"
          (dblclick)="openEntry(entry)"
          (contextmenu)="onEntryContextMenu(entry, $event)"
          (dragstart)="onEntryDragStart(entry, $event)"
          (dragend)="onDragEnd()"
          (dragover)="onEntryDragOver(entry, $event)"
          (dragleave)="onEntryDragLeave(entry)"
          (drop)="onEntryDrop(entry, $event)"
          [class.drop-target]="dragOverPath === entry.path"
          [class.drop-invalid]="dragOverPath === entry.path && dropIntent === 'invalid'"
          [class.drop-copy]="dragOverPath === entry.path && dropIntent === 'copy'"
          [class.drop-move]="dragOverPath === entry.path && dropIntent === 'move'"
          [style.grid-template-columns]="viewMode === 'details' ? detailsGridTemplate : null">

          <span class="icon" aria-hidden="true" [style.color]="entryCategory(entry)?.color || null">
            {{ entryDisplayIcon(entry) }}
          </span>

          <app-file-entry-inline
            *ngIf="renamingPath !== entry.path"
            class="name entry-name"
            [entry]="entry"
            [showIcon]="false">
          </app-file-entry-inline>
          <span
            class="rename-editor"
            *ngIf="renamingPath === entry.path"
            (click)="$event.stopPropagation()"
            (dblclick)="$event.stopPropagation()">
            <input
              class="rename-input"
              [class.rename-input--invalid]="!!renameError"
              [value]="renameValue"
              [disabled]="renameBusy"
              [attr.data-rename-input]="entry.path"
              [attr.aria-invalid]="renameError ? 'true' : null"
              [attr.aria-describedby]="renameError ? 'rename-error-' + paneId : null"
              (input)="onRenameInput($event)"
              (keydown)="onRenameKeyDown(entry, $event)"
              (blur)="onRenameBlur(entry)" />
            <span
              *ngIf="renameError"
              class="rename-error"
              [id]="'rename-error-' + paneId"
              role="alert">{{ renameError }}</span>
          </span>

          <span class="kind" [style.color]="entryCategory(entry)?.color || null">
            {{ entryDisplayKind(entry) }}
          </span>

          <span class="size">{{ entryDisplayType(entry) }}</span>
          <span class="modified" [title]="entryDisplayType(entry) + (entryDisplayValue(entry) ? ' · ' + entryDisplayValue(entry) : '')">{{ entryDisplayValue(entry) }}</span>
          <span class="plugin-column" *ngFor="let column of pluginPanelColumns">{{ panelColumnValue(column, entry) }}</span>
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

      <div
        class="drop-intent"
        *ngIf="dropIntentText"
        [class.invalid]="dropIntent === 'invalid'"
        role="status"
        aria-live="polite">
        {{ dropIntentText }}
      </div>

      <footer class="pane-footer">
        <span class="footer-item" *ngIf="cursorEntry as cursor" [title]="cursor.path">
          <strong>{{ cursor.name }}</strong>
        </span>

        <span class="footer-item" *ngIf="selectedPaths.size">
          {{ selectedPaths.size }} selected
          <ng-container *ngIf="selectedTotalBytes > 0">
            &#x00B7; {{ formatSize(selectedTotalBytes) }}
          </ng-container>
        </span>

        <span class="footer-item" *ngIf="filterText">
          {{ visibleEntryCount }} / {{ totalEntryCount }} visible
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
            &#x00B7; {{ driveFileSystem }}
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

            <button type="button" [disabled]="!viewerSupports('showPrevious')" (click)="viewerInvoke('showPrevious')" title="Previous item (Left)">&#x2039;</button>
            <button type="button" [disabled]="!viewerSupports('showNext')" (click)="viewerInvoke('showNext')" title="Next item (Right)">&#x203A;</button>
            <button type="button" [disabled]="!viewerSupports('zoomOut')" (click)="viewerInvoke('zoomOut')" title="Zoom out (Ctrl+-)">&#x2212;</button>
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
              &#x00D7;
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
    /* Universal entry-name clipping. */
    .entry .name,
    .list.view-list .entry .name,
    .list.view-details .entry .name,
    .list.view-grid .entry .name,
    .tree-row .name {
      min-width: 0;
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }


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
      background: var(--vsc-editor-background, var(--vsc-editor-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
      border: 1px solid transparent;
    
      overscroll-behavior: none;}

    /* Pane-level active marker: this is the only large active rectangle. */
    .pane.active-pane {
      border-color: color-mix(
        in srgb,
        var(--vsc-accent-blue, var(--vsc-focus-border)) 62%,
        transparent
      );
    }



    .tab-strip {
      display: flex;
      align-items: stretch;
      min-height: 31px;
      overflow-x: auto;
      overflow-y: hidden;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-control-background));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
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
      border-right: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      background: var(--vsc-tab-inactive-background, var(--vsc-toolbar-background));
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      cursor: pointer;
      font: inherit;
    }

    .tab {
      display: grid;
      transition: background-color 80ms ease, color 80ms ease, box-shadow 80ms ease;
      grid-template-columns: minmax(48px, 1fr) 20px;
      align-items: center;
      width: min(180px, 34vw);
      padding: 0 4px 0 9px;
      text-align: left;
    }

    .tab.active {
      background: var(--vsc-editor-background, var(--vsc-editor-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
      box-shadow: inset 0 2px 0 var(--vsc-accent-blue, var(--vsc-accent-blue));
    }

    .tab:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
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
      background: var(--vsc-list-hover-background, var(--vsc-button-secondary));
      opacity: 1;
    }

    .new-tab {
      width: 32px;
      background: transparent;
      font-size: 18px;
    }

    .new-tab:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }

    .navigation-bar {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) 32px 34px;
      align-items: center;
      gap: 7px;
      padding: 6px 8px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-control-background));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
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
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .nav-button:disabled {
      opacity: .28;
      cursor: default;
    }

    .nav-button.pin {
      font-size: 16px;
    }

    .nav-button.pin.pinned {
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
    }

    .status-row {
      display: flex;
      gap: 14px;
      min-height: 24px;
      align-items: center;
      padding: 0 10px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
    }

    .hint {
      margin-left: auto;
      opacity: .7;
    }

    .clipboard-state,
    .hidden-state {
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
    }

    .results-collection-bar{
      display:flex;
      align-items:center;
      gap:6px;
      min-height:25px;
      padding:2px 6px;
      border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border));
      background:var(--vsc-sideBar-background,var(--vsc-panel-background));
      color:var(--vsc-description-foreground,var(--vsc-foreground-dim));
      font:10px/1.2 var(--vsc-font-family,system-ui,sans-serif);
    }
    .results-collection-bar .collection-title{
      max-width:260px;
      overflow:hidden;
      text-overflow:ellipsis;
      white-space:nowrap;
      color:var(--vsc-foreground);
      font-weight:600;
    }
    .results-collection-bar .collection-kind{
      padding:1px 5px;
      border:1px solid var(--vsc-border-subtle,var(--vsc-border));
      border-radius:8px;
    }
    .results-collection-bar .collection-spacer{flex:1}
    .results-collection-bar button{
      min-height:20px;
      padding:1px 6px;
      border:1px solid transparent;
      border-radius:3px;
      background:transparent;
      color:inherit;
      font:inherit;
      cursor:pointer;
    }
    .results-collection-bar button:hover:not(:disabled){
      border-color:var(--vsc-border);
      background:var(--vsc-list-hover-background);
      color:var(--vsc-foreground);
    }
    .results-collection-bar button.active{color:var(--vsc-accent-blue)}
    .results-collection-bar button:disabled{opacity:.4;cursor:default}

    .action-bar {
      display: flex;
      align-items: center;
      gap: 4px;
      min-height: 34px;
      min-width: 0;
      overflow: hidden;
      padding: 3px 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
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
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      border-color: var(--vsc-border, var(--vsc-control-background));
    }

    .action-bar button:disabled {
      opacity: .32;
      cursor: default;
    }

    .action-bar .danger:not(:disabled) {
      color: var(--vsc-error-foreground, var(--vsc-error));
    }

    .view-switcher {
      display: flex;
      gap: 2px;
    }

    .view-switcher button.active {
      background: var(--vsc-list-active-background, var(--vsc-list-selected-emphasis));
      color: var(--vsc-list-active-foreground, var(--vsc-on-accent));
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
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 3px;
      background: var(--vsc-input-background, var(--vsc-popover-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
      font: inherit;
    }

    .sort-controls button.active {
      background: var(--vsc-list-active-background, var(--vsc-list-selected-emphasis));
      color: var(--vsc-list-active-foreground, var(--vsc-on-accent));
    }

    .tree-view {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      padding: 4px 0;
      background: var(--vsc-editor-background, var(--vsc-editor-background));
    
      overscroll-behavior: none;
      scroll-behavior: auto;
      scroll-padding-top: 25px;
      touch-action: pan-y;}

    .entry.compare-only,
    .tree-row.compare-only {
      background: transparent;
    }

    .entry.compare-newer,
    .tree-row.compare-newer {
      background: color-mix(
        in srgb,
        var(--vsc-focus-ring, var(--vsc-accent-blue)) 8%,
        transparent
      );
    }

    .entry.compare-different,
    .tree-row.compare-different {
      background: color-mix(
        in srgb,
        var(--vsc-errorForeground, var(--vsc-foreground)) 6%,
        transparent
      );
    }

    .tree-row {
      display: grid;
      grid-template-columns: 18px 24px minmax(0, 1fr) auto;

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
    
      column-gap: var(--pane-column-gap, 6px);
}

    .tree-row:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .tree-row.selected {
      background: var(--vsc-list-active-background, var(--vsc-list-selected-emphasis));
      color: var(--vsc-list-active-foreground, var(--vsc-on-accent));
    }

    .tree-row.cursor {
      outline: 1px solid color-mix(
        in srgb,
        var(--vsc-focus-border, var(--vsc-accent-blue)) 60%,
        transparent
      );
      outline-offset: -1px;
    }

    .tree-expander {
      display: grid;
      place-items: center;
      width: 18px;
      height: 22px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      cursor: pointer;
      user-select: none;
    }

    .tree-expander.placeholder {
      cursor: default;
    }

    .tree-meta {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 9px;
    }


    .details-header-viewport {
      position: sticky;
      top: 0;
      z-index: 4;
      width: max-content;
      min-width: 100%;
      overflow: visible;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
    }

    .details-header {
      display: grid;
      grid-template-columns: 28px minmax(120px, 1fr) 90px 82px 150px;

      align-items: center;
      min-height: 25px;
      padding: 0 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
    
      column-gap: var(--pane-column-gap, 6px);
      width: max-content;
      min-width: 100%;
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
    
      column-gap: var(--pane-column-gap, 6px);
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
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
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
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
    }

    .action-separator {
      width: 1px;
      height: 18px;
      margin: 0 3px;
      background: var(--vsc-border, var(--vsc-control-background));
    }

    .virtual-spacer{
      flex:0 0 auto;
      min-width:1px;
      pointer-events:none;
      visibility:hidden;
    }
    .tree-virtual-spacer{width:100%}
    .list.view-grid .list-virtual-spacer{
      grid-column:1 / -1;
      width:100%;
    }
    .list.view-list .list-virtual-spacer,
    .list.view-details .list-virtual-spacer{
      width:100%;
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
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .entry.cursor {
      box-shadow: inset 2px 0 0 var(--vsc-accent-blue, var(--vsc-accent-blue));
      background: color-mix(
        in srgb,
        var(--vsc-list-hover-background, var(--vsc-list-hover-background)) 75%,
        transparent
      );
    }

    .entry.selected {
      background: var(--vsc-list-active-background, var(--vsc-list-selected-emphasis));
      color: var(--vsc-list-active-foreground, var(--vsc-on-accent));
    }


    .entry.drop-target {
      outline: 1px solid var(--vsc-accent-blue, var(--vsc-accent-blue));
      outline-offset: -1px;
      background: color-mix(
        in srgb,
        var(--vsc-accent-blue, var(--vsc-accent-blue)) 18%,
        var(--vsc-list-hover-background, var(--vsc-list-hover-background))
      );
    }

    .list.background-drop-target {
      box-shadow: inset 0 0 0 2px color-mix(
        in srgb,
        var(--vsc-accent-blue, var(--vsc-accent-blue)) 72%,
        transparent
      );
    }

    .entry.drop-move,.tree-row.drop-move,.list.drop-move{
      outline-color:var(--vsc-success,var(--vsc-accent-green));
    }
    .entry.drop-invalid,.tree-row.drop-invalid,.list.drop-invalid{
      outline-color:var(--vsc-error,var(--vsc-danger));
      cursor:not-allowed;
    }
    .drop-intent{
      position:absolute;
      z-index:30;
      right:8px;
      bottom:30px;
      max-width:min(320px,70%);
      padding:4px 8px;
      border:1px solid var(--vsc-focus-border,var(--vsc-accent-blue));
      border-radius:4px;
      background:var(--vsc-menu-background,var(--vsc-panel-background));
      color:var(--vsc-menu-foreground,var(--vsc-foreground));
      box-shadow:0 3px 12px var(--vsc-scrim);
      pointer-events:none;
      font-size:10px;
      white-space:nowrap;
      overflow:hidden;
      text-overflow:ellipsis;
    }
    .drop-intent.invalid{
      border-color:var(--vsc-error,var(--vsc-danger));
      color:var(--vsc-error,var(--vsc-danger));
    }

    .entry.cursor.selected {
      box-shadow: inset 2px 0 0 var(--vsc-focus-border, var(--vsc-focus-ring));
    }

    .icon {
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
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
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
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .error {
      flex: 1 1 auto;
    }

    .pane-footer {
      display: flex;
      flex: 0 0 auto;
      align-items: center;
      gap: 10px;
      min-height: 27px;
      margin-top: auto;
      padding: 0 8px;
      border-top: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
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
      color: var(--vsc-foreground, var(--vsc-foreground));
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
      color: var(--vsc-foreground, var(--vsc-foreground));
      font-variant-numeric: tabular-nums;
    }

    .footer-item.capability {
      flex: 0 0 auto;
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      text-transform: uppercase;
      letter-spacing: .04em;
      font-size: 9px;
    }

    .footer-item.disabled-capability {
      opacity: .28;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .error button {
      padding: 5px 10px;
    }

    app-panel-presentation-outlet,
    .viewer-surface,
    .plugin-surface,
    app-viewer-host-surface,
    app-surface-outlet{
      display:flex;
      flex:1 1 auto;
      min-width:0;
      min-height:0;
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
      background: var(--vsc-editor-background, var(--vsc-editor-background));
    }

    .viewer-toolbar {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) 28px;
      align-items: center;
      gap: 8px;
      min-height: 32px;
      padding: 0 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
      font-size: 11px;
    }

    .viewer-label {
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      font-weight: 600;
    }

    .viewer-kind {
      flex: 0 0 auto;
      padding: 1px 5px;
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 3px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 9px;
      text-transform: uppercase;
    }

    .viewer-separator {
      width: 1px;
      height: 18px;
      background: var(--vsc-border, var(--vsc-border));
      flex: 0 0 auto;
    }

    .viewer-path {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
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
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
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
    .list:focus,
    .list:focus-visible,
    .tree-view:focus,
    .tree-view:focus-visible {
      outline: none !important;
      box-shadow: none !important;
    }

    .pane:focus-visible {
      outline: none;
    }
    .pane:focus-within .tab.active{
      color:var(--vsc-foreground);
    }
    .tab:focus-visible,
    .new-tab:focus-visible,
    .nav-button:focus-visible,
    .action-bar button:focus-visible,
    .action-bar select:focus-visible{
      outline:1px solid var(--vsc-focus-border,var(--vsc-accent-blue));
      outline-offset:-1px;
    }
    .empty{display:grid;gap:4px;place-items:center;align-content:center;min-height:96px;text-align:center}
    .rename-editor{
      position:relative;
      min-width:0;
      width:100%;
      display:block;
      z-index:2;
    }
    .rename-input{
      box-sizing:border-box;
      width:100%;
      min-width:56px;
      height:22px;
      padding:1px 4px;
      border:1px solid var(--vsc-focus-border,var(--vsc-accent-blue));
      border-radius:2px;
      outline:0;
      background:var(--vsc-input-background,var(--vsc-control-background));
      color:var(--vsc-input-foreground,var(--vsc-foreground));
      font:inherit;
    }
    .rename-input:focus{
      box-shadow:0 0 0 1px color-mix(in srgb,var(--vsc-focus-border,var(--vsc-accent-blue)) 45%,transparent);
    }
    .rename-input--invalid{
      border-color:var(--vsc-error,var(--vsc-danger));
    }
    .rename-error{
      position:absolute;
      z-index:40;
      top:calc(100% + 3px);
      left:0;
      max-width:min(360px,70vw);
      padding:4px 7px;
      border:1px solid var(--vsc-error,var(--vsc-danger));
      border-radius:3px;
      background:var(--vsc-menu-background,var(--vsc-panel-background));
      color:var(--vsc-error,var(--vsc-danger));
      box-shadow:0 4px 12px var(--vsc-scrim);
      font-size:10px;
      line-height:1.35;
      white-space:normal;
    }
    .empty strong{font-weight:500;color:var(--vsc-foreground,var(--vsc-foreground))}
    .error-actions{display:flex;gap:6px;margin-top:7px}

    /* visible pane column gutters */
    .details-header,
    .list.view-list .entry,
    .list.view-details .entry,
    .tree-row {
      column-gap: var(--pane-column-gutter, 12px);
    }

    .details-header > span:not(:first-child),
    .list.view-list .entry > span:not(:first-child),
    .list.view-details .entry > span:not(:first-child) {
      min-width: 0;
      padding-inline: 2px;
    }

    .list.view-list .entry > app-file-entry-inline,
    .list.view-details .entry > app-file-entry-inline,
    .tree-row > app-file-entry-inline {
      display: block;
      min-width: 0;
      width: 100%;
      max-width: 100%;
      padding-inline: 2px;
      overflow: hidden;
    }


    /*
     * Horizontal cursor coverage:
     * List/Details/Tree use one shared intrinsic-width track. The widest row
     * establishes the horizontal scroll extent and every sibling stretches to
     * that same extent, so cursor/selection never ends before nested or wider
     * rows. Grid tiles intentionally keep their normal tile width.
     */
    .list.view-list,
    .list.view-details,
    .tree-view {
      display: grid;
      grid-template-columns: minmax(100%, max-content);
      align-content: start;
    }

    .list.view-list .entry,
    .list.view-details .entry,
    .tree-row,
    .list.view-list .list-virtual-spacer,
    .list.view-details .list-virtual-spacer,
    .tree-virtual-spacer {
      width: 100%;
      min-width: 100%;
    }

    /*
     * Details is one solid horizontal surface: the header and every row share
     * the same grid track. Keep outer table geometry flush; cell gutters own
     * the spacing. Grid items use stretch rather than percentage widths so
     * max-content sizing cannot collapse a cursor against an indefinite track.
     */
    .list.view-details {
      padding: 0;
    }

    .list.view-details .details-header-viewport,
    .list.view-details .entry,
    .list.view-details .list-virtual-spacer {
      width: auto;
      min-width: 0;
      justify-self: stretch;
    }

    .list.view-details .details-header {
      width: 100%;
      min-width: 100%;
      margin: 0;
      padding: 0;
    }

    .list.view-details .entry {
      margin-right: 0;
      padding-left: 0;
      padding-right: 0;
      border-radius: 3px;
    }

    /* Shared horizontal surface: rows fill the available viewport width and,
     * when columns overflow, expand with the complete scrollable track.
     * The 4px row inset is preserved on both sides. */
    .list.view-list,
    .list.view-details {
      display: grid;
      grid-template-columns: minmax(100%, max-content);
      align-content: start;
      padding: 0;
    }

    .list.view-list > .entry,
    .list.view-details > .entry {
      grid-column: 1;
      width: auto !important;
      min-width: 0 !important;
      justify-self: stretch;
      margin-left: 4px;
      margin-right: 4px;
    }

    .list.view-list > .list-virtual-spacer,
    .list.view-details > .list-virtual-spacer,
    .list.view-details > .details-header-viewport {
      grid-column: 1;
      width: auto !important;
      min-width: 0 !important;
      justify-self: stretch;
    }

    .list.view-details > .details-header-viewport,
    .list.view-details > .details-header-viewport > .details-header {
      margin: 0;
      padding: 0;
    }

    .list.view-details > .details-header-viewport > .details-header {
      width: 100%;
      min-width: 100%;
    }
  `],
})
export class RegistryPaneComponent implements OnInit, OnDestroy {
  readonly FileType = FileType;

  @ViewChild('addressBar') addressBar?: AddressBarComponent;
  @ViewChild('viewerHost') viewerHost?: ViewerHostSurfaceComponent;
  @ViewChild('surfaceOutlet') surfaceOutlet?: SurfaceOutletComponent;
  @ViewChild('filterWidget') filterWidget?: FilterComponent;
  @ViewChild('paneRoot') paneRoot?: ElementRef<HTMLElement>;
  @ViewChild('listViewport') listViewport?: ElementRef<HTMLElement>;
  @ViewChild('treeViewport') treeViewport?: ElementRef<HTMLElement>;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() active = false;
  @Input() showHidden = false;
  @Input() currentPathPinned = false;
  @Input() pluginToolbarActions: readonly PresentedCommand[] = [];
  @Input() comparisonStatuses: ReadonlyMap<string, PaneComparisonStatus> =
    new Map<string, PaneComparisonStatus>();
  @Output() activateRequested = new EventEmitter<void>();
  @Output() navigationTrailRequested = new EventEmitter<{
    direction: 'back' | 'forward';
    event: MouseEvent;
    entries: readonly string[];
  }>();
  @Output() remoteRecoveryRequested = new EventEmitter<void>();
  @Output() togglePinRequested = new EventEmitter<void>();
  @Output() locationChange = new EventEmitter<string>();
  @Output() cursorChange = new EventEmitter<FileInfo | null>();
  @Output() presentationChange = new EventEmitter<PanePresentation>();
  @Output() viewportChange = new EventEmitter<{ topName: string | null; ratio: number }>();
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

  renamingPath: string | null = null;
  renameValue = '';
  renameError: string | null = null;
  renameBusy = false;
  private suppressRenameBlur = false;
  private applyingMirroredPresentation = false;

  driveFreeBytes = 0;
  driveTotalBytes = 0;
  driveFileSystem = '';

  readonly viewMode: DirectoryView = 'details';
  sortField: SortField = 'name';
  sortDirection: SortDirection = 'asc';
  directoriesFirst = true;

  readonly treeExpandedPaths = new Set<string>();
  readonly treeChildren = new Map<string, FileInfo[]>();
  readonly treeLoadingPaths = new Set<string>();
  lastAction = 'startup';

  tabs: Tab[] = [];
  activeTabId: string | null = null;
  private tabCounter = 0;
  private workspaceTabsReady = false;
  private workspaceRestoreGeneration = 0;
  private interactionGeneration = 0;
  private scrollPersistTimer: ReturnType<typeof setTimeout> | null = null;
  private projectionEntriesRef: FileInfo[] | null = null;
  private projectionSortField = '';
  private projectionSortDirection = '';
  private projectionDirectoriesFirst = true;
  private projectionFilterText = '';
  private projectionCache: FileInfo[] = [];
  private entryCategoryRef: FileInfo[] | null = null;
  private entryCategoryCache = new Map<string, ReturnType<PluginManagerService['resolveFileCategory']> | null>();
  private entryLookupRef: FileInfo[] | null = null;
  private entryLookup = new Map<string, FileInfo>();
  private treeRowsVisibleRef: FileInfo[] | null = null;
  private treeRowsCacheVersion = -1;
  private treeRowsStateVersion = 0;
  private treeRowsCache: TreeRow[] = [];
  // Backend streams arrive already filtered and ordered. Keeping that fact
  // explicit prevents a completed large listing from being synchronously
  // projected again by Angular.
  private backendPresentationActive = false;
  private backendSessionId: string | null = null;
  private backendVisibleCount = 0;
  private backendSourceCount = 0;
  private backendWindowOffset = 0;
  private backendWindowLimit = 0;
  private backendWindowRequestToken = 0;
  private backendWindowFetchPromise: Promise<void> | null = null;
  private backendWindowQueuedRange: { start: number; end: number } | null = null;
  private backendProgressRefreshTimer: ReturnType<typeof setTimeout> | null = null;
  private presentationReloadTimer: ReturnType<typeof setTimeout> | null = null;

  private virtualScrollTop = 0;
  private virtualViewportHeight = 600;
  private virtualViewportWidth = 800;

  private readonly virtualThreshold = 300;
  private readonly listRowExtent = 30;
  private readonly gridRowExtent = 109;
  private readonly treeRowExtent = 31;

  get paneStatusId(): string {
    return `${this.paneId}-status`;
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

  async canNavigateToLocation(path: string): Promise<boolean> {
    if (!path || !this.isDirectoryTab) return false;
    try {
      await this.panelSources.listDirectory(path, this.showHidden);
      return true;
    } catch {
      return false;
    }
  }

  async currentLocationAvailable(): Promise<boolean> {
    if (!this.isDirectoryTab || !this.path) return false;
    return this.canNavigateToLocation(this.path);
  }

  recoverVisibleCursor(): void {
    const visible = this.selectableEntries();

    if (!visible.length) {
      this.selectedPaths = new Set();
      this.cursorPath = null;
      this.anchorPath = null;
      this.cursorChange.emit(null);
      this.syncActiveDirSessionState();
      this.cdr.markForCheck();
      return;
    }

    const current =
      this.cursorPath !== null
        ? visible.find(entry => entry.path === this.cursorPath)
        : undefined;

    const fallback: FileInfo =
      current ??
      visible.find(entry => this.selectedPaths.has(entry.path)) ??
      visible[0];

    this.setSingleSelection(fallback);
    this.ensureCursorVisible();
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
  }

  async navigateLinkedLocation(
    path: string,
    options: { history?: 'record' | 'none'; preserveCursor?: boolean } = {},
  ): Promise<boolean> {
    if (!path || !this.isDirectoryTab) return false;

    return this.load(path, {
      history: options.history ?? 'record',
      preserveCursor: options.preserveCursor ?? false,
    });
  }

  selectPeerByName(name: string): boolean {
    if (!name || !this.isDirectoryTab) return false;
    const folded = name.toLowerCase();
    const entry =
      this.selectableEntries().find(candidate => candidate.name.toLowerCase() === folded) ??
      this.entries.find(candidate => candidate.path !== '__parent__' && candidate.name.toLowerCase() === folded);
    if (!entry) return false;

    this.setSingleSelection(entry);
    this.lastAction = `follow: ${entry.name}`;
    this.ensureCursorVisible();
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
    return true;
  }

  panePresentation(): PanePresentation {
    return {
      viewMode: 'details',
      sortField: this.sortField,
      sortDirection: this.sortDirection,
      directoriesFirst: this.directoriesFirst,
      filterText: this.filterText,
      filterVisible: this.filterVisible,
    };
  }

  viewportSnapshot(): { topName: string | null; ratio: number } {
    const viewport = this.viewMode === 'tree'
      ? this.treeViewport?.nativeElement
      : this.listViewport?.nativeElement;
    if (!viewport) return { topName: null, ratio: 0 };

    const max = Math.max(1, viewport.scrollHeight - viewport.clientHeight);
    const ratio = Math.max(0, Math.min(1, viewport.scrollTop / max));
    const viewportTop = viewport.getBoundingClientRect().top;
    const rows = Array.from(
      viewport.querySelectorAll<HTMLElement>('[data-file-entry]'),
    );
    const firstVisible = rows.find(row =>
      row.getBoundingClientRect().bottom >= viewportTop
    );
    const path = firstVisible?.getAttribute('data-file-entry') ?? null;
    const entry = path
      ? this.entries.find(candidate => candidate.path === path)
      : null;

    return { topName: entry?.name ?? null, ratio };
  }

  syncViewportToPeer(snapshot: { topName: string | null; ratio: number }): void {
    const viewport = this.viewMode === 'tree'
      ? this.treeViewport?.nativeElement
      : this.listViewport?.nativeElement;
    if (!viewport) return;

    if (snapshot.topName) {
      const folded = snapshot.topName.toLowerCase();
      const entry = this.entries.find(candidate =>
        candidate.path !== '__parent__' &&
        candidate.name.toLowerCase() === folded
      );

      if (entry) {
        const escaped =
          typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
            ? CSS.escape(entry.path)
            : entry.path.replace(/["\\]/g, '\\$&');
        const row = viewport.querySelector<HTMLElement>(
          `[data-file-entry="${escaped}"]`,
        );
        if (row) {
          row.scrollIntoView({ block: 'start' });
          return;
        }
      }
    }

    const max = Math.max(0, viewport.scrollHeight - viewport.clientHeight);
    viewport.scrollTop = Math.round(
      max * Math.max(0, Math.min(1, snapshot.ratio)),
    );
  }


  comparisonStatus(path: string): PaneComparisonStatus | null {
    return this.comparisonStatuses.get(path) ?? null;
  }

  isComparisonDifferent(path: string): boolean {
    const status = this.comparisonStatus(path);
    return status === 'size-different' ||
      status === 'hash-different' ||
      status === 'content-different' ||
      status === 'type-different';
  }

  selectComparisonItem(path: string): boolean {
    const entry = this.entries.find(candidate => candidate.path === path);
    if (!entry) return false;
    this.setSingleSelection(entry);
    this.ensureCursorVisible();
    this.focusContent();
    this.cdr.markForCheck();
    return true;
  }

  applyMirroredPresentation(
    value: PanePresentation,
    options: { filter: boolean },
  ): void {
    if (!this.isDirectoryTab) return;

    this.applyingMirroredPresentation = true;
    try {
      this.sortField = value.sortField;
      this.sortDirection = value.sortDirection;
      this.directoriesFirst = value.directoriesFirst;

      if (options.filter) {
        this.filterText = value.filterText;
        this.filterVisible = value.filterVisible;
        this.reconcileSelectionWithVisibleEntries();
      }

      this.syncActiveDirSessionState();
      this.lastAction = options.filter
        ? 'mirrored pane presentation + filter'
        : 'mirrored pane presentation';
      queueMicrotask(() => this.ensureCursorVisible());
      this.cdr.markForCheck();
    } finally {
      this.applyingMirroredPresentation = false;
    }
  }

  private emitPresentationChange(): void {
    if (this.applyingMirroredPresentation) return;
    this.presentationChange.emit(this.panePresentation());
  }

  entryAriaLabel(entry: FileInfo): string {
    const kind = this.registryEntryKind(entry);
    const type = entry.registryValueType?.trim() || '';
    const data = entry.registryPreview?.trim() || '';

    const registryDetail =
      kind === 'Value'
        ? [type || 'Registry value', data].filter(Boolean).join(', ')
        : kind;

    const comparison = this.comparisonStatus(entry.path);
    const comparisonText = comparison && comparison !== 'same'
      ? `, comparison ${comparison.replace(/-/g, ' ')}`
      : '';

    return `${entry.name}, ${registryDetail}${comparisonText}`;
  }

  entryCategory(
    _entry: FileInfo,
  ): ReturnType<PluginManagerService['resolveFileCategory']> {
    // Registry entries deliberately inherit the same neutral Directory chrome.
    // Keep the Directory template's category contract typed even though Registry
    // does not currently contribute a category color/icon override.
    return null;
  }

  entryDisplayIcon(entry: FileInfo): string {
    switch (this.registryEntryKind(entry)) {
      case 'Parent':
        return '\u{1F4C1}';
      case 'Hive':
      case 'Key':
        return '\u{1F4C1}';
      case 'Value':
        return '\u{1F511}';
    }
  }

  entryDisplayKind(entry: FileInfo): string {
    return this.registryEntryKind(entry);
  }

  entryDisplayType(entry: FileInfo): string {
    const kind = this.registryEntryKind(entry);
    if (kind === 'Value') {
      return entry.registryValueType?.trim() || 'REG_SZ';
    }
    return '';
  }

  entryDisplayValue(entry: FileInfo): string {
    if (this.registryEntryKind(entry) !== 'Value') return '';

    const type = (entry.registryValueType?.trim() || '').toUpperCase();
    const preview = entry.registryPreview?.trim() || '';

    if (type === 'REG_DWORD' || type === 'REG_DWORD_BIG_ENDIAN') {
      return this.formatRegistryInteger(preview, 32);
    }
    if (type === 'REG_QWORD') {
      return this.formatRegistryInteger(preview, 64);
    }

    if (preview) return preview;
    if (entry.size > 0) return '(binary data)';
    return '(empty)';
  }

  private formatRegistryInteger(value: string, bits: 32 | 64): string {
    const text = value.trim();
    if (!text) return '(empty)';

    const decimalMatch = /^([0-9]+)(?:\s*\(0x[0-9a-f]+\))?$/i.exec(text);
    const hexMatch = /^0x([0-9a-f]+)$/i.exec(text);

    try {
      const numeric = decimalMatch
        ? BigInt(decimalMatch[1])
        : hexMatch
          ? BigInt(`0x${hexMatch[1]}`)
          : null;

      if (numeric === null) return text;

      const mask = (1n << BigInt(bits)) - 1n;
      const normalized = numeric & mask;
      const width = bits / 4;
      return `${normalized.toString(10)} (0x${normalized
        .toString(16)
        .toUpperCase()
        .padStart(width, '0')})`;
    } catch {
      return text;
    }
  }

  private registryEntryKind(entry: FileInfo): 'Parent' | 'Hive' | 'Key' | 'Value' {
    if (entry.path === '__parent__') return 'Parent';

    const providerKind = String(entry.registryKind ?? '').trim().toLowerCase();
    if (providerKind === 'value') return 'Value';

    if (this.isRegistryHive(entry)) return 'Hive';
    if (providerKind === 'key') return 'Key';

    return entry.type === FileType.DIRECTORY ? 'Key' : 'Value';
  }

  private isRegistryHive(entry: FileInfo): boolean {
    const value = String(entry.path ?? '').trim().replace(/\\/g, '/');
    if (!/^registry:\/\//i.test(value)) return false;

    const rest = value
      .slice(value.indexOf('://') + 3)
      .replace(/^\/+/, '')
      .replace(/\/+$/, '');

    return !!rest && !rest.includes('/');
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
    ].filter(Boolean).join(' \u00B7 ');
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

  get activeTab(): Tab | null {
    if (!this.activeTabId) return null;
    return this.tabs.find(tab => tab.id === this.activeTabId) ?? null;
  }

  get activeViewerTab(): Extract<Tab, { kind: 'viewer' }> | null {
    const tab = this.activeTab;
    return tab?.kind === 'viewer' ? tab : null;
  }

  get activeSurfaceTab(): Extract<Tab, { kind: 'surface' }> | null {
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

  get activeResultsPanel() {
    return /^results:\/\//i.test(this.path)
      ? this.resultsPanels.get(this.path)
      : null;
  }

  get resultsRefreshable(): boolean {
    return !!this.activeResultsPanel &&
      this.smartResults.ensureRefreshability(this.path);
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

  get activePanelPresentation(): PanelPresentationContribution | null {
    // Registry owns its Details renderer. Generic panel presentations must not
    // replace it: doing so can leave the status count populated while the
    // Registry rows/header disappear during plugin activation or restore.
    return null;
  }

  get panelPresentationModel(): PanelPresentationModel {
    const window = this.listVirtualWindow;
    return {
      paneId: this.paneId,
      path: this.path,
      viewMode: 'details',
      entries: this.renderedEntries,
      selectedPaths: this.selectedPaths,
      cursorPath: this.cursorPath,
      visibleEntryCount: this.visibleEntryCount,
      renderedStart: this.renderedEntryStart,
      virtualEnabled: window.enabled,
      beforePx: window.beforePx,
      afterPx: window.afterPx,
      loading: this.loading,
      filterText: this.filterText,
      dragOverPath: this.dragOverPath,
      dropIntent: this.dropIntent,
      backgroundDropActive: this.backgroundDropActive,
      host: {
        scroll: event => this.onContentScroll(event),
        backgroundClick: event => this.onBackgroundClick(event),
        backgroundContextMenu: event => this.onBackgroundContextMenu(event),
        backgroundDragOver: event => this.onBackgroundDragOver(event),
        backgroundDragLeave: event => this.onBackgroundDragLeave(event),
        backgroundDrop: event => this.onBackgroundDrop(event),
        select: (entry, event) => this.selectFromPointer(entry, event),
        open: entry => void this.openEntry(entry),
        contextMenu: (entry, event) => this.onEntryContextMenu(entry, event),
        dragStart: (entry, event) => this.onEntryDragStart(entry, event),
        dragEnd: () => this.onDragEnd(),
        dragOver: (entry, event) => this.onEntryDragOver(entry, event),
        dragLeave: entry => this.onEntryDragLeave(entry),
        drop: (entry, event) => this.onEntryDrop(entry, event),
      },
    };
  }

  get pluginPanelColumns(): PanelColumnContribution[] {
    if (this.viewMode !== 'details') return [];
    return this.pluginManager.resolvePanelColumns(this.panelColumnContext);
  }

  get detailsGridTemplate(): string {
    const extra = this.pluginPanelColumns.map(column =>
      `var(--details-column-plugin-${this.detailsCssKey(column.id)}-width, ${column.width?.trim() || '120px'})`,
    );
    return [
      '28px',
      'var(--details-column-name-width, minmax(120px, 1fr))',
      'var(--details-column-kind-width, 90px)',
      'var(--details-column-type-width, 82px)',
      'var(--details-column-value-width, 150px)',
      ...extra,
    ].join(' ');
  }

  private detailsCssKey(value: string): string {
    return value.replace(/[^a-zA-Z0-9_-]/g, '-');
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

  get registryToolbarActions(): readonly PresentedCommand[] {
    const actions: PresentedCommand[] = [
      {
        id: 'registry.newFolder',
        label: 'New folder',
        shortcut: 'F7',
        enabled: !this.loading && this.canCreate,
        source: 'builtin',
        execute: () => this.newFolder(),
      },
      {
        id: 'registry.rename',
        label: 'Rename',
        shortcut: 'F2',
        enabled: !this.loading && this.canRename,
        source: 'builtin',
        execute: () => this.renameSelected(),
      },
      {
        id: 'registry.copy',
        label: 'Copy',
        shortcut: 'Ctrl+C',
        enabled: !this.loading && this.hasSelection,
        separatorBefore: true,
        source: 'builtin',
        execute: () => this.copySelected(),
      },
      {
        id: 'registry.cut',
        label: 'Cut',
        shortcut: 'Ctrl+X',
        enabled: !this.loading && this.canCut,
        source: 'builtin',
        execute: () => this.cutSelected(),
      },
      {
        id: 'registry.paste',
        label: 'Paste',
        shortcut: 'Ctrl+V',
        enabled: !this.loading && this.canPaste,
        source: 'builtin',
        execute: () => this.paste(),
      },
      {
        id: 'registry.delete',
        label: 'Delete',
        shortcut: 'Delete / F8',
        enabled: !this.loading && this.canDelete,
        separatorBefore: true,
        source: 'builtin',
        execute: () => this.deleteSelected(),
      },
      ...this.pluginToolbarActions.map((action, index) => ({
        ...action,
        separatorBefore: index === 0 ? true : action.separatorBefore,
      })),
    ];

    return actions;
  }

  readonly archiveStack: ArchiveFrame[] = [];
  private archiveExitPath = '';

  dragOverPath: string | null = null;
  private dragSpringTimer: ReturnType<typeof setTimeout> | null = null;
  private dragSpringPath: string | null = null;
  backgroundDropActive = false;
  dropIntent: 'copy' | 'move' | 'invalid' | null = null;
  dropIntentText = '';

  private anchorPath: string | null = null;
  private loadGeneration = 0;
  private readonly dirStream: DirListStreamController;
  private resultStreamAbort: AbortController | null = null;
  private streamRenderActive = false;
  private streamLoadGeneration = 0;
  private streamPreviousCursor: string | null = null;
  private streamRestoreSession = false;
  private streamFirstBatch = true;
  private streamLoadOptions: {
    preserveCursor?: boolean;
    history?: 'record' | 'none';
    syncTab?: boolean;
    restoreSession?: boolean;
  } = {};
  private readonly navigation = new NavigationController(
    path => this.directorySource.isPathInsideArchive(path)
      ? normalizeArchiveLocation(path)
      : path.trim(),
  );

  private readonly clipboardCommands: PaneClipboardController;

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
    private readonly operationHistory: OperationHistoryService,
    private readonly actionFeedback: ActionFeedbackService,
    private readonly recentFiles: RecentFilesService,
    private readonly resultsPanels: ResultsPanelService,
    private readonly smartResults: SmartResultsService,
    private readonly pluginManager: PluginManagerService,
    private readonly pluginLifecycle: PluginLifecycleCoordinatorService,
    private readonly surfaceRegistry: SurfaceRegistryService,
    private readonly cdr: ChangeDetectorRef,
  ) {
    this.dirStream = new DirListStreamController(this.api);
    this.clipboardCommands = new PaneClipboardController(
      this.fileClipboard,
      this.api,
      this.archiveService,
      this.operationHistory,
      this.actionFeedback,
      this.confirmDialog,
    );
  }

  get cursorEntry(): FileInfo | null {
    if (!this.cursorPath) return null;
    return this.entriesByPath.get(this.cursorPath) ?? null;
  }

  get treeRows(): TreeRow[] {
    const visibleEntries = this.entriesWithoutPresentationParent(this.visibleEntries);
    if (
      this.treeRowsVisibleRef === visibleEntries &&
      this.treeRowsCacheVersion === this.treeRowsStateVersion
    ) {
      return this.treeRowsCache;
    }

    const rows: TreeRow[] = [];

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

    append(visibleEntries, 0);
    this.treeRowsVisibleRef = visibleEntries;
    this.treeRowsCacheVersion = this.treeRowsStateVersion;
    this.treeRowsCache = rows;
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
    let total = 0;
    for (const path of this.selectedPaths) {
      const entry = this.entriesByPath.get(path);
      if (!entry) continue;
      if (entry.type === FileType.DIRECTORY) continue;
      const size = Number(entry.size);
      total += Number.isFinite(size) && size > 0 ? size : 0;
    }
    return total;
  }

  private get entriesByPath(): ReadonlyMap<string, FileInfo> {
    const lookupEntries = this.viewMode === 'tree'
      ? this.entriesWithoutPresentationParent(this.visibleEntries)
      : this.visibleEntries;
    if (this.entryLookupRef !== lookupEntries) {
      this.entryLookupRef = lookupEntries;
      this.entryLookup = new Map(lookupEntries.map(entry => [entry.path, entry]));
    }
    return this.entryLookup;
  }

  get visibleEntryCount(): number {
    return this.backendPresentationActive
      ? this.backendVisibleCount + (this.showPresentationParent ? 1 : 0)
      : (this.streamRenderActive ? this.visibleEntries.length : this.visibleEntries.length);
  }

  get totalEntryCount(): number {
    return this.backendPresentationActive
      ? this.backendSourceCount
      : this.entries.length;
  }

  get sortedEntries(): FileInfo[] {
    return projectSortedEntries(this.entries, {
      sortField: this.sortField,
      sortDirection: this.sortDirection,
      directoriesFirst: this.directoriesFirst,
    });
  }

  get visibleEntries(): FileInfo[] {
    if (this.backendPresentationActive) {
      return this.withPresentationParent(this.entries);
    }

    if (this.streamRenderActive) {
      const query = this.filterText.trim().toLocaleLowerCase();
      if (!query) return this.withPresentationParent(this.entries);

      // Keep filtering responsive while a stream is still growing, but defer
      // global sorting until completion. Sorting the full accumulated list on
      // every incoming batch is the main-thread failure mode we are avoiding.
      return this.withPresentationParent(this.entries.filter(entry =>
        entry.path === '__parent__' ||
        entry.name.toLocaleLowerCase().includes(query) ||
        entry.path.toLocaleLowerCase().includes(query)
      ));
    }

    if (
      this.projectionEntriesRef !== this.entries ||
      this.projectionSortField !== this.sortField ||
      this.projectionSortDirection !== this.sortDirection ||
      this.projectionDirectoriesFirst !== this.directoriesFirst ||
      this.projectionFilterText !== this.filterText
    ) {
      this.projectionEntriesRef = this.entries;
      this.projectionSortField = this.sortField;
      this.projectionSortDirection = this.sortDirection;
      this.projectionDirectoriesFirst = this.directoriesFirst;
      this.projectionFilterText = this.filterText;

      this.projectionCache = projectEntries(this.entries, {
        sortField: this.sortField,
        sortDirection: this.sortDirection,
        directoriesFirst: this.directoriesFirst,
        filterText: this.filterText,
      });
    }

    return this.withPresentationParent(this.projectionCache);
  }

  get listVirtualWindow(): VirtualWindow {
    const grid = this.viewMode === 'grid';
    const columns = grid
      ? this.estimatedGridColumns()
      : 1;

    return calculateVirtualWindow({
      itemCount: this.visibleEntryCount,
      scrollTop: this.virtualScrollTop,
      viewportHeight: this.virtualViewportHeight,
      itemExtent: grid ? this.gridRowExtent : this.listRowExtent,
      columns,
      overscanRows: grid ? 8 : 48,
      threshold: this.virtualThreshold,
    });
  }

  get renderedEntries(): FileInfo[] {
    const window = this.listVirtualWindow;
    if (this.backendPresentationActive) {
      const parentOffset = this.showPresentationParent ? 1 : 0;
      const sourceStart = Math.max(0, window.start - parentOffset);
      const sourceEnd = Math.max(sourceStart, window.end - parentOffset);
      this.queueBackendWindow(sourceStart, sourceEnd);
      const start = Math.max(0, sourceStart - this.backendWindowOffset);
      const end = Math.max(start, Math.min(
        this.entries.length,
        sourceEnd - this.backendWindowOffset,
      ));
      const rendered = this.entries.slice(start, end);
      if (parentOffset === 0 || window.start > 0) {
        return rendered;
      }
      return [this.presentationParentEntry, ...rendered];
    }
    return window.enabled
      ? this.visibleEntries.slice(window.start, window.end)
      : this.visibleEntries;
  }

  get renderedEntryStart(): number {
    if (!this.backendPresentationActive) {
      return this.listVirtualWindow.start;
    }
    return Math.max(this.listVirtualWindow.start, this.backendWindowOffset);
  }

  get treeVirtualWindow(): VirtualWindow {
    return calculateVirtualWindow({
      itemCount: this.treeRows.length,
      scrollTop: this.virtualScrollTop,
      viewportHeight: this.virtualViewportHeight,
      itemExtent: this.treeRowExtent,
      overscanRows: 12,
      threshold: this.virtualThreshold,
    });
  }

  get renderedTreeRows(): typeof this.treeRows {
    const window = this.treeVirtualWindow;
    return window.enabled
      ? this.treeRows.slice(window.start, window.end)
      : this.treeRows;
  }

  private estimatedGridColumns(): number {
    const width =
      this.listViewport?.nativeElement.clientWidth ||
      this.virtualViewportWidth ||
      800;

    // CSS grid uses minmax(108px,1fr) with a 5px gap and 7px side padding.
    return Math.max(1, Math.floor((Math.max(108, width - 14) + 5) / 113));
  }

  private updateVirtualViewport(target?: HTMLElement | null): void {
    const viewport =
      target ??
      (this.viewMode === 'tree'
        ? this.treeViewport?.nativeElement
        : this.listViewport?.nativeElement);

    if (!viewport) return;

    this.virtualScrollTop = Math.max(0, viewport.scrollTop);
    this.virtualViewportHeight = Math.max(
      this.viewMode === 'grid' ? this.gridRowExtent : this.listRowExtent,
      viewport.clientHeight || 600,
    );
    this.virtualViewportWidth = Math.max(108, viewport.clientWidth || 800);
  }

  private resetVirtualViewport(): void {
    this.virtualScrollTop = 0;
    queueMicrotask(() => {
      this.updateVirtualViewport();
      this.cdr.markForCheck();
    });
  }

  get atRoot(): boolean {
    if (this.isInArchive) return false;
    return !this.path || this.panelSources.isRootPath(
      this.path,
      this.entries,
    );
  }

  get showPresentationParent(): boolean {
    return this.isDirectoryTab && this.viewMode !== 'tree' && !this.atRoot;
  }

  private get presentationParentEntry(): FileInfo {
    return {
      path: '__parent__',
      name: '..',
      type: FileType.DIRECTORY,
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
    };
  }

  get canBack(): boolean {
    return this.navigation.canBack;
  }

  get canForward(): boolean {
    return this.navigation.canForward;
  }

  ngOnInit(): void {
    this.dirStream.bind(this.paneId, {
      onMeta: event => this.handleStreamMeta(event),
      onBatch: (event, baseEntries) => this.handleStreamBatch(event, baseEntries),
      onDone: event => this.handleStreamDone(event),
      onError: event => this.handleStreamError(event),
    });
    void this.initialize();
  }

  ngOnDestroy(): void {
    this.cancelDragSpringOpen();
    if (this.presentationReloadTimer) {
      clearTimeout(this.presentationReloadTimer);
      this.presentationReloadTimer = null;
    }
    if (this.scrollPersistTimer) {
      clearTimeout(this.scrollPersistTimer);
      this.scrollPersistTimer = null;
    }
    this.resultStreamAbort?.abort();
    this.resultStreamAbort = null;
    void this.dirStream.unbind();
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

  async refreshResultsCollection(): Promise<void> {
    const panel = this.activeResultsPanel;
    if (!panel || !this.resultsRefreshable || this.loading) return;

    const previousCursor = this.cursorPath;
    const previousSelection = new Set(this.selectedPaths);
    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      const refreshed = await this.smartResults.refresh(panel.path);
      if (!refreshed) {
        this.lastAction = 'collection refresh unavailable';
        return;
      }

      await this.load(panel.path, {
        history: 'none',
        preserveCursor: true,
        syncTab: false,
      });

      const existing = new Set(this.entries.map(entry => entry.path));
      this.selectedPaths = new Set(
        [...previousSelection].filter(path => existing.has(path)),
      );

      if (previousCursor && existing.has(previousCursor)) {
        this.cursorPath = previousCursor;
      } else {
        this.recoverVisibleCursor();
      }

      this.lastAction = `refreshed: ${refreshed.title}`;
    } catch (error) {
      this.error = error instanceof Error ? error.message : String(error);
      this.lastAction = 'collection refresh failed';
    } finally {
      this.loading = false;
      this.syncActiveDirSessionState();
      this.cdr.markForCheck();
    }
  }

  toggleResultsCollectionPin(): void {
    const panel = this.activeResultsPanel;
    if (!panel) return;

    this.resultsPanels.setPinned(panel.path, !panel.pinned);
    this.lastAction = panel.pinned ? 'collection unpinned' : 'collection pinned';
    this.cdr.markForCheck();
  }

  async renameResultsCollection(): Promise<void> {
    const panel = this.activeResultsPanel;
    if (!panel) return;

    const title = (await this.confirmDialog.prompt({
      title: 'Rename Collection',
      message: 'Collection name',
      defaultValue: panel.title,
      confirmText: 'Rename',
      historyKey: 'reassembly.results.rename',
    }))?.trim();

    if (!title || title === panel.title) return;

    this.resultsPanels.rename(panel.path, title);

    if (this.activeTabId) {
      this.workspaceService.updateTabPathAndTitle(
        this.paneId,
        this.activeTabId,
        panel.path,
        title,
      );
      this.tabs = this.mergePersistedTabs();
    }

    this.lastAction = `collection renamed: ${title}`;
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
    const path = this.path || this.fallbackDirectoryPath();
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
    if (tabId === this.activeTabId) {
      this.focusContent();
      return;
    }
    this.cancelInlineRename(false);

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
    queueMicrotask(() => this.focusContent());
  }

  private revealActiveTab(tabId = this.activeTabId): void {
    if (!tabId) return;

    queueMicrotask(() => {
      const root = this.paneRoot?.nativeElement;
      if (!root) return;

      const escaped =
        typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
          ? CSS.escape(tabId)
          : tabId.replace(/["\\]/g, '\\$&');

      root
        .querySelector<HTMLElement>(`.tab[data-tab-id="${escaped}"]`)
        ?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    });
  }

  onTabAuxClick(tab: Tab, event: MouseEvent): void {
    if (event.button !== 1) return;
    event.preventDefault();
    event.stopPropagation();

    if (!tab.isClosable || tab.isPinned) {
      this.lastAction = tab.isPinned
        ? `pinned tab kept: ${tab.title}`
        : `permanent tab kept: ${tab.title}`;
      this.cdr.markForCheck();
      return;
    }

    void this.closeTab(tab.id);
  }

  onTabStripDoubleClick(event: MouseEvent): void {
    const target = event.target as HTMLElement | null;
    if (target?.closest('.tab, .new-tab, button')) return;
    this.newTab();
  }

  onTabStripWheel(event: WheelEvent): void {
    const strip = event.currentTarget as HTMLElement | null;
    if (!strip || strip.scrollWidth <= strip.clientWidth) return;

    const delta = Math.abs(event.deltaX) > Math.abs(event.deltaY)
      ? event.deltaX
      : event.deltaY;
    if (!delta) return;

    event.preventDefault();
    strip.scrollLeft += delta;
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
    this.revealActiveTab(next.id);
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

  async reopenLastClosedTab(): Promise<boolean> {
    const restored = this.workspaceService.reopenLastClosedTab(
      this.paneId === 'right' ? 'right' : 'left',
    );

    if (!restored) {
      this.lastAction = 'no recently closed tab';
      this.cdr.markForCheck();
      return false;
    }

    if (restored.paneId !== this.paneId) {
      // WorkspaceService may fall back to the other pane if this pane has no
      // history. The dual-pane shell will route that case to the proper pane.
      return false;
    }

    await this.reloadWorkspaceTabs(restored.tab.id);
    this.lastAction = `reopen tab: ${restored.tab.title}`;
    this.revealActiveTab(restored.tab.id);
    this.focusContent();
    this.cdr.markForCheck();
    return true;
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
    const updated: Tab = { ...current, isPinned: pinned };

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

  trackTab(_index: number, tab: Tab): string {
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
    this.emitPresentationChange();
    this.scheduleBackendPresentationReload();
    this.cdr.markForCheck();
  }

  toggleSortDirection(): void {
    this.markUserInteraction();
    this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
    this.syncActiveDirSessionState();
    this.lastAction = `sort: ${this.sortDirection}`;
    this.emitPresentationChange();
    this.scheduleBackendPresentationReload();
    this.cdr.markForCheck();
  }

  toggleDirectoriesFirst(): void {
    this.markUserInteraction();
    this.directoriesFirst = !this.directoriesFirst;
    this.syncActiveDirSessionState();
    this.lastAction = this.directoriesFirst
      ? 'folders first'
      : 'mixed folders/files';
    this.emitPresentationChange();
    this.scheduleBackendPresentationReload();
    this.cdr.markForCheck();
  }

  setViewMode(_mode: DirectoryView): void {
    // Registry intentionally supports one presentation only: Directory Details.
    // Keep this method for copied command/template compatibility, but never
    // allow Registry state to diverge into list/grid/tree.
    this.resetVirtualViewport();
    this.cancelInlineRename(false);
    this.markUserInteraction();
    this.lastAction = 'view: details';
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

  trackTreeRow(_index: number, row: TreeRow): string {
    return row.entry.path;
  }

  async toggleTreeRow(
    row: TreeRow,
    event?: Event,
    userInitiated = true,
  ): Promise<void> {
    event?.preventDefault();
    event?.stopPropagation();
    if (userInitiated) this.markUserInteraction();

    if (!row.expandable) return;

    if (this.treeExpandedPaths.has(row.entry.path)) {
      this.treeExpandedPaths.delete(row.entry.path);
      this.invalidateTreeRows();
      this.syncActiveDirSessionState();
      this.lastAction = `collapse: ${row.entry.name}`;
      this.cdr.markForCheck();
      return;
    }

    this.treeExpandedPaths.add(row.entry.path);
    this.invalidateTreeRows();
    this.syncActiveDirSessionState();
    this.lastAction = `expand: ${row.entry.name}`;
    this.cdr.markForCheck();

    if (this.treeChildren.has(row.entry.path)) return;

    this.treeLoadingPaths.add(row.entry.path);
    this.invalidateTreeRows();
    this.cdr.markForCheck();

    try {
      const listing = await this.panelSources.listDirectory(
        row.entry.path,
        this.showHidden,
      );
      const children = this.sortEntries([...(listing.entries ?? [])]);
      this.treeChildren.set(row.entry.path, children);
      this.invalidateTreeRows();
    } catch (error) {
      this.treeExpandedPaths.delete(row.entry.path);
      this.invalidateTreeRows();
      this.syncActiveDirSessionState();
      this.lastAction = `tree load failed: ${
        error instanceof Error ? error.message : String(error)
      }`;
    } finally {
      this.treeLoadingPaths.delete(row.entry.path);
      this.invalidateTreeRows();
      this.cdr.markForCheck();
    }
  }

  async onTreeDoubleClick(row: TreeRow): Promise<void> {
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
      this.invalidateTreeRows();
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
    this.resetVirtualViewport();
    if (this.renamingPath) this.cancelInlineRename(false);
    this.markUserInteraction();
    this.filterText = value ?? '';
    // A backend stream owns the projection. Do not force the old, large
    // client projection synchronously for each keystroke.
    if (!this.backendPresentationActive) {
      this.reconcileSelectionWithVisibleEntries();
    } else {
      this.scheduleBackendPresentationReload();
    }
    this.lastAction = this.filterText ? `filter: ${this.filterText}` : 'filter cleared';
    this.syncActiveDirSessionState();
    this.emitPresentationChange();
    this.cdr.markForCheck();
  }

  closeFilter(): void {
    this.markUserInteraction();
    this.filterText = '';
    this.filterVisible = false;
    this.filterWidget?.deactivateContext();
    if (!this.backendPresentationActive) {
      this.reconcileSelectionWithVisibleEntries();
    } else {
      this.scheduleBackendPresentationReload();
    }
    this.lastAction = 'filter closed';
    this.syncActiveDirSessionState();
    this.emitPresentationChange();
    this.cdr.markForCheck();
  }

  nextFilterMatch(): void {
    this.moveFilterCursor(1);
  }

  previousFilterMatch(): void {
    this.moveFilterCursor(-1);
  }

  private normalizeLocationInput(path: string): string {
    let value = path.trim();
    if (!value) return value;

    // Address-bar path normalization can prepend one slash to a URI scheme,
    // e.g. `/registry:`. Remove it only when the remainder is unambiguously
    // a scheme, never for an ordinary POSIX path.
    if (/^\/[a-z][a-z0-9+.-]*:/i.test(value)) {
      value = value.slice(1);
    }

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

    if (value.toLowerCase() === `${scheme}:`) {
      return `${scheme}://`;
    }

    return value;
  }

  private async activatePluginsForLocation(path: string): Promise<void> {
    const value = this.canonicalizePluginLocation(path);
    if (!value) return;

    const scheme = this.schemeOfLocation(value);
    if (scheme === 'filesystem') return;

    const results = await this.pluginLifecycle.activateByEvent(
      `onScheme:${scheme}`,
    );

    const failed = results.find(result => !result.activated);
    if (failed) {
      throw new Error(
        failed.error ||
        `Plugin ${failed.pluginId} failed to activate for ${scheme}://.`,
      );
    }

    // A plugin scheme must be claimed after its activation event. Never allow
    // an unknown scheme to fall through to the filesystem source: on Windows
    // that turns `registry://` into an invalid native filename.
    const resolved = this.panelSources.resolve({ path: value });
    if (resolved.source.kind === 'filesystem') {
      if (results.length === 0) {
        throw new Error(
          `No enabled plugin is registered for the ${scheme}:// scheme.`,
        );
      }
      throw new Error(
        `Plugin activation completed, but no panel source registered the ${scheme}:// scheme.`,
      );
    }
  }

  navigateTo(path: string): void {
    this.cancelInlineRename(false);
    this.syncActiveDirSessionState();
    if (!this.isDirectoryTab) return;

    const location = this.canonicalizePluginLocation(path);
    this.lastAction = `navigate: ${location}`;
    // Preserve the requested scheme/location in the visible pane state while
    // plugin activation and the asynchronous load are still in flight.
    this.path = location;
    this.error = null;
    this.cdr.markForCheck();

    void (async () => {
      try {
        await this.activatePluginsForLocation(location);
        await this.load(location, { history: 'record' });
      } catch (error) {
        // Preserve the requested plugin location instead of reverting to the
        // previous filesystem directory. This also exposes the actual plugin
        // activation/registration error to the user.
        this.path = location;
        this.error = error instanceof Error
          ? error.message
          : String(error);
        this.lastAction = `navigate failed: ${location}`;
        this.syncActiveDirSessionState();
        this.cdr.markForCheck();
      }
    })();
  }

  async refresh(): Promise<void> {
    if (!this.isDirectoryTab || !this.path) return;
    this.lastAction = 'refresh';

    if (this.activeResultsPanel) {
      if (this.resultsRefreshable) {
        await this.refreshResultsCollection();
      } else {
        await this.load(this.path, {
          preserveCursor: true,
          history: 'none',
        });
      }
      return;
    }

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

  showNavigationTrail(direction: 'back' | 'forward', event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    const entries = direction === 'back' ? this.navigation.backTrail() : this.navigation.forwardTrail();
    if (entries.length) this.navigationTrailRequested.emit({ direction, event, entries });
  }

  async jumpNavigationHistory(path: string): Promise<boolean> {
    if (!this.isDirectoryTab || !path) return false;
    const index = this.navigation.indexOfTrailTarget(path);
    if (index < 0) return false;
    const loaded = await this.load(path, { history: 'none' });
    if (!loaded) return false;
    this.navigation.commitIndex(index, path);
    this.lastAction = `history jump: ${path}`;
    this.syncActiveDirSessionState();
    this.cdr.markForCheck();
    return true;
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

    if (this.registryEntryKind(entry) === 'Value') {
      void this.activateRegistryValue(entry);
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

    const selected = collapseNestedDragEntries(
      this.selectedEntries.map(candidate => ({
        path: candidate.path,
        name: candidate.name,
        type: candidate.type,
      })),
    );

    if (selected.length === 0) {
      event.preventDefault();
      return;
    }

    event.dataTransfer.effectAllowed = 'copyMove';
    event.dataTransfer.setData('application/json', JSON.stringify({
      sourcePaneId: this.paneId,
      sourceDirectory: this.path,
      entries: selected,
    }));
    event.dataTransfer.setData(
      'text/plain',
      selected.map(item => item.path).join('\n'),
    );

    this.lastAction = `drag ${selected.length}`;
    this.cdr.markForCheck();
  }

  onDragEnd(): void {
    this.cancelDragSpringOpen();
    this.clearDropFeedback();
  }

  onEntryDragOver(entry: FileInfo, event: DragEvent): void {
    if (
      entry.path === '__parent__' ||
      entry.type !== FileType.DIRECTORY ||
      !this.hasInternalDrag(event)
    ) return;

    event.preventDefault();
    event.stopPropagation();

    this.backgroundDropActive = false;
    this.dragOverPath = entry.path;
    if (this.updateDropFeedback(event, entry.path)) {
      this.scheduleDragSpringOpen(entry);
    } else {
      this.cancelDragSpringOpen();
    }
  }

  onEntryDragLeave(entry: FileInfo): void {
    if (this.dragOverPath !== entry.path) return;
    this.cancelDragSpringOpen(entry.path);
    this.dragOverPath = null;
    this.dropIntent = null;
    this.dropIntentText = '';
    this.cdr.markForCheck();
  }

  onEntryDrop(entry: FileInfo, event: DragEvent): void {
    this.cancelDragSpringOpen();
    if (
      entry.path === '__parent__' ||
      entry.type !== FileType.DIRECTORY ||
      !this.hasInternalDrag(event)
    ) return;

    event.preventDefault();
    event.stopPropagation();

    if (!this.updateDropFeedback(event, entry.path)) {
      this.clearDropFeedback();
      return;
    }

    this.clearDropFeedback(false);
    this.internalDrop.emit({
      event,
      targetPath: entry.path,
      targetEntry: entry,
    });
    this.cdr.markForCheck();
  }

  onBackgroundDragOver(event: DragEvent): void {
    if (!this.hasInternalDrag(event)) return;
    this.cancelDragSpringOpen();

    event.preventDefault();
    this.backgroundDropActive = true;
    this.dragOverPath = null;
    this.updateDropFeedback(event, this.path);
  }

  onBackgroundDragLeave(event: DragEvent): void {
    const current = event.currentTarget as HTMLElement | null;
    const related = event.relatedTarget as Node | null;
    if (current && related && current.contains(related)) return;
    this.clearDropFeedback();
  }

  onBackgroundDrop(event: DragEvent): void {
    if (!this.hasInternalDrag(event)) return;

    event.preventDefault();

    if (!this.updateDropFeedback(event, this.path)) {
      this.clearDropFeedback();
      return;
    }

    this.clearDropFeedback(false);
    this.internalDrop.emit({
      event,
      targetPath: this.path,
      targetEntry: null,
    });
    this.cdr.markForCheck();
  }

  private scheduleDragSpringOpen(entry: FileInfo): void {
    if (
      entry.type !== FileType.DIRECTORY ||
      entry.path === '__parent__' ||
      entry.path === this.dragSpringPath
    ) {
      return;
    }

    this.cancelDragSpringOpen();
    this.dragSpringPath = entry.path;
    this.dragSpringTimer = setTimeout(() => {
      const path = this.dragSpringPath;
      this.dragSpringTimer = null;
      this.dragSpringPath = null;

      if (!path || this.dragOverPath !== path) return;

      this.lastAction = `drag open: ${entry.name}`;
      void this.navigateTo(path);
    }, 700);
  }

  private cancelDragSpringOpen(path?: string): void {
    if (path && this.dragSpringPath !== path) return;

    if (this.dragSpringTimer) {
      clearTimeout(this.dragSpringTimer);
      this.dragSpringTimer = null;
    }
    this.dragSpringPath = null;
  }

  private updateDropFeedback(event: DragEvent, targetPath: string): boolean {
    const payload = this.readDragPayload(event);
    if (!payload) {
      this.dropIntent = 'invalid';
      this.dropIntentText = 'Cannot drop here';
      if (event.dataTransfer) event.dataTransfer.dropEffect = 'none';
      this.cdr.markForCheck();
      return false;
    }

    const operation = dragOperationFromModifiers(event);
    const decision = decideInternalDrop({
      entries: payload.entries,
      sourceDirectory: payload.sourceDirectory,
      targetDirectory: targetPath,
      operation,
    });

    const allowed = decision.validity === 'allowed';
    this.dropIntent = allowed ? operation : 'invalid';
    this.dropIntentText = decision.reason;

    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = allowed ? operation : 'none';
    }

    this.cdr.markForCheck();
    return allowed;
  }

  private clearDropFeedback(mark = true): void {
    this.dragOverPath = null;
    this.backgroundDropActive = false;
    this.dropIntent = null;
    this.dropIntentText = '';
    if (mark) this.cdr.markForCheck();
  }

  private readDragPayload(event: DragEvent): {
    sourcePaneId: 'left' | 'right';
    sourceDirectory: string;
    entries: Array<{ path: string; name?: string; type?: unknown }>;
  } | null {
    try {
      const raw = event.dataTransfer?.getData('application/json');
      if (!raw) return null;
      const parsed = JSON.parse(raw) as {
        sourcePaneId?: unknown;
        sourceDirectory?: unknown;
        entries?: unknown;
      };

      const entries = Array.isArray(parsed.entries)
        ? parsed.entries.filter(
            (item): item is { path: string; name?: string; type?: unknown } =>
              !!item &&
              typeof item === 'object' &&
              typeof (item as { path?: unknown }).path === 'string',
          )
        : [];

      if (entries.length === 0) return null;

      return {
        sourcePaneId: parsed.sourcePaneId === 'right' ? 'right' : 'left',
        sourceDirectory:
          typeof parsed.sourceDirectory === 'string'
            ? parsed.sourceDirectory
            : '',
        entries,
      };
    } catch {
      return null;
    }
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

    this.focusContent();
    this.entryContextMenu.emit({ event, entry });
  }

  onBackgroundClick(event: MouseEvent): void {
    if (event.defaultPrevented) return;

    const target = event.target as HTMLElement | null;
    if (target?.closest('.entry, .tree-row, button, input, select, textarea, a')) {
      return;
    }

    // Empty pane space changes pane ownership only. Selection/cursor state belongs
    // to the pane and must survive activation so keyboard navigation can continue
    // from the same item after switching between panes.
    this.markUserInteraction();
    this.lastAction = 'pane focused';
    this.activateRequested.emit();

    const current = event.currentTarget as HTMLElement | null;
    current?.focus({ preventScroll: true });

    this.cdr.markForCheck();
  }

  onBackgroundContextMenu(event: MouseEvent): void {
    if (event.defaultPrevented) return;
    event.preventDefault();

    // Background menus must not destroy a useful working selection.
    this.focusContent();
    this.backgroundContextMenu.emit(event);
  }

  onRegistryEntryClick(entry: FileInfo, event: MouseEvent): void {
    this.selectFromPointer(entry, event);

    // Modifier clicks are selection gestures, not activation gestures.
    if (event.shiftKey || event.ctrlKey || event.metaKey) return;
    if (this.registryEntryKind(entry) !== 'Value') return;

    void this.activateRegistryValue(entry);
  }

  private async activateRegistryValue(entry: FileInfo): Promise<void> {
    const source = this.activePanelSource.source as typeof this.activePanelSource.source & {
      activateEntry?: (
        context: {
          path: string;
          entries?: readonly FileInfo[];
        },
        entry: FileInfo,
      ) => boolean | Promise<boolean>;
    };

    try {
      const handled = await Promise.resolve(
        source.activateEntry?.(
          {
            path: this.path,
            entries: this.entries,
            inArchive: false,
          },
          entry,
        ) ?? false,
      );

      this.lastAction = handled
        ? `edit registry value: ${entry.name}`
        : `registry value activation unavailable: ${entry.name}`;
    } catch (error) {
      this.lastAction = `registry edit failed: ${
        error instanceof Error ? error.message : String(error)
      }`;
      throw error;
    } finally {
      this.cdr.markForCheck();
    }
  }

  selectFromPointer(entry: FileInfo, event: MouseEvent): void {
    this.markUserInteraction();

    if (entry.path === '__parent__') {
      // Parent is a navigable row, but a single click only moves keyboard
      // focus/cursor to it. Navigation is reserved for double-click / Enter.
      this.cursorPath = entry.path;
      this.selectedPaths = new Set();
      this.anchorPath = entry.path;
      this.lastAction = 'select: parent';
      this.cursorChange.emit(null);
      this.syncActiveDirSessionState();
      this.focusContent();
      this.cdr.markForCheck();
      return;
    }

    this.cursorPath = entry.path;

    if (event.shiftKey && this.anchorPath) {
      this.selectRange(this.anchorPath, entry.path);
    } else if (event.ctrlKey || event.metaKey) {
      const toggled = togglePathSelection(
        this.selectedPaths,
        entry.path,
        this.anchorPath,
      );
      this.selectedPaths = toggled.selected;
      this.anchorPath = toggled.anchor;
    } else {
      this.setSingleSelection(entry);
    }

    this.lastAction = `select: ${entry.name}`;
    this.cursorChange.emit(entry);
    this.syncActiveDirSessionState();

    // A pointer selection immediately hands keyboard ownership to the pane so
    // Arrow/Home/End/Space continue from exactly what the user just clicked.
    this.focusContent();
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
      const tab: Extract<Tab, { kind: 'viewer' }> = {
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
          ? `Modified Time \u2014 ${entries[0].name}`
          : `Modified Time \u2014 ${entries.length} items`,
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

  private get clipboardHost(): PaneClipboardHost {
    const pane = this;
    return {
      get paneId() { return pane.paneId; },
      get path() { return pane.path; },
      get selectedEntries() { return pane.selectedEntries; },
      get canCut() { return pane.canCut; },
      get canPaste() { return pane.canPaste; },
      get currentArchiveFrame() { return pane.currentArchiveFrame; },
      refresh: () => pane.refresh(),
      focusContent: () => pane.focusContent(),
    };
  }

  copySelected(): void {
    this.clipboardCommands.copy(this.clipboardHost);
  }

  cutSelected(): void {
    this.clipboardCommands.cut(this.clipboardHost);
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
      this.actionFeedback.success('Folder created', cleanName);
      await this.refresh();
      this.selectPath(destination);
      this.focusContent();
    } catch (error) {
      await this.showOperationError('Create Folder', error);
    }
  }

  async renameSelected(): Promise<void> {
    if (!this.canRename || this.renameBusy) return;

    const entry = this.selectedEntries[0];
    if (!entry || entry.path === '__parent__') return;

    this.beginInlineRename(entry);
  }

  beginInlineRename(entry: FileInfo): void {
    if (
      !this.isDirectoryTab ||
      this.isInArchive ||
      !this.sourceCapabilities.rename ||
      entry.path === '__parent__'
    ) {
      return;
    }

    this.renamingPath = entry.path;
    this.renameValue = entry.name;
    this.renameError = null;
    this.renameBusy = false;
    this.suppressRenameBlur = false;

    // Keep the edited resource as the authoritative cursor/selection.
    this.setSingleSelection(entry);
    this.syncActiveDirSessionState();
    this.cdr.detectChanges();

    requestAnimationFrame(() => {
      const input = this.renameInputForPath(entry.path);
      if (!input) return;

      input.focus({ preventScroll: true });

      const selectionEnd = this.renameSelectionEnd(entry);
      input.setSelectionRange(0, selectionEnd);
    });
  }

  onRenameInput(event: Event): void {
    const input = event.target as HTMLInputElement | null;
    this.renameValue = input?.value ?? '';
    this.renameError = null;
  }

  onRenameKeyDown(entry: FileInfo, event: KeyboardEvent): void {
    event.stopPropagation();

    if (event.key === 'Enter') {
      event.preventDefault();
      this.suppressRenameBlur = true;
      void this.commitInlineRename(entry).finally(() => {
        this.suppressRenameBlur = false;
      });
      return;
    }

    if (event.key === 'Escape') {
      event.preventDefault();
      this.suppressRenameBlur = true;
      this.cancelInlineRename(true);
      queueMicrotask(() => { this.suppressRenameBlur = false; });
      return;
    }

    if (event.key === 'Tab') {
      event.preventDefault();
      this.suppressRenameBlur = true;

      const candidates = this.visibleEntries.filter(candidate =>
        candidate.path !== '__parent__'
      );
      const currentIndex = candidates.findIndex(candidate =>
        candidate.path === entry.path
      );
      const delta = event.shiftKey ? -1 : 1;
      const next =
        currentIndex >= 0 && candidates.length > 1
          ? candidates[
              (currentIndex + delta + candidates.length) %
              candidates.length
            ]
          : null;

      void this.commitInlineRename(entry).then(() => {
        this.suppressRenameBlur = false;
        if (!this.renamingPath && next) {
          this.beginInlineRename(next);
        }
      });
      return;
    }
  }

  onRenameBlur(entry: FileInfo): void {
    if (this.suppressRenameBlur || this.renameBusy) return;
    if (this.renamingPath !== entry.path) return;

    // Commit on blur, matching Explorer/Commander expectations. Invalid names
    // keep the editor open instead of silently abandoning the operation.
    void this.commitInlineRename(entry, { fromBlur: true });
  }

  cancelInlineRename(restoreFocus = false): void {
    const path = this.renamingPath;
    this.renamingPath = null;
    this.renameValue = '';
    this.renameError = null;
    this.renameBusy = false;
    this.cdr.markForCheck();

    if (restoreFocus) {
      requestAnimationFrame(() => {
        this.focusContent();
        if (path) this.ensureCursorVisible();
      });
    }
  }

  private async commitInlineRename(
    entry: FileInfo,
    options: { fromBlur?: boolean } = {},
  ): Promise<void> {
    if (
      this.renamingPath !== entry.path ||
      this.renameBusy
    ) {
      return;
    }

    const requested = this.renameValue;
    const cleanName = this.cleanLeafName(requested);

    if (!cleanName) {
      this.renameError =
        'Invalid name. Avoid path separators, reserved characters/names, and trailing spaces or dots.';
      this.refocusRenameInput(entry.path);
      return;
    }

    if (cleanName === entry.name) {
      this.cancelInlineRename(true);
      return;
    }

    const available = await this.ensureLeafAvailableInline(
      this.path,
      cleanName,
      entry.path,
    );
    if (!available.ok) {
      this.renameError = available.message;
      this.refocusRenameInput(entry.path);
      return;
    }

    this.renameBusy = true;
    this.renameError = null;
    this.cdr.markForCheck();

    try {
      const destination = await this.performRename(entry, cleanName);
      this.cancelInlineRename(false);
      this.lastAction = `rename: ${entry.name} -> ${cleanName}`;
      this.actionFeedback.success(
        'Renamed',
        `${entry.name} \u2192 ${cleanName}`,
      );
      await this.refresh();
      this.selectPath(destination);
      this.focusContent();
    } catch (error) {
      this.renameBusy = false;
      this.renameError =
        error instanceof Error && error.message
          ? error.message
          : 'Rename failed.';
      this.cdr.markForCheck();
      this.refocusRenameInput(entry.path);

      // Blur-triggered failures must remain inline. Keyboard-triggered failures
      // also stay inline so the user can immediately correct the name.
      if (!options.fromBlur) {
        this.lastAction = 'rename failed';
      }
    }
  }

  private async performRename(
    entry: FileInfo,
    cleanName: string,
  ): Promise<string> {
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
        throw new Error(
          `Rename failed (${result.failed} item${result.failed === 1 ? '' : 's'}).`,
        );
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

    return destination;
  }

  private async ensureLeafAvailableInline(
    directory: string,
    leafName: string,
    currentPath: string,
  ): Promise<{ ok: boolean; message: string }> {
    const destination = await this.api.joinPaths([directory, leafName]);

    try {
      const existing = await this.api.getFileInfo(destination);
      if (this.samePath(existing.path, currentPath)) {
        return { ok: true, message: '' };
      }
      return {
        ok: false,
        message: `An item named "${leafName}" already exists.`,
      };
    } catch {
      return { ok: true, message: '' };
    }
  }

  private renameSelectionEnd(entry: FileInfo): number {
    if (entry.type === FileType.DIRECTORY) return entry.name.length;

    const lastDot = entry.name.lastIndexOf('.');
    return lastDot > 0 ? lastDot : entry.name.length;
  }

  private renameInputForPath(path: string): HTMLInputElement | null {
    const root = this.paneRoot?.nativeElement;
    if (!root) return null;

    const escaped =
      typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
        ? CSS.escape(path)
        : path.replace(/["\\]/g, '\\$&');

    return root.querySelector<HTMLInputElement>(
      `input[data-rename-input="${escaped}"]`,
    );
  }

  private refocusRenameInput(path: string): void {
    this.cdr.detectChanges();
    requestAnimationFrame(() => {
      const input = this.renameInputForPath(path);
      input?.focus({ preventScroll: true });
    });
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

      if (!useTrash) {
        this.operationHistory.recordOutcome({
          label: entries.length === 1
            ? `Delete ${entries[0].name} permanently`
            : `Delete ${entries.length} items permanently`,
          kind: 'delete-permanent',
          reversible: false,
          affectedPaths: sourcePaths,
          state: result.failed > 0 || result.skipped > 0 ? 'partial' : 'completed',
          summary: `${result.succeeded} succeeded \u00B7 ${result.skipped} skipped \u00B7 ${result.failed} failed`,
        });
      }

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

      this.lastAction =
        `delete: ${result.succeeded}/${result.total}` +
        (result.skipped > 0 ? ` \u00B7 ${result.skipped} skipped` : '') +
        (result.failed > 0 ? ` \u00B7 ${result.failed} failed` : '');

      if (result.failed === 0 && result.skipped === 0) {
        this.actionFeedback.success(
          useTrash ? 'Moved to Trash' : 'Deleted',
          `${result.succeeded} item${result.succeeded === 1 ? '' : 's'}`,
        );
      } else {
        this.actionFeedback.warning(
          'Delete completed with issues',
          `${result.succeeded} succeeded \u00B7 ${result.skipped} skipped \u00B7 ${result.failed} failed`,
        );
      }

      await this.refresh();
      this.focusContent();

      if (result.failed > 0 || result.skipped > 0) {
        await this.confirmDialog.alert({
          title: 'Delete Completed with Issues',
          message: [
            result.failed > 0
              ? `${result.failed} item${result.failed === 1 ? '' : 's'} failed.`
              : '',
            result.skipped > 0
              ? `${result.skipped} item${result.skipped === 1 ? '' : 's'} were skipped.`
              : '',
          ].filter(Boolean).join('\n'),
        });
      }
    } catch (error) {
      await this.showOperationError('Delete', error);
    }
  }

  async splitSelectedFile(): Promise<void> {
    if (!this.isDirectoryTab || this.isInArchive) return;

    const entry = this.cursorEntry ?? this.selectedEntries[0];
    if (!entry || entry.type === FileType.DIRECTORY) return;

    const raw = await this.confirmDialog.prompt({
      title: `Split \u2014 ${entry.name}`,
      message: 'Part size in MiB:',
      confirmText: 'Split',
      defaultValue: '100',
      historyKey: 'reassembly.splitFile.partMiB',
    });
    if (raw == null) return;

    const mib = Number(String(raw).trim());
    if (!Number.isFinite(mib) || mib <= 0) {
      await this.confirmDialog.alert({
        title: 'Invalid Part Size',
        message: 'Enter a positive part size in MiB.',
      });
      return;
    }

    try {
      const parts = await this.api.splitFile(
        entry.path,
        Math.floor(mib * 1024 * 1024),
      );

      this.operationHistory.record({
        label: `Split ${entry.name} into ${parts.length} parts`,
        kind: 'split-file',
        affectedPaths: parts,
        undo: async () => {
          const result = await this.api.batchDelete(parts, true, false);
          if (result.failed > 0) throw new Error('Undo split failed.');
        },
        redo: async () => {
          await this.api.splitFile(
            entry.path,
            Math.floor(mib * 1024 * 1024),
          );
        },
      });

      this.lastAction = `split: ${parts.length} parts`;
      await this.refresh();
      if (parts[0]) this.selectPath(parts[0]);
    } catch (error) {
      await this.showOperationError('Split File', error);
    }
  }

  async joinSelectedParts(): Promise<void> {
    if (!this.isDirectoryTab || this.isInArchive) return;

    const entries = this.selectedEntries
      .filter(entry => entry.type !== FileType.DIRECTORY)
      .slice()
      .sort((a, b) => a.name.localeCompare(b.name, undefined, {
        numeric: true,
        sensitivity: 'base',
      }));

    if (entries.length < 2) {
      await this.confirmDialog.alert({
        title: 'Join Files',
        message: 'Select at least two file parts.',
      });
      return;
    }

    const firstName = entries[0].name;
    const suggested = firstName.replace(/\.part\d+$/i, '') || 'joined.bin';
    const requested = await this.confirmDialog.prompt({
      title: 'Join File Parts',
      message: `Join ${entries.length} selected parts as:`,
      confirmText: 'Join',
      defaultValue: suggested,
      historyKey: 'reassembly.joinFiles.name',
    });
    const cleanName = await this.validateRequestedLeaf(requested, this.path);
    if (!cleanName) return;

    try {
      const destination = await this.api.joinPaths([this.path, cleanName]);
      const joined = await this.api.joinFiles(
        entries.map(entry => entry.path),
        destination,
      );

      this.operationHistory.record({
        label: `Join ${entries.length} parts as ${cleanName}`,
        kind: 'join-files',
        affectedPaths: [joined],
        undo: async () => {
          await this.api.deleteFile(joined, true, false);
        },
        redo: async () => {
          await this.api.joinFiles(
            entries.map(entry => entry.path),
            destination,
          );
        },
      });

      this.lastAction = `joined: ${cleanName}`;
      await this.refresh();
      this.selectPath(joined);
    } catch (error) {
      await this.showOperationError('Join Files', error);
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
            ? `SHA-256 \u2014 ${entries[0].name}`
            : `SHA-256 \u2014 ${entries.length} files`,
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
            ? `Size \u2014 ${entries[0].name}`
            : `Selected Size \u2014 ${entries.length} items`,
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
          title: `Attributes \u2014 ${entry.name}`,
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
          title: `Permissions \u2014 ${entry.name}`,
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
          title: `Properties \u2014 ${selected.length} items`,
          message: [
            `Items: ${selected.length}`,
            `Files: ${files}`,
            `Directories: ${directories}`,
            `Archives: ${archives}`,
            `Symbolic links: ${symlinks}`,
            `Total size: ${this.formatSize(totalSize) || '\u2014'}`,
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
          : '\u2014';
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
        title: `Properties \u2014 ${info.name}`,
        message: [
          `Name: ${info.name}`,
          `Type: ${type}`,
          `Path: ${info.path}`,
          `Size: ${this.formatSize(displaySize) || '\u2014'}`,
          `Bytes: ${displaySize.toLocaleString()}`,
          `Modified: ${modifiedText}`,
          `Created: ${createdText}`,
          `Accessed: ${accessedText}`,
          `Permissions: ${info.permissions || '\u2014'}`,
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
    await this.clipboardCommands.paste(this.clipboardHost);
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
    if (value === null) {
      return '';
    }
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
    if (entry.type === FileType.DIRECTORY) return '\u{1F4C1}';
    if (entry.type === FileType.ARCHIVE) return '\u{1F5DC}\uFE0F';
    return '\u{1F4C4}';
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
      const initialPath = drives[0]?.path || this.fallbackDirectoryPath();
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

  private defaultDirTabSession(): DirTabSession {
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

  private tabEntryCache(
    entries: readonly FileInfo[],
    limit = MAX_RUNTIME_TAB_CACHE_ENTRIES,
  ): FileInfo[] {
    if (entries.length > limit) return [];
    return entries.map(entry => ({ ...entry }));
  }

  private persistedTabEntryCache(
    entries: readonly FileInfo[],
  ): FileInfo[] {
    return this.tabEntryCache(entries, MAX_PERSISTED_TAB_CACHE_ENTRIES);
  }

  private withPresentationParent(entries: FileInfo[]): FileInfo[] {
    if (!this.showPresentationParent || entries.some(entry => entry.path === PARENT_ENTRY_PATH)) {
      return entries;
    }
    return [this.presentationParentEntry, ...entries];
  }

  private entriesWithoutPresentationParent(entries: FileInfo[]): FileInfo[] {
    return entries.filter(entry => entry.path !== '__parent__');
  }

  private loadDirTabSession(tabId: string): DirTabSession {
    const fallback = this.defaultDirTabSession();

    try {
      const storageKey = this.dirSessionStorageKey(tabId);
      const raw = localStorage.getItem(storageKey);
      if (!raw) return fallback;

      if (raw.length > MAX_LEGACY_SESSION_BYTES) {
        // Older builds persisted complete directory listings. Drop those
        // oversized snapshots instead of parsing megabytes of stale entries.
        localStorage.removeItem(storageKey);
        return fallback;
      }

      const parsed = JSON.parse(raw) as Partial<DirTabSession>;
      const version =
        typeof parsed.schemaVersion === 'number'
          ? parsed.schemaVersion
          : 1;

      // v1 was Phase 24's presentation-only shape. Missing fields naturally
      // migrate to the defaults below.
      void version;

      return {
        schemaVersion: REASSEMBLY_DIR_TAB_SESSION_VERSION,
        viewMode: 'details',
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
            ? this.persistedTabEntryCache(parsed.entries)
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
    tab: Extract<Tab, { kind: 'dir' }>,
  ): void {
    try {
      const session: DirTabSession = {
        schemaVersion: REASSEMBLY_DIR_TAB_SESSION_VERSION,
        viewMode: 'details',
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
        entries: this.persistedTabEntryCache(tab.entries),
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
  ): Extract<Tab, { kind: 'dir' }> | null {
    const active = this.activeTab;
    if (!active || active.kind !== 'dir') return null;

    const updated: Extract<Tab, { kind: 'dir' }> = {
      ...active,
      path: this.path || active.path,
      title: this.tabTitle(this.path || active.path),
      viewMode: 'details',
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
      entries: this.tabEntryCache(this.entries),
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
    sourceTab: Extract<Tab, { kind: 'dir' }>,
  ): void {
    if (this.isInArchive) return;

    const preview = this.tabs.find(
      (
        tab,
      ): tab is Extract<Tab, { kind: 'viewer' }> =>
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

    const updated: Extract<Tab, { kind: 'viewer' }> = {
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
    tab: Extract<Tab, { kind: 'dir' }>,
  ): void {
    this.sortField = tab.sortField;
    this.sortDirection = tab.sortDirection;
    this.directoriesFirst = tab.directoriesFirst;
    this.filterText = tab.filterText;
    this.filterVisible = tab.filterVisible;
    this.navigation.restore({
      entries: tab.navigationEntries,
      index: tab.navigationIndex,
    });

    // Small directories can still switch instantly. Huge directories skip the
    // runtime entry cache entirely and reload from the source; otherwise every
    // tab switch would copy thousands of FileInfo objects on the UI thread.
    this.entries = this.tabEntryCache(tab.entries);

    this.cursorPath = tab.cursorPath;
    this.selectedPaths = new Set(tab.selectedPaths);
    this.anchorPath = tab.anchorPath;

    this.treeExpandedPaths.clear();
    for (const path of tab.expandedPaths) {
      this.treeExpandedPaths.add(path);
    }
    this.treeChildren.clear();
    this.treeLoadingPaths.clear();
    this.invalidateTreeRows();

    this.reconcileRestoredSelection();
    this.cdr.markForCheck();
  }

  private async finalizeDirSessionAfterLoad(
    tab: Extract<Tab, { kind: 'dir' }>,
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
    this.invalidateTreeRows();

    const expandLevel = async (entries: FileInfo[]): Promise<void> => {
      for (const entry of entries) {
        if (
          entry.type !== FileType.DIRECTORY ||
          !desired.has(entry.path)
        ) {
          continue;
        }

        const row: TreeRow = {
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

    this.updateVirtualViewport(target);
    this.queueBackendWindowForViewport();
    this.cdr.markForCheck();

    const active = this.activeTab;
    if (!active || active.kind !== 'dir') return;

    const updated: Extract<Tab, { kind: 'dir' }> = {
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
    this.viewportChange.emit(this.viewportSnapshot());
  }

  private async tryRestoreWorkspaceTabs(): Promise<boolean> {
    for (let attempt = 0; attempt < 20; attempt += 1) {
      const persisted = this.workspaceService.getTabs(this.paneId);
      const active = this.workspaceService.getActiveTab(this.paneId);

      if (persisted.length > 0) {
        const restored = persisted
          .map(tab => this.fromPersistedWorkspaceTab(tab))
          .filter((tab): tab is Tab => !!tab);

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

  private fromPersistedDirTab(tab: any): Extract<Tab, { kind: 'dir' }> {
    const path =
      typeof tab.location === 'string'
        ? tab.location
        : tab.location?.location || tab.path || this.fallbackDirectoryPath();

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
  ): Extract<Tab, { kind: 'viewer' }> {
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

  private fromPersistedWorkspaceTab(tab: any): Tab | null {
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

  private mergePersistedTabs(): Tab[] {
    return this.workspaceService.getTabs(this.paneId)
      .map(tab => this.fromPersistedWorkspaceTab(tab))
      .filter((tab): tab is Tab => !!tab);
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
    await this.cancelActiveStreams();

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
      // Every load path (workspace restore, refresh, tab switch, direct navigation)
      // must activate the scheme owner before source/stream resolution. Previously
      // only navigateTo() did this, so a restored registry tab could briefly show
      // cached entries and then resolve registry:// through the filesystem fallback,
      // clearing the pane when that load failed.
      await this.activatePluginsForLocation(requestedPath);
      if (generation !== this.loadGeneration) return false;

      const streamMode = this.panelSources.streamMode(
        requestedPath,
        this.entries,
      );

      this.resetBackendWindowState();

      if (streamMode === 'backend') {
        this.backendPresentationActive = true;
        return await this.loadBackendStream(
          requestedPath,
          generation,
          previousCursor,
          options,
        );
      }

      if (streamMode === 'results') {
        this.backendPresentationActive = false;
        return await this.loadResultsStream(
          requestedPath,
          generation,
          previousCursor,
          options,
        );
      }

      this.backendPresentationActive = false;
      const listing = await this.panelSources.listDirectory(
        requestedPath,
        this.showHidden,
      );
      if (generation !== this.loadGeneration) return false;

      this.applyLoadedDirectory(
        listing.path || requestedPath,
        [...(listing.entries || [])],
        generation,
        previousCursor,
        options,
      );
      return true;
    } catch (error) {
      if (generation !== this.loadGeneration) return false;
      if (
        error instanceof DOMException &&
        error.name === 'AbortError'
      ) {
        return false;
      }

      if (!options.restoreSession || this.entries.length === 0) {
        this.entries = [];
        this.invalidateProjection();
        this.cursorPath = null;
        this.selectedPaths = new Set();
        this.anchorPath = null;
      }
      // Preserve the requested location in the address bar when loading fails
      // so unavailable plugin/registry roots do not fall back to "/".
      this.path = requestedPath;
      this.error = error instanceof Error ? error.message : String(error);
      return false;
    } finally {
      if (generation === this.loadGeneration) {
        this.loading = false;
        this.streamRenderActive = false;
        this.invalidateProjection();
        this.cdr.markForCheck();
      }
    }
  }

  private async loadBackendStream(
    requestedPath: string,
    generation: number,
    previousCursor: string | null,
    options: {
      preserveCursor?: boolean;
      history?: 'record' | 'none';
      syncTab?: boolean;
      restoreSession?: boolean;
    },
  ): Promise<boolean> {
    this.prepareStreamingDirectory(
      requestedPath,
      generation,
      previousCursor,
      options,
    );

    const streamId = await this.dirStream.start(
      requestedPath,
      this.showHidden,
      {
        sortField: this.sortField,
        sortDirection: this.sortDirection,
        directoriesFirst: this.directoriesFirst,
        filterText: this.filterText,
      },
    );
    this.backendSessionId = streamId;
    await this.dirStream.waitFor(streamId);
    if (generation !== this.loadGeneration) return false;

    await this.loadBackendWindowForViewport(true);
    if (generation !== this.loadGeneration) return false;

    this.applyLoadedDirectory(
      requestedPath,
      [...this.entries],
      generation,
      previousCursor,
      options,
      true,
    );

    return generation === this.loadGeneration;
  }

  private async loadResultsStream(
    requestedPath: string,
    generation: number,
    previousCursor: string | null,
    options: {
      preserveCursor?: boolean;
      history?: 'record' | 'none';
      syncTab?: boolean;
      restoreSession?: boolean;
    },
  ): Promise<boolean> {
    this.prepareStreamingDirectory(
      requestedPath,
      generation,
      previousCursor,
      options,
    );

    const abort = new AbortController();
    this.resultStreamAbort = abort;
    const accumulated: FileInfo[] = [];

    await this.panelSources.streamResultsPanel(requestedPath, {
      batchSize: 512,
      signal: abort.signal,
      onBatch: batch => {
        if (
          abort.signal.aborted ||
          generation !== this.loadGeneration
        ) return;

        accumulated.push(...batch);
        this.entries = accumulated;
        this.invalidateProjection();

        if (this.streamFirstBatch) {
          this.streamFirstBatch = false;
          this.seedStreamingCursor(previousCursor, options.restoreSession === true);
        }

        this.cdr.markForCheck();
      },
    });

    if (generation !== this.loadGeneration) return false;

    this.resultStreamAbort = null;
    this.finishStreamingDirectory(
      requestedPath,
      accumulated,
      generation,
      previousCursor,
      options,
    );

    return true;
  }

  private prepareStreamingDirectory(
    requestedPath: string,
    generation: number,
    previousCursor: string | null,
    options: {
      preserveCursor?: boolean;
      history?: 'record' | 'none';
      syncTab?: boolean;
      restoreSession?: boolean;
    },
  ): void {
    const restoreSession = options.restoreSession === true;
    this.streamLoadGeneration = generation;
    this.streamPreviousCursor = previousCursor;
    this.streamRestoreSession = restoreSession;
    this.streamLoadOptions = { ...options };
    this.streamFirstBatch = true;
    this.streamRenderActive = true;

    const pathChanged = !!this.path && this.path !== requestedPath;
    this.path = requestedPath;

    if (pathChanged && !restoreSession) {
      this.filterText = '';
      this.filterVisible = false;
      this.filterWidget?.deactivateContext();
      this.treeExpandedPaths.clear();
      this.treeChildren.clear();
      this.treeLoadingPaths.clear();
      this.invalidateTreeRows();
    }

    this.entries = [];
    this.invalidateProjection();

    if (!restoreSession) {
      this.cursorPath = null;
      this.selectedPaths = new Set();
      this.anchorPath = null;
    }

  }

  private handleStreamMeta(event: DirStreamMetaEvent): void {
    if (this.streamLoadGeneration !== this.loadGeneration) return;
    if (!this.streamRenderActive) return;

    if (event.path && event.path !== this.path) {
      this.path = event.path;
    }

    if (event.totalCount >= 0) {
      this.backendVisibleCount = event.totalCount;
      this.backendSourceCount = Math.max(
        event.totalCount,
        event.sourceCount >= 0 ? event.sourceCount : event.totalCount,
      );
    }

    if (this.backendPresentationActive) {
      this.queueBackendWindowForViewport();
      this.cdr.markForCheck();
    }
  }

  private handleStreamBatch(
    event: DirStreamBatchEvent,
    baseEntries: FileInfo[],
  ): void {
    if (this.streamLoadGeneration !== this.loadGeneration) return;
    if (!this.streamRenderActive) return;

    if (this.backendPresentationActive) {
      this.lastAction = `indexing ${event.path}: ${baseEntries.length}`;
      this.scheduleBackendProgressRefresh();
      return;
    }

    // DirListStreamController owns this stable accumulation array. We keep the
    // same reference while it grows and explicitly invalidate projections.
    this.entries = baseEntries;
    this.invalidateProjection();

    if (this.streamFirstBatch) {
      this.streamFirstBatch = false;
      this.seedStreamingCursor(
        this.streamPreviousCursor,
        this.streamRestoreSession,
      );
    }

    this.lastAction = `streaming ${event.path}: ${baseEntries.length}`;
    this.cdr.markForCheck();
  }

  private handleStreamDone(
    event: DirStreamDoneEvent,
  ): void {
    if (this.streamLoadGeneration !== this.loadGeneration) return;
    this.backendSessionId = event.streamId;
    this.backendVisibleCount = Math.max(0, event.entryCount ?? 0);
    this.backendSourceCount = Math.max(
      this.backendVisibleCount,
      event.sourceCount ?? this.backendVisibleCount,
    );
    this.streamRenderActive = false;
    if (event.path) {
      this.path = event.path;
    }
    this.cdr.markForCheck();
  }

  private handleStreamError(event: DirStreamErrorEvent): void {
    if (this.streamLoadGeneration !== this.loadGeneration) return;
    this.error = event.error || 'Failed to load directory';
    this.streamRenderActive = false;
    this.invalidateProjection();
    this.cdr.markForCheck();
  }

  private finishStreamingDirectory(
    path: string,
    entries: FileInfo[],
    generation: number,
    previousCursor: string | null,
    options: {
      preserveCursor?: boolean;
      history?: 'record' | 'none';
      syncTab?: boolean;
      restoreSession?: boolean;
    },
  ): void {
    if (generation !== this.loadGeneration) return;

    this.entries = entries;
    this.streamRenderActive = false;
    this.invalidateProjection();

    this.applyLoadedDirectory(
      path || this.path,
      entries,
      generation,
      previousCursor,
      options,
      true,
    );
  }

  private seedStreamingCursor(
    previousCursor: string | null,
    restoreSession: boolean,
  ): void {
    if (restoreSession || this.entries.length === 0) return;

    const preserved = previousCursor
      ? this.entries.find(entry => entry.path === previousCursor)
      : null;
    const next = preserved ?? this.entries[0] ?? null;

    this.cursorPath = next?.path ?? null;
    this.selectedPaths = next ? new Set([next.path]) : new Set();
    this.anchorPath = next?.path ?? null;
  }

  private applyLoadedDirectory(
    nextPath: string,
    entries: FileInfo[],
    generation: number,
    previousCursor: string | null,
    options: {
      preserveCursor?: boolean;
      history?: 'record' | 'none';
      syncTab?: boolean;
      restoreSession?: boolean;
    },
    streamed = false,
  ): void {
    if (generation !== this.loadGeneration) return;

    const pathChanged = !!this.path && this.path !== nextPath;
    this.path = nextPath;

    if (pathChanged && !options.restoreSession) {
      this.filterText = '';
      this.filterVisible = false;
      this.filterWidget?.deactivateContext();
      this.treeExpandedPaths.clear();
      this.treeChildren.clear();
      this.treeLoadingPaths.clear();
      this.invalidateTreeRows();
    }

    this.entries = entries;
    this.invalidateProjection();

    if (!options.restoreSession) {
      const preserved = previousCursor
        ? this.entries.find(entry => entry.path === previousCursor)
        : null;
      // A streamed large directory has already shown useful content. Defer the
      // expensive global ordering until visibleEntries is next requested after
      // completion; do not sort repeatedly during ingestion.
      const nextCursor = preserved ??
        (streamed ? this.entries[0] : this.visibleEntries[0]) ??
        null;

      this.cursorPath = nextCursor?.path ?? null;
      this.selectedPaths = nextCursor
        ? new Set([nextCursor.path])
        : new Set();
      this.anchorPath = nextCursor?.path ?? null;
    }

    this.error = null;
    this.commitLoadedPath(nextPath, generation, options);

    if (!options.restoreSession) {
      this.syncActiveDirSessionState();
    }
  }

  private commitLoadedPath(
    nextPath: string,
    generation: number,
    options: {
      history?: 'record' | 'none';
      syncTab?: boolean;
      restoreSession?: boolean;
    },
  ): void {
    if ((options.history ?? 'record') === 'record') {
      this.navigation.record(nextPath);
    }

    if (options.syncTab !== false) {
      this.syncActiveTab(nextPath);
    }

    if (this.activeTabId && this.activeTab?.kind === 'dir') {
      const panelTitle = this.panelSources.titleFor(nextPath);
      if (panelTitle) {
        this.workspaceService.updateTabPathAndTitle(
          this.paneId,
          this.activeTabId,
          nextPath,
          panelTitle,
        );
      } else {
        this.workspaceService.updateTabPath(
          this.paneId,
          this.activeTabId,
          nextPath,
        );
      }
    }

    this.pathHistory.addLocation(nextPath, this.paneId);
    void this.refreshDriveCapacity(nextPath).then(() => {
      if (generation === this.loadGeneration) {
        this.cdr.markForCheck();
      }
    });
  }

  private invalidateProjection(): void {
    this.projectionEntriesRef = null;
    this.entryCategoryRef = null;
    this.invalidateTreeRows();
  }

  private invalidateTreeRows(): void {
    this.treeRowsStateVersion += 1;
    this.treeRowsVisibleRef = null;
  }

  private scheduleBackendProgressRefresh(): void {
    if (this.backendProgressRefreshTimer !== null) {
      return;
    }

    this.backendProgressRefreshTimer = setTimeout(() => {
      this.backendProgressRefreshTimer = null;
      if (!this.backendPresentationActive || !this.backendSessionId) {
        return;
      }
      void this.loadBackendWindowForViewport(true);
    }, 50);
  }

  private resetBackendWindowState(): void {
    if (this.backendProgressRefreshTimer !== null) {
      clearTimeout(this.backendProgressRefreshTimer);
      this.backendProgressRefreshTimer = null;
    }
    this.backendSessionId = null;
    this.backendVisibleCount = 0;
    this.backendSourceCount = 0;
    this.backendWindowOffset = 0;
    this.backendWindowLimit = 0;
    this.backendWindowRequestToken += 1;
    this.backendWindowFetchPromise = null;
    this.backendWindowQueuedRange = null;
  }

  private queueBackendWindowForViewport(): void {
    if (!this.backendPresentationActive || !this.backendSessionId) {
      return;
    }
    const window = this.listVirtualWindow;
    this.queueBackendWindow(window.start, window.end);
  }

  private queueBackendWindow(start: number, end: number): void {
    if (!this.backendPresentationActive || !this.backendSessionId) {
      return;
    }
    this.backendWindowQueuedRange = { start, end };
    if (this.backendWindowFetchPromise) {
      return;
    }
    queueMicrotask(() => {
      if (this.backendWindowFetchPromise || !this.backendWindowQueuedRange) {
        return;
      }
      void this.loadBackendWindowForViewport();
    });
  }

  private backendWindowRangeFor(start: number, end: number): { offset: number; limit: number } {
    const visibleSpan = Math.max(1, end - start);
    const minimumLimit = this.viewMode === 'grid' ? 768 : 1024;
    const limit = Math.max(minimumLimit, visibleSpan * 4);
    const offset = Math.max(
      0,
      start - Math.floor((limit - visibleSpan) / 2),
    );
    return { offset, limit };
  }

  private async loadBackendWindowForViewport(force = false): Promise<void> {
    const streamId = this.backendSessionId;
    if (!this.backendPresentationActive || !streamId) {
      return;
    }
    const target = this.backendWindowQueuedRange ?? {
      start: this.listVirtualWindow.start,
      end: this.listVirtualWindow.end,
    };
    const range = this.backendWindowRangeFor(target.start, target.end);
    const currentEnd = this.backendWindowOffset + this.entries.length;
    if (!force && this.entries.length > 0) {
      const guard = Math.max(64, Math.floor(this.entries.length * 0.20));
      const guardedStart = this.backendWindowOffset + guard;
      const guardedEnd = currentEnd - guard;

      if (
        target.start >= guardedStart &&
        target.end <= guardedEnd
      ) {
        return;
      }
    }

    const token = ++this.backendWindowRequestToken;
    this.backendWindowFetchPromise = (async () => {
      const window = await this.dirStream.readWindow(
        streamId,
        range.offset,
        range.limit,
      );
      if (token !== this.backendWindowRequestToken) {
        return;
      }
      this.backendWindowOffset = window.offset;
      this.backendWindowLimit = window.limit;
      this.backendVisibleCount = Math.max(0, window.totalCount ?? 0);
      this.backendSourceCount = Math.max(
        this.backendVisibleCount,
        window.sourceCount ?? this.backendVisibleCount,
      );
      this.entries = [...(window.entries ?? [])];
      this.invalidateProjection();
      if (this.streamRenderActive && this.streamFirstBatch && this.entries.length > 0) {
        this.streamFirstBatch = false;
        this.seedStreamingCursor(
          this.streamPreviousCursor,
          this.streamRestoreSession,
        );
      }
      this.cdr.markForCheck();
    })();

    try {
      await this.backendWindowFetchPromise;
    } finally {
      if (token === this.backendWindowRequestToken) {
        this.backendWindowFetchPromise = null;
      }
    }
  }

  private scheduleBackendPresentationReload(): void {
    if (!this.backendPresentationActive || !this.isDirectoryTab || !this.path) {
      return;
    }
    if (this.presentationReloadTimer) {
      clearTimeout(this.presentationReloadTimer);
    }
    this.presentationReloadTimer = setTimeout(() => {
      this.presentationReloadTimer = null;
      void this.load(this.path, { preserveCursor: true, history: 'none' });
    }, 120);
  }

  private async cancelActiveStreams(): Promise<void> {
    this.resultStreamAbort?.abort();
    this.resultStreamAbort = null;
    await this.dirStream.cancelActive();
    this.resetBackendWindowState();
  }

  private createDirectoryTab(path: string): Extract<Tab, { kind: 'dir' }> {
    const normalized = path || this.fallbackDirectoryPath();
    return {
      id: `${this.paneId}-tab-${++this.tabCounter}`,
      kind: 'dir',
      path: normalized,
      title: this.tabTitle(normalized),
      viewMode: 'details',
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

  private fallbackDirectoryPath(): string {
    const candidates: string[] = [];

    const current = (this.path || '').trim();
    if (current && current !== '/') {
      candidates.push(current);
    }

    for (const tab of this.tabs) {
      if (tab.kind !== 'dir') continue;
      const candidate = (tab.path || '').trim();
      if (candidate && candidate !== '/') {
        candidates.push(candidate);
      }
    }

    for (const tab of this.workspaceService.getTabs(this.paneId)) {
      const candidate = String(tab.location?.location ?? tab.path ?? '').trim();
      if (candidate && candidate !== '/') {
        candidates.push(candidate);
      }
    }

    const exact = candidates[0];
    if (exact) {
      return exact;
    }

    const workspaceRoot = this.workspaceService.getAllTabPaths().find(path =>
      /^filesystem:\/\/[a-z](?:\/.*)?$/i.test((path || '').trim()),
    );
    if (workspaceRoot) {
      return workspaceRoot.replace(/^(filesystem:\/\/[a-z]).*$/i, '$1');
    }

    return '/';
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

    const updated: Tab = {
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
      entries:
        path === current.path
          ? this.tabEntryCache(current.entries)
          : [],
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
    this.cursorChange.emit(entry.path === '__parent__' ? null : entry);
  }

  private selectRange(anchorPath: string, targetPath: string): void {
    const visible = this.selectableEntries();
    const paths = visible.map(entry => entry.path);
    const selected = selectPathRange(paths, anchorPath, targetPath);

    if (selected) {
      this.selectedPaths = selected;
      return;
    }

    const target = visible.find(entry => entry.path === targetPath);
    if (target) this.setSingleSelection(target);
  }

  private reconcileSelectionWithVisibleEntries(): void {
    const reconciled = reconcileVisibleSelection({
      selected: this.selectedPaths,
      cursor: this.cursorPath,
      anchor: this.anchorPath,
      visiblePaths: this.selectableEntries().map(entry => entry.path),
    });

    this.selectedPaths = reconciled.selected;
    this.cursorPath = reconciled.cursor;
    this.anchorPath = reconciled.anchor;
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

    const viewport =
      this.viewMode === 'tree'
        ? this.treeViewport?.nativeElement
        : this.listViewport?.nativeElement;

    if (viewport) {
      const index =
        this.viewMode === 'tree'
          ? this.treeRows.findIndex(row => row.entry.path === this.cursorPath)
          : this.visibleEntries.findIndex(entry => entry.path === this.cursorPath);

      if (index >= 0) {
        const virtual =
          this.viewMode === 'tree'
            ? this.treeVirtualWindow
            : this.listVirtualWindow;

        if (
          virtual.enabled &&
          (index < virtual.start || index >= virtual.end)
        ) {
          if (this.viewMode === 'grid') {
            const columns = Math.max(1, virtual.columns);
            const row = Math.floor(index / columns);
            viewport.scrollTop = row * this.gridRowExtent;
          } else {
            viewport.scrollTop =
              index *
              (this.viewMode === 'tree'
                ? this.treeRowExtent
                : this.listRowExtent);
          }

          this.updateVirtualViewport(viewport);
          this.cdr.detectChanges();
        }
      }
    }

    const escaped =
      typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
        ? CSS.escape(this.cursorPath)
        : this.cursorPath.replace(/["\\]/g, '\\$&');

    queueMicrotask(() => {
      const host =
        this.viewMode === 'tree'
          ? this.treeViewport?.nativeElement
          : this.listViewport?.nativeElement;

      const cursorElement = host
        ?.querySelector<HTMLElement>(`[data-file-entry="${escaped}"]`);
      if (!host || !cursorElement) return;

      // Cursor visibility is a vertical concern. A Details/List row can be wider
      // than the viewport, so scrollIntoView must not consume the normal left/right
      // row inset by nudging the horizontal scroll position after expand/collapse.
      const scrollLeft = host.scrollLeft;
      cursorElement.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      host.scrollLeft = scrollLeft;
    });
  }
}