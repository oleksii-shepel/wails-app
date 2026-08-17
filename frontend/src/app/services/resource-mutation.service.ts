import { Injectable } from '@angular/core';
import { normalizePath, parentPath, samePath } from '../components/address-bar.utils';

export type ResourceMutationKind = 'create' | 'delete' | 'move' | 'copy' | 'modify' | 'restore';

export interface ResourceMutationItem {
  source?: string;
  destination?: string;
}

export interface ResourceMutation {
  readonly kind: ResourceMutationKind;
  readonly items: readonly ResourceMutationItem[];
  readonly origin?: string;
  readonly timestamp: number;
}

export type ResourceMutationListener = (mutation: ResourceMutation) => void | Promise<void>;

function canonical(path: string): string {
  return normalizePath(path || '');
}

export function isSameOrDescendantPath(path: string, ancestor: string): boolean {
  if (!path || !ancestor) return false;
  if (samePath(path, ancestor)) return true;
  const target = canonical(path);
  const base = canonical(ancestor);
  if (!target || !base) return false;
  const prefix = base.endsWith('/') ? base : `${base}/`;
  return target.toLocaleLowerCase().startsWith(prefix.toLocaleLowerCase());
}

export function remapDescendantPath(path: string, source: string, destination: string): string | null {
  if (!isSameOrDescendantPath(path, source)) return null;
  const target = canonical(path);
  const from = canonical(source);
  const to = canonical(destination);
  if (samePath(target, from)) return to;
  const suffix = target.slice(from.length).replace(/^\/+/, '');
  return suffix ? `${to.replace(/\/+$/, '')}/${suffix}` : to;
}

export function deletionRecoveryPath(path: string, deletedPaths: readonly string[]): string | null {
  const covering = deletedPaths
    .filter(candidate => isSameOrDescendantPath(path, candidate))
    .sort((left, right) => canonical(right).length - canonical(left).length)[0];
  return covering ? parentPath(covering) : null;
}

export function mutationTouchesDirectory(mutation: ResourceMutation, directory: string): boolean {
  const dir = canonical(directory);
  if (!dir) return false;
  return mutation.items.some(item => {
    const sourceParent = item.source ? parentPath(item.source) : '';
    const destinationParent = item.destination ? parentPath(item.destination) : '';
    if (sourceParent && samePath(sourceParent, dir)) return true;
    if (destinationParent && samePath(destinationParent, dir)) return true;
    if (mutation.kind === 'modify' && item.source && samePath(item.source, dir)) return true;
    return false;
  });
}

@Injectable({ providedIn: 'root' })
export class ResourceMutationService {
  private readonly listeners = new Set<ResourceMutationListener>();

  subscribe(listener: ResourceMutationListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  publish(input: Omit<ResourceMutation, 'timestamp'> & { timestamp?: number }): void {
    const items = input.items
      .map(item => ({
        source: item.source ? canonical(item.source) : undefined,
        destination: item.destination ? canonical(item.destination) : undefined,
      }))
      .filter(item => !!item.source || !!item.destination);
    if (!items.length) return;

    const mutation: ResourceMutation = {
      ...input,
      items,
      timestamp: input.timestamp ?? Date.now(),
    };
    for (const listener of [...this.listeners]) {
      void Promise.resolve(listener(mutation)).catch(error => {
        console.warn('Resource mutation listener failed', error);
      });
    }
  }
}
