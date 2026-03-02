// dropdown-container.component.ts
import { Component, Input, Output, EventEmitter, ViewChild, ContentChild, ElementRef, OnChanges, SimpleChanges, AfterViewInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DropdownDirective, DropdownTriggerDirective, DropdownMenuDirective } from '../directives/dropdown.directive';

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
      z-index: 998;
      cursor: default;
    }

    .dropdown-menu-wrapper {
      position: relative;
      z-index: 999;
    }
  `]
})
export class DropdownContainerComponent implements AfterViewInit, OnChanges {
  @Input() isOpen = false;
  /** Fallback anchor for when the trigger element is outside the container (e.g. right-click menus). */
  @Input() anchorEl: HTMLElement | null = null;
  @Output() isOpenChange = new EventEmitter<boolean>();

  @ViewChild('dropdownRef', { static: true }) dropdownRef?: DropdownDirective;
  @ContentChild(DropdownTriggerDirective, { static: false }) triggerDir?: DropdownTriggerDirective;

  constructor(private _el: ElementRef<HTMLElement>) {}

  onToggle(state: boolean) {
    this.isOpen = state;
    this.isOpenChange.emit(state);
  }

  onBackdropClick(event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.close();
  }

  close() {
    this.isOpen = false;
    this.isOpenChange.emit(false);
  }

  ngAfterViewInit(): void {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['isOpen']?.currentValue === true) {
      setTimeout(() => this.positionMenu());
    }
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
    wrapperEl.style.zIndex   = '9999';
  }
}