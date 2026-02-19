import { Component } from '@angular/core';
import { FileBrowserComponent } from './components/file-browser.component';
import { ThemeService } from './services/theme.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [FileBrowserComponent],
  template: `<app-file-browser></app-file-browser>`,
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
  constructor(public theme: ThemeService) {}
}