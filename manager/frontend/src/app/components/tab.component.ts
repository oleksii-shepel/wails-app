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
         [title]="tab.kind === 'size-manager' ? 'Storage Footprint' : tab.title"
         draggable="true"
         (click)="onClick()"
         (auxclick)="onMouseUp($event)"
         (mouseenter)="isHovered = true"
         (mouseleave)="isHovered = false">

      <svg *ngIf="tab.kind === 'viewer'" width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-label="Viewer" class="tab-icon tab-icon--viewer">
        <path d="M1.5 8s2.4-4.2 6.5-4.2S14.5 8 14.5 8s-2.4 4.2-6.5 4.2S1.5 8 1.5 8Z"/>
        <circle cx="8" cy="8" r="1.9"/>
      </svg>

      <svg *ngIf="tab.kind === 'size-manager'" width="13" height="13" viewBox="0 0 16 16" fill="currentColor" aria-label="Disk Usage" class="tab-icon tab-icon--disk-usage">
        <rect x="1.5" y="9" width="3" height="5.5" rx="0.75"/>
        <rect x="6.5" y="5" width="3" height="9.5" rx="0.75"/>
        <rect x="11.5" y="1.5" width="3" height="13" rx="0.75"/>
      </svg>

      <svg *ngIf="tab.kind !== 'viewer' && tab.kind !== 'size-manager' && !isArchiveTab" width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.45" stroke-linecap="round" stroke-linejoin="round" aria-label="Folder" class="tab-icon tab-icon--folder">
        <path d="M1.75 4.25A1.5 1.5 0 0 1 3.25 2.75h2.64a1.5 1.5 0 0 1 1.06.44l.86.86a.5.5 0 0 0 .35.15h4.59a1.5 1.5 0 0 1 1.5 1.5v6a1.5 1.5 0 0 1-1.5 1.5H3.25a1.5 1.5 0 0 1-1.5-1.5v-7.45Z"/>
      </svg>

      <svg *ngIf="tab.kind !== 'viewer' && tab.kind !== 'size-manager' && isArchiveTab" width="13" height="13" viewBox="0 0 16 16" fill="currentColor" aria-label="Archive" class="tab-icon tab-icon--archive">
        <path d="M2 1a1 1 0 0 0-1 1v1a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V2a1 1 0 0 0-1-1H2Z"/>
        <path d="M2 5v8a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V5H2Zm4 2.5h4a.5.5 0 0 1 0 1H8.5v1a.5.5 0 0 1-1 0v-1H6a.5.5 0 0 1 0-1Z"/>
      </svg>

      <span class="tab-title">{{ shortTitle }}</span>

      <div class="tab-actions">
        <!-- Pin/unpin is always mouse-accessible; pinned tabs keep their title. -->
        <button type="button" class="tab-action pin" (click)="onPin($event)"
                [class.pinned-state]="tab.isPinned"
                [class.active-action]="isActive"
                [title]="tab.isPinned ? 'Unpin tab (Ctrl+Alt+P)' : 'Pin tab (Ctrl+Alt+P)'"
                draggable="false"
                [attr.aria-label]="tab.isPinned ? 'Unpin tab' : 'Pin tab'">
          <svg class="pin-icon" width="11" height="11" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
            <path d="M9.828 1.5a1 1 0 0 1 1.414 0l3.258 3.258a1 1 0 0 1-.278 1.622l-2.32.928-2.08 2.08.353 2.828a.75.75 0 0 1-1.275.623L6.172 10.11l-3.44 3.44a.75.75 0 0 1-1.06-1.06l3.44-3.44-2.728-2.728a.75.75 0 0 1 .623-1.275l2.828.353 2.08-2.08.928-2.32a1 1 0 0 1 .985-.5Z"/>
          </svg>
        </button>
        <!-- Pinned tabs are protected from accidental close. -->
        <button type="button" *ngIf="isClosable && !tab.isPinned" class="tab-action close" (click)="onClose($event)"
                [class.active-action]="isActive" title="Close (Ctrl+W)"
                [attr.aria-label]="'Close tab ' + tab.title" draggable="false">
          <svg width="11" height="11" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true">
            <path d="M4 4l8 8M12 4l-8 8"/>
          </svg>
        </button>
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
      height: 31px;
      padding: 0 7px 0 11px;
      min-width: 112px;
      max-width: 212px;
      cursor: pointer;
      user-select: none;
      color: var(--vsc-tab-foreground, var(--vsc-description-foreground));
      background: transparent;
      border: 0;
      outline: none;
      border-radius: 7px 7px 0 0;
      transition:
        background-color 120ms cubic-bezier(0.16, 1, 0.3, 1),
        color 120ms ease;
    }

    :host:first-child .tab {
      border-top-left-radius: 0;
    }

    :host:first-child .tab.active::before {
      display: none;
    }

    .tab-actions {
      display: flex;
      align-items: center;
      gap: 2px;
      margin-left: auto;
      flex: 0 0 auto;
    }

    /* Hover */
    .tab:hover:not(.active) {
      background: color-mix(in srgb, var(--vsc-tab-hover-background, var(--vsc-list-hover-background)) 76%, transparent);
      color: var(--vsc-tab-hover-foreground, var(--vsc-foreground));
      z-index: 1;
    }

    /* Active */
    .tab.active {
      background: var(--vsc-editor-background);
      color: var(--vsc-tab-active-foreground, var(--vsc-foreground));
      z-index: 5;
    }

    .tab.active::before,
    .tab.active::after {
      content: "";
      position: absolute;
      bottom: 0;
      width: 6px;
      height: 6px;
      pointer-events: none;
    }

    .tab.active::before {
      left: -6px;
      background: radial-gradient(
        circle at 0 0,
        transparent 5.75px,
        var(--vsc-editor-background) 6.25px
      );
    }

    .tab.active::after {
      right: -6px;
      background: radial-gradient(
        circle at 100% 0,
        transparent 5.75px,
        var(--vsc-editor-background) 6.25px
      );
    }

    /* Icon */
    .tab-icon {
      opacity: .72;
      flex-shrink: 0;
      transition: opacity 120ms ease, color 120ms ease;
    }

    .tab:hover .tab-icon,
    .tab.active .tab-icon {
      opacity: .95;
    }

    .tab.active .tab-icon {
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
    }

    .tab-icon--archive,
    .tab.active .tab-icon--archive {
      color: var(--vsc-accent-orange);
      opacity: 1;
    }

    .tab-icon--disk-usage {
      opacity: .75;
    }

    /* Title */
    .tab-title {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 11.5px;
      font-weight: 500;
      line-height: 1.25;
      letter-spacing: 0.005em;
    }

    /* Action buttons */
    .tab-action {
      width: 18px;
      height: 18px;
      padding: 0;
      border: 0;
      outline: none;
      border-radius: 4px;
      background: transparent;
      color: inherit;
      opacity: 0;
      display: grid;
      place-items: center;
      cursor: pointer;
      transition: opacity 120ms ease, background-color 120ms ease, color 120ms ease;
    }

    .tab:hover .tab-action,
    .tab.active .tab-action {
      opacity: .72;
    }

    .tab-action:hover {
      opacity: 1 !important;
      background: var(--vsc-neutral-overlay-medium, var(--vsc-tab-hover-background));
    }

    /* Pin action stays discoverable and highlights when pinned */
    .tab-action.pin {
      opacity: .34;
      flex-shrink: 0;
    }

    .tab:hover .tab-action.pin,
    .tab.active .tab-action.pin {
      opacity: .68;
    }

    .tab-action.pin.pinned-state {
      opacity: .92;
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
    }

    .tab-action.pin:hover {
      opacity: 1;
    }

    .tab-action.pin:not(.pinned-state) .pin-icon {
      opacity: .78;
    }

    .tab-action.pin.pinned-state .pin-icon {
      opacity: 1;
    }

    /* Compact */
    :host-context(.compact-mode) .tab,
    :host-context(.compact) .tab {
      height: 26px;
      border-radius: 6px 6px 0 0;
      min-width: 92px;
      padding: 0 5px 0 9px;
    }

    :host-context(.compact-mode):first-child .tab,
    :host-context(.compact):first-child .tab {
      border-top-left-radius: 0;
    }

    /* Focus */
    .tab:focus-visible,
    .tab-action:focus-visible {
      outline: 1.5px solid var(--vsc-accent-blue);
      outline-offset: -1.5px;
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
    // e.g. "filesystem://c/foo/bar.zip/src" or "/home/user/backup.tar.gz/etc"
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
    if ((this.tab?.kind ?? 'dir') === 'size-manager') return 'Storage Footprint';
    const parts = t.replace(/\\/g, '/').split('/').filter(Boolean);
    if (parts.length === 0) return t;
    // Drive root: "C:" or "/"
    if (parts.length === 1) return t;
    return parts[parts.length - 1];
  }
}