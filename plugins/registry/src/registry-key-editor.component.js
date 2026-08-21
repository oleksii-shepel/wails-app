import { ChangeDetectionStrategy, Component, ViewChild, } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import * as i0 from "@angular/core";
import * as i1 from "./registry-key-editor.service";
import * as i2 from "@angular/common";
import * as i3 from "@angular/forms";
export class RegistryKeyEditorComponent {
    constructor(editor, cdr) {
        this.editor = editor;
        this.cdr = cdr;
        this.request = null;
        this.name = '';
        this.destroy$ = createSubject();
    }
    ngOnInit() {
        this.editor.active$
            .pipe(takeUntil(this.destroy$))
            .subscribe(active => {
            this.request = active?.request ?? null;
            this.name = active?.request.name ?? '';
            this.cdr.markForCheck();
            if (active) {
                requestAnimationFrame(() => {
                    const input = this.nameInput?.nativeElement;
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
        const name = this.name.trim();
        if (!name)
            return;
        this.editor.resolve(name);
    }
    cancel() {
        this.editor.resolve(null);
    }
    onDialogKeyDown(event) {
        event.stopPropagation();
        if (event.key === 'Escape') {
            event.preventDefault();
            this.cancel();
        }
    }
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryKeyEditorComponent, deps: [{ token: i1.RegistryKeyEditorService }, { token: i0.ChangeDetectorRef }], target: i0.ɵɵFactoryTarget.Component }); }
    static { this.ɵcmp = i0.ɵɵngDeclareComponent({ minVersion: "14.0.0", version: "22.1.2", type: RegistryKeyEditorComponent, isStandalone: true, selector: "app-registry-key-editor", viewQueries: [{ propertyName: "nameInput", first: true, predicate: ["nameInput"], descendants: true }], ngImport: i0, template: `
    <div
      *ngIf="request"
      class="registry-key-overlay"
      data-input-owned
      (click)="cancel()">
      <section
        class="registry-key-dialog"
        role="dialog"
        aria-modal="true"
        [attr.aria-label]="request.title"
        data-input-owned
        (click)="$event.stopPropagation()"
        (mousedown)="$event.stopPropagation()"
        (mouseup)="$event.stopPropagation()"
        (dblclick)="$event.stopPropagation()"
        (keydown)="onDialogKeyDown($event)"
        (keyup)="$event.stopPropagation()">
        <header>
          <strong>{{ request.title }}</strong>
          <button type="button" class="close" aria-label="Close" (click)="cancel()">×</button>
        </header>

        <form (ngSubmit)="save()" data-input-owned>
          <label>
            <span>Name</span>
            <input
              #nameInput
              name="registry-key-name"
              [(ngModel)]="name"
              autocomplete="off"
              spellcheck="false"
              data-input-owned
              (click)="$event.stopPropagation()"
              (mousedown)="$event.stopPropagation()"
              (mouseup)="$event.stopPropagation()"
              (keydown)="$event.stopPropagation()"
              (keyup)="$event.stopPropagation()" />
          </label>

          <footer>
            <button
            type="button"
            (mousedown)="$event.stopPropagation()"
            (click)="cancel(); $event.stopPropagation()">
            Cancel
          </button>
            <button
              type="submit"
              class="primary"
              [disabled]="!name.trim()"
              (mousedown)="$event.stopPropagation()"
              (click)="$event.stopPropagation()">
              {{ request.confirmText }}
            </button>
          </footer>
        </form>
      </section>
    </div>
  `, isInline: true, styles: ["\n    :host {\n      position: fixed;\n      inset: 0;\n      z-index: 40000;\n      pointer-events: none;\n    }\n\n    .registry-key-overlay {\n      position: fixed;\n      inset: 0;\n      display: grid;\n      place-items: center;\n      padding: 24px;\n      background: var(--vsc-scrim);\n      pointer-events: auto;\n    }\n\n    .registry-key-dialog {\n      box-sizing: border-box;\n      width: min(460px, calc(100vw - 48px));\n      background: var(--vsc-editor-background);\n      color: var(--vsc-foreground);\n      border: 1px solid var(--vsc-border-bright);\n      border-radius: 6px;\n      box-shadow: var(--vsc-widget-shadow), 0 20px 48px var(--vsc-scrim);\n      pointer-events: auto;\n    }\n\n    header {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 12px;\n      padding: 12px 14px;\n      border-bottom: 1px solid var(--vsc-border);\n    }\n\n    .close {\n      width: 28px;\n      height: 28px;\n      padding: 0;\n      border: 0;\n      background: transparent;\n      color: inherit;\n      cursor: pointer;\n    }\n\n    form {\n      display: grid;\n      gap: 14px;\n      padding: 14px;\n      pointer-events: auto;\n    }\n\n    label {\n      display: grid;\n      grid-template-columns: 64px minmax(0, 1fr);\n      gap: 10px;\n      align-items: center;\n    }\n\n    label > span {\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 11px;\n    }\n\n    input {\n      box-sizing: border-box;\n      width: 100%;\n      height: 28px;\n      padding: 4px 8px;\n      border: 1px solid var(--vsc-input-border, var(--vsc-border));\n      border-radius: 3px;\n      background: var(--vsc-input-background);\n      color: var(--vsc-input-foreground, var(--vsc-foreground));\n      font: inherit;\n      pointer-events: auto;\n    }\n\n    input:focus {\n      outline: 1px solid var(--vsc-focus-border, var(--vsc-accent-blue));\n      outline-offset: -1px;\n    }\n\n    footer {\n      display: flex;\n      justify-content: flex-end;\n      gap: 8px;\n    }\n\n    button {\n      min-height: 28px;\n      padding: 4px 12px;\n      border: 1px solid var(--vsc-button-border, var(--vsc-border));\n      border-radius: 3px;\n      background: var(--vsc-button-secondary-background, var(--vsc-panel-background));\n      color: var(--vsc-button-secondary-foreground, var(--vsc-foreground));\n      cursor: pointer;\n      pointer-events: auto;\n    }\n\n    button.primary {\n      background: var(--vsc-button-background);\n      color: var(--vsc-button-foreground);\n    }\n\n    button:disabled {\n      opacity: .45;\n      cursor: default;\n    }\n  "], dependencies: [{ kind: "ngmodule", type: CommonModule }, { kind: "directive", type: i2.NgIf, selector: "[ngIf]", inputs: ["ngIf", "ngIfThen", "ngIfElse"] }, { kind: "ngmodule", type: FormsModule }, { kind: "directive", type: i3.ɵNgNoValidate, selector: "form:not([ngNoForm]):not([ngNativeValidate])" }, { kind: "directive", type: i3.DefaultValueAccessor, selector: "input:not([type=checkbox]):not([ngNoCva])[formControlName],textarea:not([ngNoCva])[formControlName],input:not([type=checkbox]):not([ngNoCva])[formControl],textarea:not([ngNoCva])[formControl],input:not([type=checkbox]):not([ngNoCva])[ngModel],textarea:not([ngNoCva])[ngModel],[ngDefaultControl]" }, { kind: "directive", type: i3.NgControlStatus, selector: "[formControlName],[ngModel],[formControl]" }, { kind: "directive", type: i3.NgControlStatusGroup, selector: "[formGroupName],[formArrayName],[ngModelGroup],[formGroup],[formArray],form:not([ngNoForm]),[ngForm]" }, { kind: "directive", type: i3.NgModel, selector: "[ngModel]:not([formControlName]):not([formControl])", inputs: ["name", "disabled", "ngModel", "ngModelOptions"], outputs: ["ngModelChange"], exportAs: ["ngModel"] }, { kind: "directive", type: i3.NgForm, selector: "form:not([ngNoForm]):not([formGroup]):not([formArray]),ng-form,[ngForm]", inputs: ["ngFormOptions"], outputs: ["ngSubmit"], exportAs: ["ngForm"] }], changeDetection: i0.ChangeDetectionStrategy.OnPush }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryKeyEditorComponent, decorators: [{
            type: Component,
            args: [{ selector: 'app-registry-key-editor', standalone: true, imports: [CommonModule, FormsModule], changeDetection: ChangeDetectionStrategy.OnPush, template: `
    <div
      *ngIf="request"
      class="registry-key-overlay"
      data-input-owned
      (click)="cancel()">
      <section
        class="registry-key-dialog"
        role="dialog"
        aria-modal="true"
        [attr.aria-label]="request.title"
        data-input-owned
        (click)="$event.stopPropagation()"
        (mousedown)="$event.stopPropagation()"
        (mouseup)="$event.stopPropagation()"
        (dblclick)="$event.stopPropagation()"
        (keydown)="onDialogKeyDown($event)"
        (keyup)="$event.stopPropagation()">
        <header>
          <strong>{{ request.title }}</strong>
          <button type="button" class="close" aria-label="Close" (click)="cancel()">×</button>
        </header>

        <form (ngSubmit)="save()" data-input-owned>
          <label>
            <span>Name</span>
            <input
              #nameInput
              name="registry-key-name"
              [(ngModel)]="name"
              autocomplete="off"
              spellcheck="false"
              data-input-owned
              (click)="$event.stopPropagation()"
              (mousedown)="$event.stopPropagation()"
              (mouseup)="$event.stopPropagation()"
              (keydown)="$event.stopPropagation()"
              (keyup)="$event.stopPropagation()" />
          </label>

          <footer>
            <button
            type="button"
            (mousedown)="$event.stopPropagation()"
            (click)="cancel(); $event.stopPropagation()">
            Cancel
          </button>
            <button
              type="submit"
              class="primary"
              [disabled]="!name.trim()"
              (mousedown)="$event.stopPropagation()"
              (click)="$event.stopPropagation()">
              {{ request.confirmText }}
            </button>
          </footer>
        </form>
      </section>
    </div>
  `, styles: ["\n    :host {\n      position: fixed;\n      inset: 0;\n      z-index: 40000;\n      pointer-events: none;\n    }\n\n    .registry-key-overlay {\n      position: fixed;\n      inset: 0;\n      display: grid;\n      place-items: center;\n      padding: 24px;\n      background: var(--vsc-scrim);\n      pointer-events: auto;\n    }\n\n    .registry-key-dialog {\n      box-sizing: border-box;\n      width: min(460px, calc(100vw - 48px));\n      background: var(--vsc-editor-background);\n      color: var(--vsc-foreground);\n      border: 1px solid var(--vsc-border-bright);\n      border-radius: 6px;\n      box-shadow: var(--vsc-widget-shadow), 0 20px 48px var(--vsc-scrim);\n      pointer-events: auto;\n    }\n\n    header {\n      display: flex;\n      align-items: center;\n      justify-content: space-between;\n      gap: 12px;\n      padding: 12px 14px;\n      border-bottom: 1px solid var(--vsc-border);\n    }\n\n    .close {\n      width: 28px;\n      height: 28px;\n      padding: 0;\n      border: 0;\n      background: transparent;\n      color: inherit;\n      cursor: pointer;\n    }\n\n    form {\n      display: grid;\n      gap: 14px;\n      padding: 14px;\n      pointer-events: auto;\n    }\n\n    label {\n      display: grid;\n      grid-template-columns: 64px minmax(0, 1fr);\n      gap: 10px;\n      align-items: center;\n    }\n\n    label > span {\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 11px;\n    }\n\n    input {\n      box-sizing: border-box;\n      width: 100%;\n      height: 28px;\n      padding: 4px 8px;\n      border: 1px solid var(--vsc-input-border, var(--vsc-border));\n      border-radius: 3px;\n      background: var(--vsc-input-background);\n      color: var(--vsc-input-foreground, var(--vsc-foreground));\n      font: inherit;\n      pointer-events: auto;\n    }\n\n    input:focus {\n      outline: 1px solid var(--vsc-focus-border, var(--vsc-accent-blue));\n      outline-offset: -1px;\n    }\n\n    footer {\n      display: flex;\n      justify-content: flex-end;\n      gap: 8px;\n    }\n\n    button {\n      min-height: 28px;\n      padding: 4px 12px;\n      border: 1px solid var(--vsc-button-border, var(--vsc-border));\n      border-radius: 3px;\n      background: var(--vsc-button-secondary-background, var(--vsc-panel-background));\n      color: var(--vsc-button-secondary-foreground, var(--vsc-foreground));\n      cursor: pointer;\n      pointer-events: auto;\n    }\n\n    button.primary {\n      background: var(--vsc-button-background);\n      color: var(--vsc-button-foreground);\n    }\n\n    button:disabled {\n      opacity: .45;\n      cursor: default;\n    }\n  "] }]
        }], ctorParameters: () => [{ type: i1.RegistryKeyEditorService }, { type: i0.ChangeDetectorRef }], propDecorators: { nameInput: [{
                type: ViewChild,
                args: ['nameInput']
            }] } });
//# sourceMappingURL=registry-key-editor.component.js.map