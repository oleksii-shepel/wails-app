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
  async sync(request: NativeBrowserPaneSyncRequest): Promise<void> {
    const api = window.go?.app?.App;
    if (!api?.BrowserPaneSync) {
      return;
    }
    await api.BrowserPaneSync(request);
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
