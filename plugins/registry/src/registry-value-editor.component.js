import { ChangeDetectionStrategy, Component, ViewChild, } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import * as i0 from "@angular/core";
import * as i1 from "./registry-value-editor.service";
import * as i2 from "@angular/common";
import * as i3 from "@angular/forms";
export class RegistryValueEditorComponent {
    constructor(editor, cdr) {
        this.editor = editor;
        this.cdr = cdr;
        this.registryTypes = [
            'REG_SZ',
            'REG_EXPAND_SZ',
            'REG_BINARY',
            'REG_DWORD',
            'REG_DWORD_BIG_ENDIAN',
            'REG_MULTI_SZ',
            'REG_QWORD',
            'REG_NONE',
        ];
        this.request = null;
        this.type = 'REG_SZ';
        this.data = '';
        this.destroy$ = createSubject();
    }
    ngOnInit() {
        this.editor.active$
            .pipe(takeUntil(this.destroy$))
            .subscribe(active => {
            this.request = active?.request ?? null;
            this.type = active?.request.type ?? 'REG_SZ';
            this.data = active?.request.data ?? '';
            this.cdr.markForCheck();
            if (active) {
                requestAnimationFrame(() => {
                    const input = this.dataInput?.nativeElement;
                    if (!input)
                        return;
                    input.focus({ preventScroll: true });
                    const end = input.value.length;
                    input.setSelectionRange(end, end);
                });
            }
        });
    }
    ngOnDestroy() {
        this.destroy$.next();
        this.destroy$.complete();
    }
    save() {
        if (!this.request)
            return;
        this.editor.resolve({
            type: this.type,
            data: this.data,
        });
    }
    cancel() {
        this.editor.resolve(null);
    }
    onKeyDown(event) {
        if (event.key !== 'Escape')
            return;
        event.preventDefault();
        event.stopPropagation();
        this.cancel();
    }
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryValueEditorComponent, deps: [{ token: i1.RegistryValueEditorService }, { token: i0.ChangeDetectorRef }], target: i0.ɵɵFactoryTarget.Component }); }
    static { this.ɵcmp = i0.ɵɵngDeclareComponent({ minVersion: "14.0.0", version: "22.1.2", type: RegistryValueEditorComponent, isStandalone: true, selector: "app-registry-value-editor", viewQueries: [{ propertyName: "dataInput", first: true, predicate: ["dataInput"], descendants: true }], ngImport: i0, template: `
    <div
      *ngIf="request"
      class="registry-editor-overlay"
      data-input-owned
      (click)="cancel()"
      (keydown)="onKeyDown($event)">
      <section
        class="registry-editor"
        role="dialog"
        aria-modal="true"
        aria-label="Edit Registry Value"
        data-input-owned
        (click)="$event.stopPropagation()"
        (mousedown)="$event.stopPropagation()">
        <header>
          <div>
            <strong>Edit Registry Value</strong>
            <span>{{ request.name }}</span>
          </div>
          <button type="button" class="close" aria-label="Close" (click)="cancel()">×</button>
        </header>

        <form (ngSubmit)="save()" data-input-owned>
          <label>
            <span>Type</span>
            <select
              name="registry-value-type"
              [(ngModel)]="type"
              data-input-owned>
              <option *ngFor="let item of registryTypes" [value]="item">
                {{ item }}
              </option>
            </select>
          </label>

          <label class="data-field">
            <span>Data</span>
            <textarea
              #dataInput
              name="registry-value-data"
              [(ngModel)]="data"
              rows="6"
              spellcheck="false"
              autocomplete="off"
              data-input-owned></textarea>
          </label>

          <footer>
            <button type="button" (click)="cancel()">Cancel</button>
            <button type="submit" class="primary">Save</button>
          </footer>
        </form>
      </section>
    </div>
  `, isInline: true, styles: ["\n    :host {\n      position: fixed;\n      inset: 0;\n      z-index: 40000;\n      pointer-events: none;\n    }\n\n    .registry-editor-overlay {\n      position: fixed;\n      inset: 0;\n      display: grid;\n      place-items: center;\n      padding: 24px;\n      background: var(--vsc-scrim);\n      pointer-events: auto;\n    }\n\n    .registry-editor {\n      box-sizing: border-box;\n      width: min(560px, calc(100vw - 48px));\n      max-height: calc(100vh - 48px);\n      overflow: auto;\n      background: var(--vsc-editor-background);\n      color: var(--vsc-foreground);\n      border: 1px solid var(--vsc-border-bright);\n      border-radius: 6px;\n      box-shadow: var(--vsc-widget-shadow), 0 20px 48px var(--vsc-scrim);\n      pointer-events: auto;\n    }\n\n    header {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 16px;\n      padding: 12px 14px;\n      border-bottom: 1px solid var(--vsc-border);\n    }\n\n    header > div {\n      display: grid;\n      gap: 2px;\n      min-width: 0;\n    }\n\n    header span {\n      overflow: hidden;\n      text-overflow: ellipsis;\n      white-space: nowrap;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 11px;\n    }\n\n    .close {\n      width: 28px;\n      height: 28px;\n      padding: 0;\n      border: 0;\n      background: transparent;\n      color: inherit;\n      cursor: pointer;\n    }\n\n    form {\n      display: grid;\n      gap: 12px;\n      padding: 14px;\n      pointer-events: auto;\n    }\n\n    label {\n      display: grid;\n      grid-template-columns: 72px minmax(0, 1fr);\n      gap: 10px;\n      align-items: center;\n    }\n\n    label > span {\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 11px;\n    }\n\n    input,\n    select,\n    textarea {\n      box-sizing: border-box;\n      width: 100%;\n      border: 1px solid var(--vsc-input-border, var(--vsc-border));\n      border-radius: 3px;\n      background: var(--vsc-input-background);\n      color: var(--vsc-input-foreground, var(--vsc-foreground));\n      font: inherit;\n      pointer-events: auto;\n    }\n\n    input,\n    select {\n      height: 28px;\n      padding: 4px 8px;\n    }\n\n    textarea {\n      min-height: 120px;\n      padding: 7px 8px;\n      resize: vertical;\n      font-family: var(--vsc-editor-font-family, monospace);\n    }\n\n    input:focus,\n    select:focus,\n    textarea:focus {\n      outline: 1px solid var(--vsc-focus-border, var(--vsc-accent-blue));\n      outline-offset: -1px;\n    }\n\n    .data-field {\n      align-items: start;\n    }\n\n    .data-field > span {\n      padding-top: 7px;\n    }\n\n    footer {\n      display: flex;\n      justify-content: flex-end;\n      gap: 8px;\n      padding-top: 2px;\n    }\n\n    button {\n      min-height: 28px;\n      padding: 4px 12px;\n      border: 1px solid var(--vsc-button-border, var(--vsc-border));\n      border-radius: 3px;\n      background: var(--vsc-button-secondary-background, var(--vsc-panel-background));\n      color: var(--vsc-button-secondary-foreground, var(--vsc-foreground));\n      cursor: pointer;\n      pointer-events: auto;\n    }\n\n    button.primary {\n      background: var(--vsc-button-background);\n      color: var(--vsc-button-foreground);\n    }\n  "], dependencies: [{ kind: "ngmodule", type: CommonModule }, { kind: "directive", type: i2.NgForOf, selector: "[ngFor][ngForOf]", inputs: ["ngForOf", "ngForTrackBy", "ngForTemplate"] }, { kind: "directive", type: i2.NgIf, selector: "[ngIf]", inputs: ["ngIf", "ngIfThen", "ngIfElse"] }, { kind: "ngmodule", type: FormsModule }, { kind: "directive", type: i3.ɵNgNoValidate, selector: "form:not([ngNoForm]):not([ngNativeValidate])" }, { kind: "directive", type: i3.NgSelectOption, selector: "option", inputs: ["ngValue", "value"] }, { kind: "directive", type: i3.ɵNgSelectMultipleOption, selector: "option", inputs: ["ngValue", "value"] }, { kind: "directive", type: i3.DefaultValueAccessor, selector: "input:not([type=checkbox]):not([ngNoCva])[formControlName],textarea:not([ngNoCva])[formControlName],input:not([type=checkbox]):not([ngNoCva])[formControl],textarea:not([ngNoCva])[formControl],input:not([type=checkbox]):not([ngNoCva])[ngModel],textarea:not([ngNoCva])[ngModel],[ngDefaultControl]" }, { kind: "directive", type: i3.SelectControlValueAccessor, selector: "select:not([multiple]):not([ngNoCva])[formControlName],select:not([multiple]):not([ngNoCva])[formControl],select:not([multiple]):not([ngNoCva])[ngModel]", inputs: ["compareWith"] }, { kind: "directive", type: i3.NgControlStatus, selector: "[formControlName],[ngModel],[formControl]" }, { kind: "directive", type: i3.NgControlStatusGroup, selector: "[formGroupName],[formArrayName],[ngModelGroup],[formGroup],[formArray],form:not([ngNoForm]),[ngForm]" }, { kind: "directive", type: i3.NgModel, selector: "[ngModel]:not([formControlName]):not([formControl])", inputs: ["name", "disabled", "ngModel", "ngModelOptions"], outputs: ["ngModelChange"], exportAs: ["ngModel"] }, { kind: "directive", type: i3.NgForm, selector: "form:not([ngNoForm]):not([formGroup]):not([formArray]),ng-form,[ngForm]", inputs: ["ngFormOptions"], outputs: ["ngSubmit"], exportAs: ["ngForm"] }], changeDetection: i0.ChangeDetectionStrategy.OnPush }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryValueEditorComponent, decorators: [{
            type: Component,
            args: [{ selector: 'app-registry-value-editor', standalone: true, imports: [CommonModule, FormsModule], changeDetection: ChangeDetectionStrategy.OnPush, template: `
    <div
      *ngIf="request"
      class="registry-editor-overlay"
      data-input-owned
      (click)="cancel()"
      (keydown)="onKeyDown($event)">
      <section
        class="registry-editor"
        role="dialog"
        aria-modal="true"
        aria-label="Edit Registry Value"
        data-input-owned
        (click)="$event.stopPropagation()"
        (mousedown)="$event.stopPropagation()">
        <header>
          <div>
            <strong>Edit Registry Value</strong>
            <span>{{ request.name }}</span>
          </div>
          <button type="button" class="close" aria-label="Close" (click)="cancel()">×</button>
        </header>

        <form (ngSubmit)="save()" data-input-owned>
          <label>
            <span>Type</span>
            <select
              name="registry-value-type"
              [(ngModel)]="type"
              data-input-owned>
              <option *ngFor="let item of registryTypes" [value]="item">
                {{ item }}
              </option>
            </select>
          </label>

          <label class="data-field">
            <span>Data</span>
            <textarea
              #dataInput
              name="registry-value-data"
              [(ngModel)]="data"
              rows="6"
              spellcheck="false"
              autocomplete="off"
              data-input-owned></textarea>
          </label>

          <footer>
            <button type="button" (click)="cancel()">Cancel</button>
            <button type="submit" class="primary">Save</button>
          </footer>
        </form>
      </section>
    </div>
  `, styles: ["\n    :host {\n      position: fixed;\n      inset: 0;\n      z-index: 40000;\n      pointer-events: none;\n    }\n\n    .registry-editor-overlay {\n      position: fixed;\n      inset: 0;\n      display: grid;\n      place-items: center;\n      padding: 24px;\n      background: var(--vsc-scrim);\n      pointer-events: auto;\n    }\n\n    .registry-editor {\n      box-sizing: border-box;\n      width: min(560px, calc(100vw - 48px));\n      max-height: calc(100vh - 48px);\n      overflow: auto;\n      background: var(--vsc-editor-background);\n      color: var(--vsc-foreground);\n      border: 1px solid var(--vsc-border-bright);\n      border-radius: 6px;\n      box-shadow: var(--vsc-widget-shadow), 0 20px 48px var(--vsc-scrim);\n      pointer-events: auto;\n    }\n\n    header {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 16px;\n      padding: 12px 14px;\n      border-bottom: 1px solid var(--vsc-border);\n    }\n\n    header > div {\n      display: grid;\n      gap: 2px;\n      min-width: 0;\n    }\n\n    header span {\n      overflow: hidden;\n      text-overflow: ellipsis;\n      white-space: nowrap;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 11px;\n    }\n\n    .close {\n      width: 28px;\n      height: 28px;\n      padding: 0;\n      border: 0;\n      background: transparent;\n      color: inherit;\n      cursor: pointer;\n    }\n\n    form {\n      display: grid;\n      gap: 12px;\n      padding: 14px;\n      pointer-events: auto;\n    }\n\n    label {\n      display: grid;\n      grid-template-columns: 72px minmax(0, 1fr);\n      gap: 10px;\n      align-items: center;\n    }\n\n    label > span {\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 11px;\n    }\n\n    input,\n    select,\n    textarea {\n      box-sizing: border-box;\n      width: 100%;\n      border: 1px solid var(--vsc-input-border, var(--vsc-border));\n      border-radius: 3px;\n      background: var(--vsc-input-background);\n      color: var(--vsc-input-foreground, var(--vsc-foreground));\n      font: inherit;\n      pointer-events: auto;\n    }\n\n    input,\n    select {\n      height: 28px;\n      padding: 4px 8px;\n    }\n\n    textarea {\n      min-height: 120px;\n      padding: 7px 8px;\n      resize: vertical;\n      font-family: var(--vsc-editor-font-family, monospace);\n    }\n\n    input:focus,\n    select:focus,\n    textarea:focus {\n      outline: 1px solid var(--vsc-focus-border, var(--vsc-accent-blue));\n      outline-offset: -1px;\n    }\n\n    .data-field {\n      align-items: start;\n    }\n\n    .data-field > span {\n      padding-top: 7px;\n    }\n\n    footer {\n      display: flex;\n      justify-content: flex-end;\n      gap: 8px;\n      padding-top: 2px;\n    }\n\n    button {\n      min-height: 28px;\n      padding: 4px 12px;\n      border: 1px solid var(--vsc-button-border, var(--vsc-border));\n      border-radius: 3px;\n      background: var(--vsc-button-secondary-background, var(--vsc-panel-background));\n      color: var(--vsc-button-secondary-foreground, var(--vsc-foreground));\n      cursor: pointer;\n      pointer-events: auto;\n    }\n\n    button.primary {\n      background: var(--vsc-button-background);\n      color: var(--vsc-button-foreground);\n    }\n  "] }]
        }], ctorParameters: () => [{ type: i1.RegistryValueEditorService }, { type: i0.ChangeDetectorRef }], propDecorators: { dataInput: [{
                type: ViewChild,
                args: ['dataInput']
            }] } });
//# sourceMappingURL=registry-value-editor.component.js.map