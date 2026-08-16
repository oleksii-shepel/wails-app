import { Injectable } from '@angular/core';
import { ApiService } from './api.service';

export interface ReassemblyDriveChange {
  readonly previous: readonly string[];
  readonly current: readonly string[];
}

@Injectable({ providedIn: 'root' })
export class ReassemblyDriveLifecycleService {
  private timer: ReturnType<typeof setInterval> | null = null;
  private lastSignature = '';
  private lastPaths: string[] = [];

  constructor(private readonly api: ApiService) {}

  start(
    onChange: (change: ReassemblyDriveChange) => void,
    intervalMs = 4000,
  ): () => void {
    this.stop();

    const poll = async () => {
      const response = await this.api.getDrives().catch(() => null);
      if (!response) return;

      const physical = (response.schemes ?? [])
        .filter(drive =>
          ['fixed', 'removable', 'network', 'cdrom', 'ramdisk']
            .includes(String(drive.schemeType).toLowerCase()),
        )
        .map(drive => drive.path)
        .filter(Boolean)
        .sort((a, b) => a.localeCompare(b));

      const signature = physical.join('\n');
      if (!this.lastSignature) {
        this.lastSignature = signature;
        this.lastPaths = physical;
        return;
      }

      if (signature !== this.lastSignature) {
        const previous = this.lastPaths;
        this.lastSignature = signature;
        this.lastPaths = physical;
        onChange({ previous, current: physical });
      }
    };

    void poll();
    this.timer = setInterval(() => void poll(), intervalMs);
    return () => this.stop();
  }

  stop(): void {
    if (this.timer !== null) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }
}
