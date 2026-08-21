import { Component, Directive, ViewEncapsulation, } from '@angular/core';
import * as i0 from "@angular/core";
/**
 * Shared pane collection chrome used by Directory, Registry and future
 * specialized sibling panes.
 *
 * These primitives intentionally own only visual/layout behavior. Selection,
 * navigation, virtualization and domain semantics remain in the consuming pane.
 */
export class PaneTableComponent {
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableComponent, deps: [], target: i0.ɵɵFactoryTarget.Component }); }
    static { this.ɵcmp = i0.ɵɵngDeclareComponent({ minVersion: "14.0.0", version: "22.1.2", type: PaneTableComponent, isStandalone: true, selector: "reassembly-pane-table", ngImport: i0, template: '<ng-content></ng-content>', isInline: true, styles: ["\n    reassembly-pane-table {\n      display: flex;\n      flex-direction: column;\n      flex: 1 1 0%;\n      min-width: 0;\n      min-height: 0;\n      overflow: hidden;\n      background: var(--vsc-editor-background);\n      color: var(--vsc-foreground);\n    }\n\n    .reassembly-pane-table-header {\n      flex: 0 0 auto;\n      background: var(--vsc-tabbar-background);\n      border-bottom: 1px solid var(--vsc-border);\n      color: var(--vsc-foreground-dim);\n      user-select: none;\n    }\n\n    .reassembly-pane-table-header-row {\n      min-height: 28px;\n      height: 28px;\n      font-weight: 600;\n    }\n\n    .reassembly-pane-table-body {\n      flex: 1 1 0%;\n      min-width: 0;\n      min-height: 0;\n      overflow: auto;\n      outline: none;\n    }\n\n    .reassembly-pane-table-row {\n      box-sizing: border-box;\n      position: relative;\n      min-height: 24px;\n      height: 24px;\n      padding: 0 var(--vsc-padding-md);\n      border-bottom: 1px solid transparent;\n      cursor: default;\n      user-select: none;\n      color: var(--vsc-foreground);\n    }\n\n    .reassembly-pane-table-row:not(.reassembly-pane-table-header-row):hover {\n      background: var(--vsc-list-hover-background);\n    }\n\n    .reassembly-pane-table-row.selected {\n      background: var(--vsc-list-active-background);\n      color: var(--vsc-foreground-bright, var(--vsc-list-active-foreground, var(--vsc-foreground)));\n    }\n\n    .reassembly-pane-table-row.cursor:not(.selected) {\n      background: color-mix(in srgb, var(--vsc-list-active-background) 35%, transparent);\n      box-shadow: inset 2px 0 0 0 var(--vsc-accent-blue);\n    }\n\n    .reassembly-pane-table-row.cursor.selected {\n      outline: none;\n    }\n\n    .reassembly-pane-table-row.drop-target,\n    .reassembly-pane-table-row.drag-over {\n      outline: 1px dashed var(--vsc-focus-border);\n      outline-offset: -1px;\n      background: var(--vsc-list-hover-background);\n    }\n\n    .reassembly-pane-table-cell {\n      box-sizing: border-box;\n      position: relative;\n      min-width: 0;\n      padding: 0 var(--vsc-padding-sm);\n      overflow: hidden;\n      white-space: nowrap;\n      text-overflow: ellipsis;\n      font-size: var(--vsc-font-size);\n    }\n\n    .reassembly-pane-table-divider {\n      position: absolute;\n      top: 0;\n      bottom: 0;\n      right: -3px;\n      width: 6px;\n      cursor: col-resize;\n      background: transparent;\n      z-index: 3;\n    }\n\n    .reassembly-pane-table-divider::before {\n      content: '';\n      position: absolute;\n      top: 2px;\n      bottom: 2px;\n      left: 50%;\n      width: 1px;\n      transform: translateX(-50%);\n      background: var(--vsc-border);\n    }\n\n    .reassembly-pane-table-divider:hover::before {\n      background: var(--vsc-focus-border);\n    }\n\n    .reassembly-pane-table-empty {\n      min-height: 120px;\n      display: flex;\n      flex-direction: column;\n      align-items: center;\n      justify-content: center;\n      gap: 4px;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: var(--vsc-font-size);\n    }\n  "], encapsulation: i0.ViewEncapsulation.None }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableComponent, decorators: [{
            type: Component,
            args: [{ selector: 'reassembly-pane-table', standalone: true, template: '<ng-content></ng-content>', encapsulation: ViewEncapsulation.None, styles: ["\n    reassembly-pane-table {\n      display: flex;\n      flex-direction: column;\n      flex: 1 1 0%;\n      min-width: 0;\n      min-height: 0;\n      overflow: hidden;\n      background: var(--vsc-editor-background);\n      color: var(--vsc-foreground);\n    }\n\n    .reassembly-pane-table-header {\n      flex: 0 0 auto;\n      background: var(--vsc-tabbar-background);\n      border-bottom: 1px solid var(--vsc-border);\n      color: var(--vsc-foreground-dim);\n      user-select: none;\n    }\n\n    .reassembly-pane-table-header-row {\n      min-height: 28px;\n      height: 28px;\n      font-weight: 600;\n    }\n\n    .reassembly-pane-table-body {\n      flex: 1 1 0%;\n      min-width: 0;\n      min-height: 0;\n      overflow: auto;\n      outline: none;\n    }\n\n    .reassembly-pane-table-row {\n      box-sizing: border-box;\n      position: relative;\n      min-height: 24px;\n      height: 24px;\n      padding: 0 var(--vsc-padding-md);\n      border-bottom: 1px solid transparent;\n      cursor: default;\n      user-select: none;\n      color: var(--vsc-foreground);\n    }\n\n    .reassembly-pane-table-row:not(.reassembly-pane-table-header-row):hover {\n      background: var(--vsc-list-hover-background);\n    }\n\n    .reassembly-pane-table-row.selected {\n      background: var(--vsc-list-active-background);\n      color: var(--vsc-foreground-bright, var(--vsc-list-active-foreground, var(--vsc-foreground)));\n    }\n\n    .reassembly-pane-table-row.cursor:not(.selected) {\n      background: color-mix(in srgb, var(--vsc-list-active-background) 35%, transparent);\n      box-shadow: inset 2px 0 0 0 var(--vsc-accent-blue);\n    }\n\n    .reassembly-pane-table-row.cursor.selected {\n      outline: none;\n    }\n\n    .reassembly-pane-table-row.drop-target,\n    .reassembly-pane-table-row.drag-over {\n      outline: 1px dashed var(--vsc-focus-border);\n      outline-offset: -1px;\n      background: var(--vsc-list-hover-background);\n    }\n\n    .reassembly-pane-table-cell {\n      box-sizing: border-box;\n      position: relative;\n      min-width: 0;\n      padding: 0 var(--vsc-padding-sm);\n      overflow: hidden;\n      white-space: nowrap;\n      text-overflow: ellipsis;\n      font-size: var(--vsc-font-size);\n    }\n\n    .reassembly-pane-table-divider {\n      position: absolute;\n      top: 0;\n      bottom: 0;\n      right: -3px;\n      width: 6px;\n      cursor: col-resize;\n      background: transparent;\n      z-index: 3;\n    }\n\n    .reassembly-pane-table-divider::before {\n      content: '';\n      position: absolute;\n      top: 2px;\n      bottom: 2px;\n      left: 50%;\n      width: 1px;\n      transform: translateX(-50%);\n      background: var(--vsc-border);\n    }\n\n    .reassembly-pane-table-divider:hover::before {\n      background: var(--vsc-focus-border);\n    }\n\n    .reassembly-pane-table-empty {\n      min-height: 120px;\n      display: flex;\n      flex-direction: column;\n      align-items: center;\n      justify-content: center;\n      gap: 4px;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: var(--vsc-font-size);\n    }\n  "] }]
        }] });
export class PaneTableHeaderDirective {
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableHeaderDirective, deps: [], target: i0.ɵɵFactoryTarget.Directive }); }
    static { this.ɵdir = i0.ɵɵngDeclareDirective({ minVersion: "14.0.0", version: "22.1.2", type: PaneTableHeaderDirective, isStandalone: true, selector: "[reassemblyPaneTableHeader]", host: { classAttribute: "reassembly-pane-table-header" }, ngImport: i0 }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableHeaderDirective, decorators: [{
            type: Directive,
            args: [{
                    selector: '[reassemblyPaneTableHeader]',
                    standalone: true,
                    host: { class: 'reassembly-pane-table-header' },
                }]
        }] });
export class PaneTableHeaderRowDirective {
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableHeaderRowDirective, deps: [], target: i0.ɵɵFactoryTarget.Directive }); }
    static { this.ɵdir = i0.ɵɵngDeclareDirective({ minVersion: "14.0.0", version: "22.1.2", type: PaneTableHeaderRowDirective, isStandalone: true, selector: "[reassemblyPaneTableHeaderRow]", host: { classAttribute: "reassembly-pane-table-header-row reassembly-pane-table-row" }, ngImport: i0 }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableHeaderRowDirective, decorators: [{
            type: Directive,
            args: [{
                    selector: '[reassemblyPaneTableHeaderRow]',
                    standalone: true,
                    host: { class: 'reassembly-pane-table-header-row reassembly-pane-table-row' },
                }]
        }] });
export class PaneTableBodyDirective {
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableBodyDirective, deps: [], target: i0.ɵɵFactoryTarget.Directive }); }
    static { this.ɵdir = i0.ɵɵngDeclareDirective({ minVersion: "14.0.0", version: "22.1.2", type: PaneTableBodyDirective, isStandalone: true, selector: "[reassemblyPaneTableBody]", host: { classAttribute: "reassembly-pane-table-body" }, ngImport: i0 }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableBodyDirective, decorators: [{
            type: Directive,
            args: [{
                    selector: '[reassemblyPaneTableBody]',
                    standalone: true,
                    host: { class: 'reassembly-pane-table-body' },
                }]
        }] });
export class PaneTableRowDirective {
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableRowDirective, deps: [], target: i0.ɵɵFactoryTarget.Directive }); }
    static { this.ɵdir = i0.ɵɵngDeclareDirective({ minVersion: "14.0.0", version: "22.1.2", type: PaneTableRowDirective, isStandalone: true, selector: "[reassemblyPaneTableRow]", host: { classAttribute: "reassembly-pane-table-row" }, ngImport: i0 }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableRowDirective, decorators: [{
            type: Directive,
            args: [{
                    selector: '[reassemblyPaneTableRow]',
                    standalone: true,
                    host: { class: 'reassembly-pane-table-row' },
                }]
        }] });
export class PaneTableCellDirective {
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableCellDirective, deps: [], target: i0.ɵɵFactoryTarget.Directive }); }
    static { this.ɵdir = i0.ɵɵngDeclareDirective({ minVersion: "14.0.0", version: "22.1.2", type: PaneTableCellDirective, isStandalone: true, selector: "[reassemblyPaneTableCell]", host: { classAttribute: "reassembly-pane-table-cell" }, ngImport: i0 }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableCellDirective, decorators: [{
            type: Directive,
            args: [{
                    selector: '[reassemblyPaneTableCell]',
                    standalone: true,
                    host: { class: 'reassembly-pane-table-cell' },
                }]
        }] });
export class PaneTableDividerDirective {
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableDividerDirective, deps: [], target: i0.ɵɵFactoryTarget.Directive }); }
    static { this.ɵdir = i0.ɵɵngDeclareDirective({ minVersion: "14.0.0", version: "22.1.2", type: PaneTableDividerDirective, isStandalone: true, selector: "[reassemblyPaneTableDivider]", host: { classAttribute: "reassembly-pane-table-divider" }, ngImport: i0 }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableDividerDirective, decorators: [{
            type: Directive,
            args: [{
                    selector: '[reassemblyPaneTableDivider]',
                    standalone: true,
                    host: { class: 'reassembly-pane-table-divider' },
                }]
        }] });
export class PaneTableEmptyDirective {
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableEmptyDirective, deps: [], target: i0.ɵɵFactoryTarget.Directive }); }
    static { this.ɵdir = i0.ɵɵngDeclareDirective({ minVersion: "14.0.0", version: "22.1.2", type: PaneTableEmptyDirective, isStandalone: true, selector: "[reassemblyPaneTableEmpty]", host: { classAttribute: "reassembly-pane-table-empty" }, ngImport: i0 }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: PaneTableEmptyDirective, decorators: [{
            type: Directive,
            args: [{
                    selector: '[reassemblyPaneTableEmpty]',
                    standalone: true,
                    host: { class: 'reassembly-pane-table-empty' },
                }]
        }] });
//# sourceMappingURL=pane-table.component.js.map