// elevation-status-pill.component.ts
// Status-bar pill that surfaces the persistent elevated service session.
// Shows whether the helper process is: not started, starting, active, or failed.
// Clicking the pill starts or stops the session.
import {
  Component,
  OnInit,
  OnDestroy,
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Input,
  signal,
  computed,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { EventsOn, EventsOff } from 'wailsjs/runtime';
import { ApiService, ElevatedSessionStatus } from '../services/api.service';

type PillState = 'idle' | 'starting' | 'active' | 'stopping' | 'error';

@Component({
  selector: 'app-elevation-status-pill',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="elev-pill-host">
      <!-- Only render when there is something worth showing -->
      <button type="button"
        class="elev-pill"
        [class.idle]="pillState() === 'idle'"
        [class.starting]="pillState() === 'starting'"
        [class.active]="pillState() === 'active'"
        [class.stopping]="pillState() === 'stopping'"
        [class.error]="pillState() === 'error'"
        [class.flash-ok]="flashOk()"
        [class.flash-err]="flashErr()"
        [class.blink-op]="blinkOp() && pillState() === 'active'"
        [disabled]="monitorOnly || pillState() === 'starting' || pillState() === 'stopping'"
        (click)="onClick()"
        [title]="tooltip()"
      >
        <!-- Shield icon -->
        <svg class="icon" aria-hidden="true" viewBox="0 0 24 24" width="13" height="13"
             stroke="currentColor" stroke-width="2" fill="none"
             stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
        </svg>

        <span class="label" aria-live="polite">{{ label() }}</span>

        <!-- Op-count badge, visible only when active -->
        <span class="badge" *ngIf="pillState() === 'active' && opCount() > 0">
          {{ opCount() }}
        </span>

        <!-- Spinner while starting / stopping -->
        <span class="spinner" *ngIf="pillState() === 'starting' || pillState() === 'stopping'"></span>
      </button>
    </div>
  `,
  styles: [`
    :host { display: block; }

    .elev-pill-host { display: flex; align-items: center; }

    .elev-pill {
      display: flex;
      align-items: center;
      gap: 4px;
      position: relative;
      isolation: isolate;
      overflow: hidden;
      height: calc(var(--vsc-statusbar-height, 22px) - 4px);
      padding: 0 8px;
      border: 1px solid var(--vsc-neutral-overlay-soft);
      border-radius: 3px;
      cursor: pointer;
      font-size: 11px;
      font-family: var(--vsc-font-family, inherit);
      transition: background 150ms ease, color 150ms ease, box-shadow 150ms ease, transform 120ms ease;
      background: linear-gradient(180deg, var(--vsc-neutral-overlay-soft), var(--vsc-neutral-overlay-subtle));
      color: var(--vsc-statusbar-foreground, var(--vsc-on-accent));
      box-shadow: inset 0 1px 0 var(--vsc-neutral-overlay-subtle);
    }
    .elev-pill::after {
      content: "";
      position: absolute;
      inset: -1px;
      border-radius: inherit;
      pointer-events: none;
      opacity: 0;
      background: linear-gradient(110deg, transparent 0%, var(--vsc-neutral-overlay-soft) 45%, var(--vsc-media-control-muted) 50%, var(--vsc-neutral-overlay-soft) 55%, transparent 100%);
      transform: translateX(-120%);
      z-index: 0;
    }
    .elev-pill > * {
      position: relative;
      z-index: 1;
    }

    .elev-pill:hover:not([disabled]) {
      background: linear-gradient(180deg, var(--vsc-neutral-overlay-medium), var(--vsc-neutral-overlay-subtle));
      transform: translateY(-0.5px);
    }

    .elev-pill[disabled] { cursor: default; opacity: 0.7; }

    /* State colours */
    .elev-pill.active {
      background:
        radial-gradient(120% 130% at 20% 10%, var(--vsc-accent-selection), transparent 45%),
        linear-gradient(135deg, var(--vsc-button-background) 0%, var(--vsc-accent-blue) 55%, var(--vsc-button-background) 100%);
      color: var(--vsc-list-active-foreground, var(--vsc-on-accent));
      border-color: var(--vsc-accent-border);
      box-shadow: 0 0 0 1px var(--vsc-accent-selection), 0 0 12px var(--vsc-accent-border);
    }
    @keyframes blink-op {
      0%, 10%, 33%, 56%, 79%, 100% {
        background:
          radial-gradient(120% 130% at 20% 10%, var(--vsc-accent-border), transparent 45%),
          linear-gradient(135deg, var(--vsc-button-background) 0%, var(--vsc-accent-blue) 55%, var(--vsc-button-background) 100%);
        color: var(--vsc-list-active-foreground, var(--vsc-on-accent));
        box-shadow: 0 0 0 1px var(--vsc-accent-selection), 0 0 12px var(--vsc-accent-border);
        transform: translateY(0) scale(1);
      }
      22%, 45%, 68% {
        background:
          radial-gradient(120% 130% at 20% 10%, var(--vsc-warning-border), transparent 45%),
          linear-gradient(135deg, var(--vsc-accent-orange) 0%, var(--vsc-accent-yellow) 55%, var(--vsc-accent-orange) 100%);
        color: var(--vsc-media-background);
        box-shadow: 0 0 0 1px var(--vsc-warning-border), 0 0 16px var(--vsc-warning-border);
        transform: translateY(-0.5px) scale(1.02);
      }
    }
    @keyframes blink-sweep {
      0%   { opacity: 0; transform: translateX(-120%); }
      8%   { opacity: 0.85; }
      38%  { opacity: 0.55; }
      72%  { opacity: 0.35; }
      100% { opacity: 0; transform: translateX(120%); }
    }
    .elev-pill.blink-op {
      animation: blink-op 900ms cubic-bezier(0.22, 1, 0.36, 1) 1;
    }
    .elev-pill.blink-op .icon {
      animation: icon-pulse 900ms cubic-bezier(0.22, 1, 0.36, 1) 1;
    }
    .elev-pill.blink-op::after {
      animation: blink-sweep 900ms ease-out 1;
    }
    .elev-pill.error {
      background: var(--vsc-input-validation-error-background, var(--vsc-danger-surface));
      color: var(--vsc-error-foreground, var(--vsc-error));
    }
    .elev-pill.starting, .elev-pill.stopping {
      opacity: 0.75;
    }

    /* Flash animations */
    @keyframes flash-ok {
      0%   { background: var(--vsc-charts-yellow, var(--vsc-accent-yellow)); color: var(--vsc-media-foreground-strong); }
      25%  { background: var(--vsc-charts-orange, var(--vsc-chart-6)); color: var(--vsc-media-foreground-strong); }
      50%  { background: var(--vsc-charts-yellow, var(--vsc-accent-yellow)); color: var(--vsc-media-foreground-strong); }
      75%  { background: var(--vsc-charts-orange, var(--vsc-chart-6)); color: var(--vsc-media-foreground-strong); }
      100% { background: var(--vsc-badge-background, var(--vsc-button-background)); color: var(--vsc-list-active-foreground, var(--vsc-on-accent)); }
    }
    @keyframes flash-err {
      0%   { background: var(--vsc-charts-red, var(--vsc-error)); }
      100% { background: transparent; }
    }
    .elev-pill.flash-ok { animation: flash-ok 1.5s ease-in-out; }
    .elev-pill.flash-err { animation: flash-err 500ms ease-out; }

    /* Op-count badge */
    .badge {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 14px;
      height: 14px;
      padding: 0 3px;
      border-radius: 8px;
      font-size: 9px;
      background: color-mix(in srgb, var(--vsc-foreground) 25%, transparent);
      line-height: 1;
    }

    /* Spinner */
    .spinner {
      width: 10px; height: 10px;
      border: 1.5px solid currentColor;
      border-top-color: transparent;
      border-radius: 50%;
      animation: spin 600ms linear infinite;
    }
    @keyframes icon-pulse {
      0%, 10%, 33%, 56%, 79%, 100% { transform: scale(1); }
      22%, 45%, 68% { transform: scale(1.16); }
    }
    @keyframes spin { to { transform: rotate(360deg); } }

    .icon { flex-shrink: 0; }
    .label { white-space: nowrap; }
  `],
})
export class ElevationStatusPillComponent implements OnInit, OnDestroy {
  @Input() monitorOnly = false;
  // Reactive state (Angular signals)
  private _state = signal<PillState>('idle');
  private _opCount = signal(0);
  private _errorMsg = signal('');
  private _flashOk = signal(false);
  private _flashErr = signal(false);
  private _blinkOp  = signal(false);

  readonly pillState = computed(() => this._state());
  readonly opCount   = computed(() => this._opCount());
  readonly flashOk   = computed(() => this._flashOk());
  readonly flashErr  = computed(() => this._flashErr());
  readonly blinkOp   = computed(() => this._blinkOp());

  readonly label = computed(() => {
    switch (this._state()) {
      case 'starting':  return 'Starting...';
      case 'stopping':  return 'Stopping...';
      case 'active':    return 'Elevated';
      case 'error':     return 'Elev. Error';
      default:          return 'Elevate';
    }
  });

  readonly tooltip = computed(() => {
    switch (this._state()) {
      case 'active':
        return this.monitorOnly
          ? `Elevated session active (${this._opCount()} ops dispatched).`
          : `Elevated session active (${this._opCount()} ops dispatched). Click to stop.`;
      case 'error':
        return this.monitorOnly
          ? `Elevated session error: ${this._errorMsg()}.`
          : `Elevated session error: ${this._errorMsg()}. Click to retry.`;
      case 'starting':
        return 'Starting elevated session...';
      case 'stopping':
        return 'Stopping elevated session...';
      default:
        return this.monitorOnly ? 'Monitors hidden elevated helper activity.' : 'Start an elevated service session (one UAC/sudo prompt, then reuse).';
    }
  });

  constructor(private api: ApiService, private cdr: ChangeDetectorRef) {}

  // Lifecycle

  ngOnInit(): void {
    // Sync initial state from backend safely, in case the Wails runtime
    // hasn't registered the method yet (stale bindings before wails dev restart).
    this.api.getElevatedSessionStatus()
      .then(s => { this.applyStatus(s); this.cdr.markForCheck(); })
      .catch(() => { /* method not yet available in runtime - ignore */ });

    // Backend events
    EventsOn('elevation:uac-requested', () => {
      this._errorMsg.set('');
      this._state.set('starting');
      this.cdr.markForCheck();
    });
    EventsOn('elevation:session-starting', () => {
      // Auto-start triggered by a permission-denied operation; show UAC is pending.
      this._errorMsg.set('');
      this._state.set('starting');
      this.cdr.markForCheck();
    });
    EventsOn('elevation:session-started', (status: ElevatedSessionStatus) => {
      this.applyStatus(status);
      this.cdr.markForCheck();
    });
    EventsOn('elevation:session-stopped', () => {
      this._state.set('idle');
      this._opCount.set(0);
      this.cdr.markForCheck();
    });
    EventsOn('elevation:auto-started', () => {
      this._opCount.update(n => n + 1);
      if (this._state() !== 'active') {
        this._state.set('idle');
        this.triggerFlash('ok');
      }
      this.triggerBlinkOp();
      this.cdr.markForCheck();
    });
    EventsOn('elevation:auto-failed', (ev: { error?: string }) => {
      this._errorMsg.set(ev?.error ?? 'Unknown error');
      this._state.set('error');
      this.triggerFlash('err');
      this.cdr.markForCheck();
    });
  }

  ngOnDestroy(): void {
    EventsOff(
      'elevation:uac-requested',
      'elevation:session-starting',
      'elevation:session-started',
      'elevation:session-stopped',
      'elevation:auto-started',
      'elevation:auto-failed',
    );
  }

  // User interaction

  async toggle(): Promise<void> {
    const current = this._state();

    if (current === 'active') {
      this._state.set('stopping');
      this.cdr.markForCheck();
      await this.api.stopElevatedSession().catch(() => {});
      this._state.set('idle');
      this._opCount.set(0);
      this.cdr.markForCheck();
      return;
    }

    // idle / error -> start session
    this._state.set('starting');
    this._errorMsg.set('');
    this.cdr.markForCheck();

    try {
      const status = await this.api.startElevatedSession();
      this.applyStatus(status);
    } catch (err: any) {
      this._state.set('error');
      this._errorMsg.set(err?.message ?? String(err));
      this.triggerFlash('err');
    }
    this.cdr.markForCheck();
  }

  onClick(): void {
    if (this.monitorOnly) return;
    void this.toggle();
  }

  // Helpers

  private applyStatus(s: ElevatedSessionStatus): void {
    if (!s) {
      this._state.set('idle');
      return;
    }
    this._opCount.set(s.opCount ?? 0);
    if (s.active) {
      this._state.set('active');
      this._errorMsg.set('');
    } else if (s.error) {
      this._state.set('error');
      this._errorMsg.set(s.error);
    } else {
      this._state.set('idle');
    }
  }

  private triggerBlinkOp(): void {
    this._blinkOp.set(false);
    this.cdr.markForCheck();
    // force reflow so animation restarts
    setTimeout(() => {
      this._blinkOp.set(true);
      this.cdr.markForCheck();
      // 3 cycles x 300ms = 900ms
      setTimeout(() => { this._blinkOp.set(false); this.cdr.markForCheck(); }, 950);
    }, 16);
  }

  private triggerFlash(kind: 'ok' | 'err'): void {
    if (kind === 'ok') {
      this._flashOk.set(true);
      this.cdr.markForCheck();
      setTimeout(() => { this._flashOk.set(false); this.cdr.markForCheck(); }, 450);
    } else {
      this._flashErr.set(true);
      this.cdr.markForCheck();
      setTimeout(() => { this._flashErr.set(false); this.cdr.markForCheck(); }, 550);
    }
  }
}