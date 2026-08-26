import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  HostListener,
  Input,
  OnChanges,
  Output,
  SimpleChanges,
} from '@angular/core';
import type { TabInfo } from '../shared/protocol-enhanced';
import type { SizeManagerSurfaceContext } from '../surfaces/surface-context';
import { SizeManagerSurfaceComponent } from '../surfaces/size-manager/size-manager-surface.component';

@Component({
  selector: 'app-disk-usage-dialog',
  standalone: true,
  imports: [CommonModule, SizeManagerSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      *ngIf="visible && ctx"
      class="du-backdrop palette-dialog-backdrop dialog-group"
      role="presentation"
      (mousedown)="onBackdropMouseDown($event)">
      <section
        class="du-dialog palette-dialog-surface dialog-surface"
        role="dialog"
        aria-modal="true"
        aria-label="Disk Usage"
        (mousedown)="$event.stopPropagation()">
        <header class="palette-dialog-header du-header">
          <div class="palette-dialog-title"><strong>Disk Usage</strong><span class="palette-dialog-subtitle">{{ rootPath }}</span></div>
          <button type="button" class="dialog-close-btn" aria-label="Close Disk Usage" title="Close (Esc)" (click)="close()">×</button>
        </header>
        <app-size-manager-surface [ctx]="ctx"></app-size-manager-surface>
      </section>
    </div>
  `,
  styleUrls: ['./tools-dialog-remote-theme.scss'],
  styles: [`
    :host { display: contents; }

    .du-backdrop {
      position: fixed;
      inset: 0;
      z-index: 1400;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 28px;
      background: var(--vsc-scrim, rgba(0, 0, 0, .48));
    }

    .du-dialog {
      display: flex;
      flex-direction: column;
      width: min(1180px, calc(100vw - 56px));
      height: min(820px, calc(100vh - 56px));
      min-width: 640px;
      min-height: 420px;
      overflow: hidden;
      color: var(--vsc-foreground);
      background: var(--vsc-editor-background);
      border: 1px solid var(--vsc-border);
      border-radius: 6px;
      box-shadow: 0 16px 48px rgba(0, 0, 0, .38);
    }

    app-size-manager-surface {
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
    }

    @media (max-width: 760px), (max-height: 520px) {
      .du-backdrop { padding: 8px; }
      .du-dialog {
        width: calc(100vw - 16px);
        height: calc(100vh - 16px);
        min-width: 0;
        min-height: 0;
      }
    }
  `],
})
export class DiskUsageDialogComponent implements OnChanges {
  @Input() visible = false;
  @Input() rootPath = '';
  @Output() closed = new EventEmitter<void>();

  ctx: SizeManagerSurfaceContext | null = null;

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['visible'] || changes['rootPath']) {
      if (this.visible && this.rootPath.trim()) {
        this.ctx = this.createContext(this.rootPath.trim());
      } else if (!this.visible) {
        // Destroy the scanner surface when the dialog closes so its OnDestroy
        // cleanup runs immediately and no scan remains attached to the shell.
        this.ctx = null;
      }
    }
  }

  @HostListener('document:keydown.escape', ['$event'])
  onEscape(event: Event): void {
    if (!this.visible) return;
    event.preventDefault();
    event.stopPropagation();
    this.close();
  }

  onBackdropMouseDown(event: MouseEvent): void {
    if (event.target === event.currentTarget) this.close();
  }

  close(): void {
    if (!this.visible) return;
    this.ctx = null;
    this.closed.emit();
  }

  private createContext(path: string): SizeManagerSurfaceContext {
    const tab: TabInfo = {
      id: 'disk-usage-dialog',
      title: 'Disk Usage',
      path,
      kind: 'size-manager',
      surfaceId: 'size-manager',
      isActive: true,
      isPinned: false,
      isClosable: true,
      createdAt: Date.now(),
    };

    return {
      paneId: 'left',
      tab,
      surfaceId: 'size-manager',
      kind: 'size-manager',
      sizeVm: {
        paneId: 'left',
        analysisPath: path,
        // Navigation is internal to the utility while the dialog is open.
        onAnalyse: () => undefined,
        onClose: () => this.close(),
      },
    };
  }
}