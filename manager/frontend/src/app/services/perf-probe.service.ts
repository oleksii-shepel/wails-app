import { ApplicationRef, Injectable, NgZone } from '@angular/core';
import { BehaviorSubject } from 'rxjs';

export interface PerfProbeSnapshot {
  readonly fps: number;
  readonly avgFrameMs: number;
  readonly worstFrameMs: number;
  readonly longFramesPerSecond: number;
  readonly ticksPerSecond: number;
  readonly avgTickMs: number;
  readonly pointerMovesPerSecond: number;
  readonly benchmark: PerfBenchmarkResult | null;
}

export interface PerfBenchmarkResult {
  readonly durationMs: number;
  readonly fps: number;
  readonly avgFrameMs: number;
  readonly worstFrameMs: number;
  readonly longFrameCount: number;
  readonly longFramesPerSecond: number;
  readonly ticksPerSecond: number;
  readonly avgTickMs: number;
  readonly pointerMovesPerSecond: number;
}

declare global {
  interface Window {
    __afmPerf?: {
      runBenchmark: (durationMs?: number) => Promise<PerfBenchmarkResult>;
      clearBenchmark: () => void;
      getSnapshot: () => PerfProbeSnapshot;
    };
  }
}

@Injectable({ providedIn: 'root' })
export class PerfProbeService {
  private readonly snapshotSubject = new BehaviorSubject<PerfProbeSnapshot>({
    fps: 0,
    avgFrameMs: 0,
    worstFrameMs: 0,
    longFramesPerSecond: 0,
    ticksPerSecond: 0,
    avgTickMs: 0,
    pointerMovesPerSecond: 0,
    benchmark: null,
  });

  readonly snapshot$ = this.snapshotSubject.asObservable();

  private started = false;
  private reportTimer = 0;
  private tickCount = 0;
  private tickTotalMs = 0;
  private pointerMoveCount = 0;
  private sampledFps = 0;
  private sampledWorstFrameMs = 0;
  private sampleActive = false;
  private sampleHandle = 0;
  private sampleStartedAt = 0;
  private sampleLastFrameAt = 0;
  private sampleFrameCount = 0;
  private sampleFrameTotalMs = 0;
  private sampleWorstFrameMs = 0;
  private sampleLongFrameCount = 0;
  private sampledAvgFrameMs = 0;
  private latestBenchmark: PerfBenchmarkResult | null = null;

  constructor(
    private readonly appRef: ApplicationRef,
    private readonly ngZone: NgZone,
  ) {
    if (typeof window !== 'undefined') {
      this.installWindowApi();
    }
  }

  start(): void {
    if (this.started || typeof window === 'undefined') {
      return;
    }

    this.started = true;
    this.instrumentTicks();
    this.ngZone.runOutsideAngular(() => {
      this.installWindowApi();
      window.addEventListener('pointermove', this.onPointerMove, { passive: true });
      this.beginFrameSample();
      this.reportTimer = window.setInterval(() => {
        this.publishSnapshot();
        this.beginFrameSample();
      }, 1000);
    });
  }

  private instrumentTicks(): void {
    const appRef = this.appRef as ApplicationRef & { tick: () => void };
    const originalTick = appRef.tick.bind(this.appRef);
    appRef.tick = () => {
      const startedAt = performance.now();
      try {
        originalTick();
      } finally {
        this.tickCount += 1;
        this.tickTotalMs += performance.now() - startedAt;
      }
    };
  }

  private readonly onPointerMove = (): void => {
    this.pointerMoveCount += 1;
  };

  private installWindowApi(): void {
    window.__afmPerf = {
      runBenchmark: (durationMs?: number) => {
        this.start();
        return this.runBenchmark(durationMs);
      },
      clearBenchmark: () => {
        this.latestBenchmark = null;
        this.publishSnapshot();
      },
      getSnapshot: () => this.snapshotSubject.getValue(),
    };
  }

  private beginFrameSample(): void {
    if (this.sampleActive) {
      return;
    }

    this.sampleActive = true;
    this.sampleStartedAt = performance.now();
    this.sampleLastFrameAt = 0;
    this.sampleFrameCount = 0;
    this.sampleFrameTotalMs = 0;
    this.sampleWorstFrameMs = 0;
    this.sampleLongFrameCount = 0;
    this.sampleHandle = requestAnimationFrame(this.onSampleFrame);
  }

  private readonly onSampleFrame = (timestamp: number): void => {
    if (!this.sampleActive) {
      return;
    }

    if (this.sampleLastFrameAt > 0) {
      const delta = timestamp - this.sampleLastFrameAt;
      this.sampleFrameTotalMs += delta;
      this.sampleWorstFrameMs = Math.max(this.sampleWorstFrameMs, delta);
      if (delta >= 20) {
        this.sampleLongFrameCount += 1;
      }
    }
    this.sampleLastFrameAt = timestamp;
    this.sampleFrameCount += 1;

    const elapsed = timestamp - this.sampleStartedAt;
    if (elapsed >= 250) {
      const seconds = elapsed > 0 ? elapsed / 1000 : 0.25;
      this.sampledFps = seconds > 0 ? Math.round(this.sampleFrameCount / seconds) : 0;
      this.sampledAvgFrameMs = this.sampleFrameCount > 1 ? this.sampleFrameTotalMs / (this.sampleFrameCount - 1) : 0;
      this.sampledWorstFrameMs = this.sampleWorstFrameMs;
      this.sampleActive = false;
      this.sampleHandle = 0;
      return;
    }

    this.sampleHandle = requestAnimationFrame(this.onSampleFrame);
  };

  private publishSnapshot(): void {
    const ticksPerSecond = this.tickCount;
    const avgTickMs = ticksPerSecond > 0 ? this.tickTotalMs / ticksPerSecond : 0;
    this.snapshotSubject.next({
      fps: this.sampledFps,
      avgFrameMs: this.sampledAvgFrameMs,
      worstFrameMs: this.sampledWorstFrameMs,
      longFramesPerSecond: this.sampleLongFrameCount,
      ticksPerSecond,
      avgTickMs,
      pointerMovesPerSecond: this.pointerMoveCount,
      benchmark: this.latestBenchmark,
    });
    this.tickCount = 0;
    this.tickTotalMs = 0;
    this.pointerMoveCount = 0;
  }

  async runBenchmark(durationMs = 5000): Promise<PerfBenchmarkResult> {
    const clampedDurationMs = Math.max(250, Math.round(durationMs));
    return await new Promise<PerfBenchmarkResult>((resolve) => {
      let startedAt = 0;
      let lastFrameAt = 0;
      let frameCount = 0;
      let frameTotalMs = 0;
      let worstFrameMs = 0;
      let longFrameCount = 0;
      const tickCountStart = this.tickCount;
      const tickTotalMsStart = this.tickTotalMs;
      const pointerMoveCountStart = this.pointerMoveCount;

      const onFrame = (timestamp: number): void => {
        if (startedAt === 0) {
          startedAt = timestamp;
          lastFrameAt = timestamp;
          frameCount = 1;
          requestAnimationFrame(onFrame);
          return;
        }

        const frameDelta = timestamp - lastFrameAt;
        lastFrameAt = timestamp;
        frameCount += 1;
        frameTotalMs += frameDelta;
        worstFrameMs = Math.max(worstFrameMs, frameDelta);
        if (frameDelta >= 20) {
          longFrameCount += 1;
        }

        const elapsed = timestamp - startedAt;
        if (elapsed < clampedDurationMs) {
          requestAnimationFrame(onFrame);
          return;
        }

        const durationSeconds = elapsed > 0 ? elapsed / 1000 : clampedDurationMs / 1000;
        const benchTickCount = this.tickCount - tickCountStart;
        const benchTickTotalMs = this.tickTotalMs - tickTotalMsStart;
        const avgTickMs = benchTickCount > 0 ? benchTickTotalMs / benchTickCount : 0;
        const result: PerfBenchmarkResult = {
          durationMs: elapsed,
          fps: durationSeconds > 0 ? Math.round(frameCount / durationSeconds) : 0,
          avgFrameMs: frameCount > 1 ? frameTotalMs / (frameCount - 1) : 0,
          worstFrameMs,
          longFrameCount,
          longFramesPerSecond: durationSeconds > 0 ? longFrameCount / durationSeconds : 0,
          ticksPerSecond: durationSeconds > 0 ? benchTickCount / durationSeconds : 0,
          avgTickMs,
          pointerMovesPerSecond: durationSeconds > 0 ? (this.pointerMoveCount - pointerMoveCountStart) / durationSeconds : 0,
        };
        this.latestBenchmark = result;
        this.publishSnapshot();
        resolve(result);
      };

      requestAnimationFrame(onFrame);
    });
  }
}
