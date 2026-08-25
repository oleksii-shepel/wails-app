import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output, ViewChild } from '@angular/core';
import { ResponsiveToolbarComponent } from './responsive-toolbar.component';
import { PaneArrangeControlsComponent } from './pane-arrange-controls.component';
import { AddressBarComponent } from './address-bar.component';
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
    PaneArrangeControlsComponent,
    FileViewContainerComponent,
  ],
  template: `
    <app-address-bar
      *ngIf="vm.pathReady && vm.showPathBar"
      [path]="vm.currentPath"
      [paneId]="vm.paneId"
      (navigate)="vm.onPathChange($event)">
    </app-address-bar>

    <div class="pane-toolbar-row">
      <app-pane-arrange-controls
        [currentSort]="vm.sortField"
        [sortDirection]="vm.sortDirection"
        [directoriesFirst]="vm.directoriesFirst"
        [colorizeByMime]="vm.colorizeByMime"
        [showColorizeByMime]="true"
        (sortChange)="vm.onSortChange($event)"
        (directoriesFirstChange)="vm.onDirectoriesFirstChange($event)"
        (colorizeByMimeChange)="vm.onColorizeByMimeChange($event)">
      </app-pane-arrange-controls>

      <span class="toolbar-divider" aria-hidden="true"></span>

      <app-responsive-toolbar
        [actions]="vm.toolbarActions"
        [minVisible]="0"
        ariaLabel="Pane actions"
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


    .toolbar-divider {
      flex: 0 0 1px;
      width: 1px;
      height: 20px;
      background: var(--vsc-border);
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

}
