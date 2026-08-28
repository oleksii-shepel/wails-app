import {
  AfterViewInit,
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnChanges,
  OnDestroy,
  OnInit,
  Output,
  SimpleChanges,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import {
  formatPathForDisplay,
  formatSearchAliasForEdit,
  normalizeAddressBarPathInput,
  normalizePath,
} from './address-bar.utils';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { PanelSourceRegistryService } from '../services/panel-source-registry.service';
import { FileType, type FileInfo } from '../shared/protocol';
import {
  PathHistoryService,
  type PathSuggestion,
} from '../services/path-history.service';

interface BreadcrumbSegment {
  label: string;
  path: string;
  root: boolean;
  archiveBoundary: boolean;
}

const SOURCE_ROOT_SUGGESTIONS = [
  { path: 'bin://', displayName: 'Recycle Bin' },
  { path: 'autorun://', displayName: 'Autorun' },
  { path: 'environment://', displayName: 'Environment' },
  { path: 'network://', displayName: 'Network' },
  { path: 'registry://', displayName: 'Registry' },
] as const;

const COMMAND = {
  accept: 'accept',
  cancel: 'cancel',
  next: 'nextSuggestion',
  previous: 'previousSuggestion',
  complete: 'completeSuggestions',
} as const;

function editorBindings(paneId: string, contextId: string): readonly InputBindingDefinition[] {
  const id = `reassembly.address.${paneId}`;
  return [
    {
      id: `${id}.accept.enter`,
      commandId: `${id}.${COMMAND.accept}`,
      contextId,
      source: 'default',
      description: 'Open location',
      category: 'Navigation',
      when: 'typing',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'enter' }] } },
    },
    {
      id: `${id}.cancel.escape`,
      commandId: `${id}.${COMMAND.cancel}`,
      contextId,
      source: 'default',
      description: 'Cancel location edit',
      category: 'Navigation',
      when: 'typing',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'escape' }] } },
    },
    {
      id: `${id}.next.arrowDown`,
      commandId: `${id}.${COMMAND.next}`,
      contextId,
      source: 'default',
      description: 'Next location suggestion',
      category: 'Navigation',
      when: 'typing',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowdown' }] } },
    },
    {
      id: `${id}.previous.arrowUp`,
      commandId: `${id}.${COMMAND.previous}`,
      contextId,
      source: 'default',
      description: 'Previous location suggestion',
      category: 'Navigation',
      when: 'typing',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowup' }] } },
    },
    {
      id: `${id}.complete.ctrlSpace`,
      commandId: `${id}.${COMMAND.complete}`,
      contextId,
      source: 'default',
      description: 'Show location suggestions',
      category: 'Navigation',
      when: 'typing',
      trigger: {
        device: 'keyboard',
        chord: { steps: [{ key: 'space', modifiers: ['ctrl'] }] },
      },
    },
  ];
}

import { fuzzyPathScore } from '../services/navigation-intelligence';

@Component({
  selector: 'app-address-bar',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="address-shell">
      <div
        *ngIf="!editing"
        class="display-path breadcrumb-path"
        title="Click a segment to navigate · click empty space to edit"
        (click)="onDisplayPathClick($event)">
        <button
          type="button"
          class="location-icon breadcrumb-home"
          title="Edit location (Ctrl+L)"
          (click)="beginEdit(); $event.stopPropagation()"
          aria-label="Edit location">⌂</button>

        <div
          #breadcrumbs
          class="breadcrumbs"
          (wheel)="onBreadcrumbWheel($event)">
          <ng-container *ngFor="let segment of breadcrumbSegments; let last = last">
            <button
              type="button"
              class="breadcrumb"
              [class.current]="last"
              [class.archive-boundary]="segment.archiveBoundary"
              [title]="segment.path"
              (click)="navigateToSegment(segment); $event.stopPropagation()">
              {{ segment.label }}
            </button>
            <span class="separator" *ngIf="!last">›</span>
          </ng-container>
        </div>

        <button
          type="button"
          class="edit-path"
          title="Edit location (Ctrl+L)"
          aria-label="Edit location"
          (click)="beginEdit(); $event.stopPropagation()">✎</button>
      </div>

      <div class="editor" *ngIf="editing">
        <span class="location-icon" aria-hidden="true">⌂</span>
        <input
          #editor
          class="location-input"
          [(ngModel)]="value"
          (ngModelChange)="onValueChange($event)"
          (focus)="activateEditorContext()"
          (blur)="onBlur()"
          autocomplete="off"
          spellcheck="false"
          aria-label="Location" />

        <button
          type="button"
          class="cancel"
          title="Cancel (Escape)"
          (mousedown)="$event.preventDefault()"
          (click)="cancelEdit()">
          ×
        </button>
      </div>

      <div
        class="suggestions"
        *ngIf="editing && suggestions.length > 0">
        <button
          *ngFor="let suggestion of suggestions; let i = index"
          type="button"
          class="suggestion"
          [class.active]="i === suggestionIndex"
          (mousedown)="$event.preventDefault()"
          (click)="chooseSuggestion(i)">
          <span class="suggestion-name">{{ suggestion.displayName }}</span>
          <span class="suggestion-path">{{ suggestion.path }}</span>
          <span class="suggestion-kind">{{ suggestion.matchType }}</span>
        </button>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      min-width: 0;
      position: relative;
    }

    .address-shell {
      position: relative;
      min-width: 0;
    }

    .display-path,
    .editor {
      box-sizing: border-box;
      display: grid;
      grid-template-columns: 24px minmax(0, 1fr) 24px;
      align-items: center;
      width: 100%;
      height: 30px;
      padding: 0 7px;
      border: 1px solid var(--vsc-border, var(--vsc-control-background));
      border-radius: 4px;
      background: var(--vsc-input-background, var(--vsc-popover-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }

    .display-path {
      cursor: text;
      text-align: left;
      font: inherit;
    }

    .display-path:hover {
      border-color: color-mix(
        in srgb,
        var(--vsc-border, var(--vsc-control-background)) 65%,
        var(--vsc-foreground, var(--vsc-foreground))
      );
    }

    .editor {
      grid-template-columns: 24px minmax(0, 1fr) 24px;
      border-color: var(--vsc-focus-border, var(--vsc-accent-blue));
    }

    .location-icon {
      opacity: .7;
      text-align: center;
      user-select: none;
    }

    button.location-icon,
    .breadcrumb {
      border: 0;
      background: transparent;
      color: inherit;
      font: inherit;
      cursor: pointer;
    }

    .breadcrumb-home {
      width: 22px;
      height: 22px;
      padding: 0;
      border-radius: 3px;
    }

    .edit-path {
      width: 22px;
      height: 22px;
      padding: 0;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      cursor: pointer;
      font: 12px/22px var(--vsc-font-family, system-ui, sans-serif);
      text-align: center;
      flex: 0 0 auto;
    }

    .edit-path:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }

    .breadcrumb-home:hover,
    .breadcrumb:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .breadcrumbs {
      min-width: 0;
      display: flex;
      align-items: center;
      gap: 2px;
      overflow-x: auto;
      overflow-y: hidden;
      white-space: nowrap;
      scrollbar-width: none;
      -ms-overflow-style: none;
      overscroll-behavior-x: contain;
    }

    .breadcrumbs::-webkit-scrollbar {
      display: none;
      width: 0;
      height: 0;
    }

    .breadcrumb {
      flex: 0 0 auto;
      min-width: 0;
      max-width: 220px;
      padding: 3px 5px;
      border-radius: 3px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 11px;
    }

    .breadcrumb.current {
      color: var(--vsc-foreground-bright, var(--vsc-on-accent));
    }

    .breadcrumb.archive-boundary {
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
    }

    .separator {
      flex: 0 0 auto;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      user-select: none;
    }


    .display-value,
    .location-input {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 12px;
    }

    .location-input {
      width: 100%;
      height: 26px;
      padding: 0;
      border: 0;
      outline: 0;
      background: transparent;
      color: inherit;
    }

    .cancel {
      width: 22px;
      height: 22px;
      padding: 0;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font-size: 16px;
      line-height: 20px;
    }

    .cancel:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .breadcrumb-home:focus-visible,
    .breadcrumb:focus-visible,
    .edit-path:focus-visible,
    .cancel:focus-visible,
    .location-input:focus-visible {
      outline: 1px solid var(--vsc-focus-border, var(--vsc-accent-blue));
      outline-offset: 1px;
    }

    .suggestions {
      position: absolute;
      z-index: 200;
      top: calc(100% + 3px);
      left: 0;
      right: 0;
      overflow: auto;
      max-height: min(320px, 50vh);
      padding: 4px;
      border: 1px solid var(--vsc-border, var(--vsc-control-background));
      border-radius: 5px;
      background: var(--vsc-menu-background, var(--vsc-panel-background));
      box-shadow: 0 8px 24px var(--vsc-scrim);
    }

    .suggestion {
      box-sizing: border-box;
      display: grid;
      grid-template-columns: minmax(100px, .7fr) minmax(0, 1.3fr) 58px;
      gap: 10px;
      align-items: center;
      width: 100%;
      min-height: 30px;
      padding: 4px 7px;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      text-align: left;
      cursor: pointer;
      font: inherit;
    }

    .suggestion:hover,
    .suggestion.active {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .suggestion-name,
    .suggestion-path {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .suggestion-path {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 11px;
    }

    .suggestion-kind {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      text-align: right;
      font-size: 10px;
      text-transform: uppercase;
    }
  `],
})
export class AddressBarComponent implements OnInit, OnChanges, AfterViewInit, OnDestroy {
  @ViewChild('editor') editor?: ElementRef<HTMLInputElement>;
  @ViewChild('breadcrumbs') breadcrumbs?: ElementRef<HTMLDivElement>;

  @Input() path = '';
  @Input() paneId: 'left' | 'right' = 'left';
  @Output() navigate = new EventEmitter<string>();

  editing = false;
  value = '';
  suggestions: PathSuggestion[] = [];
  suggestionIndex = -1;

  private suggestionGeneration = 0;
  private blurTimer: ReturnType<typeof setTimeout> | null = null;
  private bindings: readonly InputBindingDefinition[] = [];

  private get contextId(): string {
    return `reassembly.address.${this.paneId}`;
  }

  private get ownerId(): string {
    return `reassembly.address.${this.paneId}`;
  }

  private commandId(name: string): string {
    return `${this.ownerId}.${name}`;
  }

  constructor(
    private readonly input: InputEngineService,
    private readonly history: PathHistoryService,
    private readonly panelSources: PanelSourceRegistryService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get displayPath(): string {
    return formatPathForDisplay(this.path || '/');
  }

  get breadcrumbSegments(): BreadcrumbSegment[] {
    const raw = this.displayPath || '/';
    const normalized = raw.replace(/\\/g, '/');

    const scheme = /^([a-z][a-z0-9+.-]*):\/\/(.*)$/i.exec(normalized);
    if (scheme) {
      const schemeName = scheme[1];
      const prefix = `${schemeName}://`;
      const parts = scheme[2].split('/').filter(Boolean);
      const segments: BreadcrumbSegment[] = [{
        // Show the semantic scheme root, not a folder-looking name.
        // Preserve the scheme spelling supplied by the location.
        label: `${schemeName}:`,
        path: prefix,
        root: true,
        archiveBoundary: false,
      }];

      let current = prefix;
      for (const part of parts) {
        current = `${current}${current.endsWith('/') ? '' : '/'}${part}`;
        segments.push({
          // Presentation is case-preserving. Never case-fold breadcrumb labels.
          label: part,
          path: current,
          root: false,
          archiveBoundary: this.isArchiveBreadcrumb(part),
        });
      }
      return segments;
    }

    const drive = /^([a-z]:)(?:\/(.*))?$/i.exec(normalized);
    if (drive) {
      const segments: BreadcrumbSegment[] = [{
        label: drive[1],
        path: `${drive[1]}/`,
        root: true,
        archiveBoundary: false,
      }];
      let current = `${drive[1]}/`;
      for (const part of (drive[2] ?? '').split('/').filter(Boolean)) {
        current = `${current.replace(/\/$/, '')}/${part}`;
        segments.push({
          label: part,
          path: current,
          root: false,
          archiveBoundary: this.isArchiveBreadcrumb(part),
        });
      }
      return segments;
    }

    const parts = normalized.split('/').filter(Boolean);
    const segments: BreadcrumbSegment[] = [{
      label: '/',
      path: '/',
      root: true,
      archiveBoundary: false,
    }];
    let current = '';
    for (const part of parts) {
      current += `/${part}`;
      segments.push({
          label: part,
          path: current,
          root: false,
          archiveBoundary: this.isArchiveBreadcrumb(part),
        });
    }
    return segments;
  }

  private isArchiveBreadcrumb(label: string): boolean {
    return /\.(?:zip|7z|rar|tar|tgz|gz|bz2|xz|jar|war|ear|apk|cbz|cbr)$/i.test(
      label,
    );
  }

  onDisplayPathClick(event: MouseEvent): void {
    if (this.editing) return;

    const target = event.target as HTMLElement | null;
    if (target?.closest('button, input, a, [role="button"]')) return;

    this.beginEdit();
  }

  navigateToSegment(segment: BreadcrumbSegment): void {
    if (!segment.path || segment.path === this.path) return;
    this.navigate.emit(segment.path);
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['path'] && !this.editing) {
      this.scheduleBreadcrumbScrollToEnd();
    }
  }

  ngAfterViewInit(): void {
    this.scheduleBreadcrumbScrollToEnd();
  }

  onBreadcrumbWheel(event: WheelEvent): void {
    const el = this.breadcrumbs?.nativeElement;
    if (!el || el.scrollWidth <= el.clientWidth) return;

    const delta =
      Math.abs(event.deltaX) > Math.abs(event.deltaY)
        ? event.deltaX
        : event.deltaY;

    if (!delta) return;
    el.scrollLeft += delta;
    event.preventDefault();
  }

  private scheduleBreadcrumbScrollToEnd(): void {
    queueMicrotask(() => {
      const el = this.breadcrumbs?.nativeElement;
      if (!el || this.editing) return;
      el.scrollLeft = el.scrollWidth;
    });
  }

  ngOnInit(): void {
    this.input.registerContext({
      id: this.contextId,
      kind: 'overlay',
      priority: 250,
      parentId: 'reassembly.manager',
      description: `Reassembly location editor (${this.paneId})`,
    });

    this.input.registerCommandDescriptor({
      id: this.commandId(COMMAND.accept),
      title: 'Open Location',
      category: 'Navigation',
      execute: () => this.acceptEdit(),
    }, this.ownerId);

    this.input.registerCommandDescriptor({
      id: this.commandId(COMMAND.cancel),
      title: 'Cancel Location Edit',
      category: 'Navigation',
      execute: () => this.cancelEdit(),
    }, this.ownerId);

    this.input.registerCommandDescriptor({
      id: this.commandId(COMMAND.next),
      title: 'Next Location Suggestion',
      category: 'Navigation',
      execute: () => this.moveSuggestion(1),
    }, this.ownerId);

    this.input.registerCommandDescriptor({
      id: this.commandId(COMMAND.previous),
      title: 'Previous Location Suggestion',
      category: 'Navigation',
      execute: () => this.moveSuggestion(-1),
    }, this.ownerId);

    this.input.registerCommandDescriptor({
      id: this.commandId(COMMAND.complete),
      title: 'Show Location Suggestions',
      category: 'Navigation',
      execute: () => this.forceSourceSuggestions(),
    }, this.ownerId);

    this.bindings = editorBindings(this.paneId, this.contextId);
    for (const binding of this.bindings) {
      this.input.registerBinding(binding);
    }
  }

  ngOnDestroy(): void {
    if (this.blurTimer) {
      clearTimeout(this.blurTimer);
      this.blurTimer = null;
    }

    this.input.deactivateContext(this.contextId);

    for (const binding of this.bindings) {
      this.input.unregisterBinding(binding.id);
    }

    for (const commandName of Object.values(COMMAND)) {
      this.input.unregisterCommandDescriptor(this.commandId(commandName), this.ownerId);
    }
  }

  beginEdit(): void {
    if (this.editing) {
      this.focusAndSelect();
      return;
    }

    this.editing = true;
    this.value = formatSearchAliasForEdit(this.path) ?? formatPathForDisplay(this.path || '/');
    this.suggestions = [];
    this.suggestionIndex = -1;
    this.activateEditorContext();
    this.cdr.detectChanges();
    this.focusAndSelect();

    queueMicrotask(() => this.focusAndSelect());
    void this.refreshSuggestions();
  }

  activateEditorContext(): void {
    if (!this.editing) return;
    this.input.activateContext(this.contextId);
  }

  onBlur(): void {
    if (this.blurTimer) clearTimeout(this.blurTimer);
    this.blurTimer = setTimeout(() => {
      this.blurTimer = null;
      if (this.editing) {
        this.cancelEdit();
      }
    }, 0);
  }

  onValueChange(value: string): void {
    this.value = value;
    this.suggestionIndex = -1;
    void this.refreshSuggestions();
  }

  chooseSuggestion(index: number): void {
    const suggestion = this.suggestions[index];
    if (!suggestion) return;
    this.value = formatPathForDisplay(suggestion.path);
    this.acceptEdit(suggestion.path);
  }

  cancelEdit(): void {
    if (!this.editing) return;

    if (this.blurTimer) {
      clearTimeout(this.blurTimer);
      this.blurTimer = null;
    }

    this.editing = false;
    this.value = '';
    this.suggestions = [];
    this.suggestionIndex = -1;
    this.input.deactivateContext(this.contextId);
    this.cdr.markForCheck();
  }

  private acceptEdit(explicitPath?: string): void {
    const candidate = explicitPath ?? (
      this.suggestionIndex >= 0
        ? this.suggestions[this.suggestionIndex]?.path
        : undefined
    ) ?? this.value;

    const resolvedCandidate = this.resolveImplicitAddressRoot(candidate) ?? candidate;
    const normalized = normalizeAddressBarPathInput(resolvedCandidate, this.path);
    if (!normalized) return;

    this.cancelEdit();
    this.navigate.emit(normalized);
  }

  private async moveSuggestion(delta: -1 | 1): Promise<void> {
    if (this.suggestions.length === 0) {
      await this.forceSourceSuggestions();
    }
    if (this.suggestions.length === 0) return;

    if (this.suggestionIndex < 0) {
      this.suggestionIndex = delta > 0 ? 0 : this.suggestions.length - 1;
    } else {
      this.suggestionIndex = (
        this.suggestionIndex + delta + this.suggestions.length
      ) % this.suggestions.length;
    }

    const suggestion = this.suggestions[this.suggestionIndex];
    if (suggestion) {
      this.value = formatPathForDisplay(suggestion.path);
    }

    this.cdr.markForCheck();
  }

  private splitSourceAutocompleteContext(value: string): { dirPath: string; prefix: string } | null {
    const raw = (value || '').trim().replace(/\\/g, '/');
    if (!raw) return null;

    const resolvedRaw = this.resolveImplicitAddressRoot(raw) ?? raw;
    const normalized = normalizeAddressBarPathInput(resolvedRaw, this.path);
    if (!normalized) return null;

    // The editor displays Windows roots as C:/ while the source boundary uses
    // filesystem://c/. Treat the root itself as the directory context.
    if (/^[a-z]:\/$/i.test(raw)) {
      const root = normalizePath(normalized);
      return {
        dirPath: root.endsWith('/') ? root : `${root}/`,
        prefix: '',
      };
    }

    // Scheme roots are complete directory contexts too.
    if (/^[a-z][a-z0-9+.-]*:\/\/$/i.test(raw)) {
      return { dirPath: raw, prefix: '' };
    }

    // A trailing slash means "list this directory".
    if (raw.endsWith('/')) {
      return { dirPath: normalized, prefix: '' };
    }

    const normalizedSlashes = normalized.replace(/\\/g, '/');
    const slash = normalizedSlashes.lastIndexOf('/');
    if (slash < 0) {
      return { dirPath: this.path || '/', prefix: raw };
    }

    return {
      dirPath: normalizedSlashes.slice(0, slash + 1),
      prefix: normalizedSlashes.slice(slash + 1),
    };
  }

  private resolveImplicitAddressRoot(value: string): string | null {
    const raw = (value || '').trim();
    if (!raw) return null;

    const normalized = raw
      .replace(/\\/g, '/')
      .replace(/:(?:\/\/?)?$/, '')
      .trim()
      .toLowerCase();

    if (!normalized || normalized.includes('/')) {
      return null;
    }

    if (/^[a-z]$/i.test(normalized)) {
      return `${normalized.toUpperCase()}:`;
    }

    for (const root of SOURCE_ROOT_SUGGESTIONS) {
      const resolved = this.panelSources.resolve({
        path: root.path,
        entries: [],
        inArchive: false,
      });
      if (resolved.source.kind === 'filesystem') continue;

      const schemeName = root.path.slice(0, root.path.indexOf(':')).toLowerCase();
      const displayName = root.displayName.toLowerCase();
      if (normalized === schemeName || normalized === displayName) {
        return root.path;
      }
    }

    return null;
  }

  private async forceSourceSuggestions(): Promise<void> {
    if (!this.editing) return;

    const generation = ++this.suggestionGeneration;
    const split = this.splitSourceAutocompleteContext(this.value);
    if (!split) {
      await this.refreshSuggestions();
      return;
    }

    try {
      const context = {
        path: split.dirPath,
        entries: [] as FileInfo[],
        inArchive: false,
      };
      const resolved = this.panelSources.resolve(context);
      const listing = await resolved.source.list(context, false);

      if (generation !== this.suggestionGeneration || !this.editing) return;

      const prefix = split.prefix.toLowerCase();
      const seen = new Set<string>();
      const suggestions: PathSuggestion[] = [];

      for (const entry of listing.entries) {
        if (entry.type !== FileType.DIRECTORY) continue;
        if (prefix && !entry.name.toLowerCase().startsWith(prefix)) continue;
        if (seen.has(entry.path)) continue;
        seen.add(entry.path);

        suggestions.push({
          path: entry.path,
          displayName: entry.name,
          matchType: 'directory',
          score: 100,
          exists: true,
        });

        // Keep the live dropdown bounded even when a source contains thousands
        // of records. Source-specific optimized completion can replace list()
        // later without changing the address-bar command contract.
        if (suggestions.length >= 100) break;
      }

      suggestions.sort((left, right) =>
        left.displayName.localeCompare(right.displayName),
      );

      this.suggestions = suggestions;
      this.suggestionIndex = suggestions.length > 0 ? 0 : -1;
      this.cdr.markForCheck();
    } catch {
      if (generation !== this.suggestionGeneration || !this.editing) return;
      this.suggestions = [];
      this.suggestionIndex = -1;
      this.cdr.markForCheck();
    }
  }

  private async refreshSuggestions(): Promise<void> {
    const generation = ++this.suggestionGeneration;

    try {
      const historySuggestions = await this.history.getSuggestions(this.value, this.path);
      if (generation !== this.suggestionGeneration || !this.editing) return;

      const query = this.value.trim();
      const sourceSuggestions = this.buildSourceRootSuggestions(query);
      const seen = new Set<string>();
      const suggestions = [...sourceSuggestions, ...historySuggestions].filter(suggestion => {
        const key = suggestion.path.toLowerCase();
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      });

      this.suggestions = suggestions.sort((left, right) => {
        const leftScore = fuzzyPathScore(query, left.displayName, left.path);
        const rightScore = fuzzyPathScore(query, right.displayName, right.path);
        return rightScore - leftScore || right.score - left.score;
      });
      this.suggestionIndex = -1;
      this.cdr.markForCheck();
    } catch {
      if (generation !== this.suggestionGeneration || !this.editing) return;

      this.suggestions = [];
      this.suggestionIndex = -1;
      this.cdr.markForCheck();
    }
  }

  private buildSourceRootSuggestions(query: string): PathSuggestion[] {
    const normalizedQuery = query.trim().toLowerCase();
    const results: PathSuggestion[] = [];

    for (const root of SOURCE_ROOT_SUGGESTIONS) {
      const resolved = this.panelSources.resolve({
        path: root.path,
        entries: [],
        inArchive: false,
      });

      // Optional/plugin roots (for example Registry) only appear when a real
      // PanelSource owns them. bin:// is built in and therefore always resolves
      // to the dedicated BinPanelSource rather than the filesystem fallback.
      if (resolved.source.kind === 'filesystem') continue;

      const schemeName = root.path.slice(0, root.path.indexOf(':'));
      const matches = !normalizedQuery ||
        root.path.toLowerCase().startsWith(normalizedQuery) ||
        schemeName.startsWith(normalizedQuery) ||
        root.displayName.toLowerCase().includes(normalizedQuery);
      if (!matches) continue;

      results.push({
        path: root.path,
        displayName: root.displayName,
        matchType: 'directory',
        score: 140,
        exists: true,
      });
    }

    return results;
  }

  private focusAndSelect(): void {
    const input = this.editor?.nativeElement;
    if (!input) return;

    input.focus();
    input.select();
    this.scrollEditorToEnd(input);

    // Angular/ngModel and browser selection painting may update the input's
    // horizontal viewport after select(). Re-apply the tail position once the
    // DOM has settled so long paths always open showing their rightmost part.
    requestAnimationFrame(() => {
      if (this.editor?.nativeElement === input && this.editing) {
        this.scrollEditorToEnd(input);
      }
    });
  }

  private scrollEditorToEnd(input: HTMLInputElement): void {
    input.scrollLeft = input.scrollWidth;
  }
}
