import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DropdownDirective } from '../directives/dropdown.directive';

@Component({
  selector: 'app-dropdown-container',
  standalone: true,
  imports: [CommonModule, DropdownDirective],
  template: `
    <div 
      class="dropdown-wrapper" 
      [appDropdown]="isOpen" 
      (isOpenChange)="onToggle($event)"
    >
      <div *ngIf="isOpen" class="dropdown-backdrop" (click)="close()" aria-hidden="true"></div>

      <ng-content></ng-content>
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
      background-color: rgba(0, 0, 0, 0.4); /* Dimmed effect */
      z-index: 998; /* Just below the menu */
      cursor: default;
    }

    /* We use ::ng-deep to ensure the projected menu stays above the backdrop */
    ::ng-deep [appDropdownMenu] {
      position: absolute;
      z-index: 999;
    }
  `]
})
export class DropdownContainerComponent {
  @Input() isOpen = false;
  @Output() isOpenChange = new EventEmitter<boolean>();

  onToggle(state: boolean) {
    this.isOpen = state;
    this.isOpenChange.emit(state);
  }

  close() {
    this.isOpen = false;
    this.isOpenChange.emit(false);
  }
}