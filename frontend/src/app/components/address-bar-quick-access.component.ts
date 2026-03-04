import { Component, Input, Output, EventEmitter, OnInit, ChangeDetectionStrategy, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SpecialFolder } from '../shared/protocol-enhanced';
import { PathHistoryService, PathHistoryEntry } from '../services/path-history.service';
import { PinnedPath } from './address-bar.types';

export interface QuickAccessSection {
  title: string;
  items: QuickAccessItem[];
}

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
    <div class="ab-quick-access-drop" role="menu">
      <!-- Header -->
      <div class="ab-drop-head">
        <span>Quick Access</span>
        <span class="ab-qa-hint">Ctrl+Q</span>
      </div>

      <!-- Empty State -->
      <div class="ab-drop-empty" *ngIf="sections.length === 0">
        <span class="ab-drop-empty-icon">📂</span>
        <span>No quick access items available</span>
      </div>

      <!-- Sections -->
      <div class="ab-drop-scroll" *ngIf="sections.length > 0" (keydown)="onKeyDown($event)">
        <div *ngFor="let section of sections; let sIdx = index" class="qa-section" role="group" [attr.aria-label]="section.title">
          <!-- Section Header -->
          <div class="qa-section-title" *ngIf="section.items.length > 0">
            {{ section.title }}
            <span class="qa-section-count">{{ section.items.length }}</span>
          </div>

          <!-- Section Items -->
          <button
            *ngFor="let item of section.items; let iIdx = index"
            class="ab-drop-item qa-item"
            [class.ab-drop-item--active]="item.path === currentPath"
            [class.qa-item--highlighted]="isHighlighted(sIdx, iIdx)"
            role="menuitem"
            (click)="selectItem(sIdx, iIdx)"
            (mouseenter)="highlightItem(sIdx, iIdx)"
            [title]="item.path"
            type="button"
          >
            <span class="qa-item-icon">{{ item.icon }}</span>
            <span class="qa-item-name">{{ item.name }}</span>
            <span *ngIf="item.accessCount" class="qa-item-count">{{ item.accessCount }}×</span>
          </button>
        </div>
      </div>

      <!-- Footer Hint -->
      <div class="ab-drop-footer" *ngIf="sections.length > 0">
        <kbd>↑↓</kbd> navigate &nbsp; <kbd>⏎</kbd> select &nbsp; <kbd>Esc</kbd> close
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
    }

    .ab-drop-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 32px;
      padding: 0 var(--vsc-padding-lg);
      background-color: var(--vsc-panel-background, #252526) !important;
      border-bottom: 1px solid var(--vsc-border-subtle);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      text-transform: uppercase;
      letter-spacing: 0.5px;
      font-weight: 600;
    }

    .ab-qa-hint {
      font-size: 10px;
      opacity: 0.6;
      font-weight: normal;
      letter-spacing: normal;
      text-transform: none;
    }

    .ab-drop-scroll {
      max-height: min(400px, 60vh);
      overflow-x: hidden;
      overflow-y: auto;
      padding: var(--vsc-padding-xs) 0;
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

    .ab-drop-empty {
      padding: 20px;
      text-align: center;
      color: var(--vsc-foreground-dim);
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 8px;
    }

    .ab-drop-empty-icon {
      font-size: 24px;
      opacity: 0.5;
    }

    .ab-drop-footer {
      padding: 8px var(--vsc-padding-lg);
      border-top: 1px solid var(--vsc-border-subtle);
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      background-color: var(--vsc-panel-background, #252526) !important;
      text-align: center;
    }

    .ab-drop-footer kbd {
      display: inline-block;
      padding: 2px 4px;
      background: var(--vsc-keybinding-background);
      border: 1px solid var(--vsc-border);
      border-radius: 3px;
      font-size: 10px;
      font-family: monospace;
    }

    .qa-section {
      margin-bottom: 0;

      &:not(:first-child) {
        border-top: 1px solid var(--vsc-border-subtle);
      }
    }

    .qa-section-title {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 6px var(--vsc-padding-lg);
      margin-top: 4px;
      font-size: 11px;
      font-weight: 600;
      color: var(--vsc-foreground-dim);
      text-transform: uppercase;
      letter-spacing: 0.5px;
      background: transparent;
    }

    .qa-section-count {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 18px;
      height: 16px;
      padding: 0 4px;
      background: var(--vsc-badge-background);
      color: var(--vsc-badge-foreground);
      border-radius: 8px;
      font-size: 10px;
      font-weight: 600;
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

    .qa-item-count {
      font-size: 10px;
      color: var(--vsc-foreground-dim);
      background: var(--vsc-badge-background);
      padding: 1px 5px;
      border-radius: 8px;
      flex-shrink: 0;
      text-align: center;
    }

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

    .ab-drop-footer {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      height: 28px;
      padding: 0 var(--vsc-padding-md);
      font-size: 10px;
      color: var(--vsc-foreground-dim);
      border-top: 1px solid var(--vsc-border-subtle);
      background-color: var(--vsc-panel-background, #252526) !important;

      kbd {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 18px;
        padding: 2px 4px;
        background: var(--vsc-badge-background);
        color: var(--vsc-foreground-dim);
        border: 1px solid var(--vsc-border-subtle);
        border-radius: 2px;
        font-family: var(--vsc-font-family-mono);
        font-size: 9px;
        font-weight: 600;
      }
    }

  `]
})
export class AddressBarQuickAccessComponent implements OnInit {
  @Input() currentPath = '';
  @Input() specialFolders: SpecialFolder[] = [];
  @Input() pinnedPaths: PinnedPath[] = [];
  @Output() navigate = new EventEmitter<string>();

  sections: QuickAccessSection[] = [];
  highlightedItem: { sectionIdx: number; itemIdx: number } | null = null;

  constructor(private pathHistoryService: PathHistoryService) {}

  ngOnInit(): void {
    this.buildSections();
  }

  private buildSections(): void {
    this.sections = [];

    // System Folders
    if (this.specialFolders.length > 0) {
      this.sections.push({
        title: 'System Folders',
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

    // Initialize highlight to first item
    if (this.sections.length > 0 && this.sections[0].items.length > 0) {
      this.highlightedItem = { sectionIdx: 0, itemIdx: 0 };
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

  isHighlighted(sectionIdx: number, itemIdx: number): boolean {
    return this.highlightedItem?.sectionIdx === sectionIdx && this.highlightedItem?.itemIdx === itemIdx;
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

  onKeyDown(event: KeyboardEvent): void {
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
      default:
        break;
    }
  }

  private moveDown(): void {
    if (!this.highlightedItem) return;

    let { sectionIdx, itemIdx } = this.highlightedItem;

    // Try next item in current section
    if (itemIdx < this.sections[sectionIdx].items.length - 1) {
      itemIdx++;
      this.highlightedItem = { sectionIdx, itemIdx };
      return;
    }

    // Try next section
    if (sectionIdx < this.sections.length - 1) {
      sectionIdx++;
      itemIdx = 0;
      if (this.sections[sectionIdx].items.length > 0) {
        this.highlightedItem = { sectionIdx, itemIdx };
      }
    }
  }

  private moveUp(): void {
    if (!this.highlightedItem) return;

    let { sectionIdx, itemIdx } = this.highlightedItem;

    // Try previous item in current section
    if (itemIdx > 0) {
      itemIdx--;
      this.highlightedItem = { sectionIdx, itemIdx };
      return;
    }

    // Try previous section
    if (sectionIdx > 0) {
      sectionIdx--;
      itemIdx = this.sections[sectionIdx].items.length - 1;
      this.highlightedItem = { sectionIdx, itemIdx };
    }
  }
}
