import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, Input, Output } from '@angular/core';

import { AutorunPanelSource } from '../services/autorun-panel-source.service';
import { FileType, type FileInfo } from '../shared/protocol';
import type { PanelSourcePaneSession } from './panel-source-pane-session';

type AutorunEntry = FileInfo & {
  autorunId?: string;
  autorunCommand?: string;
  autorunExecutable?: string;
  autorunSourcePath?: string;
  autorunScope?: 'user' | 'machine';
  autorunOrigin?: 'registry-run' | 'registry-run-once' | 'startup-folder';
  autorunEnabled?: boolean;
};

@Component({
  selector: 'app-autorun-pane',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="surface">
      <div class="heading" *ngIf="hasLeafRows">
        <span></span>
        <span>Name</span>
        <span>Enabled</span>
        <span>Scope</span>
        <span>Source</span>
        <span>Command</span>
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
          *ngFor="let raw of session.entries; trackBy: track"
          class="row"
          [class.folder]="raw.type === directoryType"
          [class.selected]="session.selectedPaths.has(raw.path)"
          [class.cursor]="session.cursorPath === raw.path"
          [class.leaf]="raw.type !== directoryType"
          (click)="entrySelect.emit({ entry: raw, event: $event })"
          (dblclick)="entryOpen.emit(raw)"
          (contextmenu)="entryContextMenu.emit({ entry: raw, event: $event })">
          <span class="icon">{{ raw.type === directoryType ? '›' : 'A' }}</span>
          <span class="name">{{ raw.name }}</span>

          <ng-container *ngIf="asAutorun(raw) as entry">
            <span class="enabled" *ngIf="raw.type !== directoryType; else blank">
              <button
                type="button"
                class="toggle"
                [class.on]="entry.autorunEnabled"
                (click)="toggle(entry, $event)">
                {{ entry.autorunEnabled ? 'On' : 'Off' }}
              </button>
            </span>
            <span *ngIf="raw.type !== directoryType">{{ scope(entry) }}</span>
            <span *ngIf="raw.type !== directoryType">{{ origin(entry) }}</span>
            <span class="command" *ngIf="raw.type !== directoryType" [title]="entry.autorunCommand || ''">
              {{ entry.autorunCommand || '' }}
            </span>
          </ng-container>

          <ng-template #blank><span></span></ng-template>
        </div>

        <div class="empty" *ngIf="!session.loading && session.entries.length === 0">
          No autorun entries.
        </div>
      </div>
    </section>
  `,
  styles: [`
    :host,.surface{display:flex;flex:1 1 0%;min-width:0;min-height:0;overflow:hidden}
    .surface{flex-direction:column;background:var(--vsc-editor-background,#fff)}
    .heading,.row {display:grid;grid-template-columns:28px minmax(150px,.7fr) 74px 92px 130px minmax(240px,1.6fr);      column-gap: var(--pane-column-gap, 6px);align-items:center}
    .heading{min-height:26px;padding:0 8px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border));font-size:10px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    .rows{flex:1 1 0%;min-height:0;overflow:auto;outline:none;background:var(--vsc-editor-background,#fff)}
    .row{min-height:34px;padding:3px 8px;font-size:11px;cursor:default}
    .row.folder{grid-template-columns:28px minmax(0,1fr)}
    .row:hover{background:var(--vsc-list-hover-background)}
    .row.selected{background:var(--vsc-list-active-background);color:var(--vsc-list-active-foreground,var(--vsc-foreground))}
    .row.cursor{box-shadow:inset 0 0 0 1px var(--vsc-focus-border,var(--vsc-accent-blue))}
    .icon{opacity:.75;text-align:center}.name,.command{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .toggle{min-width:48px;border:1px solid var(--vsc-border);border-radius:999px;background:transparent;color:inherit;font:inherit}
    .toggle.on{background:color-mix(in srgb,var(--vsc-accent-blue) 15%,transparent);border-color:var(--vsc-accent-blue)}
    .empty{padding:28px;text-align:center;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}

    /* visible pane column gutters */
    .heading,
    .row {
      column-gap: var(--pane-column-gutter, 12px);
    }

    .heading > span:not(:first-child),
    .row > span:not(:first-child) {
      min-width: 0;
      padding-inline: 2px;
    }
  `],
})
export class AutorunPaneComponent {
  @Input({ required: true }) session!: PanelSourcePaneSession;
  @Output() readonly backgroundClick = new EventEmitter<MouseEvent>();
  @Output() readonly backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() readonly entrySelect = new EventEmitter<{ entry: FileInfo; event: MouseEvent }>();
  @Output() readonly entryOpen = new EventEmitter<FileInfo>();
  @Output() readonly entryContextMenu = new EventEmitter<{ entry: FileInfo; event: MouseEvent }>();

  readonly directoryType = FileType.DIRECTORY;

  constructor(
    private readonly source: AutorunPanelSource,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get hasLeafRows(): boolean {
    return this.session.entries.some(entry => entry.type !== FileType.DIRECTORY);
  }

  asAutorun(entry: FileInfo): AutorunEntry { return entry as AutorunEntry; }
  track(_index: number, entry: FileInfo): string { return entry.path; }

  scope(entry: AutorunEntry): string {
    return entry.autorunScope === 'machine' ? 'All Users' : entry.autorunScope === 'user' ? 'Current User' : '';
  }

  origin(entry: AutorunEntry): string {
    return String(entry.autorunOrigin ?? '').replaceAll('-', ' ');
  }

  async toggle(entry: AutorunEntry, event: MouseEvent): Promise<void> {
    event.stopPropagation();
    const id = entry.autorunId ?? this.source.decodeEntryPath(entry.path);
    if (!id || entry.autorunEnabled === undefined) return;
    await this.source.setEnabled(id, !entry.autorunEnabled);
    await this.session.refresh();
    this.cdr.markForCheck();
  }
}
