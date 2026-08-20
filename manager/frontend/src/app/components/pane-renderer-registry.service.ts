import { Injectable } from '@angular/core';

export type PaneRendererKind =
  | 'directory'
  | 'registry'
  | 'autorun'
  | 'system-table';

export interface PaneRendererMatch {
  readonly kind: PaneRendererKind;
  readonly scheme: string;
}

export interface PaneRendererContribution {
  readonly id: string;
  score(scheme: string, path: string): number;
  readonly kind: Exclude<PaneRendererKind, 'directory'>;
}

/**
 * Chooses the concrete sibling pane implementation.
 *
 * Data providers remain PanelSource contributions. This registry answers only
 * the presentation-level question: which pane component owns this location?
 */
@Injectable({ providedIn: 'root' })
export class PaneRendererRegistryService {
  private readonly contributions = new Map<string, PaneRendererContribution>();

  constructor() {
    this.register({
      id: 'builtin.registry-pane',
      kind: 'registry',
      score: scheme => scheme === 'registry' ? 100 : 0,
    });
    this.register({
      id: 'builtin.autorun-pane',
      kind: 'autorun',
      score: scheme => scheme === 'autorun' ? 100 : 0,
    });
    this.register({
      id: 'builtin.system-table-pane',
      kind: 'system-table',
      score: scheme => SYSTEM_TABLE_SCHEMES.has(scheme) ? 100 : 0,
    });
  }

  register(contribution: PaneRendererContribution): () => void {
    this.contributions.set(contribution.id, contribution);
    return () => this.contributions.delete(contribution.id);
  }

  resolve(path: string): PaneRendererMatch {
    const scheme = schemeOf(path);
    let winner: PaneRendererContribution | null = null;
    let score = 0;

    for (const candidate of this.contributions.values()) {
      const next = candidate.score(scheme, path);
      if (next > score) {
        score = next;
        winner = candidate;
      }
    }

    return {
      kind: winner?.kind ?? 'directory',
      scheme,
    };
  }
}

export const SYSTEM_TABLE_SCHEMES = new Set([
  'tasks',
  'services',
  'processes',
  'devices',
  'network',
  'environment',
  'shell-extensions',
]);

export function schemeOf(path: string): string {
  return /^([a-z][a-z0-9+.-]*):/i
    .exec(String(path ?? '').trim())?.[1]?.toLowerCase() ?? '';
}
