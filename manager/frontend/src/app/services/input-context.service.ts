import { Injectable } from '@angular/core';
import { InputEngineService } from './input-engine.service';

@Injectable({
  providedIn: 'root'
})
export class InputContextService {
  private readonly leaseCounts = new Map<string, number>();

  constructor(private inputEngine: InputEngineService) {}

  acquire(contextId: string): () => void {
    const nextCount = (this.leaseCounts.get(contextId) ?? 0) + 1;
    this.leaseCounts.set(contextId, nextCount);
    if (nextCount === 1) {
      this.inputEngine.activateContext(contextId);
    }

    let released = false;
    return () => {
      if (released) return;
      released = true;
      this.release(contextId);
    };
  }

  private release(contextId: string): void {
    const currentCount = this.leaseCounts.get(contextId);
    if (!currentCount) return;

    if (currentCount <= 1) {
      this.leaseCounts.delete(contextId);
      this.inputEngine.deactivateContext(contextId);
      return;
    }

    this.leaseCounts.set(contextId, currentCount - 1);
  }
}