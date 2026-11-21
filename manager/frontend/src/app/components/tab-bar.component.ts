import { Component, Input, Output, EventEmitter, ElementRef, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TabComponent } from './tab.component';
import { TabInfo } from '../shared/protocol-enhanced';

export type PaneId = 'left' | 'right';

export interface TabDropEvent {
  tabId: string;
  fromPaneId: PaneId;
  toPaneId: PaneId;
  toIndex: number;
}

interface TabDragPayload {
  source?: 'tab';
  tabId: string;
  fromPaneId: PaneId;
}

@Component({
  selector: 'app-tab-bar',
  standalone: true,
  imports: [CommonModule, TabComponent],
  template: `
    <div class="tab-bar-container" [class.compact]="isCompact" [class.in-archive]="inArchive">
      <div class="tabs-scroll-viewport"
           #scrollViewport
           (dragover)="onDragOver($event)"
           (dragleave)="onDragLeave($event)"
           (drop)="onDrop($event)">
        <div class="tabs-list" #tabsList>
          <app-tab *ngFor="let tab of tabs; trackBy: trackByTabId"
                   [tab]="tab"
                   [isActive]="tab.id === activeTabId"
                   [attr.draggable]="true"
                   (dragstart)="onDragStart($event, tab)"
                   (dragend)="onDragEnd($event)"
                   (tabClick)="onTabClick($event)"
                   (tabClose)="onTabClose($event)"
                   (tabPin)="onTabPin($event)">
          </app-tab>

          <!-- Drop indicator (VS Code style) -->
          <div class="tab-drop-indicator" *ngIf="dropIndicatorX !== null" [style.left.px]="dropIndicatorX"></div>
          
          <!-- New Tab Button -->
          <button type="button" class="new-tab-btn" (click)="onNewTab()" [class.compact]="isCompact" title="New Tab (Ctrl+N)" aria-label="New tab">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" focusable="false">
              <path d="M8 2a.75.75 0 0 1 .75.75v4.5h4.5a.75.75 0 0 1 0 1.5h-4.5v4.5a.75.75 0 0 1-1.5 0v-4.5h-4.5a.75.75 0 0 1 0-1.5h4.5v-4.5A.75.75 0 0 1 8 2z"/>
            </svg>
          </button>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      background: var(--vsc-tabbar-background);
      border-bottom: 1px solid var(--vsc-tab-border);
      overflow: hidden;
      height: 35px;
    }

    .tab-bar-container {
      display: flex;
      height: 100%;
      align-items: flex-end;
      background: var(--vsc-tabbar-background);
    }

    .tab-bar-container.compact {
      height: 100%;
      min-height: 0;
    }

    /* Amber bottom border when browsing inside an archive */
    .tab-bar-container.in-archive {
      border-bottom-color: var(--vsc-warning-border);
    }

    .tabs-scroll-viewport {
      flex: 1;
      display: flex;
      overflow-x: auto;
      height: 100%;
      scrollbar-width: none;
      -ms-overflow-style: none;
    }

    .tabs-scroll-viewport::-webkit-scrollbar {
      width: 0;
      height: 0;
      display: none;
    }

    .tabs-list {
      display: flex;
      align-items: stretch;
      height: 100%;
      min-height: 0;
      position: relative;
    }

    ::ng-deep app-tab {
      display: flex;
      align-items: stretch;
      height: 100%;
    }

    .tab-drop-indicator {
      position: absolute;
      top: 0;
      bottom: 0;
      width: 2px;
      background: color-mix(in srgb, var(--vsc-foreground) 30%, transparent);
      border-radius: 1px;
      pointer-events: none;
      z-index: 10;
    }

    /* Fade overflow */
    .tabs-scroll-viewport::after {
      content: "";
      position: sticky;
      right: 0;
      width: 24px;
      height: 100%;
      pointer-events: none;
      background: linear-gradient(
        to right,
        transparent,
        var(--vsc-tabbar-background)
      );
      z-index: 3;
    }

    /* New Tab Button */
    .new-tab-btn {
      width: 30px;
      height: 30px;
      margin: auto 4px 0 6px;
      border: none;
      background: transparent;
      color: var(--vsc-tab-foreground);
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .new-tab-btn:hover {
      background: var(--vsc-tab-hover-background);
      color: var(--vsc-tab-hover-foreground);
    }

    .new-tab-btn.compact {
      width: 26px;
      height: 26px;
    }
  `]
})
export class TabBarComponent {
  @ViewChild('scrollViewport', { static: true }) scrollViewport?: ElementRef<HTMLElement>;
  @ViewChild('tabsList', { static: true }) tabsList?: ElementRef<HTMLElement>;

  /** The pane that owns this tab bar. Required for cross-pane drag/drop. */
  @Input() paneId: PaneId = 'left';
  @Input() tabs: TabInfo[] = [];
  @Input() activeTabId = '';
  @Input() isCompact = false;
  /** Forwarded from pane host — true when browsing inside an archive. */
  @Input() inArchive = false;

  @Output() tabChange = new EventEmitter<TabInfo>();
  @Output() tabClose = new EventEmitter<TabInfo>();
  @Output() tabPin = new EventEmitter<TabInfo>();
  @Output() newTab = new EventEmitter<void>();
  @Output() tabDragStart = new EventEmitter<TabInfo>();

  /** Fired when a tab is dropped into this tab bar (reorder or cross-pane move). */
  @Output() tabDrop = new EventEmitter<TabDropEvent>();

  dropIndicatorX: number | null = null;
  private dropIndex: number | null = null;

  trackByTabId(index: number, tab: TabInfo): string {
    return tab.id;
  }

  onTabClick(tab: TabInfo) { this.tabChange.emit(tab); }
  onTabClose(tab: TabInfo) { this.tabClose.emit(tab); }
  onTabPin(tab: TabInfo) { this.tabPin.emit(tab); }
  onNewTab() { this.newTab.emit(); }

  onDragStart(event: DragEvent, tab: TabInfo): void {
    // Avoid starting a drag when interacting with tab action buttons.
    const target = event.target as HTMLElement | null;
    if (target?.closest('.tab-actions')) {
      event.preventDefault();
      return;
    }

    if (!event.dataTransfer) return;

    this.tabDragStart.emit(tab);
    event.dataTransfer.effectAllowed = 'move';

    const payload: TabDragPayload = { source: 'tab', tabId: tab.id, fromPaneId: this.paneId };
    // IMPORTANT: Some embedded engines (notably WebView2) can be quirky with
    // custom MIME types and/or hiding getData() during dragover.
    // Mirror the internal app DnD convention used by PaneListComponent.
    const payloadStr = JSON.stringify(payload);
    try { event.dataTransfer.setData('application/json', payloadStr); } catch {}
    try { event.dataTransfer.setData('application/x-wails-tab', payloadStr); } catch {}
    try { event.dataTransfer.setData('text/plain', payloadStr); } catch {}

    this.dropIndicatorX = null;
    this.dropIndex = null;
  }

  onDragEnd(_event: DragEvent): void {
    this.dropIndicatorX = null;
    this.dropIndex = null;
  }

  onDragOver(event: DragEvent): void {
    if (!this.isSupportedTabDrag(event)) return;

    // Prevent default to allow drop; otherwise cursor shows "not allowed".
    event.preventDefault();
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';

    const computed = this.computeDropPosition(event.clientX);
    if (!computed) {
      this.dropIndicatorX = null;
      this.dropIndex = null;
      return;
    }
    this.dropIndicatorX = computed.indicatorX;
    this.dropIndex = computed.index;
  }

  onDragLeave(event: DragEvent): void {
    const related = (event.relatedTarget as HTMLElement | null);
    const viewport = this.scrollViewport?.nativeElement;
    if (!viewport) return;
    if (related && viewport.contains(related)) return;
    this.dropIndicatorX = null;
    this.dropIndex = null;
  }

  onDrop(event: DragEvent): void {
    if (this.isSupportedTabDrag(event)) {
      event.preventDefault();
    }

    const payload = this.readPayload(event);
    if (!payload) {
      this.dropIndicatorX = null;
      this.dropIndex = null;
      return;
    }

    const toIndex = this.dropIndex ?? this.tabs.length;
    this.tabDrop.emit({
      tabId: payload.tabId,
      fromPaneId: payload.fromPaneId,
      toPaneId: this.paneId,
      toIndex,
    });

    this.dropIndicatorX = null;
    this.dropIndex = null;
  }

  private readPayload(event: DragEvent): TabDragPayload | null {
    const dt = event.dataTransfer;
    if (!dt) return null;
    const raw = dt.getData('application/json') || dt.getData('application/x-wails-tab') || dt.getData('text/plain');
    if (!raw) return null;
    try {
      const parsed = JSON.parse(raw) as TabDragPayload;
      if (parsed?.source && parsed.source !== 'tab') return null;
      if (!parsed?.tabId) return null;
      if (parsed.fromPaneId !== 'left' && parsed.fromPaneId !== 'right') return null;
      return parsed;
    } catch {
      return null;
    }
  }

  private isSupportedTabDrag(event: DragEvent): boolean {
    const dt = event.dataTransfer;
    if (!dt) return false;
    const types = Array.from(dt.types ?? []);
    // Only accept our internal drags; avoid interfering with external OS drags.
    if (types.length === 0) {
      return dt.effectAllowed === 'move' || dt.effectAllowed === 'copyMove';
    }
    return types.includes('application/json') || types.includes('application/x-wails-tab');
  }

  private computeDropPosition(clientX: number): { index: number; indicatorX: number } | null {
    const viewport = this.scrollViewport?.nativeElement;
    const listEl = this.tabsList?.nativeElement;
    if (!viewport || !listEl) return null;

    const tabEls = Array.from(listEl.querySelectorAll('app-tab')) as HTMLElement[];
    const listRect = listEl.getBoundingClientRect();

    if (tabEls.length === 0) {
      const indicatorX = (clientX - listRect.left) + viewport.scrollLeft;
      return { index: 0, indicatorX: Math.max(0, indicatorX) };
    }

    let index = tabEls.length;
    let indicatorClientX = tabEls[tabEls.length - 1].getBoundingClientRect().right;

    for (let i = 0; i < tabEls.length; i++) {
      const r = tabEls[i].getBoundingClientRect();
      const mid = r.left + r.width / 2;
      if (clientX < mid) {
        index = i;
        indicatorClientX = r.left;
        break;
      }
    }

    const indicatorX = (indicatorClientX - listRect.left) + viewport.scrollLeft;
    return { index, indicatorX: Math.max(0, indicatorX) };
  }
}
