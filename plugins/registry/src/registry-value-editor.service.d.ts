import * as i0 from "@angular/core";
export interface RegistryValueEditRequest {
    path: string;
    name: string;
    type: string;
    data: string;
}
export interface RegistryValueEditResult {
    type: string;
    data: string;
}
interface ActiveRegistryValueEdit {
    request: RegistryValueEditRequest;
    resolve: (result: RegistryValueEditResult | null) => void;
}
export declare class RegistryValueEditorService {
    readonly active$: import("@epikodelabs/streamix").BehaviorSubject<ActiveRegistryValueEdit | null>;
    edit(request: RegistryValueEditRequest): Promise<RegistryValueEditResult | null>;
    resolve(result: RegistryValueEditResult | null): void;
    static ɵfac: i0.ɵɵFactoryDeclaration<RegistryValueEditorService, never>;
    static ɵprov: i0.ɵɵInjectableDeclaration<RegistryValueEditorService>;
}
export {};
