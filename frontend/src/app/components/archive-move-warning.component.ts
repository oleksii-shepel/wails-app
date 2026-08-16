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

import { Component, ElementRef, EventEmitter, Input, OnChanges, OnDestroy, OnInit, Output, SimpleChanges, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';
import { moveFocusWithin } from '../directives/focus-trap.utils';

@Component({
  selector: 'app-archive-move-warning',
  standalone: true,
  imports: [CommonModule, DialogFocusTrapDirective],
  template: `
<div *ngIf="visible" class="amw-backdrop" (click)="onBackdrop($event)">
  <div #dialogPanel class="amw-panel" [appTrapFocus]="true" role="alertdialog" aria-modal="true" tabindex="-1">

    <div class="amw-content">
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
    </div>

    <div class="amw-actions">
      <button class="amw-btn amw-btn--cancel" data-dialog-initial-focus data-dialog-action="cancel" (click)="cancelled.emit()">Cancel</button>
      <button class="amw-btn amw-btn--confirm" data-dialog-action="confirm" (click)="confirmed.emit()">Move</button>
    </div>

  </div>
</div>
  `,
  styles: [`
    .amw-backdrop {
      position: fixed; inset: 0; z-index: 1300;
      background: var(--vsc-scrim);
      backdrop-filter: blur(2px);
      display: flex; align-items: center; justify-content: center;
    }

    .amw-panel {
      width: min(360px, calc(100vw - 40px));
      padding: 0;
      background: var(--vsc-editor-background);
      border: 1px solid var(--vsc-border-bright);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow), 0 20px 48px var(--vsc-scrim-strong);
      display: flex; flex-direction: column; align-items: center;
      color: var(--vsc-foreground, var(--vsc-foreground));
      font-size: 13px;
      overflow: hidden;
      animation: slideDown .28s cubic-bezier(.16,1,.3,1);
    }
    @keyframes slideDown {
      from { opacity: 0; transform: translateY(-16px) }
      to   { opacity: 1; transform: none }
    }

    .amw-icon svg {
      width: 34px; height: 34px;
      color: var(--vsc-warning, var(--vsc-accent-yellow));
    }

    .amw-title {
      margin: 0;
      font-size: 14px;
      font-weight: 600;
      color: var(--vsc-foreground-bright, var(--vsc-foreground));
    }

    .amw-body, .amw-note {
      margin: 0;
      text-align: center;
      line-height: 1.55;
      padding: 0 20px;
    }
    .amw-note {
      font-size: 11px;
      opacity: .45;
      padding-bottom: 4px;
    }

    .amw-content {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 12px;
      padding: 24px 20px 4px;
      width: 100%;
    }

    .amw-actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      width: 100%;
      padding: 12px 16px;
      border-top: 1px solid var(--vsc-border-subtle);
      background: color-mix(in srgb, var(--vsc-panel-background) 85%, var(--vsc-sidebar-background) 15%);
    }

    .amw-btn {
      padding: 6px 22px;
      border-radius: 4px;
      font-size: 12px; font-weight: 500;
      cursor: pointer;
      border: 1px solid var(--vsc-border, var(--vsc-border-bright));
      transition: background .12s;
    }
    .amw-btn--cancel {
      background: color-mix(in srgb, var(--vsc-foreground) 6%, transparent);
      color: var(--vsc-foreground, var(--vsc-foreground));
    }
    .amw-btn--cancel:hover { background: color-mix(in srgb, var(--vsc-foreground) 10%, transparent); }

    .amw-btn--confirm {
      background: var(--vsc-accent-orange, var(--vsc-warning));
      color: var(--vsc-media-background); font-weight: 600;
      border-color: transparent;
    }
    .amw-btn--confirm:hover { filter: brightness(1.08); }
  `],
})
export class ArchiveMoveWarningComponent implements OnInit, OnChanges, OnDestroy {
  @ViewChild('dialogPanel') private dialogPanel?: ElementRef<HTMLElement>;

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

    this.inputEngine.registerCommand('dialog.archiveMoveWarning.focusNext', () => {
      if (!this.visible) return;
      this.moveDialogFocus(1);
    });

    this.inputEngine.registerCommand('dialog.archiveMoveWarning.focusPrev', () => {
      if (!this.visible) return;
      this.moveDialogFocus(-1);
    });

    this.inputEngine.registerCommand('dialog.archiveMoveWarning.focusNextArrow', () => {
      if (!this.visible || !this.shouldUseActionArrowNavigation()) return;
      this.moveActionFocus(1);
    });

    this.inputEngine.registerCommand('dialog.archiveMoveWarning.focusPrevArrow', () => {
      if (!this.visible || !this.shouldUseActionArrowNavigation()) return;
      this.moveActionFocus(-1);
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
    this.inputEngine.unregisterCommand('dialog.archiveMoveWarning.focusNext');
    this.inputEngine.unregisterCommand('dialog.archiveMoveWarning.focusPrev');
    this.inputEngine.unregisterCommand('dialog.archiveMoveWarning.focusNextArrow');
    this.inputEngine.unregisterCommand('dialog.archiveMoveWarning.focusPrevArrow');
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

  private moveDialogFocus(delta: number): void {
    const panel = this.dialogPanel?.nativeElement;
    if (!panel) {
      return;
    }

    moveFocusWithin(panel, delta);
  }

  private moveActionFocus(delta: number): void {
    const panel = this.dialogPanel?.nativeElement;
    if (!panel) {
      return;
    }

    moveFocusWithin(panel, delta, '[data-dialog-action]');
  }

  private shouldUseActionArrowNavigation(): boolean {
    const panel = this.dialogPanel?.nativeElement;
    const activeElement = document.activeElement;
    if (!panel || !(activeElement instanceof HTMLElement) || !panel.contains(activeElement)) {
      return false;
    }

    return !!activeElement.closest('[data-dialog-action]');
  }
}