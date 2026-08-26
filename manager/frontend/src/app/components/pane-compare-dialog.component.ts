import { ChangeDetectionStrategy, Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  PaneCompareDialogService,
  type PaneCompareDialogRequest,
} from '../services/pane-compare-dialog.service';
import type { PaneComparisonPolicy } from '../services/pane-comparison.service';

@Component({
  selector: 'app-pane-compare-dialog',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container *ngIf="activeRequest as request">
      <div class="dialog-backdrop" [class.dialog-backdrop--closing]="isClosing" (click)="dialogs.close()" (animationend)="onAnimationEnd($event)">
        <section class="dialog-panel" role="dialog" aria-modal="true" aria-label="Compare panes" (click)="$event.stopPropagation()">
          <header class="dialog-header">
            <div>
              <h2>Compare Panes</h2>
              <p [title]="request.viewModel.getLeftPath() + ' ↔ ' + request.viewModel.getRightPath()">
                {{ request.viewModel.getLeftPath() }} ↔ {{ request.viewModel.getRightPath() }}
              </p>
            </div>
            <button type="button" class="close" (click)="dialogs.close()">×</button>
          </header>

          <div class="dialog-body">
            <div class="metrics">
              <span class="metric only-left" *ngIf="request.viewModel.getResult().summary.onlyLeft">{{ request.viewModel.getResult().summary.onlyLeft }} left only</span>
              <span class="metric only-right" *ngIf="request.viewModel.getResult().summary.onlyRight">{{ request.viewModel.getResult().summary.onlyRight }} right only</span>
              <span class="metric newer" *ngIf="request.viewModel.getResult().summary.leftNewer">{{ request.viewModel.getResult().summary.leftNewer }} newer left</span>
              <span class="metric newer" *ngIf="request.viewModel.getResult().summary.rightNewer">{{ request.viewModel.getResult().summary.rightNewer }} newer right</span>
              <span class="metric different" *ngIf="request.viewModel.getResult().summary.sizeDifferent">{{ request.viewModel.getResult().summary.sizeDifferent }} size diff</span>
              <span class="metric different" *ngIf="request.viewModel.getResult().summary.typeDifferent">{{ request.viewModel.getResult().summary.typeDifferent }} type diff</span>
              <span class="metric different" *ngIf="request.viewModel.getResult().summary.hashDifferent">{{ request.viewModel.getResult().summary.hashDifferent }} hash diff</span>
              <span class="metric different" *ngIf="request.viewModel.getResult().summary.contentDifferent">{{ request.viewModel.getResult().summary.contentDifferent }} content diff</span>
              <span class="metric same" *ngIf="request.viewModel.getResult().summary.different === 0">{{ request.viewModel.getResult().summary.same }} same</span>
            </div>

            <div class="actions">
              <label class="policy">
                <span>Policy</span>
                <select [value]="request.viewModel.getPolicy()" (change)="onPolicyChange(request.viewModel, $event)">
                  <option value="size-date">Size + date</option>
                  <option value="size">Size</option>
                  <option value="date">Date</option>
                  <option value="hash">SHA-256</option>
                  <option value="content">Content</option>
                </select>
              </label>
              <button type="button" (click)="request.viewModel.onSelectLeft()" [disabled]="request.viewModel.getResult().summary.different === 0">Select Left</button>
              <button type="button" (click)="request.viewModel.onSelectRight()" [disabled]="request.viewModel.getResult().summary.different === 0">Select Right</button>
              <button type="button" (click)="request.viewModel.onCopyMissingLeftToRight()" [disabled]="request.viewModel.getResult().summary.onlyLeft === 0">Missing →</button>
              <button type="button" (click)="request.viewModel.onCopyMissingRightToLeft()" [disabled]="request.viewModel.getResult().summary.onlyRight === 0">← Missing</button>
              <button type="button" (click)="request.viewModel.onOpenRecursiveCompare()">Recursive Compare…</button>
              <button type="button" (click)="request.viewModel.onOpenSyncReview()" [disabled]="request.viewModel.getResult().summary.different === 0">Review Sync Plan…</button>
            </div>
          </div>
        </section>
      </div>
    </ng-container>
  `,
  styleUrls: ['./tools-dialog-remote-theme.scss'],
  styles: [`
    :host { position: fixed; inset: 0; z-index: 30010; }
    .dialog-backdrop { position: fixed; inset: 0; display: flex; align-items: center; justify-content: center; padding: 24px; background: var(--vsc-scrim); backdrop-filter: blur(2px); animation: fadeIn 0.2s ease; }
    .dialog-backdrop--closing { animation: fadeOut 0.2s ease forwards; pointer-events: none; }
    .dialog-panel { width: min(900px, calc(100vw - 48px)); overflow: hidden; background: var(--vsc-editor-background); color: var(--vsc-foreground); border: 1px solid var(--vsc-border-bright); border-radius: var(--vsc-border-radius-lg, 8px); box-shadow: var(--vsc-widget-shadow), 0 32px 64px var(--vsc-scrim-soft), 0 0 0 1px var(--vsc-neutral-overlay-subtle) inset; animation: slideDown 0.28s cubic-bezier(0.16, 1, 0.3, 1); }
    .dialog-backdrop--closing .dialog-panel { animation: slideUp 0.2s ease forwards; }
    @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
    @keyframes fadeOut { from { opacity: 1; } to { opacity: 0; } }
    @keyframes slideDown { from { transform: translateY(-16px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }
    @keyframes slideUp { from { transform: translateY(0); opacity: 1; } to { transform: translateY(-16px); opacity: 0; } }
    .dialog-header { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; padding:12px 16px; border-bottom:1px solid var(--vsc-border-subtle); background: var(--vsc-titlebar-background); }
    .dialog-header h2, .dialog-header p { margin:0; }
    .dialog-header h2 { color: var(--vsc-foreground-bright); font-size: 14px; line-height: 1.15; font-weight: 600; }
    .dialog-header p { margin-top:4px; color: var(--vsc-foreground-dim); font-size:12px; }
    .dialog-body { display:grid; gap:16px; padding:16px; background: var(--vsc-editor-background); }
    .metrics, .actions { display:flex; flex-wrap:wrap; gap:8px; align-items:center; }
    .metrics { padding: 12px 14px; border: 1px solid var(--vsc-border-subtle); border-radius: var(--vsc-border-radius, 6px); background: color-mix(in srgb, var(--vsc-foreground) 3%, var(--vsc-editor-background)); }
    .metric { display:inline-flex; align-items:center; min-height:24px; padding:0 8px; border-radius:999px; font-size:11px; color: var(--vsc-foreground-dim); background: color-mix(in srgb, var(--vsc-foreground) 5%, transparent); }
    .only-left, .only-right, .different { font-weight:600; color: var(--vsc-foreground-bright); }
    .policy { display:flex; gap:6px; align-items:center; padding-right: 8px; color: var(--vsc-foreground-dim); }
    button, select { border:1px solid transparent; background: var(--vsc-button-secondary, var(--vsc-control-background)); color:inherit; border-radius: var(--vsc-border-radius-sm, 4px); min-height:28px; padding:0 10px; font:inherit; transition: background-color var(--vsc-transition-fast), border-color var(--vsc-transition-fast), color var(--vsc-transition-fast); }
    button:hover:not(:disabled), select:hover:not(:disabled) { background: var(--vsc-button-secondary-hover, var(--vsc-list-hover-background)); }
    button:focus-visible, select:focus-visible { outline: 1px solid var(--vsc-input-focus-border); outline-offset: 1px; }
    .close { width:28px; padding:0; font-size:18px; background: transparent; color: var(--vsc-foreground-dim); }
    .close:hover { background: color-mix(in srgb, var(--vsc-foreground) 10%, transparent); color: var(--vsc-foreground-bright); }
  `],
})
export class PaneCompareDialogComponent {
  constructor(readonly dialogs: PaneCompareDialogService) {}

  get activeRequest(): PaneCompareDialogRequest | null {
    return this.dialogs.mountedRequest$.value;
  }

  get isClosing(): boolean {
    return !!this.dialogs.closingRequest$.value;
  }

  onPolicyChange(viewModel: { onPolicyChange(policy: PaneComparisonPolicy): void | Promise<void> }, event: Event): void {
    void viewModel.onPolicyChange((event.target as HTMLSelectElement).value as PaneComparisonPolicy);
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.dialogs.finishClose();
  }
}
