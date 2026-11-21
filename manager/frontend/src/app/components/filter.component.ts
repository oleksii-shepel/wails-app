import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnDestroy,
  OnInit,
  Output,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';

const COMMAND = {
  close: 'close',
  next: 'next',
  previous: 'previous',
} as const;

function bindings(
  paneId: string,
  contextId: string,
): readonly InputBindingDefinition[] {
  const base = `reassembly.filter.${paneId}`;
  return [
    {
      id: `${base}.close.escape`,
      commandId: `${base}.${COMMAND.close}`,
      contextId,
      source: 'default',
      description: 'Close filter',
      category: 'Search & Filter',
      when: 'typing',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'escape' }] } },
    },
    {
      id: `${base}.next.enter`,
      commandId: `${base}.${COMMAND.next}`,
      contextId,
      source: 'default',
      description: 'Next filter match',
      category: 'Search & Filter',
      when: 'typing',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'enter' }] } },
    },
    {
      id: `${base}.previous.shiftEnter`,
      commandId: `${base}.${COMMAND.previous}`,
      contextId,
      source: 'default',
      description: 'Previous filter match',
      category: 'Search & Filter',
      when: 'typing',
      trigger: {
        device: 'keyboard',
        chord: { steps: [{ key: 'enter', modifiers: ['shift'] }] },
      },
    },
  ];
}

@Component({
  selector: 'app-filter',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="filter" *ngIf="visible">
      <span class="glyph" aria-hidden="true">⌕</span>

      <input
        #input
        [(ngModel)]="value"
        (ngModelChange)="valueChange.emit($event)"
        (focus)="activateContext()"
        placeholder="Filter current folder"
        autocomplete="off"
        spellcheck="false"
        aria-label="Filter current folder" />

      <span class="count">
        {{ matchCount }}<ng-container *ngIf="totalCount >= 0"> / {{ totalCount }}</ng-container>
      </span>

      <button type="button" aria-label="Previous filter match" title="Previous match (Shift+Enter)" (click)="previous.emit()">↑</button>
      <button type="button" aria-label="Next filter match" title="Next match (Enter)" (click)="next.emit()">↓</button>
      <button type="button" aria-label="Close the file filter" title="Close (Escape)" (click)="close.emit()">×</button>
    </div>
  `,
  styles: [`
    :host {
      display: block;
    }

    .filter {
      display: grid;
      grid-template-columns: 24px minmax(80px, 1fr) auto 26px 26px 26px;
      align-items: center;
      gap: 3px;
      min-height: 34px;
      padding: 3px 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
    }

    .glyph {
      text-align: center;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    input {
      box-sizing: border-box;
      width: 100%;
      height: 26px;
      padding: 0 8px;
      border: 1px solid var(--vsc-focus-border, var(--vsc-accent-blue));
      border-radius: 4px;
      outline: 0;
      background: var(--vsc-input-background, var(--vsc-popover-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
      font: 11px/1.3 var(--vsc-font-family, system-ui, sans-serif);
    }

    .count {
      min-width: 48px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      text-align: right;
      font-size: 10px;
    }

    button {
      width: 24px;
      height: 24px;
      padding: 0;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font-size: 13px;
    }

    button:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }
  `],
})
export class FilterComponent implements OnInit, OnDestroy {
  @ViewChild('input') input?: ElementRef<HTMLInputElement>;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() visible = false;
  @Input() value = '';
  @Input() matchCount = 0;
  @Input() totalCount = -1;

  @Output() valueChange = new EventEmitter<string>();
  @Output() close = new EventEmitter<void>();
  @Output() next = new EventEmitter<void>();
  @Output() previous = new EventEmitter<void>();

  private registeredBindings: readonly InputBindingDefinition[] = [];

  constructor(
    private readonly inputEngine: InputEngineService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  private get contextId(): string {
    return `reassembly.filter.${this.paneId}`;
  }

  private commandId(name: string): string {
    return `reassembly.filter.${this.paneId}.${name}`;
  }

  ngOnInit(): void {
    this.inputEngine.registerContext({
      id: this.contextId,
      kind: 'overlay',
      priority: 240,
      parentId: 'reassembly.manager',
      exclusive: true,
      description: `Inline folder filter (${this.paneId})`,
    });

    this.inputEngine.registerCommandDescriptor({
      id: this.commandId(COMMAND.close),
      title: 'Close Filter',
      category: 'Search & Filter',
      execute: () => this.close.emit(),
    }, this.contextId);

    this.inputEngine.registerCommandDescriptor({
      id: this.commandId(COMMAND.next),
      title: 'Next Filter Match',
      category: 'Search & Filter',
      execute: () => this.next.emit(),
    }, this.contextId);

    this.inputEngine.registerCommandDescriptor({
      id: this.commandId(COMMAND.previous),
      title: 'Previous Filter Match',
      category: 'Search & Filter',
      execute: () => this.previous.emit(),
    }, this.contextId);

    this.registeredBindings = bindings(this.paneId, this.contextId);
    for (const binding of this.registeredBindings) {
      this.inputEngine.registerBinding(binding);
    }
  }

  ngOnDestroy(): void {
    this.inputEngine.deactivateContext(this.contextId);

    for (const binding of this.registeredBindings) {
      this.inputEngine.unregisterBinding(binding.id);
    }

    for (const name of Object.values(COMMAND)) {
      this.inputEngine.unregisterCommandDescriptor(
        this.commandId(name),
        this.contextId,
      );
    }
  }

  focus(select = false): void {
    if (!this.visible) return;

    queueMicrotask(() => {
      const input = this.input?.nativeElement;
      if (!input) return;
      input.focus();
      if (select) input.select();
      this.activateContext();
    });
  }

  activateContext(): void {
    if (!this.visible) return;
    this.inputEngine.activateContext(this.contextId);
    this.cdr.markForCheck();
  }

  deactivateContext(): void {
    this.inputEngine.deactivateContext(this.contextId);
  }
}
