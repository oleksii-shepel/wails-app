import { ChangeDetectionStrategy, Component, EventEmitter, Input, OnChanges, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import type { FileInfo } from '../shared/protocol';
import { PaneComparisonResult, PaneComparisonService, type PaneComparisonPolicy } from '../services/pane-comparison.service';

@Component({
  selector: 'app-pane-comparison-bar',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="compare-bar" *ngIf="visible">
      <div class="compare-summary" [title]="leftPath + ' ↔ ' + rightPath">
        <strong>{{ recursive ? 'Compare · Recursive' : 'Compare' }}</strong>
        <span class="metric" *ngIf="scanning">Scanning…</span>
        <span class="metric different" *ngIf="error">{{ error }}</span>
        <span class="metric only-left" *ngIf="result.summary.onlyLeft">{{ result.summary.onlyLeft }} left only</span>
        <span class="metric only-right" *ngIf="result.summary.onlyRight">{{ result.summary.onlyRight }} right only</span>
        <span class="metric newer" *ngIf="result.summary.leftNewer">{{ result.summary.leftNewer }} newer left</span>
        <span class="metric newer" *ngIf="result.summary.rightNewer">{{ result.summary.rightNewer }} newer right</span>
        <span class="metric different" *ngIf="result.summary.sizeDifferent">{{ result.summary.sizeDifferent }} size diff</span>
        <span class="metric different" *ngIf="result.summary.typeDifferent">{{ result.summary.typeDifferent }} type diff</span>
        <span class="metric different" *ngIf="result.summary.hashDifferent">{{ result.summary.hashDifferent }} hash diff</span>
        <span class="metric different" *ngIf="result.summary.contentDifferent">{{ result.summary.contentDifferent }} content diff</span>
        <span class="metric same" *ngIf="result.summary.different === 0">{{ result.summary.same }} same</span>
      </div>

      <div class="compare-actions">
        <label class="policy" title="Comparison policy">
          <span>Policy</span>
          <select [value]="policy" (change)="policyChange.emit($any($event.target).value)" [disabled]="scanning">
            <option value="size-date">Size + date</option>
            <option value="size">Size</option>
            <option value="date">Date</option>
            <option value="hash">SHA-256</option>
            <option value="content">Content</option>
          </select>
        </label>
        <button type="button" class="btn" (click)="toggleRecursive.emit()" [disabled]="scanning" [class.active]="recursive" [attr.aria-pressed]="recursive" aria-label="Compare complete directory trees" title="Compare complete directory trees">Recursive</button>
        <button type="button" class="btn" *ngIf="recursive" (click)="refreshRecursive.emit()" [disabled]="scanning" aria-label="Rescan both directory trees" title="Rescan both directory trees">Refresh</button>
        <button type="button" class="btn" (click)="selectLeft.emit()" [disabled]="leftDifferenceCount === 0" aria-label="Select differing items in the left pane" title="Select differing items in the left pane">Select Left</button>
        <button type="button" class="btn" (click)="selectRight.emit()" [disabled]="rightDifferenceCount === 0" aria-label="Select differing items in the right pane" title="Select differing items in the right pane">Select Right</button>
        <button type="button" class="btn" (click)="copyMissingLeftToRight.emit()" [disabled]="result.summary.onlyLeft === 0" aria-label="Copy items that exist only on the left to the right pane" title="Copy items that exist only on the left to the right pane">Missing →</button>
        <button type="button" class="btn" (click)="copyMissingRightToLeft.emit()" [disabled]="result.summary.onlyRight === 0" aria-label="Copy items that exist only on the right to the left pane" title="Copy items that exist only on the right to the left pane">← Missing</button>
        <button type="button" class="btn" (click)="review.emit()" [disabled]="result.summary.different === 0" aria-label="Review and approve a synchronization plan" title="Review and approve a synchronization plan">Review…</button>
        <button type="button" class="close" (click)="close.emit()" aria-label="Close pane comparison" title="Close comparison">×</button>
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
      border-top: 1px solid var(--vsc-border, var(--vsc-neutral-overlay-medium));
      border-bottom: 1px solid var(--vsc-border, var(--vsc-neutral-overlay-medium));
      background: color-mix(in srgb, var(--vsc-background, var(--vsc-editor-background)) 92%, var(--vsc-accent-blue, var(--vsc-accent-blue)));
      font-size: 12px;
    }
    .compare-summary, .compare-actions { display: flex; align-items: center; gap: 8px; min-width: 0; }
    .compare-summary { overflow: hidden; white-space: nowrap; }
    .metric { opacity: .9; }
    .only-left, .only-right { font-weight: 600; }
    .different { font-weight: 600; }
    .same { opacity: .72; }
    .policy { display:flex; align-items:center; gap:4px; opacity:.9; }
    .policy select { background:var(--vsc-input-background,var(--vsc-code-background)); color:inherit; border:1px solid var(--vsc-border,var(--vsc-neutral-overlay-strong)); border-radius:3px; padding:2px 4px; font:inherit; }
    .btn, .close {
      border: 1px solid var(--vsc-border, var(--vsc-neutral-overlay-strong));
      background: var(--vsc-button-background, transparent);
      color: inherit;
      border-radius: 3px;
      padding: 3px 8px;
      font: inherit;
      cursor: pointer;
    }
    .btn:disabled { opacity: .42; cursor: default; }
    .btn.active { background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 24%, transparent); }
    .close { width: 25px; padding-inline: 0; font-size: 16px; line-height: 18px; }
  `],
})
export class PaneComparisonBarComponent implements OnChanges {
  @Input() visible = false;
  @Input() leftEntries: readonly FileInfo[] = [];
  @Input() rightEntries: readonly FileInfo[] = [];
  @Input() leftPath = '';
  @Input() rightPath = '';
  @Input() recursive = false;
  @Input() scanning = false;
  @Input() error = '';
  @Input() comparisonResult: PaneComparisonResult | null = null;
  @Input() policy: PaneComparisonPolicy = 'size-date';

  @Output() policyChange = new EventEmitter<PaneComparisonPolicy>();
  @Output() toggleRecursive = new EventEmitter<void>();
  @Output() refreshRecursive = new EventEmitter<void>();
  @Output() selectLeft = new EventEmitter<void>();
  @Output() selectRight = new EventEmitter<void>();
  @Output() copyMissingLeftToRight = new EventEmitter<void>();
  @Output() copyMissingRightToLeft = new EventEmitter<void>();
  @Output() review = new EventEmitter<void>();
  @Output() close = new EventEmitter<void>();

  result: PaneComparisonResult = this.comparison.compare([], []);

  constructor(private readonly comparison: PaneComparisonService) {}

  ngOnChanges(): void {
    this.result = this.comparisonResult ?? this.comparison.compare(this.leftEntries, this.rightEntries, { policy: this.policy });
  }

  get leftDifferenceCount(): number {
    return this.comparison.leftDifferencePaths(this.result).length;
  }

  get rightDifferenceCount(): number {
    return this.comparison.rightDifferencePaths(this.result).length;
  }
}