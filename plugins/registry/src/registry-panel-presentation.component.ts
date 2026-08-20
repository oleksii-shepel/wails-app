import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { RegistryPaneComponent } from './registry-pane.component';
import type { RegistryFileInfo } from './registry-model';

export interface RegistryPanelPresentationModel {
  readonly paneId: 'left' | 'right';
  readonly path: string;
  readonly viewMode: 'list' | 'grid' | 'details' | 'tree';
  readonly entries: readonly RegistryFileInfo[];
  readonly selectedPaths: ReadonlySet<string>;
  readonly cursorPath: string | null;
  readonly visibleEntryCount: number;
  readonly renderedStart: number;
  readonly virtualEnabled: boolean;
  readonly beforePx: number;
  readonly afterPx: number;
  readonly loading: boolean;
  readonly filterText: string;
  readonly dragOverPath: string | null;
  readonly dropIntent: 'copy' | 'move' | 'invalid' | null;
  readonly backgroundDropActive: boolean;
  readonly host: {
    scroll(event: Event): void;
    backgroundClick(event: MouseEvent): void;
    backgroundContextMenu(event: MouseEvent): void;
    backgroundDragOver(event: DragEvent): void;
    backgroundDragLeave(event: DragEvent): void;
    backgroundDrop(event: DragEvent): void;
    select(entry: RegistryFileInfo, event: MouseEvent): void;
    open(entry: RegistryFileInfo): void;
    contextMenu(entry: RegistryFileInfo, event: MouseEvent): void;
    dragStart(entry: RegistryFileInfo, event: DragEvent): void;
    dragEnd(): void;
    dragOver(entry: RegistryFileInfo, event: DragEvent): void;
    dragLeave(entry: RegistryFileInfo): void;
    drop(entry: RegistryFileInfo, event: DragEvent): void;
  };
}

@Component({
  selector: 'reassembly-registry-panel-presentation',
  standalone: true,
  imports: [CommonModule, RegistryPaneComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-registry-pane
      [paneId]="model.paneId"
      [path]="model.path"
      [viewMode]="model.viewMode === 'details' ? 'details' : 'list'"
      [entries]="model.entries"
      [selectedPaths]="model.selectedPaths"
      [cursorPath]="model.cursorPath"
      [visibleEntryCount]="model.visibleEntryCount"
      [renderedStart]="model.renderedStart"
      [virtualEnabled]="model.virtualEnabled"
      [beforePx]="model.beforePx"
      [afterPx]="model.afterPx"
      [loading]="model.loading"
      [filterText]="model.filterText"
      [dragOverPath]="model.dragOverPath"
      [dropIntent]="model.dropIntent"
      [backgroundDropActive]="model.backgroundDropActive"
      (scrolled)="model.host.scroll($event)"
      (backgroundClick)="model.host.backgroundClick($event)"
      (backgroundContextMenu)="model.host.backgroundContextMenu($event)"
      (backgroundDragOver)="model.host.backgroundDragOver($event)"
      (backgroundDragLeave)="model.host.backgroundDragLeave($event)"
      (backgroundDrop)="model.host.backgroundDrop($event)"
      (entrySelect)="model.host.select($event.entry, $event.event)"
      (entryOpen)="model.host.open($event)"
      (entryContextMenu)="model.host.contextMenu($event.entry, $event.event)"
      (entryDragStart)="model.host.dragStart($event.entry, $event.event)"
      (entryDragEnd)="model.host.dragEnd()"
      (entryDragOver)="model.host.dragOver($event.entry, $event.event)"
      (entryDragLeave)="model.host.dragLeave($event)"
      (entryDrop)="model.host.drop($event.entry, $event.event)">
    </app-registry-pane>
  `,
  styles: [`
    :host, app-registry-pane {
      display: flex;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }
  `],
})
export class RegistryPanelPresentationComponent {
  @Input({ required: true }) model!: RegistryPanelPresentationModel;
}
