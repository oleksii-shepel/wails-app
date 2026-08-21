import * as i0 from "@angular/core";
export interface RegistryKeyEditRequest {
    title: string;
    name: string;
    confirmText: string;
}
interface ActiveRegistryKeyEdit {
    request: RegistryKeyEditRequest;
    resolve: (result: string | null) => void;
}
export declare class RegistryKeyEditorService {
    readonly active$: import("@epikodelabs/streamix").BehaviorSubject<ActiveRegistryKeyEdit | null>;
    open(request: RegistryKeyEditRequest): Promise<string | null>;
    resolve(result: string | null): void;
    static ɵfac: i0.ɵɵFactoryDeclaration<RegistryKeyEditorService, never>;
    static ɵprov: i0.ɵɵInjectableDeclaration<RegistryKeyEditorService>;
}
export {};
