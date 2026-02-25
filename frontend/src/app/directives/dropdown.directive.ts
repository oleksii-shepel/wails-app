import {
  Directive,
  ElementRef,
  Input,
  OnDestroy,
  OnInit,
  Output,
  EventEmitter,
  ContentChild,
  AfterContentInit,
  AfterContentChecked,
} from '@angular/core';
import { DropdownService } from '../services/dropdown.service';

/**
 * DropdownTriggerDirective — marks the trigger element of a dropdown.
 *
 * Usage:
 *   <button appDropdownTrigger>Open</button>
 */
@Directive({ selector: '[appDropdownTrigger]', standalone: true })
export class DropdownTriggerDirective {
  constructor(public el: ElementRef<HTMLElement>) {}
}

/**
 * DropdownMenuDirective — marks the menu panel of a dropdown.
 * Works with *ngIf — the directive re-registers whenever the element
 * appears or disappears in the DOM.
 *
 * Usage:
 *   <div appDropdownMenu *ngIf="isOpen">...</div>
 */
@Directive({ selector: '[appDropdownMenu]', standalone: true })
export class DropdownMenuDirective {
  constructor(public el: ElementRef<HTMLElement>) {}
}

/**
 * DropdownDirective — the host directive that wires everything together.
 *
 * USAGE:
 *
 * ```html
 * <div appDropdown [(isOpen)]="menuOpen">
 *   <button appDropdownTrigger (click)="menuOpen = !menuOpen">Filter ▾</button>
 *   <div appDropdownMenu *ngIf="menuOpen">
 *     <div class="item" (click)="selectItem(x)">Item</div>
 *   </div>
 * </div>
 * ```
 *
 * Handles:
 *  - Registering/updating with DropdownService (including after *ngIf re-creates the menu)
 *  - Closing on outside click (via DropdownService)
 *  - NOT closing when clicking inside — no race conditions
 *  - Closing other open dropdowns when this one opens
 *  - Escape key closes
 *  - Cleanup on destroy
 */
@Directive({
  selector: '[appDropdown]',
  standalone: true,
  host: {
    '(keydown.escape)': 'close()',
  },
  exportAs: 'appDropdown',
})
export class DropdownDirective implements OnInit, AfterContentInit, AfterContentChecked, OnDestroy {
  // Bind to the host attribute name so templates can use `[appDropdown]`.
  @Input('appDropdown') isOpen = false;
  @Output() isOpenChange = new EventEmitter<boolean>();
  @Output() closed = new EventEmitter<void>();

  @ContentChild(DropdownTriggerDirective) triggerDir?: DropdownTriggerDirective;
  @ContentChild(DropdownMenuDirective) menuDir?: DropdownMenuDirective;

  private dropdownId!: string;
  private lastMenuEl: HTMLElement | null = null;

  constructor(
    private el: ElementRef<HTMLElement>,
    private dropdownService: DropdownService,
  ) {}

  ngOnInit(): void {
    this.dropdownId = this.dropdownService.register(
      this.el.nativeElement,
      this.el.nativeElement,
      () => this.close(),
    );
  }

  ngAfterContentInit(): void {
    this.syncRegistration();
  }

  ngAfterContentChecked(): void {
    // Re-sync whenever *ngIf creates or destroys the menu element
    const currentMenuEl = this.menuDir?.el.nativeElement ?? null;
    if (currentMenuEl !== this.lastMenuEl) {
      this.syncRegistration();
      this.lastMenuEl = currentMenuEl;
    }
  }

  private syncRegistration(): void {
    const trigger = this.triggerDir?.el.nativeElement ?? this.el.nativeElement;
    const menu    = this.menuDir?.el.nativeElement    ?? this.el.nativeElement;
    this.dropdownService.update(this.dropdownId, trigger, menu);
  }

  open(): void {
    this.dropdownService.closeOthers(this.dropdownId);
    this.isOpen = true;
    this.isOpenChange.emit(true);
  }

  close(): void {
    if (!this.isOpen) return;
    if (this.dropdownService.isInteractionPending(this.dropdownId)) return;
    this.isOpen = false;
    this.isOpenChange.emit(false);
    this.closed.emit();
  }

  toggle(): void {
    this.isOpen ? this.close() : this.open();
  }

  ngOnDestroy(): void {
    this.dropdownService.unregister(this.dropdownId);
  }
}
