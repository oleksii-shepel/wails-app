export interface ScrollPeerSnapshot {
  readonly topName: string | null;
  readonly ratio: number;
}

export function normalizeScrollRatio(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(1, value));
}

export function chooseScrollAlignment(
  snapshot: ScrollPeerSnapshot,
  peerNames: readonly string[],
): { mode:'name';name:string } | { mode:'ratio';ratio:number } {
  if (snapshot.topName) {
    const folded = snapshot.topName.toLowerCase();
    const match = peerNames.find(name => name.toLowerCase() === folded);
    if (match) return { mode:'name', name:match };
  }
  return { mode:'ratio', ratio:normalizeScrollRatio(snapshot.ratio) };
}
