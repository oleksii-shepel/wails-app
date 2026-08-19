import { Injectable } from '@angular/core';

import type { FileInfo } from '../shared/protocol';
import { FileType } from '../shared/protocol';
import { ApiService } from './api.service';
import {
  WorkSchedulerService,
  type WorkPriority,
} from './work-scheduler.service';

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

const IMAGE_EXTENSIONS = new Set([
  'avif', 'bmp', 'gif', 'heic', 'heif', 'hdr', 'jfif', 'jpe', 'jpeg', 'jpg',
  'jxl', 'jp2', 'png', 'pnm', 'ppm', 'psd', 'svg', 'tif', 'tiff', 'webp',
]);

const VIDEO_EXTENSIONS = new Set([
  'avi', 'm4v', 'mkv', 'mov', 'mp4', 'mpeg', 'mpg', 'webm', 'wmv',
]);

const THUMBNAIL_PRIORITY: Record<ThumbnailPriority, WorkPriority> = {
  visible: 'visible',
  overscan: 'prefetch',
  idle: 'idle',
};

@Injectable({ providedIn: 'root' })
export class ThumbnailPipelineService {
  private readonly cache = new Map<string, ThumbnailCacheEntry>();
  private decodedBytes = 0;

  private readonly maxDecodedEntries = 192;
  private readonly maxDecodedBytes = 96 * 1024 * 1024;

  constructor(
    private readonly api: ApiService,
    private readonly work: WorkSchedulerService,
  ) {}

  beginGeneration(scope: string): number {
    return this.work.beginGeneration(this.workScope(scope));
  }

  cancelScope(scope: string): void {
    this.work.cancelScope(this.workScope(scope));
  }

  isCurrent(scope: string, generation: number): boolean {
    return this.work.isCurrent(this.workScope(scope), generation);
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

    const scope = this.workScope(options.scope);
    if (!this.work.isCurrent(scope, options.generation)) {
      return Promise.resolve(null);
    }

    const cacheKey = this.cacheKey(entry, width, height);
    const cached = this.cache.get(cacheKey);
    if (cached) {
      cached.lastUsed = performance.now();
      return Promise.resolve(cached.objectUrl);
    }

    const handle = this.work.schedule<string | null>({
      key: `thumbnail:${cacheKey}:${scope}:${options.generation}`,
      scope,
      generation: options.generation,
      priority: THUMBNAIL_PRIORITY[options.priority],
      kind: 'thumbnail',
      run: async context => {
        const alreadyCached = this.cache.get(cacheKey);
        if (alreadyCached) {
          alreadyCached.lastUsed = performance.now();
          return alreadyCached.objectUrl;
        }

        const sourceUrl = await this.buildSourceUrl(entry, width, height);
        if (!sourceUrl || context.signal.aborted) return null;

        const response = await fetch(sourceUrl, {
          signal: context.signal,
          cache: 'force-cache',
        });
        if (!response.ok || context.signal.aborted) return null;

        const blob = await response.blob();
        if (context.signal.aborted) return null;

        const objectUrl = URL.createObjectURL(blob);
        const decoded = new Image();
        decoded.decoding = 'async';
        decoded.src = objectUrl;

        try {
          await decoded.decode();
        } catch {
          URL.revokeObjectURL(objectUrl);
          return null;
        }

        if (context.signal.aborted) {
          decoded.src = '';
          URL.revokeObjectURL(objectUrl);
          return null;
        }

        const decodedBytes = Math.max(
          blob.size,
          decoded.naturalWidth * decoded.naturalHeight * 4,
        );

        const item: ThumbnailCacheEntry = {
          key: cacheKey,
          objectUrl,
          decodedImage: decoded,
          decodedBytes,
          lastUsed: performance.now(),
        };
        this.cache.set(cacheKey, item);
        this.decodedBytes += decodedBytes;
        this.evictIfNeeded();

        context.report({
          completed: 1,
          total: 1,
          message: entry.name,
          units: 'thumbnail',
        });
        return objectUrl;
      },
    });

    return handle.promise.catch(error => {
      if (error instanceof DOMException && error.name === 'AbortError') {
        return null;
      }
      throw error;
    });
  }

  stats(): {
    readonly decodedEntries: number;
    readonly decodedBytes: number;
    readonly pending: number;
    readonly active: number;
  } {
    const stats = this.work.stats('thumbnail');
    return {
      decodedEntries: this.cache.size,
      decodedBytes: this.decodedBytes,
      pending: stats.queued,
      active: stats.running,
    };
  }

  clear(): void {
    for (const cached of this.cache.values()) {
      this.disposeCached(cached);
    }
    this.cache.clear();
    this.decodedBytes = 0;
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

  private workScope(scope: string): string {
    return `thumbnail:${scope}`;
  }

  private extension(value: string): string {
    const leaf = value.split(/[\\/]/).pop() ?? value;
    const index = leaf.lastIndexOf('.');
    return index > 0 ? leaf.slice(index + 1).toLocaleLowerCase() : '';
  }
}