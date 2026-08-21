import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output, ViewChild } from '@angular/core';
import { ResponsiveToolbarComponent, type ResponsiveToolbarCommand } from './responsive-toolbar.component';
import { AddressBarComponent } from './address-bar.component';
import { ArchiveMoveWarningComponent } from './archive-move-warning.component';
import { FileViewContainerComponent } from './file-view-container.component';
import { FindWidgetComponent } from './filter-widget.component';
import { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';

@Component({
  selector: 'app-dir-surface',
  standalone: true,
  imports: [
    CommonModule,
    AddressBarComponent,
    FindWidgetComponent,
    ResponsiveToolbarComponent,
    FileViewContainerComponent,
    ArchiveMoveWarningComponent,
  ],
  template: `
    <app-address-bar
      *ngIf="vm.pathReady && vm.showPathBar"
      [path]="vm.currentPath"
      [paneId]="vm.paneId"
      (navigate)="vm.onPathChange($event)">
    </app-address-bar>

    <div class="pane-toolbar-row">
      <app-responsive-toolbar
        [actions]="responsiveToolbarActions"
        [minVisible]="0"
        ariaLabel="Pane toolbar"
        overflowLabel="More pane actions">
      </app-responsive-toolbar>
    </div>

    <!-- File list + find widget overlay share the same relative container -->
    <div class="list-host">
      <app-find-widget
        #findWidget
        (stateChange)="vm.onFindStateChange($event)"
        (nextMatch)="vm.onFindNext()"
        (prevMatch)="vm.onFindPrev()">
      </app-find-widget>

      <app-file-view-container
        [surfaceId]="surfaceId"
        [surfaceActive]="surfaceActive"
        [browserPath]="surfacePath"
        [paneId]="vm.paneId"
        [persistedBrowserPaneSplit]="vm.browserPaneSplit"
        [entries]="vm.entries"
        [currentPath]="vm.currentPath"
        [showHidden]="vm.showHidden"
        [selectedFiles]="vm.selectedFiles"
        [cursorPath]="vm.cursorPath"
        [error]="vm.error"
        [currentView]="vm.currentView"
        [singleClickToOpen]="vm.singleClickToOpen"
        [sortField]="vm.sortField"
        [sortDirection]="vm.sortDirection"
        [directoriesFirst]="vm.directoriesFirst"
        [colorizeByMime]="vm.colorizeByMime"
        [paneActive]="vm.paneActive"
        (viewChange)="vm.onViewChange($event)"
        (selectionChange)="vm.onSelectionChange($event)"
        (navigate)="vm.onNavigate($event)"
        (openInNewTab)="vm.onOpenInNewTab($event)"
        (sortChange)="vm.onSortChange($event)"
        (contextMenu)="vm.onContextMenu($event)"
        (backgroundContextMenu)="vm.onBackgroundContextMenu($event)"
        (nodeDoubleClick)="vm.onNodeDoubleClick($event)"
        (dragDrop)="vm.onDragDrop($event)"
        (browserPaneSplitChange)="vm.onBrowserPaneSplitChange($event)"
        (viewportChange)="viewportChange.emit()"
        (rename)="vm.onRename($event)">
      </app-file-view-container>
    </div>

    <!-- Archive move warning dialog -->
    <app-archive-move-warning
      [visible]="vm.archiveMoveWarning"
      [entryCount]="vm.archiveMoveEntryCount"
      (confirmed)="vm.onArchiveMoveConfirmed()"
      (cancelled)="vm.onArchiveMoveCancelled()">
    </app-archive-move-warning>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    app-address-bar {
      display: block;
      flex-shrink: 0;
      position: relative;
      z-index: 3;
    }

    .pane-toolbar-row {
      position: relative;
      z-index: 2;
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      height: var(--vsc-titlebar-height);
      min-width: 0;
      padding: 0 var(--vsc-padding-lg);
      border-bottom: 1px solid var(--vsc-border);
      background: var(--vsc-sidebar-background);
      flex-shrink: 0;
      overflow: hidden;
    }


    :host-context(.compact-mode) .pane-toolbar-row {
      height: 28px;
    }

    /* The list host is the anchor for the find widget overlay */
    .list-host {
      position: relative;
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      z-index: 1;
    }

    app-find-widget {
      /* find-widget positions itself absolute inside .list-host */
    }

    app-file-view-container {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
    }
  `]
})
export class DirSurfaceComponent {
  @ViewChild(AddressBarComponent) addressBar?: AddressBarComponent;
  @ViewChild(FileViewContainerComponent) fileViewContainer?: FileViewContainerComponent;
  @ViewChild('findWidget') findWidget?: FindWidgetComponent;

  @Input({ required: true }) vm!: DirSurfaceVm;
  @Input() surfaceId = '';
  /** Semantic surface type persisted by Quick Access pins. */
  @Input() pinSurfaceId = 'dir';
  @Input() surfaceActive = true;
  @Input() surfacePath = '';
  @Output() viewportChange = new EventEmitter<void>();
  get responsiveToolbarActions(): readonly ResponsiveToolbarCommand[] {
    const viewActions: ResponsiveToolbarCommand[] = [
      {
        id: 'surface.view.list',
        label: 'List',
        shortcut: 'Ctrl+1',
        enabled: true,
        active: this.vm.currentView === 'list',
        source: 'builtin',
        execute: () => this.vm.onViewChange('list'),
      },
      {
        id: 'surface.view.grid',
        label: 'Grid',
        shortcut: 'Ctrl+2',
        enabled: true,
        active: this.vm.currentView === 'grid',
        source: 'builtin',
        execute: () => this.vm.onViewChange('grid'),
      },
      {
        id: 'surface.view.tree',
        label: 'Tree',
        shortcut: 'Ctrl+3',
        enabled: true,
        active: this.vm.currentView === 'tree',
        source: 'builtin',
        execute: () => this.vm.onViewChange('tree'),
      },
      {
        id: 'surface.view.details',
        label: 'Details',
        shortcut: 'Ctrl+4',
        enabled: true,
        active: this.vm.currentView === 'details',
        source: 'builtin',
        execute: () => this.vm.onViewChange('details'),
      },
    ];

    const arrangeActions: ResponsiveToolbarCommand[] = [
      {
        id: 'surface.sort.name',
        label: 'Sort: Name',
        enabled: true,
        active: this.vm.sortField === 'name',
        separatorBefore: true,
        source: 'builtin',
        execute: () => this.vm.onSortChange({ field: 'name', direction: this.vm.sortDirection }),
      },
      {
        id: 'surface.sort.size',
        label: 'Sort: Size',
        enabled: true,
        active: this.vm.sortField === 'size',
        source: 'builtin',
        execute: () => this.vm.onSortChange({ field: 'size', direction: this.vm.sortDirection }),
      },
      {
        id: 'surface.sort.modified',
        label: 'Sort: Modified',
        enabled: true,
        active: this.vm.sortField === 'modified',
        source: 'builtin',
        execute: () => this.vm.onSortChange({ field: 'modified', direction: this.vm.sortDirection }),
      },
      {
        id: 'surface.sort.type',
        label: 'Sort: Type',
        enabled: true,
        active: this.vm.sortField === 'type',
        source: 'builtin',
        execute: () => this.vm.onSortChange({ field: 'type', direction: this.vm.sortDirection }),
      },
      {
        id: 'surface.sort.direction',
        label: this.vm.sortDirection === 'asc' ? 'Ascending' : 'Descending',
        enabled: true,
        active: true,
        source: 'builtin',
        execute: () => this.vm.onSortChange({
          field: this.vm.sortField,
          direction: this.vm.sortDirection === 'asc' ? 'desc' : 'asc',
        }),
      },
      {
        id: 'surface.folders-first',
        label: 'Folders first',
        enabled: true,
        active: this.vm.directoriesFirst,
        source: 'builtin',
        execute: () => this.vm.onDirectoriesFirstChange(!this.vm.directoriesFirst),
      },
      {
        id: 'surface.colorize',
        label: 'Colorize by Category',
        enabled: true,
        active: this.vm.colorizeByMime,
        source: 'builtin',
        execute: () => this.vm.onColorizeByMimeChange(!this.vm.colorizeByMime),
      },
    ];

    return [
      ...viewActions,
      ...this.vm.toolbarActions,
      ...arrangeActions,
    ];
  }

}