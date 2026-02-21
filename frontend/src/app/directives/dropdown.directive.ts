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
 * <!-- Basic -->
 * <div appDropdown [(isOpen)]="menuOpen">
 *   <button appDropdownTrigger (click)="menuOpen = !menuOpen">Filter ▾</button>
 *   <div appDropdownMenu *ngIf="menuOpen">...</div>
 * </div>
 *
 * <!-- With backdrop (closes on click-outside, blocks scroll) -->
 * <div appDropdown [(isOpen)]="menuOpen" [backdrop]="true">
 *   ...
 * </div>
 *
 * <!-- With dim backdrop (visually darkens page behind the menu) -->
 * <div appDropdown [(isOpen)]="menuOpen" [backdrop]="true" [backdropDim]="true">
 *   ...
 * </div>
 * ```
 *
 * Backdrop behaviour:
 *  - A full-screen fixed <div> is inserted into document.body when the dropdown opens.
 *  - It sits at z-index var(--vsc-z-dropdown) - 1, so below the menu panel.
 *  - Clicking the backdrop calls close() (identical to an outside-click).
 *  - `backdropDim` adds a semi-transparent dark tint (useful for modal-style dropdowns).
 *  - The backdrop is removed from the DOM as soon as the dropdown closes.
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
  @Input() isOpen = false;
  @Output() isOpenChange = new EventEmitter<boolean>();
  @Output() closed = new EventEmitter<void>();

  /**
   * When true, a transparent backdrop element is injected into document.body
   * while the dropdown is open. Clicking it closes the dropdown.
   * Defaults to false — existing dropdowns are unaffected unless opted in.
   */
  backdrop = true;

  /**
   * When true (requires backdrop=true), the backdrop has a semi-transparent
   * dark tint, visually dimming everything behind the menu.
   */
  @Input() backdropDim = false;

  @ContentChild(DropdownTriggerDirective) triggerDir?: DropdownTriggerDirective;
  @ContentChild(DropdownMenuDirective) menuDir?: DropdownMenuDirective;

  private dropdownId!: string;
  private lastMenuEl: HTMLElement | null = null;
  private backdropEl: HTMLElement | null = null;

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

  // ── Backdrop ──────────────────────────────────────────────────────────────

  private attachBackdrop(): void {
    if (!this.backdrop || this.backdropEl) return;

    const el = document.createElement('div');
    el.setAttribute('data-dropdown-backdrop', this.dropdownId);

    Object.assign(el.style, {
      position:        'fixed',
      inset:           '0',
      zIndex:          'calc(var(--vsc-z-dropdown, 1000) - 1)',
      // Transparent by default; dim mode adds a subtle tint
      background:      this.backdropDim
                         ? 'rgba(0, 0, 0, 0.32)'
                         : 'transparent',
      // Animate opacity when dimming so it doesn't snap in harshly
      transition:      this.backdropDim ? 'opacity 0.12s ease' : 'none',
    });

    // Clicking the backdrop is treated as an outside click → close
    el.addEventListener('mousedown', () => this.close());

    document.body.appendChild(el);
    this.backdropEl = el;
  }

  private detachBackdrop(): void {
    if (!this.backdropEl) return;
    this.backdropEl.remove();
    this.backdropEl = null;
  }

  // ── Open / close / toggle ─────────────────────────────────────────────────

  open(): void {
    this.dropdownService.closeOthers(this.dropdownId);
    this.isOpen = true;
    this.isOpenChange.emit(true);
    this.attachBackdrop();
  }

  close(): void {
    if (!this.isOpen) return;
    if (this.dropdownService.isInteractionPending(this.dropdownId)) return;
    this.isOpen = false;
    this.isOpenChange.emit(false);
    this.closed.emit();
    this.detachBackdrop();
  }

  toggle(): void {
    this.isOpen ? this.close() : this.open();
  }

  ngOnDestroy(): void {
    this.detachBackdrop();
    this.dropdownService.unregister(this.dropdownId);
  }
}