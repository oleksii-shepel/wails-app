import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, EventEmitter, Input, Output } from '@angular/core';

import type { FileInfo } from '../shared/protocol';
import { SYSTEM_SURFACES } from '../services/system-surface-definitions';
import { systemField, type SystemSurfaceDescriptor } from '../services/system-surface-panel-source';
import type { PanelSourcePaneSession } from './panel-source-pane-session';
import { schemeOf } from './pane-renderer-registry.service';

@Component({
  selector: 'app-system-table-pane',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="surface">
      <div class="title-row">
        <strong>{{ descriptor?.title || 'System' }}</strong>
        <span>{{ session.entries.length }} items</span>
      </div>

      <div class="table-header" [style.grid-template-columns]="gridTemplate">
        <span>Name</span>
        <span *ngFor="let column of descriptor?.columns">{{ column.label }}</span>
      </div>

      <div
        class="rows"
        data-pane-content
        tabindex="-1"
        role="listbox"
        (scroll)="session.onScroll($event)"
        (click)="backgroundClick.emit($event)"
        (contextmenu)="backgroundContextMenu.emit($event)">
        <div
          *ngFor="let entry of session.entries; trackBy: track"
          class="row"
          [style.grid-template-columns]="gridTemplate"
          [class.selected]="session.selectedPaths.has(entry.path)"
          [class.cursor]="session.cursorPath === entry.path"
          (click)="entrySelect.emit({ entry: entry, event: $event })"
          (dblclick)="entryOpen.emit(entry)"
          (contextmenu)="entryContextMenu.emit({ entry: entry, event: $event })">
          <span class="name" [title]="entry.name">{{ entry.name }}</span>
          <span
            *ngFor="let column of descriptor?.columns"
            class="field"
            [title]="field(entry,column.field)">
            {{ field(entry, column.field) }}
          </span>
        </div>

        <div class="empty" *ngIf="!session.loading && session.entries.length === 0">
          No items.
        </div>
      </div>
    </section>
  `,
  styles: [`
    :host,.surface{display:flex;flex:1 1 0%;min-width:0;min-height:0;overflow:hidden}
    .surface{flex-direction:column;background:var(--vsc-editor-background,#fff)}
    .title-row{display:flex;align-items:center;justify-content:space-between;min-height:30px;padding:0 9px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border));font-size:11px}
    .title-row span{color:var(--vsc-description-foreground,var(--vsc-foreground-dim));font-size:10px}
    .table-header,.row {display:grid;      column-gap: var(--pane-column-gap, 6px);align-items:center}
    .table-header{min-height:26px;padding:0 8px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border));font-size:10px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    .rows{flex:1 1 0%;min-height:0;overflow:auto;background:var(--vsc-editor-background,#fff);outline:none}
    .row{min-height:30px;padding:2px 8px;font-size:11px;cursor:default}
    .row:hover{background:var(--vsc-list-hover-background)}
    .row.selected{background:var(--vsc-list-active-background);color:var(--vsc-list-active-foreground,var(--vsc-foreground))}
    .row.cursor{box-shadow:inset 0 0 0 1px var(--vsc-focus-border,var(--vsc-accent-blue))}
    .name,.field{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .field{color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    .row.selected .field{color:inherit}
    .empty{padding:28px;text-align:center;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}

    /* visible pane column gutters */
    .table-header,
    .row {
      column-gap: var(--pane-column-gutter, 12px);
    }

    .table-header > span,
    .row > span {
      min-width: 0;
      padding-inline: 2px;
    }
  `],
})
export class SystemTablePaneComponent {
  @Input({ required: true }) session!: PanelSourcePaneSession;
  @Output() readonly backgroundClick = new EventEmitter<MouseEvent>();
  @Output() readonly backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() readonly entrySelect = new EventEmitter<{ entry: FileInfo; event: MouseEvent }>();
  @Output() readonly entryOpen = new EventEmitter<FileInfo>();
  @Output() readonly entryContextMenu = new EventEmitter<{ entry: FileInfo; event: MouseEvent }>();

  get descriptor(): SystemSurfaceDescriptor | undefined {
    const scheme = schemeOf(this.session.path);
    return SYSTEM_SURFACES.find(candidate => candidate.kind === scheme);
  }

  get gridTemplate(): string {
    const columns = this.descriptor?.columns ?? [];
    return [
      'minmax(160px, .8fr)',
      ...columns.map(column => column.width ?? 'minmax(140px, 1fr)'),
    ].join(' ');
  }

  field(entry: FileInfo, field: string): string { return systemField(entry, field); }
  track(_index: number, entry: FileInfo): string { return entry.path; }
}
