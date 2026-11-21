import { DecimalPipe, NgIf } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, HostListener, OnDestroy, OnInit } from '@angular/core';
import { Subscription } from 'rxjs';
import { PerfProbeService, type PerfProbeSnapshot } from '../services/perf-probe.service';

@Component({
  selector: 'app-perf-overlay',
  standalone: true,
  imports: [DecimalPipe, NgIf],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <aside
      class="perf-overlay"
      [class.perf-overlay--dragging]="dragging"
      [style.left.px]="positionLeft"
      [style.top.px]="positionTop"
      [style.bottom.px]="positionTop === null ? 8 : null"
      aria-live="polite"
      title="Drag performance overlay"
      (pointerdown)="startDrag($event)"
      (dblclick)="resetPosition($event)"
    >
      <div>fps {{ snapshot.fps }}</div>
      <div>avg {{ snapshot.avgFrameMs | number:'1.0-1' }} ms</div>
      <div>frame {{ snapshot.worstFrameMs | number:'1.0-0' }} ms</div>
      <div>long {{ snapshot.longFramesPerSecond | number:'1.0-0' }}/s</div>
      <div>ticks {{ snapshot.ticksPerSecond }}/s</div>
      <div>tick {{ snapshot.avgTickMs | number:'1.0-0' }} ms</div>
      <div>ptr {{ snapshot.pointerMovesPerSecond }}/s</div>
      <ng-container *ngIf="snapshot.benchmark as benchmark">
        <div class="divider"></div>
        <div>bench {{ benchmark.durationMs | number:'1.0-0' }} ms</div>
        <div>b fps {{ benchmark.fps }}</div>
        <div>b avg {{ benchmark.avgFrameMs | number:'1.0-1' }} ms</div>
        <div>b max {{ benchmark.worstFrameMs | number:'1.0-0' }} ms</div>
        <div>b long {{ benchmark.longFramesPerSecond | number:'1.0-1' }}/s</div>
        <div>b tick {{ benchmark.avgTickMs | number:'1.0-1' }} ms</div>
      </ng-container>
    </aside>
  `,
  styles: [`
    .perf-overlay {
      position: fixed;
      z-index: 20000;
      min-width: 112px;
      padding: 8px 10px;
      background: var(--vsc-popover-background);
      border: 1px solid var(--vsc-border-subtle);
      border-radius: 4px;
      color: var(--vsc-foreground);
      font: 12px/1.5 Consolas, 'Courier New', monospace;
      cursor: move;
      pointer-events: auto;
      white-space: nowrap;
      box-shadow: 0 1px 3px var(--vsc-shadow-soft);
      touch-action: none;
      user-select: none;
    }

    .perf-overlay:hover {
      border-color: var(--vsc-border);
      box-shadow: 0 2px 8px var(--vsc-shadow);
    }

    .perf-overlay--dragging {
      opacity: 0.94;
      cursor: grabbing;
    }

    .divider {
      margin: 6px 0;
      border-top: 1px solid var(--vsc-border-subtle);
    }
  `],
})
export class PerfOverlayComponent implements OnInit, OnDestroy {
  snapshot: PerfProbeSnapshot = {
    fps: 0,
    avgFrameMs: 0,
    worstFrameMs: 0,
    longFramesPerSecond: 0,
    ticksPerSecond: 0,
    avgTickMs: 0,
    pointerMovesPerSecond: 0,
    benchmark: null,
  };
  private subscription: Subscription | null = null;
  dragging = false;
  positionLeft = 8;
  positionTop: number | null = null;
  private readonly viewportMarginPx = 4;
  private activePointerId: number | null = null;
  private dragElement: HTMLElement | null = null;
  private dragOffsetX = 0;
  private dragOffsetY = 0;
  private dragWidth = 0;
  private dragHeight = 0;

  constructor(
    private readonly perf: PerfProbeService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    this.perf.start();
    this.subscription = this.perf.snapshot$.subscribe(snapshot => {
      this.snapshot = snapshot;
      this.cdr.markForCheck();
    });
  }

  ngOnDestroy(): void {
    this.subscription?.unsubscribe();
    this.subscription = null;
    this.finishDrag(false);
  }

  startDrag(event: PointerEvent): void {
    if (event.button !== 0) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();

    const element = event.currentTarget as HTMLElement;
    const rect = element.getBoundingClientRect();
    this.dragging = true;
    this.activePointerId = event.pointerId;
    this.dragElement = element;
    this.dragOffsetX = event.clientX - rect.left;
    this.dragOffsetY = event.clientY - rect.top;
    this.dragWidth = rect.width;
    this.dragHeight = rect.height;
    this.positionLeft = rect.left;
    this.positionTop = rect.top;

    try {
      element.setPointerCapture(event.pointerId);
    } catch {
      // Pointer capture can fail in synthetic tests or after a stale pointer.
    }

    this.cdr.markForCheck();
  }

  resetPosition(event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.positionLeft = 8;
    this.positionTop = null;
    this.cdr.markForCheck();
  }

  @HostListener('document:pointermove', ['$event'])
  onPointerMove(event: PointerEvent): void {
    if (!this.dragging || this.activePointerId !== event.pointerId) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();

    const maxLeft = Math.max(this.viewportMarginPx, window.innerWidth - this.dragWidth - this.viewportMarginPx);
    const maxTop = Math.max(this.viewportMarginPx, window.innerHeight - this.dragHeight - this.viewportMarginPx);
    this.positionLeft = this.clamp(event.clientX - this.dragOffsetX, this.viewportMarginPx, maxLeft);
    this.positionTop = this.clamp(event.clientY - this.dragOffsetY, this.viewportMarginPx, maxTop);
    this.cdr.markForCheck();
  }

  @HostListener('document:pointerup', ['$event'])
  onPointerUp(event: PointerEvent): void {
    if (this.activePointerId !== event.pointerId) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    this.finishDrag();
  }

  @HostListener('document:pointercancel', ['$event'])
  onPointerCancel(event: PointerEvent): void {
    if (this.activePointerId === event.pointerId) {
      this.finishDrag();
    }
  }

  private finishDrag(markForCheck = true): void {
    if (this.activePointerId !== null && this.dragElement) {
      try {
        this.dragElement.releasePointerCapture(this.activePointerId);
      } catch {
        // Ignore stale pointer capture state.
      }
    }

    this.dragging = false;
    this.activePointerId = null;
    this.dragElement = null;
    if (markForCheck) {
      this.cdr.markForCheck();
    }
  }

  private clamp(value: number, min: number, max: number): number {
    return Math.max(min, Math.min(max, value));
  }
}
