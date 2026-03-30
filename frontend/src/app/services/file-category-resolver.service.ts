import { Injectable } from '@angular/core';
import { FileInfo, FileProbe, FileType } from '../shared/protocol';
import { ApiService } from './api.service';

type ResolvedCategoryId = 'code' | 'video';
type CollisionExtension = 'ts' | 'mts';

interface CollisionResolutionContext {
  entry: FileInfo;
  probe(): Promise<FileProbe>;
}

interface CollisionSpec {
  fallback: ResolvedCategoryId;
  resolve: (ctx: CollisionResolutionContext) => Promise<ResolvedCategoryId>;
}

@Injectable({
  providedIn: 'root',
})
export class FileCategoryResolverService {
  private readonly resolvedByPath = new Map<string, ResolvedCategoryId>();
  private readonly pendingPaths = new Set<string>();
  private readonly listenersByPath = new Map<string, Set<() => void>>();
  private readonly collisionSpecs: Record<CollisionExtension, CollisionSpec> = {
    ts: {
      fallback: 'code',
      resolve: (ctx) => this.resolveTypeScriptOrTransportStream(ctx),
    },
    mts: {
      fallback: 'code',
      resolve: (ctx) => this.resolveTypeScriptOrTransportStream(ctx),
    },
  };

  constructor(private readonly api: ApiService) {}

  getResolvedCategory(path: string): ResolvedCategoryId | undefined {
    return this.resolvedByPath.get(path);
  }

  watch(path: string, listener: () => void): () => void {
    let listeners = this.listenersByPath.get(path);
    if (!listeners) {
      listeners = new Set();
      this.listenersByPath.set(path, listeners);
    }
    listeners.add(listener);

    return () => {
      const current = this.listenersByPath.get(path);
      if (!current) return;
      current.delete(listener);
      if (!current.size) {
        this.listenersByPath.delete(path);
      }
    };
  }

  ensureResolved(entry: FileInfo | null | undefined): void {
    const extension = this.getCollisionExtension(entry);
    if (!entry || !extension) return;
    if (this.pendingPaths.has(entry.path) || this.resolvedByPath.has(entry.path)) return;

    this.pendingPaths.add(entry.path);
    void this.resolveAndCache(entry, extension);
  }

  private getCollisionExtension(entry: FileInfo | null | undefined): CollisionExtension | null {
    if (!entry || entry.type !== FileType.FILE || !entry.path) return null;
    const extension = this.getExtension(entry.name);
    return extension === 'ts' || extension === 'mts' ? extension : null;
  }

  private async resolveAndCache(entry: FileInfo, extension: CollisionExtension): Promise<void> {
    const spec = this.collisionSpecs[extension];
    let resolved = spec.fallback;

    try {
      const context = this.createResolutionContext(entry, extension);
      resolved = await spec.resolve(context);
    } catch {
      resolved = spec.fallback;
    } finally {
      this.pendingPaths.delete(entry.path);
      this.resolvedByPath.set(entry.path, resolved);
      this.notify(entry.path);
    }
  }

  private createResolutionContext(entry: FileInfo, extension: CollisionExtension): CollisionResolutionContext {
    let probePromise: Promise<FileProbe> | null = null;

    return {
      entry,
      probe: () => {
        probePromise ??= this.api.probeFile(entry.path);
        return probePromise;
      },
    };
  }

  private async resolveTypeScriptOrTransportStream(ctx: CollisionResolutionContext): Promise<ResolvedCategoryId> {
    const probe = await ctx.probe();
    return this.looksLikeTransportStream(probe) ? 'video' : 'code';
  }

  private looksLikeTransportStream(probe: FileProbe): boolean {
    const mime = (probe.mimeType ?? '').split(';', 1)[0].trim().toLowerCase();
    return mime === 'video/mp2t' || mime === 'application/mp2t' || !probe.isText;
  }

  private getExtension(name: string | null | undefined): string {
    const value = (name ?? '').trim().toLowerCase();
    const lastDot = value.lastIndexOf('.');
    return lastDot >= 0 ? value.slice(lastDot + 1) : '';
  }

  private notify(path: string): void {
    const listeners = this.listenersByPath.get(path);
    if (!listeners) return;
    for (const listener of listeners) {
      listener();
    }
  }
}
