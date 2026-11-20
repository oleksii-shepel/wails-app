export function schemeOf(path: string): string {
  const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec((path ?? '').trim());
  return match?.[1]?.toLowerCase() ?? '';
}

export interface PaneRendererContribution<TKind extends string = string> {
  readonly id: string;
  readonly kind: TKind;
  score(scheme: string, path: string): number;
}

export interface PaneRendererMatch<TKind extends string = string> {
  readonly id: string;
  readonly kind: TKind;
  readonly score: number;
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
    let best: PaneRendererMatch<TKind> | null = null;

    for (const contribution of this.contributions.values()) {
      const score = contribution.score(scheme, path);
      if (score > 0 && (!best || score > best.score)) {
        best = {
          id: contribution.id,
          kind: contribution.kind,
          score,
        };
      }
    }

    return best ?? {
      id: `builtin.${this.fallbackKind}`,
      kind: this.fallbackKind,
      score: 0,
    };
  }
}
