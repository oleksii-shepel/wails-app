import { Injectable } from '@angular/core';

import type { FileInfo } from '../shared/protocol';
import { FileType } from '../shared/protocol';
import { ApiService } from './api.service';

export type ThumbnailPriority = 'visible' | 'overscan' | 'idle';

export interface ThumbnailRequestOptions {
  readonly scope: string;
  readonly generation: number;
  readonly priority: ThumbnailPriority;
  readonly width?: number;
  readonly height?: number;
}

interface ThumbnailCacheEntry {
  readonly key: string;
  readonly objectUrl: string;
  readonly decodedImage: HTMLImageElement;
  readonly decodedBytes: number;
  lastUsed: number;
}

interface PendingThumbnailRequest {
  readonly key: string;
  readonly cacheKey: string;
  readonly entry: FileInfo;
  readonly scope: string;
  readonly generation: number;
  readonly priority: number;
  readonly sequence: number;
  readonly width: number;
  readonly height: number;
  readonly resolve: (url: string | null) => void;
}

interface ScopeState {
  generation: number;
  controller: AbortController;
}

const IMAGE_EXTENSIONS = new Set([
  'avif', 'bmp', 'gif', 'heic', 'heif', 'hdr', 'jfif', 'jpe', 'jpeg', 'jpg',
  'jxl', 'jp2', 'png', 'pnm', 'ppm', 'psd', 'svg', 'tif', 'tiff', 'webp',
]);

const VIDEO_EXTENSIONS = new Set([
  'avi', 'm4v', 'mkv', 'mov', 'mp4', 'mpeg', 'mpg', 'webm', 'wmv',
]);

const PRIORITY: Record<ThumbnailPriority, number> = {
  visible: 0,
  overscan: 1,
  idle: 2,
};

@Injectable({ providedIn: 'root' })
export class ThumbnailPipelineService {
  private readonly cache = new Map<string, ThumbnailCacheEntry>();
  private readonly pendingByKey = new Map<string, PendingThumbnailRequest[]>();
  private readonly queue: PendingThumbnailRequest[] = [];
  private readonly scopes = new Map<string, ScopeState>();

  private active = 0;
  private sequence = 0;
  private decodedBytes = 0;

  private readonly maxConcurrency = 4;
  private readonly maxDecodedEntries = 192;
  private readonly maxDecodedBytes = 96 * 1024 * 1024;

  constructor(private readonly api: ApiService) {}

  beginGeneration(scope: string): number {
    const previous = this.scopes.get(scope);
    previous?.controller.abort();

    const generation = (previous?.generation ?? 0) + 1;
    this.scopes.set(scope, {
      generation,
      controller: new AbortController(),
    });

    this.dropStaleQueued(scope, generation);
    return generation;
  }

  cancelScope(scope: string): void {
    const state = this.scopes.get(scope);
    state?.controller.abort();
    this.scopes.delete(scope);

    for (let index = this.queue.length - 1; index >= 0; index -= 1) {
      const request = this.queue[index];
      if (request.scope !== scope) continue;
      this.queue.splice(index, 1);
      request.resolve(null);
      this.removePendingReference(request);
    }
  }

  isCurrent(scope: string, generation: number): boolean {
    return this.scopes.get(scope)?.generation === generation;
  }

  canThumbnail(entry: FileInfo): boolean {
    if (!entry?.path || entry.type === FileType.DIRECTORY) return false;
    const ext = this.extension(entry.name || entry.path);
    return IMAGE_EXTENSIONS.has(ext) || VIDEO_EXTENSIONS.has(ext);
  }

  peek(entry: FileInfo, width = 160, height = 120): string | null {
    const key = this.cacheKey(entry, width, height);
    const cached = this.cache.get(key);
    if (!cached) return null;
    cached.lastUsed = performance.now();
    return cached.objectUrl;
  }

  request(
    entry: FileInfo,
    options: ThumbnailRequestOptions,
  ): Promise<string | null> {
    const width = Math.max(32, Math.min(512, Math.round(options.width ?? 160)));
    const height = Math.max(32, Math.min(512, Math.round(options.height ?? 120)));

    if (!this.canThumbnail(entry)) {
      return Promise.resolve(null);
    }

    if (!this.isCurrent(options.scope, options.generation)) {
      return Promise.resolve(null);
    }

    const cacheKey = this.cacheKey(entry, width, height);
    const cached = this.cache.get(cacheKey);
    if (cached) {
      cached.lastUsed = performance.now();
      return Promise.resolve(cached.objectUrl);
    }

    // Deduplicate only within the same cancellable scope generation. A scroll
    // in the left pane must never cancel a matching thumbnail requested by the
    // right pane.
    const key = `${cacheKey}|${options.scope}|${options.generation}`;

    return new Promise<string | null>(resolve => {
      const request: PendingThumbnailRequest = {
        key,
        cacheKey,
        entry,
        scope: options.scope,
        generation: options.generation,
        priority: PRIORITY[options.priority],
        sequence: this.sequence++,
        width,
        height,
        resolve,
      };

      const existing = this.pendingByKey.get(key);
      if (existing) {
        existing.push(request);
      } else {
        this.pendingByKey.set(key, [request]);
        this.queue.push(request);
        this.queue.sort(
          (left, right) =>
            left.priority - right.priority ||
            left.sequence - right.sequence,
        );
        this.pump();
      }
    });
  }

  stats(): {
    readonly decodedEntries: number;
    readonly decodedBytes: number;
    readonly pending: number;
    readonly active: number;
  } {
    return {
      decodedEntries: this.cache.size,
      decodedBytes: this.decodedBytes,
      pending: this.queue.length,
      active: this.active,
    };
  }

  clear(): void {
    for (const cached of this.cache.values()) {
      this.disposeCached(cached);
    }
    this.cache.clear();
    this.decodedBytes = 0;
  }

  private pump(): void {
    while (this.active < this.maxConcurrency && this.queue.length) {
      const request = this.queue.shift()!;
      if (!this.isCurrent(request.scope, request.generation)) {
        this.resolvePending(request.key, null);
        continue;
      }

      this.active += 1;
      void this.execute(request)
        .finally(() => {
          this.active -= 1;
          this.pump();
        });
    }
  }

  private async execute(request: PendingThumbnailRequest): Promise<void> {
    const scope = this.scopes.get(request.scope);
    if (!scope || scope.generation !== request.generation) {
      this.resolvePending(request.key, null);
      return;
    }

    try {
      const alreadyCached = this.cache.get(request.cacheKey);
      if (alreadyCached) {
        alreadyCached.lastUsed = performance.now();
        this.resolvePending(request.key, alreadyCached.objectUrl);
        return;
      }

      const sourceUrl = await this.buildSourceUrl(
        request.entry,
        request.width,
        request.height,
      );

      if (
        !sourceUrl ||
        scope.controller.signal.aborted ||
        !this.isCurrent(request.scope, request.generation)
      ) {
        this.resolvePending(request.key, null);
        return;
      }

      const response = await fetch(sourceUrl, {
        signal: scope.controller.signal,
        cache: 'force-cache',
      });
      if (!response.ok) {
        this.resolvePending(request.key, null);
        return;
      }

      const blob = await response.blob();
      if (
        scope.controller.signal.aborted ||
        !this.isCurrent(request.scope, request.generation)
      ) {
        this.resolvePending(request.key, null);
        return;
      }

      const objectUrl = URL.createObjectURL(blob);
      const decoded = new Image();
      decoded.decoding = 'async';
      decoded.src = objectUrl;

      try {
        await decoded.decode();
      } catch {
        URL.revokeObjectURL(objectUrl);
        this.resolvePending(request.key, null);
        return;
      }

      if (
        scope.controller.signal.aborted ||
        !this.isCurrent(request.scope, request.generation)
      ) {
        decoded.src = '';
        URL.revokeObjectURL(objectUrl);
        this.resolvePending(request.key, null);
        return;
      }

      const decodedBytes = Math.max(
        blob.size,
        decoded.naturalWidth * decoded.naturalHeight * 4,
      );

      const cached: ThumbnailCacheEntry = {
        key: request.cacheKey,
        objectUrl,
        decodedImage: decoded,
        decodedBytes,
        lastUsed: performance.now(),
      };
      this.cache.set(request.cacheKey, cached);
      this.decodedBytes += decodedBytes;
      this.evictIfNeeded();

      this.resolvePending(request.key, objectUrl);
    } catch (error) {
      if (!(error instanceof DOMException && error.name === 'AbortError')) {
        // Unsupported/corrupt previews intentionally fall back to the file icon.
      }
      this.resolvePending(request.key, null);
    }
  }

  private async buildSourceUrl(
    entry: FileInfo,
    width: number,
    height: number,
  ): Promise<string | null> {
    const ext = this.extension(entry.name || entry.path);
    if (VIDEO_EXTENSIONS.has(ext)) {
      return await this.api.getVideoPreviewURL(entry.path);
    }
    if (ext === 'svg') {
      return await this.api.getStreamURL(entry.path);
    }
    return await this.api.getImageThumbnailURL(entry.path, width, height);
  }

  private resolvePending(key: string, value: string | null): void {
    const waiters = this.pendingByKey.get(key) ?? [];
    this.pendingByKey.delete(key);
    for (const waiter of waiters) {
      if (this.isCurrent(waiter.scope, waiter.generation)) {
        waiter.resolve(value);
      } else {
        waiter.resolve(null);
      }
    }
  }

  private removePendingReference(request: PendingThumbnailRequest): void {
    const waiters = this.pendingByKey.get(request.key);
    if (!waiters) return;
    const filtered = waiters.filter(waiter => waiter !== request);
    if (filtered.length) {
      this.pendingByKey.set(request.key, filtered);
    } else {
      this.pendingByKey.delete(request.key);
    }
  }

  private dropStaleQueued(scope: string, generation: number): void {
    for (let index = this.queue.length - 1; index >= 0; index -= 1) {
      const request = this.queue[index];
      if (
        request.scope !== scope ||
        request.generation === generation
      ) {
        continue;
      }
      this.queue.splice(index, 1);
      request.resolve(null);
      this.removePendingReference(request);
    }
  }

  private evictIfNeeded(): void {
    if (
      this.cache.size <= this.maxDecodedEntries &&
      this.decodedBytes <= this.maxDecodedBytes
    ) {
      return;
    }

    const entries = [...this.cache.values()]
      .sort((left, right) => left.lastUsed - right.lastUsed);

    for (const cached of entries) {
      if (
        this.cache.size <= this.maxDecodedEntries &&
        this.decodedBytes <= this.maxDecodedBytes
      ) {
        break;
      }
      this.cache.delete(cached.key);
      this.decodedBytes = Math.max(0, this.decodedBytes - cached.decodedBytes);
      this.disposeCached(cached);
    }
  }

  private disposeCached(cached: ThumbnailCacheEntry): void {
    cached.decodedImage.src = '';
    URL.revokeObjectURL(cached.objectUrl);
  }

  private cacheKey(entry: FileInfo, width: number, height: number): string {
    return `${entry.path}|${entry.modified ?? 0}|${entry.size ?? 0}|${width}x${height}`;
  }

  private extension(value: string): string {
    const leaf = value.split(/[\\/]/).pop() ?? value;
    const index = leaf.lastIndexOf('.');
    return index > 0 ? leaf.slice(index + 1).toLocaleLowerCase() : '';
  }
}
