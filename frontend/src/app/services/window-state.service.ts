import { Injectable, NgZone, OnDestroy } from '@angular/core';
import { GetPreferences, SavePreferences } from 'wailsjs/go/app/App';
import {
  WindowGetSize,
  WindowGetPosition,
  WindowIsMaximised,
} from 'wailsjs/runtime';
import type { WindowState } from '../shared/protocol-enhanced';

interface BackendUserPreferences {
  windowState?: WindowState;
  [key: string]: unknown;
}

/**
 * Captures window geometry (size, position, maximised state) and persists it
 * via backend `UserPreferences`. Debounced so resize/move drags don't flood IPC.
 *
 * Restoration of size+maximised happens at backend bootstrap before wails.Run().
 * Restoration of position happens in the Wails OnStartup hook (also backend).
 * This service only handles the *capture* side from the frontend.
 */
@Injectable({ providedIn: 'root' })
export class WindowStateService implements OnDestroy {
  private debounceTimer: ReturnType<typeof setTimeout> | null = null;
  private cachedPreferences: BackendUserPreferences | null = null;
  private readonly captureHandler = () => this.scheduleCapture();
  private started = false;

  constructor(private readonly ngZone: NgZone) {}

  /** Start listening for resize / visibility changes. Idempotent. */
  start(): void {
    if (this.started) return;
    this.started = true;
    this.ngZone.runOutsideAngular(() => {
      window.addEventListener('resize', this.captureHandler, { passive: true });
      window.addEventListener('beforeunload', this.captureHandler);
    });
  }

  ngOnDestroy(): void {
    window.removeEventListener('resize', this.captureHandler);
    window.removeEventListener('beforeunload', this.captureHandler);
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = null;
    }
  }

  /** Force an immediate capture (e.g. before close). */
  async captureNow(): Promise<void> {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = null;
    }
    await this.captureAndPersist();
  }

  private scheduleCapture(): void {
    if (this.debounceTimer) clearTimeout(this.debounceTimer);
    this.debounceTimer = setTimeout(() => {
      this.debounceTimer = null;
      void this.captureAndPersist();
    }, 400);
  }

  private async captureAndPersist(): Promise<void> {
    let snapshot: WindowState;
    try {
      const [size, position, maximised] = await Promise.all([
        WindowGetSize(),
        WindowGetPosition(),
        WindowIsMaximised(),
      ]);
      snapshot = {
        width: Math.round(size.w),
        height: Math.round(size.h),
        x: Math.round(position.x),
        y: Math.round(position.y),
        maximised: !!maximised,
      };
    } catch {
      return; // Wails runtime not yet attached.
    }

    if (snapshot.width <= 0 || snapshot.height <= 0) return;

    try {
      const current = this.cachedPreferences ?? (await GetPreferences() as unknown as BackendUserPreferences);
      this.cachedPreferences = current;

      if (this.isSameWindowState(current.windowState, snapshot)) return;

      await SavePreferences({
        ...current,
        windowState: snapshot,
      } as any);
      this.cachedPreferences = { ...current, windowState: snapshot };
    } catch {
      // Ignore backend errors during early startup or shutdown races.
    }
  }

  private isSameWindowState(a: WindowState | undefined, b: WindowState): boolean {
    if (!a) return false;
    return (
      a.width === b.width &&
      a.height === b.height &&
      a.x === b.x &&
      a.y === b.y &&
      a.maximised === b.maximised
    );
  }
}
