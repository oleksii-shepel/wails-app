export interface ReassemblyNavigationState {
  entries: string[];
  index: number;
}

export class ReassemblyNavigationController {
  private readonly maxEntries = 200;
  private entries: string[] = [];
  private index = -1;

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
    const value = path.trim();
    if (!value) return;

    if (this.current === value) return;

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
    if (this.canBack && this.entries[this.index - 1] === path) {
      this.index -= 1;
    }
  }

  commitForward(path: string): void {
    if (this.canForward && this.entries[this.index + 1] === path) {
      this.index += 1;
    }
  }

  snapshot(): readonly string[] {
    return this.entries.slice();
  }

  snapshotState(): ReassemblyNavigationState {
    return {
      entries: this.entries.slice(),
      index: this.index,
    };
  }

  restore(state: ReassemblyNavigationState | null | undefined): void {
    if (!state || !Array.isArray(state.entries)) {
      this.entries = [];
      this.index = -1;
      return;
    }

    const entries = state.entries
      .filter((entry): entry is string => typeof entry === 'string' && !!entry.trim())
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
