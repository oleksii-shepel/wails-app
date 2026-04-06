import {
  Component, Input, OnChanges, OnDestroy, AfterViewInit,
  SimpleChanges, ViewChild, ElementRef, NgZone,
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
      <canvas #canvasA class="native-overlay-canvas"></canvas>
      <canvas #canvasB class="native-overlay-canvas"></canvas>
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
      top: 0;
      left: 0;
      display: none;
      z-index: 1;
      pointer-events: none;
    }
  `],
})
export class NativeOverlayComponent implements OnChanges, OnDestroy, AfterViewInit {
  @ViewChild('host') private hostEl?: ElementRef<HTMLElement>;
  @ViewChild('backdrop') private backdropEl?: ElementRef<HTMLElement>;
  @ViewChild('canvasA') private canvasA?: ElementRef<HTMLCanvasElement>;
  @ViewChild('canvasB') private canvasB?: ElementRef<HTMLCanvasElement>;

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
  private activeCanvas: 'a' | 'b' | null = null;
  private hoverFreezeActive = false;
  private screenshotPending = false;
  private screenshotCooldownTimer: ReturnType<typeof setTimeout> | null = null;
  private lastScreenshotStartedAt = 0;
  private readonly screenshotMinInterval = 50;

  // ── Event handlers ──────────────────────────────────────────

  private readonly onDividerStateChange = () => {
    this.updateHoverFreezeState();
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
    if (changes['url'] || changes['surfaceId']) {
      this.screenshotRequestId++;
      this.wasFrozen = false;
      this.hideCanvas(true);
    }
    if (changes['url'] || changes['active'] || changes['surfaceId']) {
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
      this.screenshotRequestId++;
      this.wasFrozen = false;
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
    const backgroundColor = this.resolveBackgroundColor(theme);

    const backdrop = this.backdropEl?.nativeElement;
    if (backdrop) {
      backdrop.style.background = backgroundColor;
    }

    // ── Freeze transitions ────────────────────────────────────
    const frozen = this.isFreezeRequested();

    if (frozen && !this.wasFrozen && this.active) {
      this.wasFrozen = true;
      void this.prepareScreenshot();
    }

    if (!frozen && this.wasFrozen) {
      this.wasFrozen = false;
      this.hoverFreezeActive = false;
      this.screenshotRequestId++;
      this.hideCanvas(false);
    }

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

    const rect = host.getBoundingClientRect();
    const scale = (typeof window !== 'undefined' && Number.isFinite(window.devicePixelRatio) && window.devicePixelRatio > 0)
      ? window.devicePixelRatio
      : 1;

    const width = Math.max(0, Math.round(rect.width * scale));
    const height = Math.max(0, Math.round(rect.height * scale));
    if (width <= 0 || height <= 0) {
      return;
    }

    const request = {
      surfaceId: this.surfaceId,
      url: this.url,
      theme,
      backgroundColor,
      visible: !freezeNativePane,
      x: Math.round(rect.left * scale),
      y: Math.round(rect.top * scale),
      width,
      height,
    };
    const syncKey = JSON.stringify(request);
    if (syncKey === this.lastSyncKey) {
      return;
    }

    try {
      if (await this.nativeBrowserPane.sync(request)) {
        this.lastSyncKey = syncKey;
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
    return document.body.classList.contains('divider-dragging')
      || this.overlayFreeze.frozen
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
    this.revealCachedCanvasIfAvailable();
    if (this.syncInFlight) {
      this.scheduleScreenshotCooldown(16);
      return;
    }
    this.maybeStartScreenshotCapture();
  }

  private async captureScreenshot(forceRefresh: boolean): Promise<void> {
    if (!this.surfaceId || !this.active || this.capturingScreenshot || (!forceRefresh && this.screenshotVisible)) {
      return;
    }

    const requestId = ++this.screenshotRequestId;
    this.capturingScreenshot = true;
    try {
      const data = await this.nativeBrowserPane.captureScreenshot(this.surfaceId);
      if (!data || requestId !== this.screenshotRequestId || !this.isFreezeRequested()) {
        return;
      }
      const drawn = await this.drawOnCanvas(data, requestId);
      if (!drawn || requestId !== this.screenshotRequestId || !this.isFreezeRequested()) {
        return;
      }
      if (!document.body.classList.contains('divider-dragging') && document.body.classList.contains('divider-hover')) {
        this.hoverFreezeActive = true;
      }
      this.lastSyncKey = '';
      this.requestSync();
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
    this.screenshotPending = true;
    this.maybeStartScreenshotCapture();
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
    if (this.screenshotVisible && !this.shouldRefreshScreenshot()) {
      this.screenshotPending = false;
      return;
    }
    this.clearScreenshotCooldown();
    this.screenshotPending = false;
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

  private drawOnCanvas(base64Data: string, requestId: number): Promise<boolean> {
    const host = this.hostEl?.nativeElement;
    const canvas = this.getBackBuffer();
    if (!canvas || !host) {
      return Promise.resolve(false);
    }
    const hostRect = host.getBoundingClientRect();
    const cssW = hostRect.width;
    const cssH = hostRect.height;
    return this.decodeBitmap(base64Data).then((bitmap) => {
      if (!bitmap) {
        return false;
      }
      return new Promise<boolean>((resolve) => {
        if (requestId !== this.screenshotRequestId || !this.isFreezeRequested()) {
          if ('close' in bitmap) {
            (bitmap as ImageBitmap).close();
          }
          resolve(false);
          return;
        }
        canvas.width = bitmap.width;
        canvas.height = bitmap.height;
        canvas.style.width = `${cssW}px`;
        canvas.style.height = `${cssH}px`;

        const ctx = canvas.getContext('2d', { alpha: false });
        if (!ctx) {
          if ('close' in bitmap) {
            (bitmap as ImageBitmap).close();
          }
          resolve(false);
          return;
        }
        ctx.globalCompositeOperation = 'copy';
        ctx.drawImage(bitmap, 0, 0);

        if ('close' in bitmap) {
          (bitmap as ImageBitmap).close();
        }

        this.showCanvas(canvas);
        this.lastScreenshotWidth = cssW;
        this.lastScreenshotHeight = cssH;
        this.screenshotVisible = true;
        resolve(true);
      });
    });
  }

  private hideCanvas(clearCache: boolean): void {
    this.screenshotVisible = false;
    this.lastScreenshotWidth = 0;
    this.lastScreenshotHeight = 0;
    this.screenshotPending = false;
    this.lastScreenshotStartedAt = 0;
    this.clearScreenshotCooldown();
    this.canvasA?.nativeElement && this.hideOneCanvas(this.canvasA.nativeElement);
    this.canvasB?.nativeElement && this.hideOneCanvas(this.canvasB.nativeElement);
    if (clearCache) {
      this.activeCanvas = null;
      this.clearCanvasBitmap(this.canvasA?.nativeElement ?? null);
      this.clearCanvasBitmap(this.canvasB?.nativeElement ?? null);
    }
  }

  private getBackBuffer(): HTMLCanvasElement | null {
    if (this.activeCanvas === 'a') {
      return this.canvasB?.nativeElement ?? null;
    }
    return this.canvasA?.nativeElement ?? this.canvasB?.nativeElement ?? null;
  }

  private getActiveBuffer(): HTMLCanvasElement | null {
    if (this.activeCanvas === 'a') {
      return this.canvasA?.nativeElement ?? null;
    }
    if (this.activeCanvas === 'b') {
      return this.canvasB?.nativeElement ?? null;
    }
    return null;
  }

  private showCanvas(canvas: HTMLCanvasElement): void {
    const previous = this.getActiveBuffer();
    canvas.style.display = 'block';
    canvas.style.zIndex = '2';
    if (previous && previous !== canvas) {
      previous.style.zIndex = '1';
      previous.style.display = 'none';
    }
    this.activeCanvas = canvas === this.canvasA?.nativeElement ? 'a' : 'b';
  }

  private hideOneCanvas(canvas: HTMLCanvasElement): void {
    canvas.style.display = 'none';
    canvas.style.zIndex = '1';
  }

  private clearCanvasBitmap(canvas: HTMLCanvasElement | null): void {
    if (!canvas) {
      return;
    }
    canvas.width = 0;
    canvas.height = 0;
    canvas.style.width = '0px';
    canvas.style.height = '0px';
  }

  private revealCachedCanvasIfAvailable(): void {
    if (this.screenshotVisible) {
      return;
    }
    const canvas = this.getActiveBuffer();
    if (!canvas || canvas.width <= 0 || canvas.height <= 0) {
      return;
    }
    canvas.style.display = 'block';
    canvas.style.zIndex = '2';
    this.screenshotVisible = true;
    this.lastSyncKey = '';
    this.requestSync();
  }

  private async decodeBitmap(base64Data: string): Promise<ImageBitmap | HTMLImageElement | null> {
    const dataUrl = `data:image/jpeg;base64,${base64Data}`;
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

  private resolveBackgroundColor(theme: 'dark' | 'light'): string {
    const fallback = theme === 'light' ? '#ffffff' : '#1e1e1e';
    if (typeof window === 'undefined') {
      return fallback;
    }
    let candidate = '';
    if (this.hostEl?.nativeElement) {
      const elStyles = getComputedStyle(this.hostEl.nativeElement);
      candidate = elStyles.backgroundColor;
      if (candidate === 'rgba(0, 0, 0, 0)' || candidate === 'transparent') {
        candidate = '';
      }
    }
    if (!candidate) {
      const rootStyles = getComputedStyle(document.documentElement);
      const bodyStyles = getComputedStyle(document.body);
      candidate = rootStyles.getPropertyValue('--vsc-editor-background').trim()
        || rootStyles.backgroundColor
        || bodyStyles.backgroundColor
        || fallback;
    }
    return this.normalizeColorToHex(candidate, fallback);
  }

  private normalizeColorToHex(value: string, fallback: string): string {
    const n = (value ?? '').trim().toLowerCase();
    if (!n) return fallback;
    if (/^#[0-9a-f]{6}$/i.test(n)) return n;
    const shortHex = n.match(/^#([0-9a-f]{3})$/i);
    if (shortHex) {
      const [r, g, b] = shortHex[1].split('');
      return `#${r}${r}${g}${g}${b}${b}`;
    }
    const rgb = n.match(/^rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})(?:\s*,\s*[\d.]+\s*)?\)$/i);
    if (rgb) {
      const hex = (c: string) => Math.max(0, Math.min(255, Number(c) || 0)).toString(16).padStart(2, '0');
      return `#${hex(rgb[1])}${hex(rgb[2])}${hex(rgb[3])}`;
    }
    return fallback;
  }
}
