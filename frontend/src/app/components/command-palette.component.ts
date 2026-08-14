import { Component, OnInit, OnDestroy, ViewChild, ElementRef, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { InputBindingDefinition } from '../services/input-system.types';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { CommandPaletteHistoryService } from '../services/command-palette-history.service';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';
import { NO_ACTIONS, type ActionAvailability } from '../services/action-availability';
import { BC } from './file-browser-command-registry';
import { VscodeScrollbarDirective } from '../directives/vscode-scrollbar.directive';

interface PaletteItem {
  commandId: string;
  description: string;
  category: string;
  binding: InputBindingDefinition | null;
  enabled: boolean;
}

interface PaletteGroup {
  category: string;
  items: PaletteItem[];
}

@Component({
  selector: 'app-command-palette',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective, VscodeScrollbarDirective],
  template: `
    <div class="cp-overlay" *ngIf="isVisible" (click)="hide()">
      <div
        #dialogPanel
        class="cp-panel"
        [appTrapFocus]="true"
        role="dialog"
        aria-modal="true"
        aria-label="Command Palette"
        tabindex="-1"
        (click)="$event.stopPropagation()"
      >
        <div class="cp-search-row">
          <span class="cp-search-icon">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
              <path d="M11.742 10.344a6.5 6.5 0 1 0-1.397 1.398h-.001c.03.04.062.078.098.115l3.85 3.85a1 1 0 0 0 1.415-1.414l-3.85-3.85a1.007 1.007 0 0 0-.115-.1zM12 6.5a5.5 5.5 0 1 1-11 0 5.5 5.5 0 0 1 11 0z"/>
            </svg>
          </span>
          <input
            #searchInput
            type="text"
            [(ngModel)]="searchQuery"
            (ngModelChange)="onSearchChange()"
            (keydown)="onSearchKeydown($event)"
            placeholder="Type a command..."
            class="cp-search-input"
            data-dialog-initial-focus
            autofocus
          />
          <span class="cp-shortcut-hint" *ngIf="!searchQuery">Ctrl+Shift+P</span>
        </div>

        <div class="cp-results" appVscodeScrollbar>
          <div *ngIf="filteredGroups.length === 0" class="cp-no-results">
            No matching commands
          </div>

          <ng-container *ngFor="let group of filteredGroups">
            <div class="cp-group-header">{{ group.category }}</div>
            <div
              *ngFor="let item of group.items; let i = index"
              class="cp-item"
              [class.selected]="isSelected(item)"
               [class.disabled]="!item.enabled"
              (click)="executeItem(item)"
              (mouseenter)="selectItem(item)"
            >
              <span class="cp-item-icon">
                <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
                  <path d="M8 4a.5.5 0 0 1 .5.5v3h3a.5.5 0 0 1 0 1h-3v3a.5.5 0 0 1-1 0v-3h-3a.5.5 0 0 1 0-1h3v-3A.5.5 0 0 1 8 4z"/>
                </svg>
              </span>
              <span class="cp-item-label">{{ item.description }}</span>
              <span class="cp-item-keys" *ngIf="item.binding">
                {{ formatBinding(item.binding) }}
              </span>
            </div>
          </ng-container>
        </div>

        <div class="cp-footer">
          <span>{{ totalItems }} commands</span>
          <span class="cp-footer-hint">
            <kbd>↑</kbd><kbd>↓</kbd> navigate · <kbd>Enter</kbd> run · <kbd>Esc</kbd> close
          </span>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .cp-overlay {
      position: fixed;
      top: 0; left: 0; right: 0; bottom: 0;
      background: rgba(0, 0, 0, 0.35);
      display: flex;
      justify-content: center;
      align-items: flex-start;
      padding-top: 12vh;
      z-index: 10000;
      backdrop-filter: blur(2px);
      animation: cpFadeIn 0.15s ease;
    }
    @keyframes cpFadeIn { from { opacity: 0; } to { opacity: 1; } }

    .cp-panel {
      background: var(--vsc-editor-background);
      border: 1px solid var(--vsc-border-bright);
      border-radius: var(--vsc-border-radius);
      width: 600px;
      max-width: 90vw;
      max-height: 70vh;
      display: flex;
      flex-direction: column;
      box-shadow: var(--vsc-widget-shadow), 0 24px 56px rgba(0,0,0,0.45);
      animation: cpSlideDown 0.2s cubic-bezier(0.16,1,0.3,1);
      overflow: hidden;
    }
    @keyframes cpSlideDown {
      from { transform: translateY(-12px); opacity: 0; }
      to { transform: translateY(0); opacity: 1; }
    }

    .cp-search-row {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 10px 14px;
      border-bottom: 1px solid var(--vsc-border-subtle);
    }
    .cp-search-icon {
      color: var(--vsc-foreground-dim);
      flex-shrink: 0;
    }
    .cp-search-input {
      flex: 1;
      background: transparent;
      border: none;
      color: var(--vsc-input-foreground);
      font-size: var(--vsc-font-size);
      outline: none;
      padding: 2px 0;
    }
    .cp-search-input::placeholder { color: var(--vsc-input-placeholder); }
    .cp-shortcut-hint {
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      background: var(--vsc-input-background);
      padding: 2px 6px;
      border-radius: var(--vsc-border-radius-sm);
      border: 1px solid var(--vsc-border);
      white-space: nowrap;
    }

    .cp-results {
      flex: 1;
      overflow-y: auto;
      padding: 4px 0;
    }
    .cp-no-results {
      padding: 2rem;
      text-align: center;
      color: var(--vsc-foreground-dim);
      font-size: var(--vsc-font-size);
    }
    .cp-group-header {
      padding: 6px 14px 4px;
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--vsc-foreground-dim);
      background: var(--vsc-panel-background);
      border-bottom: 1px solid var(--vsc-border-subtle);
      position: sticky;
      top: 0;
    }
    .cp-item {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 7px 14px;
      cursor: pointer;
      transition: background 0.08s ease;
      border-left: 2px solid transparent;
    }
    .cp-item.disabled { opacity: 0.45; cursor: not-allowed; }
    .cp-item:hover:not(.disabled), .cp-item.selected:not(.disabled) {
      background: var(--vsc-list-hover-background);
      border-left-color: var(--vsc-accent-blue);
    }
    .cp-item-icon {
      color: var(--vsc-foreground-dim);
      flex-shrink: 0;
      opacity: 0.7;
    }
    .cp-item-label {
      flex: 1;
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .cp-item-keys {
      font-family: var(--vsc-font-family-mono);
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      background: var(--vsc-input-background);
      padding: 2px 6px;
      border-radius: var(--vsc-border-radius-sm);
      border: 1px solid var(--vsc-border);
      white-space: nowrap;
    }

    .cp-footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 6px 14px;
      border-top: 1px solid var(--vsc-border-subtle);
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      background: var(--vsc-panel-background);
    }
    .cp-footer-hint kbd {
      display: inline-block;
      padding: 1px 4px;
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-border);
      border-bottom-width: 2px;
      border-radius: var(--vsc-border-radius-sm);
      font-family: var(--vsc-font-family-mono);
      font-size: 10px;
      margin: 0 1px;
    }
  `]
})
export class CommandPaletteComponent implements OnInit, OnDestroy {
  @Input() actionAvailability: ActionAvailability = NO_ACTIONS;
  @ViewChild('searchInput') searchInput?: ElementRef<HTMLInputElement>;
  @ViewChild('dialogPanel') dialogPanel?: ElementRef<HTMLElement>;

  isVisible = false;
  searchQuery = '';
  groups: PaletteGroup[] = [];
  filteredGroups: PaletteGroup[] = [];
  selectedCommandId: string | null = null;
  private flatItems: PaletteItem[] = [];
  private releaseContext: (() => void) | null = null;
  private destroy$ = createSubject<void>();

  private readonly modNames: Record<string, string> = {
    ctrl: 'Ctrl', shift: 'Shift', alt: 'Alt', meta: 'Cmd',
  };
  private readonly keyNames: Record<string, string> = {
    ' ': 'Space', arrowup: '↑', arrowdown: '↓', arrowleft: '←', arrowright: '→',
    escape: 'Esc', enter: 'Enter', backspace: 'Backspace', delete: 'Del',
    tab: 'Tab', home: 'Home', end: 'End', pageup: 'PgUp', pagedown: 'PgDn',
    insert: 'Ins', space: 'Space',
  };

  constructor(
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
    private overlayFreeze: NativeOverlayFreezeService,
    private historyService: CommandPaletteHistoryService,
  ) {}

  ngOnInit(): void {
    this.loadCommands();
    this.registerInputCommands();

    // Refresh palette contents when active contexts change (for context-aware filtering)
    this.inputEngine.activeContexts$
      .pipe(takeUntil(this.destroy$))
      .subscribe(() => {
        if (this.isVisible) {
          this.loadCommands();
          this.onSearchChange();
        }
      });
  }

  ngOnDestroy(): void {
    this.overlayFreeze.release('dialog.commandPalette');
    this.releaseContext?.();
    this.releaseContext = null;
    this.unregisterInputCommands();
    this.destroy$.next();
    this.destroy$.complete();
  }

  show(): void {
    if (this.isVisible) return;
    this.isVisible = true;
    this.searchQuery = '';
    this.loadCommands();
    this.onSearchChange();
    this.releaseContext ??= this.inputContexts.acquire('dialog.commandPalette');
    this.overlayFreeze.acquire('dialog.commandPalette');
    setTimeout(() => {
      this.searchInput?.nativeElement.focus();
      this.searchInput?.nativeElement.select();
    }, 0);
  }

  hide(): void {
    if (!this.isVisible) return;
    this.isVisible = false;
    this.searchQuery = '';
    this.releaseContext?.();
    this.releaseContext = null;
    this.overlayFreeze.release('dialog.commandPalette');
  }

  private loadCommands(): void {
    // Build a lookup of commandId -> best keyboard binding for display
    const bindingByCommand = new Map<string, InputBindingDefinition>();
    for (const binding of this.inputEngine.getBindingSnapshot()) {
      if (binding.trigger.device !== 'keyboard') continue;
      const existing = bindingByCommand.get(binding.commandId);
      if (!existing || (existing.trigger.device === 'keyboard' && binding.trigger.chord.steps.length < existing.trigger.chord.steps.length)) {
        bindingByCommand.set(binding.commandId, binding);
      }
    }

    // Primary source: registered command definitions (supports plugin contributions)
    const defs = this.inputEngine.getCommandDefinitions();
    const activeContextIds = this.inputEngine.getActiveContextIds();
    const allItems = new Map<string, PaletteItem>();

    for (const def of defs) {
      // Context-aware filtering: only show if no contextIds specified or any context is active
      if (def.contextIds && def.contextIds.length > 0 && !def.contextIds.some(cid => activeContextIds.has(cid))) {
        continue;
      }
      const cat = def.category ?? 'General';
      allItems.set(def.id, {
        commandId: def.id,
        description: def.title,
        category: cat,
        binding: bindingByCommand.get(def.id) ?? null,
        enabled: this.isCommandAvailable(def.id),
      });
      bindingByCommand.delete(def.id);
    }

    // Fallback: any commands with bindings but no explicit definition
    for (const [commandId, binding] of bindingByCommand) {
      const cat = binding.category ?? 'General';
      allItems.set(commandId, {
        commandId,
        description: binding.description ?? commandId,
        category: cat,
        binding,
        enabled: this.isCommandAvailable(commandId),
      });
    }

    // Build groups
    const catMap = new Map<string, PaletteItem[]>();
    for (const item of allItems.values()) {
      if (!catMap.has(item.category)) catMap.set(item.category, []);
      catMap.get(item.category)!.push(item);
    }

    const categoryOrder = ['Navigation', 'Tab Management', 'File Operations', 'Search & Filter', 'Workspaces', 'Pane Control', 'Tools', 'General'];
    this.groups = [];
    this.flatItems = [];

    // Add Recent group first
    const history = this.historyService.getHistory();
    const recentItems: PaletteItem[] = [];
    for (const commandId of history) {
      const item = allItems.get(commandId);
      if (item) recentItems.push(item);
    }
    if (recentItems.length > 0) {
      this.groups.push({ category: 'Recent', items: recentItems });
      this.flatItems.push(...recentItems);
    }

    const pushGroup = (cat: string) => {
      const items = catMap.get(cat);
      if (items && items.length > 0) {
        items.sort((a, b) => a.description.localeCompare(b.description));
        this.groups.push({ category: cat, items });
        this.flatItems.push(...items);
        catMap.delete(cat);
      }
    };

    for (const cat of categoryOrder) pushGroup(cat);
    for (const [cat, items] of catMap) pushGroup(cat);

    this.totalItems = this.flatItems.length;
  }

  totalItems = 0;

  onSearchChange(): void {
    const query = this.searchQuery.toLowerCase().trim();
    if (!query) {
      this.filteredGroups = this.groups;
      this.selectedCommandId = this.flatItems.find(item => item.enabled)?.commandId ?? null;
      return;
    }

    const filteredFlat: PaletteItem[] = [];
    for (const item of this.flatItems) {
      const descMatch = item.description.toLowerCase().includes(query);
      const keyMatch = this.formatBinding(item.binding).toLowerCase().includes(query);
      const idMatch = item.commandId.toLowerCase().includes(query);
      if (descMatch || keyMatch || idMatch) {
        filteredFlat.push(item);
      }
    }

    const catMap = new Map<string, PaletteItem[]>();
    for (const item of filteredFlat) {
      if (!catMap.has(item.category)) catMap.set(item.category, []);
      catMap.get(item.category)!.push(item);
    }

    this.filteredGroups = [];
    for (const group of this.groups) {
      const items = catMap.get(group.category);
      if (items && items.length > 0) {
        this.filteredGroups.push({ category: group.category, items });
      }
    }

    this.selectedCommandId = filteredFlat.find(item => item.enabled)?.commandId ?? null;
  }

  onSearchKeydown(event: KeyboardEvent): void {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      this.moveSelection(1);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      this.moveSelection(-1);
    } else if (event.key === 'Enter') {
      event.preventDefault();
      const item = this.findSelectedItem();
      if (item) this.executeItem(item);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      this.hide();
    }
  }

  private moveSelection(delta: number): void {
    const flat = this.filteredGroups.flatMap(g => g.items);
    if (flat.length === 0) return;
    let idx = flat.findIndex(i => i.commandId === this.selectedCommandId);
    if (idx < 0) idx = delta > 0 ? -1 : 0;
    for (let i = 0; i < flat.length; i++) {
      idx = (idx + delta + flat.length) % flat.length;
      if (flat[idx].enabled) {
        this.selectedCommandId = flat[idx].commandId;
        return;
      }
    }
    this.selectedCommandId = null;
  }

  private findSelectedItem(): PaletteItem | null {
    return this.flatItems.find(i => i.commandId === this.selectedCommandId) ?? null;
  }

  isSelected(item: PaletteItem): boolean {
    return item.commandId === this.selectedCommandId;
  }

  selectItem(item: PaletteItem): void {
    if (item.enabled) this.selectedCommandId = item.commandId;
  }

  executeItem(item: PaletteItem): void {
    if (!item.enabled) return;
    this.hide();
    this.historyService.recordCommand(item.commandId);
    // Small delay so the modal closes before the command runs (prevents focus issues)
    setTimeout(() => {
      const ok = this.inputEngine.dispatchCommandById(item.commandId);
      if (!ok) {
        console.warn('Command palette: command not found or failed:', item.commandId);
      }
    }, 50);
  }

  private isCommandAvailable(commandId: string): boolean {
    switch (commandId) {
      case BC.copy: return this.actionAvailability.copySelected;
      case BC.cut: return this.actionAvailability.cutSelected;
      case BC.paste: return this.actionAvailability.paste;
      case BC.deleteSelected: return this.actionAvailability.deleteSelected;
      case BC.newFolder: return this.actionAvailability.createFolder;
      case BC.rename:
      case BC.batchRename: return this.actionAvailability.rename;
      case BC.copyToOtherPane: return this.actionAvailability.copyToOther;
      case BC.moveToOtherPane: return this.actionAvailability.moveToOther;
      default: return true;
    }
  }

  formatBinding(binding: InputBindingDefinition | null): string {
    if (!binding || binding.trigger.device !== 'keyboard') return '';
    return binding.trigger.chord.steps.map(step => {
      const mods = (step.modifiers ?? []).map(m => this.modNames[m] ?? m).join('+');
      const key = this.keyNames[step.key] ?? step.key.toUpperCase();
      return mods ? `${mods}+${key}` : key;
    }).join(' → ');
  }

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('dialog.commandPalette.close', () => this.hide());
    this.inputEngine.registerCommand('dialog.commandPalette.next', () => this.moveSelection(1));
    this.inputEngine.registerCommand('dialog.commandPalette.prev', () => this.moveSelection(-1));
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('dialog.commandPalette.close');
    this.inputEngine.unregisterCommand('dialog.commandPalette.next');
    this.inputEngine.unregisterCommand('dialog.commandPalette.prev');
  }
}