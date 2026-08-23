import {
  Component,
  Directive,
  ViewEncapsulation,
} from '@angular/core';

/**
 * Shared pane collection chrome used by Directory, Registry and future
 * specialized sibling panes.
 *
 * These primitives intentionally own only visual/layout behavior. Selection,
 * navigation, virtualization and domain semantics remain in the consuming pane.
 */
@Component({
  selector: 'reassembly-pane-table',
  standalone: true,
  template: '<ng-content></ng-content>',
  encapsulation: ViewEncapsulation.None,
  styles: [`
    reassembly-pane-table {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background);
      color: var(--vsc-foreground);
    }

    .reassembly-pane-table-header {
      flex: 0 0 auto;
      background: var(--vsc-tabbar-background);
      border-bottom: 1px solid var(--vsc-border);
      color: var(--vsc-foreground-dim);
      user-select: none;
    }

    .reassembly-pane-table-header-row {
      min-height: 28px;
      height: 28px;
      font-weight: 600;
    }

    .reassembly-pane-table-body {
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: auto;
      outline: none;
    }

    .reassembly-pane-table-row {
      box-sizing: border-box;
      position: relative;
      width: max-content;
      min-width: 100%;
      min-height: 24px;
      height: 24px;
      padding: 0 var(--vsc-padding-md);
      border-bottom: 1px solid transparent;
      cursor: default;
      user-select: none;
      color: var(--vsc-foreground);
    }

    .reassembly-pane-table-row:not(.reassembly-pane-table-header-row):hover {
      background: var(--vsc-list-hover-background);
    }

    .reassembly-pane-table-row.selected {
      background: var(--vsc-list-active-background);
      color: var(--vsc-foreground-bright, var(--vsc-list-active-foreground, var(--vsc-foreground)));
    }

    .reassembly-pane-table-row.cursor:not(.selected) {
      background: color-mix(in srgb, var(--vsc-list-active-background) 35%, transparent);
      box-shadow: inset 2px 0 0 0 var(--vsc-accent-blue);
    }

    .reassembly-pane-table-row.cursor.selected {
      outline: none;
    }

    /*
     * Pane ownership must be visible on the row itself, not only on the outer
     * pane border. The active pane is the transfer source; the inactive pane is
     * the destination. Keep inactive selections legible but deliberately quiet.
     */
    .pane:not(.active-pane) .reassembly-pane-table-row.selected {
      background: var(
        --vsc-list-inactive-selection,
        color-mix(in srgb, var(--vsc-list-active-background) 48%, transparent)
      );
      color: var(--vsc-foreground);
    }

    .pane:not(.active-pane) .reassembly-pane-table-row.cursor {
      box-shadow: inset 2px 0 0 color-mix(
        in srgb,
        var(--vsc-foreground-dim, var(--vsc-border-bright)) 58%,
        transparent
      );
    }

    .pane:not(.active-pane) .reassembly-pane-table-row.cursor:not(.selected) {
      background: color-mix(
        in srgb,
        var(--vsc-list-inactive-selection, var(--vsc-list-active-background)) 38%,
        transparent
      );
    }

    .pane.active-pane .reassembly-pane-table-row.cursor {
      box-shadow:
        inset 3px 0 0 var(--vsc-accent-blue, var(--vsc-focus-border)),
        inset 0 0 0 1px color-mix(
          in srgb,
          var(--vsc-accent-blue, var(--vsc-focus-border)) 42%,
          transparent
        );
    }

    .pane.active-pane .reassembly-pane-table-row.cursor.selected {
      background: color-mix(
        in srgb,
        var(--vsc-list-active-background) 82%,
        var(--vsc-accent-blue, var(--vsc-focus-border)) 18%
      );
    }

    .reassembly-pane-table-row.drop-target,
    .reassembly-pane-table-row.drag-over {
      outline: 1px dashed var(--vsc-focus-border);
      outline-offset: -1px;
      background: var(--vsc-list-hover-background);
    }

    .reassembly-pane-table-cell {
      box-sizing: border-box;
      position: relative;
      min-width: 0;
      padding: 0 var(--vsc-padding-sm);
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
      font-size: var(--vsc-font-size);
    }

    .reassembly-pane-table-divider {
      position: absolute;
      top: 0;
      bottom: 0;
      right: -3px;
      width: 6px;
      cursor: col-resize;
      background: transparent;
      z-index: 3;
    }

    .reassembly-pane-table-divider::before {
      content: '';
      position: absolute;
      top: 2px;
      bottom: 2px;
      left: 50%;
      width: 1px;
      transform: translateX(-50%);
      background: var(--vsc-border);
    }

    .reassembly-pane-table-divider:hover::before {
      background: var(--vsc-focus-border);
    }

    .reassembly-pane-table-empty {
      min-height: 120px;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 4px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: var(--vsc-font-size);
    }
  `],
})
export class PaneTableComponent {}

@Directive({
  selector: '[reassemblyPaneTableHeader]',
  standalone: true,
  host: { class: 'reassembly-pane-table-header' },
})
export class PaneTableHeaderDirective {}

@Directive({
  selector: '[reassemblyPaneTableHeaderRow]',
  standalone: true,
  host: { class: 'reassembly-pane-table-header-row reassembly-pane-table-row' },
})
export class PaneTableHeaderRowDirective {}

@Directive({
  selector: '[reassemblyPaneTableBody]',
  standalone: true,
  host: { class: 'reassembly-pane-table-body' },
})
export class PaneTableBodyDirective {}

@Directive({
  selector: '[reassemblyPaneTableRow]',
  standalone: true,
  host: { class: 'reassembly-pane-table-row' },
})
export class PaneTableRowDirective {}

@Directive({
  selector: '[reassemblyPaneTableCell]',
  standalone: true,
  host: { class: 'reassembly-pane-table-cell' },
})
export class PaneTableCellDirective {}

@Directive({
  selector: '[reassemblyPaneTableDivider]',
  standalone: true,
  host: { class: 'reassembly-pane-table-divider' },
})
export class PaneTableDividerDirective {}

@Directive({
  selector: '[reassemblyPaneTableEmpty]',
  standalone: true,
  host: { class: 'reassembly-pane-table-empty' },
})
export class PaneTableEmptyDirective {}
