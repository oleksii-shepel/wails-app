import { DecimalPipe, NgIf } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnDestroy, OnInit } from '@angular/core';
import { Subscription } from 'rxjs';
import { PerfProbeService, type PerfProbeSnapshot } from '../services/perf-probe.service';

@Component({
  selector: 'app-perf-overlay',
  standalone: true,
  imports: [DecimalPipe, NgIf],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <aside class="perf-overlay" aria-live="polite">
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
      left: 8px;
      bottom: 8px;
      z-index: 20000;
      min-width: 112px;
      padding: 8px 10px;
      background: rgba(245, 245, 245, 0.96);
      border: 1px solid rgba(0, 0, 0, 0.12);
      border-radius: 4px;
      color: #111;
      font: 12px/1.5 Consolas, 'Courier New', monospace;
      pointer-events: none;
      white-space: nowrap;
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
    }

    .divider {
      margin: 6px 0;
      border-top: 1px solid rgba(0, 0, 0, 0.12);
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
  }
}
