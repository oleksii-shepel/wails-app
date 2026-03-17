import { Directive, ElementRef, NgZone, OnDestroy, OnInit, Renderer2 } from '@angular/core';

@Directive({
  selector: '[appVscodeScrollbar]',
  standalone: true,
})
export class VscodeScrollbarDirective implements OnInit, OnDestroy {
  private readonly host: HTMLElement;
  private readonly anchor: HTMLElement;
  private scrollbarEl: HTMLElement | null = null;
  private thumbEl: HTMLElement | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private removeScrollListener: (() => void) | null = null;
  private removeMouseEnterListener: (() => void) | null = null;
  private removeMouseLeaveListener: (() => void) | null = null;
  private removeThumbPointerDownListener: (() => void) | null = null;
  private removeTrackPointerDownListener: (() => void) | null = null;
  private removeWindowPointerMoveListener: (() => void) | null = null;
  private removeWindowPointerUpListener: (() => void) | null = null;
  private showTimeout: ReturnType<typeof setTimeout> | null = null;
  private dragging = false;
  private dragStartY = 0;
  private dragStartScrollTop = 0;

  constructor(
    elementRef: ElementRef<HTMLElement>,
    private renderer: Renderer2,
    private ngZone: NgZone,
  ) {
    this.host = elementRef.nativeElement;
    this.anchor = this.host.parentElement ?? this.host;
  }

  ngOnInit(): void {
    this.renderer.addClass(this.host, 'vsc-scroll-host');
    this.renderer.addClass(this.anchor, 'vsc-scroll-anchor');
    this.createScrollbar();
    this.bindEvents();
    this.updateScrollbar();
  }

  ngOnDestroy(): void {
    this.clearShowTimeout();
    this.resizeObserver?.disconnect();
    this.removeScrollListener?.();
    this.removeMouseEnterListener?.();
    this.removeMouseLeaveListener?.();
    this.removeThumbPointerDownListener?.();
    this.removeTrackPointerDownListener?.();
    this.removeWindowPointerMoveListener?.();
    this.removeWindowPointerUpListener?.();
  }

  private createScrollbar(): void {
    const scrollbar = this.renderer.createElement('div');
    const thumb = this.renderer.createElement('div');

    this.renderer.addClass(scrollbar, 'vsc-scrollbar');
    this.renderer.addClass(thumb, 'vsc-scrollbar-thumb');
    this.renderer.appendChild(scrollbar, thumb);
    this.renderer.appendChild(this.anchor, scrollbar);

    this.scrollbarEl = scrollbar;
    this.thumbEl = thumb;
  }

  private bindEvents(): void {
    this.ngZone.runOutsideAngular(() => {
      this.removeScrollListener = this.renderer.listen(this.host, 'scroll', () => {
        this.updateScrollbar();
        this.showScrollbarTemporarily();
      });

      this.removeMouseEnterListener = this.renderer.listen(this.host, 'mouseenter', () => {
        this.renderer.addClass(this.anchor, 'vsc-scroll-anchor--hover');
        this.updateScrollbar();
        this.showScrollbarTemporarily();
      });

      this.removeMouseLeaveListener = this.renderer.listen(this.host, 'mouseleave', () => {
        if (!this.dragging) this.renderer.removeClass(this.anchor, 'vsc-scroll-anchor--hover');
      });

      if (this.thumbEl) {
        this.removeThumbPointerDownListener = this.renderer.listen(this.thumbEl, 'pointerdown', (event: PointerEvent) => {
          if (!this.thumbEl) return;
          event.preventDefault();
          this.dragging = true;
          this.dragStartY = event.clientY;
          this.dragStartScrollTop = this.host.scrollTop;
          this.renderer.addClass(this.anchor, 'vsc-scroll-anchor--dragging');
          this.thumbEl.setPointerCapture?.(event.pointerId);
        });
      }

      if (this.scrollbarEl) {
        this.removeTrackPointerDownListener = this.renderer.listen(this.scrollbarEl, 'pointerdown', (event: PointerEvent) => {
          if (!this.thumbEl || event.target === this.thumbEl) return;
          const rect = this.scrollbarEl!.getBoundingClientRect();
          const clickOffset = event.clientY - rect.top;
          const thumbHeight = this.thumbEl.offsetHeight;
          const maxOffset = Math.max(0, rect.height - thumbHeight);
          const nextOffset = Math.min(Math.max(0, clickOffset - thumbHeight / 2), maxOffset);
          const maxScroll = Math.max(0, this.host.scrollHeight - this.host.clientHeight);
          this.host.scrollTop = maxOffset > 0 ? (nextOffset / maxOffset) * maxScroll : 0;
          this.updateScrollbar();
          this.showScrollbarTemporarily();
        });
      }

      this.removeWindowPointerMoveListener = this.renderer.listen('window', 'pointermove', (event: PointerEvent) => {
        if (!this.dragging || !this.scrollbarEl || !this.thumbEl) return;
        const trackHeight = this.scrollbarEl.clientHeight;
        const thumbHeight = this.thumbEl.offsetHeight;
        const maxThumbOffset = Math.max(0, trackHeight - thumbHeight);
        const maxScroll = Math.max(0, this.host.scrollHeight - this.host.clientHeight);
        if (maxThumbOffset <= 0 || maxScroll <= 0) return;

        const deltaY = event.clientY - this.dragStartY;
        const scrollDelta = (deltaY / maxThumbOffset) * maxScroll;
        this.host.scrollTop = this.dragStartScrollTop + scrollDelta;
        this.updateScrollbar();
      });

      this.removeWindowPointerUpListener = this.renderer.listen('window', 'pointerup', () => {
        if (!this.dragging) return;
        this.dragging = false;
        this.renderer.removeClass(this.anchor, 'vsc-scroll-anchor--dragging');
        this.renderer.removeClass(this.anchor, 'vsc-scroll-anchor--hover');
        this.showScrollbarTemporarily();
      });

      if (typeof ResizeObserver !== 'undefined') {
        this.resizeObserver = new ResizeObserver(() => this.updateScrollbar());
        this.resizeObserver.observe(this.host);
        if (this.anchor !== this.host) this.resizeObserver.observe(this.anchor);
      }
    });
  }

  private updateScrollbar(): void {
    if (!this.scrollbarEl || !this.thumbEl) return;

    const clientHeight = this.host.clientHeight;
    const scrollHeight = this.host.scrollHeight;
    const maxScroll = Math.max(0, scrollHeight - clientHeight);

    if (clientHeight <= 0 || maxScroll <= 0) {
      this.renderer.setStyle(this.scrollbarEl, 'display', 'none');
      return;
    }

    this.renderer.setStyle(this.scrollbarEl, 'display', 'block');

    const hostRect = this.host.getBoundingClientRect();
    const anchorRect = this.anchor.getBoundingClientRect();
    const scrollbarWidth = 12;
    const top = Math.max(0, hostRect.top - anchorRect.top + 2);
    const left = Math.max(0, hostRect.left - anchorRect.left + hostRect.width - scrollbarWidth - 2);
    const height = Math.max(0, hostRect.height - 4);

    this.renderer.setStyle(this.scrollbarEl, 'top', `${Math.round(top)}px`);
    this.renderer.setStyle(this.scrollbarEl, 'left', `${Math.round(left)}px`);
    this.renderer.setStyle(this.scrollbarEl, 'height', `${Math.round(height)}px`);
    this.renderer.removeStyle(this.scrollbarEl, 'right');
    this.renderer.removeStyle(this.scrollbarEl, 'bottom');

    const trackHeight = this.scrollbarEl.clientHeight;
    const thumbHeight = Math.max(20, Math.round((clientHeight / scrollHeight) * trackHeight));
    const maxThumbOffset = Math.max(0, trackHeight - thumbHeight);
    const thumbOffset = maxScroll > 0 ? (this.host.scrollTop / maxScroll) * maxThumbOffset : 0;

    this.renderer.setStyle(this.thumbEl, 'height', `${thumbHeight}px`);
    this.renderer.setStyle(this.thumbEl, 'transform', `translateY(${Math.round(thumbOffset)}px)`);
  }

  private showScrollbarTemporarily(): void {
    this.renderer.addClass(this.anchor, 'vsc-scroll-anchor--visible');
    this.clearShowTimeout();
    this.showTimeout = setTimeout(() => {
      if (!this.dragging) this.renderer.removeClass(this.anchor, 'vsc-scroll-anchor--visible');
    }, 700);
  }

  private clearShowTimeout(): void {
    if (this.showTimeout) {
      clearTimeout(this.showTimeout);
      this.showTimeout = null;
    }
  }
}