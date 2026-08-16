export class ReassemblyNavigationController {
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

    if (this.current === value) {
      return;
    }

    if (this.canForward) {
      this.entries = this.entries.slice(0, this.index + 1);
    }

    this.entries.push(value);
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
}
