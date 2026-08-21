import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import * as i0 from "@angular/core";
export class RegistryValueEditorService {
    constructor() {
        this.active$ = createBehaviorSubject(null);
    }
    edit(request) {
        // Registry edit is intentionally single-instance. Resolve any stale request
        // as cancelled instead of stacking invisible prompt surfaces.
        const active = this.active$.value;
        if (active) {
            active.resolve(null);
        }
        return new Promise(resolve => {
            this.active$.next({ request, resolve });
        });
    }
    resolve(result) {
        const active = this.active$.value;
        if (!active)
            return;
        this.active$.next(null);
        active.resolve(result);
    }
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryValueEditorService, deps: [], target: i0.ɵɵFactoryTarget.Injectable }); }
    static { this.ɵprov = i0.ɵɵngDeclareInjectable({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryValueEditorService, providedIn: 'root' }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryValueEditorService, decorators: [{
            type: Injectable,
            args: [{ providedIn: 'root' }]
        }] });
//# sourceMappingURL=registry-value-editor.service.js.map