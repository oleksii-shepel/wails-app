import { EventsOff, EventsOn, OnFileDrop, OnFileDropOff } from 'wailsjs/runtime/runtime';
import type { ApiService } from '../services/api.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';

export interface FileBrowserRuntimeHost {
  apiService: ApiService;
  confirmDialog: ConfirmDialogService;
  getElevationActive(): boolean;
  getElevationBusy(): boolean;
  setElevationActive(active: boolean): void;
  setElevationBusy(busy: boolean): void;
  handleNativeFileDrop(x: number, y: number, paths: string[]): Promise<void>;
}

export class FileBrowserRuntimeController {
  async initialize(host: FileBrowserRuntimeHost): Promise<void> {
    await this.checkServerConnection(host);
    await this.syncElevationState(host);

    EventsOn('elevation:session-starting', () => {
      host.setElevationBusy(true);
    });
    EventsOn('elevation:session-started', () => {
      host.setElevationActive(true);
      host.setElevationBusy(false);
    });
    EventsOn('elevation:session-stopped', () => {
      host.setElevationActive(false);
      host.setElevationBusy(false);
    });

    OnFileDrop((x: number, y: number, paths: string[]) => {
      console.log('[FileBrowser] 🎯🎯🎯 wails:file-drop received! x:', x, 'y:', y, 'paths:', paths);
      void host.handleNativeFileDrop(x, y, paths);
    }, false);
  }

  dispose(): void {
    EventsOff('elevation:session-starting', 'elevation:session-started', 'elevation:session-stopped');
    OnFileDropOff();
  }

  async toggleElevation(host: FileBrowserRuntimeHost): Promise<void> {
    if (host.getElevationBusy()) {
      return;
    }

    host.setElevationBusy(true);
    try {
      if (host.getElevationActive()) {
        await host.apiService.stopElevatedSession();
        host.setElevationActive(false);
        return;
      }

      const status = await host.apiService.startElevatedSession();
      host.setElevationActive(!!status?.active);
    } catch (error: any) {
      console.error('Elevation toggle failed:', error);
      host.confirmDialog.alert(`Elevation Error\n${error?.message ?? String(error)}`);
    } finally {
      host.setElevationBusy(false);
    }
  }

  private async checkServerConnection(host: FileBrowserRuntimeHost): Promise<void> {
    const isHealthy = await host.apiService.checkHealth();
    if (!isHealthy) {
      console.error('Cannot connect to server');
    }
  }

  private async syncElevationState(host: FileBrowserRuntimeHost): Promise<void> {
    try {
      const status = await host.apiService.getElevatedSessionStatus();
      host.setElevationActive(!!status?.active);
    } catch {
      host.setElevationActive(false);
    } finally {
      host.setElevationBusy(false);
    }
  }
}