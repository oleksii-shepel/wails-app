import { Component } from '@angular/core';
import { ReassemblyDirectoryPaneComponent } from './components/reassembly-directory-pane.component';
import { ShortcutDebugOverlayComponent } from './components/shortcut-debug-overlay.component';
import { ConfirmDialogComponent } from './components/confirm-dialog.component';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    ReassemblyDirectoryPaneComponent,
    ShortcutDebugOverlayComponent,
    ConfirmDialogComponent,
  ],
  template: `
    <main class="app-shell">
      <app-reassembly-directory-pane #pane></app-reassembly-directory-pane>

      <app-shortcut-debug-overlay
        [path]="pane.path"
        [cursorName]="pane.cursorEntry?.name ?? null"
        [selectedCount]="pane.selectedPaths.size"
        [lastAction]="pane.lastAction">
      </app-shortcut-debug-overlay>
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
