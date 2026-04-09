import { Injectable } from '@angular/core';
import { EventsOn } from 'wailsjs/runtime';

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

export interface NativeBrowserPaneIncomingMessage<T = unknown> {
  surfaceId: string;
  message: string;
  data: T;
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

  async getDocumentBackground(surfaceId: string): Promise<string> {
    if (!surfaceId) {
      return '';
    }
    const api = window.go?.app?.App;
    if (!api?.BrowserPaneGetDocumentBackground) {
      return '';
    }
    try {
      return await api.BrowserPaneGetDocumentBackground(surfaceId);
    } catch {
      return '';
    }
  }

  async postMessage(surfaceId: string, message: unknown): Promise<boolean> {
    if (!surfaceId) {
      return false;
    }
    const api = window.go?.app?.App;
    if (!api?.BrowserPanePostMessage) {
      return false;
    }
    if (!this._ready && !await this.probeReady()) {
      return false;
    }
    try {
      await api.BrowserPanePostMessage(surfaceId, this.serializeMessage(message));
      return true;
    } catch {
      return false;
    }
  }

  onMessage(handler: (event: NativeBrowserPaneIncomingMessage) => void): (() => void) {
    return EventsOn('browser-pane:message', (event: { surfaceId?: string; message?: string } | null) => {
      const surfaceId = typeof event?.surfaceId === 'string' ? event.surfaceId : '';
      const message = typeof event?.message === 'string' ? event.message : '';
      if (!surfaceId) {
        return;
      }
      handler({
        surfaceId,
        message,
        data: this.parseMessage(message),
      });
    });
  }

  onSurfaceMessage(surfaceId: string, handler: (event: NativeBrowserPaneIncomingMessage) => void): (() => void) {
    return this.onMessage((event) => {
      if (event.surfaceId !== surfaceId) {
        return;
      }
      handler(event);
    });
  }

  private serializeMessage(message: unknown): string {
    const serialized = JSON.stringify(message);
    return serialized === undefined ? 'null' : serialized;
  }

  private parseMessage(message: string): unknown {
    try {
      return JSON.parse(message);
    } catch {
      return message;
    }
  }
}
