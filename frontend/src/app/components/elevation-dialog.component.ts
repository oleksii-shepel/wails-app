import { CommonModule } from '@angular/common';
import { Component } from '@angular/core';
import { ElevationDialogService } from '../services/elevation-dialog.service';

@Component({
  selector: 'app-elevation-dialog',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="overlay" *ngIf="service.state() as state">
      <section class="dialog" role="dialog" aria-modal="true" aria-label="Elevation required">
        <header class="header">
          <span class="title">Administrator Permission</span>
          <button class="close" (click)="service.dismiss()" aria-label="Close">x</button>
        </header>

        <div class="content">
          <div class="icon" [class.ok]="state.phase === 'result' && state.success">
            {{ state.phase === 'result' && state.success ? 'OK' : '!' }}
          </div>

          <div class="details">
            <p class="message">{{ state.message }}</p>
            <p class="meta"><strong>Operation:</strong> {{ state.operation }}</p>
            <p class="meta"><strong>Target:</strong> <code>{{ state.target }}</code></p>

            <p class="hint" *ngIf="state.phase === 'requested'">
              Click Continue to launch the OS elevation prompt.
            </p>

            <p class="error" *ngIf="state.phase === 'result' && !state.success && state.error">
              {{ state.error }}
            </p>
          </div>
        </div>

        <footer class="actions">
          <ng-container *ngIf="state.phase === 'requested'; else resultActions">
            <button class="btn secondary" [disabled]="service.busy()" (click)="service.cancel()">Cancel</button>
            <button class="btn" [disabled]="service.busy()" (click)="service.approve()">Continue</button>
          </ng-container>
          <ng-template #resultActions>
          <button class="btn" *ngIf="state.phase === 'result'" (click)="service.dismiss()">Close</button>
          </ng-template>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    .overlay {
      position: fixed;
      inset: 0;
      z-index: var(--vsc-z-modal);
      background: color-mix(in srgb, var(--vsc-editor-background) 75%, black);
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 20px;
    }
    .dialog {
      width: min(560px, calc(100vw - 40px));
      border: 1px solid var(--vsc-border);
      background: var(--vsc-panel-background);
      box-shadow: var(--vsc-widget-shadow);
      border-radius: var(--vsc-border-radius);
      overflow: hidden;
    }
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 10px 12px;
      background: var(--vsc-titlebar-background);
      border-bottom: 1px solid var(--vsc-border-subtle);
    }
    .title {
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground-bright);
      font-weight: 600;
    }
    .close {
      border: 1px solid transparent;
      background: transparent;
      color: var(--vsc-foreground);
      width: 24px;
      height: 24px;
      cursor: pointer;
    }
    .close:hover {
      background: var(--vsc-list-hover-background);
      border-color: var(--vsc-border-subtle);
    }
    .content {
      display: grid;
      grid-template-columns: 34px 1fr;
      gap: 12px;
      padding: 14px 12px 10px;
    }
    .icon {
      width: 28px;
      height: 28px;
      border-radius: 999px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: 700;
      background: color-mix(in srgb, var(--vsc-warning) 22%, transparent);
      color: var(--vsc-warning);
      border: 1px solid color-mix(in srgb, var(--vsc-warning) 60%, transparent);
    }
    .icon.ok {
      background: color-mix(in srgb, var(--vsc-success) 22%, transparent);
      color: var(--vsc-success);
      border-color: color-mix(in srgb, var(--vsc-success) 60%, transparent);
    }
    .details p {
      margin: 0 0 6px;
      line-height: 1.35;
    }
    .message {
      color: var(--vsc-foreground-bright);
      font-size: var(--vsc-font-size);
      margin-bottom: 10px;
    }
    .meta {
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size-small);
    }
    code {
      font-family: var(--vsc-font-family-mono);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-border-subtle);
      padding: 1px 4px;
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-bright);
    }
    .hint {
      margin-top: 8px;
      color: var(--vsc-info);
      font-size: var(--vsc-font-size-small);
    }
    .error {
      margin-top: 8px;
      color: var(--vsc-error);
      font-size: var(--vsc-font-size-small);
      white-space: pre-wrap;
      word-break: break-word;
    }
    .actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      padding: 10px 12px 12px;
      border-top: 1px solid var(--vsc-border-subtle);
    }
  `]
})
export class ElevationDialogComponent {
  constructor(public readonly service: ElevationDialogService) {}
}
