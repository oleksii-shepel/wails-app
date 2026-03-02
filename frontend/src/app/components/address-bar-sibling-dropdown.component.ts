import { Component, Input, Output, EventEmitter, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SiblingEntry, PathSegment } from './address-bar.types';

@Component({
  selector: 'app-address-bar-sibling-dropdown',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="ab-dropdown ab-sib-drop"
    >
      <div class="ab-drop-head">
        <span>{{ headLabel }}</span>
        <span class="ab-badge" *ngIf="!loading">{{ entries.length }}</span>
      </div>
      <div class="ab-drop-loading" *ngIf="loading">
        <span class="ab-spinner"></span>Loading…
      </div>
      <div class="ab-drop-empty" *ngIf="!loading && entries.length === 0">
        <span class="ab-drop-empty-icon">📭</span>
        <span>No sub-folders</span>
      </div>
      <div class="ab-drop-scroll">
        <div
          *ngFor="let s of entries"
          class="ab-drop-item ab-sib-item"
          [class.ab-drop-item--active]="s.isCurrent"
          (click)="entryClick.emit(s)"
        >
          <span class="ab-sib-icon">{{ s.isArchive ? '🗜️' : '📁' }}</span>
          <span class="ab-sib-name">{{ s.name }}</span>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host { display: contents; }

    .ab-sib-drop {
      min-width: 210px;
      max-width: 320px;
    }

    .ab-sib-icon { font-size: 14px; flex-shrink: 0; opacity: 0.8; }

    .ab-sib-name {
      flex: 1;
      font-size: var(--vsc-font-size);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    /* ── dropdown base (duplicated here so component is self-contained) ── */
    .ab-dropdown {
      background-color: var(--vsc-panel-background, #252526) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      z-index: 9999;
      overflow: hidden;
    }

    .ab-drop-head {
      display: flex; align-items: center; justify-content: space-between;
      height: 32px; padding: 0 var(--vsc-padding-lg);
      background: var(--vsc-panel-background, #252526) !important;
      border-bottom: 1px solid var(--vsc-border-subtle);
      font-size: var(--vsc-font-size-small); color: var(--vsc-foreground-dim);
      text-transform: uppercase; letter-spacing: 0.5px; font-weight: 600;
    }

    .ab-badge {
      display: inline-flex; align-items: center; justify-content: center;
      min-width: 20px; height: 20px; padding: 0 var(--vsc-padding-xs);
      background: var(--vsc-badge-background); color: var(--vsc-badge-foreground);
      border-radius: 10px; font-size: 11px; font-weight: 600;
    }

    .ab-drop-scroll {
      max-height: 300px; overflow-y: auto; padding: var(--vsc-padding-xs) 0;
      background-color: var(--vsc-panel-background, #252526) !important;
      &::-webkit-scrollbar { width: 10px; }
      &::-webkit-scrollbar-track { background: var(--vsc-scrollbar-track); }
      &::-webkit-scrollbar-thumb { background: var(--vsc-scrollbar-thumb); border-radius: 5px; }
    }

    .ab-drop-item {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      height: 28px; padding: 0 var(--vsc-padding-lg); cursor: pointer;
      transition: background-color var(--vsc-transition-fast);
      border-left: 2px solid transparent;
      color: var(--vsc-foreground); font-size: var(--vsc-font-size);
      background: transparent;
      &:hover { background: var(--vsc-list-hover-background); }
      &.ab-drop-item--active {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
        border-left-color: var(--vsc-accent-blue);
      }
    }

    .ab-drop-loading {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-lg); font-size: var(--vsc-font-size);
      color: var(--vsc-foreground-dim);
      background-color: var(--vsc-panel-background, #252526) !important;
    }

    .ab-spinner {
      display: inline-block; width: 14px; height: 14px;
      border: 2px solid var(--vsc-border); border-top-color: var(--vsc-accent-blue);
      border-radius: 50%; animation: ab-spin 0.7s linear infinite;
    }

    @keyframes ab-spin { to { transform: rotate(360deg); } }

    .ab-drop-empty {
      display: flex; flex-direction: column; align-items: center;
      gap: var(--vsc-padding-sm); padding: var(--vsc-padding-xl);
      font-size: var(--vsc-font-size-small); color: var(--vsc-foreground-dim);
      text-align: center; line-height: 1.5;
      background-color: var(--vsc-panel-background, #252526) !important;
    }

    .ab-drop-empty-icon { font-size: 24px; opacity: 0.5; margin-bottom: var(--vsc-padding-xs); }
  `],
})
export class AddressBarSiblingDropdownComponent {
  @Input() entries: SiblingEntry[] = [];
  @Input() loading = false;
  @Input() headLabel = '';
  @Output() entryClick = new EventEmitter<SiblingEntry>();
}
