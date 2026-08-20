import {
  Component,
  HostListener,
  OnInit,
  OnDestroy,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { ConfirmDialogComponent } from './components/confirm-dialog.component';
import { DualPaneComponent } from './components/dual-pane.component';
import { OperationSurfaceComponent } from './components/operation-surface.component';
import { ActionFeedbackComponent } from './components/action-feedback.component';
import { BatchRenameDialogComponent } from './components/batch-rename-dialog.component';
import { RemoteConnectDialogComponent } from './components/remote-connect-dialog.component';
import { SettingsDialogComponent } from './components/settings-dialog.component';
import { TrashDialogComponent } from './components/trash-dialog.component';
import { PluginBootstrapService } from './services/plugin-bootstrap.service';
import { ModalStackService } from './services/modal-stack.service';
import { InputCustomizationService } from './services/input-customization.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    DualPaneComponent,
    OperationSurfaceComponent,
    ActionFeedbackComponent,
    BatchRenameDialogComponent,
    RemoteConnectDialogComponent,
    SettingsDialogComponent,
    TrashDialogComponent,
    ConfirmDialogComponent,
  ],
  template: `
    <main class="app-shell">
      <app-dual-pane *ngIf="pluginsReady" #manager></app-dual-pane>


      <app-operation-surface></app-operation-surface>
      <app-action-feedback></app-action-feedback>
      <app-batch-rename-dialog></app-batch-rename-dialog>
      <app-remote-connect-dialog></app-remote-connect-dialog>
      <app-settings-dialog></app-settings-dialog>
      <app-trash-dialog></app-trash-dialog>
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
      background: var(--vsc-editor-background, var(--vsc-editor-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }
  `],
})
export class AppComponent implements OnInit, OnDestroy {
  @ViewChild('manager')
  set manager(value: DualPaneComponent | undefined) {
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
    void this.bootstrapPlugins();
  }

  ngOnDestroy(): void {
    void this.pluginBootstrap.shutdown();
  }

  private async bootstrapPlugins(): Promise<void> {
    try {
      await this.pluginBootstrap.initialize(() => []);
    } finally {
      this.pluginsReady = true;
    }
  }
}
