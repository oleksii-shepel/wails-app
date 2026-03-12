import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';

@Component({
  selector: 'app-status-bar',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="status-bar">
      <div class="status-left">

        <!-- Selection active -->
        <ng-container *ngIf="selectedCount > 0">
          <span class="status-text status-selected">{{ selectedCount }} selected</span>
          <span class="status-sep">·</span>
          <span class="status-text">{{ formatSize(selectionSize) }}</span>
          <span class="status-sep status-sep-pipe"></span>
          <span class="status-dim">{{ fileCount + dirCount }} total</span>
        </ng-container>

        <!-- No selection -->
        <ng-container *ngIf="selectedCount === 0">
          <ng-container *ngIf="fileCount > 0 || dirCount > 0; else emptyDir">
            <span class="status-text" *ngIf="fileCount > 0">
              {{ fileCount }} file{{ fileCount !== 1 ? 's' : '' }}
            </span>
            <span class="status-sep" *ngIf="fileCount > 0 && dirCount > 0">·</span>
            <span class="status-text" *ngIf="dirCount > 0">
              {{ dirCount }} folder{{ dirCount !== 1 ? 's' : '' }}
            </span>
            <span class="status-sep status-sep-pipe"></span>
            <span class="status-dim">{{ formatSize(dirTotalSize) }}</span>
          </ng-container>
          <ng-template #emptyDir>
            <span class="status-dim">Empty folder</span>
          </ng-template>
        </ng-container>

      </div>

      <div class="status-center"></div>

      <div class="status-right">
        <ng-content></ng-content>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      height: var(--vsc-statusbar-height, 22px);
      background: var(--vsc-statusbar-background, #007acc);
      border-top: 1px solid rgba(255,255,255,0.1);
    }

    .status-bar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 100%;
      padding: 0 8px;
      font-size: 11px;
      color: rgba(255,255,255,0.85);
      font-family: var(--vsc-font-family, -apple-system, BlinkMacSystemFont, sans-serif);
    }

    .status-left, .status-right {
      display: flex;
      align-items: center;
      gap: 4px;
      min-width: 0;
    }

    .status-center {
      position: absolute;
      left: 50%;
      transform: translateX(-50%);
    }

    .status-text {
      color: rgba(255,255,255,0.9);
      white-space: nowrap;
    }

    .status-selected {
      font-weight: 600;
      color: #fff;
    }

    .status-dim {
      color: rgba(255,255,255,0.55);
      white-space: nowrap;
    }

    .status-sep {
      color: rgba(255,255,255,0.35);
      margin: 0 1px;
    }

    .status-sep-pipe {
      display: inline-block;
      width: 1px;
      height: 10px;
      background: rgba(255,255,255,0.25);
      margin: 0 4px;
      vertical-align: middle;
    }
  `]
})
export class StatusBarComponent {
  @Input() filteredEntries: FileInfo[] = [];
  @Input() selectedPaths: Set<string> = new Set();
  @Input() allEntries: FileInfo[] = [];

  get selectedCount(): number { return this.selectedPaths.size; }

  get fileCount(): number {
    return this.filteredEntries.filter(e => e.type === FileType.FILE || e.type === FileType.SYMLINK).length;
  }

  get dirCount(): number {
    return this.filteredEntries.filter(e => e.type === FileType.DIRECTORY).length;
  }

  /** Sum of sizes of selected entries (files only — dirs typically report 0) */
  get selectionSize(): number {
    if (this.selectedCount === 0) return 0;
    return this.allEntries
      .filter(e => this.selectedPaths.has(e.path))
      .reduce((sum, e) => sum + (e.size ?? 0), 0);
  }

  /** Sum of all file sizes in the filtered listing */
  get dirTotalSize(): number {
    return this.filteredEntries
      .filter(e => e.type === FileType.FILE || e.type === FileType.SYMLINK)
      .reduce((sum, e) => sum + (e.size ?? 0), 0);
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }
}