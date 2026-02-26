import { Component, Input } from '@angular/core';

@Component({
  selector: 'app-status-bar',
  standalone: true,
  template: `
    <div class="status-bar">
      <span>{{ statusText }}</span>
    </div>
  `,
  styles: [`
    .status-bar {
      display: flex;
      align-items: center;
      height: 28px;
      padding: 0 12px;
      background: #222;
      color: #eee;
      font-size: 13px;
      border-top: 1px solid #333;
      box-shadow: 0 -1px 2px rgba(0,0,0,0.05);
    }
    .status-bar span {
      flex: 1;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
  `]
})
export class StatusBarComponent {
  @Input() statusText: string = '';
}
