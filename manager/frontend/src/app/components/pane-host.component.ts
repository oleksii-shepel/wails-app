import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  DoCheck,
  EventEmitter,
  Input,
  Output,
  ViewChild,
} from '@angular/core';
import { RegistryPaneComponent } from '@reassembly/registry-plugin';

import { DirectoryPaneComponent } from './directory-pane.component';
import { AutorunPaneComponent } from './autorun-pane.component';
import { SystemTablePaneComponent } from './system-table-pane.component';
import { TabBarComponent } from './tab-bar.component';
import { PaneRendererRegistryService, type PaneRendererKind } from './pane-renderer-registry.service';
import { PanelSourcePaneSession } from './panel-source-pane-session';
import type {
  PaneCommands,
  PaneComponent,
  PaneNavigation,
  PanePresentation,
  PanePresentationPort,
  PaneSelection,
  PaneTabs,
  PaneViewerCommands,
  PaneViewportPort,
} from './pane-component.contract';
import type { PresentedCommand } from './presented-command';
import type { PaneComparisonStatus } from '../services/pane-comparison.service';
import type { FileInfo } from '../shared/protocol';
import { getSourceCapabilities } from '../services/source-capabilities';
import { WorkspaceService } from '../services/workspace.service';
import { PanelSourceRegistryService } from '../services/panel-source-registry.service';
import { PluginLifecycleCoordinatorService } from '../services/plugin-lifecycle-coordinator.service';
import type { PanePeerNavigation, PanePeerPort, PanePeerSelection, PanePeerState } from './pane-peer.port';
import type { PaneContextState, PaneMutationState, PanePinState, PaneStatePort, PaneTabState, PaneViewerState } from './pane-state.port';

@Component({
  selector: 'app-pane-host',
  standalone: true,
  imports: [
    CommonModule,
    DirectoryPaneComponent,
    RegistryPaneComponent,
    AutorunPaneComponent,
    SystemTablePaneComponent,
    TabBarComponent,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container *ngIf="rendererKind !== 'directory'; else directoryPane">
      <section
        class="specialized-pane"
        tabindex="-1"
        role="region"
        [class.active-pane]="active"
        [attr.data-pane-active]="active ? 'true' : null"
        (focusin)="activateRequested.emit()"
        (mousedown)="activateRequested.emit()">

        <app-tab-bar
          [paneId]="paneId"
          [tabs]="provider.tabs.slice()"
          [activeTabId]="provider.activeTab?.id ?? ''"
          [inArchive]="false"
          (tabChange)="onProviderTabChange($event)"
          (tabClose)="onProviderTabClose($event)"
          (tabPin)="onProviderTabPin($event)"
          (newTab)="provider.newTab()">
        </app-tab-bar>

        <header class="specialized-header">
          <div class="navigation">
            <button type="button" (click)="provider.goBack()" [disabled]="!provider.canGoBack()" title="Back">‹</button>
            <button type="button" (click)="provider.goForward()" [disabled]="!provider.canGoForward()" title="Forward">›</button>
            <button type="button" (click)="provider.navigateUp()" title="Parent">↑</button>
            <div class="location" [title]="provider.path">{{ provider.path }}</div>
            <button type="button" (click)="provider.refresh()" title="Refresh">↻</button>
          </div>

          <div class="actions">
            <button type="button" [class.active]="provider.viewMode === 'list'" (click)="provider.setViewMode('list')">List</button>
            <button type="button" [class.active]="provider.viewMode === 'details'" (click)="provider.setViewMode('details')">Details</button>

            <span class="separator" *ngIf="pluginToolbarActions.length"></span>

            <button
              *ngFor="let action of pluginToolbarActions"
              type="button"
              class="plugin-action"
              [disabled]="!action.enabled"
              [title]="action.shortcut ? action.label + ' (' + action.shortcut + ')' : action.label"
              (click)="action.execute()">
              <span class="plugin-icon" aria-hidden="true">{{ action.iconText || '⋯' }}</span>
              <span>{{ action.label }}</span>
            </button>
          </div>
        </header>

        <div class="error" *ngIf="provider.error">
          <strong>Location could not be loaded.</strong>
          <span>{{ provider.error }}</span>
          <button type="button" (click)="provider.refresh()">Retry</button>
        </div>

        <ng-container [ngSwitch]="rendererKind" *ngIf="!provider.error">
          <app-registry-pane
            *ngSwitchCase="'registry'"
            [paneId]="paneId"
            [path]="provider.path"
            [viewMode]="provider.viewMode"
            [entries]="provider.entries"
            [selectedPaths]="provider.selectedPaths"
            [cursorPath]="provider.cursorPath"
            [visibleEntryCount]="provider.entries.length"
            [renderedStart]="0"
            [virtualEnabled]="false"
            [beforePx]="0"
            [afterPx]="0"
            [loading]="provider.loading"
            [filterText]="''"
            [dragOverPath]="null"
            [dropIntent]="null"
            [backgroundDropActive]="false"
            (scrolled)="provider.onScroll($event)"
            (backgroundClick)="onProviderBackgroundClick($event)"
            (backgroundContextMenu)="backgroundContextMenu.emit($event)"
            (entrySelect)="onProviderSelect($event)"
            (entryOpen)="onProviderOpen($event)"
            (entryContextMenu)="onProviderContextMenu($event)">
          </app-registry-pane>

          <app-autorun-pane
            *ngSwitchCase="'autorun'"
            [session]="provider"
            (backgroundClick)="onProviderBackgroundClick($event)"
            (backgroundContextMenu)="backgroundContextMenu.emit($event)"
            (entrySelect)="onProviderSelect($event)"
            (entryOpen)="onProviderOpen($event)"
            (entryContextMenu)="onProviderContextMenu($event)">
          </app-autorun-pane>

          <app-system-table-pane
            *ngSwitchCase="'system-table'"
            [session]="provider"
            (backgroundClick)="onProviderBackgroundClick($event)"
            (backgroundContextMenu)="backgroundContextMenu.emit($event)"
            (entrySelect)="onProviderSelect($event)"
            (entryOpen)="onProviderOpen($event)"
            (entryContextMenu)="onProviderContextMenu($event)">
          </app-system-table-pane>
        </ng-container>
      </section>
    </ng-container>

    <ng-template #directoryPane>
      <app-directory-pane
        #component
        [paneId]="paneId"
        [active]="active"
        [showHidden]="showHidden"
        [currentPathPinned]="currentPathPinned"
        [pluginToolbarActions]="pluginToolbarActions"
        [comparisonStatuses]="comparisonStatuses"
        (activateRequested)="activateRequested.emit()"
        (navigationTrailRequested)="navigationTrailRequested.emit($event)"
        (remoteRecoveryRequested)="remoteRecoveryRequested.emit()"
        (togglePinRequested)="togglePinRequested.emit()"
        (locationChange)="locationChange.emit($event)"
        (cursorChange)="cursorChange.emit($event)"
        (presentationChange)="presentationChange.emit($event)"
        (viewportChange)="viewportChange.emit($event)"
        (entryContextMenu)="entryContextMenu.emit($event)"
        (backgroundContextMenu)="backgroundContextMenu.emit($event)"
        (internalDrop)="internalDrop.emit($event)">
      </app-directory-pane>
    </ng-template>
  `,
  styles: [`
    :host{height:100%;width:100%;display:flex;flex:1 1 0%;min-width:0;min-height:0;overflow:hidden}
    .specialized-pane{height:100%;width:100%;display:flex;flex:1 1 0%;min-width:0;min-height:0;flex-direction:column;overflow:hidden;background:var(--vsc-editor-background,#fff);color:var(--vsc-foreground);border:1px solid transparent}
    .specialized-pane.active-pane{border-color:color-mix(in srgb,var(--vsc-accent-blue,var(--vsc-focus-border)) 62%,transparent)}
    .specialized-header{display:grid;gap:4px;padding:5px 7px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border));background:var(--vsc-editor-background,#fff)}
    .navigation,.actions{display:flex;align-items:center;gap:4px;min-width:0}
    .location{flex:1 1 auto;min-width:0;height:26px;padding:4px 8px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;border:1px solid var(--vsc-border);border-radius:3px;background:var(--vsc-input-background,var(--vsc-editor-background));font:11px/16px var(--vsc-font-family-mono,monospace)}
    button{min-height:25px;border:1px solid var(--vsc-border,transparent);border-radius:3px;background:transparent;color:inherit;font:inherit;cursor:pointer}
    button:hover:not(:disabled){background:var(--vsc-list-hover-background)}
    button.active{background:var(--vsc-list-active-background);color:var(--vsc-list-active-foreground,var(--vsc-foreground))}
    button:disabled{opacity:.4;cursor:default}
    .plugin-action{display:inline-flex;align-items:center;gap:5px;padding:0 7px}
    .plugin-icon{display:inline-grid;place-items:center;width:16px;height:16px}
    .separator{width:1px;height:18px;margin:0 2px;background:var(--vsc-border)}
    .error{display:flex;flex:1 1 0%;flex-direction:column;align-items:center;justify-content:center;gap:6px;padding:20px;text-align:center}
    app-registry-pane,app-autorun-pane,app-system-table-pane{display:flex;flex:1 1 0%;min-width:0;min-height:0;overflow:hidden;background:var(--vsc-editor-background,#fff)}
  `],
})
export class PaneHostComponent implements DoCheck, PanePeerPort, PaneStatePort {
  @ViewChild('component') private inner?: DirectoryPaneComponent;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() active = false;
  @Input() showHidden = false;
  @Input() currentPathPinned = false;
  @Input() pluginToolbarActions: readonly PresentedCommand[] = [];
  @Input() comparisonStatuses: ReadonlyMap<string, PaneComparisonStatus> = new Map<string, PaneComparisonStatus>();

  @Output() readonly activateRequested = new EventEmitter<void>();
  @Output() readonly navigationTrailRequested = new EventEmitter<{ direction: 'back' | 'forward'; event: MouseEvent; entries: readonly string[] }>();
  @Output() readonly remoteRecoveryRequested = new EventEmitter<void>();
  @Output() readonly togglePinRequested = new EventEmitter<void>();
  @Output() readonly locationChange = new EventEmitter<string>();
  @Output() readonly cursorChange = new EventEmitter<FileInfo | null>();
  @Output() readonly presentationChange = new EventEmitter<PanePresentation>();
  @Output() readonly viewportChange = new EventEmitter<{ topName: string | null; ratio: number }>();
  @Output() readonly entryContextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() readonly backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() readonly internalDrop = new EventEmitter<{ event: DragEvent; targetPath: string; targetEntry: FileInfo | null }>();

  readonly provider: PanelSourcePaneSession;

  constructor(
    private readonly workspaceService: WorkspaceService,
    panelSources: PanelSourceRegistryService,
    pluginLifecycle: PluginLifecycleCoordinatorService,
    private readonly renderers: PaneRendererRegistryService,
    changeDetector: ChangeDetectorRef,
  ) {
    this.provider = new PanelSourcePaneSession(
      workspaceService,
      panelSources,
      pluginLifecycle,
      {
        changed: () => changeDetector.markForCheck(),
        locationChanged: path => this.locationChange.emit(path),
        cursorChanged: entry => this.cursorChange.emit(entry),
        presentationChanged: presentation => this.presentationChange.emit(presentation),
        viewportChanged: viewport => this.viewportChange.emit(viewport),
      },
    );
  }

  ngDoCheck(): void {
    if (this.rendererKind === 'directory') return;
    const path = this.workspaceService.getActiveTab(this.paneId)?.path ?? '';
    void this.provider.sync(this.paneId, path, this.showHidden);
  }

  get rendererKind(): PaneRendererKind {
    const path = this.workspaceService.getActiveTab(this.paneId)?.path ?? '';
    return this.renderers.resolve(path).kind;
  }

  private get activeComponent(): PaneComponent | undefined {
    return this.rendererKind === 'directory'
      ? (this.inner as unknown as PaneComponent | undefined)
      : this.provider;
  }

  get contextState(): PaneContextState {
    const component = this.activeComponent;
    return {
      get path() { return component?.path ?? ''; },
      get entries() { return component?.entries ?? []; },
      get visibleEntries() { return component?.visibleEntries ?? []; },
      get activeSurfaceId() { return component?.activeSurfaceId ?? ''; },
      get viewMode() { return component?.viewMode ?? 'list'; },
      get sourceCapabilities() { return component?.sourceCapabilities ?? getSourceCapabilities(component?.path ?? ''); },
      get activePanelSource() { return component?.activePanelSource; },
      get currentArchiveFrame() { return component?.currentArchiveFrame ?? null; },
      get isDirectoryTab() { return !!component?.isDirectoryTab; },
      get isInArchive() { return !!component?.isInArchive; },
    };
  }

  get tabState(): PaneTabState {
    const component = this.activeComponent;
    return {
      get tabs() { return component?.tabs ?? []; },
      get activeTab() { return component?.activeTab; },
      get activeViewerTab() { return component?.activeViewerTab; },
    };
  }

  get viewerState(): PaneViewerState {
    const component = this.activeComponent;
    return {
      get mountedViewer() { return component?.mountedViewer; },
      get viewerArchiveContext() { return component?.viewerArchiveContext ?? null; },
    };
  }

  get pinState(): PanePinState {
    const component = this.activeComponent;
    return {
      get location() { return component?.pinnableLocation ?? ''; },
      get surfaceId() { return component?.pinnableSurfaceId ?? 'dir'; },
      get title() { return component?.pinnableTitle ?? ''; },
    };
  }

  get mutationState(): PaneMutationState {
    const component = this.activeComponent;
    return {
      get hasSelection() { return !!component?.hasSelection; },
      get canCreate() { return !!component?.canCreate; },
      get canRename() { return !!component?.canRename; },
      get canDelete() { return !!component?.canDelete; },
      get canCut() { return !!component?.canCut; },
      get canPaste() { return !!component?.canPaste; },
    };
  }

  get peerState(): PanePeerState {
    const component = this.activeComponent;
    return {
      get path() { return component?.path ?? ''; },
      get entries() { return component?.entries ?? []; },
      get cursorEntry() { return component?.cursorEntry ?? null; },
      get selectedPaths() { return component?.selectedPaths ?? new Set<string>(); },
      get sourceCapabilities() { return component?.sourceCapabilities ?? getSourceCapabilities(component?.path ?? ''); },
      get isDirectoryTab() { return !!component?.isDirectoryTab; },
      get isInArchive() { return !!component?.isInArchive; },
    };
  }

  get peerNavigation(): PanePeerNavigation {
    const component = this.activeComponent;
    return {
      refresh: () => component?.refresh() ?? Promise.resolve(),
      canNavigateToLocation: path => component?.canNavigateToLocation(path) ?? false,
      navigateLinkedLocation: (path, options) =>
        component?.navigateLinkedLocation(path, options).then(Boolean) ?? Promise.resolve(false),
    };
  }

  get peerSelection(): PanePeerSelection {
    const component = this.activeComponent;
    return {
      selectPeerByName: name => component?.selectPeerByName(name) ?? false,
      selectPaths: paths => component?.selectPaths(paths),
    };
  }

  get navigation(): PaneNavigation | undefined { return this.activeComponent; }
  get selection(): PaneSelection { return this.activeComponent!; }
  get commands(): PaneCommands | undefined { return this.activeComponent; }
  get presentation(): PanePresentationPort | undefined { return this.activeComponent; }
  get viewport(): PaneViewportPort | undefined { return this.activeComponent; }
  get tabCommands(): PaneTabs | undefined { return this.activeComponent; }
  get viewerCommands(): PaneViewerCommands | undefined { return this.activeComponent; }

  onProviderBackgroundClick(event: MouseEvent): void {
    if (event.target === event.currentTarget) this.provider.clearSelection();
  }

  onProviderSelect(event: any): void {
    this.provider.selectFromPointer(event.entry as FileInfo, event.event as MouseEvent);
  }

  onProviderOpen(entry: any): void { void this.provider.openEntry(entry as FileInfo); }

  onProviderContextMenu(event: any): void {
    this.entryContextMenu.emit({ entry: event.entry as FileInfo, event: event.event as MouseEvent });
  }

  onProviderTabChange(tab: any): void { this.workspaceService.switchTab(this.paneId, tab.id); }
  onProviderTabClose(tab: any): void { this.workspaceService.closeTab(this.paneId, tab.id); }
  onProviderTabPin(tab: any): void { this.workspaceService.toggleTabPin(this.paneId, tab.id); }
}
