// dropdown-container.component.ts
import { Component, Input, Output, EventEmitter, ViewChild, ContentChild, ElementRef, OnChanges, SimpleChanges, AfterViewInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DropdownDirective, DropdownTriggerDirective, DropdownMenuDirective } from '../directives/dropdown.directive';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';

@Component({
  selector: 'app-dropdown-container',
  standalone: true,
  imports: [CommonModule, DropdownDirective],
  template: `
    <div 
      class="dropdown-wrapper" 
      [appDropdown]="isOpen" 
      (isOpenChange)="onToggle($event)"
      #dropdownRef="appDropdown"
    >
      <!-- Trigger slot — always rendered, never inside *ngIf -->
      <ng-content select="[appDropdownTrigger]"></ng-content>

      <!-- Backdrop - captures clicks outside the menu -->
      <div 
        *ngIf="isOpen" 
        class="dropdown-backdrop" 
        (click)="onBackdropClick($event)"
        aria-hidden="true">
      </div>

      <!-- Menu content wrapper - stops click propagation -->
      <div 
        *ngIf="isOpen" 
        class="dropdown-menu-wrapper"
        appDropdownMenu
        tabindex="-1"
        (click)="$event.stopPropagation()">
        <ng-content></ng-content>
      </div>
    </div>
  `,
  styles: [`
    .dropdown-wrapper {
      position: relative;
      display: inline-block;
    }

    .dropdown-backdrop {
      position: fixed;
      top: 0;
      left: 0;
      width: 100vw;
      height: 100vh;
      background-color: rgba(0, 0, 0, 0.4);
      z-index: calc(var(--vsc-z-dropdown) - 1);
      cursor: default;
    }

    .dropdown-menu-wrapper {
      position: fixed;
      z-index: var(--vsc-z-dropdown);
      animation: dropdown-appear 0.12s cubic-bezier(0.4, 0, 0.2, 1);
    }

    @keyframes dropdown-appear {
      from { opacity: 0; transform: translateY(-6px) scale(0.98); }
      to   { opacity: 1; transform: translateY(0)   scale(1); }
    }
  `]
})
export class DropdownContainerComponent implements AfterViewInit, OnChanges, OnDestroy {
  @Input() isOpen = false;
  /** Fallback anchor for when the trigger element is outside the container (e.g. right-click menus). */
  @Input() anchorEl: HTMLElement | null = null;
  @Output() isOpenChange = new EventEmitter<boolean>();

  @ViewChild('dropdownRef', { static: true }) dropdownRef?: DropdownDirective;
  @ContentChild(DropdownTriggerDirective, { static: false }) triggerDir?: DropdownTriggerDirective;
  private readonly overlayFreezeKey = `dropdown:${Math.random().toString(36).slice(2, 8)}`;
  private overlayFreezeHeld = false;

  constructor(
    private _el: ElementRef<HTMLElement>,
    private overlayFreeze: NativeOverlayFreezeService,
  ) {}

  onToggle(state: boolean) {
    this.isOpen = state;
    this.isOpenChange.emit(state);
    this.syncOverlayFreeze();
  }

  onBackdropClick(event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.close();
  }

  close() {
    this.isOpen = false;
    this.isOpenChange.emit(false);
    this.syncOverlayFreeze();
  }

  ngAfterViewInit(): void {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['isOpen']) {
      this.syncOverlayFreeze();
    }
    if (changes['isOpen']?.currentValue === true) {
      // Use requestAnimationFrame to ensure DOM is ready
      requestAnimationFrame(() => {
        setTimeout(() => {
          this.positionMenu();
          this.focusMenu();
        }, 0);
      });
    }
  }

  ngOnDestroy(): void {
    if (!this.overlayFreezeHeld) {
      return;
    }
    this.overlayFreeze.release(this.overlayFreezeKey);
    this.overlayFreezeHeld = false;
  }

  private focusMenu(): void {
    if (!this.isOpen) return;
    const wrapperEl = this._el.nativeElement.querySelector('.dropdown-menu-wrapper') as HTMLElement | null;
    const focusTarget = wrapperEl?.querySelector<HTMLElement>(
      '.ab-quick-access-drop, [role="menu"], [role="dialog"], [role="listbox"]'
    ) ?? wrapperEl;
    focusTarget?.focus({ preventScroll: true });
  }

  private positionMenu(): void {
    if (!this.isOpen) return;
    const anchor = (this.triggerDir?.el.nativeElement ?? this.anchorEl) as HTMLElement | null;
    if (!anchor) return;
    const wrapperEl = this._el.nativeElement.querySelector('.dropdown-menu-wrapper') as HTMLElement | null;
    if (!wrapperEl) return;
    const rect  = anchor.getBoundingClientRect();
    const menuW = wrapperEl.offsetWidth  || 200;
    const menuH = wrapperEl.offsetHeight || 100;
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    let left = rect.left;
    let top  = rect.bottom + 4;
    if (left + menuW > vw) left = Math.max(0, vw - menuW - 8);
    if (top  + menuH > vh) top  = Math.max(0, rect.top - menuH - 4);
    if (left < 0) left = 8;
    wrapperEl.style.position = 'fixed';
    wrapperEl.style.left     = `${Math.round(left)}px`;
    wrapperEl.style.top      = `${Math.round(top)}px`;
    wrapperEl.style.zIndex   = getComputedStyle(document.documentElement)
      .getPropertyValue('--vsc-z-dropdown')
      .trim() || '1000';
  }

  private syncOverlayFreeze(): void {
    if (this.isOpen) {
      if (!this.overlayFreezeHeld) {
        this.overlayFreeze.acquire(this.overlayFreezeKey);
        this.overlayFreezeHeld = true;
      }
      return;
    }

    if (!this.overlayFreezeHeld) {
      return;
    }
    this.overlayFreeze.release(this.overlayFreezeKey);
    this.overlayFreezeHeld = false;
  }
}
