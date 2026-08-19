import {
  Component, Input, OnChanges, OnDestroy, AfterViewInit,
  SimpleChanges, ViewChild, ElementRef, NgZone, HostBinding,
  ChangeDetectionStrategy,
} from '@angular/core';
import { NativeBrowserPaneService } from '../services/native-browser-pane.service';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';

/**
 * Manages a native Win32 browser overlay that tracks its host element position.
 *
 * The overlay is automatically replaced with a canvas screenshot when:
 *  - The split-divider is hovered / dragged (`divider-hover`, `divider-dragging`)
 *  - A modal session is active (`native-overlay-frozen` on body)
 *
 * Usage:
 * ```html
 * <app-native-overlay
 *   [surfaceId]="surfaceId"
 *   [url]="currentPath"
 *   [active]="surfaceActive">
 * </app-native-overlay>
 * ```
 */
@Component({
  selector: 'app-native-overlay',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div #host class="native-overlay-host">
      <div #backdrop class="native-overlay-backdrop"></div>
      <canvas #screenshotCanvas class="native-overlay-canvas"></canvas>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      width: 100%;
      height: 100%;
      position: relative;
      flex: 1 1 auto;
      min-width: 0;
      overflow: hidden;
    }

    .native-overlay-host {
      width: 100%;
      height: 100%;
      position: relative;
      z-index: var(--vsc-z-native-overlay);
    }

    .native-overlay-backdrop {
      position: absolute;
      inset: 0;
    }

    .native-overlay-canvas {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
      display: none;
      z-index: 1;
      pointer-events: none;
      image-rendering: auto;
    }
  `],
})
export class NativeOverlayComponent implements OnChanges, OnDestroy, AfterViewInit {
  @HostBinding('style.backgroundColor') hostBackgroundColor = 'var(--vsc-on-accent)';
  @ViewChild('host') private hostEl?: ElementRef<HTMLElement>;
  @ViewChild('backdrop') private backdropEl?: ElementRef<HTMLElement>;
  @ViewChild('screenshotCanvas') private canvasEl?: ElementRef<HTMLCanvasElement>;

  @Input() surfaceId = '';
  @Input() url = '';
  @Input() active = true;

  // ── Sync state ──────────────────────────────────────────────

  private syncTimer: ReturnType<typeof setInterval> | null = null;
  private syncFrame: number | null = null;
  private syncInFlight = false;
  private syncQueued = false;
  private lastSyncKey = '';

  // ── Screenshot / freeze state ───────────────────────────────

  private wasFrozen = false;
  private capturingScreenshot = false;
  private screenshotVisible = false;
  private screenshotRequestId = 0;
  private lastScreenshotWidth = 0;
  private lastScreenshotHeight = 0;
  private lastScreenshotURL = '';
  private subpixelOffsetX = 0;
  private subpixelOffsetY = 0;
  private hoverFreezeActive = false;
  private documentBackgroundColor: string | null = null;
  private documentBackgroundRequestInFlight = false;
  private lastDocumentBackgroundURL = '';
  private lastDocumentBackgroundRequestAt = 0;
  private screenshotPending = false;
  private forceFreshScreenshot = false;
  private screenshotCooldownTimer: ReturnType<typeof setTimeout> | null = null;
  private lastScreenshotStartedAt = 0;
  private readonly screenshotMinInterval = 50;
  private readonly screenshotPrewarmDelayMs = 16;
  private readonly documentBackgroundRetryMs = 350;
  private readonly documentBackgroundRefreshMs = 2000;
  private readonly frozenCanvasBackgroundToken = '--vsc-on-accent';

  private get frozenCanvasBackground(): string {
    return getComputedStyle(this.hostEl?.nativeElement ?? document.documentElement)
      .getPropertyValue(this.frozenCanvasBackgroundToken)
      .trim();
  }
  private liveBackgroundColor = 'var(--vsc-on-accent)';
  private documentBackgroundRequestToken = 0;
  private screenshotPrewarmTimer: ReturnType<typeof setTimeout> | null = null;

  private measureHostBounds(host: HTMLElement): {
    rect: DOMRect;
    scale: number;
    x: number;
    y: number;
    width: number;
    height: number;
    cssWidth: number;
    cssHeight: number;
    subpixelOffsetX: number;
    subpixelOffsetY: number;
  } {
    const rect = host.getBoundingClientRect();
    const scale = (typeof window !== 'undefined' && Number.isFinite(window.devicePixelRatio) && window.devicePixelRatio > 0)
      ? window.devicePixelRatio
      : 1;
    const left = Math.round(rect.left * scale);
    const top = Math.round(rect.top * scale);
    const right = Math.round(rect.right * scale);
    const bottom = Math.round(rect.bottom * scale);
    const width = Math.max(0, right - left);
    const height = Math.max(0, bottom - top);

    return {
      rect,
      scale,
      x: left,
      y: top,
      width,
      height,

      // To make the captured image perfectly align back into CSS DOM space:
      cssWidth: width / scale,
      cssHeight: height / scale,
      subpixelOffsetX: (rect.left * scale - left) / scale,
      subpixelOffsetY: (rect.top * scale - top) / scale,
    };
  }

  // ── Event handlers ──────────────────────────────────────────

  private readonly onDividerStateChange = () => {
    this.updateHoverFreezeState();
    if (this.active && this.isFreezeRequested()) {
      this.revealCachedCanvasIfAvailable();
    }
    this.refreshTracking();
    this.requestSync();
  };

  private readonly onFreezeChange = () => {
    this.refreshTracking();
    this.requestSync();
  };

  private readonly onDividerMove = () => {
    if (!this.active || !document.body.classList.contains('divider-dragging')) {
      return;
    }
    this.scheduleFrameSync();
    this.requestScreenshotRefresh();
  };

  constructor(
    private nativeBrowserPane: NativeBrowserPaneService,
    private zone: NgZone,
    private overlayFreeze: NativeOverlayFreezeService,
  ) {}

  // ── Lifecycle ───────────────────────────────────────────────

  ngAfterViewInit(): void {
    window.addEventListener('file-manager-divider-statechange', this.onDividerStateChange);
    window.addEventListener('file-manager-divider-move', this.onDividerMove);
    window.addEventListener('native-overlay-freeze-change', this.onFreezeChange);
    this.refresh();
  }

  ngOnChanges(changes: SimpleChanges): void {
    const urlChanged = !!changes['url'];
    const surfaceChanged = !!changes['surfaceId'];

    if (urlChanged || surfaceChanged) {
      this.documentBackgroundRequestToken++;
      this.screenshotRequestId++;
      this.lastSyncKey = '';
      this.documentBackgroundColor = null;
      this.documentBackgroundRequestInFlight = false;
      this.lastDocumentBackgroundURL = '';
      this.lastDocumentBackgroundRequestAt = 0;

      const hasCachedScreenshot = this.screenshotVisible || this.hasCachedCanvas();
      const preserveScreenshotDuringSwap = urlChanged && !surfaceChanged && hasCachedScreenshot;

      if (preserveScreenshotDuringSwap) {
        this.wasFrozen = this.isFreezeRequested();
        this.revealCachedCanvasIfAvailable();
        this.screenshotPending = true;
        this.forceFreshScreenshot = true;
        this.lastScreenshotStartedAt = 0;
      } else {
        this.wasFrozen = false;
        this.hoverFreezeActive = false;
        this.hideCanvas(true);
      }
    }
    if (changes['active'] && this.active) {
      this.lastSyncKey = '';
      this.lastDocumentBackgroundRequestAt = 0;
    }
    if (urlChanged || changes['active'] || surfaceChanged) {
      this.refresh();
    }
  }

  ngOnDestroy(): void {
    window.removeEventListener('file-manager-divider-statechange', this.onDividerStateChange);
    window.removeEventListener('file-manager-divider-move', this.onDividerMove);
    window.removeEventListener('native-overlay-freeze-change', this.onFreezeChange);
    this.stopTracking();
    void this.nativeBrowserPane.dispose(this.surfaceId);
  }

  // ── Public ──────────────────────────────────────────────────

  /** The underlying host element (for external getBoundingClientRect, etc.). */
  get hostElement(): HTMLElement | undefined {
    return this.hostEl?.nativeElement;
  }

  // ── Internal: refresh / tracking ────────────────────────────

  private refresh(): void {
    if (!this.active) {
      this.documentBackgroundRequestToken++;
      this.screenshotRequestId++;
      this.wasFrozen = false;
      this.hoverFreezeActive = false;
      this.lastSyncKey = '';
      this.hideCanvas(true);
      this.stopTracking();
      this.requestSync();
      return;
    }
    this.refreshTracking();
    this.requestSync();
  }

  private refreshTracking(): void {
    if (!this.active) {
      this.stopTracking();
      return;
    }
    if (document.body.classList.contains('divider-dragging')) {
      this.stopTimer();
      return;
    }
    this.startTimer();
  }

  private startTimer(): void {
    if (this.syncTimer) {
      return;
    }
    this.zone.runOutsideAngular(() => {
      this.syncTimer = setInterval(() => {
        this.requestSync();
      }, 100);
    });
  }

  private stopTracking(): void {
    this.stopTimer();
    this.clearScreenshotCooldown();
    this.clearScreenshotPrewarm();
    if (this.syncFrame !== null) {
      cancelAnimationFrame(this.syncFrame);
      this.syncFrame = null;
    }
    this.syncQueued = false;
  }

  private stopTimer(): void {
    if (!this.syncTimer) {
      return;
    }
    clearInterval(this.syncTimer);
    this.syncTimer = null;
  }

  private scheduleFrameSync(): void {
    if (this.syncFrame !== null) {
      return;
    }
    this.syncFrame = requestAnimationFrame(() => {
      this.syncFrame = null;
      this.requestSync();
    });
  }

  // ── Internal: sync loop ─────────────────────────────────────

  private requestSync(): void {
    if (this.syncInFlight) {
      this.syncQueued = true;
      return;
    }
    this.syncInFlight = true;
    void this.sync()
      .finally(() => {
        this.syncInFlight = false;
        if (this.syncQueued) {
          this.syncQueued = false;
          this.requestSync();
          return;
        }
        if (this.screenshotPending && this.isFreezeRequested()) {
          this.scheduleScreenshotCooldown(16);
        }
      });
  }

  private async sync(): Promise<void> {
    if (!this.surfaceId) {
      return;
    }

    const theme = this.resolveTheme();
    const backgroundColor = this.documentBackgroundColor ?? this.resolveBackgroundColor(theme);
    this.liveBackgroundColor = backgroundColor;
    this.applyBackgroundColor(this.screenshotVisible ? this.frozenCanvasBackground : backgroundColor);

    // ── Freeze transitions ────────────────────────────────────
    const frozen = this.isFreezeRequested();

    if (frozen && !this.wasFrozen && this.active) {
      this.wasFrozen = true;
      void this.prepareScreenshot();
    }

    if (!frozen && this.wasFrozen) {
      this.wasFrozen = false;
      this.hoverFreezeActive = false;
      
      const expectedId = ++this.screenshotRequestId;
      // Delay hiding the canvas slightly so the native Win32 window has time
      // to paint on screen, eliminating the transparent flicker during unfreeze.
      setTimeout(() => {
        if (this.screenshotRequestId === expectedId && !this.wasFrozen) {
          this.hideCanvas(false);
        }
      }, 100);
    }

    // Hide native pane only after we have a screenshot to replace it.
    const freezeNativePane = this.shouldFreezeNativePane()
      && this.screenshotVisible;

    // ── Hidden ────────────────────────────────────────────────
    if (!this.active) {
      this.lastSyncKey = '';
      await this.nativeBrowserPane.sync({
        surfaceId: this.surfaceId,
        url: this.url,
        theme,
        backgroundColor,
        visible: false,
        x: 0, y: 0, width: 0, height: 0,
      });
      return;
    }

    // ── Measure host element ──────────────────────────────────
    const host = this.hostEl?.nativeElement;
    if (!host) {
      return;
    }

    const bounds = this.measureHostBounds(host);

    // Track sub-pixel offset between Win32 integer position and CSS element edge.
    this.subpixelOffsetX = bounds.subpixelOffsetX;
    this.subpixelOffsetY = bounds.subpixelOffsetY;
    if (bounds.width <= 0 || bounds.height <= 0) {
      return;
    }

    const request = {
      surfaceId: this.surfaceId,
      url: this.url,
      theme,
      backgroundColor,
      visible: !freezeNativePane,
      x: bounds.x,
      y: bounds.y,
      width: bounds.width,
      height: bounds.height,
    };
    const syncKey = JSON.stringify(request);
    if (syncKey === this.lastSyncKey) {
      this.scheduleScreenshotPrewarm();
      if (!frozen) {
        this.requestDocumentBackground(false, theme);
      }
      return;
    }

    try {
      if (await this.nativeBrowserPane.sync(request)) {
        this.lastSyncKey = syncKey;
        this.scheduleScreenshotPrewarm();
        if (!frozen) {
          this.requestDocumentBackground(false, theme);
        }
      }
    } catch (error) {
      void error;
    }
  }

  // ── Internal: freeze detection ──────────────────────────────

  private isDividerInteracting(): boolean {
    return document.body.classList.contains('divider-hover')
      || document.body.classList.contains('divider-dragging');
  }

  private isFreezeRequested(): boolean {
    return this.isDividerInteracting()
      || this.overlayFreeze.frozen;
  }

  private shouldFreezeNativePane(): boolean {
    return this.isFreezeRequested()
      || this.hoverFreezeActive;
  }

  private updateHoverFreezeState(): void {
    if (document.body.classList.contains('divider-dragging')) {
      this.hoverFreezeActive = true;
      return;
    }
    if (document.body.classList.contains('divider-hover')) {
      return;
    }
    this.hoverFreezeActive = false;
  }

  // ── Internal: screenshot capture ────────────────────────────

  private async prepareScreenshot(): Promise<void> {
    this.screenshotPending = true;
    this.forceFreshScreenshot = true;
    if (this.isFreezeRequested()) {
      this.revealCachedCanvasIfAvailable();
    }
    if (this.syncInFlight) {
      this.scheduleScreenshotCooldown(16);
      return;
    }
    this.maybeStartScreenshotCapture();
  }

  private async captureScreenshot(forceRefresh: boolean, allowWithoutFreeze = false, reveal = true): Promise<void> {
    if (!this.surfaceId || !this.active || this.capturingScreenshot || (!forceRefresh && this.screenshotVisible)) {
      return;
    }

    const requestId = ++this.screenshotRequestId;
    this.capturingScreenshot = true;
    try {
      const data = await this.nativeBrowserPane.captureScreenshot(this.surfaceId);
      if (!data || requestId !== this.screenshotRequestId || (!allowWithoutFreeze && !this.isFreezeRequested())) {
        return;
      }
      const drawn = await this.drawOnCanvas(data, requestId, reveal, allowWithoutFreeze);
      if (!drawn || requestId !== this.screenshotRequestId || (!allowWithoutFreeze && !this.isFreezeRequested())) {
        return;
      }
      if (reveal && !document.body.classList.contains('divider-dragging') && document.body.classList.contains('divider-hover')) {
        this.hoverFreezeActive = true;
      }
      this.lastSyncKey = '';
      if (reveal) {
        this.requestSync();
      }
    } catch (e) {
      void e;
    } finally {
      this.capturingScreenshot = false;
      if (this.isFreezeRequested() && (this.screenshotPending || this.shouldRefreshScreenshot())) {
        this.requestScreenshotRefresh();
      }
    }
  }

  private requestScreenshotRefresh(): void {
    if (!this.isFreezeRequested()) {
      return;
    }
    this.clearScreenshotPrewarm();
    this.screenshotPending = true;
    this.maybeStartScreenshotCapture();
  }

  private scheduleScreenshotPrewarm(): void {
    if (!this.active || !this.surfaceId || !this.url || this.isFreezeRequested() || this.capturingScreenshot) {
      this.clearScreenshotPrewarm();
      return;
    }
    if (this.lastScreenshotURL === this.url && this.hasCachedCanvas()) {
      this.clearScreenshotPrewarm();
      return;
    }
    if (this.screenshotPrewarmTimer !== null) {
      return;
    }
    this.screenshotPrewarmTimer = setTimeout(() => {
      this.screenshotPrewarmTimer = null;
      if (!this.active || !this.surfaceId || !this.url || this.isFreezeRequested() || this.capturingScreenshot) {
        return;
      }
      void this.captureScreenshot(true, true, false);
    }, this.screenshotPrewarmDelayMs);
  }

  private clearScreenshotPrewarm(): void {
    if (this.screenshotPrewarmTimer === null) {
      return;
    }
    clearTimeout(this.screenshotPrewarmTimer);
    this.screenshotPrewarmTimer = null;
  }

  private maybeStartScreenshotCapture(): void {
    if (!this.screenshotPending || !this.isFreezeRequested() || this.capturingScreenshot) {
      return;
    }
    if (this.syncInFlight) {
      this.scheduleScreenshotCooldown(16);
      return;
    }
    const elapsed = Date.now() - this.lastScreenshotStartedAt;
    if (elapsed < this.screenshotMinInterval) {
      this.scheduleScreenshotCooldown(this.screenshotMinInterval - elapsed);
      return;
    }
    if (this.screenshotVisible && !this.forceFreshScreenshot && !this.shouldRefreshScreenshot()) {
      this.screenshotPending = false;
      return;
    }
    this.clearScreenshotCooldown();
    this.screenshotPending = false;
    this.forceFreshScreenshot = false;
    this.lastScreenshotStartedAt = Date.now();
    void this.captureScreenshot(true);
  }

  private scheduleScreenshotCooldown(delay: number): void {
    if (this.screenshotCooldownTimer !== null) {
      return;
    }
    this.screenshotCooldownTimer = setTimeout(() => {
      this.screenshotCooldownTimer = null;
      this.maybeStartScreenshotCapture();
    }, Math.max(0, delay));
  }

  private clearScreenshotCooldown(): void {
    if (this.screenshotCooldownTimer === null) {
      return;
    }
    clearTimeout(this.screenshotCooldownTimer);
    this.screenshotCooldownTimer = null;
  }

  private shouldRefreshScreenshot(): boolean {
    if (!document.body.classList.contains('divider-dragging')) {
      return false;
    }
    const host = this.hostEl?.nativeElement;
    if (!host) {
      return false;
    }
    const rect = host.getBoundingClientRect();
    return Math.abs(rect.width - this.lastScreenshotWidth) >= 8
      || Math.abs(rect.height - this.lastScreenshotHeight) >= 8;
  }

  // ── Internal: canvas rendering ──────────────────────────────

  private drawOnCanvas(base64Data: string, requestId: number, reveal: boolean, allowWithoutFreeze: boolean): Promise<boolean> {
    const host = this.hostEl?.nativeElement;
    const canvas = this.canvasEl?.nativeElement;
    if (!canvas || !host) {
      return Promise.resolve(false);
    }
    const bounds = this.measureHostBounds(host);
    return this.decodeBitmap(base64Data).then((bitmap) => {
      if (!bitmap) {
        return false;
      }
      return new Promise<boolean>((resolve) => {
        if (requestId !== this.screenshotRequestId || (!allowWithoutFreeze && !this.isFreezeRequested())) {
          if ('close' in bitmap) {
            (bitmap as ImageBitmap).close();
          }
          resolve(false);
          return;
        }

        // Set backing-store to match the screenshot's native resolution.
        canvas.width = bitmap.width;
        canvas.height = bitmap.height;
        canvas.style.width = `${bounds.cssWidth}px`;
        canvas.style.height = `${bounds.cssHeight}px`;

        const ctx = canvas.getContext('2d', { alpha: false });
        if (!ctx) {
          if ('close' in bitmap) {
            (bitmap as ImageBitmap).close();
          }
          resolve(false);
          return;
        }
        ctx.globalCompositeOperation = 'source-over';
        ctx.fillStyle = this.frozenCanvasBackground;
        ctx.fillRect(0, 0, canvas.width, canvas.height);
        ctx.drawImage(bitmap, 0, 0);

        if ('close' in bitmap) {
          (bitmap as ImageBitmap).close();
        }

        if (reveal) {
          this.showCanvas();
          this.lastScreenshotWidth = bounds.rect.width;
          this.lastScreenshotHeight = bounds.rect.height;
          this.lastScreenshotURL = this.url;

          if (document.body.classList.contains('divider-dragging')) {
            this.screenshotVisible = true;
            resolve(true);
            return;
          }
          
          requestAnimationFrame(() => {
            requestAnimationFrame(() => {
              if (requestId === this.screenshotRequestId) {
                this.screenshotVisible = true;
                resolve(true);
              } else {
                resolve(false);
              }
            });
          });
        } else {
          this.hideCanvasElement(canvas);
          this.lastScreenshotWidth = bounds.rect.width;
          this.lastScreenshotHeight = bounds.rect.height;
          this.lastScreenshotURL = this.url;
          this.screenshotVisible = false;
          resolve(true);
        }
      });
    });
  }

  private hideCanvas(clearCache: boolean): void {
    this.screenshotVisible = false;
    this.lastScreenshotWidth = 0;
    this.lastScreenshotHeight = 0;
    this.screenshotPending = false;
    this.forceFreshScreenshot = false;
    this.lastScreenshotStartedAt = 0;
    this.clearScreenshotCooldown();
    const canvas = this.canvasEl?.nativeElement ?? null;
    if (canvas) {
      this.hideCanvasElement(canvas);
    }
    this.applyBackgroundColor(this.liveBackgroundColor);
    if (clearCache) {
      this.lastScreenshotURL = '';
      this.clearCanvasBitmap(canvas);
    }
  }

  private showCanvas(): void {
    const canvas = this.canvasEl?.nativeElement;
    if (!canvas) {
      return;
    }
    // Shift canvas by the sub-pixel offset so the screenshot content
    // aligns with the exact physical pixel position of the Win32 window.
    if (this.subpixelOffsetX !== 0 || this.subpixelOffsetY !== 0) {
      canvas.style.transform = `translate(${-this.subpixelOffsetX}px, ${-this.subpixelOffsetY}px)`;
    } else {
      canvas.style.transform = '';
    }
    // Use page background color so an empty canvas (before screenshot arrives)
    // matches the page rather than flashing white.
    const bg = this.documentBackgroundColor || this.liveBackgroundColor;
    canvas.style.background = bg;
    this.applyBackgroundColor(bg);
    canvas.style.zIndex = '2';
    canvas.style.display = 'block';
  }

  private hideCanvasElement(canvas: HTMLCanvasElement): void {
    canvas.style.display = 'none';
    canvas.style.transform = '';
    canvas.style.zIndex = '1';
    canvas.style.background = '';
  }

  private clearCanvasBitmap(canvas: HTMLCanvasElement | null): void {
    if (!canvas) {
      return;
    }
    canvas.width = 0;
    canvas.height = 0;
    canvas.style.width = '';
    canvas.style.height = '';
  }

  private revealCachedCanvasIfAvailable(): void {
    if (this.screenshotVisible) {
      return;
    }
    if (this.lastScreenshotURL !== this.url) {
      return;
    }
    if (!this.hasCachedCanvas()) {
      return;
    }
    this.showCanvas();
    if (document.body.classList.contains('divider-hover')) {
      this.hoverFreezeActive = true;
    }
    this.screenshotVisible = true;
    this.lastSyncKey = '';
    this.requestSync();
  }

  private hasCachedCanvas(): boolean {
    const canvas = this.canvasEl?.nativeElement;
    return !!canvas && canvas.width > 0 && canvas.height > 0;
  }

  private async decodeBitmap(base64Data: string): Promise<ImageBitmap | HTMLImageElement | null> {
    const dataUrl = `data:image/png;base64,${base64Data}`;
    if (typeof createImageBitmap === 'function') {
      try {
        const response = await fetch(dataUrl);
        const blob = await response.blob();
        return await createImageBitmap(blob);
      } catch {
        // Fall through to HTMLImageElement
      }
    }
    return await new Promise<HTMLImageElement | null>((resolve) => {
      const img = new Image();
      img.onload = () => resolve(img);
      img.onerror = () => resolve(null);
      img.src = dataUrl;
    });
  }

  // ── Internal: color helpers ─────────────────────────────────

  private resolveTheme(): 'dark' | 'light' {
    return document.documentElement.classList.contains('vscode-light') ? 'light' : 'dark';
  }

  private resolveBackgroundColor(_theme: 'dark' | 'light'): string {
    return 'var(--vsc-on-accent)';
  }

  private parseColor(value: string): { r: number; g: number; b: number; a: number } | null {
    const normalized = (value ?? '').trim().toLowerCase();
    if (!normalized || normalized === 'transparent') {
      return null;
    }

    const clampByte = (input: number): number => Math.max(0, Math.min(255, Math.round(input)));
    const clampAlpha = (input: number): number => Math.max(0, Math.min(1, input));

    const hex6 = normalized.match(/^#([0-9a-f]{6})$/i);
    if (hex6) {
      return {
        r: parseInt(hex6[1].slice(0, 2), 16),
        g: parseInt(hex6[1].slice(2, 4), 16),
        b: parseInt(hex6[1].slice(4, 6), 16),
        a: 1,
      };
    }

    const hex3 = normalized.match(/^#([0-9a-f]{3})$/i);
    if (hex3) {
      return {
        r: parseInt(`${hex3[1][0]}${hex3[1][0]}`, 16),
        g: parseInt(`${hex3[1][1]}${hex3[1][1]}`, 16),
        b: parseInt(`${hex3[1][2]}${hex3[1][2]}`, 16),
        a: 1,
      };
    }

    const hex8 = normalized.match(/^#([0-9a-f]{8})$/i);
    if (hex8) {
      return {
        r: parseInt(hex8[1].slice(0, 2), 16),
        g: parseInt(hex8[1].slice(2, 4), 16),
        b: parseInt(hex8[1].slice(4, 6), 16),
        a: parseInt(hex8[1].slice(6, 8), 16) / 255,
      };
    }

    const hex4 = normalized.match(/^#([0-9a-f]{4})$/i);
    if (hex4) {
      return {
        r: parseInt(`${hex4[1][0]}${hex4[1][0]}`, 16),
        g: parseInt(`${hex4[1][1]}${hex4[1][1]}`, 16),
        b: parseInt(`${hex4[1][2]}${hex4[1][2]}`, 16),
        a: parseInt(`${hex4[1][3]}${hex4[1][3]}`, 16) / 255,
      };
    }

    const rgb = normalized.match(/^rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})(?:\s*,\s*([\d.]+)\s*)?\)$/i);
    if (rgb) {
      return {
        r: clampByte(Number(rgb[1])),
        g: clampByte(Number(rgb[2])),
        b: clampByte(Number(rgb[3])),
        a: clampAlpha(rgb[4] !== undefined ? Number(rgb[4]) : 1),
      };
    }

    return null;
  }

  private toHex(color: { r: number; g: number; b: number }): string {
    const hex = (c: number) => Math.max(0, Math.min(255, Math.round(c))).toString(16).padStart(2, '0');
    return `#${hex(color.r)}${hex(color.g)}${hex(color.b)}`;
  }

  private compositeColor(
    foreground: { r: number; g: number; b: number; a: number },
    background: { r: number; g: number; b: number; a: number },
  ): { r: number; g: number; b: number } {
    const alpha = Math.max(0, Math.min(1, foreground.a));
    return {
      r: foreground.r * alpha + background.r * (1 - alpha),
      g: foreground.g * alpha + background.g * (1 - alpha),
      b: foreground.b * alpha + background.b * (1 - alpha),
    };
  }

  private normalizeColorToHex(value: string, fallback: string): string {
    const color = this.parseColor(value);
    if (!color) {
      return fallback;
    }
    if (color.a <= 0) {
      return fallback;
    }
    if (color.a >= 1) {
      return this.toHex(color);
    }
    const background = this.parseColor(fallback);
    if (!background) {
      return fallback;
    }
    return this.toHex(this.compositeColor(color, background));
  }

  private applyBackgroundColor(color: string): void {
    this.hostBackgroundColor = color;
    const host = this.hostEl?.nativeElement;
    if (host) {
      host.style.backgroundColor = color;
    }

    const backdrop = this.backdropEl?.nativeElement;
    if (backdrop) {
      backdrop.style.backgroundColor = color;
    }
  }

  private requestDocumentBackground(force: boolean, theme: 'dark' | 'light'): void {
    if (!this.active || !this.surfaceId || !this.url || this.documentBackgroundRequestInFlight) {
      return;
    }

    const now = Date.now();
    const sameURL = this.lastDocumentBackgroundURL === this.url;
    if (!force) {
      if (sameURL && this.documentBackgroundColor && (now - this.lastDocumentBackgroundRequestAt) < this.documentBackgroundRefreshMs) {
        return;
      }
      if (sameURL && (now - this.lastDocumentBackgroundRequestAt) < this.documentBackgroundRetryMs) {
        return;
      }
    }

    this.documentBackgroundRequestInFlight = true;
    const requestURL = this.url;
    const requestToken = ++this.documentBackgroundRequestToken;
    this.lastDocumentBackgroundURL = requestURL;
    this.lastDocumentBackgroundRequestAt = now;
    const fallback = this.resolveBackgroundColor(theme);

    void this.nativeBrowserPane.getDocumentBackground(this.surfaceId)
      .then((value) => {
        if (requestToken !== this.documentBackgroundRequestToken || requestURL !== this.url || !this.active) {
          return;
        }
        const normalized = this.normalizeColorToHex(value, fallback);
        if (!normalized || normalized === this.documentBackgroundColor) {
          return;
        }
        this.documentBackgroundColor = normalized;
        this.applyBackgroundColor(normalized);
        this.lastSyncKey = '';
        this.requestSync();
      })
      .catch(() => {
        if (requestToken === this.documentBackgroundRequestToken && requestURL === this.url && this.active && !this.documentBackgroundColor) {
          this.applyBackgroundColor(fallback);
        }
      })
      .finally(() => {
        this.documentBackgroundRequestInFlight = false;
      });
  }

}