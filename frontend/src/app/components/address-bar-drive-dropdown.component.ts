import { Component, Input, Output, EventEmitter, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DriveInfo } from '../shared/protocol-enhanced';

@Component({
  selector: 'app-addressbar-drive-dropdown',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="ab-dropdown ab-drive-drop"
      [ngStyle]="{ left: left + 'px', top: top + 'px', position: 'fixed' }"
    >
      <div class="ab-drop-head">
        <span>Available Drives</span>
        <span class="ab-badge">{{ drives.length }}</span>
      </div>

      <div class="ab-drop-loading" *ngIf="loading">
        <span class="ab-spinner"></span>Loading drives…
      </div>

      <div class="ab-drop-empty" *ngIf="!loading && drives.length === 0">
        <span class="ab-drop-empty-icon">💽</span>
        <span>No drives found</span>
      </div>

      <div class="ab-drive-grid" *ngIf="!loading && drives.length > 0">
        <div
          *ngFor="let d of drives"
          class="ab-drive-grid-item"
          [class.ab-drive-grid-item--active]="d.path === currentPath"
          (click)="driveClick.emit(d)"
        >
          <div class="ab-drive-grid-top">
            <span class="ab-drive-grid-icon">{{ getIcon(d) }}</span>
            <span class="ab-drive-grid-letter">{{ d.name }}</span>
            <span class="ab-drive-grid-label">Local Drive</span>
            <svg *ngIf="d.path === currentPath" class="ab-drive-grid-check" width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
              <path d="M13.485 1.929L5.5 9.914 2.515 6.929a1 1 0 0 0-1.414 1.414l3.692 3.693a1 1 0 0 0 1.414 0l8.692-8.693a1 1 0 0 0-1.414-1.414z"/>
            </svg>
          </div>
          <div class="ab-drive-grid-details">
            <div class="ab-drive-grid-space" *ngIf="d.totalSpace > 0">
              <div class="ab-drive-grid-space-labels">
                <span class="ab-drive-grid-space-free">{{ formatSize(d.freeSpace) }} free</span>
                <span>of {{ formatSize(d.totalSpace) }}</span>
              </div>
              <div class="ab-drive-space-bar">
                <div class="ab-drive-space-fill" [style.width.%]="(d.freeSpace / d.totalSpace) * 100"></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="ab-drop-footer" *ngIf="!loading && drives.length > 0">
        <kbd>↑↓</kbd> navigate &nbsp; <kbd>⏎</kbd> select &nbsp; <kbd>Esc</kbd> close
      </div>
    </div>
  `,
  styles: [`
    :host { display: contents; }

    .ab-drive-drop {
      min-width: 320px; max-width: 500px; padding: 0;
      z-index: 9999 !important;
      background-color: var(--vsc-panel-background, #252526) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      overflow: hidden;
      animation: ab-appear var(--vsc-transition-fast) cubic-bezier(0.4, 0, 0.2, 1);
    }

    @keyframes ab-appear {
      from { opacity: 0; transform: translateY(-4px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    .ab-drop-head {
      display: flex; align-items: center; justify-content: space-between;
      height: 32px; padding: 0 var(--vsc-padding-lg);
      background-color: var(--vsc-panel-background, #252526) !important;
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

    .ab-drive-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-lg);
      max-height: 350px; overflow-y: auto;
      background-color: var(--vsc-panel-background, #252526) !important;
    }

    .ab-drive-grid-item {
      display: flex; flex-direction: column;
      padding: var(--vsc-padding-lg);
      background: var(--vsc-sidebar-background, #1e1e1e) !important;
      border: 1px solid var(--vsc-border-subtle);
      border-radius: var(--vsc-border-radius);
      cursor: pointer; transition: all var(--vsc-transition-fast);
      gap: var(--vsc-padding-sm);

      &:hover {
        background: var(--vsc-list-hover-background) !important;
        border-color: var(--vsc-accent-blue);
        transform: translateY(-1px);
        box-shadow: 0 2px 8px var(--vsc-shadow);
      }

      &.ab-drive-grid-item--active {
        background: var(--vsc-list-active-background) !important;
        border-color: var(--vsc-accent-blue);
      }
    }

    .ab-drive-grid-top { display: flex; align-items: center; gap: var(--vsc-padding-sm); }
    .ab-drive-grid-icon { font-size: 20px; flex-shrink: 0; }
    .ab-drive-grid-letter { font-family: var(--vsc-font-family-mono); font-size: 14px; font-weight: 700; color: var(--vsc-accent-blue); }
    .ab-drive-grid-label { font-size: var(--vsc-font-size); font-weight: 500; color: var(--vsc-foreground); flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .ab-drive-grid-check { color: var(--vsc-accent-blue); margin-left: auto; flex-shrink: 0; }

    .ab-drive-grid-details { display: flex; flex-direction: column; gap: 5px; font-size: 11px; color: var(--vsc-foreground-dim); margin-top: 2px; }
    .ab-drive-grid-space { display: flex; flex-direction: column; gap: 4px; }
    .ab-drive-grid-space-labels { display: flex; align-items: center; justify-content: space-between; font-size: 10px; color: var(--vsc-foreground-dim); }
    .ab-drive-grid-space-free { font-weight: 600; color: var(--vsc-foreground); }
    .ab-drive-space-bar { width: 100%; height: 4px; background: var(--vsc-border-subtle); border-radius: 2px; overflow: hidden; }
    .ab-drive-space-fill { height: 100%; background: var(--vsc-accent-blue); border-radius: 2px; transition: width var(--vsc-transition-normal); }

    .ab-drop-footer {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      height: 28px; padding: 0 var(--vsc-padding-lg);
      background-color: var(--vsc-panel-background, #252526) !important;
      border-top: 1px solid var(--vsc-border-subtle);
      font-size: 11px; color: var(--vsc-foreground-dim);

      kbd {
        display: inline-flex; align-items: center; justify-content: center;
        min-width: 18px; height: 18px; padding: 0 3px;
        background: var(--vsc-input-background); border: 1px solid var(--vsc-border);
        border-bottom-width: 2px; border-radius: var(--vsc-border-radius-sm);
        font-family: var(--vsc-font-family-mono); font-size: 10px;
        color: var(--vsc-foreground); margin: 0 var(--vsc-padding-xs);
      }
    }

    .ab-drop-loading {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-lg); background-color: var(--vsc-panel-background, #252526) !important;
    }

    .ab-spinner {
      display: inline-block; width: 14px; height: 14px;
      border: 2px solid var(--vsc-border); border-top-color: var(--vsc-accent-blue);
      border-radius: 50%; animation: ab-spin 0.7s linear infinite;
    }

    @keyframes ab-spin { to { transform: rotate(360deg); } }

    .ab-drop-empty {
      display: flex; flex-direction: column; align-items: center;
      padding: var(--vsc-padding-xl); text-align: center;
      background-color: var(--vsc-panel-background, #252526) !important;
      font-size: var(--vsc-font-size-small); color: var(--vsc-foreground-dim);
    }

    .ab-drop-empty-icon { font-size: 24px; opacity: 0.5; margin-bottom: var(--vsc-padding-xs); }
  `],
})
export class AddressBarDriveDropdownComponent {
  @Input() drives: DriveInfo[] = [];
  @Input() loading = false;
  @Input() left = 0;
  @Input() top = 0;
  @Input() currentPath = '';
  @Output() driveClick = new EventEmitter<DriveInfo>();

  getIcon(d: DriveInfo): string {
    if (d.driveType === 'fixed')    return '💽';
    if (d.driveType === 'removable') return '💾';
    if (d.driveType === 'cdrom')    return '💿';
    if (d.driveType === 'network')  return '🌐';
    if (d.driveType === 'ramdisk')  return '⚡';
    return '💽';
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
  }
}
