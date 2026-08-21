import { CommonModule } from '@angular/common';
import { PaneTableBodyDirective, PaneTableCellDirective, PaneTableComponent, PaneTableDividerDirective, PaneTableEmptyDirective, PaneTableHeaderDirective, PaneTableHeaderRowDirective, PaneTableRowDirective, } from '@reassembly/core/ui/panes';
import { ChangeDetectionStrategy, Component, EventEmitter, Input, Output, ViewChild, } from '@angular/core';
import { RegistryFileType as FileType, } from './registry-model';
import * as i0 from "@angular/core";
import * as i1 from "@angular/common";
const ROOT_HIVE_NAMES = [
    'HKEY_CLASSES_ROOT',
    'HKEY_CURRENT_CONFIG',
    'HKEY_CURRENT_USER',
    'HKEY_LOCAL_MACHINE',
    'HKEY_USERS',
];
const ROOT_HIVE_ENTRIES = ROOT_HIVE_NAMES.map(name => ({
    name,
    path: 'registry://' + name,
    type: FileType.DIRECTORY,
    size: 0,
    created: 0,
    modified: 0,
    accessed: 0,
    permissions: '',
    isHidden: false,
    registryKind: 'Key',
}));
/**
 * Registry-specific collection presentation.
 *
 * Navigation, selection, virtualization and commands remain owned by the pane
 * controller. This component owns only registry semantics and presentation:
 * hives, subkeys, REG_* types and inline value payloads.
 */
export class RegistryPaneComponent {
    constructor() {
        this.paneId = 'left';
        this.path = '';
        this.viewMode = 'list';
        this.entries = [];
        this.selectedPaths = new Set();
        this.cursorPath = null;
        this.visibleEntryCount = 0;
        this.renderedStart = 0;
        this.virtualEnabled = false;
        this.beforePx = 0;
        this.afterPx = 0;
        this.loading = false;
        this.filterText = '';
        this.dragOverPath = null;
        this.dropIntent = null;
        this.backgroundDropActive = false;
        this.scrolled = new EventEmitter();
        this.backgroundClick = new EventEmitter();
        this.backgroundContextMenu = new EventEmitter();
        this.backgroundDragOver = new EventEmitter();
        this.backgroundDragLeave = new EventEmitter();
        this.backgroundDrop = new EventEmitter();
        this.entrySelect = new EventEmitter();
        this.entryOpen = new EventEmitter();
        this.entryContextMenu = new EventEmitter();
        this.entryDragStart = new EventEmitter();
        this.entryDragEnd = new EventEmitter();
        this.entryDragOver = new EventEmitter();
        this.entryDragLeave = new EventEmitter();
        this.entryDrop = new EventEmitter();
        this.widths = new Map();
        this.resizeKey = null;
        this.resizeStartX = 0;
        this.resizeStartWidth = 0;
        this.minWidth = 64;
    }
    get viewportElement() {
        return this.viewport?.nativeElement;
    }
    get detailsGridTemplate() {
        return [
            '28px',
            this.track('name', 'minmax(180px, 1fr)'),
            this.track('kind', '96px'),
            this.track('type', '120px'),
            this.track('data', 'minmax(220px, 1fr)'),
        ].join(' ');
    }
    get rootGridTemplate() {
        return '28px minmax(160px, 1fr) minmax(140px, 240px)';
    }
    /**
     * The Registry root is structurally fixed. Keep it visible even during the
     * short interval between mounting the dedicated sibling and provider
     * activation/list completion.
     */
    get rootEntries() {
        return this.isRootView && this.entries.length === 0
            ? ROOT_HIVE_ENTRIES
            : this.entries;
    }
    get effectiveVisibleEntryCount() {
        return this.isRootView ? this.rootEntries.length : this.visibleEntryCount;
    }
    get isRootView() {
        return this.normalizeRegistryPath(this.path) === 'registry://';
    }
    get parentEntry() {
        return this.entries.find(entry => this.entryKind(entry) === 'parent') ?? null;
    }
    get keyEntries() {
        return this.entries.filter(entry => this.entryKind(entry) === 'key');
    }
    get valueEntries() {
        return this.entries.filter(entry => this.entryKind(entry) === 'value');
    }
    get focusedValueEntry() {
        const byCursor = this.cursorPath
            ? this.entries.find(entry => entry.path === this.cursorPath && this.entryKind(entry) === 'value') ?? null
            : null;
        if (byCursor)
            return byCursor;
        const selected = this.entries.find(entry => this.selectedPaths.has(entry.path) && this.entryKind(entry) === 'value');
        return selected ?? null;
    }
    ngOnDestroy() {
        this.endResize();
    }
    trackEntry(_index, entry) {
        return entry.path;
    }
    entryIndex(entry) {
        const index = this.entries.findIndex(candidate => candidate.path === entry.path);
        return index < 0 ? 0 : index;
    }
    entryKind(entry) {
        if (entry.path === '__parent__')
            return 'parent';
        const kind = String(entry.registryKind ?? '').trim().toLowerCase();
        if (kind === 'value')
            return 'value';
        if (this.isHive(entry))
            return 'hive';
        if (kind === 'key')
            return 'key';
        return entry.type === FileType.DIRECTORY ? 'key' : 'value';
    }
    kindLabel(entry) {
        switch (this.entryKind(entry)) {
            case 'parent':
                return 'Parent';
            case 'hive':
                return 'Hive';
            case 'key':
                return 'Key';
            case 'value':
                return 'Value';
        }
    }
    typeLabel(entry) {
        switch (this.entryKind(entry)) {
            case 'parent':
                return 'Navigate up';
            case 'hive':
                return 'Root hive';
            case 'key':
                return 'Subkey';
            case 'value':
                return entry.registryValueType?.trim() || 'REG_SZ';
        }
    }
    metaLabel(entry) {
        switch (this.entryKind(entry)) {
            case 'parent':
                return 'Open parent key';
            case 'hive':
                return 'Top-level registry hive';
            case 'key':
                return 'Registry key';
            case 'value':
                return this.formatValueMeta(entry);
        }
    }
    dataLabel(entry) {
        switch (this.entryKind(entry)) {
            case 'parent':
                return '';
            case 'hive':
                return 'Top-level registry hive';
            case 'key':
                return '';
            case 'value':
                return this.formatValueData(entry);
        }
    }
    badgeLabel(entry) {
        switch (this.entryKind(entry)) {
            case 'parent':
                return 'Up';
            case 'hive':
                return 'Hive';
            case 'key':
                return 'Key';
            case 'value':
                return entry.registryValueType?.trim() || 'Value';
        }
    }
    icon(entry) {
        switch (this.entryKind(entry)) {
            case 'parent':
                return '..';
            case 'hive':
                return 'H';
            case 'key':
                return 'K';
            case 'value':
                return '=';
        }
    }
    ariaLabel(entry) {
        const parts = [
            entry.name,
            this.kindLabel(entry),
            this.typeLabel(entry),
            this.dataLabel(entry),
        ].filter(value => !!value);
        return parts.join(', ');
    }
    beginResize(event, key) {
        event.preventDefault();
        event.stopPropagation();
        const heading = event.target?.closest('.heading');
        if (!heading)
            return;
        this.resizeKey = key;
        this.resizeStartX = event.clientX;
        this.resizeStartWidth = Math.max(this.minWidth, Math.round(heading.getBoundingClientRect().width));
        this.widths.set(key, this.resizeStartWidth);
        this.moveHandler = move => this.resize(move);
        this.upHandler = () => this.endResize();
        window.addEventListener('mousemove', this.moveHandler);
        window.addEventListener('mouseup', this.upHandler);
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
    }
    resize(event) {
        if (!this.resizeKey)
            return;
        this.widths.set(this.resizeKey, Math.max(this.minWidth, Math.round(this.resizeStartWidth + event.clientX - this.resizeStartX)));
    }
    endResize() {
        this.persistWidths();
        if (this.moveHandler)
            window.removeEventListener('mousemove', this.moveHandler);
        if (this.upHandler)
            window.removeEventListener('mouseup', this.upHandler);
        this.moveHandler = undefined;
        this.upHandler = undefined;
        this.resizeKey = null;
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
    }
    track(key, fallback) {
        if (this.widths.size === 0)
            this.restoreWidths();
        const width = this.widths.get(key);
        return typeof width === 'number' ? `${width}px` : fallback;
    }
    get storageKey() {
        return `reassembly.registryColumns:${this.paneId}`;
    }
    restoreWidths() {
        try {
            const raw = localStorage.getItem(this.storageKey);
            if (!raw)
                return;
            const value = JSON.parse(raw);
            for (const [key, width] of Object.entries(value)) {
                if (typeof width === 'number' &&
                    Number.isFinite(width) &&
                    width >= this.minWidth) {
                    this.widths.set(key, Math.round(width));
                }
            }
        }
        catch {
            // Presentation persistence is best effort.
        }
    }
    persistWidths() {
        try {
            const widths = {};
            for (const [column, width] of this.widths) {
                widths[column] = width;
            }
            localStorage.setItem(this.storageKey, JSON.stringify(widths));
        }
        catch {
            // Presentation persistence is best effort.
        }
    }
    isHive(entry) {
        const path = this.normalizeRegistryPath(entry.path);
        if (!path.toLowerCase().startsWith('registry://'))
            return false;
        const relative = path.slice('registry://'.length).replace(/^\/+/, '');
        return !!relative && !relative.includes('/');
    }
    normalizeRegistryPath(path) {
        const value = String(path ?? '').trim().replace(/\\/g, '/');
        if (!/^registry:\/\//i.test(value))
            return value;
        const rest = value.slice(value.indexOf('://') + 3).replace(/^\/+/, '').replace(/\/+/g, '/');
        return rest ? `registry://${rest}` : 'registry://';
    }
    formatValueMeta(entry) {
        const type = entry.registryValueType?.trim() || 'REG_SZ';
        const bytes = this.formatByteSize(entry.size);
        return bytes ? `${type} value, ${bytes}` : `${type} value`;
    }
    formatValueData(entry) {
        const preview = entry.registryPreview?.trim() ?? '';
        if (preview)
            return preview;
        if (entry.size > 0)
            return '(binary data)';
        return '(empty)';
    }
    formatByteSize(bytes) {
        if (!Number.isFinite(bytes) || bytes <= 0)
            return '';
        return `${Math.round(bytes)} B`;
    }
    static { this.ɵfac = i0.ɵɵngDeclareFactory({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryPaneComponent, deps: [], target: i0.ɵɵFactoryTarget.Component }); }
    static { this.ɵcmp = i0.ɵɵngDeclareComponent({ minVersion: "14.0.0", version: "22.1.2", type: RegistryPaneComponent, isStandalone: true, selector: "app-registry-pane", inputs: { paneId: "paneId", path: "path", viewMode: "viewMode", entries: "entries", selectedPaths: "selectedPaths", cursorPath: "cursorPath", visibleEntryCount: "visibleEntryCount", renderedStart: "renderedStart", virtualEnabled: "virtualEnabled", beforePx: "beforePx", afterPx: "afterPx", loading: "loading", filterText: "filterText", dragOverPath: "dragOverPath", dropIntent: "dropIntent", backgroundDropActive: "backgroundDropActive" }, outputs: { scrolled: "scrolled", backgroundClick: "backgroundClick", backgroundContextMenu: "backgroundContextMenu", backgroundDragOver: "backgroundDragOver", backgroundDragLeave: "backgroundDragLeave", backgroundDrop: "backgroundDrop", entrySelect: "entrySelect", entryOpen: "entryOpen", entryContextMenu: "entryContextMenu", entryDragStart: "entryDragStart", entryDragEnd: "entryDragEnd", entryDragOver: "entryDragOver", entryDragLeave: "entryDragLeave", entryDrop: "entryDrop" }, viewQueries: [{ propertyName: "viewport", first: true, predicate: ["viewport"], descendants: true, static: true }], ngImport: i0, template: `
    <div class="registry-surface-header" *ngIf="isRootView; else keyHeader">
      <div class="registry-surface-title">Registry Hives</div>
      <div class="registry-surface-subtitle">
        System root hives available in this session
      </div>
    </div>

    <ng-template #keyHeader>
      <div class="registry-surface-header">
        <div class="registry-surface-title">Registry Key</div>
        <div class="registry-surface-subtitle">
          {{ keyEntries.length }} subkeys and {{ valueEntries.length }} values
        </div>
      </div>
    </ng-template>

    <reassembly-pane-table class="registry-table">
    <div
      class="registry-details-header"
      reassemblyPaneTableHeader
      reassemblyPaneTableHeaderRow
      *ngIf="viewMode === 'details' && !isRootView"
      [style.grid-template-columns]="detailsGridTemplate">
      <span></span>
      <span class="heading">
        Name
        <i class="resizer" reassemblyPaneTableDivider (mousedown)="beginResize($event, 'name')"></i>
      </span>
      <span class="heading">
        Kind
        <i class="resizer" reassemblyPaneTableDivider (mousedown)="beginResize($event, 'kind')"></i>
      </span>
      <span class="heading">
        Type
        <i class="resizer" reassemblyPaneTableDivider (mousedown)="beginResize($event, 'type')"></i>
      </span>
      <span class="heading">Data</span>
    </div>

    <div
      #viewport
      class="registry-list"
      reassemblyPaneTableBody
      data-pane-content
      tabindex="-1"
      role="listbox"
      [attr.aria-label]="'Registry entries in ' + (path || 'location')"
      [class.view-list]="viewMode === 'list'"
      [class.view-details]="viewMode === 'details'"
      [class.background-drop-target]="backgroundDropActive"
      [attr.aria-busy]="loading"
      (scroll)="scrolled.emit($event)"
      (click)="backgroundClick.emit($event)"
      (contextmenu)="backgroundContextMenu.emit($event)"
      (dragover)="backgroundDragOver.emit($event)"
      (dragleave)="backgroundDragLeave.emit($event)"
      (drop)="backgroundDrop.emit($event)">

      <div
        *ngIf="virtualEnabled && beforePx > 0"
        class="virtual-spacer"
        [style.height.px]="beforePx"
        aria-hidden="true"></div>

      <ng-container *ngIf="isRootView; else groupedKeyView">
        <div class="registry-section registry-section-root">
          <div
            *ngFor="let entry of rootEntries; let virtualIndex = index; trackBy: trackEntry"
            class="registry-entry registry-entry-root"
            reassemblyPaneTableRow
            tabindex="-1"
            role="option"
            draggable="true"
            [attr.data-file-entry]="entry.path"
            [attr.aria-posinset]="renderedStart + virtualIndex + 1"
            [attr.aria-setsize]="isRootView ? effectiveVisibleEntryCount : visibleEntryCount"
            [attr.aria-selected]="selectedPaths.has(entry.path)"
            [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
            [attr.aria-label]="ariaLabel(entry)"
            [class.cursor]="entry.path === cursorPath"
            [class.selected]="selectedPaths.has(entry.path)"
            [class.drop-target]="dragOverPath === entry.path"
            [class.drop-invalid]="dragOverPath === entry.path && dropIntent === 'invalid'"
            [class.drop-copy]="dragOverPath === entry.path && dropIntent === 'copy'"
            [class.drop-move]="dragOverPath === entry.path && dropIntent === 'move'"
            [class.kind-parent]="entryKind(entry) === 'parent'"
            [class.kind-hive]="entryKind(entry) === 'hive'"
            [style.grid-template-columns]="rootGridTemplate"
            (click)="entrySelect.emit({ entry: entry, event: $event })"
            (dblclick)="entryOpen.emit(entry)"
            (contextmenu)="entryContextMenu.emit({ entry: entry, event: $event })"
            (dragstart)="entryDragStart.emit({ entry: entry, event: $event })"
            (dragend)="entryDragEnd.emit()"
            (dragover)="entryDragOver.emit({ entry: entry, event: $event })"
            (dragleave)="entryDragLeave.emit(entry)"
            (drop)="entryDrop.emit({ entry: entry, event: $event })">
            <span class="icon" reassemblyPaneTableCell aria-hidden="true">{{ icon(entry) }}</span>
            <span class="name-cell" reassemblyPaneTableCell>
              <span class="name-line">
                <span class="name">{{ entry.name }}</span>
                <span class="badge">{{ badgeLabel(entry) }}</span>
              </span>
              <span
                class="meta"
                *ngIf="metaLabel(entry) as value"
                [title]="value">{{ value }}</span>
            </span>
            <span class="data root-data" reassemblyPaneTableCell [title]="dataLabel(entry)">{{ dataLabel(entry) }}</span>
          </div>
        </div>
      </ng-container>

      <ng-template #groupedKeyView>
        <div class="registry-value-inspector" *ngIf="focusedValueEntry as entry">
          <div class="registry-value-inspector-header">
            <span class="registry-value-inspector-title">Focused Value</span>
            <span class="registry-value-inspector-type">{{ entry.registryValueType?.trim() || 'REG_SZ' }}</span>
          </div>
          <div class="registry-value-inspector-name">{{ entry.name }}</div>
          <div class="registry-value-inspector-meta">{{ metaLabel(entry) }}</div>
          <div class="registry-value-inspector-data" [title]="dataLabel(entry)">{{ dataLabel(entry) }}</div>
        </div>

        <div class="registry-section" *ngIf="parentEntry as entry">
          <div class="registry-section-label">Navigation</div>
          <ng-container
            [ngTemplateOutlet]="entryRow"
            [ngTemplateOutletContext]="{ $implicit: entry, virtualIndex: entryIndex(entry) }">
          </ng-container>
        </div>

        <div class="registry-section" *ngIf="keyEntries.length">
          <div class="registry-section-label">Subkeys</div>
          <ng-container *ngFor="let entry of keyEntries; trackBy: trackEntry">
            <ng-container
              [ngTemplateOutlet]="entryRow"
              [ngTemplateOutletContext]="{ $implicit: entry, virtualIndex: entryIndex(entry) }">
            </ng-container>
          </ng-container>
        </div>

        <div class="registry-section" *ngIf="valueEntries.length">
          <div class="registry-section-label">Values</div>
          <ng-container *ngFor="let entry of valueEntries; trackBy: trackEntry">
            <ng-container
              [ngTemplateOutlet]="entryRow"
              [ngTemplateOutletContext]="{ $implicit: entry, virtualIndex: entryIndex(entry) }">
            </ng-container>
          </ng-container>
        </div>
      </ng-template>

      <ng-template #entryRow let-entry let-virtualIndex="virtualIndex">
        <div
          class="registry-entry"
          reassemblyPaneTableRow
          tabindex="-1"
          role="option"
          draggable="true"
          [attr.data-file-entry]="entry.path"
          [attr.aria-posinset]="renderedStart + virtualIndex + 1"
          [attr.aria-setsize]="isRootView ? effectiveVisibleEntryCount : visibleEntryCount"
          [attr.aria-selected]="selectedPaths.has(entry.path)"
          [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
          [attr.aria-label]="ariaLabel(entry)"
          [class.cursor]="entry.path === cursorPath"
          [class.selected]="selectedPaths.has(entry.path)"
          [class.drop-target]="dragOverPath === entry.path"
          [class.drop-invalid]="dragOverPath === entry.path && dropIntent === 'invalid'"
          [class.drop-copy]="dragOverPath === entry.path && dropIntent === 'copy'"
          [class.drop-move]="dragOverPath === entry.path && dropIntent === 'move'"
          [class.kind-parent]="entryKind(entry) === 'parent'"
          [class.kind-hive]="entryKind(entry) === 'hive'"
          [class.kind-key]="entryKind(entry) === 'key'"
          [class.kind-value]="entryKind(entry) === 'value'"
          [style.grid-template-columns]="viewMode === 'details' ? detailsGridTemplate : null"
          (click)="entrySelect.emit({ entry: entry, event: $event })"
          (dblclick)="entryOpen.emit(entry)"
          (contextmenu)="entryContextMenu.emit({ entry: entry, event: $event })"
          (dragstart)="entryDragStart.emit({ entry: entry, event: $event })"
          (dragend)="entryDragEnd.emit()"
          (dragover)="entryDragOver.emit({ entry: entry, event: $event })"
          (dragleave)="entryDragLeave.emit(entry)"
          (drop)="entryDrop.emit({ entry: entry, event: $event })">

          <span class="icon" reassemblyPaneTableCell aria-hidden="true">{{ icon(entry) }}</span>

          <span class="name-cell" reassemblyPaneTableCell>
            <span class="name-line">
              <span class="name">{{ entry.name }}</span>
              <span class="badge">{{ badgeLabel(entry) }}</span>
            </span>
            <span
              class="meta"
              *ngIf="metaLabel(entry) as value"
              [title]="value">{{ value }}</span>
          </span>

          <span class="kind" reassemblyPaneTableCell>{{ kindLabel(entry) }}</span>
          <span class="type" reassemblyPaneTableCell>{{ typeLabel(entry) }}</span>
          <span class="data" reassemblyPaneTableCell [title]="dataLabel(entry)">{{ dataLabel(entry) }}</span>
        </div>
      </ng-template>

      <div
        *ngIf="virtualEnabled && afterPx > 0"
        class="virtual-spacer"
        [style.height.px]="afterPx"
        aria-hidden="true"></div>

      <div class="empty" reassemblyPaneTableEmpty *ngIf="!loading && effectiveVisibleEntryCount === 0" role="status">
        <strong>{{ filterText ? 'No matching registry entries.' : 'This registry key is empty.' }}</strong>
        <span *ngIf="filterText">Clear the filter to see all entries.</span>
      </div>
    </div>
    </reassembly-pane-table>
  `, isInline: true, styles: ["\n    :host {\n      display: flex;\n      flex: 1 1 0%;\n      height: 100%;\n      min-width: 0;\n      min-height: 0;\n      flex-direction: column;\n      overflow: hidden;\n    }\n\n    .registry-surface-header {\n      display: grid;\n      gap: 2px;\n      padding: 8px 10px 7px;\n      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));\n      background:\n        linear-gradient(\n          180deg,\n          color-mix(in srgb, var(--vsc-tabbar-background, var(--vsc-panel-background)) 90%, white 10%),\n          var(--vsc-tabbar-background, var(--vsc-panel-background))\n        );\n    }\n\n    .registry-surface-title {\n      font-size: 12px;\n      font-weight: 600;\n      color: var(--vsc-foreground);\n      letter-spacing: .02em;\n    }\n\n    .registry-surface-subtitle {\n      font-size: var(--vsc-font-size);\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-details-header {\n      display: grid;\n      align-items: center;\n      min-height: 28px;\n      height: 28px;\n      padding: 0 var(--vsc-padding-md);\n      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));\n      background: var(--vsc-tabbar-background, var(--vsc-panel-background));\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 10px;\n      user-select: none;      column-gap: var(--pane-column-gap, 6px);\n    }\n\n    .heading {\n      position: relative;\n      min-width: 0;\n      padding-inline: 3px;\n    }\n\n    .resizer {\n      position: absolute;\n      z-index: 3;\n      top: 2px;\n      right: -5px;\n      bottom: 2px;\n      width: 9px;\n      cursor: col-resize;\n    }\n\n    .resizer::after {\n      content: '';\n      position: absolute;\n      top: 3px;\n      bottom: 3px;\n      left: 4px;\n      width: 1px;\n      background: transparent;\n    }\n\n    .resizer:hover::after {\n      background: var(--vsc-focus-border, var(--vsc-accent-blue));\n    }\n\n    .registry-list {\n      flex: 1 1 0%;\n      height: 100%;\n      min-width: 0;\n      min-height: 0;\n      overflow: auto;\n      outline: none;\n      scrollbar-width: thin;\n    }\n\n    .registry-entry {\n      box-sizing: border-box;\n      display: grid;\n      align-items: center;\n\n      min-height: 24px;\n      height: 24px;\n      padding: 0 var(--vsc-padding-md);\n      background: transparent;\n      cursor: default;\n      user-select: none;\n      color: var(--vsc-foreground);\n    \n      column-gap: var(--pane-column-gap, 6px);\n}\n\n    .registry-section {\n      display: grid;\n      gap: 2px;\n      padding: 6px 0 2px;\n    }\n\n    .registry-value-inspector {\n      display: grid;\n      gap: 4px;\n      margin: 10px 10px 4px;\n      padding: 10px 12px;\n      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 45%, transparent);\n      border-radius: 10px;\n      background:\n        linear-gradient(\n          135deg,\n          color-mix(in srgb, var(--vsc-editor-background, #fff) 94%, var(--vsc-accent-blue, #06c) 6%),\n          color-mix(in srgb, var(--vsc-editor-background, #fff) 98%, var(--vsc-panel-background, #fff) 2%)\n        );\n    }\n\n    .registry-value-inspector-header {\n      display: flex;\n      align-items: center;\n      gap: 8px;\n      min-width: 0;\n    }\n\n    .registry-value-inspector-title {\n      font-size: 10px;\n      font-weight: 700;\n      letter-spacing: .08em;\n      text-transform: uppercase;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-value-inspector-type {\n      padding: 1px 6px;\n      border-radius: 999px;\n      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 14%, transparent);\n      color: var(--vsc-accent-blue, var(--vsc-focus-border));\n      font-size: 9px;\n      font-weight: 600;\n    }\n\n    .registry-value-inspector-name {\n      font-size: 12px;\n      font-weight: 600;\n      color: var(--vsc-foreground);\n      overflow: hidden;\n      text-overflow: ellipsis;\n      white-space: nowrap;\n    }\n\n    .registry-value-inspector-meta {\n      font-size: 10px;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-value-inspector-data {\n      padding: 8px 10px;\n      border-radius: 8px;\n      background: color-mix(in srgb, var(--vsc-editor-background, #fff) 82%, black 18%);\n      color: var(--vsc-foreground);\n      font: 12px/1.4 var(--vsc-font-family-mono, ui-monospace, monospace);\n      white-space: pre-wrap;\n      word-break: break-word;\n    }\n\n    .registry-section-root {\n      padding: 0;\n      gap: 0;\n    }\n\n    .registry-section-label {\n      padding: 0 10px 2px;\n      font-size: 10px;\n      font-weight: 700;\n      letter-spacing: .08em;\n      text-transform: uppercase;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-list.view-list .registry-entry {\n      grid-template-columns: 22px minmax(0, 1fr) 110px minmax(120px, 1fr);\n    }\n\n    .registry-list.view-list .type {\n      display: none;\n    }\n\n    .icon {\n      display: inline-flex;\n      align-items: center;\n      justify-content: center;\n      width: 18px;\n      height: 18px;\n      text-align: center;\n      opacity: .9;\n      font: 600 10px/1 var(--vsc-font-family-mono, ui-monospace, monospace);\n      letter-spacing: .04em;\n      border-radius: 999px;\n      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-description-foreground)) 85%, transparent);\n      background: color-mix(in srgb, var(--vsc-editor-background, #fff) 84%, var(--vsc-panel-background, #fff));\n    }\n\n    .name-cell {\n      display: grid;\n      min-width: 0;\n      grid-template-rows: auto auto;\n      align-content: center;\n      gap: 2px;\n      overflow: hidden;\n    }\n\n    .name-line {\n      display: inline-flex;\n      align-items: center;\n      gap: 6px;\n      min-width: 0;\n    }\n\n    .name,\n    .meta,\n    .badge,\n    .kind,\n    .type,\n    .data {\n      min-width: 0;\n      overflow: hidden;\n      text-overflow: ellipsis;\n      white-space: nowrap;\n    }\n\n    .name {\n      font-size: 11px;\n      line-height: 13px;\n    }\n\n    .badge {\n      flex: 0 0 auto;\n      max-width: 88px;\n      padding: 1px 6px;\n      border-radius: 999px;\n      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 12%, transparent);\n      color: var(--vsc-accent-blue, var(--vsc-focus-border));\n      font-size: 9px;\n      line-height: 12px;\n      font-weight: 600;\n      letter-spacing: .03em;\n      text-transform: uppercase;\n    }\n\n    .meta {\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 9px;\n      line-height: 11px;\n    }\n\n    .kind,\n    .type,\n    .data {\n      font-size: 10px;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .kind {\n      font-weight: 500;\n    }\n\n    .registry-entry.kind-parent .badge,\n    .registry-entry.kind-parent .icon {\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-entry.kind-hive .badge,\n    .registry-entry.kind-hive .icon {\n      color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 90%, white 10%);\n    }\n\n    .registry-entry.kind-key .badge,\n    .registry-entry.kind-key .icon {\n      color: color-mix(in srgb, var(--vsc-entry-directory, var(--vsc-accent-blue)) 85%, white 15%);\n    }\n\n    .registry-entry.kind-value .badge,\n    .registry-entry.kind-value .icon {\n      color: color-mix(in srgb, var(--vsc-foreground, #000) 75%, var(--vsc-accent-blue, #06c) 25%);\n    }\n\n    .registry-entry-root {\n      min-height: 24px;\n      height: 24px;\n      padding: 0 var(--vsc-padding-md);\n      border: 0;\n      border-radius: 0;\n      background: transparent;\n    }\n\n    .registry-entry-root .icon {\n      width: 18px;\n      height: 18px;\n      font-size: 10px;\n    }\n\n    .root-data {\n      font-size: 10px;\n    }\n\n    .registry-entry.selected .badge,\n    .registry-entry.selected .meta,\n    .registry-entry.selected .kind,\n    .registry-entry.selected .type,\n    .registry-entry.selected .data {\n      color: inherit;\n      opacity: .88;\n    }\n\n    .virtual-spacer {\n      width: 1px;\n      pointer-events: none;\n    }\n\n    .empty {\n      min-height: 120px;\n      display: flex;\n      flex-direction: column;\n      align-items: center;\n      justify-content: center;\n      gap: 4px;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 11px;\n    }\n\n    .background-drop-target,\n    .drop-target {\n      outline: 1px dashed var(--vsc-focus-border, var(--vsc-accent-blue));\n      outline-offset: -1px;\n    }\n\n    .drop-invalid {\n      opacity: .55;\n    }\n\n    /* visible pane column gutters */\n    .registry-details-header,\n    .registry-entry {\n      column-gap: var(--pane-column-gutter, 12px);\n    }\n\n    .registry-details-header > span:not(:first-child),\n    .registry-entry > span:not(:first-child) {\n      min-width: 0;\n      padding-inline: 2px;\n    }\n\n    /* Resizer sits in the middle of the gutter rather than on top of text. */\n    .registry-details-header .resizer {\n      right: calc(var(--pane-column-gutter, 12px) / -2 - 1px);\n    }\n  "], dependencies: [{ kind: "ngmodule", type: CommonModule }, { kind: "directive", type: i1.NgForOf, selector: "[ngFor][ngForOf]", inputs: ["ngForOf", "ngForTrackBy", "ngForTemplate"] }, { kind: "directive", type: i1.NgIf, selector: "[ngIf]", inputs: ["ngIf", "ngIfThen", "ngIfElse"] }, { kind: "directive", type: i1.NgTemplateOutlet, selector: "[ngTemplateOutlet]", inputs: ["ngTemplateOutletContext", "ngTemplateOutlet", "ngTemplateOutletInjector"] }, { kind: "component", type: PaneTableComponent, selector: "reassembly-pane-table" }, { kind: "directive", type: PaneTableHeaderDirective, selector: "[reassemblyPaneTableHeader]" }, { kind: "directive", type: PaneTableHeaderRowDirective, selector: "[reassemblyPaneTableHeaderRow]" }, { kind: "directive", type: PaneTableBodyDirective, selector: "[reassemblyPaneTableBody]" }, { kind: "directive", type: PaneTableRowDirective, selector: "[reassemblyPaneTableRow]" }, { kind: "directive", type: PaneTableCellDirective, selector: "[reassemblyPaneTableCell]" }, { kind: "directive", type: PaneTableDividerDirective, selector: "[reassemblyPaneTableDivider]" }, { kind: "directive", type: PaneTableEmptyDirective, selector: "[reassemblyPaneTableEmpty]" }], changeDetection: i0.ChangeDetectionStrategy.OnPush }); }
}
i0.ɵɵngDeclareClassMetadata({ minVersion: "12.0.0", version: "22.1.2", ngImport: i0, type: RegistryPaneComponent, decorators: [{
            type: Component,
            args: [{ selector: 'app-registry-pane', standalone: true, imports: [
                        CommonModule,
                        PaneTableComponent,
                        PaneTableHeaderDirective,
                        PaneTableHeaderRowDirective,
                        PaneTableBodyDirective,
                        PaneTableRowDirective,
                        PaneTableCellDirective,
                        PaneTableDividerDirective,
                        PaneTableEmptyDirective,
                    ], changeDetection: ChangeDetectionStrategy.OnPush, template: `
    <div class="registry-surface-header" *ngIf="isRootView; else keyHeader">
      <div class="registry-surface-title">Registry Hives</div>
      <div class="registry-surface-subtitle">
        System root hives available in this session
      </div>
    </div>

    <ng-template #keyHeader>
      <div class="registry-surface-header">
        <div class="registry-surface-title">Registry Key</div>
        <div class="registry-surface-subtitle">
          {{ keyEntries.length }} subkeys and {{ valueEntries.length }} values
        </div>
      </div>
    </ng-template>

    <reassembly-pane-table class="registry-table">
    <div
      class="registry-details-header"
      reassemblyPaneTableHeader
      reassemblyPaneTableHeaderRow
      *ngIf="viewMode === 'details' && !isRootView"
      [style.grid-template-columns]="detailsGridTemplate">
      <span></span>
      <span class="heading">
        Name
        <i class="resizer" reassemblyPaneTableDivider (mousedown)="beginResize($event, 'name')"></i>
      </span>
      <span class="heading">
        Kind
        <i class="resizer" reassemblyPaneTableDivider (mousedown)="beginResize($event, 'kind')"></i>
      </span>
      <span class="heading">
        Type
        <i class="resizer" reassemblyPaneTableDivider (mousedown)="beginResize($event, 'type')"></i>
      </span>
      <span class="heading">Data</span>
    </div>

    <div
      #viewport
      class="registry-list"
      reassemblyPaneTableBody
      data-pane-content
      tabindex="-1"
      role="listbox"
      [attr.aria-label]="'Registry entries in ' + (path || 'location')"
      [class.view-list]="viewMode === 'list'"
      [class.view-details]="viewMode === 'details'"
      [class.background-drop-target]="backgroundDropActive"
      [attr.aria-busy]="loading"
      (scroll)="scrolled.emit($event)"
      (click)="backgroundClick.emit($event)"
      (contextmenu)="backgroundContextMenu.emit($event)"
      (dragover)="backgroundDragOver.emit($event)"
      (dragleave)="backgroundDragLeave.emit($event)"
      (drop)="backgroundDrop.emit($event)">

      <div
        *ngIf="virtualEnabled && beforePx > 0"
        class="virtual-spacer"
        [style.height.px]="beforePx"
        aria-hidden="true"></div>

      <ng-container *ngIf="isRootView; else groupedKeyView">
        <div class="registry-section registry-section-root">
          <div
            *ngFor="let entry of rootEntries; let virtualIndex = index; trackBy: trackEntry"
            class="registry-entry registry-entry-root"
            reassemblyPaneTableRow
            tabindex="-1"
            role="option"
            draggable="true"
            [attr.data-file-entry]="entry.path"
            [attr.aria-posinset]="renderedStart + virtualIndex + 1"
            [attr.aria-setsize]="isRootView ? effectiveVisibleEntryCount : visibleEntryCount"
            [attr.aria-selected]="selectedPaths.has(entry.path)"
            [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
            [attr.aria-label]="ariaLabel(entry)"
            [class.cursor]="entry.path === cursorPath"
            [class.selected]="selectedPaths.has(entry.path)"
            [class.drop-target]="dragOverPath === entry.path"
            [class.drop-invalid]="dragOverPath === entry.path && dropIntent === 'invalid'"
            [class.drop-copy]="dragOverPath === entry.path && dropIntent === 'copy'"
            [class.drop-move]="dragOverPath === entry.path && dropIntent === 'move'"
            [class.kind-parent]="entryKind(entry) === 'parent'"
            [class.kind-hive]="entryKind(entry) === 'hive'"
            [style.grid-template-columns]="rootGridTemplate"
            (click)="entrySelect.emit({ entry: entry, event: $event })"
            (dblclick)="entryOpen.emit(entry)"
            (contextmenu)="entryContextMenu.emit({ entry: entry, event: $event })"
            (dragstart)="entryDragStart.emit({ entry: entry, event: $event })"
            (dragend)="entryDragEnd.emit()"
            (dragover)="entryDragOver.emit({ entry: entry, event: $event })"
            (dragleave)="entryDragLeave.emit(entry)"
            (drop)="entryDrop.emit({ entry: entry, event: $event })">
            <span class="icon" reassemblyPaneTableCell aria-hidden="true">{{ icon(entry) }}</span>
            <span class="name-cell" reassemblyPaneTableCell>
              <span class="name-line">
                <span class="name">{{ entry.name }}</span>
                <span class="badge">{{ badgeLabel(entry) }}</span>
              </span>
              <span
                class="meta"
                *ngIf="metaLabel(entry) as value"
                [title]="value">{{ value }}</span>
            </span>
            <span class="data root-data" reassemblyPaneTableCell [title]="dataLabel(entry)">{{ dataLabel(entry) }}</span>
          </div>
        </div>
      </ng-container>

      <ng-template #groupedKeyView>
        <div class="registry-value-inspector" *ngIf="focusedValueEntry as entry">
          <div class="registry-value-inspector-header">
            <span class="registry-value-inspector-title">Focused Value</span>
            <span class="registry-value-inspector-type">{{ entry.registryValueType?.trim() || 'REG_SZ' }}</span>
          </div>
          <div class="registry-value-inspector-name">{{ entry.name }}</div>
          <div class="registry-value-inspector-meta">{{ metaLabel(entry) }}</div>
          <div class="registry-value-inspector-data" [title]="dataLabel(entry)">{{ dataLabel(entry) }}</div>
        </div>

        <div class="registry-section" *ngIf="parentEntry as entry">
          <div class="registry-section-label">Navigation</div>
          <ng-container
            [ngTemplateOutlet]="entryRow"
            [ngTemplateOutletContext]="{ $implicit: entry, virtualIndex: entryIndex(entry) }">
          </ng-container>
        </div>

        <div class="registry-section" *ngIf="keyEntries.length">
          <div class="registry-section-label">Subkeys</div>
          <ng-container *ngFor="let entry of keyEntries; trackBy: trackEntry">
            <ng-container
              [ngTemplateOutlet]="entryRow"
              [ngTemplateOutletContext]="{ $implicit: entry, virtualIndex: entryIndex(entry) }">
            </ng-container>
          </ng-container>
        </div>

        <div class="registry-section" *ngIf="valueEntries.length">
          <div class="registry-section-label">Values</div>
          <ng-container *ngFor="let entry of valueEntries; trackBy: trackEntry">
            <ng-container
              [ngTemplateOutlet]="entryRow"
              [ngTemplateOutletContext]="{ $implicit: entry, virtualIndex: entryIndex(entry) }">
            </ng-container>
          </ng-container>
        </div>
      </ng-template>

      <ng-template #entryRow let-entry let-virtualIndex="virtualIndex">
        <div
          class="registry-entry"
          reassemblyPaneTableRow
          tabindex="-1"
          role="option"
          draggable="true"
          [attr.data-file-entry]="entry.path"
          [attr.aria-posinset]="renderedStart + virtualIndex + 1"
          [attr.aria-setsize]="isRootView ? effectiveVisibleEntryCount : visibleEntryCount"
          [attr.aria-selected]="selectedPaths.has(entry.path)"
          [attr.aria-current]="entry.path === cursorPath ? 'true' : null"
          [attr.aria-label]="ariaLabel(entry)"
          [class.cursor]="entry.path === cursorPath"
          [class.selected]="selectedPaths.has(entry.path)"
          [class.drop-target]="dragOverPath === entry.path"
          [class.drop-invalid]="dragOverPath === entry.path && dropIntent === 'invalid'"
          [class.drop-copy]="dragOverPath === entry.path && dropIntent === 'copy'"
          [class.drop-move]="dragOverPath === entry.path && dropIntent === 'move'"
          [class.kind-parent]="entryKind(entry) === 'parent'"
          [class.kind-hive]="entryKind(entry) === 'hive'"
          [class.kind-key]="entryKind(entry) === 'key'"
          [class.kind-value]="entryKind(entry) === 'value'"
          [style.grid-template-columns]="viewMode === 'details' ? detailsGridTemplate : null"
          (click)="entrySelect.emit({ entry: entry, event: $event })"
          (dblclick)="entryOpen.emit(entry)"
          (contextmenu)="entryContextMenu.emit({ entry: entry, event: $event })"
          (dragstart)="entryDragStart.emit({ entry: entry, event: $event })"
          (dragend)="entryDragEnd.emit()"
          (dragover)="entryDragOver.emit({ entry: entry, event: $event })"
          (dragleave)="entryDragLeave.emit(entry)"
          (drop)="entryDrop.emit({ entry: entry, event: $event })">

          <span class="icon" reassemblyPaneTableCell aria-hidden="true">{{ icon(entry) }}</span>

          <span class="name-cell" reassemblyPaneTableCell>
            <span class="name-line">
              <span class="name">{{ entry.name }}</span>
              <span class="badge">{{ badgeLabel(entry) }}</span>
            </span>
            <span
              class="meta"
              *ngIf="metaLabel(entry) as value"
              [title]="value">{{ value }}</span>
          </span>

          <span class="kind" reassemblyPaneTableCell>{{ kindLabel(entry) }}</span>
          <span class="type" reassemblyPaneTableCell>{{ typeLabel(entry) }}</span>
          <span class="data" reassemblyPaneTableCell [title]="dataLabel(entry)">{{ dataLabel(entry) }}</span>
        </div>
      </ng-template>

      <div
        *ngIf="virtualEnabled && afterPx > 0"
        class="virtual-spacer"
        [style.height.px]="afterPx"
        aria-hidden="true"></div>

      <div class="empty" reassemblyPaneTableEmpty *ngIf="!loading && effectiveVisibleEntryCount === 0" role="status">
        <strong>{{ filterText ? 'No matching registry entries.' : 'This registry key is empty.' }}</strong>
        <span *ngIf="filterText">Clear the filter to see all entries.</span>
      </div>
    </div>
    </reassembly-pane-table>
  `, styles: ["\n    :host {\n      display: flex;\n      flex: 1 1 0%;\n      height: 100%;\n      min-width: 0;\n      min-height: 0;\n      flex-direction: column;\n      overflow: hidden;\n    }\n\n    .registry-surface-header {\n      display: grid;\n      gap: 2px;\n      padding: 8px 10px 7px;\n      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));\n      background:\n        linear-gradient(\n          180deg,\n          color-mix(in srgb, var(--vsc-tabbar-background, var(--vsc-panel-background)) 90%, white 10%),\n          var(--vsc-tabbar-background, var(--vsc-panel-background))\n        );\n    }\n\n    .registry-surface-title {\n      font-size: 12px;\n      font-weight: 600;\n      color: var(--vsc-foreground);\n      letter-spacing: .02em;\n    }\n\n    .registry-surface-subtitle {\n      font-size: var(--vsc-font-size);\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-details-header {\n      display: grid;\n      align-items: center;\n      min-height: 28px;\n      height: 28px;\n      padding: 0 var(--vsc-padding-md);\n      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));\n      background: var(--vsc-tabbar-background, var(--vsc-panel-background));\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 10px;\n      user-select: none;      column-gap: var(--pane-column-gap, 6px);\n    }\n\n    .heading {\n      position: relative;\n      min-width: 0;\n      padding-inline: 3px;\n    }\n\n    .resizer {\n      position: absolute;\n      z-index: 3;\n      top: 2px;\n      right: -5px;\n      bottom: 2px;\n      width: 9px;\n      cursor: col-resize;\n    }\n\n    .resizer::after {\n      content: '';\n      position: absolute;\n      top: 3px;\n      bottom: 3px;\n      left: 4px;\n      width: 1px;\n      background: transparent;\n    }\n\n    .resizer:hover::after {\n      background: var(--vsc-focus-border, var(--vsc-accent-blue));\n    }\n\n    .registry-list {\n      flex: 1 1 0%;\n      height: 100%;\n      min-width: 0;\n      min-height: 0;\n      overflow: auto;\n      outline: none;\n      scrollbar-width: thin;\n    }\n\n    .registry-entry {\n      box-sizing: border-box;\n      display: grid;\n      align-items: center;\n\n      min-height: 24px;\n      height: 24px;\n      padding: 0 var(--vsc-padding-md);\n      background: transparent;\n      cursor: default;\n      user-select: none;\n      color: var(--vsc-foreground);\n    \n      column-gap: var(--pane-column-gap, 6px);\n}\n\n    .registry-section {\n      display: grid;\n      gap: 2px;\n      padding: 6px 0 2px;\n    }\n\n    .registry-value-inspector {\n      display: grid;\n      gap: 4px;\n      margin: 10px 10px 4px;\n      padding: 10px 12px;\n      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 45%, transparent);\n      border-radius: 10px;\n      background:\n        linear-gradient(\n          135deg,\n          color-mix(in srgb, var(--vsc-editor-background, #fff) 94%, var(--vsc-accent-blue, #06c) 6%),\n          color-mix(in srgb, var(--vsc-editor-background, #fff) 98%, var(--vsc-panel-background, #fff) 2%)\n        );\n    }\n\n    .registry-value-inspector-header {\n      display: flex;\n      align-items: center;\n      gap: 8px;\n      min-width: 0;\n    }\n\n    .registry-value-inspector-title {\n      font-size: 10px;\n      font-weight: 700;\n      letter-spacing: .08em;\n      text-transform: uppercase;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-value-inspector-type {\n      padding: 1px 6px;\n      border-radius: 999px;\n      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 14%, transparent);\n      color: var(--vsc-accent-blue, var(--vsc-focus-border));\n      font-size: 9px;\n      font-weight: 600;\n    }\n\n    .registry-value-inspector-name {\n      font-size: 12px;\n      font-weight: 600;\n      color: var(--vsc-foreground);\n      overflow: hidden;\n      text-overflow: ellipsis;\n      white-space: nowrap;\n    }\n\n    .registry-value-inspector-meta {\n      font-size: 10px;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-value-inspector-data {\n      padding: 8px 10px;\n      border-radius: 8px;\n      background: color-mix(in srgb, var(--vsc-editor-background, #fff) 82%, black 18%);\n      color: var(--vsc-foreground);\n      font: 12px/1.4 var(--vsc-font-family-mono, ui-monospace, monospace);\n      white-space: pre-wrap;\n      word-break: break-word;\n    }\n\n    .registry-section-root {\n      padding: 0;\n      gap: 0;\n    }\n\n    .registry-section-label {\n      padding: 0 10px 2px;\n      font-size: 10px;\n      font-weight: 700;\n      letter-spacing: .08em;\n      text-transform: uppercase;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-list.view-list .registry-entry {\n      grid-template-columns: 22px minmax(0, 1fr) 110px minmax(120px, 1fr);\n    }\n\n    .registry-list.view-list .type {\n      display: none;\n    }\n\n    .icon {\n      display: inline-flex;\n      align-items: center;\n      justify-content: center;\n      width: 18px;\n      height: 18px;\n      text-align: center;\n      opacity: .9;\n      font: 600 10px/1 var(--vsc-font-family-mono, ui-monospace, monospace);\n      letter-spacing: .04em;\n      border-radius: 999px;\n      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-description-foreground)) 85%, transparent);\n      background: color-mix(in srgb, var(--vsc-editor-background, #fff) 84%, var(--vsc-panel-background, #fff));\n    }\n\n    .name-cell {\n      display: grid;\n      min-width: 0;\n      grid-template-rows: auto auto;\n      align-content: center;\n      gap: 2px;\n      overflow: hidden;\n    }\n\n    .name-line {\n      display: inline-flex;\n      align-items: center;\n      gap: 6px;\n      min-width: 0;\n    }\n\n    .name,\n    .meta,\n    .badge,\n    .kind,\n    .type,\n    .data {\n      min-width: 0;\n      overflow: hidden;\n      text-overflow: ellipsis;\n      white-space: nowrap;\n    }\n\n    .name {\n      font-size: 11px;\n      line-height: 13px;\n    }\n\n    .badge {\n      flex: 0 0 auto;\n      max-width: 88px;\n      padding: 1px 6px;\n      border-radius: 999px;\n      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 12%, transparent);\n      color: var(--vsc-accent-blue, var(--vsc-focus-border));\n      font-size: 9px;\n      line-height: 12px;\n      font-weight: 600;\n      letter-spacing: .03em;\n      text-transform: uppercase;\n    }\n\n    .meta {\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 9px;\n      line-height: 11px;\n    }\n\n    .kind,\n    .type,\n    .data {\n      font-size: 10px;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .kind {\n      font-weight: 500;\n    }\n\n    .registry-entry.kind-parent .badge,\n    .registry-entry.kind-parent .icon {\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n    }\n\n    .registry-entry.kind-hive .badge,\n    .registry-entry.kind-hive .icon {\n      color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-border)) 90%, white 10%);\n    }\n\n    .registry-entry.kind-key .badge,\n    .registry-entry.kind-key .icon {\n      color: color-mix(in srgb, var(--vsc-entry-directory, var(--vsc-accent-blue)) 85%, white 15%);\n    }\n\n    .registry-entry.kind-value .badge,\n    .registry-entry.kind-value .icon {\n      color: color-mix(in srgb, var(--vsc-foreground, #000) 75%, var(--vsc-accent-blue, #06c) 25%);\n    }\n\n    .registry-entry-root {\n      min-height: 24px;\n      height: 24px;\n      padding: 0 var(--vsc-padding-md);\n      border: 0;\n      border-radius: 0;\n      background: transparent;\n    }\n\n    .registry-entry-root .icon {\n      width: 18px;\n      height: 18px;\n      font-size: 10px;\n    }\n\n    .root-data {\n      font-size: 10px;\n    }\n\n    .registry-entry.selected .badge,\n    .registry-entry.selected .meta,\n    .registry-entry.selected .kind,\n    .registry-entry.selected .type,\n    .registry-entry.selected .data {\n      color: inherit;\n      opacity: .88;\n    }\n\n    .virtual-spacer {\n      width: 1px;\n      pointer-events: none;\n    }\n\n    .empty {\n      min-height: 120px;\n      display: flex;\n      flex-direction: column;\n      align-items: center;\n      justify-content: center;\n      gap: 4px;\n      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 11px;\n    }\n\n    .background-drop-target,\n    .drop-target {\n      outline: 1px dashed var(--vsc-focus-border, var(--vsc-accent-blue));\n      outline-offset: -1px;\n    }\n\n    .drop-invalid {\n      opacity: .55;\n    }\n\n    /* visible pane column gutters */\n    .registry-details-header,\n    .registry-entry {\n      column-gap: var(--pane-column-gutter, 12px);\n    }\n\n    .registry-details-header > span:not(:first-child),\n    .registry-entry > span:not(:first-child) {\n      min-width: 0;\n      padding-inline: 2px;\n    }\n\n    /* Resizer sits in the middle of the gutter rather than on top of text. */\n    .registry-details-header .resizer {\n      right: calc(var(--pane-column-gutter, 12px) / -2 - 1px);\n    }\n  "] }]
        }], propDecorators: { viewport: [{
                type: ViewChild,
                args: ['viewport', { static: true }]
            }], paneId: [{
                type: Input
            }], path: [{
                type: Input
            }], viewMode: [{
                type: Input
            }], entries: [{
                type: Input
            }], selectedPaths: [{
                type: Input
            }], cursorPath: [{
                type: Input
            }], visibleEntryCount: [{
                type: Input
            }], renderedStart: [{
                type: Input
            }], virtualEnabled: [{
                type: Input
            }], beforePx: [{
                type: Input
            }], afterPx: [{
                type: Input
            }], loading: [{
                type: Input
            }], filterText: [{
                type: Input
            }], dragOverPath: [{
                type: Input
            }], dropIntent: [{
                type: Input
            }], backgroundDropActive: [{
                type: Input
            }], scrolled: [{
                type: Output
            }], backgroundClick: [{
                type: Output
            }], backgroundContextMenu: [{
                type: Output
            }], backgroundDragOver: [{
                type: Output
            }], backgroundDragLeave: [{
                type: Output
            }], backgroundDrop: [{
                type: Output
            }], entrySelect: [{
                type: Output
            }], entryOpen: [{
                type: Output
            }], entryContextMenu: [{
                type: Output
            }], entryDragStart: [{
                type: Output
            }], entryDragEnd: [{
                type: Output
            }], entryDragOver: [{
                type: Output
            }], entryDragLeave: [{
                type: Output
            }], entryDrop: [{
                type: Output
            }] } });
//# sourceMappingURL=registry-pane.component.js.map