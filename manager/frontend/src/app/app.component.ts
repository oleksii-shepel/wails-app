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
    RegistryPluginHostComponent,
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
      <app-batch-rename-dialog></app-batch-rename-dialog>
      <app-remote-connect-dialog></app-remote-connect-dialog>
      <app-settings-dialog></app-settings-dialog>
      <app-trash-dialog></app-trash-dialog>
      <app-confirm-dialog></app-confirm-dialog>
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
    private readonly ngZone: NgZone,
    private readonly cdr: ChangeDetectorRef,
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
      this.ngZone.run(() => {
        this.pluginsReady = true;
        this.cdr.markForCheck();
      });
    }
  }
}
