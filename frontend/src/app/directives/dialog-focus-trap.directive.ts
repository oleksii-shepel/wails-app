import { AfterViewInit, Directive, ElementRef, HostListener, Input, NgZone, OnChanges, OnDestroy, inject, Injectable } from '@angular/core';
import { focusInitialElement, isFocusableElement, trapFocusWithin } from './focus-trap.utils';

@Injectable({ providedIn: 'root' })
class DialogFocusTrapStackService {
  private stack: HTMLElement[] = [];

  push(container: HTMLElement): () => void {
    this.stack = this.stack.filter(item => item !== container);
    this.stack.push(container);
    return () => {
      this.stack = this.stack.filter(item => item !== container);
    };
  }

  isTop(container: HTMLElement): boolean {
    return this.stack[this.stack.length - 1] === container;
  }
}

@Directive({
  selector: '[appTrapFocus]',
  standalone: true,
})
export class DialogFocusTrapDirective implements AfterViewInit, OnChanges, OnDestroy {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly zone = inject(NgZone);
  private readonly stack = inject(DialogFocusTrapStackService);

  @Input('appTrapFocus') enabled = true;
  @Input() appTrapFocusInitial?: string | null;
  @Input() appTrapFocusRestoreFocus = true;

  private active = false;
  private releaseStack: (() => void) | null = null;
  private cleanupFocusListener: (() => void) | null = null;
  private previousFocus: HTMLElement | null = null;
  private focusTimer: ReturnType<typeof setTimeout> | null = null;
  private viewReady = false;

  ngAfterViewInit(): void {
    this.viewReady = true;
    this.syncTrapState();
  }

  ngOnChanges(): void {
    this.syncTrapState();
  }

  ngOnDestroy(): void {
    this.deactivate();
    this.viewReady = false;
  }

  @HostListener('keydown', ['$event'])
  onKeyDown(event: KeyboardEvent): void {
    if (!this.active || !this.stack.isTop(this.host.nativeElement)) {
      return;
    }

    trapFocusWithin(this.host.nativeElement, event);
  }

  private syncTrapState(): void {
    if (!this.viewReady) {
      return;
    }

    if (this.enabled) {
      this.activate();
      return;
    }

    this.deactivate();
  }

  private activate(): void {
    if (this.active) {
      return;
    }

    this.active = true;
    const activeElement = this.host.nativeElement.ownerDocument.activeElement;
    if (activeElement instanceof HTMLElement && !this.host.nativeElement.contains(activeElement)) {
      this.previousFocus = activeElement;
    }

    this.releaseStack = this.stack.push(this.host.nativeElement);
    this.installFocusListener();
    this.scheduleInitialFocus();
  }

  private deactivate(): void {
    if (!this.active) {
      return;
    }

    this.active = false;
    this.clearScheduledFocus();
    this.cleanupFocusListener?.();
    this.cleanupFocusListener = null;
    this.releaseStack?.();
    this.releaseStack = null;

    if (!this.appTrapFocusRestoreFocus) {
      this.previousFocus = null;
      return;
    }

    const target = this.previousFocus;
    this.previousFocus = null;
    if (!target) {
      return;
    }

    this.zone.runOutsideAngular(() => {
      setTimeout(() => {
        if (target.isConnected && isFocusableElement(target)) {
          target.focus();
        }
      }, 0);
    });
  }

  private installFocusListener(): void {
    const ownerDocument = this.host.nativeElement.ownerDocument;
    const handleFocusIn = (event: FocusEvent) => {
      if (!this.active || !this.stack.isTop(this.host.nativeElement)) {
        return;
      }

      const target = event.target;
      if (target instanceof Node && this.host.nativeElement.contains(target)) {
        return;
      }

      this.scheduleInitialFocus();
    };

    this.zone.runOutsideAngular(() => {
      ownerDocument.addEventListener('focusin', handleFocusIn, true);
    });

    this.cleanupFocusListener = () => {
      ownerDocument.removeEventListener('focusin', handleFocusIn, true);
    };
  }

  private scheduleInitialFocus(): void {
    this.clearScheduledFocus();
    this.zone.runOutsideAngular(() => {
      this.focusTimer = setTimeout(() => {
        this.focusTimer = null;
        if (!this.active || !this.stack.isTop(this.host.nativeElement)) {
          return;
        }

        focusInitialElement(this.host.nativeElement, this.appTrapFocusInitial);
      }, 0);
    });
  }

  private clearScheduledFocus(): void {
    if (this.focusTimer === null) {
      return;
    }

    clearTimeout(this.focusTimer);
    this.focusTimer = null;
  }
}