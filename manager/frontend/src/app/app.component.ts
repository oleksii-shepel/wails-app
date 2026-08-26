import {
  ChangeDetectorRef,
  Component,
  HostListener,
  NgZone,
  OnInit,
  OnDestroy,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { ConfirmDialogComponent } from './components/confirm-dialog.component';
import { DualPaneComponent } from './components/dual-pane.component';
import { ApplicationMenuBarComponent } from './components/application-menu-bar.component';
import { OperationSurfaceComponent } from './components/operation-surface.component';
import { ActionFeedbackComponent } from './components/action-feedback.component';
import { BatchRenameDialogComponent } from './components/batch-rename-dialog.component';
import { RemoteConnectDialogComponent } from './components/remote-connect-dialog.component';
import { SettingsDialogComponent } from './components/settings-dialog.component';
import { TrashDialogComponent } from './components/trash-dialog.component';
import { PerfOverlayComponent } from './components/perf-overlay.component';
import { PluginBootstrapService } from './services/plugin-bootstrap.service';
import { ModalStackService } from './services/modal-stack.service';
import { InputCustomizationService } from './services/input-customization.service';
import { RegistryPluginHostComponent } from '@reassembly/registry-plugin';
import { BatchRenameDialogService } from './services/batch-rename-dialog.service';
import { RemoteConnectDialogService } from './services/remote-connect-dialog.service';
import { SettingsService } from './services/settings.service';
import { TrashDialogService } from './services/trash-dialog.service';
import { ConfirmDialogService } from './services/confirm-dialog.service';
import { PaneCompareDialogService } from './services/pane-compare-dialog.service';
import { PaneRecursiveCompareDialogService } from './services/pane-recursive-compare-dialog.service';
import { PaneSyncReviewDialogService } from './services/pane-sync-review-dialog.service';
import { PaneCompareDialogComponent } from './components/pane-compare-dialog.component';
import { PaneRecursiveCompareDialogComponent } from './components/pane-recursive-compare-dialog.component';
import { PaneSyncReviewComponent } from './components/pane-sync-review.component';
import { ArchiveFilesDialogComponent } from './components/archive-files-dialog.component';
import { ArchiveFilesDialogService } from './services/archive-files-dialog.service';
import { CommandPaletteComponent, type PaletteCommand } from './components/command-palette.component';
import { GlobalSearchComponent } from './components/global-search.component';
import { ToolsCenterComponent } from './components/tools-center.component';
import { DiskUsageDialogComponent } from './components/disk-usage-dialog.component';
import { SelectionBasketComponent } from './components/selection-basket.component';
import { WorkspaceSwitcherComponent } from './components/workspace-switcher.component';
import { PathHistoryComponent } from './components/path-history.component';
import { QuickAccessComponent } from './components/quick-access.component';
import { RemoteConnectionsComponent } from './components/remote-connections.component';
import { ClosedTabsComponent } from './components/closed-tabs.component';
import { OperationJournalComponent } from './components/operation-journal.component';
import { KeyboardHelpComponent } from './components/keyboard-help.component';
import { InputCenterComponent } from './components/input-center.component';
import {
  ApplicationOverlayHostService,
  type ApplicationOverlayHost,
  type AppOverlayHistoryMode,
  type AppOverlayToolsPage,
} from './services/application-overlay-host.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    
    ApplicationMenuBarComponent,DualPaneComponent,
    OperationSurfaceComponent,
    ActionFeedbackComponent,
    BatchRenameDialogComponent,
    RemoteConnectDialogComponent,
    SettingsDialogComponent,
    TrashDialogComponent,
    PerfOverlayComponent,
    ConfirmDialogComponent,
    PaneCompareDialogComponent,
    PaneRecursiveCompareDialogComponent,
    PaneSyncReviewComponent,
    ArchiveFilesDialogComponent,
    RegistryPluginHostComponent,
    CommandPaletteComponent,
    GlobalSearchComponent,
    ToolsCenterComponent,
    DiskUsageDialogComponent,
    SelectionBasketComponent,
    WorkspaceSwitcherComponent,
    PathHistoryComponent,
    QuickAccessComponent,
    RemoteConnectionsComponent,
    ClosedTabsComponent,
    OperationJournalComponent,
    KeyboardHelpComponent,
    InputCenterComponent,
  ],
  template: `
    <main class="app-shell">
      <app-application-menu-bar></app-application-menu-bar>
      <app-dual-pane *ngIf="pluginsReady" #manager></app-dual-pane>
      <div *ngIf="!pluginsReady" class="startup-shell" aria-live="polite">
        Loading workspace…
      </div>


      <app-operation-surface></app-operation-surface>
      <app-action-feedback></app-action-feedback>
      <app-batch-rename-dialog *ngIf="batchRenameMounted$ | async"></app-batch-rename-dialog>
      <app-remote-connect-dialog *ngIf="remoteConnectMounted$ | async"></app-remote-connect-dialog>
      <app-settings-dialog *ngIf="settingsMounted$ | async"></app-settings-dialog>
      <app-trash-dialog *ngIf="trashMounted$ | async"></app-trash-dialog>
      <app-confirm-dialog *ngIf="confirmMounted$ | async"></app-confirm-dialog>
      <app-pane-compare-dialog *ngIf="paneCompareMounted$ | async"></app-pane-compare-dialog>
      <app-pane-recursive-compare-dialog *ngIf="paneRecursiveCompareMounted$ | async"></app-pane-recursive-compare-dialog>
      <app-pane-sync-review *ngIf="paneSyncReviewMounted$ | async"></app-pane-sync-review>
      <app-archive-files-dialog *ngIf="archiveFilesMounted$ | async"></app-archive-files-dialog>

      <app-command-palette #commandPalette [commands]="commandPaletteCommands"></app-command-palette>
      <app-global-search
        #globalSearch
        (openResult)="managerRef?.openSearchResult($event)"
        (openContainingFolder)="managerRef?.openSearchContainingFolder($event)"
        (openAsPanel)="managerRef?.openSearchResultsPanel($event)"
        (openAsWorkspace)="managerRef?.openSearchWorkspace($event)"
        (openPanelPath)="managerRef?.openDiscoveryPanelPath($event)">
      </app-global-search>
      <app-tools-center
        #toolsCenter
        (changed)="managerRef?.refreshBothPanes()"
        (navigateTo)="managerRef?.navigateFromTools($event)"
        (openBasketRequested)="selectionBasket?.show()">
      </app-tools-center>
      <app-disk-usage-dialog
        [visible]="diskUsageDialogVisible"
        [rootPath]="diskUsageDialogPath"
        (closed)="closeDiskUsageDialog()">
      </app-disk-usage-dialog>
      <app-selection-basket
        #selectionBasket
        (copyTo)="managerRef?.copyBasketTo($event)"
        (moveTo)="managerRef?.moveBasketTo($event)"
        (deleteRequested)="managerRef?.deleteBasket()">
      </app-selection-basket>
      <app-workspace-switcher
        #workspaceSwitcher
        (workspaceActivated)="managerRef?.applyActiveWorkspace()"
        (toolWorkspaceActivated)="managerRef?.activateToolWorkspace($event)">
      </app-workspace-switcher>
      <app-path-history
        #pathHistory
        [paneId]="managerRef?.activePaneId ?? 'left'"
        [currentPath]="managerRef?.activePane?.contextState?.path ?? ''"
        (navigate)="managerRef?.navigateFromHistory($event)">
      </app-path-history>
      <app-quick-access
        #quickAccess
        [paneId]="managerRef?.activePaneId ?? 'left'"
        [currentPath]="managerRef?.activePane?.contextState?.path ?? ''"
        [currentSurfaceId]="managerRef?.activePane?.contextState?.activeSurfaceId || 'dir'"
        (navigate)="managerRef?.navigateFromQuickAccess($event)"
        (openTarget)="managerRef?.openQuickAccessTarget($event)">
      </app-quick-access>
      <app-remote-connections
        #remoteConnections
        (navigate)="managerRef?.navigateFromRemoteConnections($event)">
      </app-remote-connections>
      <app-closed-tabs
        [visible]="closedTabsVisible"
        (closed)="closedTabsVisible = false"
        (reopenRequested)="managerRef?.reopenClosedTabEntry($event)">
      </app-closed-tabs>
      <app-operation-journal
        [visible]="operationJournalVisible"
        (closed)="operationJournalVisible = false"
        (undoRequested)="managerRef?.undoLastOperation()"
        (redoRequested)="managerRef?.redoLastOperation()">
      </app-operation-journal>
      <app-keyboard-help #keyboardHelp></app-keyboard-help>
      <app-input-center
        [visible]="inputCenterVisible"
        (closed)="inputCenterVisible = false">
      </app-input-center>
      <app-perf-overlay></app-perf-overlay>
      <reassembly-registry-plugin-host></reassembly-registry-plugin-host>
    </main>
  `,
  styles: [`
    :host {
      display: block;
      width: 100vw;
      height: 100vh;
      overflow: hidden;
    }

    .app-shell {
      flex-direction: column;
      display: flex;
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background, var(--vsc-editor-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }

    .app-shell > app-dual-pane {
      display: flex;
      flex: 1 1 0%;
      width: 100%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }
    .startup-shell {
      display: grid;
      place-items: center;
      width: 100%;
      flex: 1 1 0%;
      min-height: 0;
      font: 14px/1.4 var(--vsc-font-family, system-ui, sans-serif);
      color: var(--vsc-description-foreground, var(--vsc-foreground));
    }
  `],
})
export class AppComponent implements OnInit, OnDestroy, ApplicationOverlayHost {
  readonly batchRenameMounted$ = this.batchRenameDialog.mountedRequest$;
  readonly remoteConnectMounted$ = this.remoteConnectDialog.mountedRequest$;
  readonly settingsMounted$ = this.settingsService.getMountedRequest$();
  readonly trashMounted$ = this.trashDialog.mountedRequest$;
  readonly confirmMounted$ = this.confirmDialog.mountedRequest$;
  readonly paneCompareMounted$ = this.paneCompareDialog.mountedRequest$;
  readonly paneRecursiveCompareMounted$ = this.paneRecursiveCompareDialog.mountedRequest$;
  readonly paneSyncReviewMounted$ = this.paneSyncReviewDialog.mountedRequest$;
  readonly archiveFilesMounted$ = this.archiveFilesDialog.mountedRequest$;

  commandPaletteCommands: PaletteCommand[] = [];
  inputCenterVisible = false;
  operationJournalVisible = false;
  closedTabsVisible = false;
  diskUsageDialogVisible = false;
  diskUsageDialogPath = '';
  managerRef: DualPaneComponent | undefined;

  @ViewChild('commandPalette') commandPalette?: CommandPaletteComponent;
  @ViewChild('globalSearch') globalSearch?: GlobalSearchComponent;
  @ViewChild('toolsCenter') toolsCenter?: ToolsCenterComponent;
  @ViewChild('selectionBasket') selectionBasket?: SelectionBasketComponent;
  @ViewChild('workspaceSwitcher') workspaceSwitcher?: WorkspaceSwitcherComponent;
  @ViewChild('pathHistory') pathHistory?: PathHistoryComponent;
  @ViewChild('quickAccess') quickAccess?: QuickAccessComponent;
  @ViewChild('remoteConnections') remoteConnections?: RemoteConnectionsComponent;
  @ViewChild('keyboardHelp') keyboardHelp?: KeyboardHelpComponent;

  @ViewChild('manager')
  set manager(value: DualPaneComponent | undefined) {
    this.managerRef = value;
    if (!value) return;
    this.pluginBootstrap.bindSelectionProvider(
      () => value.pluginSelectionPaths() ?? [],
    );
  }

  pluginsReady = false;

  constructor(
    private readonly pluginBootstrap: PluginBootstrapService,
    private readonly modalStack: ModalStackService,
    private readonly inputCustomization: InputCustomizationService,
    private readonly ngZone: NgZone,
    private readonly cdr: ChangeDetectorRef,
    private readonly batchRenameDialog: BatchRenameDialogService,
    private readonly remoteConnectDialog: RemoteConnectDialogService,
    private readonly settingsService: SettingsService,
    private readonly trashDialog: TrashDialogService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly paneCompareDialog: PaneCompareDialogService,
    private readonly paneRecursiveCompareDialog: PaneRecursiveCompareDialogService,
    private readonly paneSyncReviewDialog: PaneSyncReviewDialogService,
    private readonly archiveFilesDialog: ArchiveFilesDialogService,
    private readonly overlays: ApplicationOverlayHostService,
  ) {}

  @HostListener('document:keydown.escape', ['$event'])
  onGlobalEscape(event: Event): void {
    const keyboardEvent = event as KeyboardEvent;
    if (keyboardEvent.defaultPrevented) return;
    if (!this.modalStack.dismissTopmost()) return;

    keyboardEvent.preventDefault();
    keyboardEvent.stopPropagation();
  }

  ngOnInit(): void {
    this.overlays.bind(this);
    void this.bootstrapPlugins();
  }

  ngOnDestroy(): void {
    this.overlays.unbind(this);
    void this.pluginBootstrap.shutdown();
  }

  openCommandPalette(commands: readonly unknown[]): void {
    this.commandPaletteCommands = commands as PaletteCommand[];
    this.cdr.markForCheck();
    queueMicrotask(() => {
      const palette = this.commandPalette as (CommandPaletteComponent & { show?: () => void; open?: () => void }) | undefined;
      if (typeof palette?.show === 'function') palette.show();
      else if (typeof palette?.open === 'function') palette.open();
    });
  }

  openGlobalSearch(rootPath: string): void { this.globalSearch?.open(rootPath); }
  closeGlobalSearch(): void { this.globalSearch?.close(); }
  openTools(page: AppOverlayToolsPage, rootPath: string): void { this.toolsCenter?.open(page, rootPath); }
  showSelectionBasket(): void { this.selectionBasket?.show(); }
  openWorkspaceSwitcher(): void { this.workspaceSwitcher?.open(); }
  createWorkspace(): void { this.workspaceSwitcher?.createWorkspace(); }
  openPathHistory(mode: AppOverlayHistoryMode): void { this.pathHistory?.open(mode); }
  openQuickAccess(sectionId?: string): void { void this.quickAccess?.open(sectionId); }
  openRemoteConnections(): void { this.remoteConnections?.open(); }
  showKeyboardHelp(): void { this.keyboardHelp?.show(); }
  toggleInputCenter(): void { this.inputCenterVisible = !this.inputCenterVisible; this.cdr.markForCheck(); }
  showOperationJournal(): void { this.operationJournalVisible = true; this.cdr.markForCheck(); }
  showClosedTabs(): void { this.closedTabsVisible = true; this.cdr.markForCheck(); }
  closeClosedTabs(): void { this.closedTabsVisible = false; this.cdr.markForCheck(); }
  openDiskUsage(rootPath: string): void {
    this.diskUsageDialogPath = rootPath;
    this.diskUsageDialogVisible = true;
    this.cdr.markForCheck();
  }
  closeDiskUsageDialog(): void {
    this.diskUsageDialogVisible = false;
    this.diskUsageDialogPath = '';
    this.cdr.markForCheck();
  }

  private async bootstrapPlugins(): Promise<void> {
    try {
      await this.pluginBootstrap.initialize(() => []);
    } finally {
      this.ngZone.run(() => {
        this.pluginsReady = true;
        this.cdr.markForCheck();
      });
    }
  }
}