import { Component, Input, Output, EventEmitter, OnInit, OnDestroy, OnChanges, SimpleChanges, ChangeDetectionStrategy, ChangeDetectorRef, ElementRef, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { SpecialFolder } from '../shared/protocol';
import { PathHistoryService } from '../services/path-history.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { PinnedPath } from './address-bar.types';

export interface QuickAccessSection {
  title: string;
  items: QuickAccessItem[];
}

interface OrderedQuickAccessSection {
  section: QuickAccessSection;
  originalIndex: number;
}

interface RankedQuickAccessItem {
  item: QuickAccessItem;
  score: number;
  originalIndex: number;
}

type TabSpinDirection = 'left' | 'right' | null;

export interface QuickAccessItem {
  name: string;
  path: string;
  icon: string;
  accessCount?: number;
  timestamp?: number;
  pin?: PinnedPath;
}

@Component({
  selector: 'app-address-bar-quick-access',
  standalone: true,
  imports: [CommonModule, FormsModule],
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

      <div class="ab-qa-filter-row" *ngIf="allSections.length > 0">
        <span class="ab-qa-filter-icon" aria-hidden="true">⌕</span>
        <input
          #filterInput
          type="text"
          class="ab-qa-filter-input"
          [(ngModel)]="filterQuery"
          (ngModelChange)="onFilterChange($event)"
          placeholder="Filter quick access"
          aria-label="Filter quick access items"
        />
        <button
          *ngIf="filterQuery"
          class="ab-qa-filter-clear"
          type="button"
          (click)="clearFilter()"
          title="Clear filter"
          aria-label="Clear filter"
        >✕</button>
      </div>

      <!-- Empty State -->
      <div class="ab-drop-empty" *ngIf="sections.length === 0">
        <span class="ab-drop-empty-icon">{{ filterQuery ? '🔎' : '📂' }}</span>
        <span>{{ filterQuery ? 'No quick access items match this filter' : 'No quick access items available' }}</span>
      </div>

      <!-- Active Section Items -->
      <div class="ab-drop-scroll" *ngIf="sections.length > 0">
        <div class="qa-section" role="group">
          <ng-container *ngIf="sections[activeTabIdx] as activeSection">
            <div class="ab-drop-empty" *ngIf="activeSection.items.length === 0">
              <ng-container *ngIf="activeSection.title === 'Pinned'; else genericEmptyState">
                <span class="ab-drop-empty-icon">📍</span>
                <span>No pinned paths yet.<br>Click ☆ to pin the current path.</span>
              </ng-container>
              <ng-template #genericEmptyState>
                <span>No items in this section</span>
              </ng-template>
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
              <button
                *ngIf="item.pin"
                class="qa-item-remove"
                (click)="removePinnedItem(item, $event)"
                title="Remove pin"
                aria-label="Remove pin"
                type="button"
              >✕</button>
            </button>
          </ng-container>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host { display: contents; }
    .ab-quick-access-drop {
      min-width: 360px;
      max-width: 520px;
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

    .ab-qa-filter-row {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: 8px var(--vsc-padding-lg);
      border-bottom: 1px solid var(--vsc-border-subtle);
      background: color-mix(in srgb, var(--vsc-panel-background, #252526) 88%, var(--vsc-sidebar-background) 12%);
    }

    .ab-qa-filter-icon {
      flex: 0 0 auto;
      color: var(--vsc-foreground-dim);
      font-size: 12px;
    }

    .ab-qa-filter-input {
      flex: 1 1 auto;
      min-width: 0;
      height: 26px;
      padding: 0 var(--vsc-padding-sm);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm, 2px);
      background: var(--vsc-input-background);
      color: var(--vsc-foreground);
      font: inherit;

      &:focus {
        outline: 1px solid var(--vsc-input-focus-border, var(--vsc-accent-blue));
        border-color: var(--vsc-input-focus-border, var(--vsc-accent-blue));
      }
    }

    .ab-qa-filter-clear {
      flex: 0 0 auto;
      width: 22px;
      height: 22px;
      padding: 0;
      border: none;
      border-radius: var(--vsc-border-radius-sm, 2px);
      background: transparent;
      color: var(--vsc-foreground-dim);
      cursor: pointer;

      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground);
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

    .qa-item-remove {
      flex: 0 0 auto;
      width: 20px;
      height: 20px;
      padding: 0;
      border: none;
      border-radius: var(--vsc-border-radius-sm, 2px);
      background: transparent;
      color: transparent;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: background-color var(--vsc-transition-fast), color var(--vsc-transition-fast);

      .qa-item:hover &,
      .qa-item--highlighted &,
      .ab-drop-item--active & {
        color: var(--vsc-foreground-dim);
      }

      &:hover {
        background: var(--vsc-error, #f48771);
        color: var(--vsc-foreground-bright, #ffffff);
      }
    }
  `]
})
export class AddressBarQuickAccessComponent implements OnInit, OnDestroy, OnChanges {
  @ViewChild('filterInput') filterInput?: ElementRef<HTMLInputElement>;
  @Input() currentPath = '';
  @Input() specialFolders: SpecialFolder[] = [];
  @Input() pinnedPaths: PinnedPath[] = [];
  @Input() preferredSectionTitle: string | null = null;
  @Output() navigate = new EventEmitter<string>();
  @Output() removePin = new EventEmitter<PinnedPath>();
  @Output() closeRequested = new EventEmitter<void>();

  allSections: QuickAccessSection[] = [];
  sections: QuickAccessSection[] = [];
  activeTabIdx = 0;
  filterQuery = '';
  tabSpinDirection: TabSpinDirection = null;
  tabTransitionActive = false;
  animationFromSections: OrderedQuickAccessSection[] = [];
  animationToSections: OrderedQuickAccessSection[] = [];
  highlightedItem: { sectionIdx: number; itemIdx: number } | null = null;
  private tabSpinTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly commandOwner = `overlay.quickAccess:${Math.random().toString(36).slice(2, 8)}`;
  private releaseOverlayContext: (() => void) | null = null;

  constructor(
    private pathHistoryService: PathHistoryService,
    private cdr: ChangeDetectorRef,
    private hostEl: ElementRef<HTMLElement>,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
  ) {}

  ngOnInit(): void {
    this.registerInputCommands();
    this.releaseOverlayContext ??= this.inputContexts.acquire('overlay.quickAccess');
    this.buildSections();
    this.focusFilterField();
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['specialFolders'] || changes['pinnedPaths']) {
      this.buildSections();
    }
    if (changes['preferredSectionTitle'] && !changes['preferredSectionTitle'].firstChange) {
      this.applyPreferredSection();
    }
  }

  ngOnDestroy(): void {
    if (this.tabSpinTimer) clearTimeout(this.tabSpinTimer);
    this.releaseOverlayContext?.();
    this.releaseOverlayContext = null;
    this.unregisterInputCommands();
  }

  // no-op if no items
  private buildSections(): void {
    const previousSectionTitle = this.sections[this.activeTabIdx]?.title ?? this.preferredSectionTitle;
    this.allSections = [];

    // System Folders
    if (this.specialFolders.length > 0) {
      this.allSections.push({
        title: 'System',
        items: this.specialFolders.map(f => ({
          name: f.name,
          path: f.path,
          icon: f.icon,
        }))
      });
    }

    // Pinned Folders
    this.allSections.push({
      title: 'Pinned',
      items: this.pinnedPaths.map(p => ({
        name: p.label,
        path: p.path,
        icon: p.icon,
        pin: p,
      }))
    });

    // Frequent Folders (top 5 by access count)
    const frequentItems = this.getFrequentFolders(5);
    if (frequentItems.length > 0) {
      this.allSections.push({
        title: 'Frequent',
        items: frequentItems
      });
    }

    // Recent Folders (last 5)
    const recentItems = this.getRecentFolders(5);
    if (recentItems.length > 0) {
      this.allSections.push({
        title: 'Recent',
        items: recentItems
      });
    }

    this.applyFilterQuery(this.filterQuery, previousSectionTitle ?? null);
  }

  private applyPreferredSection(): void {
    const preferred = (this.preferredSectionTitle || '').trim().toLowerCase();
    if (!preferred || this.sections.length === 0) return;

    const preferredIdx = this.sections.findIndex(section => section.title.toLowerCase() === preferred);
    if (preferredIdx >= 0) {
      this.activeTabIdx = preferredIdx;
      this.updateHighlightForNewTab();
      this.cdr.markForCheck();
    }
  }

  setActiveTab(idx: number): void {
    if (idx < 0 || idx >= this.sections.length) return;
    if (this.activeTabIdx === idx) return;
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

  private getSpinDirection(_previousIdx: number, _nextIdx: number): TabSpinDirection {
    return 'left';
  }

  private startTabSpin(direction: TabSpinDirection): void {
    if (this.tabSpinTimer) clearTimeout(this.tabSpinTimer);
    this.tabSpinDirection = direction;
    this.tabTransitionActive = true;
    this.cdr.markForCheck();
    this.focusFilterField();

    this.tabSpinTimer = setTimeout(() => {
      this.tabSpinDirection = null;
      this.tabTransitionActive = false;
      this.animationFromSections = [];
      this.animationToSections = [];
      this.tabSpinTimer = null;
      this.cdr.markForCheck();
      this.focusFilterField();
    }, 260);
  }

  onFilterChange(query: string): void {
    this.applyFilterQuery(query, this.sections[this.activeTabIdx]?.title ?? null);
  }

  clearFilter(): void {
    this.applyFilterQuery('', this.sections[this.activeTabIdx]?.title ?? null);
    this.focusFilterField();
  }

  private focusFilterField(): void {
    requestAnimationFrame(() => {
      const input = this.filterInput?.nativeElement;
      if (input) {
        input.focus({ preventScroll: true });
        input.select();
        return;
      }

      const menu = this.hostEl.nativeElement.querySelector<HTMLElement>('.ab-quick-access-drop');
      menu?.focus({ preventScroll: true });
    });
  }

  private applyFilterQuery(query: string, preferredTitle: string | null): void {
    this.filterQuery = query;
    const normalized = query.trim().toLowerCase();
    const nextSections = normalized
      ? this.allSections
          .map(section => ({
            title: section.title,
            items: this.rankItemsForQuery(section.items, normalized),
          }))
          .filter(section => section.items.length > 0)
      : this.allSections.map(section => ({ title: section.title, items: [...section.items] }));

    this.sections = nextSections;

    if (this.sections.length === 0) {
      this.activeTabIdx = 0;
      this.highlightedItem = null;
      this.cdr.markForCheck();
      return;
    }

    const preferred = (preferredTitle || this.preferredSectionTitle || '').trim().toLowerCase();
    const preferredIdx = preferred
      ? this.sections.findIndex(section => section.title.toLowerCase() === preferred)
      : -1;

    if (preferredIdx >= 0) {
      this.activeTabIdx = preferredIdx;
    } else if (this.activeTabIdx >= this.sections.length) {
      this.activeTabIdx = this.sections.length - 1;
    }

    this.updateHighlightForNewTab();
    this.cdr.markForCheck();
  }

  private rankItemsForQuery(items: QuickAccessItem[], query: string): QuickAccessItem[] {
    const ranked = items
      .map((item, originalIndex): RankedQuickAccessItem => ({
        item,
        score: this.rankItemForQuery(item, query),
        originalIndex,
      }))
      .filter(entry => entry.score > 0)
      .sort((left, right) => right.score - left.score || left.originalIndex - right.originalIndex);

    return ranked.map(entry => entry.item);
  }

  private rankItemForQuery(item: QuickAccessItem, query: string): number {
    const tokens = query.split(/\s+/).filter(Boolean);
    if (tokens.length === 0) {
      return 1;
    }

    const fields = [
      { text: item.name, weight: 8 },
      { text: item.pin?.label, weight: 7 },
      { text: item.path, weight: 4 },
      { text: item.pin?.path, weight: 3 },
    ];

    let total = 0;
    for (const token of tokens) {
      let bestScore = 0;
      for (const field of fields) {
        bestScore = Math.max(bestScore, this.fuzzyFieldScore(field.text, token, field.weight));
      }

      if (bestScore <= 0) {
        return 0;
      }

      total += bestScore;
    }

    total += Math.min(item.accessCount ?? 0, 25);
    if (item.pin) {
      total += 10;
    }

    return total;
  }

  private fuzzyFieldScore(value: string | undefined, query: string, weight: number): number {
    if (!value) {
      return 0;
    }

    const text = value.toLowerCase();
    if (text === query) {
      return 320 * weight;
    }

    if (text.startsWith(query)) {
      return (220 - Math.min(text.length - query.length, 40)) * weight;
    }

    const containsIndex = text.indexOf(query);
    if (containsIndex >= 0) {
      const boundaryBonus = containsIndex === 0 || this.isBoundaryChar(text[containsIndex - 1]) ? 18 : 0;
      return (150 - Math.min(containsIndex, 40) + boundaryBonus) * weight;
    }

    let queryIndex = 0;
    let score = 0;
    let contiguousRun = 0;

    for (let textIndex = 0; textIndex < text.length && queryIndex < query.length; textIndex++) {
      if (text[textIndex] !== query[queryIndex]) {
        contiguousRun = 0;
        continue;
      }

      const atBoundary = textIndex === 0 || this.isBoundaryChar(text[textIndex - 1]);
      contiguousRun += 1;
      score += 14;
      score += contiguousRun * 10;
      if (atBoundary) {
        score += 18;
      }
      if (textIndex === queryIndex) {
        score += 6;
      }

      queryIndex += 1;
    }

    if (queryIndex !== query.length) {
      return 0;
    }

    return score * weight;
  }

  private isBoundaryChar(char: string | undefined): boolean {
    return !char || char === '/' || char === '\\' || char === '-' || char === '_' || char === ' ' || char === '.';
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

  private selectActiveSlot(slotIndex: number): void {
    const activeSection = this.sections[this.activeTabIdx];
    const item = activeSection?.items[slotIndex];
    if (!item) return;
    this.highlightedItem = { sectionIdx: this.activeTabIdx, itemIdx: slotIndex };
    this.selectItem(this.activeTabIdx, slotIndex);
  }

  removePinnedItem(item: QuickAccessItem, event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    if (item.pin) this.removePin.emit(item.pin);
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

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('overlay.quickAccess.close', () => {
      this.closeRequested.emit();
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.quickAccess.nextSection', () => {
      const len = this.sections.length;
      if (len <= 1) return;
      const next = (this.activeTabIdx + 1) % len;
      this.setActiveTab(next);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.quickAccess.prevSection', () => {
      const len = this.sections.length;
      if (len <= 1) return;
      const next = (this.activeTabIdx - 1 + len) % len;
      this.setActiveTab(next);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.quickAccess.prevItem', () => {
      this.moveUp();
      this.cdr.markForCheck();
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.quickAccess.nextItem', () => {
      this.moveDown();
      this.cdr.markForCheck();
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.quickAccess.openSelected', () => {
      if (!this.highlightedItem) return;
      this.selectItem(this.highlightedItem.sectionIdx, this.highlightedItem.itemIdx);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.quickAccess.focusFilter', () => {
      this.focusFilterField();
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.quickAccess.removeSelected', () => {
      if (!this.highlightedItem) return;
      const activeSection = this.sections[this.highlightedItem.sectionIdx];
      const item = activeSection?.items[this.highlightedItem.itemIdx];
      if (!item?.pin) return;
      this.removePin.emit(item.pin);
    }, this.commandOwner);

    for (let slot = 1; slot <= 9; slot++) {
      this.inputEngine.registerCommand(`overlay.quickAccess.openSlot${slot}`, () => {
        this.selectActiveSlot(slot - 1);
      }, this.commandOwner);
    }
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('overlay.quickAccess.close', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.quickAccess.nextSection', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.quickAccess.prevSection', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.quickAccess.prevItem', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.quickAccess.nextItem', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.quickAccess.openSelected', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.quickAccess.focusFilter', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.quickAccess.removeSelected', this.commandOwner);
    for (let slot = 1; slot <= 9; slot++) {
      this.inputEngine.unregisterCommand(`overlay.quickAccess.openSlot${slot}`, this.commandOwner);
    }
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
