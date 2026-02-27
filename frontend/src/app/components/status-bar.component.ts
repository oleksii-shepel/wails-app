import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { OperationStatusBarComponent } from './operation-status-bar.component';

@Component({
  selector: 'app-status-bar',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="status-bar">
      <div class="status-left">
        <span class="status-text">{{ itemsSelected }} items selected</span>
        <span class="divider"></span>
        <span class="status-text">{{ totalSize }}</span>
      </div>

      <div class="status-center">
      </div>

      <div class="status-right">
        <ng-content></ng-content>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      height: var(--vsc-statusbar-height, 22px);
      background: var(--vsc-statusbar-background, #007acc);
      border-top: 1px solid var(--vsc-border, #454545);
    }

    .status-bar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 100%;
      padding: 0 var(--vsc-padding-md, 8px);
      position: relative;
      font-size: var(--vsc-font-size-small, 12px);
      color: var(--vsc-foreground, #cccccc);
      font-family: var(--vsc-font-family, -apple-system, BlinkMacSystemFont, sans-serif);
    }

    .status-left, .status-right {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-md, 8px);
    }

    .divider {
      width: 1px;
      height: 14px;
      background: var(--vsc-border, #454545);
    }

    .status-center {
      position: absolute;
      left: 50%;
      transform: translateX(-50%);
    }
  `]
})
export class StatusBarComponent {
  @Input() itemsSelected = 0;
  @Input() totalSize = '0 B';
  @Input() currentPath = '/';
}