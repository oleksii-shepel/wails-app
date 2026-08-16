import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, Input, ViewChild } from '@angular/core';
import { DirSurfaceComponent } from '../../components/dir-surface.component';
import { SearchResultsSurfaceContext } from '../surface-context';

@Component({
  selector: 'app-search-results-surface-tab',
  standalone: true,
  imports: [CommonModule, DirSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="search-summary">
      <span class="search-icon" aria-hidden="true">🔎</span>
      <span class="search-title">Search results</span>
      <span class="search-query" [title]="querySummary">{{ querySummary }}</span>
    </div>
    <app-dir-surface
      #dir
      [vm]="ctx.dirVm"
      [surfaceId]="ctx.paneId + ':' + ctx.tab.id"
      [pinSurfaceId]="'search-results'"
      [surfaceActive]="active"
      [surfacePath]="currentSurfacePath">
    </app-dir-surface>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    .search-summary {
      display: flex;
      align-items: center;
      gap: 6px;
      min-height: 26px;
      padding: 3px 8px;
      border-bottom: 1px solid var(--vscode-panel-border, var(--vsc-neutral-overlay-medium));
      background: var(--vscode-sideBar-background, transparent);
      font-size: 12px;
      min-width: 0;
    }

    .search-title {
      font-weight: 600;
      white-space: nowrap;
    }

    .search-query {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      opacity: .78;
    }

    app-dir-surface {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
  `],
})
export class SearchResultsSurfaceTabComponent {
  @Input({ required: true }) ctx!: SearchResultsSurfaceContext;
  @Input() active = false;
  @ViewChild('dir') private dir?: DirSurfaceComponent;

  get currentSurfacePath(): string {
    if (this.active) return this.ctx?.dirVm?.currentPath || this.ctx?.tab?.path || '';
    return this.ctx?.tab?.path || this.ctx?.dirVm?.currentPath || '';
  }

  get querySummary(): string {
    const location = this.ctx?.tab?.location?.location || this.ctx?.tab?.path || '';
    try {
      const raw = location.slice('search://'.length);
      const params = new URLSearchParams(raw.startsWith('?') ? raw.slice(1) : raw);
      const pattern = params.get('pattern') || params.get('q') || 'Search';
      const base = params.get('path');
      return base ? `${pattern} in ${base}` : pattern;
    } catch {
      return location || 'Search';
    }
  }

  get addressBar() { return this.dir?.addressBar; }
  get fileViewContainer() { return this.dir?.fileViewContainer; }
  get findWidget() { return this.dir?.findWidget; }
}