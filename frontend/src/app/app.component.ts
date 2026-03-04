import { Component } from '@angular/core';
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
export class AppComponent {
  title = 'File Manager';
  constructor(public theme: ThemeService, public operationService: OperationService) {}
}