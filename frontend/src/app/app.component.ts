import { Component } from '@angular/core';
import { ConfirmDialogComponent } from './components/confirm-dialog.component';
import { ReassemblyDualPaneComponent } from './components/reassembly-dual-pane.component';
import { ShortcutDebugOverlayComponent } from './components/shortcut-debug-overlay.component';
import { ReassemblyOperationSurfaceComponent } from './components/reassembly-operation-surface.component';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    ReassemblyDualPaneComponent,
    ShortcutDebugOverlayComponent,
    ReassemblyOperationSurfaceComponent,
    ConfirmDialogComponent,
  ],
  template: `
    <main class="app-shell">
      <app-reassembly-dual-pane #manager></app-reassembly-dual-pane>

      <app-shortcut-debug-overlay
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
        [splitPercent]="manager.leftPanePercent">
      </app-shortcut-debug-overlay>

      <app-reassembly-operation-surface></app-reassembly-operation-surface>
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
export class AppComponent {}
