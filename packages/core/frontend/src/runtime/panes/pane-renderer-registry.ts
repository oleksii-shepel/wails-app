export interface PaneRendererMatch<TKind extends string = string> {
  readonly kind: TKind;
  readonly scheme: string;
}

export interface PaneRendererContribution<TKind extends string = string> {
  readonly id: string;
  readonly kind: TKind;
  score(scheme: string, path: string): number;
}

/** Framework-neutral renderer resolver. Angular/React/etc. adapters live outside core. */
export class PaneRendererRegistry<TKind extends string = string> {
  private readonly contributions = new Map<string, PaneRendererContribution<TKind>>();

  constructor(private readonly fallbackKind: TKind) {}

  register(contribution: PaneRendererContribution<TKind>): () => void {
    this.contributions.set(contribution.id, contribution);
    return () => this.contributions.delete(contribution.id);
  }

  resolve(path: string): PaneRendererMatch<TKind> {
    const scheme = schemeOf(path);
    let winner: PaneRendererContribution<TKind> | null = null;
    let score = 0;

    for (const candidate of this.contributions.values()) {
      const next = candidate.score(scheme, path);
      if (next > score) {
        score = next;
        winner = candidate;
      }
    }

    return { kind: winner?.kind ?? this.fallbackKind, scheme };
  }
}

export function schemeOf(path: string): string {
  return /^([a-z][a-z0-9+.-]*):/i
    .exec(String(path ?? '').trim())?.[1]?.toLowerCase() ?? '';
}
