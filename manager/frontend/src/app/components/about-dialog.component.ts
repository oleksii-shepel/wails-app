import { writeClipboardText } from '../utils/clipboard-text';
import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  OnChanges,
  OnDestroy,
  SimpleChanges,
  inject,
  input,
  output,
} from '@angular/core';
import { ApiService } from '../services/api.service';
import { ModalStackService } from '../services/modal-stack.service';

@Component({
  selector: 'app-about-dialog',
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (visible()) {
      <div
        class="backdrop palette-dialog-backdrop dialog-group"
        (mousedown)="onBackdropMouseDown($event)">
        <section
          class="panel palette-dialog-surface dialog-surface"
          role="dialog"
          aria-modal="true"
          aria-label="About Reassembly"
          tabindex="-1"
          (mousedown)="$event.stopPropagation()"
          (keydown.escape)="onEscape($event)">
          <header class="palette-dialog-header">
            <div class="palette-dialog-title">
              <strong>About Reassembly</strong>
              <span class="palette-dialog-subtitle">Resource Commander workspace &amp; runtime</span>
            </div>
            <span
              class="dialog-close"
              aria-label="Close"
              title="Close (Esc)"
              (click)="close.emit()"
              role="button"
              tabindex="0"
              (keydown.enter)="close.emit()"
              (keydown.space)="onClose($event)">×</span>
          </header>

          <div class="about-body">
            <figure class="hero-frame">
              @if (!imageFailed) {
                <img
                  class="hero-image"
                  src="/assets/reassembly-about.png"
                  alt="Reassembly Resource Commander overview graphic showing Files, Archives, Registry, System, Cloud, and Plugins"
                  (error)="onImageError()" />
              } @else {
                <div class="hero-fallback" role="img" aria-label="Reassembly Resource Commander">
                  <svg viewBox="0 0 64 40" aria-hidden="true">
                    <path d="M8 6l18 7v21L8 27V6zm48 0L38 13v21l18-7V6z" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linejoin="round" />
                    <path d="M32 9v22" stroke="currentColor" stroke-width="2.2" stroke-dasharray="3 3" stroke-linecap="round" />
                  </svg>
                  <div class="fallback-title">Reassembly</div>
                  <div class="fallback-sub">Resource Commander</div>
                </div>
              }
            </figure>

            <div class="about-summary">
              <div class="summary-heading">
                <h2>Reassembly — Resource Commander</h2>
                <p>Dual-pane workspace for fast navigation, batch operations, archive inspection, and structured file workflows.</p>
              </div>

              <dl class="runtime-grid">
                <div class="runtime-item">
                  <dt>Platform</dt>
                  <dd>{{ platformLabel }}</dd>
                </div>
                <div class="runtime-item">
                  <dt>Host</dt>
                  <dd>{{ hostLabel }}</dd>
                </div>
                <div class="runtime-item">
                  <dt>Frontend</dt>
                  <dd>Angular 22 · Dual-Pane UI</dd>
                </div>
                <div class="runtime-item">
                  <dt>Viewport</dt>
                  <dd class="tabular">{{ viewportLabel }}</dd>
                </div>
              </dl>

              <div class="capability-grid">
                <div class="capability-col">
                  <h3>Files &amp; Panes</h3>
                  <p>Synchronized dual panes, tabs, uploads, downloads, checksums, timestamps, and Trash recovery.</p>
                </div>
                <div class="capability-col">
                  <h3>Archives</h3>
                  <p>Browse inside ZIP and TAR packages, preview contents inline, selectively extract, or pack selections.</p>
                </div>
                <div class="capability-col">
                  <h3>Command &amp; Search</h3>
                  <p>Recursive wildcard search, disk usage inspection, batch rename, and persistent workspaces.</p>
                </div>
              </div>
            </div>
          </div>

          <footer class="palette-dialog-footer about-footer">
            <div class="palette-dialog-footer-actions">
              <button
                type="button"
                class="palette-dialog-button"
                (click)="copySystemInfo()">
                {{ copied ? 'Copied' : 'Copy System Info' }}
              </button>
              <button
                type="button"
                class="palette-dialog-button"
                (click)="openShortcuts()">
                Keyboard Shortcuts
              </button>
              <button
                type="button"
                class="palette-dialog-button"
                (click)="openSettings()">
                Settings
              </button>
            </div>
            <div class="palette-dialog-footer-actions">
              <button
                type="button"
                class="palette-dialog-button palette-dialog-button--primary"
                (click)="close.emit()">
                Close
              </button>
            </div>
          </footer>
        </section>
      </div>
    }
  `,
  styles: [`
    :host {
      position: fixed;
      inset: 0;
      z-index: 8800;
      pointer-events: none;
    }

    .backdrop {
      pointer-events: auto;
    }

    .panel {
      display: grid;
      grid-template-rows: auto minmax(0, 1fr) auto;
      width: min(760px, calc(100vw - 32px));
      max-height: min(760px, calc(100vh - 96px));
      overflow: hidden;
      border: 1px solid var(--vsc-border-bright, var(--vsc-border));
      border-radius: 8px;
      background: var(--vsc-dialog-background, var(--vsc-editor-background));
      color: var(--vsc-foreground);
      box-shadow: 0 20px 56px var(--vsc-scrim);
      font: 12px/1.45 var(--vsc-font-family, system-ui, sans-serif);
    }

    .about-body {
      min-height: 0;
      overflow: auto;
      padding: 16px 20px 18px;
      display: grid;
      gap: 16px;
    }

    .hero-frame {
      margin: 0;
      border-radius: 6px;
      overflow: hidden;
      border: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-about-hero-background);
      aspect-ratio: 1942 / 809;
      width: 100%;
      display: grid;
      place-items: center;
    }

    .hero-image {
      display: block;
      width: 100%;
      height: auto;
    }

    .hero-fallback {
      display: grid;
      place-items: center;
      gap: 6px;
      padding: 24px;
      color: var(--vsc-about-hero-foreground);
      text-align: center;
    }

    .hero-fallback svg {
      width: 56px;
      height: 36px;
      color: var(--vsc-accent-blue);
    }

    .fallback-title {
      font-size: 18px;
      font-weight: 700;
      letter-spacing: 0.08em;
    }

    .fallback-sub {
      font-size: 12px;
      opacity: 0.78;
    }

    .about-summary {
      display: grid;
      gap: 16px;
    }

    .summary-heading h2 {
      margin: 0;
      font-size: 16px;
      font-weight: 600;
      color: var(--vsc-foreground-bright, var(--vsc-foreground));
      line-height: 1.3;
    }

    .summary-heading p {
      margin: 4px 0 0;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 12.5px;
      line-height: 1.5;
    }

    .runtime-grid {
      margin: 0;
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 10px 20px;
      padding: 12px 0;
      border-top: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
    }

    .runtime-item {
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      gap: 12px;
      min-width: 0;
    }

    .runtime-item dt {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 12px;
      flex: 0 0 auto;
    }

    .runtime-item dd {
      margin: 0;
      color: var(--vsc-foreground-bright, var(--vsc-foreground));
      font-size: 12px;
      font-weight: 500;
      text-align: right;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .tabular {
      font-variant-numeric: tabular-nums;
    }

    .capability-grid {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 16px;
    }

    .capability-col h3 {
      margin: 0 0 4px;
      font-size: 12.5px;
      font-weight: 600;
      color: var(--vsc-foreground-bright, var(--vsc-foreground));
    }

    .capability-col p {
      margin: 0;
      font-size: 11.5px;
      line-height: 1.45;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .about-footer {
      gap: 10px;
      flex-wrap: wrap;
    }

    @media (max-width: 680px) {
      .runtime-grid,
      .capability-grid {
        grid-template-columns: 1fr;
      }
    }
  `],
})
export class AboutDialogComponent implements OnChanges, OnDestroy {
  private readonly hostElement = inject(ElementRef<HTMLElement>);
  private readonly api = inject(ApiService);
  private readonly modalStack = inject(ModalStackService);
  private readonly cdr = inject(ChangeDetectorRef);

  readonly visible = input<boolean>(false);
  readonly close = output<void>();
  readonly shortcutsRequested = output<void>();
  readonly settingsRequested = output<void>();

  imageFailed = false;
  copied = false;
  platformLabel = 'Linux · Web Workspace';
  hostLabel = typeof window !== 'undefined' ? window.location.host : 'localhost';
  viewportLabel = typeof window !== 'undefined' ? `${window.innerWidth} × ${window.innerHeight}` : '—';

  private unregisterModal: (() => void) | null = null;
  private copiedTimer: ReturnType<typeof setTimeout> | null = null;

  ngOnChanges(changes: SimpleChanges): void {
    if (!changes['visible']) return;
    if (this.visible()) {
      this.imageFailed = false;
      this.copied = false;
      if (typeof window !== 'undefined') {
        this.viewportLabel = `${window.innerWidth} × ${window.innerHeight}`;
      }
      this.unregisterModal?.();
      this.unregisterModal = this.modalStack.register({
        id: 'about-dialog',
        element: this.hostElement.nativeElement,
        dismiss: () => this.close.emit(),
        dismissible: () => true,
      });
      void this.loadOSInfo();
    } else {
      this.unregisterModal?.();
      this.unregisterModal = null;
    }
  }

  ngOnDestroy(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
    if (this.copiedTimer) clearTimeout(this.copiedTimer);
  }

  onBackdropMouseDown(event: MouseEvent): void {
    if (event.target === event.currentTarget) {
      this.close.emit();
    }
  }

  onEscape(event: Event): void {
    if (!this.modalStack.isTopmost('about-dialog')) return;
    this.onClose(event);
  }

  onClose(event: Event): void {
    event.preventDefault();
    event.stopPropagation();
    this.close.emit();
  }

  onImageError(): void {
    this.imageFailed = true;
    this.cdr.markForCheck();
  }

  openShortcuts(): void {
    this.close.emit();
    this.shortcutsRequested.emit();
  }

  openSettings(): void {
    this.close.emit();
    this.settingsRequested.emit();
  }

  async copySystemInfo(): Promise<void> {
    const info = [
      'Application: Reassembly — Resource Commander',
      `Platform: ${this.platformLabel}`,
      `Host: ${this.hostLabel}`,
      'Frontend: Angular 22 (Zoneless Dual-Pane)',
      `Viewport: ${this.viewportLabel}`,
      `User Agent: ${typeof navigator !== 'undefined' ? navigator.userAgent : 'unknown'}`,
    ].join('\n');

    try {
      await writeClipboardText(info);
      this.copied = true;
      if (this.copiedTimer) clearTimeout(this.copiedTimer);
      this.copiedTimer = setTimeout(() => {
        this.copied = false;
        this.cdr.markForCheck();
      }, 2000);
      this.cdr.markForCheck();
    } catch {
      // Ignore clipboard rejection in restricted contexts
    }
  }

  private async loadOSInfo(): Promise<void> {
    try {
      const info = await this.api.getOSInfo();
      const osName = String(info?.os || 'linux');
      const arch = String(info?.arch || '');
      const version = String(info?.version || '');
      this.platformLabel = [osName, arch, version].filter(Boolean).join(' · ');
      if (info?.hostname) {
        this.hostLabel = `${info.hostname} (${window.location.host})`;
      }
      this.cdr.markForCheck();
    } catch {
      // Keep defaults
    }
  }
}
