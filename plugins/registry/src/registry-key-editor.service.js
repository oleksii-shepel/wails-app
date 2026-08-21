import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import * as i0 from "@angular/core";
export class RegistryKeyEditorService {
    constructor() {
        this.active$ = createBehaviorSubject(null);
    }
    open(request) {
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
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryKeyEditorService, deps: [], target: i0.ɵɵFactoryTarget.Injectable }); }
    static { this.ɵprov = i0.ɵɵngDeclareInjectable({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryKeyEditorService, providedIn: 'root' }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryKeyEditorService, decorators: [{
            type: Injectable,
            args: [{ providedIn: 'root' }]
        }] });
//# sourceMappingURL=registry-key-editor.service.js.map