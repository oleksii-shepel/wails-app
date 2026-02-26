// dropdown-container.component.ts
import { Component, Input, Output, EventEmitter, ViewChild, ContentChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DropdownDirective, DropdownMenuDirective } from '../directives/dropdown.directive';

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

    ::ng-deep [appDropdownMenu] {
      position: absolute;
      z-index: 999;
    }
  `]
})
export class DropdownContainerComponent {
  @Input() isOpen = false;
  @Output() isOpenChange = new EventEmitter<boolean>();
  
  @ViewChild('dropdownRef', { static: true }) dropdownRef?: DropdownDirective;
  @ContentChild(DropdownMenuDirective, { static: false }) menuDir?: DropdownMenuDirective;

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

  ngAfterViewInit() {
    this.positionDropdown();
  }

  ngOnChanges() {
    this.positionDropdown();
  }

  private positionDropdown() {
    if (!this.isOpen) return;
    const menuEl = this.dropdownRef?.menuDir?.el.nativeElement;
    const triggerEl = this.dropdownRef?.triggerDir?.el.nativeElement;
    if (!menuEl || !triggerEl) return;

    // Get bounding rects
    const triggerRect = triggerEl.getBoundingClientRect();
    const menuRect = menuEl.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;

    // Default position: below trigger, left-aligned
    let top = triggerRect.bottom;
    let left = triggerRect.left;

    // Adjust if dropdown would overflow right
    if (left + menuRect.width > viewportWidth) {
      left = viewportWidth - menuRect.width - 8;
    }
    // Adjust if dropdown would overflow bottom
    if (top + menuRect.height > viewportHeight) {
      top = triggerRect.top - menuRect.height;
      if (top < 0) top = 8;
    }
    // Adjust if dropdown would overflow left
    if (left < 0) left = 8;

    // Apply position
    menuEl.style.position = 'absolute';
    menuEl.style.left = `${left}px`;
    menuEl.style.top = `${top}px`;
    menuEl.style.zIndex = '999';
  }
}