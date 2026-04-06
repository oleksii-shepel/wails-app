import { ChangeDetectionStrategy, ChangeDetectorRef, Component, ElementRef, EventEmitter, Input, OnChanges, OnDestroy, OnInit, Output, SimpleChanges, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DriveInfo } from '../shared/protocol';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { normalizePath as normalizeAddressBarPath } from './address-bar.utils';

@Component({
  selector: 'app-address-bar-scheme-dropdown',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      #rootEl
      class="ab-dropdown ab-scheme-drop"
      role="listbox"
      tabindex="-1"
      aria-label="Available schemes"
    >
      <div class="ab-drop-head">
        <span>Available schemes</span>
        <span class="ab-badge">{{ schemes.length }}</span>
      </div>

      <div class="ab-drop-loading" *ngIf="loading">
        <span class="ab-spinner"></span>Loading schemes...
      </div>

      <div class="ab-drop-empty" *ngIf="!loading && schemes.length === 0">
        <span class="ab-drop-empty-icon">💽</span>
        <span>No schemes found</span>
      </div>

      <div class="ab-scheme" *ngIf="!loading && schemes.length > 0">
        <div
          *ngFor="let d of schemes"
          class="ab-scheme-item"
          role="option"
          [attr.tabindex]="selectedIndex >= 0 && schemes[selectedIndex].path === d.path ? 0 : -1"
          [attr.aria-selected]="selectedIndex >= 0 && schemes[selectedIndex].path === d.path"
          [class.ab-scheme-item--active]="isActive(d)"
          [class.ab-scheme-item--selected]="selectedIndex >= 0 && schemes[selectedIndex].path === d.path"
          [title]="getDisplayPath(d.path)"
          (focus)="setSelectedIndex(schemes.indexOf(d))"
          (click)="schemeClick.emit(d)"
          (mouseenter)="setSelectedIndex(schemes.indexOf(d))"
        >
          <div class="ab-scheme-top">
            <span class="ab-scheme-icon">{{ getIcon(d) }}</span>
            <span>{{ getSchemeRootLabel(d) }}</span>
            <span class="ab-scheme-label">{{ getSchemeTypeLabel(d) }}</span>
            <svg *ngIf="isSelected(d)" class="ab-scheme-dot" width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <circle cx="8" cy="8" r="2.5" fill="currentColor" />
            </svg>
          </div>
          <div class="ab-scheme-details">
            <div class="ab-scheme-path" [title]="getDisplayPath(d.path)">
              {{ getDisplayPath(d.path) }}
            </div>
            <div class="ab-scheme-space" *ngIf="d.totalSpace > 0">
              <div class="ab-scheme-space-labels">
                <span class="ab-scheme-space-free">{{ formatSize(d.freeSpace) }} free</span>
                <span>of {{ formatSize(d.totalSpace) }}</span>
              </div>
              <div class="ab-scheme-space-bar">
                <div class="ab-scheme-space-fill" [style.width.%]="(d.freeSpace / d.totalSpace) * 100"></div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host { display: contents; }

    .ab-scheme-drop {
      position: static;
      min-width: 320px; max-width: 500px; padding: 0;
      z-index: 9999 !important;
      background-color: var(--vsc-panel-background, #252526) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      overflow: hidden;
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

    .ab-scheme {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-lg);
      max-height: 350px; overflow-y: auto;
      background-color: var(--vsc-panel-background, #252526) !important;
      scrollbar-width: none;
      -ms-overflow-style: none;

      &::-webkit-scrollbar {
        display: none;
        width: 0;
        height: 0;
      }
    }

    .ab-scheme-item {
      display: flex; flex-direction: column;
      padding: var(--vsc-padding-lg);
      background: var(--vsc-sidebar-background, #1e1e1e) !important;
      border: 1px solid var(--vsc-border-subtle);
      border-radius: var(--vsc-border-radius);
      cursor: pointer; transition: all var(--vsc-transition-fast);
      gap: var(--vsc-padding-sm);
      min-height: 108px;

      &:hover {
        background: var(--vsc-panel-background) !important;
        border-color: var(--vsc-border);
      }

      &.ab-scheme-item--active {
        background: var(--vsc-panel-background) !important;
        border-color: var(--vsc-border);
      }

      &.ab-scheme-item--selected {
        border-color: var(--vsc-accent-blue);
        box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--vsc-accent-blue) 50%, transparent);
      }
    }

    .ab-scheme-top { display: flex; align-items: center; gap: var(--vsc-padding-sm); }
    .ab-scheme-icon { font-size: 20px; flex-shrink: 0; }
    .ab-scheme-label { font-size: var(--vsc-font-size); font-weight: 500; color: var(--vsc-foreground); flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .ab-scheme-dot { color: var(--vsc-accent-blue); margin-left: auto; flex-shrink: 0; }

    .ab-scheme-details {
      display: flex;
      flex-direction: column;
      gap: 5px;
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      margin-top: 2px;
      min-height: 32px;
    }
    .ab-scheme-path {
      font-family: var(--vsc-font-family-mono);
      font-size: 10px;
      color: var(--vsc-foreground-dim);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      max-width: 100%;
      opacity: 0.95;
    }
    .ab-scheme-space { display: flex; flex-direction: column; gap: 4px; }
    .ab-scheme-space--placeholder { min-height: 18px; }
    .ab-scheme-space-labels { display: flex; align-items: center; justify-content: space-between; font-size: 10px; color: var(--vsc-foreground-dim); }
    .ab-scheme-space-free { font-weight: 600; color: var(--vsc-foreground); }
    .ab-scheme-space-bar { width: 100%; height: 4px; background: var(--vsc-border-subtle); border-radius: 2px; overflow: hidden; }
    .ab-scheme-space-fill { height: 100%; background: var(--vsc-accent-blue); border-radius: 2px; transition: width var(--vsc-transition-normal); }

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
export class AddressBarSchemeDropdownComponent {
  @ViewChild('rootEl') rootElRef?: ElementRef<HTMLElement>;

  @Input() schemes: DriveInfo[] = [];
  @Input() loading = false;
  @Input() currentPath = '';
  @Output() schemeClick = new EventEmitter<DriveInfo>();
  @Output() closeRequested = new EventEmitter<void>();

  selectedIndex = -1;

  private readonly commandOwner = `overlay.addressBar.schemes:${Math.random().toString(36).slice(2, 8)}`;
  private releaseOverlayContext: (() => void) | null = null;

  constructor(
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    this.registerInputCommands();
    this.releaseOverlayContext ??= this.inputContexts.acquire('overlay.addressBar.schemes');
    this.syncSelection();
  }

  ngOnChanges(_changes: SimpleChanges): void {
    this.syncSelection();
  }

  ngOnDestroy(): void {
    this.releaseOverlayContext?.();
    this.releaseOverlayContext = null;
    this.unregisterInputCommands();
  }

  private normalizePath(path: string): string {
    return normalizeAddressBarPath(path || '');
  }

  isActive(d: DriveInfo): boolean {
    const current = this.normalizePath(this.currentPath).toLowerCase();
    const root = this.normalizePath(d.path).toLowerCase();
    if (/^[a-z][a-z0-9+.-]*:\/\//.test(root)) {
      return current === root || current.startsWith(root);
    }
    return current === root || current.startsWith(`${root}/`);
  }

  isSelected(d: DriveInfo): boolean {
    return this.isActive(d) || (this.selectedIndex >= 0 && this.schemes[this.selectedIndex]?.path === d.path);
  }

  getSchemeRootLabel(d: DriveInfo): string {
    const path = this.normalizePath(d.path || '');
    const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec(path);
    if (match && match[1].toLowerCase() !== 'drive') {
      return `${match[1].toLowerCase()}:`;
    }
    return d.name;
  }

  getSchemeTypeLabel(d: DriveInfo): string {
    if (d.schemeType?.startsWith('cloud-')) {
      return 'Cloud Storage';
    }
    if (d.schemeType === 'memory') {
      return 'In-Memory FS';
    }
    if (d.schemeType === 'registry') {
      return 'Registry';
    }
    if (d.schemeType === 'search') {
      return 'Search FS';
    }
    if (d.schemeType === 'http') {
      return 'HTTP';
    }
    if (d.schemeType === 'https') {
      return 'HTTPS';
    }
    if (d.schemeType === 'smb') {
      return 'SMB Share';
    }
    if (d.schemeType === 'network') {
      return 'Network Drive';
    }
    return 'Local Drive';
  }

  getDisplayPath(path: string): string {
    return this.normalizePath(path || '');
  }

  getIcon(d: DriveInfo): string {
    if (d.schemeType === 'cloud-onedrive') return '☁️';
    if (d.schemeType === 'cloud-google')   return '🗂️';
    if (d.schemeType === 'cloud-dropbox')  return '📦';
    if (d.schemeType === 'memory')         return '🧠';
    if (d.schemeType === 'registry')       return '🧩';
    if (d.schemeType === 'search')         return '🔎';
    if (d.schemeType === 'http')           return '🌐';
    if (d.schemeType === 'https')          return '🔒';
    if (d.schemeType === 'smb')            return '🖧';
    if (d.schemeType === 'fixed')          return '💽';
    if (d.schemeType === 'removable')      return '💾';
    if (d.schemeType === 'cdrom')          return '💿';
    if (d.schemeType === 'network')        return '🌐';
    if (d.schemeType === 'ramdisk')        return '⚡';
    return '💽';
  }
  
  formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
  }

  setSelectedIndex(index: number): void {
    if (index < 0 || index >= this.schemes.length) return;
    this.selectedIndex = index;
    this.cdr.markForCheck();
    this.focusSelectedItem();
  }

  openSelected(): void {
    const selected = this.schemes[this.selectedIndex];
    if (!selected) return;
    this.schemeClick.emit(selected);
  }

  private syncSelection(): void {
    if (this.schemes.length === 0) {
      this.selectedIndex = -1;
      this.cdr.markForCheck();
      return;
    }

    const activeIndex = this.schemes.findIndex(drive => this.isActive(drive));
    this.selectedIndex = activeIndex >= 0 ? activeIndex : 0;
    this.cdr.markForCheck();
    this.focusSelectedItem();
  }

  private moveSelection(delta: number): void {
    if (this.schemes.length === 0) return;
    const baseIndex = this.selectedIndex >= 0 ? this.selectedIndex : 0;
    const next = Math.max(0, Math.min(this.schemes.length - 1, baseIndex + delta));
    this.selectedIndex = next;
    this.cdr.markForCheck();
    this.focusSelectedItem();
  }

  private focusSelectedItem(): void {
    requestAnimationFrame(() => {
      const item = this.rootElRef?.nativeElement.querySelector<HTMLElement>('.ab-scheme-item--selected');
      item?.focus({ preventScroll: true });
    });
  }

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('overlay.addressBar.schemes.close', () => {
      this.closeRequested.emit();
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.addressBar.schemes.prevItem', () => {
      this.moveSelection(-1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.addressBar.schemes.nextItem', () => {
      this.moveSelection(1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.addressBar.schemes.prevColumn', () => {
      this.moveSelection(-1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.addressBar.schemes.nextColumn', () => {
      this.moveSelection(1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.addressBar.schemes.openSelected', () => {
      this.openSelected();
    }, this.commandOwner);
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('overlay.addressBar.schemes.close', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.addressBar.schemes.prevItem', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.addressBar.schemes.nextItem', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.addressBar.schemes.prevColumn', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.addressBar.schemes.nextColumn', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.addressBar.schemes.openSelected', this.commandOwner);
  }
}
