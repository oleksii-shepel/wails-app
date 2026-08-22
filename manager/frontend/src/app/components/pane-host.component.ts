import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  DoCheck,
  ElementRef,
  EventEmitter,
  Input,
  Output,
  ViewChild,
} from '@angular/core';
import { DirectoryPaneComponent } from './directory-pane.component';
import { RegistryPaneComponent } from './registry-pane.component';
import { AutorunPaneComponent } from './autorun-pane.component';
import { SystemTablePaneComponent } from './system-table-pane.component';
import { NetworkPaneComponent } from './network-pane.component';
import { EnvironmentPaneComponent } from './environment-pane.component';
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
import { adaptPluginToolbarActions } from './pane-toolbar-action';
import type { ToolbarContext } from '../plugin/plugin-registry-api';
import { PluginManagerService } from '../services/plugin-manager.service';
import { InputEngineService } from '../services/input-engine.service';
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
    NetworkPaneComponent,
    EnvironmentPaneComponent,
    TabBarComponent,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container *ngIf="rendererKind !== 'directory' && rendererKind !== 'registry' && rendererKind !== 'autorun' && rendererKind !== 'network' && rendererKind !== 'environment'; else standardPane">
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

        <header class="navigation-bar">
          <div class="nav-cluster">
            <button
              type="button"
              class="nav-button"
              [disabled]="provider.loading || !provider.canGoBack()"
              (click)="provider.goBack()"
              aria-label="Back"
              title="Back (Alt+Left)">
              &#x2190;
            </button>

            <button
              type="button"
              class="nav-button"
              [disabled]="provider.loading || !provider.canGoForward()"
              (click)="provider.goForward()"
              aria-label="Forward"
              title="Forward (Alt+Right)">
              &#x2192;
            </button>

            <button
              type="button"
              class="nav-button"
              [disabled]="provider.loading"
              (click)="provider.navigateUp()"
              aria-label="Parent directory"
              title="Parent directory (Backspace)">
              &#x2191;
            </button>
          </div>

          <div class="address-shell">
            <div
              *ngIf="!specializedLocationEditing; else specializedLocationEditor"
              class="display-path breadcrumb-path"
              title="Click to edit location"
              (click)="beginSpecializedLocationEdit()">
              <button
                type="button"
                class="location-icon breadcrumb-home"
                title="Edit location (Ctrl+L)"
                (click)="beginSpecializedLocationEdit(); $event.stopPropagation()"
                aria-label="Edit location">
                &#x2302;
              </button>

              <div class="breadcrumbs">
                <button
                  type="button"
                  class="breadcrumb current"
                  [title]="provider.path || ''"
                  (click)="beginSpecializedLocationEdit(); $event.stopPropagation()">
                  {{ provider.path || '' }}
                </button>
              </div>

              <button
                type="button"
                class="edit-path"
                title="Edit location (Ctrl+L)"
                aria-label="Edit location"
                (click)="beginSpecializedLocationEdit(); $event.stopPropagation()">
                &#x270E;
              </button>
            </div>
          </div>

          <button
            type="button"
            class="nav-button pin"
            [class.pinned]="currentPathPinned"
            [disabled]="!provider.path"
            (click)="togglePinRequested.emit()"
            [attr.aria-label]="currentPathPinned ? 'Unpin current location' : 'Pin current location'"
            [title]="currentPathPinned ? 'Unpin current location' : 'Pin current location'">
            {{ currentPathPinned ? '\u2605' : '\u2606' }}
          </button>

          <button
            type="button"
            class="nav-button refresh"
            [disabled]="provider.loading"
            (click)="provider.refresh()"
            aria-label="Refresh"
            title="Refresh (Ctrl+R)">
            &#x21BB;
          </button>
        </header>

        <ng-template #specializedLocationEditor>
          <div class="address-shell">
            <div class="editor">
              <span class="location-icon" aria-hidden="true">&#x2302;</span>
              <input
                #locationEditor
                class="location-input"
                [value]="specializedLocationDraft"
                (input)="specializedLocationDraft = $any($event.target).value"
                (keydown.enter)="commitSpecializedLocationEdit()"
                (keydown.escape)="cancelSpecializedLocationEdit()"
                (blur)="cancelSpecializedLocationEdit()"
                autocomplete="off"
                spellcheck="false"
                aria-label="Location" />

              <button
                type="button"
                class="cancel"
                title="Cancel (Escape)"
                (mousedown)="$event.preventDefault()"
                (click)="cancelSpecializedLocationEdit()">
                &#x00D7;
              </button>
            </div>
          </div>
        </ng-template>

        <div
          class="status-row"
          role="status"
          aria-live="polite"
          aria-atomic="true">
          <span *ngIf="provider.loading">Loading {{ provider.path || 'location' }}&#x2026;</span>
          <span *ngIf="!provider.loading && !provider.error">{{ provider.entries.length }} items</span>
          <span *ngIf="provider.selectedPaths.size">{{ provider.selectedPaths.size }} selected</span>
          <span *ngIf="showHidden" class="hidden-state">hidden visible</span>
          <span class="hint">Ctrl+L location</span>
        </div>

        <div class="action-bar">
          <div class="view-switcher" role="group" aria-label="Directory view">
            <button type="button" [class.active]="provider.viewMode === 'list'" [attr.aria-pressed]="provider.viewMode === 'list'" (click)="provider.setViewMode('list')">List</button>
            <button type="button" [class.active]="provider.viewMode === 'details'" [attr.aria-pressed]="provider.viewMode === 'details'" (click)="provider.setViewMode('details')">Details</button>
          </div>

          <span class="action-separator"></span>
          <button type="button" [disabled]="provider.loading || !provider.canCreate" (click)="provider.newFolder()" title="New folder (F7)">New folder</button>
          <button type="button" [disabled]="provider.loading || !provider.canRename" (click)="provider.renameSelected()" title="Rename (F2)">Rename</button>
          <span class="action-separator"></span>
          <button type="button" [disabled]="provider.loading || !provider.hasSelection" (click)="provider.copySelected()" title="Copy (Ctrl+C)">Copy</button>
          <button type="button" [disabled]="provider.loading || !provider.canCut" (click)="provider.cutSelected()" title="Cut (Ctrl+X)">Cut</button>
          <button type="button" [disabled]="provider.loading || !provider.canPaste" (click)="provider.paste()" title="Paste (Ctrl+V)">Paste</button>
          <span class="action-separator"></span>
          <button type="button" class="danger" [disabled]="provider.loading || !provider.canDelete" (click)="provider.deleteSelected()" title="Delete (Delete / F8)">Delete</button>

          <ng-container *ngIf="effectivePluginToolbarActions.length">
            <span class="action-separator"></span>
            <button
              *ngFor="let action of effectivePluginToolbarActions"
              type="button"
              class="plugin-action"
              [disabled]="provider.loading || !action.enabled"
              [title]="action.shortcut ? action.label + ' (' + action.shortcut + ')' : action.label"
              (click)="action.execute()">
              <span class="plugin-icon" aria-hidden="true">{{ action.iconText || '\u25C6' }}</span>
              <span>{{ action.label }}</span>
            </button>
          </ng-container>
        </div>

        <div class="error" *ngIf="provider.error">
          <strong>Location could not be loaded.</strong>
          <span>{{ provider.error }}</span>
          <button type="button" (click)="provider.refresh()">Retry</button>
        </div>

        <ng-container [ngSwitch]="rendererKind" *ngIf="!provider.error">
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

    <ng-template #standardPane>
      <ng-container [ngSwitch]="rendererKind">
      <app-directory-pane
        *ngSwitchCase="'directory'"
        #component
        [paneId]="paneId"
        [active]="active"
        [showHidden]="showHidden"
        [currentPathPinned]="currentPathPinned"
        [pluginToolbarActions]="effectivePluginToolbarActions"
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

      <app-registry-pane
        *ngSwitchCase="'registry'"
        #component
        [paneId]="paneId"
        [active]="active"
        [showHidden]="showHidden"
        [currentPathPinned]="currentPathPinned"
        [pluginToolbarActions]="effectivePluginToolbarActions"
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

      <app-autorun-pane
        *ngSwitchCase="'autorun'"
        #component
        [paneId]="paneId"
        [active]="active"
        [showHidden]="showHidden"
        [currentPathPinned]="currentPathPinned"
        [pluginToolbarActions]="effectivePluginToolbarActions"
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
      </app-autorun-pane>

      <app-network-pane
        *ngSwitchCase="'network'"
        #component
        [paneId]="paneId"
        [active]="active"
        [showHidden]="showHidden"
        [currentPathPinned]="currentPathPinned"
        [pluginToolbarActions]="effectivePluginToolbarActions"
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
      </app-network-pane>

      <app-environment-pane
        *ngSwitchCase="'environment'"
        #component
        [paneId]="paneId"
        [active]="active"
        [showHidden]="showHidden"
        [currentPathPinned]="currentPathPinned"
        [pluginToolbarActions]="effectivePluginToolbarActions"
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
      </app-environment-pane>
      </ng-container>
    </ng-template>
  `,
  styles: [`
    :host {--pane-column-gutter:12px;height:100%;width:100%;display:flex;flex:1 1 0%;min-width:0;min-height:0;overflow:hidden
      --pane-column-gap: 6px;
}
    .specialized-pane{height:100%;width:100%;display:flex;flex:1 1 0%;min-width:0;min-height:0;flex-direction:column;overflow:hidden;background:var(--vsc-editor-background,#fff);color:var(--vsc-foreground);border:1px solid transparent}
    .specialized-pane.active-pane{border-color:color-mix(in srgb,var(--vsc-accent-blue,var(--vsc-focus-border)) 62%,transparent)}
    .navigation-bar {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 6px 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-editor-background, #fff);
    }
    .nav-cluster {
      display: flex;
      gap: 4px;
      flex-shrink: 0;
    }
    .address-shell {
      position: relative;
      display: block;
      flex: 1 1 auto;
      min-width: 0;
    }
    .display-path,
    .editor {
      box-sizing: border-box;
      display: grid;
      grid-template-columns: 24px minmax(0, 1fr) 24px;
      align-items: center;
      width: 100%;
      height: 30px;
      padding: 0 7px;
      border: 1px solid var(--vsc-border, var(--vsc-control-background));
      border-radius: 4px;
      background: var(--vsc-input-background, var(--vsc-popover-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }
    .display-path {
      cursor: text;
      text-align: left;
      font: inherit;
    }
    .display-path:hover {
      border-color: color-mix(
        in srgb,
        var(--vsc-border, var(--vsc-control-background)) 65%,
        var(--vsc-foreground, var(--vsc-foreground))
      );
    }
    .editor {
      border-color: var(--vsc-focus-border, var(--vsc-accent-blue));
    }
    .location-icon {
      opacity: .7;
      text-align: center;
      user-select: none;
    }
    button.location-icon,
    .breadcrumb {
      border: 0;
      background: transparent;
      color: inherit;
      font: inherit;
      cursor: pointer;
    }
    .breadcrumb-home {
      width: 22px;
      height: 22px;
      padding: 0;
      border-radius: 3px;
    }
    .breadcrumbs {
      display: flex;
      align-items: center;
      min-width: 0;
      overflow: hidden;
      white-space: nowrap;
    }
    .breadcrumb {
      min-width: 0;
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      padding: 0;
      text-align: left;
    }
    .edit-path,
    .cancel {
      width: 22px;
      height: 22px;
      padding: 0;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      cursor: pointer;
      font: 12px/22px var(--vsc-font-family, system-ui, sans-serif);
      text-align: center;
      flex: 0 0 auto;
    }
    .edit-path:hover,
    .cancel:hover,
    .breadcrumb-home:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }
    .location-input {
      width: 100%;
      min-width: 0;
      border: 0;
      outline: 0;
      background: transparent;
      color: inherit;
      font: inherit;
    }
    .nav-button {
      min-width: 28px;
      min-height: 28px;
      border: 1px solid transparent;
      border-radius: 4px;
      background: transparent;
      color: inherit;
      font: inherit;
      cursor: pointer;
    }
    .nav-button:hover:not(:disabled) {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      border-color: var(--vsc-border, var(--vsc-control-background));
    }
    .nav-button:disabled {
      opacity: .38;
      cursor: default;
    }
    .nav-button.pin {
      font-size: 18px;
      line-height: 1;
    }
    .nav-button.pin.pinned {
      color: var(--vsc-accent-blue, var(--vsc-focus-border));
    }
    .status-row {
      display: flex;
      align-items: center;
      gap: 12px;
      min-height: 28px;
      padding: 0 10px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-panel-background, var(--vsc-editor-background, #fff));
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
    }
    .status-row .hint {
      margin-left: auto;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }
    .action-bar {
      display: flex;
      align-items: center;
      gap: 4px;
      min-height: 34px;
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
    .action-bar button.active {
      background: var(--vsc-list-active-background, var(--vsc-list-selected-emphasis));
      color: var(--vsc-list-active-foreground, var(--vsc-on-accent));
    }
    .action-bar .danger:not(:disabled) {
      color: var(--vsc-error-foreground, var(--vsc-error));
    }
    .view-switcher {
      display: flex;
      gap: 2px;
    }
    .action-separator {
      width: 1px;
      height: 18px;
      margin: 0 2px;
      background: var(--vsc-border);
    }
    .plugin-action{display:inline-flex;align-items:center;gap:5px;padding:3px 9px}
    .plugin-icon{display:inline-grid;place-items:center;width:16px;height:16px}
    .error{display:flex;flex:1 1 0%;flex-direction:column;align-items:center;justify-content:center;gap:6px;padding:20px;text-align:center}
    app-registry-pane,app-autorun-pane,app-network-pane,app-environment-pane,app-system-table-pane{display:flex;flex:1 1 0%;min-width:0;min-height:0;overflow:hidden;background:var(--vsc-editor-background,#fff)}
  `],
})
export class PaneHostComponent implements DoCheck, PanePeerPort, PaneStatePort {
  @ViewChild('component') private inner?: DirectoryPaneComponent | RegistryPaneComponent | AutorunPaneComponent | NetworkPaneComponent | EnvironmentPaneComponent;
  @ViewChild('locationEditor') private locationEditor?: ElementRef<HTMLInputElement>;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() active = false;
  @Input() showHidden = false;
  @Input() currentPathPinned = false;
  @Input() pluginToolbarActions: readonly PresentedCommand[] = [];
  @Input() comparisonStatuses: ReadonlyMap<string, PaneComparisonStatus> = new Map<string, PaneComparisonStatus>();

  specializedLocationEditing = false;
  specializedLocationDraft = '';

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
    private readonly pluginManager: PluginManagerService,
    private readonly inputEngine: InputEngineService,
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

  get effectivePluginToolbarActions(): readonly PresentedCommand[] {
    if (this.pluginToolbarActions.length) {
      return this.pluginToolbarActions;
    }

    const component = this.activeComponent;
    const currentDirectory =
      component?.path ??
      this.workspaceService.getActiveTab(this.paneId)?.path ??
      '';

    if (!currentDirectory) return [];

    const selectedPaths = Array.from(component?.selectedPaths ?? [])
      .filter(path => !!path && path !== '__parent__');

    const context: ToolbarContext = {
      selectedPaths,
      currentDirectory,
    };

    const contributed = adaptPluginToolbarActions(
      this.pluginManager.resolveToolbarActions(context),
      context,
      this.inputEngine,
    );

    if (!/^bin:\/\//i.test(currentDirectory)) {
      return contributed;
    }

    const commandId = 'reassembly.trash.emptyBin';
    const invocation = {
      source: 'toolbar' as const,
      context: {
        selectedPaths,
        currentDirectory,
        paneId: this.paneId,
      },
    };
    const state = this.inputEngine.getCommandState(commandId, invocation);
    const emptyBin: PresentedCommand = {
      id: 'bin.empty',
      label: 'Empty Bin',
      iconText: '🗑',
      enabled: !!state?.registered && state.enabled,
      source: 'builtin',
      commandId,
      execute: () => {
        this.inputEngine.dispatchCommandById(commandId, invocation);
      },
    };

    return [emptyBin, ...contributed];
  }

  ngDoCheck(): void {
    if (this.rendererKind === 'directory' || this.rendererKind === 'registry' || this.rendererKind === 'autorun' || this.rendererKind === 'network' || this.rendererKind === 'environment') return;
    const path = this.workspaceService.getActiveTab(this.paneId)?.path ?? '';
    void this.provider.sync(this.paneId, path, this.showHidden);
    if (!this.specializedLocationEditing) this.specializedLocationDraft = this.provider.path;
  }

  get rendererKind(): PaneRendererKind {
    const path = this.workspaceService.getActiveTab(this.paneId)?.path ?? '';
    return this.renderers.resolve(path).kind;
  }

  private get activeComponent(): PaneComponent | undefined {
    return this.rendererKind === 'directory' || this.rendererKind === 'registry' || this.rendererKind === 'autorun' || this.rendererKind === 'network' || this.rendererKind === 'environment'
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

  beginSpecializedLocationEdit(): void {
    this.specializedLocationDraft = this.provider.path;
    this.specializedLocationEditing = true;
    queueMicrotask(() => this.locationEditor?.nativeElement.focus());
  }

  cancelSpecializedLocationEdit(): void {
    this.specializedLocationDraft = this.provider.path;
    this.specializedLocationEditing = false;
  }

  commitSpecializedLocationEdit(): void {
    const next = this.specializedLocationDraft.trim();
    this.specializedLocationEditing = false;
    if (!next || next === this.provider.path) return;
    void this.provider.navigateTo(next);
  }

  onProviderTabChange(tab: any): void { this.workspaceService.switchTab(this.paneId, tab.id); }
  onProviderTabClose(tab: any): void { this.workspaceService.closeTab(this.paneId, tab.id); }
  onProviderTabPin(tab: any): void { this.workspaceService.toggleTabPin(this.paneId, tab.id); }
}