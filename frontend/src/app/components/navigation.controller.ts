export interface NavigationState {
  entries: string[];
  index: number;
}

export class NavigationController {
  private readonly maxEntries = 200;
  private entries: string[] = [];
  private index = -1;

  constructor(
    private readonly normalizer: (path: string) => string =
      path => path.trim(),
  ) {}


  get canBack(): boolean {
    return this.index > 0;
  }

  get canForward(): boolean {
    return this.index >= 0 && this.index < this.entries.length - 1;
  }

  get current(): string | null {
    return this.index >= 0 ? this.entries[this.index] ?? null : null;
  }

  record(path: string): void {
    const value = this.normalizer(path);
    if (!value) return;

    if (
      this.current !== null &&
      this.normalizer(this.current) === value
    ) return;

    if (this.canForward) {
      this.entries = this.entries.slice(0, this.index + 1);
    }

    this.entries.push(value);

    if (this.entries.length > this.maxEntries) {
      const overflow = this.entries.length - this.maxEntries;
      this.entries = this.entries.slice(overflow);
    }

    this.index = this.entries.length - 1;
  }

  backTarget(): string | null {
    return this.canBack ? this.entries[this.index - 1] ?? null : null;
  }

  forwardTarget(): string | null {
    return this.canForward ? this.entries[this.index + 1] ?? null : null;
  }

  commitBack(path: string): void {
    if (
      this.canBack &&
      this.normalizer(this.entries[this.index - 1] ?? '') ===
        this.normalizer(path)
    ) {
      this.index -= 1;
    }
  }

  commitForward(path: string): void {
    if (
      this.canForward &&
      this.normalizer(this.entries[this.index + 1] ?? '') ===
        this.normalizer(path)
    ) {
      this.index += 1;
    }
  }

  backTrail(limit = 12): readonly string[] {
    if (!this.canBack) return [];
    return this.entries.slice(Math.max(0, this.index - Math.max(1, limit)), this.index).reverse();
  }

  forwardTrail(limit = 12): readonly string[] {
    if (!this.canForward) return [];
    return this.entries.slice(this.index + 1, this.index + 1 + Math.max(1, limit));
  }

  indexOfTrailTarget(path: string): number {
    const value = this.normalizer(path);
    if (!value) return -1;
    let best = -1;
    let distance = Number.POSITIVE_INFINITY;
    for (let i = 0; i < this.entries.length; i += 1) {
      if (this.normalizer(this.entries[i] ?? '') !== value) continue;
      const nextDistance = Math.abs(i - this.index);
      if (nextDistance < distance) {
        best = i;
        distance = nextDistance;
      }
    }
    return best;
  }

  commitIndex(index: number, path: string): boolean {
    if (
      index < 0 ||
      index >= this.entries.length ||
      this.normalizer(this.entries[index] ?? '') !== this.normalizer(path)
    ) return false;
    this.index = index;
    return true;
  }

  snapshot(): readonly string[] {
    return this.entries.slice();
  }

  snapshotState(): NavigationState {
    return {
      entries: this.entries.slice(),
      index: this.index,
    };
  }

  restore(state: NavigationState | null | undefined): void {
    if (!state || !Array.isArray(state.entries)) {
      this.entries = [];
      this.index = -1;
      return;
    }

    const entries = state.entries
      .filter(
        (entry): entry is string =>
          typeof entry === 'string' && !!entry.trim(),
      )
      .map(entry => this.normalizer(entry))
      .filter(Boolean)
      .filter(
        (entry, index, all) =>
          index === 0 || entry !== all[index - 1],
      )
      .slice(-this.maxEntries);

    this.entries = entries;
    this.index = entries.length === 0
      ? -1
      : Math.max(0, Math.min(entries.length - 1, Number(state.index) || 0));
  }

  reset(path?: string): void {
    this.entries = [];
    this.index = -1;
    if (path?.trim()) this.record(path);
  }
}