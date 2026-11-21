import { Component, Input, Output, EventEmitter, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PinnedLocation } from './address-bar.types';
import { shortenPath } from './address-bar.utils';

@Component({
  selector: 'app-address-bar-pin-dropdown',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="ab-dropdown ab-pin-drop">
      <div class="ab-drop-head">
        <span>Pinned Locations</span>
        <span class="ab-badge">{{ pins.length }}</span>
      </div>
      <div class="ab-drop-empty" *ngIf="pins.length === 0">
        <span class="ab-drop-empty-icon">📍</span>
        <span>No pinned locations yet.<br>Click ☆ to pin the current path.</span>
      </div>
      <div class="ab-drop-scroll">
        <div
          *ngFor="let p of pins"
          class="ab-drop-item ab-pin-item"
          [class.ab-drop-item--active]="p.location === currentPath"
          (click)="navigate.emit(p.location)"
        >
          <span class="ab-pin-icon">{{ p.icon }}</span>
          <span class="ab-pin-label" [title]="p.location">{{ p.label }}</span>
          <span class="ab-pin-sub">{{ shorten(p.location) }}</span>
          <button type="button"
            class="ab-pin-remove"
            (click)="remove.emit({ pin: p, event: $event })"
            title="Remove"
            aria-label="Remove pin"
          >✕</button>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host { display: contents; }

    .ab-pin-drop {
      position: static;
      min-width: 270px;
      max-width: 370px;
    }
    .ab-pin-item { padding-right: var(--vsc-padding-sm); }
    .ab-pin-icon { font-size: 14px; flex-shrink: 0; }

    .ab-pin-label {
      flex: 1; font-size: var(--vsc-font-size); font-weight: 500;
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
    }

    .ab-pin-sub {
      font-size: 11px; color: var(--vsc-foreground-dim);
      font-family: var(--vsc-font-family-mono);
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
      max-width: 120px;
    }

    .ab-pin-remove {
      flex-shrink: 0; width: 20px; height: 20px; padding: 0;
      background: transparent; border: none; color: transparent;
      font-size: 12px; cursor: pointer; border-radius: var(--vsc-border-radius-sm);
      transition: all var(--vsc-transition-fast);
      display: flex; align-items: center; justify-content: center;

      .ab-drop-item:hover & { color: var(--vsc-foreground-dim); }
      &:hover { background: var(--vsc-error) !important; color: var(--vsc-foreground-bright) !important; }
    }

    /* dropdown base */
    .ab-dropdown {
      background-color: var(--vsc-panel-background, var(--vsc-panel-background)) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      overflow: hidden;
    }

    .ab-drop-head {
      display: flex; align-items: center; justify-content: space-between;
      height: 32px; padding: 0 var(--vsc-padding-lg);
      background: var(--vsc-panel-background, var(--vsc-panel-background)) !important;
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
      background-color: var(--vsc-panel-background, var(--vsc-panel-background)) !important;
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
        .ab-pin-sub { color: var(--vsc-foreground-bright); opacity: 0.9; }
      }
    }

    .ab-drop-empty {
      display: flex; flex-direction: column; align-items: center;
      gap: var(--vsc-padding-sm); padding: var(--vsc-padding-xl);
      font-size: var(--vsc-font-size-small); color: var(--vsc-foreground-dim);
      text-align: center; line-height: 1.5;
      background-color: var(--vsc-panel-background, var(--vsc-panel-background)) !important;
    }

    .ab-drop-empty-icon { font-size: 24px; opacity: 0.5; margin-bottom: var(--vsc-padding-xs); }
  `],
})
export class AddressBarPinDropdownComponent {
  @Input() pins: PinnedLocation[] = [];
  @Input() currentPath = '';
  @Output() navigate = new EventEmitter<string>();
  @Output() remove = new EventEmitter<{ pin: PinnedLocation; event: MouseEvent }>();

  shorten = shortenPath;
}