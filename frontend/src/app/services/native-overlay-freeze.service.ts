import { Injectable } from '@angular/core';

/**
 * Ref-counted freeze for native browser overlays.
 *
 * Any component that needs to draw HTML on top of the native Win32 browser
 * pane (dialogs, dropdowns, filter widgets, etc.) should acquire a freeze
 * before showing and release it on close.  While at least one freeze is
 * held the body gets a `native-overlay-frozen` class which tells
 * `FileViewContainerComponent` to replace the live overlay with a canvas
 * screenshot.
 */
@Injectable({ providedIn: 'root' })
export class NativeOverlayFreezeService {
  private holders = new Set<string>();

  get frozen(): boolean {
    return this.holders.size > 0;
  }

  acquire(key: string): void {
    if (this.holders.has(key)) {
      return;
    }
    this.holders.add(key);
    this.sync();
  }

  release(key: string): void {
    if (!this.holders.delete(key)) {
      return;
    }
    this.sync();
  }

  private sync(): void {
    if (typeof document === 'undefined' || typeof window === 'undefined') {
      return;
    }
    if (this.holders.size > 0) {
      document.body.classList.add('native-overlay-frozen');
    } else {
      document.body.classList.remove('native-overlay-frozen');
    }
    window.dispatchEvent(new CustomEvent('native-overlay-freeze-change'));
  }
}