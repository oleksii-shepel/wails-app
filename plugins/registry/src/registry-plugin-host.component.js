import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RegistryKeyEditorComponent } from './registry-key-editor.component';
import { RegistryValueEditorComponent } from './registry-value-editor.component';
import * as i0 from "@angular/core";
/**
 * Single host-mounted overlay outlet for Registry-owned dialogs.
 * Manager does not own the individual editor components.
 */
export class RegistryPluginHostComponent {
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryPluginHostComponent, deps: [], target: i0.ɵɵFactoryTarget.Component }); }
    static { this.ɵcmp = i0.ɵɵngDeclareComponent({ minVersion: "14.0.0", version: "22.1.2", type: RegistryPluginHostComponent, isStandalone: true, selector: "reassembly-registry-plugin-host", ngImport: i0, template: `
    <app-registry-value-editor></app-registry-value-editor>
    <app-registry-key-editor></app-registry-key-editor>
  `, isInline: true, dependencies: [{ kind: "component", type: RegistryKeyEditorComponent, selector: "app-registry-key-editor" }, { kind: "component", type: RegistryValueEditorComponent, selector: "app-registry-value-editor" }], changeDetection: i0.ChangeDetectionStrategy.OnPush }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryPluginHostComponent, decorators: [{
            type: Component,
            args: [{
                    selector: 'reassembly-registry-plugin-host',
                    standalone: true,
                    imports: [RegistryKeyEditorComponent, RegistryValueEditorComponent],
                    changeDetection: ChangeDetectionStrategy.OnPush,
                    template: `
    <app-registry-value-editor></app-registry-value-editor>
    <app-registry-key-editor></app-registry-key-editor>
  `,
                }]
        }] });
//# sourceMappingURL=registry-plugin-host.component.js.map