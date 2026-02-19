import { CommonModule } from "@angular/common";
import { Component, EventEmitter, Input, Output } from "@angular/core";
import { TabInfo } from "../shared/protocol-enhanced";

@Component({
  selector: 'app-tab',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="tab"
         [class.active]="isActive"
         [class.pinned]="tab.isPinned"
         [class.in-archive]="inArchive"
         [title]="tab.title"
         (click)="onClick()"
         (auxclick)="onMouseUp($event)"
         (mouseenter)="isHovered = true"
         (mouseleave)="isHovered = false">

      <!-- Icon: archive zip icon when inside archive, folder otherwise -->
      <span class="tab-icon" *ngIf="!inArchive">📁</span>
      <svg *ngIf="inArchive" class="tab-icon tab-icon--archive" width="14" height="14"
           viewBox="0 0 16 16" fill="currentColor" aria-label="Archive">
        <path d="M6.5 7.5a1 1 0 0 0-1 1v.5h-1v-.5a2 2 0 0 1 4 0v.5h-1v-.5a1 1 0 0 0-1-1zM5.5 11a1 1 0 1 0 2 0v-.5h-2V11zm7-6H10V2.5a.5.5 0 0 0-.5-.5h-3a.5.5 0 0 0-.5.5V5H3.5A1.5 1.5 0 0 0 2 6.5v6A1.5 1.5 0 0 0 3.5 14h9a1.5 1.5 0 0 0 1.5-1.5v-6A1.5 1.5 0 0 0 12.5 5zM7 2h2v3H7V2zm5.5 10.5a.5.5 0 0 1-.5.5h-8a.5.5 0 0 1-.5-.5v-6a.5.5 0 0 1 .5-.5h8a.5.5 0 0 1 .5.5v6z"/>
      </svg>

      <span class="tab-title">{{ shortTitle }}</span>

      <!-- Active Tab Indicators -->
      <div class="tab-border-top" *ngIf="isActive"></div>
      <div class="tab-active-indicator"></div>
      <div class="tab-actions">
        <!-- Pinned indicator/button -->
        <button *ngIf="tab.isPinned" class="tab-action pin" (click)="onPin($event)" 
                [class.active-action]="isActive" title="Unpin tab">
          <span class="pin-icon">📌</span>
        </button>
        <!-- Close button -->
        <button class="tab-action close" (click)="onClose($event)" 
                [class.active-action]="isActive" title="Close (Ctrl+W)"></button>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      align-items: flex-end;
      height: 100%;
    }

    .tab {
      position: relative;
      display: inline-flex;
      align-items: center;
      gap: 6px;
      height: 100%;
      padding: 2px 14px 0 14px;
      min-width: 120px;
      max-width: 240px;
      cursor: pointer;
      user-select: none;
      color: var(--vsc-tab-foreground);
      background: var(--vsc-tab-background);
      border-right: 1px solid var(--vsc-tab-border);
      border-left: 1px solid transparent;
      border-top: 1px solid transparent;
      border-bottom: 1px solid transparent;
      transition:
        background var(--vsc-transition-fast),
        color var(--vsc-transition-fast);
    }

    .tab-actions {
      display: flex;
      align-items: center;
      gap: 2px;
      margin-left: auto;
      height: 100%;
    }

    /* Hover */
    .tab:hover:not(.active) {
      background: var(--vsc-tab-hover-background);
      color: var(--vsc-tab-hover-foreground);
    }

    /* Active */
    .tab.active {
      background: var(--vsc-editor-background);
      color: var(--vsc-tab-active-foreground);
      border-left-color: var(--vsc-tab-border);
      border-right-color: var(--vsc-tab-border);
      border-top-color: var(--vsc-tab-active-border-top);
      border-bottom: 1px solid transparent;
      position: relative;
      z-index: 5;
    }

    .tab.active::after {
      display: none;
    }

    /* Archive mode: amber top border */
    .tab.in-archive {
      border-top-color: #ffb347 !important;
    }

    .tab.in-archive.active {
      border-top-color: #ffb347 !important;
    }

    /* Icon */
    .tab-icon {
      font-size: 15px;
      opacity: .75;
      flex-shrink: 0;
    }

    .tab-icon--archive {
      color: #ffb347;
      opacity: 1;
    }

    /* Title */
    .tab-title {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--vsc-font-size);
    }

    /* Archive badge */
    .tab-archive-badge {
      flex-shrink: 0;
      font-size: 9px;
      font-weight: 700;
      letter-spacing: 0.4px;
      padding: 1px 4px;
      border-radius: 3px;
      background: color-mix(in srgb, #ffb347 22%, transparent);
      border: 1px solid rgba(255, 179, 71, 0.45);
      color: #ffb347;
      line-height: 14px;
    }

    /* Action buttons */
    .tab-action {
      width: 18px;
      height: 18px;
      border: none;
      background: transparent;
      color: inherit;
      opacity: 0;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .tab:hover .tab-action {
      opacity: .7;
    }

    .tab-action:hover {
      opacity: 1 !important;
      background: var(--vsc-tab-hover-background);
    }

    .tab.active .tab-action {
      opacity: .6;
    }

    /* Close icon */
    .tab-action.close::before {
      content: "✕";
      font-size: 13px;
    }

    /* Pinned */
    .tab.pinned {
      width: 42px;
      min-width: 42px;
      max-width: 42px;
      justify-content: center;
      padding: 0;
    }

    .tab.pinned .tab-title,
    .tab.pinned .close,
    .tab.pinned .tab-archive-badge {
      display: none;
    }

    /* Dirty dot */
    .dirty-dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: var(--vsc-tab-foreground);
    }

    .dirty-dot.active-dirty {
      background: var(--vsc-accent-blue);
    }

    /* Compact */
    :host-context(.compact-mode) .tab {
      height: 100%;
      padding: 0 10px;
    }

    :host-context(.compact-mode) .tab.pinned {
      width: 36px;
    }

    /* Focus */
    .tab:focus-visible {
      outline: 1px solid var(--vsc-accent-blue);
      outline-offset: -1px;
    }
  `]
})
export class TabComponent {
  @Input() tab!: TabInfo;
  @Input() isActive = false;
  /** Set to true when this tab is currently browsing inside an archive. */
  @Input() inArchive = false;

  @Output() tabClick = new EventEmitter<TabInfo>();
  @Output() tabClose = new EventEmitter<TabInfo>();
  @Output() tabPin = new EventEmitter<TabInfo>();

  isHovered = false;

  onClick(): void { this.tabClick.emit(this.tab); }

  onMouseUp(event: MouseEvent): void {
    if (event.button === 1) {
      this.onClose(event);
    }
  }

  onClose(event: Event): void {
    event.stopPropagation();
    this.tabClose.emit(this.tab);
  }

  onPin(event: Event): void {
    event.stopPropagation();
    this.tabPin.emit(this.tab);
  }
  
  get shortTitle(): string {
    const t = this.tab?.title ?? '';
    if (!t) return '';
    const parts = t.replace(/\\/g, '/').split('/').filter(Boolean);
    if (parts.length === 0) return t;
    // Drive root: "C:" or "/"
    if (parts.length === 1) return t;
    return parts[parts.length - 1];
  }
}
