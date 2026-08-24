import { Injectable, OnDestroy } from '@angular/core';
import { EventsOn } from 'wailsjs/runtime';

import {
  ResourceMutationService,
  mutationTouchesDirectory,
} from './resource-mutation.service';
import { normalizePath } from '../components/address-bar.utils';

const BIN_ROOT = 'bin://';
const BIN_VISIBLE_POLL_MS = 3_000;
const BIN_HIDDEN_POLL_MS = 12_000;

interface DirectoryChangedEvent {
  path?: string;
}

interface TrashFingerprintEntry {
  id?: string;
  deletedAt?: string;
  size?: number;
  originalPath?: string;
}

interface ObservationRecord {
  readonly paneId: string;
  location: string;
  readonly refresh: () => void | Promise<void>;
  refreshing: boolean;
  pending: boolean;
}

export interface SourceObservation {
  setLocation(location: string): void;
  close(): void;
}

/**
 * One change-observation coordinator for pane sources.
 *
 * Producers are source-specific:
 * - native/filesystem:// locations use the backend fsnotify watcher;
 * - bin:// uses one shared, adaptive Recycle Bin fingerprint poll while observed.
 *
 * Every producer publishes into ResourceMutationService. Consumers are dispatched
 * from that single coalesced mutation stream and never listen to producer events
 * directly.
 */
@Injectable({ providedIn: 'root' })
export class SourceChangeObserverService implements OnDestroy {
  private readonly observations = new Map<symbol, ObservationRecord>();
  private readonly releaseFilesystemEvents: () => void;
  private readonly releaseMutations: () => void;

  private binObserverCount = 0;
  private binPollTimer: ReturnType<typeof setTimeout> | null = null;
  private binPollRunning = false;
  private binFingerprint: string | null = null;
  private destroyed = false;

  constructor(private readonly mutations: ResourceMutationService) {
    this.releaseFilesystemEvents = EventsOn(
      'fs:directoryChanged',
      (event: DirectoryChangedEvent | null) => {
        const path = normalizePath(String(event?.path ?? '').trim());
        if (!path) return;
        this.mutations.publish({
          kind: 'modify',
          items: [{ source: path }],
          origin: 'filesystem-watch',
        });
      },
    );

    this.releaseMutations = this.mutations.subscribe(mutation => {
      // A mutation performed by this application already tells us Bin changed.
      // Reset the polling baseline so the next OS poll does not echo the same
      // logical change back into the bus a second time.
      if (
        mutation.origin === 'recycle-bin' &&
        mutationTouchesDirectory(mutation, BIN_ROOT)
      ) {
        this.binFingerprint = null;
      }

      for (const [token, record] of this.observations) {
        if (!record.location) continue;
        if (!mutationTouchesDirectory(mutation, record.location)) continue;
        void this.refreshObservation(token, record);
      }
    });
  }

  observe(
    paneId: string,
    refresh: () => void | Promise<void>,
  ): SourceObservation {
    const token = Symbol(paneId);
    const record: ObservationRecord = {
      paneId,
      location: '',
      refresh,
      refreshing: false,
      pending: false,
    };
    this.observations.set(token, record);

    let closed = false;
    return {
      setLocation: location => {
        if (closed) return;
        this.setObservationLocation(record, location);
      },
      close: () => {
        if (closed) return;
        closed = true;
        this.removeObservation(token, record);
      },
    };
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.releaseFilesystemEvents();
    this.releaseMutations();
    for (const [token, record] of [...this.observations]) {
      this.removeObservation(token, record);
    }
    this.stopBinPolling();
  }

  private setObservationLocation(
    record: ObservationRecord,
    location: string,
  ): void {
    const next = normalizePath(String(location ?? '').trim());
    if (record.location === next) return;

    const wasBin = this.isBin(record.location);
    const isBin = this.isBin(next);
    record.location = next;

    if (wasBin !== isBin) {
      if (isBin) this.acquireBinObservation();
      else this.releaseBinObservation();
    }

    // The backend watcher is pane-scoped but internally ref-counts identical
    // native directories. Clearing it for logical sources prevents fsnotify
    // from being asked to watch bin://, registry://, archives, etc.
    const watchLocation = this.isFilesystem(next) ? next : '';
    void this.setBackendPaneDirectory(record.paneId, watchLocation);
  }

  private removeObservation(
    token: symbol,
    record: ObservationRecord,
  ): void {
    this.observations.delete(token);
    if (this.isBin(record.location)) this.releaseBinObservation();
    void this.setBackendPaneDirectory(record.paneId, '');
  }

  private async refreshObservation(
    token: symbol,
    record: ObservationRecord,
  ): Promise<void> {
    if (!this.observations.has(token)) return;

    if (record.refreshing) {
      record.pending = true;
      return;
    }

    record.refreshing = true;
    try {
      do {
        record.pending = false;
        if (!this.observations.has(token)) return;
        await record.refresh();
      } while (record.pending && this.observations.has(token));
    } catch (error) {
      console.warn('Source change refresh failed', error);
    } finally {
      record.refreshing = false;
    }
  }

  private async setBackendPaneDirectory(
    paneId: string,
    location: string,
  ): Promise<void> {
    try {
      const app = window.go?.app?.App as unknown as {
        SetPaneDirectory?: (paneId: string, path: string) => Promise<void>;
      };
      if (typeof app?.SetPaneDirectory !== 'function') return;
      await app.SetPaneDirectory(paneId, location);
    } catch (error) {
      console.warn('Unable to update directory watcher', error);
    }
  }

  private acquireBinObservation(): void {
    this.binObserverCount += 1;
    if (this.binObserverCount !== 1) return;
    this.binFingerprint = null;
    this.scheduleBinPoll(0);
  }

  private releaseBinObservation(): void {
    this.binObserverCount = Math.max(0, this.binObserverCount - 1);
    if (this.binObserverCount !== 0) return;
    this.stopBinPolling();
    this.binFingerprint = null;
  }

  private scheduleBinPoll(delay: number): void {
    if (this.destroyed || this.binObserverCount === 0) return;
    if (this.binPollTimer !== null) clearTimeout(this.binPollTimer);
    this.binPollTimer = setTimeout(() => {
      this.binPollTimer = null;
      void this.pollBin();
    }, delay);
  }

  private async pollBin(): Promise<void> {
    if (
      this.destroyed ||
      this.binObserverCount === 0 ||
      this.binPollRunning
    ) {
      return;
    }

    this.binPollRunning = true;
    try {
      const app = window.go?.app?.App as unknown as {
        ListTrashEntries?: () => Promise<TrashFingerprintEntry[]>;
      };
      if (typeof app?.ListTrashEntries !== 'function') return;

      const entries = await app.ListTrashEntries();
      const next = this.trashFingerprint(entries ?? []);

      if (this.binFingerprint !== null && this.binFingerprint !== next) {
        this.mutations.publish({
          kind: 'modify',
          items: [{ source: BIN_ROOT }],
          origin: 'bin-os-observer',
        });
      }
      this.binFingerprint = next;
    } catch (error) {
      // Polling is a fallback observer. A transient Shell/OS failure must not
      // affect the pane or start a retry storm.
      console.warn('Recycle Bin observation failed', error);
    } finally {
      this.binPollRunning = false;
      if (this.binObserverCount > 0 && !this.destroyed) {
        const delay =
          typeof document !== 'undefined' &&
          document.visibilityState === 'hidden'
            ? BIN_HIDDEN_POLL_MS
            : BIN_VISIBLE_POLL_MS;
        this.scheduleBinPoll(delay);
      }
    }
  }

  private stopBinPolling(): void {
    if (this.binPollTimer !== null) {
      clearTimeout(this.binPollTimer);
      this.binPollTimer = null;
    }
  }

  private trashFingerprint(
    entries: readonly TrashFingerprintEntry[],
  ): string {
    return entries
      .map(entry => [
        String(entry.id ?? ''),
        String(entry.deletedAt ?? ''),
        String(entry.size ?? ''),
        normalizePath(String(entry.originalPath ?? '')),
      ].join('\u0001'))
      .sort()
      .join('\u0002');
  }

  private isBin(location: string): boolean {
    return /^bin:\/\/(?:|.*)$/i.test(location);
  }

  private isFilesystem(location: string): boolean {
    if (!location) return false;
    if (/^filesystem:\/\//i.test(location)) return true;
    return !/^[a-z][a-z0-9+.-]*:\/\//i.test(location);
  }
}
