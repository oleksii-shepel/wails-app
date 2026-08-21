import {
  AfterViewInit,
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  Input,
  OnChanges,
  OnDestroy,
  QueryList,
  SimpleChanges,
  ViewChild,
  ViewChildren,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { DropdownContainerComponent } from './dropdown-container.component';
import { DropdownTriggerDirective } from '../directives/dropdown.directive';
import type { PresentedCommand } from './presented-command';

@Component({
  selector: 'app-responsive-toolbar',
  standalone: true,
  imports: [
    CommonModule,
    DropdownContainerComponent,
    DropdownTriggerDirective,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      #toolbar
      class="responsive-toolbar"
      role="toolbar"
      [attr.aria-label]="ariaLabel">
      <div class="visible-actions">
        <ng-container *ngFor="let action of visibleActions; trackBy: trackAction">
          <span
            *ngIf="action.separatorBefore"
            class="toolbar-separator"
            aria-hidden="true">
          </span>

          <button
            type="button"
            class="toolbar-action"
            [class.danger]="isDanger(action)"
            [class.active]="action.active"
            [disabled]="!action.enabled"
            [title]="actionTitle(action)"
            (click)="invoke(action)">
            <span
              *ngIf="action.iconText || action.icon"
              class="action-icon"
              aria-hidden="true">
              {{ action.iconText || iconGlyph(action.icon) }}
            </span>
            <span class="action-label">{{ action.label }}</span>
          </button>
        </ng-container>
      </div>

      <app-dropdown-container
        *ngIf="overflowActions.length"
        class="overflow"
        [(isOpen)]="overflowOpen">
        <button
          appDropdownTrigger
          type="button"
          class="overflow-trigger"
          [class.active]="overflowOpen"
          aria-haspopup="menu"
          [attr.aria-expanded]="overflowOpen"
          [title]="overflowLabel"
          (click)="toggleOverflow($event)">
          &#x2026;
        </button>

        <div class="overflow-menu" role="menu" [attr.aria-label]="overflowLabel">
          <ng-container *ngFor="let action of overflowActions; trackBy: trackAction">
            <div
              *ngIf="action.separatorBefore"
              class="overflow-separator"
              role="separator">
            </div>

            <button
              type="button"
              role="menuitem"
              class="overflow-item"
              [class.danger]="isDanger(action)"
              [class.active]="action.active"
              [attr.aria-checked]="action.active ? 'true' : null"
              [disabled]="!action.enabled"
              (click)="invoke(action)">
              <span class="overflow-label">{{ action.label }}</span>
              <span *ngIf="action.shortcut" class="overflow-shortcut">
                {{ action.shortcut }}
              </span>
            </button>
          </ng-container>
        </div>
      </app-dropdown-container>

      <div class="measure-strip" aria-hidden="true">
        <button
          #measureAction
          *ngFor="let action of actions; trackBy: trackAction"
          type="button"
          class="toolbar-action measure-action">
          <span
            *ngIf="action.iconText || action.icon"
            class="action-icon"
            aria-hidden="true">
            {{ action.iconText || iconGlyph(action.icon) }}
          </span>
          <span class="action-label">{{ action.label }}</span>
        </button>
        <button #measureOverflow type="button" class="overflow-trigger">
          &#x2026;
        </button>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      flex: 1 1 auto;
      min-width: 34px;
      max-width: 100%;
      overflow: hidden;
    }

    .responsive-toolbar {
      position: relative;
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 4px;
      width: 100%;
      min-width: 0;
      overflow: hidden;
      white-space: nowrap;
    }

    .visible-actions {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 4px;
      min-width: 0;
      overflow: hidden;
    }

    .toolbar-action,
    .overflow-trigger {
      flex: 0 0 auto;
      min-height: 26px;
      padding: 3px 9px;
      border: 1px solid transparent;
      border-radius: 4px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font: inherit;
      font-size: 11px;
      white-space: nowrap;
    }

    .toolbar-action:hover:not(:disabled),
    .toolbar-action.active,
    .overflow-trigger:hover,
    .overflow-trigger.active {
      background: var(--vsc-list-hover-background);
      border-color: var(--vsc-border);
    }

    .toolbar-action:disabled,
    .overflow-item:disabled {
      opacity: .32;
      cursor: default;
    }

    .toolbar-action.active {
      color: var(--vsc-accent-blue);
    }

    .toolbar-action.danger:not(:disabled),
    .overflow-item.danger:not(:disabled) {
      color: var(--vsc-error-foreground, var(--vsc-error));
    }

    .action-icon {
      display: inline-flex;
      width: 14px;
      justify-content: center;
      margin-right: 4px;
    }

    .toolbar-separator {
      flex: 0 0 1px;
      align-self: stretch;
      margin: 4px 2px;
      background: var(--vsc-border-subtle, var(--vsc-border));
    }

    .overflow {
      flex: 0 0 auto;
    }

    .overflow-trigger {
      min-width: 30px;
      padding-inline: 7px;
      font-size: 16px;
      line-height: 16px;
      font-weight: 700;
    }

    .overflow-menu {
      display: flex;
      flex-direction: column;
      min-width: 190px;
      max-width: min(320px, calc(100vw - 16px));
      padding: 4px;
      border: 1px solid var(--vsc-border);
      border-radius: 5px;
      background: var(--vsc-menu-background, var(--vsc-popover-background));
      box-shadow: var(--vsc-shadow-medium);
      color: var(--vsc-menu-foreground, var(--vsc-foreground));
    }

    .overflow-item {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 16px;
      align-items: center;
      width: 100%;
      min-height: 26px;
      padding: 4px 8px;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font: inherit;
      text-align: left;
    }

    .overflow-item:hover:not(:disabled),
    .overflow-item.active {
      background: var(--vsc-list-hover-background);
    }

    .overflow-item.active {
      color: var(--vsc-accent-blue);
    }

    .overflow-label {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .overflow-shortcut {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
      white-space: nowrap;
    }

    .overflow-separator {
      height: 1px;
      margin: 4px 5px;
      background: var(--vsc-border-subtle, var(--vsc-border));
    }

    .measure-strip {
      position: absolute;
      left: -100000px;
      top: -100000px;
      display: flex;
      align-items: center;
      gap: 4px;
      visibility: hidden;
      pointer-events: none;
    }
  `],
})
export class ResponsiveToolbarComponent
  implements AfterViewInit, OnChanges, OnDestroy {
  @Input() actions: readonly PresentedCommand[] = [];
  @Input() ariaLabel = 'Toolbar actions';
  @Input() overflowLabel = 'More actions';
  @Input() minVisible = 0;

  @ViewChild('toolbar', { static: true })
  private toolbarRef!: ElementRef<HTMLElement>;

  @ViewChildren('measureAction', { read: ElementRef })
  private measureActionRefs!: QueryList<ElementRef<HTMLElement>>;

  @ViewChild('measureOverflow', { read: ElementRef })
  private measureOverflowRef!: ElementRef<HTMLElement>;

  visibleActions: readonly PresentedCommand[] = [];
  overflowActions: readonly PresentedCommand[] = [];
  overflowOpen = false;

  private resizeObserver: ResizeObserver | null = null;
  private measurementFrame = 0;
  private lastSignature = '';

  constructor(private readonly cdr: ChangeDetectorRef) {}

  ngAfterViewInit(): void {
    this.resizeObserver = new ResizeObserver(() => this.scheduleMeasurement());
    this.resizeObserver.observe(this.toolbarRef.nativeElement);
    this.measureActionRefs.changes.subscribe(() => this.scheduleMeasurement());
    this.scheduleMeasurement();
  }

  ngOnChanges(_changes: SimpleChanges): void {
    const signature = this.actions
      .map(action => [
        action.id,
        action.label,
        action.shortcut ?? '',
        action.enabled ? '1' : '0',
        action.separatorBefore ? '1' : '0',
      ].join(':'))
      .join('|');

    if (signature === this.lastSignature) return;
    this.lastSignature = signature;
    this.visibleActions = this.actions;
    this.overflowActions = [];
    this.overflowOpen = false;
    this.scheduleMeasurement();
  }

  ngOnDestroy(): void {
    this.resizeObserver?.disconnect();
    this.resizeObserver = null;
    if (this.measurementFrame) {
      cancelAnimationFrame(this.measurementFrame);
    }
  }

  toggleOverflow(event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.overflowOpen = !this.overflowOpen;
  }

  invoke(action: PresentedCommand): void {
    if (!action.enabled) return;
    this.overflowOpen = false;
    void Promise.resolve(action.execute());
  }

  trackAction(_index: number, action: PresentedCommand): string {
    return action.id;
  }

  actionTitle(action: PresentedCommand): string {
    return action.shortcut
      ? `${action.label} (${action.shortcut})`
      : action.label;
  }

  isDanger(action: PresentedCommand): boolean {
    return action.icon === 'delete' || /\b(delete|remove)\b/i.test(action.label);
  }

  iconGlyph(icon?: string): string {
    switch (icon) {
      case 'new-folder': return '+';
      case 'delete': return '\u2715';
      case 'copy': return '\u2398';
      case 'move': return '\u2192';
      case 'refresh': return '\u21BB';
      case 'plugin': return '\u25C6';
      default: return '';
    }
  }

  private scheduleMeasurement(): void {
    if (!this.toolbarRef || typeof requestAnimationFrame !== 'function') return;
    if (this.measurementFrame) cancelAnimationFrame(this.measurementFrame);
    this.measurementFrame = requestAnimationFrame(() => {
      this.measurementFrame = 0;
      this.measure();
    });
  }

  private measure(): void {
    const available = this.toolbarRef.nativeElement.clientWidth;
    const refs = this.measureActionRefs?.toArray() ?? [];
    if (!available || refs.length !== this.actions.length) {
      this.visibleActions = this.actions;
      this.overflowActions = [];
      this.cdr.markForCheck();
      return;
    }

    const widths = refs.map(ref =>
      Math.ceil(ref.nativeElement.getBoundingClientRect().width),
    );
    const overflowWidth = Math.ceil(
      this.measureOverflowRef?.nativeElement.getBoundingClientRect().width || 30,
    );
    const gap = 4;

    const fullWidth = widths.reduce(
      (sum, width, index) => sum + width + (index > 0 ? gap : 0),
      0,
    );

    if (fullWidth <= available) {
      this.visibleActions = this.actions;
      this.overflowActions = [];
      this.overflowOpen = false;
      this.cdr.markForCheck();
      return;
    }

    const usable = Math.max(0, available - overflowWidth - gap);
    let used = 0;
    let visibleCount = 0;

    for (const width of widths) {
      const next = used + (visibleCount > 0 ? gap : 0) + width;
      if (next > usable) break;
      used = next;
      visibleCount++;
    }

    visibleCount = Math.min(
      this.actions.length,
      Math.max(Math.min(this.minVisible, this.actions.length), visibleCount),
    );

    this.visibleActions = this.actions.slice(0, visibleCount);
    this.overflowActions = this.actions.slice(visibleCount);
    if (!this.overflowActions.length) this.overflowOpen = false;
    this.cdr.markForCheck();
  }
}
