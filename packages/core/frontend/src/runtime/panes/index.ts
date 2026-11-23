export interface PaneRendererContribution<TKind extends string = string> {
  readonly id: string;
  readonly kind: TKind;
  score(scheme: string, path: string): number;
}

export interface PaneRendererMatch<TKind extends string = string> {
  readonly kind: TKind;
  readonly contributionId: string | null;
}

export function schemeOf(path: string): string {
  const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec((path ?? '').trim());
  return match?.[1]?.toLowerCase() ?? '';
}

export class PaneRendererRegistry<TKind extends string = string> {
  private readonly contributions = new Map<string, PaneRendererContribution<TKind>>();

  constructor(private readonly fallbackKind: TKind) {}

  register(contribution: PaneRendererContribution<TKind>): () => void {
    this.contributions.set(contribution.id, contribution);
    return () => {
      this.contributions.delete(contribution.id);
    };
  }

  resolve(path: string): PaneRendererMatch<TKind> {
    const scheme = schemeOf(path);
    let best: PaneRendererContribution<TKind> | null = null;
    let bestScore = 0;

    for (const contribution of this.contributions.values()) {
      const score = contribution.score(scheme, path);
      if (score > bestScore) {
        bestScore = score;
        best = contribution;
      }
    }

    if (best) {
      return { kind: best.kind, contributionId: best.id };
    }

    return { kind: this.fallbackKind, contributionId: null };
  }
}
