import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output, ViewChild } from '@angular/core';

import { DirectoryPaneComponent } from './directory-pane.component';
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
import { RegistryPaneComponent } from './registry-pane.component';
import type { PresentedCommand } from './presented-command';
import type { PaneComparisonStatus } from '../services/pane-comparison.service';
import type { FileInfo } from '../shared/protocol';
import { getSourceCapabilities } from '../services/source-capabilities';
import { WorkspaceService } from '../services/workspace.service';
import type { PanePeerNavigation, PanePeerPort, PanePeerSelection, PanePeerState } from './pane-peer.port';
import type { PaneContextState, PaneMutationState, PanePinState, PaneStatePort, PaneTabState, PaneViewerState } from './pane-state.port';

@Component({
  selector: 'app-pane-host',
  standalone: true,
  imports: [CommonModule, DirectoryPaneComponent, RegistryPaneComponent],
  template: `
    <ng-container *ngIf="useRegistryPane; else directoryPane">
      <app-registry-pane
        #registry
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
      </app-registry-pane>
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
})
export class PaneHostComponent implements PanePeerPort, PaneStatePort {
  @ViewChild('component') private inner?: DirectoryPaneComponent;
  @ViewChild('registry') private registry?: RegistryPaneComponent;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() active = false;
  @Input() showHidden = false;
  @Input() currentPathPinned = false;
  @Input() pluginToolbarActions: readonly PresentedCommand[] = [];
  @Input() comparisonStatuses: ReadonlyMap<string, PaneComparisonStatus> =
    new Map<string, PaneComparisonStatus>();

  @Output() readonly activateRequested = new EventEmitter<void>();
  @Output() readonly navigationTrailRequested = new EventEmitter<{
    direction: 'back' | 'forward';
    event: MouseEvent;
    entries: readonly string[];
  }>();
  @Output() readonly remoteRecoveryRequested = new EventEmitter<void>();
  @Output() readonly togglePinRequested = new EventEmitter<void>();
  @Output() readonly locationChange = new EventEmitter<string>();
  @Output() readonly cursorChange = new EventEmitter<FileInfo | null>();
  @Output() readonly presentationChange = new EventEmitter<PanePresentation>();
  @Output() readonly viewportChange = new EventEmitter<{
    topName: string | null;
    ratio: number;
  }>();
  @Output() readonly entryContextMenu = new EventEmitter<{
    event: MouseEvent;
    entry: FileInfo;
  }>();
  @Output() readonly backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() readonly internalDrop = new EventEmitter<{
    event: DragEvent;
    targetPath: string;
    targetEntry: FileInfo | null;
  }>();

  constructor(private readonly workspaceService: WorkspaceService) {}

  get useRegistryPane(): boolean {
    const path = this.workspaceService.getActiveTab(this.paneId)?.path ?? '';
    return /^registry:\/\//i.test(path);
  }

  private get activeComponent(): PaneComponent | undefined {
    return this.registry ?? (this.inner as unknown as PaneComponent | undefined);
  }

  get contextState(): PaneContextState {
    const component = this.activeComponent;
    return {
      get path() { return component?.path ?? ''; },
      get entries() { return component?.entries ?? []; },
      get visibleEntries() { return component?.visibleEntries ?? []; },
      get activeSurfaceId() { return component?.activeSurfaceId ?? ''; },
      get viewMode() { return component?.viewMode ?? 'list'; },
      get sourceCapabilities() {
        return component?.sourceCapabilities ?? getSourceCapabilities(component?.path ?? '');
      },
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
      get sourceCapabilities() {
        return component?.sourceCapabilities ?? getSourceCapabilities(component?.path ?? '');
      },
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

  // Transitional capability ports. The host no longer mirrors the full
  // PaneComponent API; consumers choose the narrow capability they need.
  get navigation(): PaneNavigation | undefined { return this.activeComponent; }
  get selection(): PaneSelection | undefined { return this.activeComponent; }
  get commands(): PaneCommands | undefined { return this.activeComponent; }
  get presentation(): PanePresentationPort | undefined { return this.activeComponent; }
  get viewport(): PaneViewportPort | undefined { return this.activeComponent; }
  get tabCommands(): PaneTabs | undefined { return this.activeComponent; }
  get viewerCommands(): PaneViewerCommands | undefined { return this.activeComponent; }

}
