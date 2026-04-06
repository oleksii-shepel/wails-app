import { Component, OnInit, OnDestroy, ViewChild, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject } from '@epikodelabs/streamix';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { InputBindingDefinition } from '../services/input-system.types';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';
import { moveFocusWithin } from '../directives/focus-trap.utils';
import { VscodeScrollbarDirective } from '../directives/vscode-scrollbar.directive';

interface BindingCategory {
  name: string;
  bindings: InputBindingDefinition[];
}

@Component({
  selector: 'app-keyboard-help',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective, VscodeScrollbarDirective],
  templateUrl: './keyboard-help.component.html',
  styleUrls: ['./keyboard-help.component.scss']
})
export class KeyboardHelpComponent implements OnInit, OnDestroy {
  @ViewChild('dialogPanel') dialogPanel?: ElementRef<HTMLElement>;
  @ViewChild('searchInput') searchInput?: ElementRef<HTMLInputElement>;

  isVisible = false;
  categories: BindingCategory[] = [];
  searchText = '';
  filteredCategories: BindingCategory[] = [];

  private readonly modNames: Record<string, string> = {
    Control: 'Ctrl', Alt: 'Alt', Shift: 'Shift', Meta: 'Cmd',
  };
  private readonly keyNames: Record<string, string> = {
    ' ': 'Space', ArrowUp: '↑', ArrowDown: '↓', ArrowLeft: '←', ArrowRight: '→',
    Escape: 'Esc', Enter: 'Enter', Backspace: 'Backspace', Delete: 'Del',
    Tab: 'Tab', Home: 'Home', End: 'End', PageUp: 'PgUp', PageDown: 'PgDn',
    F1: 'F1', F2: 'F2', F3: 'F3', F4: 'F4', F5: 'F5', F6: 'F6',
    F7: 'F7', F8: 'F8', F9: 'F9', F10: 'F10', F11: 'F11', F12: 'F12',
  };

  private destroy$ = createSubject<void>();
  private releaseDialogContext: (() => void) | null = null;

  constructor(
    private confirmDialog: ConfirmDialogService,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
    private overlayFreeze: NativeOverlayFreezeService,
  ) {}

  ngOnInit(): void {
    this.registerInputCommands();
    this.loadShortcuts();
  }

  ngOnDestroy(): void {
    this.overlayFreeze.release('dialog.keyboardHelp');
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.unregisterInputCommands();
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadShortcuts(): void {
    const snapshot = this.inputEngine.getBindingSnapshot();
    const byCategory = new Map<string, InputBindingDefinition[]>();
    for (const b of snapshot) {
      if (!b.category || b.trigger.device !== 'keyboard') continue;
      if (!byCategory.has(b.category)) byCategory.set(b.category, []);
      byCategory.get(b.category)!.push(b);
    }
    this.categories = Array.from(byCategory.entries())
      .map(([name, bindings]) => ({ name, bindings }));
    this.filteredCategories = this.categories;
  }

  /**
   * Show help modal
   */
  show(): void {
    this.isVisible = true;
    this.releaseDialogContext ??= this.inputContexts.acquire('dialog.keyboardHelp');
    this.overlayFreeze.acquire('dialog.keyboardHelp');
    this.searchText = '';
    this.loadShortcuts();
  }

  /**
   * Hide help modal
   */
  hide(): void {
    this.isVisible = false;
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.overlayFreeze.release('dialog.keyboardHelp');
  }

  onSearchChange(): void {
    const search = this.searchText.toLowerCase().trim();
    if (!search) {
      this.filteredCategories = this.categories;
      return;
    }
    this.filteredCategories = this.categories
      .map(cat => ({
        name: cat.name,
        bindings: cat.bindings.filter(b => {
          const keyMatch = this.formatBinding(b).toLowerCase().includes(search);
          const descMatch = (b.description ?? '').toLowerCase().includes(search);
          return keyMatch || descMatch;
        }),
      }))
      .filter(cat => cat.bindings.length > 0);
  }

  /**
   * Clear search
   */
  clearSearch(): void {
    this.searchText = '';
    this.onSearchChange();
  }

  formatBinding(b: InputBindingDefinition): string {
    if (b.trigger.device !== 'keyboard') return '';
    return b.trigger.chord.steps.map(step => {
      const mods = (step.modifiers ?? []).map(m => this.modNames[m] ?? m).join('+');
      const key = this.keyNames[step.key] ?? step.key.toUpperCase();
      return mods ? `${mods}+${key}` : key;
    }).join(' → ');
  }

  toggleBinding(binding: InputBindingDefinition): void {
    this.inputEngine.setBindingEnabled(binding.id, binding.enabled === false);
    this.loadShortcuts();
  }

  exportConfig(): void {
    const snapshot = this.inputEngine.getBindingSnapshot();
    const config = {
      version: '2.0',
      bindings: snapshot.map(b => ({ id: b.id, enabled: b.enabled !== false })),
    };
    const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'keybindings.json';
    a.click();
    URL.revokeObjectURL(url);
  }

  importConfig(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (e) => {
      try {
        const config = JSON.parse(e.target?.result as string);
        const entries: Array<{ id: string; enabled: boolean }> = config.bindings ?? [];
        for (const entry of entries) {
          this.inputEngine.setBindingEnabled(entry.id, entry.enabled);
        }
        this.loadShortcuts();
        this.confirmDialog.alert('Import Successful\nKeybindings have been imported.');
      } catch {
        this.confirmDialog.alert('Import Failed\nInvalid file format.');
      }
    };
    reader.readAsText(file);
  }

  /**
   * Reset to default shortcuts
   */
  async resetToDefaults(): Promise<void> {
    const confirmed = await this.confirmDialog.confirm({
      title: 'Reset Shortcuts',
      message: 'Reset all shortcuts to defaults?',
      confirmText: 'Reset',
      danger: true,
    });
    if (confirmed) {
      for (const b of this.inputEngine.getBindingSnapshot()) {
        if (b.enabled === false) {
          this.inputEngine.setBindingEnabled(b.id, true);
        }
      }
      this.loadShortcuts();
    }
  }

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('dialog.keyboardHelp.close', () => {
      if (!this.isVisible) return;
      this.hide();
    });

    this.inputEngine.registerCommand('dialog.keyboardHelp.focusSearch', () => {
      if (!this.isVisible) return;
      this.searchInput?.nativeElement.focus();
      this.searchInput?.nativeElement.select();
    });

    this.inputEngine.registerCommand('dialog.keyboardHelp.exportBindings', () => {
      if (!this.isVisible) return;
      this.exportConfig();
    });

    this.inputEngine.registerCommand('dialog.keyboardHelp.focusNextArrow', () => {
      if (!this.isVisible || !this.shouldUseArrowFocusNavigation()) return;
      this.moveDialogFocus(1);
    });

    this.inputEngine.registerCommand('dialog.keyboardHelp.focusPrevArrow', () => {
      if (!this.isVisible || !this.shouldUseArrowFocusNavigation()) return;
      this.moveDialogFocus(-1);
    });
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.close');
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.focusSearch');
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.exportBindings');
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.focusNextArrow');
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.focusPrevArrow');
  }

  private moveDialogFocus(delta: number): void {
    const panel = this.dialogPanel?.nativeElement;
    if (!panel) {
      return;
    }

    moveFocusWithin(panel, delta, '.modal-footer button');
  }

  private shouldUseArrowFocusNavigation(): boolean {
    const panel = this.dialogPanel?.nativeElement;
    const activeElement = document.activeElement;
    if (!panel || !(activeElement instanceof HTMLElement) || !panel.contains(activeElement)) {
      return false;
    }

    return !!activeElement.closest('.modal-footer button');
  }
}



