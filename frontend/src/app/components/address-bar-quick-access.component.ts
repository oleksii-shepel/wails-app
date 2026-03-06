import { Component, Input, Output, EventEmitter, OnInit, OnDestroy, ChangeDetectionStrategy, HostListener, ChangeDetectorRef, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SpecialFolder } from '../shared/protocol-enhanced';
import { PathHistoryService, PathHistoryEntry } from '../services/path-history.service';
import { PinnedPath } from './address-bar.types';

export interface QuickAccessSection {
  title: string;
  items: QuickAccessItem[];
}

interface OrderedQuickAccessSection {
  section: QuickAccessSection;
  originalIndex: number;
}

type TabSpinDirection = 'left' | 'right' | null;

export interface QuickAccessItem {
  name: string;
  path: string;
  icon: string;
  accessCount?: number;
  timestamp?: number;
}

@Component({
  selector: 'app-address-bar-quick-access',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="ab-quick-access-drop" role="menu" tabindex="-1">
      <!-- Header with Tabs -->
      <div class="ab-drop-head">
        <div class="ab-qa-tabs-stage">
          <ng-container *ngIf="tabTransitionActive; else staticTabs">
            <div
              class="ab-qa-tabs ab-qa-tabs--anim ab-qa-tabs--outgoing"
              [class.ab-qa-tabs--left]="tabSpinDirection === 'left'"
              [class.ab-qa-tabs--right]="tabSpinDirection === 'right'"
            >
              <button
                *ngIf="leadingSection(animationFromSections) as activeSection"
                class="ab-qa-tab ab-qa-tab--active"
                role="tab"
                [attr.aria-selected]="true"
                type="button"
              >
                {{ activeSection.section.title }}
              </button>

              <div class="ab-qa-tab-trail">
                <div
                  *ngFor="let orderedSection of trailingSectionsFrom(animationFromSections); trackBy: trackByOrderedSection"
                  class="ab-qa-tab-trailing-item"
                >
                  <span class="ab-qa-tab-divider" aria-hidden="true"></span>
                  <button
                    class="ab-qa-tab ab-qa-tab--trailing"
                    role="tab"
                    [attr.aria-selected]="false"
                    type="button"
                  >
                    {{ orderedSection.section.title }}
                  </button>
                </div>
              </div>
            </div>

            <div
              class="ab-qa-tabs ab-qa-tabs--anim ab-qa-tabs--incoming"
              [class.ab-qa-tabs--left]="tabSpinDirection === 'left'"
              [class.ab-qa-tabs--right]="tabSpinDirection === 'right'"
            >
              <button
                *ngIf="leadingSection(animationToSections) as activeSection"
                class="ab-qa-tab ab-qa-tab--active"
                role="tab"
                [attr.aria-selected]="true"
                type="button"
              >
                {{ activeSection.section.title }}
              </button>

              <div class="ab-qa-tab-trail">
                <div
                  *ngFor="let orderedSection of trailingSectionsFrom(animationToSections); trackBy: trackByOrderedSection"
                  class="ab-qa-tab-trailing-item"
                >
                  <span class="ab-qa-tab-divider" aria-hidden="true"></span>
                  <button
                    class="ab-qa-tab ab-qa-tab--trailing"
                    role="tab"
                    [attr.aria-selected]="false"
                    type="button"
                  >
                    {{ orderedSection.section.title }}
                  </button>
                </div>
              </div>
            </div>
          </ng-container>

          <ng-template #staticTabs>
            <div class="ab-qa-tabs" role="tablist">
              <button
                *ngIf="activeSectionEntry() as activeSection"
                class="ab-qa-tab ab-qa-tab--active"
                (click)="setActiveTab(activeSection.originalIndex)"
                role="tab"
                [attr.aria-selected]="true"
                type="button"
              >
                {{ activeSection.section.title }}
              </button>

              <div class="ab-qa-tab-trail">
                <div
                  *ngFor="let orderedSection of trailingSections(); trackBy: trackByOrderedSection"
                  class="ab-qa-tab-trailing-item"
                >
                  <span class="ab-qa-tab-divider" aria-hidden="true"></span>
                  <button
                    class="ab-qa-tab ab-qa-tab--trailing"
                    (click)="setActiveTab(orderedSection.originalIndex)"
                    role="tab"
                    [attr.aria-selected]="false"
                    type="button"
                  >
                    {{ orderedSection.section.title }}
                  </button>
                </div>
              </div>
            </div>
          </ng-template>
        </div>
      </div>

      <!-- Empty State -->
      <div class="ab-drop-empty" *ngIf="sections.length === 0">
        <span class="ab-drop-empty-icon">📂</span>
        <span>No quick access items available</span>
      </div>

      <!-- Active Section Items -->
      <div class="ab-drop-scroll" *ngIf="sections.length > 0">
        <div class="qa-section" role="group">
          <ng-container *ngIf="sections[activeTabIdx] as activeSection">
            <div class="ab-drop-empty" *ngIf="activeSection.items.length === 0">
              <span>No items in this section</span>
            </div>

            <button
              *ngFor="let item of activeSection.items; let iIdx = index"
              class="ab-drop-item qa-item"
              [class.ab-drop-item--active]="item.path === currentPath"
              [class.qa-item--highlighted]="isHighlighted(iIdx)"
              role="menuitem"
              (click)="selectItem(activeTabIdx, iIdx)"
              (mouseenter)="highlightItem(activeTabIdx, iIdx)"
              [title]="item.path"
              type="button"
            >
              <span class="qa-item-icon">{{ item.icon }}</span>
              <span class="qa-item-name">{{ item.name }}</span>
              <span *ngIf="item.accessCount" class="qa-item-count">{{ item.accessCount }}×</span>
            </button>
          </ng-container>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host { display: contents; }
    .ab-quick-access-drop {
      min-width: 280px;
      max-width: 380px;
      z-index: 9999 !important;
      background-color: var(--vsc-panel-background, #252526) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      overflow: hidden;
      display: flex;
      flex-direction: column;
    }

    .ab-drop-head {
      display: flex;
      align-items: center;
      height: 36px;
      padding: 0 var(--vsc-padding-lg);
      background-color: var(--vsc-panel-background, #252526) !important;
      border-bottom: 1px solid var(--vsc-border-subtle);
      overflow: hidden;
      perspective: 900px;
    }

    .ab-qa-tabs-stage {
      position: relative;
      width: 100%;
      height: 100%;
      overflow: hidden;
    }

    .ab-qa-tabs {
      display: flex;
      justify-content: space-between;
      height: 100%;
      align-items: center;
      width: 100%;
      gap: 12px;
      transform-origin: center center;
      will-change: transform;
      transform-style: preserve-3d;
    }

    .ab-qa-tabs--anim {
      position: absolute;
      inset: 0;
      pointer-events: none;
    }

    .ab-qa-tabs--outgoing.ab-qa-tabs--left {
      animation: ab-qa-strip-out-left 260ms cubic-bezier(0.22, 1, 0.36, 1) forwards;
    }

    .ab-qa-tabs--incoming.ab-qa-tabs--left {
      animation: ab-qa-strip-in-left 260ms cubic-bezier(0.22, 1, 0.36, 1) forwards;
    }

    .ab-qa-tabs--outgoing.ab-qa-tabs--right {
      animation: ab-qa-strip-out-right 260ms cubic-bezier(0.22, 1, 0.36, 1) forwards;
    }

    .ab-qa-tabs--incoming.ab-qa-tabs--right {
      animation: ab-qa-strip-in-right 260ms cubic-bezier(0.22, 1, 0.36, 1) forwards;
    }

    .ab-qa-tab-trail {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 0;
      min-width: 0;
      margin-left: auto;
    }

    .ab-qa-tab-trailing-item {
      display: flex;
      align-items: center;
      flex: 0 0 auto;
      margin-left: 6px;
    }

    .ab-qa-tab-divider {
      display: block;
      width: 1px;
      height: 14px;
      margin-right: 10px;
      background: color-mix(in srgb, var(--vsc-foreground-dim) 60%, transparent);
      transform: rotate(22deg);
      transform-origin: center;
      opacity: 0.7;
      pointer-events: none;
      user-select: none;
      -webkit-user-select: none;
      flex: 0 0 auto;
      backface-visibility: hidden;
    }

    .ab-qa-tab {
      background: none;
      border: none;
      color: var(--vsc-foreground-dim);
      border-bottom: 2px solid transparent;
      height: 100%;
      padding: 0;
      cursor: pointer;
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      user-select: none;
      -webkit-user-select: none;
      backface-visibility: hidden;
      transition: color 0.14s ease, border-color 0.14s ease, opacity 0.14s ease, transform 0.14s ease;

      &:hover {
        color: var(--vsc-foreground);
      }

      &.active {
        color: var(--vsc-foreground);
        border-bottom-color: var(--vsc-accent-blue);
      }
    }

    .ab-qa-tab--active {
      flex: 0 0 auto;
      padding-left: 0;
      padding-right: 0;
      color: var(--vsc-foreground);
      border-bottom-color: var(--vsc-accent-blue);
      text-shadow: 0 0 10px color-mix(in srgb, var(--vsc-accent-blue) 22%, transparent);
    }

    .ab-qa-tab--trailing {
      flex: 0 0 auto;
      opacity: 0.72;
      padding-left: 0;
    }

    .ab-qa-tab--trailing:hover {
      opacity: 1;
      transform: translateY(-1px);
    }

    @keyframes ab-qa-strip-out-left {
      0% {
        transform: translateX(0) rotateY(0deg) rotateX(0deg) scale(1);
        opacity: 1;
      }
      100% {
        transform: translateX(-108%) rotateY(-18deg) rotateX(3deg) scale(0.985);
        opacity: 0.18;
      }
    }

    @keyframes ab-qa-strip-in-left {
      0% {
        transform: translateX(108%) rotateY(18deg) rotateX(-3deg) scale(0.985);
        opacity: 0.18;
      }
      100% {
        transform: translateX(0) rotateY(0deg) rotateX(0deg) scale(1);
        opacity: 1;
      }
    }

    @keyframes ab-qa-strip-out-right {
      0% {
        transform: translateX(0) rotateY(0deg) rotateX(0deg) scale(1);
        opacity: 1;
      }
      100% {
        transform: translateX(108%) rotateY(18deg) rotateX(3deg) scale(0.985);
        opacity: 0.18;
      }
    }

    @keyframes ab-qa-strip-in-right {
      0% {
        transform: translateX(-108%) rotateY(-18deg) rotateX(-3deg) scale(0.985);
        opacity: 0.18;
      }
      100% {
        transform: translateX(0) rotateY(0deg) rotateX(0deg) scale(1);
        opacity: 1;
      }
    }

    .ab-drop-scroll {
      max-height: min(300px, 50vh);
      overflow-x: hidden;
      overflow-y: auto;
      padding: 4px 0;
      background-color: var(--vsc-panel-background, #252526) !important;
      scrollbar-width: thin;
      scrollbar-color: var(--vsc-scrollbar-thumb) var(--vsc-scrollbar-track);

      &::-webkit-scrollbar {
        width: 10px;
      }

      &::-webkit-scrollbar-track {
        background: var(--vsc-scrollbar-track);
      }

      &::-webkit-scrollbar-thumb {
        background: var(--vsc-scrollbar-thumb);
        border-radius: 5px;

        &:hover {
          background: var(--vsc-scrollbar-thumb-hover);
        }
      }
    }

    /* Common Styles */
    .ab-drop-empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-xl);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      text-align: center;
      line-height: 1.5;
      background-color: var(--vsc-panel-background, #252526) !important;
    }

    .ab-drop-empty-icon {
      font-size: 28px;
      opacity: 0.4;
    }

    .qa-section {
      margin-bottom: 0;
    }

    .qa-item {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      height: 28px;
      width: 100%;
      padding: 0 var(--vsc-padding-lg);
      cursor: pointer;
      transition: background-color var(--vsc-transition-fast);
      border: none;
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family);
      background: transparent;
      text-align: left;
      border-left: 3px solid transparent;
      box-sizing: border-box;

      &:hover {
        background: var(--vsc-list-hover-background);
      }

      &.qa-item--highlighted {
        background: var(--vsc-list-hover-background);
      }

      &.ab-drop-item--active {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
        border-left-color: var(--vsc-accent-blue);
        font-weight: 500;
      }
    }

    .qa-item-icon {
      font-size: 14px;
      flex-shrink: 0;
      width: 18px;
      text-align: center;
      opacity: 0.85;
    }

    .qa-item-name {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--vsc-font-size);
    }
  `]
})
export class AddressBarQuickAccessComponent implements OnInit, OnDestroy {
  @Input() currentPath = '';
  @Input() specialFolders: SpecialFolder[] = [];
  @Input() pinnedPaths: PinnedPath[] = [];
  @Output() navigate = new EventEmitter<string>();

  sections: QuickAccessSection[] = [];
  activeTabIdx = 0;
  tabSpinDirection: TabSpinDirection = null;
  tabTransitionActive = false;
  animationFromSections: OrderedQuickAccessSection[] = [];
  animationToSections: OrderedQuickAccessSection[] = [];
  highlightedItem: { sectionIdx: number; itemIdx: number } | null = null;
  private tabSpinTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private pathHistoryService: PathHistoryService,
    private cdr: ChangeDetectorRef,
    private hostEl: ElementRef<HTMLElement>,
  ) {}

  ngOnInit(): void {
    this.buildSections();
  }

  ngOnDestroy(): void {
    if (this.tabSpinTimer) clearTimeout(this.tabSpinTimer);
  }

  // no-op if no items
  private buildSections(): void {
    this.sections = [];

    // System Folders
    if (this.specialFolders.length > 0) {
      this.sections.push({
        title: 'System',
        items: this.specialFolders.map(f => ({
          name: f.name,
          path: f.path,
          icon: f.icon,
        }))
      });
    }

    // Pinned Folders
    if (this.pinnedPaths.length > 0) {
      this.sections.push({
        title: 'Pinned',
        items: this.pinnedPaths.map(p => ({
          name: p.label,
          path: p.path,
          icon: p.icon,
        }))
      });
    }

    // Frequent Folders (top 5 by access count)
    const frequentItems = this.getFrequentFolders(5);
    if (frequentItems.length > 0) {
      this.sections.push({
        title: 'Frequent',
        items: frequentItems
      });
    }

    // Recent Folders (last 5)
    const recentItems = this.getRecentFolders(5);
    if (recentItems.length > 0) {
      this.sections.push({
        title: 'Recent',
        items: recentItems
      });
    }

    // Reset initial state
    this.activeTabIdx = 0;
    this.updateHighlightForNewTab();
  }

  setActiveTab(idx: number): void {
    if (this.activeTabIdx === idx) return;
    this.focusMenuRoot();
    const previousIdx = this.activeTabIdx;
    this.animationFromSections = this.orderedSectionsFor(previousIdx);
    this.animationToSections = this.orderedSectionsFor(idx);
    this.activeTabIdx = idx;
    this.startTabSpin(this.getSpinDirection(previousIdx, idx));
    this.updateHighlightForNewTab();
    this.cdr.markForCheck();
  }

  activeSectionEntry(): OrderedQuickAccessSection | null {
    const section = this.sections[this.activeTabIdx];
    if (!section) return null;
    return { section, originalIndex: this.activeTabIdx };
  }

  trailingSections(): OrderedQuickAccessSection[] {
    return this.trailingSectionsFrom(this.orderedSectionsFor(this.activeTabIdx));
  }

  leadingSection(sections: OrderedQuickAccessSection[]): OrderedQuickAccessSection | null {
    return sections[0] ?? null;
  }

  trailingSectionsFrom(sections: OrderedQuickAccessSection[]): OrderedQuickAccessSection[] {
    return sections.slice(1);
  }

  private orderedSectionsFor(startIdx: number): OrderedQuickAccessSection[] {
    if (this.sections.length === 0) return [];

    const ordered = this.sections.map((section, originalIndex) => ({ section, originalIndex }));
    return ordered.slice(startIdx).concat(ordered.slice(0, startIdx));
  }

  trackByOrderedSection(_index: number, orderedSection: OrderedQuickAccessSection): number {
    return orderedSection.originalIndex;
  }

  private getSpinDirection(previousIdx: number, nextIdx: number): TabSpinDirection {
    if (nextIdx === previousIdx) return null;
    const lastIdx = this.sections.length - 1;
    if (previousIdx === lastIdx && nextIdx === 0) return 'left';
    if (previousIdx === 0 && nextIdx === lastIdx) return 'right';
    return nextIdx > previousIdx ? 'left' : 'right';
  }

  private startTabSpin(direction: TabSpinDirection): void {
    if (this.tabSpinTimer) clearTimeout(this.tabSpinTimer);
    this.tabSpinDirection = direction;
    this.tabTransitionActive = true;
    this.cdr.markForCheck();
    this.focusMenuRoot();

    this.tabSpinTimer = setTimeout(() => {
      this.tabSpinDirection = null;
      this.tabTransitionActive = false;
      this.animationFromSections = [];
      this.animationToSections = [];
      this.tabSpinTimer = null;
      this.cdr.markForCheck();
      this.focusMenuRoot();
    }, 260);
  }

  private focusMenuRoot(): void {
    requestAnimationFrame(() => {
      const menu = this.hostEl.nativeElement.querySelector<HTMLElement>('.ab-quick-access-drop');
      menu?.focus({ preventScroll: true });
    });
  }

  private updateHighlightForNewTab(): void {
    if (this.sections.length > 0 && this.sections[this.activeTabIdx].items.length > 0) {
      this.highlightedItem = { sectionIdx: this.activeTabIdx, itemIdx: 0 };
    } else {
      this.highlightedItem = null;
    }
  }

  isHighlighted(itemIdx: number): boolean {
    return this.highlightedItem?.sectionIdx === this.activeTabIdx && this.highlightedItem?.itemIdx === itemIdx;
  }

  highlightItem(sectionIdx: number, itemIdx: number): void {
    this.highlightedItem = { sectionIdx, itemIdx };
  }

  selectItem(sectionIdx: number, itemIdx: number): void {
    if (sectionIdx < this.sections.length && itemIdx < this.sections[sectionIdx].items.length) {
      const item = this.sections[sectionIdx].items[itemIdx];
      this.navigate.emit(item.path);
    }
  }

  @HostListener('document:keydown', ['$event'])
  onKeyDown(event: KeyboardEvent): void {
    // Tab switching logic
    if (event.key === 'Tab') {
      event.preventDefault();
      event.stopPropagation();
      const len = this.sections.length;
      if (len <= 1) return;

      const delta = event.shiftKey ? -1 : 1;
      const next = (this.activeTabIdx + delta + len) % len;
      this.setActiveTab(next);
      return;
    }

    if (!this.highlightedItem) return;

    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        this.moveDown();
        break;
      case 'ArrowUp':
        event.preventDefault();
        this.moveUp();
        break;
      case 'Enter':
        event.preventDefault();
        this.selectItem(this.highlightedItem.sectionIdx, this.highlightedItem.itemIdx);
        break;
    }
  }

  private moveDown(): void {
    if (!this.highlightedItem) return;
    const { sectionIdx, itemIdx } = this.highlightedItem;
    const currentSection = this.sections[sectionIdx];

    if (itemIdx < currentSection.items.length - 1) {
      this.highlightedItem = { sectionIdx, itemIdx: itemIdx + 1 };
    }
    // No wrapping to next section; just stop at bottom of list
  }

  private moveUp(): void {
    if (!this.highlightedItem) return;
    const { sectionIdx, itemIdx } = this.highlightedItem;

    if (itemIdx > 0) {
      this.highlightedItem = { sectionIdx, itemIdx: itemIdx - 1 };
    }
    // No wrapping to prev section; just stop at top of list
  }

  private getFrequentFolders(limit: number): QuickAccessItem[] {
    const history = this.pathHistoryService.getAllHistory();
    return history
      .filter(e => e.accessCount > 1)
      .sort((a, b) => b.accessCount - a.accessCount)
      .slice(0, limit)
      .map(e => ({
        name: this.getDisplayName(e.path),
        path: e.path,
        icon: '📁',
        accessCount: e.accessCount,
      }));
  }

  private getRecentFolders(limit: number): QuickAccessItem[] {
    const history = this.pathHistoryService.getAllHistory();
    return history
      .sort((a, b) => b.timestamp - a.timestamp)
      .slice(0, limit)
      .map(e => ({
        name: this.getDisplayName(e.path),
        path: e.path,
        icon: '🕒',
        timestamp: e.timestamp,
      }));
  }

  private getDisplayName(path: string): string {
    if (path === '/' || path === '') return 'Root';
    const parts = path.split('/').filter(p => p.length > 0);
    return parts.length > 0 ? parts[parts.length - 1] : 'Root';
  }
}
