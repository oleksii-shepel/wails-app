import { ChangeDetectionStrategy, Component, EventEmitter, Input, OnChanges, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import type { FileInfo } from '../shared/protocol';
import { PaneComparisonResult, PaneComparisonService } from '../services/pane-comparison.service';

@Component({
  selector: 'app-pane-comparison-bar',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="compare-bar" *ngIf="visible">
      <div class="compare-summary" [title]="leftPath + ' ↔ ' + rightPath">
        <strong>Compare</strong>
        <span class="metric only-left" *ngIf="result.summary.onlyLeft">{{ result.summary.onlyLeft }} left only</span>
        <span class="metric only-right" *ngIf="result.summary.onlyRight">{{ result.summary.onlyRight }} right only</span>
        <span class="metric newer" *ngIf="result.summary.leftNewer">{{ result.summary.leftNewer }} newer left</span>
        <span class="metric newer" *ngIf="result.summary.rightNewer">{{ result.summary.rightNewer }} newer right</span>
        <span class="metric different" *ngIf="result.summary.sizeDifferent">{{ result.summary.sizeDifferent }} size diff</span>
        <span class="metric different" *ngIf="result.summary.typeDifferent">{{ result.summary.typeDifferent }} type diff</span>
        <span class="metric same" *ngIf="result.summary.different === 0">{{ result.summary.same }} same</span>
      </div>

      <div class="compare-actions">
        <button type="button" class="btn" (click)="selectLeft.emit()" [disabled]="leftDifferenceCount === 0" title="Select differing items in the left pane">Select Left</button>
        <button type="button" class="btn" (click)="selectRight.emit()" [disabled]="rightDifferenceCount === 0" title="Select differing items in the right pane">Select Right</button>
        <button type="button" class="btn" (click)="copyMissingLeftToRight.emit()" [disabled]="result.summary.onlyLeft === 0" title="Copy items that exist only on the left to the right pane">Missing →</button>
        <button type="button" class="btn" (click)="copyMissingRightToLeft.emit()" [disabled]="result.summary.onlyRight === 0" title="Copy items that exist only on the right to the left pane">← Missing</button>
        <button type="button" class="btn" (click)="review.emit()" [disabled]="result.summary.different === 0" title="Review and approve a synchronization plan">Review…</button>
        <button type="button" class="close" (click)="close.emit()" title="Close comparison">×</button>
      </div>
    </div>
  `,
  styles: [`
    :host { display: block; flex: 0 0 auto; }
    .compare-bar {
      min-height: 34px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 4px 10px;
      border-top: 1px solid var(--vsc-border, rgba(127,127,127,.25));
      border-bottom: 1px solid var(--vsc-border, rgba(127,127,127,.25));
      background: color-mix(in srgb, var(--vsc-background, #1e1e1e) 92%, var(--vsc-accent-blue, #0078d4));
      font-size: 12px;
    }
    .compare-summary, .compare-actions { display: flex; align-items: center; gap: 8px; min-width: 0; }
    .compare-summary { overflow: hidden; white-space: nowrap; }
    .metric { opacity: .9; }
    .only-left, .only-right { font-weight: 600; }
    .different { font-weight: 600; }
    .same { opacity: .72; }
    .btn, .close {
      border: 1px solid var(--vsc-border, rgba(127,127,127,.35));
      background: var(--vsc-button-background, transparent);
      color: inherit;
      border-radius: 3px;
      padding: 3px 8px;
      font: inherit;
      cursor: pointer;
    }
    .btn:disabled { opacity: .42; cursor: default; }
    .close { width: 25px; padding-inline: 0; font-size: 16px; line-height: 18px; }
  `],
})
export class PaneComparisonBarComponent implements OnChanges {
  @Input() visible = false;
  @Input() leftEntries: readonly FileInfo[] = [];
  @Input() rightEntries: readonly FileInfo[] = [];
  @Input() leftPath = '';
  @Input() rightPath = '';

  @Output() selectLeft = new EventEmitter<void>();
  @Output() selectRight = new EventEmitter<void>();
  @Output() copyMissingLeftToRight = new EventEmitter<void>();
  @Output() copyMissingRightToLeft = new EventEmitter<void>();
  @Output() review = new EventEmitter<void>();
  @Output() close = new EventEmitter<void>();

  result: PaneComparisonResult = this.comparison.compare([], []);

  constructor(private readonly comparison: PaneComparisonService) {}

  ngOnChanges(): void {
    this.result = this.comparison.compare(this.leftEntries, this.rightEntries);
  }

  get leftDifferenceCount(): number {
    return this.comparison.leftDifferencePaths(this.result).length;
  }

  get rightDifferenceCount(): number {
    return this.comparison.rightDifferencePaths(this.result).length;
  }
}
