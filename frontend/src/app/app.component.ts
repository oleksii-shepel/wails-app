import {
  AfterViewInit,
  Component,
  OnDestroy,
  ViewChild,
} from '@angular/core';
import { ConfirmDialogComponent } from './components/confirm-dialog.component';
import { ReassemblyDualPaneComponent } from './components/reassembly-dual-pane.component';
import { ShortcutDebugOverlayComponent } from './components/shortcut-debug-overlay.component';
import { ReassemblyOperationSurfaceComponent } from './components/reassembly-operation-surface.component';
import { BatchRenameDialogComponent } from './components/batch-rename-dialog.component';
import { RemoteConnectDialogComponent } from './components/remote-connect-dialog.component';
import { PluginBootstrapService } from './services/plugin-bootstrap.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    ReassemblyDualPaneComponent,
    ShortcutDebugOverlayComponent,
    ReassemblyOperationSurfaceComponent,
    BatchRenameDialogComponent,
    RemoteConnectDialogComponent,
    ConfirmDialogComponent,
  ],
  template: `
    <main class="app-shell">
      <app-reassembly-dual-pane #manager></app-reassembly-dual-pane>

      <app-shortcut-debug-overlay
        [visible]="manager.shortcutHelpVisible"
        [path]="manager.activePane?.path ?? ''"
        [cursorName]="manager.activePane?.cursorEntry?.name ?? null"
        [selectedCount]="manager.activePane?.selectedPaths?.size ?? 0"
        [lastAction]="manager.activePane?.lastAction ?? 'startup'"
        [activePane]="manager.activePaneId"
        [otherPath]="manager.otherPane?.path ?? ''"
        [lastTransfer]="manager.lastTransfer"
        [lastDrop]="manager.lastDrop"
        [tabCount]="manager.activePane?.tabs?.length ?? 0"
        [surfaceKind]="manager.activePane?.activeTab?.kind ?? 'dir'"
        [splitPercent]="manager.leftPanePercent"
        [showHidden]="manager.showHidden"
        [bindings]="manager.shortcutHelpItems">
      </app-shortcut-debug-overlay>

      <app-reassembly-operation-surface></app-reassembly-operation-surface>
      <app-batch-rename-dialog></app-batch-rename-dialog>
      <app-remote-connect-dialog></app-remote-connect-dialog>
      <app-confirm-dialog></app-confirm-dialog>
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
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background, #1e1e1e);
      color: var(--vsc-foreground, #cccccc);
    }
  `],
})
export class AppComponent implements AfterViewInit, OnDestroy {
  @ViewChild('manager')
  private manager?: ReassemblyDualPaneComponent;

  constructor(
    private readonly pluginBootstrap: PluginBootstrapService,
  ) {}

  ngAfterViewInit(): void {
    void this.pluginBootstrap.initialize(
      () => this.manager?.pluginSelectionPaths() ?? [],
    );
  }

  ngOnDestroy(): void {
    void this.pluginBootstrap.shutdown();
  }
}
