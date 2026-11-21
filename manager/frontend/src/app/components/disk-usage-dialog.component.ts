import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  HostListener,
  Input,
  OnChanges,
  OnDestroy,
  Output,
  SimpleChanges,
} from '@angular/core';
import type { TabInfo } from '../shared/protocol-enhanced';
import type { SizeManagerSurfaceContext } from '../surfaces/surface-context';
import { SizeManagerSurfaceComponent } from '../surfaces/size-manager/size-manager-surface.component';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';

@Component({
  selector: 'app-disk-usage-dialog',
  standalone: true,
  imports: [CommonModule, SizeManagerSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      *ngIf="renderVisible && ctx"
      class="du-backdrop palette-dialog-backdrop dialog-group"
      [class.dialog-group--closing]="isClosing"
      (animationend)="onAnimationEnd($event)"
      role="presentation"
      (mousedown)="onBackdropMouseDown($event)">
      <section
        class="du-dialog palette-dialog-surface dialog-surface"
        role="dialog"
        aria-modal="true"
        aria-label="Disk Usage"
        (mousedown)="$event.stopPropagation()">
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
      background: var(--vsc-scrim-soft, var(--vsc-scrim));
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
      box-shadow: 0 16px 48px var(--vsc-scrim);
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
export class DiskUsageDialogComponent implements OnChanges, OnDestroy {
  @Input() visible = false;
  @Input() rootPath = '';
  @Output() closed = new EventEmitter<void>();

  ctx: SizeManagerSurfaceContext | null = null;
  renderVisible = false;
  isClosing = false;
  private emitClosedOnFinish = false;
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

  constructor(private readonly cdr: ChangeDetectorRef) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['visible'] || changes['rootPath']) {
      if (this.visible && this.rootPath.trim()) {
        this.ctx = this.createContext(this.rootPath.trim());
        this.renderVisible = true;
        this.isClosing = false;
        this.emitClosedOnFinish = false;
        if (this.modalStack && this.hostElement) {
          this.unregisterModal = this.modalStack.register({
            id: 'disk-usage-dialog',
            element: this.hostElement.nativeElement,
            dismiss: () => this.close(),
            dismissible: () => this.visible && !this.isClosing,
          });
        }
      } else if (!this.visible && this.renderVisible && !this.isClosing) {
        this.unregisterModal?.();
        this.unregisterModal = null;
        this.isClosing = true;
      }
    }
  }

  ngOnDestroy(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
  }

  @HostListener('document:keydown.escape', ['$event'])
  onEscape(event: Event): void {
    if (!this.visible || this.isClosing) return;
    if (this.modalStack && !this.modalStack.isTopmost('disk-usage-dialog')) return;
    event.preventDefault();
    event.stopPropagation();
    this.close();
  }

  onBackdropMouseDown(event: MouseEvent): void {
    if (event.target === event.currentTarget) this.close();
  }

  close(): void {
    if (!this.renderVisible || this.isClosing) return;
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.isClosing = true;
    this.emitClosedOnFinish = true;
    this.cdr.markForCheck();
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) return;
    this.renderVisible = false;
    this.isClosing = false;
    this.ctx = null;
    const emit = this.emitClosedOnFinish;
    this.emitClosedOnFinish = false;
    if (emit) this.closed.emit();
    this.cdr.markForCheck();
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
