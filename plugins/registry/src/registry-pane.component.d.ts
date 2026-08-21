import { ElementRef, EventEmitter, OnDestroy } from '@angular/core';
import { RegistryFileInfo as FileInfo } from './registry-model';
import * as i0 from "@angular/core";
export type RegistryPaneViewMode = 'list' | 'details';
type RegistryEntryKind = 'parent' | 'hive' | 'key' | 'value';
export interface RegistryEntryPointerEvent {
    entry: FileInfo;
    event: MouseEvent;
}
export interface RegistryEntryDragEvent {
    entry: FileInfo;
    event: DragEvent;
}
/**
 * Registry-specific collection presentation.
 *
 * Navigation, selection, virtualization and commands remain owned by the pane
 * controller. This component owns only registry semantics and presentation:
 * hives, subkeys, REG_* types and inline value payloads.
 */
export declare class RegistryPaneComponent implements OnDestroy {
    viewport?: ElementRef<HTMLElement>;
    paneId: 'left' | 'right';
    path: string;
    viewMode: RegistryPaneViewMode;
    entries: readonly FileInfo[];
    selectedPaths: ReadonlySet<string>;
    cursorPath: string | null;
    visibleEntryCount: number;
    renderedStart: number;
    virtualEnabled: boolean;
    beforePx: number;
    afterPx: number;
    loading: boolean;
    filterText: string;
    dragOverPath: string | null;
    dropIntent: 'copy' | 'move' | 'invalid' | null;
    backgroundDropActive: boolean;
    scrolled: EventEmitter<Event>;
    backgroundClick: EventEmitter<MouseEvent>;
    backgroundContextMenu: EventEmitter<MouseEvent>;
    backgroundDragOver: EventEmitter<DragEvent>;
    backgroundDragLeave: EventEmitter<DragEvent>;
    backgroundDrop: EventEmitter<DragEvent>;
    entrySelect: EventEmitter<RegistryEntryPointerEvent>;
    entryOpen: EventEmitter<import("@reassembly/core/contracts/files").FileInfo>;
    entryContextMenu: EventEmitter<RegistryEntryPointerEvent>;
    entryDragStart: EventEmitter<RegistryEntryDragEvent>;
    entryDragEnd: EventEmitter<void>;
    entryDragOver: EventEmitter<RegistryEntryDragEvent>;
    entryDragLeave: EventEmitter<import("@reassembly/core/contracts/files").FileInfo>;
    entryDrop: EventEmitter<RegistryEntryDragEvent>;
    private readonly widths;
    private resizeKey;
    private resizeStartX;
    private resizeStartWidth;
    private moveHandler?;
    private upHandler?;
    private readonly minWidth;
    get viewportElement(): HTMLElement | undefined;
    get detailsGridTemplate(): string;
    get rootGridTemplate(): string;
    /**
     * The Registry root is structurally fixed. Keep it visible even during the
     * short interval between mounting the dedicated sibling and provider
     * activation/list completion.
     */
    get rootEntries(): readonly FileInfo[];
    get effectiveVisibleEntryCount(): number;
    get isRootView(): boolean;
    get parentEntry(): FileInfo | null;
    get keyEntries(): readonly FileInfo[];
    get valueEntries(): readonly FileInfo[];
    get focusedValueEntry(): FileInfo | null;
    ngOnDestroy(): void;
    trackEntry(_index: number, entry: FileInfo): string;
    entryIndex(entry: FileInfo): number;
    entryKind(entry: FileInfo): RegistryEntryKind;
    kindLabel(entry: FileInfo): string;
    typeLabel(entry: FileInfo): string;
    metaLabel(entry: FileInfo): string;
    dataLabel(entry: FileInfo): string;
    badgeLabel(entry: FileInfo): string;
    icon(entry: FileInfo): string;
    ariaLabel(entry: FileInfo): string;
    beginResize(event: MouseEvent, key: string): void;
    private resize;
    private endResize;
    private track;
    private get storageKey();
    private restoreWidths;
    private persistWidths;
    private isHive;
    private normalizeRegistryPath;
    private formatValueMeta;
    private formatValueData;
    private formatByteSize;
    static ɵfac: i0.ɵɵFactoryDeclaration<RegistryPaneComponent, never>;
    static ɵcmp: i0.ɵɵComponentDeclaration<RegistryPaneComponent, "app-registry-pane", never, { "paneId": { "alias": "paneId"; "required": false; }; "path": { "alias": "path"; "required": false; }; "viewMode": { "alias": "viewMode"; "required": false; }; "entries": { "alias": "entries"; "required": false; }; "selectedPaths": { "alias": "selectedPaths"; "required": false; }; "cursorPath": { "alias": "cursorPath"; "required": false; }; "visibleEntryCount": { "alias": "visibleEntryCount"; "required": false; }; "renderedStart": { "alias": "renderedStart"; "required": false; }; "virtualEnabled": { "alias": "virtualEnabled"; "required": false; }; "beforePx": { "alias": "beforePx"; "required": false; }; "afterPx": { "alias": "afterPx"; "required": false; }; "loading": { "alias": "loading"; "required": false; }; "filterText": { "alias": "filterText"; "required": false; }; "dragOverPath": { "alias": "dragOverPath"; "required": false; }; "dropIntent": { "alias": "dropIntent"; "required": false; }; "backgroundDropActive": { "alias": "backgroundDropActive"; "required": false; }; }, { "scrolled": "scrolled"; "backgroundClick": "backgroundClick"; "backgroundContextMenu": "backgroundContextMenu"; "backgroundDragOver": "backgroundDragOver"; "backgroundDragLeave": "backgroundDragLeave"; "backgroundDrop": "backgroundDrop"; "entrySelect": "entrySelect"; "entryOpen": "entryOpen"; "entryContextMenu": "entryContextMenu"; "entryDragStart": "entryDragStart"; "entryDragEnd": "entryDragEnd"; "entryDragOver": "entryDragOver"; "entryDragLeave": "entryDragLeave"; "entryDrop": "entryDrop"; }, never, never, true, never>;
}
export {};
