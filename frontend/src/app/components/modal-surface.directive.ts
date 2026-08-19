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
import { ModalStackService } from '../services/modal-stack.service';

let modalSequence = 0;

@Directive({ selector: '[ModalSurface]', standalone: true })
export class ModalSurfaceDirective implements AfterViewInit, OnDestroy {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly modalStack = inject(ModalStackService);

  @Input() ModalInitial: string | null = null;
  @Input() ModalDismissible = true;
  @Output() ModalDismiss = new EventEmitter<void>();

  private readonly modalId = `modal-${++modalSequence}`;
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
    this.unregisterModal = this.modalStack.register({
      id: this.modalId,
      element: this.host.nativeElement,
      dismiss: () => this.ModalDismiss.emit(),
      dismissible: () => this.ModalDismissible,
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

      if (this.ModalDismissible) {
        this.ModalDismiss.emit();
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

      focusInitialElement(this.host.nativeElement, this.ModalInitial);

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