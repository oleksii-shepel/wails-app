import {
  AfterViewInit,
  Directive,
  ElementRef,
  EventEmitter,
  HostListener,
  Input,
  OnDestroy,
  Output,
  inject,
} from '@angular/core';

import {
  focusInitialElement,
  isFocusableElement,
  trapFocusWithin,
} from '../directives/focus-trap.utils';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { ReassemblyModalStackService } from '../services/reassembly-modal-stack.service';

let modalSequence = 0;

@Directive({ selector: '[reassemblyModalSurface]', standalone: true })
export class ReassemblyModalSurfaceDirective implements AfterViewInit, OnDestroy {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly freeze = inject(NativeOverlayFreezeService);
  private readonly modalStack = inject(ReassemblyModalStackService);

  @Input() reassemblyModalInitial: string | null = null;
  @Input() reassemblyModalDismissible = true;
  @Output() reassemblyModalDismiss = new EventEmitter<void>();

  private readonly modalId = `reassembly-modal-${++modalSequence}`;
  private readonly freezeKey = this.modalId;
  private previousFocus: HTMLElement | null = null;
  private initialFocusTimer: ReturnType<typeof setTimeout> | null = null;
  private unregisterModal: (() => void) | null = null;
  private destroyed = false;

  ngAfterViewInit(): void {
    const active = this.host.nativeElement.ownerDocument.activeElement;
    this.previousFocus =
      active instanceof HTMLElement && !this.host.nativeElement.contains(active)
        ? active
        : null;

    this.freeze.acquire(this.freezeKey);
    this.unregisterModal = this.modalStack.register({
      id: this.modalId,
      element: this.host.nativeElement,
      dismiss: () => this.reassemblyModalDismiss.emit(),
      dismissible: () => this.reassemblyModalDismissible,
    });
    this.scheduleInitialFocus();
  }

  ngOnDestroy(): void {
    this.destroyed = true;

    if (this.initialFocusTimer !== null) {
      clearTimeout(this.initialFocusTimer);
      this.initialFocusTimer = null;
    }

    this.unregisterModal?.();
    this.unregisterModal = null;
    this.freeze.release(this.freezeKey);

    const target = this.previousFocus;
    this.previousFocus = null;
    queueMicrotask(() => {
      if (target?.isConnected && isFocusableElement(target)) {
        target.focus({ preventScroll: true });
      }
    });
  }

  @HostListener('mousedown')
  onMouseDown(): void {
    this.modalStack.bringToFront(this.modalId);
  }

  @HostListener('keydown', ['$event'])
  onKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      const topmost = this.modalStack.topmost();
      if (topmost?.id !== this.modalId) return;

      event.preventDefault();
      event.stopPropagation();

      if (this.reassemblyModalDismissible) {
        this.reassemblyModalDismiss.emit();
      }
      return;
    }

    trapFocusWithin(this.host.nativeElement, event);
  }

  private scheduleInitialFocus(attempt = 0): void {
    if (this.initialFocusTimer !== null) clearTimeout(this.initialFocusTimer);

    this.initialFocusTimer = setTimeout(() => {
      this.initialFocusTimer = null;
      if (this.destroyed) return;

      focusInitialElement(this.host.nativeElement, this.reassemblyModalInitial);

      const active = this.host.nativeElement.ownerDocument.activeElement;
      if (
        attempt < 3 &&
        !(active instanceof Node && this.host.nativeElement.contains(active))
      ) {
        this.scheduleInitialFocus(attempt + 1);
      }
    }, attempt === 0 ? 0 : 16);
  }
}
