import { Injectable, NgZone, OnDestroy, signal } from '@angular/core';
import { EventsOff, EventsOn } from 'wailsjs/runtime';
import { ApiService } from './api.service';

export interface ElevationDialogState {
  visible: boolean;
  requestId?: string;
  phase: 'requested' | 'result';
  operation: string;
  target: string;
  message: string;
  success?: boolean;
  error?: string;
}

@Injectable({ providedIn: 'root' })
export class ElevationDialogService implements OnDestroy {
  readonly state = signal<ElevationDialogState | null>(null);
  readonly busy = signal<boolean>(false);

  constructor(private readonly zone: NgZone, private readonly api: ApiService) {
    this.bindEvents();
  }

  dismiss(): void {
    this.state.set(null);
  }

  async approve(): Promise<void> {
    const state = this.state();
    if (!state || state.phase !== 'requested' || !state.requestId || this.busy()) {
      return;
    }
    this.busy.set(true);
    try {
      await this.api.confirmElevationRequest(state.requestId, true);
      this.state.update((s) => (s ? { ...s, message: 'Waiting for OS elevation prompt...' } : s));
    } finally {
      this.busy.set(false);
    }
  }

  async cancel(): Promise<void> {
    const state = this.state();
    if (!state || state.phase !== 'requested' || !state.requestId || this.busy()) {
      return;
    }
    this.busy.set(true);
    try {
      await this.api.confirmElevationRequest(state.requestId, false);
    } finally {
      this.busy.set(false);
    }
  }

  ngOnDestroy(): void {
    EventsOff('elevation:requested');
    EventsOff('elevation:result');
  }

  private bindEvents(): void {
    EventsOn('elevation:requested', (evt: any) => {
      this.zone.run(() => {
        this.state.set({
          visible: true,
          requestId: evt?.requestId ? String(evt.requestId) : undefined,
          phase: 'requested',
          operation: String(evt?.operation ?? ''),
          target: String(evt?.target ?? ''),
          message: String(evt?.message ?? 'Elevation required'),
        });
      });
    });

    EventsOn('elevation:result', (evt: any) => {
      this.zone.run(() => {
        const next: ElevationDialogState = {
          visible: true,
          requestId: evt?.requestId ? String(evt.requestId) : undefined,
          phase: 'result',
          operation: String(evt?.operation ?? ''),
          target: String(evt?.target ?? ''),
          message: String(evt?.message ?? ''),
          success: !!evt?.success,
          error: evt?.error ? String(evt.error) : undefined,
        };
        this.state.set(next);

        if (next.success) {
          window.setTimeout(() => {
            const current = this.state();
            if (current && current.phase === 'result' && current.success) {
              this.dismiss();
            }
          }, 1800);
        }
      });
    });
  }
}
