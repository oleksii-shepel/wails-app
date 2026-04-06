import { Injectable } from '@angular/core';

declare const window: any;

export interface NativeBrowserPaneSyncRequest {
  surfaceId: string;
  url: string;
  theme: 'dark' | 'light';
  backgroundColor: string;
  visible: boolean;
  x: number;
  y: number;
  width: number;
  height: number;
}

@Injectable({
  providedIn: 'root',
})
export class NativeBrowserPaneService {
  /** True once the Wails Go runtime has confirmed it's handling calls. */
  private _ready = false;

  get runtimeReady(): boolean {
    return this._ready;
  }

  async sync(request: NativeBrowserPaneSyncRequest): Promise<boolean> {
    const api = window.go?.app?.App;
    if (!api?.BrowserPaneSync) {
      return false;
    }
    if (!this._ready) {
      // Probe readiness: call a binding that returns a value.
      // If Wails callbacks aren't registered yet, the call resolves undefined.
      if (!await this.probeReady()) {
        return false;
      }
    }
    try {
      await api.BrowserPaneSync(request);
      return true;
    } catch {
      return false;
    }
  }

  /** Check if Go bindings are responsive by calling a function that returns a known value. */
  private async probeReady(): Promise<boolean> {
    try {
      // BrowserPaneCaptureScreenshot with empty surfaceId should return '' from Go.
      // If Wails isn't ready, it resolves undefined.
      const api = window.go?.app?.App;
      if (!api?.BrowserPaneCaptureScreenshot) {
        return false;
      }
      const result = await api.BrowserPaneCaptureScreenshot('');
      // Go returns '' for empty surfaceId. If Wails isn't ready, result is undefined.
      if (result !== undefined) {
        this._ready = true;
        return true;
      }
      return false;
    } catch {
      // Go returns an error for empty surfaceId — but at least it responded!
      this._ready = true;
      return true;
    }
  }

  async dispose(surfaceId: string): Promise<void> {
    if (!surfaceId) {
      return;
    }
    const api = window.go?.app?.App;
    if (!api?.BrowserPaneDispose) {
      return;
    }
    await api.BrowserPaneDispose(surfaceId);
  }

  async captureScreenshot(surfaceId: string): Promise<string> {
    if (!surfaceId) {
      return '';
    }
    const api = window.go?.app?.App;
    if (!api?.BrowserPaneCaptureScreenshot) {
      return '';
    }
    return await api.BrowserPaneCaptureScreenshot(surfaceId);
  }
}
