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
         [class.in-archive]="isArchiveTab"
         [title]="tab.title"
          draggable="true"
         (click)="onClick()"
         (auxclick)="onMouseUp($event)"
         (mouseenter)="isHovered = true"
         (mouseleave)="isHovered = false">

      <!-- Icon: per-tab — archive icon if this tab's path is inside an archive, folder otherwise -->
      <svg *ngIf="tab.kind === 'viewer'" width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-label="Viewer" class="tab-icon tab-icon--viewer">
        <path d="M8 3c-4.2 0-7 5-7 5s2.8 5 7 5 7-5 7-5-2.8-5-7-5Zm0 8.2A3.2 3.2 0 1 1 8 4.8a3.2 3.2 0 0 1 0 6.4Zm0-1.8A1.4 1.4 0 1 0 8 6.6a1.4 1.4 0 0 0 0 2.8Z"/>
      </svg>

      <svg *ngIf="tab.kind === 'size-manager'" width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-label="Disk Usage" class="tab-icon tab-icon--disk-usage">
        <rect x="1" y="9" width="3" height="5" rx="0.5"/>
        <rect x="6" y="5" width="3" height="9" rx="0.5"/>
        <rect x="11" y="1" width="3" height="13" rx="0.5"/>
      </svg>

      <svg *ngIf="tab.kind !== 'viewer' && tab.kind !== 'size-manager' && !isArchiveTab" width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-label="Folder" class="tab-icon tab-icon--folder">
        <path d="M1 3.5A1.5 1.5 0 0 1 2.5 2h3.086a1.5 1.5 0 0 1 1.06.44l.915.914A.5.5 0 0 0 7.914 3.5H13.5A1.5 1.5 0 0 1 15 5v7.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 1 12.5v-9Z"/>
      </svg>

      <svg *ngIf="tab.kind !== 'viewer' && tab.kind !== 'size-manager' && isArchiveTab" width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-label="Archive" class="tab-icon tab-icon--archive">
        <path d="M2 1a1 1 0 0 0-1 1v1a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V2a1 1 0 0 0-1-1H2Z"/>
        <path d="M2 5v8a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V5H2Zm4 2.5h4a.5.5 0 0 1 0 1H8.5v1a.5.5 0 0 1-1 0v-1H6a.5.5 0 0 1 0-1Z"/>
      </svg>
      
      <span class="tab-title">{{ shortTitle }}</span>

      <!-- Active Tab Indicators -->
      <div class="tab-border-top" *ngIf="isActive"></div>
      <div class="tab-active-indicator"></div>
      <div class="tab-actions">
        <!-- Pinned indicator/button -->
        <button *ngIf="tab.isPinned" class="tab-action pin" (click)="onPin($event)" 
                [class.active-action]="isActive" title="Unpin tab" draggable="false">
          <span class="pin-icon">📌</span>
        </button>
        <!-- Close button -->
        <button *ngIf="isClosable" class="tab-action close" (click)="onClose($event)"
                [class.active-action]="isActive" title="Close (Ctrl+W)" draggable="false"></button>
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
      transition: opacity 120ms ease, filter 120ms ease;
    }

    .tab-icon--archive {
      color: #ffb347;
      opacity: 1;
    }

    .tab-icon--disk-usage {
      color: #4ec9b0;
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

    /* Soften the tab icon when the tab is active to reduce perceived brightness */
    .tab.active .tab-icon {
      opacity: 0.82;
      filter: saturate(0.94) brightness(0.95);
    }
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
  /** True when this specific tab's saved path is inside an archive virtual path.
   *  Derived from tab.path so each tab carries its own icon independently. */
  get isArchiveTab(): boolean {
    const p = this.tab?.path ?? '';
    // Archive virtual paths contain a known archive extension followed by a slash.
    // e.g. "C:/foo/bar.zip/src" or "/home/user/backup.tar.gz/etc"
    return /\.(zip|tar|gz|bz2|xz|7z|rar|iso|dmg|tgz|tbz2)([\/\\]|$)/i.test(p);
  }

  @Output() tabClick = new EventEmitter<TabInfo>();
  @Output() tabClose = new EventEmitter<TabInfo>();
  @Output() tabPin = new EventEmitter<TabInfo>();

  isHovered = false;

  get isClosable(): boolean {
    return this.tab?.isClosable !== false;
  }

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
    if ((this.tab?.kind ?? 'dir') === 'size-manager') return t;
    const parts = t.replace(/\\/g, '/').split('/').filter(Boolean);
    if (parts.length === 0) return t;
    // Drive root: "C:" or "/"
    if (parts.length === 1) return t;
    return parts[parts.length - 1];
  }
}
