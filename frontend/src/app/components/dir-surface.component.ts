import { CommonModule } from '@angular/common';
import { Component, Input, ViewChild } from '@angular/core';
import { ActionBarComponent } from './action-bar.component';
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
    ActionBarComponent,
    FileViewContainerComponent,
    ArchiveMoveWarningComponent,
  ],
  template: `
    <app-address-bar
      *ngIf="vm.pathReady"
      [initialPath]="vm.currentPath"
      [loading]="vm.loading"
      [paneId]="vm.paneId"
      (pathChange)="vm.onPathChange($event)"
      (archiveSegmentClick)="vm.onArchiveSegmentClick($event)"
      (navigateUpClicked)="vm.onNavigateUp()"
      (refreshClicked)="vm.onRefresh()">
    </app-address-bar>

    <app-action-bar
      [selectedCount]="vm.selectedFiles.size || 0"
      [panePosition]="vm.paneId"
      [currentView]="vm.currentView"
      [currentSort]="vm.sortField"
      [sortDirection]="vm.sortDirection"
      (viewChange)="vm.onViewChange($event)"
      (sortChange)="vm.onSortChange($event)"
      [directoriesFirst]="vm.directoriesFirst"
      (directoriesFirstChange)="vm.onDirectoriesFirstChange($event)"
      [colorizeByMime]="vm.colorizeByMime"
      (colorizeByMimeChange)="vm.onColorizeByMimeChange($event)"
      (createFolder)="vm.onCreateFolder()"
      (deleteSelected)="vm.onDeleteSelected()"
      (copyToOther)="vm.onCopyToOther()"
      (moveToOther)="vm.onMoveToOther()">
    </app-action-bar>

    <!-- File list + find widget overlay share the same relative container -->
    <div class="list-host">
      <app-find-widget
        #findWidget
        (stateChange)="vm.onFindStateChange($event)"
        (nextMatch)="vm.onFindNext()"
        (prevMatch)="vm.onFindPrev()">
      </app-find-widget>

      <app-file-view-container
        [paneId]="vm.paneId"
        [entries]="vm.entries"
        [currentPath]="vm.currentPath"
        [showHidden]="vm.showHidden"
        [selectedFiles]="vm.selectedFiles"
        [cursorPath]="vm.cursorPath"
        [error]="vm.error"
        [currentView]="vm.currentView"
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
        (nodeDoubleClick)="vm.onNodeDoubleClick($event)"
        (dragDrop)="vm.onDragDrop($event)"
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

    app-address-bar,
    app-action-bar {
      display: block;
      flex-shrink: 0;
    }

    /* The list host is the anchor for the find widget overlay */
    .list-host {
      position: relative;
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
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
}
