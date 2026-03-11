/**
 * archive-move-warning.component.ts
 *
 * One-shot confirmation shown before ArchiveMoveEntries runs, so the user
 * understands what will happen:
 *   - Entries ARE extracted to the destination (tracked in operation toast)
 *   - Source entries ARE deleted from the archive (archive is rewritten)
 *   - For nested archives: extraction runs, deletion is skipped
 *
 * Wired in FilePaneComponent template:
 *
 *   <app-archive-move-warning
 *     [visible]="archiveMoveWarning"
 *     [entryCount]="archivePendingMoveEntries.length"
 *     (confirmed)="executePendingMove()"
 *     (cancelled)="archiveMoveWarning = false">
 *   </app-archive-move-warning>
 */

import { Component, Input, Output, EventEmitter, OnChanges, OnDestroy, OnInit, SimpleChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';

@Component({
  selector: 'app-archive-move-warning',
  standalone: true,
  imports: [CommonModule],
  template: `
<div *ngIf="visible" class="amw-backdrop" (click)="onBackdrop($event)">
  <div class="amw-panel" role="alertdialog" aria-modal="true">

    <div class="amw-icon" aria-hidden="true">
      <svg viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd"
          d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17
             2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485
             2.495zM10 5a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5
             0v-3.5A.75.75 0 0 1 10 5zm0 9a1 1 0 1 0 0-2 1 1 0 0 0 0 2z"
          clip-rule="evenodd"/>
      </svg>
    </div>

    <p class="amw-title">Move from archive</p>

    <p class="amw-body">
      <strong>{{ entryCount }} {{ entryCount === 1 ? 'entry' : 'entries' }}</strong>
      will be extracted to the destination and then
      <strong>removed from the archive</strong> by rewriting it.
    </p>
    <p class="amw-note">
      Progress and any conflicts will appear in the operation panel.
      For nested archives only the extraction step runs — source stays intact.
    </p>

    <div class="amw-actions">
      <button class="amw-btn amw-btn--cancel"  (click)="cancelled.emit()">Cancel</button>
      <button class="amw-btn amw-btn--confirm" (click)="confirmed.emit()">Move</button>
    </div>

  </div>
</div>
  `,
  styles: [`
    .amw-backdrop {
      position: fixed; inset: 0; z-index: 1300;
      background: rgba(0,0,0,.55);
      backdrop-filter: blur(2px);
      display: flex; align-items: center; justify-content: center;
    }

    .amw-panel {
      width: min(360px, calc(100vw - 40px));
      padding: 24px 20px 18px;
      background: var(--vsc-panel-background, #252526);
      border: 1px solid color-mix(in srgb,
        var(--vsc-warning, #d7ba7d) 35%,
        var(--vsc-border, #3c3c3c));
      border-radius: 8px;
      box-shadow: 0 18px 52px rgba(0,0,0,.6);
      display: flex; flex-direction: column; align-items: center; gap: 12px;
      color: var(--vsc-foreground, #ccc);
      font-size: 13px;
      animation: amw-in .15s cubic-bezier(.16,1,.3,1);
    }
    @keyframes amw-in {
      from { opacity: 0; transform: translateY(10px) scale(.97) }
      to   { opacity: 1; transform: none }
    }

    .amw-icon svg {
      width: 34px; height: 34px;
      color: var(--vsc-warning, #d7ba7d);
    }

    .amw-title {
      margin: 0;
      font-size: 14px;
      font-weight: 600;
      color: var(--vsc-foreground-bright, #e0e0e0);
    }

    .amw-body, .amw-note {
      margin: 0;
      text-align: center;
      line-height: 1.55;
    }
    .amw-note {
      font-size: 11px;
      opacity: .45;
    }

    .amw-actions {
      display: flex; gap: 10px; padding-top: 4px;
    }

    .amw-btn {
      padding: 6px 22px;
      border-radius: 4px;
      font-size: 12px; font-weight: 500;
      cursor: pointer;
      border: 1px solid var(--vsc-border, #555);
      transition: background .12s;
    }
    .amw-btn--cancel {
      background: rgba(255,255,255,.06);
      color: var(--vsc-foreground, #ccc);
    }
    .amw-btn--cancel:hover { background: rgba(255,255,255,.1); }

    .amw-btn--confirm {
      background: var(--vsc-accent-orange, #e8a94d);
      color: #1a1a1a; font-weight: 600;
      border-color: transparent;
    }
    .amw-btn--confirm:hover { filter: brightness(1.08); }
  `],
})
export class ArchiveMoveWarningComponent implements OnInit, OnChanges, OnDestroy {
  private releaseDialogContext: (() => void) | null = null;

  @Input()  visible    = false;
  @Input()  entryCount = 1;
  @Output() confirmed  = new EventEmitter<void>();
  @Output() cancelled  = new EventEmitter<void>();

  constructor(
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
  ) {}

  ngOnInit(): void {
    this.inputEngine.registerCommand('dialog.archiveMoveWarning.confirm', () => {
      if (!this.visible) return;
      this.confirmed.emit();
    });

    this.inputEngine.registerCommand('dialog.archiveMoveWarning.cancel', () => {
      if (!this.visible) return;
      this.cancelled.emit();
    });

    this.syncContextLease();
  }

  ngOnChanges(_changes: SimpleChanges): void {
    this.syncContextLease();
  }

  ngOnDestroy(): void {
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.inputEngine.unregisterCommand('dialog.archiveMoveWarning.confirm');
    this.inputEngine.unregisterCommand('dialog.archiveMoveWarning.cancel');
  }

  onBackdrop(e: MouseEvent): void {
    if ((e.target as HTMLElement).classList.contains('amw-backdrop')) {
      this.cancelled.emit();
    }
  }

  private syncContextLease(): void {
    if (this.visible) {
      this.releaseDialogContext ??= this.inputContexts.acquire('dialog.archiveMoveWarning');
      return;
    }

    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
  }
}