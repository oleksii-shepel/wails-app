import { Component, HostListener, OnDestroy, OnInit } from '@angular/core';
import { FileBrowserComponent } from './components/file-browser.component';
import { ConfirmDialogComponent } from './components/confirm-dialog.component';
import { ThemeService } from './services/theme.service';
import { OperationService } from './services/operation.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [FileBrowserComponent, ConfirmDialogComponent],
  template: `
    <app-file-browser></app-file-browser>
    <app-confirm-dialog></app-confirm-dialog>
  `,
  styles: [`
    :host {
      display: block;
      width: 100vw;
      height: 100vh;
      overflow: hidden;
    }
  `]
})
export class AppComponent implements OnInit, OnDestroy {
  title = 'File Manager';
  constructor(public theme: ThemeService, public operationService: OperationService) {}

  private captureDragGuard = (event: DragEvent): void => {
    if (this.isExternalFileDrag(event)) {
      // For dragenter/dragover: prevent default to enable drop
      // For drop: DON'T prevent default - let Wails handle it
      if (event.type === 'dragenter' || event.type === 'dragover') {
        event.preventDefault();
        if (event.dataTransfer) {
          event.dataTransfer.dropEffect = 'copy';
        }
      }
      // Let drop event propagate - Wails will intercept it
    }
  };

  ngOnInit(): void {
    // Wails' OnFileDrop() registers its own dragover/drop handlers - no custom listeners needed
  }

  ngOnDestroy(): void {
    window.removeEventListener('dragenter', this.captureDragGuard, true);
    window.removeEventListener('dragover', this.captureDragGuard, true);

    document.removeEventListener('dragenter', this.captureDragGuard, true);
    document.removeEventListener('dragover', this.captureDragGuard, true);
  }

  @HostListener('window:dragover', ['$event'])
  onWindowDragOver(event: DragEvent): void {
    if (this.isExternalFileDrag(event)) {
      event.preventDefault();
      if (event.dataTransfer) {
        event.dataTransfer.dropEffect = event.ctrlKey || event.metaKey ? 'copy' : 'move';
      }
    }
  }

  // Removed window:drop HostListener - let Wails handle drops natively

  private isExternalFileDrag(event: DragEvent): boolean {
    const types = event.dataTransfer?.types;
    if (!types) return false;
    if (types.includes('application/json')) return false;
    return types.includes('Files') || (event.dataTransfer?.files?.length ?? 0) > 0;
  }
}