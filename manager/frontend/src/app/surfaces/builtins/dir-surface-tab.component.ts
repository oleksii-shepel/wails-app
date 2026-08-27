import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, EventEmitter, Input, ViewChild } from '@angular/core';
import { AddressBarComponent } from '../../components/address-bar.component';
import { FindWidgetComponent } from '../../components/filter-widget.component';
import { PaneArrangeControlsComponent } from '../../components/pane-arrange-controls.component';
import { PaneViewHostComponent } from '../../components/pane-view-host.component';
import { ResponsiveToolbarComponent } from '../../components/responsive-toolbar.component';
import { DirSurfaceContext } from '../surface-context';

@Component({
  selector: 'app-dir-surface-tab',
  standalone: true,
  imports: [
    CommonModule,
    AddressBarComponent,
    FindWidgetComponent,
    PaneArrangeControlsComponent,
    PaneViewHostComponent,
    ResponsiveToolbarComponent,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-address-bar
      *ngIf="ctx.dirVm.pathReady && ctx.dirVm.showPathBar"
      [path]="ctx.dirVm.currentPath"
      [paneId]="ctx.dirVm.paneId"
      (navigate)="ctx.dirVm.onPathChange($event)">
    </app-address-bar>

    <div class="pane-toolbar-row">
      <app-pane-arrange-controls
        [currentSort]="ctx.dirVm.sortField"
        [sortDirection]="ctx.dirVm.sortDirection"
        [directoriesFirst]="ctx.dirVm.directoriesFirst"
        [colorizeByMime]="ctx.dirVm.colorizeByMime"
        [showColorizeByMime]="true"
        (sortChange)="ctx.dirVm.onSortChange($event)"
        (directoriesFirstChange)="ctx.dirVm.onDirectoriesFirstChange($event)"
        (colorizeByMimeChange)="ctx.dirVm.onColorizeByMimeChange($event)">
      </app-pane-arrange-controls>

      <span class="toolbar-divider" aria-hidden="true"></span>

      <app-responsive-toolbar
        [actions]="ctx.dirVm.toolbarActions"
        [minVisible]="0"
        ariaLabel="Pane actions"
        overflowLabel="More pane actions">
      </app-responsive-toolbar>
    </div>

    <div class="list-host">
      <app-find-widget
        #findWidget
        (stateChange)="ctx.dirVm.onFindStateChange($event)"
        (nextMatch)="ctx.dirVm.onFindNext()"
        (prevMatch)="ctx.dirVm.onFindPrev()">
      </app-find-widget>

      <app-pane-view-host
        #paneViewHost
        [surfaceId]="ctx.paneId + ':' + ctx.tab.id"
        [surfaceActive]="active"
        [browserPath]="currentSurfacePath"
        [paneId]="ctx.dirVm.paneId"
        [persistedBrowserPaneSplit]="ctx.dirVm.browserPaneSplit"
        [entries]="ctx.dirVm.entries"
        [currentPath]="ctx.dirVm.currentPath"
        [showHidden]="ctx.dirVm.showHidden"
        [selectedFiles]="ctx.dirVm.selectedFiles"
        [cursorPath]="ctx.dirVm.cursorPath"
        [loading]="ctx.dirVm.loading"
        [error]="ctx.dirVm.error"
        [currentView]="ctx.dirVm.currentView"
        [singleClickToOpen]="ctx.dirVm.singleClickToOpen"
        [sortField]="ctx.dirVm.sortField"
        [sortDirection]="ctx.dirVm.sortDirection"
        [directoriesFirst]="ctx.dirVm.directoriesFirst"
        [colorizeByMime]="ctx.dirVm.colorizeByMime"
        [paneActive]="ctx.dirVm.paneActive"
        (viewChange)="ctx.dirVm.onViewChange($event)"
        (selectionChange)="ctx.dirVm.onSelectionChange($event)"
        (navigate)="ctx.dirVm.onNavigate($event)"
        (openInNewTab)="ctx.dirVm.onOpenInNewTab($event)"
        (sortChange)="ctx.dirVm.onSortChange($event)"
        (contextMenu)="ctx.dirVm.onContextMenu($event)"
        (backgroundContextMenu)="ctx.dirVm.onBackgroundContextMenu($event)"
        (nodeDoubleClick)="ctx.dirVm.onNodeDoubleClick($event)"
        (dragDrop)="ctx.dirVm.onDragDrop($event)"
        (browserPaneSplitChange)="ctx.dirVm.onBrowserPaneSplitChange($event)"
        (viewportChange)="viewportChange.emit()"
        (rename)="ctx.dirVm.onRename($event)">
      </app-pane-view-host>
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

    .list-host {
      position: relative;
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    app-pane-view-host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
  `],
})
export class DirSurfaceTabComponent {
  @Input({ required: true }) ctx!: DirSurfaceContext;
  @Input() active = false;
  readonly viewportChange = new EventEmitter<void>();
  @ViewChild(AddressBarComponent) addressBar?: AddressBarComponent;
  @ViewChild('paneViewHost') paneViewHost?: PaneViewHostComponent;
  @ViewChild('findWidget') findWidget?: FindWidgetComponent;

  get currentSurfacePath(): string {
    if (this.active) {
      return this.ctx?.dirVm?.currentPath || this.ctx?.tab?.path || '';
    }

    return this.ctx?.tab?.path || this.ctx?.dirVm?.currentPath || '';
  }

  // Exposed for SurfaceOutletComponent proxying.
  get fileViewContainer() { return this.paneViewHost; }
}
