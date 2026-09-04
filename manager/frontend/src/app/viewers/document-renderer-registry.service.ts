import { Injectable } from '@angular/core';
import type { DocumentRendererContribution } from '../plugin/plugin-registry-api';
import type { ViewerRequest } from './viewer-contribution';

@Injectable({ providedIn: 'root' })
export class DocumentRendererRegistryService {
  private readonly renderers: DocumentRendererContribution[] = [];

  register(renderer: DocumentRendererContribution): void {
    const index = this.renderers.findIndex(candidate => candidate.id === renderer.id);
    if (index >= 0) this.renderers.splice(index, 1, renderer);
    else this.renderers.push(renderer);
  }

  unregister(id: string): void {
    const index = this.renderers.findIndex(candidate => candidate.id === id);
    if (index >= 0) this.renderers.splice(index, 1);
  }

  resolve(request: ViewerRequest): DocumentRendererContribution | null {
    return this.renderers
      .map(renderer => ({ renderer, score: renderer.score(request) }))
      .filter(candidate => candidate.score > 0)
      .sort((left, right) => right.score - left.score || left.renderer.id.localeCompare(right.renderer.id))
      .map(candidate => candidate.renderer)[0] ?? null;
  }
}
