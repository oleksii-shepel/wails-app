import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  Input,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import {
  WorkspaceService,
  type RecentlyClosedTab,
} from '../services/workspace.service';
import { LocalDateTimePipe } from '../utils/local-date-time.pipe';
@Component({
  selector: 'app-closed-tabs',
  standalone: true,
  imports: [LocalDateTimePipe, CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop" *ngIf="visible" (mousedown)="close()">
      <section
        class="panel"
        role="dialog"
        aria-modal="true"
        aria-label="Recently Closed Tabs"
        (mousedown)="$event.stopPropagation()">
        <header>
          <div>
            <strong>Recently Closed Tabs</strong>
            <span>{{ entries.length }} recoverable tabs</span>
          </div>
          <button type="button" class="close" (click)="close()">×</button>
        </header>

        <div class="tabs">
          <button
            *ngFor="let entry of entries; trackBy: trackEntry"
            type="button"
            class="closed-tab"
            (click)="reopen(entry)">
            <span class="icon">{{ icon(entry) }}</span>
            <span class="main">
              <strong>{{ entry.tab.title }}</strong>
              <small>{{ entry.tab.path || viewerPath(entry) || entry.tab.surfaceId || 'surface' }}</small>
            </span>
            <span class="meta">
              {{ entry.paneId }} · {{ entry.closedAt | localDateTime:'time' }}
            </span>
          </button>

          <div class="empty" *ngIf="entries.length === 0">
            No recently closed tabs in this workspace.
          </div>
        </div>

        <footer>
          <button type="button" [disabled]="entries.length === 0" (click)="clear()">
            Clear
          </button>
          <button type="button" class="close" (click)="close()">Close</button>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:8750;pointer-events:none}
    .backdrop{position:absolute;inset:0;display:grid;place-items:center;padding:20px;background:var(--vsc-scrim);pointer-events:auto}
    .panel{display:grid;grid-template-rows:auto minmax(0,1fr) auto;width:min(720px,calc(100vw - 40px));height:min(600px,calc(100vh - 40px));overflow:hidden;border:1px solid var(--vsc-border);border-radius:7px;background:var(--vsc-dialog-background,var(--vsc-panel-background));color:var(--vsc-foreground);box-shadow:0 14px 48px var(--vsc-scrim);font:11px/1.35 var(--vsc-font-family,system-ui,sans-serif)}
    header,footer{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:8px 10px;border-color:var(--vsc-border-subtle,var(--vsc-border))}
    header{border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border))}header>div{display:grid}header span{color:var(--vsc-description-foreground)}
    footer{border-top:1px solid var(--vsc-border-subtle,var(--vsc-border))}
    button{font:inherit;color:inherit}.tabs{min-height:0;overflow:auto;padding:4px}
    .closed-tab{display:grid;grid-template-columns:24px minmax(0,1fr) auto;align-items:center;gap:8px;width:100%;min-height:45px;padding:5px 8px;border:1px solid transparent;border-radius:4px;background:transparent;text-align:left;cursor:pointer}
    .closed-tab:hover,.closed-tab:focus-visible{background:var(--vsc-list-hover-background);border-color:var(--vsc-border)}
    .main{display:grid;min-width:0}.main strong,.main small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.main small,.meta{color:var(--vsc-description-foreground)}.meta{font-size:10px}
    footer button,header button{min-height:24px;padding:2px 8px;border:1px solid var(--vsc-border);border-radius:3px;background:var(--vsc-button-secondary-background,var(--vsc-control-background));cursor:pointer}.empty{padding:42px;text-align:center;color:var(--vsc-description-foreground)}
  `],
})
export class ClosedTabsComponent {
  @Input() visible = false;
  @Output() closed = new EventEmitter<void>();
  @Output() reopenRequested = new EventEmitter<RecentlyClosedTab>();

  constructor(
    readonly workspaces: WorkspaceService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get entries(): readonly RecentlyClosedTab[] {
    return this.workspaces.getRecentlyClosedTabs();
  }

  close(): void {
    this.closed.emit();
  }

  reopen(entry: RecentlyClosedTab): void {
    this.reopenRequested.emit(entry);
  }

  clear(): void {
    this.workspaces.clearRecentlyClosedTabs();
    this.cdr.markForCheck();
  }

  icon(entry: RecentlyClosedTab): string {
    const kind = entry.tab.kind ?? 'dir';
    if (kind === 'viewer') return '◫';
    if (kind === 'size-manager') return '▦';
    return '▣';
  }

  viewerPath(entry: RecentlyClosedTab): string {
    return entry.tab.viewer?.targetPath ?? '';
  }

  trackEntry(_index: number, entry: RecentlyClosedTab): string {
    return entry.id;
  }
}