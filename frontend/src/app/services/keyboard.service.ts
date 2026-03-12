import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import { InputEngineService } from './input-engine.service';
import type { InputBindingDefinition, KeyStroke } from './input-system.types';

/**
 * Keyboard shortcut definition
 */
export interface ShortcutDefinition {
  id: string;
  key: string;
  modifiers?: ('ctrl' | 'shift' | 'alt' | 'meta')[];
  sequence?: KeyStroke[];
  action: () => void | Promise<void>;
  context?: 'global' | 'pane' | 'tab' | 'filter' | 'modal';
  category: 'navigation' | 'tabs' | 'files' | 'search' | 'workspace' | 'pane';
  description: string;
  enabled?: boolean;
  /**
   * When true the global listener calls preventDefault() but NOT stopPropagation(),
   * so Angular component (keydown) bindings at lower levels can still react.
   * Default: true (stop propagation). Set to false for address-bar / focus shortcuts
   * that need to be handled both globally AND by the focused component.
   */
  allowBubble?: boolean;
}

/**
 * Shortcut category for help display
 */
export interface ShortcutCategory {
  name: string;
  shortcuts: ShortcutDefinition[];
}

/**
 * Global keyboard shortcut service
 * Compatibility adapter over the shared input engine.
 */
@Injectable({
  providedIn: 'root'
})
export class KeyboardService {
  private shortcuts = new Map<string, ShortcutDefinition>();
  private shortcutBindings = new Map<string, string>();
  private enabledSubject = createBehaviorSubject<boolean>(true);
  public enabled$ = this.enabledSubject;

  private historyStack: string[] = [];
  private maxHistorySize = 50;

  constructor(private inputEngine: InputEngineService) {
    this.inputEngine.setKeyboardEnabled(true);
  }

  /**
   * Register a keyboard shortcut
   */
  register(shortcut: ShortcutDefinition): void {
    const normalizedKey = this.normalizeKey(shortcut.key);
    const normalizedSequence = this.normalizeShortcutSequence(shortcut, normalizedKey);
    const key = this.buildShortcutKey(normalizedSequence);
    const previousShortcut = this.shortcuts.get(key);

    if (previousShortcut) {
      console.warn(`Shortcut conflict: ${key} is already registered`);
      this.unregisterFromInputEngine(previousShortcut.id);
    }

    const normalizedShortcut: ShortcutDefinition = {
      ...shortcut,
      key: normalizedKey,
      sequence: normalizedSequence.length > 1 ? normalizedSequence : undefined,
      enabled: shortcut.enabled !== false,
    };

    this.shortcuts.set(key, normalizedShortcut);
    this.registerWithInputEngine(normalizedShortcut);
  }

  /**
   * Unregister a keyboard shortcut
   */
  unregister(id: string): void {
    for (const [key, shortcut] of this.shortcuts.entries()) {
      if (shortcut.id === id) {
        this.unregisterFromInputEngine(shortcut.id);
        this.shortcuts.delete(key);
        break;
      }
    }
  }

  /**
   * Build shortcut key string
   */
  private buildShortcutKey(sequence: KeyStroke[]): string {
    return sequence
      .map(step => {
        const parts: string[] = [];

        if (step.modifiers?.length) {
          parts.push(...[...step.modifiers].sort());
        }

        parts.push(step.key.toLowerCase());
        return parts.join('+');
      })
      .join(' ');
  }

  /**
   * Normalize key name
   */
  private normalizeKey(key: string): string {
    const keyMap: Record<string, string> = {
      ' ': 'space',
      'Insert': 'insert',
      'Escape': 'escape',
      'Enter': 'enter',
      'Tab': 'tab',
      'Backspace': 'backspace',
      'Delete': 'delete',
      'ArrowUp': 'arrowup',
      'ArrowDown': 'arrowdown',
      'ArrowLeft': 'arrowleft',
      'ArrowRight': 'arrowright',
      'Control': 'ctrl',
      'Meta': 'meta',
      'Shift': 'shift',
      'Alt': 'alt',
      'NumpadAdd': 'numpadadd',
      'NumpadSubtract': 'numpadsubtract',
      'NumpadMultiply': 'numpadmultiply',
    };

    return keyMap[key] || key.toLowerCase();
  }

  /**
   * Add shortcut to history
   */
  private addToHistory(shortcutId: string): void {
    this.historyStack.push(shortcutId);

    if (this.historyStack.length > this.maxHistorySize) {
      this.historyStack.shift();
    }
  }

  /**
   * Get all shortcuts grouped by category
   */
  getShortcutsByCategory(): ShortcutCategory[] {
    const categories = new Map<string, ShortcutDefinition[]>();

    for (const shortcut of this.shortcuts.values()) {
      const category = shortcut.category;
      if (!categories.has(category)) {
        categories.set(category, []);
      }
      categories.get(category)!.push(shortcut);
    }

    const categoryNames: Record<string, string> = {
      navigation: 'Navigation',
      tabs: 'Tab Management',
      files: 'File Operations',
      search: 'Search & Filter',
      workspace: 'Workspaces',
      pane: 'Pane Control',
    };

    return Array.from(categories.entries()).map(([key, shortcuts]) => ({
      name: categoryNames[key] || key,
      shortcuts: shortcuts.sort((a, b) => a.description.localeCompare(b.description)),
    }));
  }

  /**
   * Get all shortcuts
   */
  getAllShortcuts(): ShortcutDefinition[] {
    return Array.from(this.shortcuts.values());
  }

  /**
   * Enable/disable keyboard shortcuts globally
   */
  setEnabled(enabled: boolean): void {
    this.enabledSubject.next(enabled);
    this.inputEngine.setKeyboardEnabled(enabled);

    for (const shortcut of this.shortcuts.values()) {
      this.inputEngine.setBindingEnabled(this.getBindingId(shortcut.id), enabled && shortcut.enabled !== false);
    }
  }

  /**
   * Toggle a specific shortcut
   */
  toggleShortcut(id: string, enabled: boolean): void {
    for (const shortcut of this.shortcuts.values()) {
      if (shortcut.id === id) {
        shortcut.enabled = enabled;
        this.inputEngine.setBindingEnabled(this.getBindingId(id), this.enabledSubject.value && enabled);
        break;
      }
    }
  }

  /**
   * Format shortcut for display
   */
  formatShortcut(shortcut: ShortcutDefinition): string {
    const sequence = this.getShortcutSequence(shortcut);
    const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;

    return sequence.map(step => this.formatShortcutStep(step, isMac)).join(' ');
  }

  private formatShortcutStep(step: KeyStroke, isMac: boolean): string {
    const parts: string[] = [];

    if (step.modifiers) {
      for (const mod of step.modifiers) {
        switch (mod) {
          case 'ctrl':
            parts.push(isMac ? '⌃' : 'Ctrl');
            break;
          case 'meta':
            parts.push(isMac ? '⌘' : 'Win');
            break;
          case 'shift':
            parts.push(isMac ? '⇧' : 'Shift');
            break;
          case 'alt':
            parts.push(isMac ? '⌥' : 'Alt');
            break;
        }
      }
    }

    const keyName = this.formatKeyName(step.key);
    parts.push(keyName);

    return parts.join(isMac ? '' : '+');
  }

  /**
   * Format key name for display
   */
  private formatKeyName(key: string): string {
    const keyMap: Record<string, string> = {
      'space': '␣',
      'escape': 'Esc',
      'enter': '⏎',
      'tab': '⇥',
      'backspace': '⌫',
      'delete': 'Del',
      'arrowup': '↑',
      'arrowdown': '↓',
      'arrowleft': '←',
      'arrowright': '→',
    };

    return keyMap[key.toLowerCase()] || key.toUpperCase();
  }

  /**
   * Get recent shortcut history
   */
  getHistory(): string[] {
    return [...this.historyStack].reverse();
  }

  /**
   * Clear shortcut history
   */
  clearHistory(): void {
    this.historyStack = [];
  }

  /**
   * Check if a key combination is available
   */
  isAvailable(key: string, modifiers?: string[]): boolean {
    const normalizedModifiers = modifiers?.length
      ? modifiers.filter((modifier): modifier is 'ctrl' | 'shift' | 'alt' | 'meta' =>
          modifier === 'ctrl' || modifier === 'shift' || modifier === 'alt' || modifier === 'meta')
      : undefined;
    const shortcutKey = this.buildShortcutKey([{ key: this.normalizeKey(key), modifiers: normalizedModifiers }]);
    return !this.shortcuts.has(shortcutKey);
  }

  private registerWithInputEngine(shortcut: ShortcutDefinition): void {
    const commandId = this.getCommandId(shortcut.id);
    const bindingId = this.getBindingId(shortcut.id);

    this.unregisterFromInputEngine(shortcut.id);

    this.inputEngine.registerCommand(commandId, () => {
      const result = shortcut.action();
      if (result instanceof Promise) {
        return result.then(() => this.addToHistory(shortcut.id));
      }

      this.addToHistory(shortcut.id);
      return result;
    }, this.getCommandOwnerId(shortcut.id));

    const binding: InputBindingDefinition = {
      id: bindingId,
      commandId,
      contextId: 'global',
      source: 'default',
      description: shortcut.description,
      when: this.buildWhenClause(shortcut),
      preventDefault: true,
      stopPropagation: !shortcut.allowBubble,
      trigger: {
        device: 'keyboard',
        chord: {
          steps: this.getShortcutSequence(shortcut).map(step => ({
            key: step.key,
            modifiers: step.modifiers?.length ? [...step.modifiers].sort() : undefined,
          })),
        },
      },
    };

    this.inputEngine.registerBinding(binding);
    this.inputEngine.setBindingEnabled(bindingId, this.enabledSubject.value && shortcut.enabled !== false);
    this.shortcutBindings.set(shortcut.id, bindingId);
  }

  private unregisterFromInputEngine(shortcutId: string): void {
    const bindingId = this.shortcutBindings.get(shortcutId) ?? this.getBindingId(shortcutId);
    this.inputEngine.unregisterBinding(bindingId);
    this.inputEngine.unregisterCommand(this.getCommandId(shortcutId), this.getCommandOwnerId(shortcutId));
    this.shortcutBindings.delete(shortcutId);
  }

  private getBindingId(shortcutId: string): string {
    return `legacy.shortcut.binding.${shortcutId}`;
  }

  private getCommandId(shortcutId: string): string {
    return `legacy.shortcut.command.${shortcutId}`;
  }

  private getCommandOwnerId(shortcutId: string): string {
    return `legacy.shortcut.owner.${shortcutId}`;
  }

  private buildWhenClause(shortcut: ShortcutDefinition): string {
    switch (shortcut.context) {
      case 'modal':
        return 'modal';
      case 'filter':
        return 'filterInput';
      case 'pane':
        return '!typing && !quickAccess && activePane';
      case 'global':
      default:
        return '!typing && !quickAccess';
    }
  }

  private getShortcutSequence(shortcut: ShortcutDefinition): KeyStroke[] {
    if (shortcut.sequence?.length) {
      return shortcut.sequence;
    }

    return [{
      key: shortcut.key,
      modifiers: shortcut.modifiers?.length ? [...shortcut.modifiers].sort() : undefined,
    }];
  }

  private normalizeShortcutSequence(shortcut: ShortcutDefinition, normalizedKey: string): KeyStroke[] {
    const rawSequence = shortcut.sequence?.length
      ? shortcut.sequence
      : [{ key: normalizedKey, modifiers: shortcut.modifiers }];

    return rawSequence.map((step, index) => ({
      key: this.normalizeKey(index === 0 && !shortcut.sequence?.length ? normalizedKey : step.key),
      modifiers: step.modifiers?.length ? [...step.modifiers].sort() : undefined,
    }));
  }

  /**
   * Export shortcuts configuration
   */
  exportConfig(): any {
    const shortcuts: any[] = [];

    for (const shortcut of this.shortcuts.values()) {
      shortcuts.push({
        id: shortcut.id,
        key: shortcut.key,
        modifiers: shortcut.modifiers,
        category: shortcut.category,
        description: shortcut.description,
        enabled: shortcut.enabled,
      });
    }

    return {
      version: '1.0',
      shortcuts,
    };
  }

  /**
   * Import shortcuts configuration
   */
  importConfig(config: any): void {
    if (!config.shortcuts || !Array.isArray(config.shortcuts)) {
      console.error('Invalid shortcuts configuration');
      return;
    }

    for (const shortcutData of config.shortcuts) {
      const existing = Array.from(this.shortcuts.values())
        .find(s => s.id === shortcutData.id);

      if (existing) {
        existing.enabled = shortcutData.enabled !== false;
      }
    }
  }
}


