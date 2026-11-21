import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  HostListener,
  Input,
  OnChanges,
  OnDestroy,
  Output,
  SimpleChanges,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputCustomizationService } from '../services/input-customization.service';
import { InputEngineService } from '../services/input-engine.service';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';
import type {
  InputBindingConflict,
  InputBindingDefinition,
  InputTrigger,
  KeyStroke,
  MouseGesture,
} from '../services/input-system.types';

@Component({
  selector: 'app-input-center',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop" *ngIf="visible" (mousedown)="requestClose()">
      <section
        #panel
        class="panel palette-dialog-surface dialog-surface"
        role="dialog"
        aria-modal="true"
        aria-label="Keyboard and Mouse Bindings"
        (mousedown)="$event.stopPropagation()" (keydown.escape)="onPanelEscape($event)">
        <header>
          <div class="title-block">
            <h2>Input & Shortcuts</h2>
            <p>Rebind keyboard, chords, mouse gestures, and plugin commands live.</p>
          </div>
          <button type="button" class="dialog-close" title="Close" aria-label="Close dialog" (click)="requestClose()"> × </button>
        </header>

        <div class="toolbar">
          <input
            class="search"
            type="search"
            [(ngModel)]="searchText"
            (ngModelChange)="applyFilters()"
            placeholder="Search command, shortcut, context, plugin…"
            aria-label="Search input bindings">
          <select [(ngModel)]="deviceFilter" (ngModelChange)="applyFilters()" aria-label="Input device filter">
            <option value="all">All devices</option>
            <option value="keyboard">Keyboard</option>
            <option value="mouse">Mouse</option>
          </select>
          <label class="check">
            <input type="checkbox" [(ngModel)]="conflictsOnly" (ngModelChange)="applyFilters()">
            Conflicts only
          </label>
          <span class="summary">
            {{ visibleBindings.length }} bindings · {{ conflicts.length }} conflicts
          </span>
        </div>

        <div class="capture-banner" *ngIf="editingBinding">
          <strong>Recording: {{ editingBinding.description }}</strong>
          <span>{{ captureHint }}</span>
          <span class="captured" *ngIf="capturedSteps.length">
            {{ capturedLabel }}
          </span>
          <button type="button" (click)="acceptKeyboardCapture()"
            [disabled]="capturedSteps.length === 0">Apply</button>
          <button type="button" (click)="cancelCapture()">Cancel</button>
        </div>

        <div class="content" [class.empty-content]="visibleBindings.length === 0">
          <div class="empty-state" *ngIf="visibleBindings.length === 0">
            <ng-container *ngIf="conflictsOnly && conflicts.length === 0; else noBindingMatches">
              <strong>No shortcut conflicts</strong>
              <span>All currently registered bindings resolve without an actionable conflict.</span>
            </ng-container>
            <ng-template #noBindingMatches>
              <strong>No matching bindings</strong>
              <span>Try changing the search text, device filter, or conflict filter.</span>
            </ng-template>
          </div>

          <div class="binding-list" role="list" *ngIf="visibleBindings.length > 0">
            <article
              *ngFor="let binding of visibleBindings; trackBy: trackBinding"
              class="binding"
              [class.disabled]="binding.enabled === false"
              [class.overridden]="isOverridden(binding.id)"
              [class.conflict]="conflictsFor(binding.id).length > 0"
              role="listitem">
              <div class="main">
                <strong>{{ binding.description }}</strong>
                <small>
                  {{ binding.category || 'General' }} · {{ binding.contextId }}
                  <span *ngIf="binding.source === 'plugin'"> · Plugin</span>
                  <span *ngIf="binding.source === 'user'"> · User override</span>
                  <span *ngIf="isOverridden(binding.id)"> · Customized</span>
                </small>
              </div>

              <button
                type="button"
                class="trigger"
                [title]="'Change ' + binding.description"
                (click)="beginCapture(binding)">
                {{ triggerLabel(binding.id) }}
              </button>

              <label class="enabled" title="Enable or disable binding">
                <input
                  type="checkbox"
                  [checked]="binding.enabled !== false"
                  (change)="toggleEnabled(binding, $event)">
              </label>

              <button
                type="button"
                class="reset"
                [disabled]="!isOverridden(binding.id)"
                title="Reset this binding"
                (click)="reset(binding)">
                Reset
              </button>

              <div class="conflict-copy" *ngIf="conflictsFor(binding.id) as bindingConflicts">
                <span *ngFor="let conflict of bindingConflicts">
                  Conflicts with {{ peerDescription(binding.id, conflict) }}
                  — winner: {{ conflict.winnerBindingId }}
                </span>
              </div>
            </article>
          </div>
        </div>

        <footer>
          <div>
            <button type="button" (click)="exportConfig()">Export</button>
            <button type="button" (click)="fileInput.click()">Import</button>
            <input #fileInput hidden type="file" accept=".json,application/json"
              (change)="importConfig($event)">
          </div>
          <div>
            <button type="button" (click)="resetAll()">Reset All</button>
            <button type="button" class="primary" (click)="requestClose()">Close</button>
          </div>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:8600;pointer-events:none}
    .backdrop{position:absolute;inset:0;box-sizing:border-box;display:grid;place-items:start center;padding-top:min(8vh,72px);background:var(--vsc-scrim-soft);pointer-events:auto;overflow:hidden;animation:dialog-backdrop-enter var(--dialog-enter-duration) var(--dialog-enter-ease)}
    .panel{box-sizing:border-box;display:flex;flex-direction:column;width:min(980px,calc(100vw - 40px));height:min(760px,calc(100dvh - 40px));max-height:calc(100dvh - 40px);min-height:0;overflow:hidden;border:1px solid var(--vsc-border);border-radius:8px;background:var(--vsc-menu-background,var(--vsc-panel-background));color:var(--vsc-dialog-foreground,var(--vsc-foreground));box-shadow:0 18px 60px var(--vsc-scrim);font:11px/1.35 var(--vsc-font-family,system-ui,sans-serif);transform-origin:center top;animation:dialog-surface-enter var(--dialog-enter-duration) var(--dialog-enter-ease)}
    header,footer,.toolbar,.capture-banner{display:flex;align-items:center;gap:8px;padding:8px 10px;border-color:var(--vsc-border-subtle,var(--vsc-border))}
    header{justify-content:space-between;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border))}
    .title-block{min-width:0}
    header h2{margin:0;font-size:14px} header p{margin:2px 0 0;color:var(--vsc-description-foreground)}
    button,select,input{font:inherit}
    button,select{min-height:24px;border:1px solid var(--vsc-border);border-radius:3px;background:var(--vsc-button-secondary-background,var(--vsc-control-background));color:var(--vsc-foreground)}
    button{padding:2px 8px;cursor:pointer} button:disabled{opacity:.4;cursor:default}
    button:hover:not(:disabled){background:var(--vsc-list-hover-background)}
    button:focus-visible,input:focus-visible,select:focus-visible{outline:1px solid var(--vsc-focus-border);outline-offset:1px}
    .dialog-close{margin-left:8px}
    .toolbar{border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border));flex-wrap:wrap}
    .search{flex:1;min-width:240px;height:26px;padding:3px 7px;border:1px solid var(--vsc-input-border,var(--vsc-border));border-radius:3px;background:var(--vsc-input-background);color:var(--vsc-input-foreground)}
    .check{display:flex;align-items:center;gap:4px}.summary{margin-left:auto;color:var(--vsc-description-foreground);font-variant-numeric:tabular-nums}
    .capture-banner{border-bottom:1px solid var(--vsc-border);background:color-mix(in srgb,var(--vsc-accent-blue) 8%,var(--vsc-panel-background));flex-wrap:wrap}
    .capture-banner strong{color:var(--vsc-foreground)}.capture-banner span{color:var(--vsc-description-foreground)}.capture-banner .captured{padding:2px 6px;border:1px solid var(--vsc-focus-border);border-radius:3px;color:var(--vsc-foreground)}
    header,.toolbar,.capture-banner,footer{flex:0 0 auto}
    .content{flex:1 1 auto;min-height:0;overflow:auto;overscroll-behavior:contain}
    .content.empty-content{display:grid;place-items:center;overflow:hidden}
    .empty-state{display:grid;justify-items:center;gap:6px;max-width:460px;padding:28px;text-align:center;color:var(--vsc-description-foreground)}
    .empty-state strong{color:var(--vsc-foreground);font-size:14px;font-weight:600}
    .empty-state span{max-width:420px;line-height:1.45}
    footer{position:relative;z-index:1;justify-content:space-between;border-top:1px solid var(--vsc-border-subtle,var(--vsc-border));background:var(--vsc-dialog-background,var(--vsc-panel-background))}
    .binding-list{display:grid}
    .binding{display:grid;grid-template-columns:minmax(220px,1fr) minmax(150px,260px) 32px 56px;align-items:center;gap:7px;min-height:44px;padding:5px 9px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border))}
    .binding:hover{background:var(--vsc-list-hover-background)}.binding.disabled{opacity:.58}.binding.overridden{background:color-mix(in srgb,var(--vsc-accent-blue) 6%,transparent)}.binding.conflict{background:color-mix(in srgb,var(--vsc-warning) 6%,transparent)}
    .main{display:grid;min-width:0}.main strong,.main small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.main small{color:var(--vsc-description-foreground)}
    .trigger{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;text-align:left;background:var(--vsc-input-background)}
    .enabled{display:grid;place-items:center}.reset{padding-inline:5px}
    .conflict-copy{grid-column:1/-1;display:flex;flex-wrap:wrap;gap:4px 12px;color:var(--vsc-warning)}
    footer{justify-content:space-between;border-top:1px solid var(--vsc-border-subtle,var(--vsc-border))}footer>div{display:flex;gap:5px}.primary{background:var(--vsc-button-background);color:var(--vsc-button-foreground)}
    @supports not (height: 100dvh){.panel{height:min(760px,calc(100vh - 40px));max-height:calc(100vh - 40px)}}
    @media(max-width:700px){.binding{grid-template-columns:minmax(0,1fr) 120px 28px}.reset{grid-column:1/-1;justify-self:end}.summary{width:100%;margin-left:0}}
  `],
})
export class InputCenterComponent implements OnChanges, OnDestroy {
  @Input() visible = false;
  @Output() closed = new EventEmitter<void>();
  @ViewChild('panel') panel?: ElementRef<HTMLElement>;

  searchText = '';
  deviceFilter: 'all' | 'keyboard' | 'mouse' = 'all';
  conflictsOnly = false;
  editingBinding: InputBindingDefinition | null = null;
  capturedSteps: KeyStroke[] = [];

  visibleBindings: readonly InputBindingDefinition[] = [];
  conflicts: readonly InputBindingConflict[] = [];

  private bindingsSnapshot: readonly InputBindingDefinition[] = [];
  private readonly conflictsByBindingId = new Map<string, InputBindingConflict[]>();
  private readonly triggerLabels = new Map<string, string>();
  private readonly overriddenIds = new Set<string>();
  private modelDirty = true;
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;
  private readonly bindingSubscription: { unsubscribe?: () => void } | (() => void) | void;

  constructor(
    readonly customization: InputCustomizationService,
    private readonly engine: InputEngineService,
    private readonly dialogs: ConfirmDialogService,
    private readonly cdr: ChangeDetectorRef,
  ) {
    // Binding registration is noisy during startup. Keep the editor cheap while
    // hidden and rebuild its presentation model only when it is actually open.
    this.bindingSubscription = this.engine.bindings$.subscribe(() => {
      this.modelDirty = true;
      if (!this.visible) return;
      this.refreshPresentationModel();
      this.cdr.markForCheck();
    });
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['visible']) {
      if (this.visible) {
        if (this.modelDirty) {
          this.refreshPresentationModel();
        }
        if (this.modalStack && this.hostElement && !this.unregisterModal) {
          this.unregisterModal = this.modalStack.register({
            id: 'input-center',
            element: this.hostElement.nativeElement,
            dismiss: () => this.onPanelEscape(),
            dismissible: () => this.visible,
          });
        }
      } else {
        this.unregisterModal?.();
        this.unregisterModal = null;
      }
    }
  }

  ngOnDestroy(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
    if (typeof this.bindingSubscription === 'function') {
      this.bindingSubscription();
    } else {
      this.bindingSubscription?.unsubscribe?.();
    }
  }

  applyFilters(): void {
    const search = this.searchText.trim().toLocaleLowerCase();
    this.visibleBindings = this.bindingsSnapshot.filter(binding => {
      if (this.deviceFilter !== 'all' && binding.trigger.device !== this.deviceFilter) {
        return false;
      }
      if (this.conflictsOnly && (this.conflictsByBindingId.get(binding.id)?.length ?? 0) === 0) {
        return false;
      }
      if (!search) return true;

      const trigger = this.triggerLabels.get(binding.id) ?? '';
      return [
        binding.id,
        binding.commandId,
        binding.description,
        binding.category ?? '',
        binding.contextId,
        binding.source,
        trigger,
      ].some(value => value.toLocaleLowerCase().includes(search));
    });
  }

  get captureHint(): string {
    return this.editingBinding?.trigger.device === 'mouse'
      ? 'Click, double-click, wheel, or drag inside this dialog to capture a mouse gesture. Esc cancels.'
      : 'Press a shortcut. For a chord, press the first combination and then the second. Esc cancels.';
  }

  get capturedLabel(): string {
    return this.engine.formatInputTrigger({
      device: 'keyboard',
      chord: { steps: this.capturedSteps },
    });
  }

  onPanelEscape(event?: Event): void {
    event?.preventDefault();
    event?.stopPropagation();
    if (this.editingBinding) {
      this.cancelCapture();
      return;
    }
    this.requestClose();
  }

  requestClose(): void {
    this.cancelCapture();
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.closed.emit();
  }

  beginCapture(binding: InputBindingDefinition): void {
    this.editingBinding = binding;
    this.capturedSteps = [];

    // Recording must be inert: the old shortcut must not execute while the
    // user is trying to replace it, and mouse gestures must not dispatch
    // commands underneath the editor.
    this.engine.setKeyboardEnabled(false);
    this.cdr.markForCheck();
  }

  cancelCapture(): void {
    const wasCapturing = !!this.editingBinding;
    this.editingBinding = null;
    this.capturedSteps = [];
    if (wasCapturing) this.engine.setKeyboardEnabled(true);
    this.cdr.markForCheck();
  }

  @HostListener('document:keydown', ['$event'])
  captureKeyboard(event: KeyboardEvent): void {
    const binding = this.editingBinding;
    if (!this.visible) return;
    if (this.modalStack && !this.modalStack.isTopmost('input-center')) return;
    if (!binding) {
      if (event.key === 'Escape') {
        if (this.modalStack) return;
        event.preventDefault();
        event.stopPropagation();
        this.requestClose();
      }
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      event.stopImmediatePropagation?.();
      this.cancelCapture();
      return;
    }
    if (binding.trigger.device !== 'keyboard') return;

    event.preventDefault();
    event.stopPropagation();

    const step = this.keyStroke(event);
    if (!step.key || ['control', 'shift', 'alt', 'meta'].includes(step.key)) return;

    this.capturedSteps = [...this.capturedSteps, step].slice(0, 2);
    this.cdr.markForCheck();

    if (this.capturedSteps.length >= 2) {
      this.acceptKeyboardCapture();
    }
  }

  @HostListener('document:click', ['$event'])
  captureClick(event: MouseEvent): void {
    this.captureMouse('click', event);
  }

  @HostListener('document:dblclick', ['$event'])
  captureDoubleClick(event: MouseEvent): void {
    this.captureMouse('doubleClick', event);
  }

  @HostListener('document:wheel', ['$event'])
  captureWheel(event: WheelEvent): void {
    if (!this.editingBinding || this.editingBinding.trigger.device !== 'mouse') return;
    const direction: MouseGesture['direction'] =
      Math.abs(event.deltaX) > Math.abs(event.deltaY)
        ? (event.deltaX < 0 ? 'left' : 'right')
        : (event.deltaY < 0 ? 'up' : 'down');
    this.applyMouseGesture({
      type: 'wheel',
      modifiers: this.modifiers(event),
      direction,
    }, event);
  }

  acceptKeyboardCapture(): void {
    const binding = this.editingBinding;
    if (!binding || !this.capturedSteps.length) return;

    this.customization.setTrigger(binding.id, {
      device: 'keyboard',
      chord: { steps: this.capturedSteps },
    });
    this.refreshPresentationModel();
    this.cancelCapture();
  }

  toggleEnabled(binding: InputBindingDefinition, event: Event): void {
    const input = event.target as HTMLInputElement;
    this.customization.setEnabled(binding.id, input.checked);
    this.refreshPresentationModel();
    this.cdr.markForCheck();
  }

  reset(binding: InputBindingDefinition): void {
    this.customization.reset(binding.id);
    this.refreshPresentationModel();
    this.cdr.markForCheck();
  }

  async resetAll(): Promise<void> {
    const confirmed = await this.dialogs.confirm({
      title: 'Reset Input Bindings',
      message: 'Reset all user keyboard and mouse customizations?',
      confirmText: 'Reset',
      danger: true,
    });
    if (!confirmed) return;

    this.customization.resetAll();
    this.refreshPresentationModel();
    this.cdr.markForCheck();
  }

  exportConfig(): void {
    const blob = new Blob(
      [this.customization.exportConfig()],
      { type: 'application/json' },
    );
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = 'input-bindings.json';
    anchor.click();
    URL.revokeObjectURL(url);
  }

  importConfig(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;

    const reader = new FileReader();
    reader.onload = async () => {
      try {
        const result = this.customization.importConfig(String(reader.result ?? ''));
        await this.dialogs.alert({
          title: 'Input Bindings Imported',
          message: `${result.imported} imported${result.ignored ? ` · ${result.ignored} ignored` : ''}.`,
        });
        this.refreshPresentationModel();
        this.cdr.markForCheck();
      } catch (error) {
        await this.dialogs.alert({
          title: 'Import Failed',
          message: error instanceof Error ? error.message : String(error),
        });
      }
    };
    reader.readAsText(file);
  }

  conflictsFor(bindingId: string): readonly InputBindingConflict[] {
    return this.conflictsByBindingId.get(bindingId) ?? [];
  }

  isOverridden(bindingId: string): boolean {
    return this.overriddenIds.has(bindingId);
  }

  triggerLabel(bindingId: string): string {
    return this.triggerLabels.get(bindingId) || 'Unbound';
  }

  peerDescription(bindingId: string, conflict: InputBindingConflict): string {
    const peerId =
      conflict.bindingId === bindingId
        ? conflict.conflictingBindingId
        : conflict.bindingId;
    return this.engine.getBinding(peerId)?.description ?? peerId;
  }

  format(trigger: InputTrigger): string {
    return this.engine.formatInputTrigger(trigger);
  }

  private refreshPresentationModel(): void {
    const bindings = this.customization.bindings();
    const conflicts = this.customization.conflicts();

    this.bindingsSnapshot = bindings;
    this.conflicts = conflicts;
    this.conflictsByBindingId.clear();
    this.triggerLabels.clear();
    this.overriddenIds.clear();

    for (const binding of bindings) {
      this.triggerLabels.set(binding.id, this.engine.formatInputTrigger(binding.trigger));
      if (this.customization.hasOverride(binding.id)) {
        this.overriddenIds.add(binding.id);
      }
    }

    for (const conflict of conflicts) {
      for (const id of [conflict.bindingId, conflict.conflictingBindingId]) {
        const current = this.conflictsByBindingId.get(id);
        if (current) {
          current.push(conflict);
        } else {
          this.conflictsByBindingId.set(id, [conflict]);
        }
      }
    }

    this.modelDirty = false;
    this.applyFilters();
  }

  trackBinding(_index: number, binding: InputBindingDefinition): string {
    return binding.id;
  }

  private captureMouse(
    type: 'click' | 'doubleClick',
    event: MouseEvent,
  ): void {
    if (!this.editingBinding || this.editingBinding.trigger.device !== 'mouse') return;

    // Ignore the click used to press the Change button itself.
    if ((event.target as HTMLElement | null)?.closest('.trigger')) return;

    this.applyMouseGesture({
      type,
      button: event.button === 1 ? 'middle' : event.button === 2 ? 'right' : 'left',
      modifiers: this.modifiers(event),
    }, event);
  }

  private applyMouseGesture(
    gesture: MouseGesture,
    event: Event,
  ): void {
    const binding = this.editingBinding;
    if (!binding) return;

    event.preventDefault();
    event.stopPropagation();
    this.customization.setTrigger(binding.id, {
      device: 'mouse',
      gesture,
    });
    this.refreshPresentationModel();
    this.cancelCapture();
  }

  private keyStroke(event: KeyboardEvent): KeyStroke {
    const keyMap: Record<string, string> = {
      ' ': 'space',
      Escape: 'escape',
      Enter: 'enter',
      Tab: 'tab',
      Backspace: 'backspace',
      Delete: 'delete',
      Home: 'home',
      End: 'end',
      PageUp: 'pageup',
      PageDown: 'pagedown',
      ArrowUp: 'arrowup',
      ArrowDown: 'arrowdown',
      ArrowLeft: 'arrowleft',
      ArrowRight: 'arrowright',
    };

    return {
      key: keyMap[event.key] ?? event.key.toLocaleLowerCase(),
      modifiers: this.modifiers(event),
    };
  }

  private modifiers(
    event: Pick<KeyboardEvent | MouseEvent | WheelEvent,
      'ctrlKey' | 'shiftKey' | 'altKey' | 'metaKey'>,
  ): KeyStroke['modifiers'] {
    const values: NonNullable<KeyStroke['modifiers']> = [];
    if (event.ctrlKey) values.push('ctrl');
    if (event.shiftKey) values.push('shift');
    if (event.altKey) values.push('alt');
    if (event.metaKey) values.push('meta');
    return values.length ? values.sort() : undefined;
  }
}