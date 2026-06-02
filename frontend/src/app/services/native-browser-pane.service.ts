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

export interface NativeBrowserPaneLifecycleEvent {
  surfaceId: string;
  state: 'started' | 'completed' | 'failed';
  url: string;
}

@Injectable({
  providedIn: 'root',
})
export class NativeBrowserPaneService {
  /** True once the Wails Go runtime has confirmed it's handling calls. */
  private _ready = false;
  private pendingRpcs = new Map<string, { resolve: (value: unknown) => void; reject: (reason: Error) => void; timer: ReturnType<typeof setTimeout> }>();

  get runtimeReady(): boolean {
    return this._ready;
  }

  constructor() {
    // Always listen for RPC responses so pending promises can resolve
    EventsOn('browser-pane:rpc', (event: { surfaceId?: string; message?: string } | null) => {
      const message = typeof event?.message === 'string' ? event.message : '';
      if (!message) return;
      try {
        const data = JSON.parse(message) as { id?: string; result?: unknown; error?: string } | null;
        if (!data?.id) return;
        this.handleRpcResponse(data.id, data.result, data.error);
      } catch {
        // ignore non-JSON
      }
    });
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

  onLifecycleEvent(handler: (event: NativeBrowserPaneLifecycleEvent) => void): (() => void) {
    return EventsOn('browser-pane:lifecycle', (event: { surfaceId?: string; state?: string; url?: string } | null) => {
      const surfaceId = typeof event?.surfaceId === 'string' ? event.surfaceId : '';
      const state = typeof event?.state === 'string' ? event.state : '';
      const url = typeof event?.url === 'string' ? event.url : '';
      if (!surfaceId || !state) {
        return;
      }
      handler({
        surfaceId,
        state: state as NativeBrowserPaneLifecycleEvent['state'],
        url,
      });
    });
  }

  onSurfaceLifecycleEvent(surfaceId: string, handler: (event: NativeBrowserPaneLifecycleEvent) => void): (() => void) {
    return this.onLifecycleEvent((event) => {
      if (event.surfaceId !== surfaceId) {
        return;
      }
      handler(event);
    });
  }

  async whenNavigationCompleted(surfaceId: string, timeoutMs = 30000): Promise<void> {
    return new Promise((resolve, reject) => {
      const unsubscribe = this.onSurfaceLifecycleEvent(surfaceId, (event) => {
        if (event.state === 'completed') {
          unsubscribe();
          resolve();
        }
      });
      setTimeout(() => {
        unsubscribe();
        reject(new Error(`Navigation completion timed out for ${surfaceId}`));
      }, timeoutMs);
    });
  }

  async rpcCall<T = unknown>(
    surfaceId: string,
    type: string,
    payload?: unknown,
    timeoutMs = 5000,
  ): Promise<T> {
    if (!surfaceId) {
      return Promise.reject(new Error('surfaceId is required'));
    }
    const id = this.generateRpcId();
    const request = { _rpc: true, id, v: 1, type, payload };

    const success = await this.postMessage(surfaceId, request);
    if (!success) {
      return Promise.reject(new Error(`Failed to send RPC request to ${surfaceId}`));
    }

    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pendingRpcs.delete(id);
        reject(new Error(`RPC '${type}' timed out after ${timeoutMs}ms`));
      }, timeoutMs);
      this.pendingRpcs.set(id, {
        resolve: resolve as (value: unknown) => void,
        reject,
        timer,
      });
    });
  }

  private generateRpcId(): string {
    return `rpc_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
  }

  private handleRpcResponse(id: string, result: unknown, error: string | undefined): void {
    const pending = this.pendingRpcs.get(id);
    if (!pending) return;
    clearTimeout(pending.timer);
    this.pendingRpcs.delete(id);
    if (error) {
      pending.reject(new Error(error));
    } else {
      pending.resolve(result);
    }
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
