import { AfterViewInit, Directive, ElementRef, HostListener, Input, OnDestroy, inject } from '@angular/core';
import { focusInitialElement, isFocusableElement, trapFocusWithin } from '../directives/focus-trap.utils';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';

let modalSequence = 0;

@Directive({ selector: '[reassemblyModalSurface]', standalone: true })
export class ReassemblyModalSurfaceDirective implements AfterViewInit, OnDestroy {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly freeze = inject(NativeOverlayFreezeService);

  @Input() reassemblyModalInitial: string | null = null;

  private readonly freezeKey = `reassembly-modal-${++modalSequence}`;
  private previousFocus: HTMLElement | null = null;
  private initialFocusTimer: ReturnType<typeof setTimeout> | null = null;
  private destroyed = false;

  ngAfterViewInit(): void {
    const active = this.host.nativeElement.ownerDocument.activeElement;
    this.previousFocus =
      active instanceof HTMLElement && !this.host.nativeElement.contains(active)
        ? active
        : null;

    this.freeze.acquire(this.freezeKey);
    this.scheduleInitialFocus();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    if (this.initialFocusTimer !== null) {
      clearTimeout(this.initialFocusTimer);
      this.initialFocusTimer = null;
    }

    this.freeze.release(this.freezeKey);

    const target = this.previousFocus;
    this.previousFocus = null;
    queueMicrotask(() => {
      if (target?.isConnected && isFocusableElement(target)) {
        target.focus({ preventScroll: true });
      }
    });
  }

  @HostListener('keydown', ['$event'])
  onKeyDown(event: KeyboardEvent): void {
    // Shared modal policy owns Tab traversal only.
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
