import { Component, OnInit, OnDestroy, ViewChild, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject } from '@epikodelabs/streamix';
import { KeyboardService, ShortcutCategory } from '../services/keyboard.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';

@Component({
  selector: 'app-keyboard-help',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective],
  templateUrl: './keyboard-help.component.html',
  styleUrls: ['./keyboard-help.component.scss']
})
export class KeyboardHelpComponent implements OnInit, OnDestroy {
  @ViewChild('searchInput') searchInput?: ElementRef<HTMLInputElement>;

  isVisible = false;
  categories: ShortcutCategory[] = [];
  searchText = '';
  filteredCategories: ShortcutCategory[] = [];

  private destroy$ = createSubject<void>();
  private releaseDialogContext: (() => void) | null = null;

  constructor(
    private keyboardService: KeyboardService,
    private confirmDialog: ConfirmDialogService,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
  ) {}

  ngOnInit(): void {
    this.registerInputCommands();
    this.loadShortcuts();
  }

  ngOnDestroy(): void {
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.unregisterInputCommands();
    this.destroy$.next();
    this.destroy$.complete();
  }

  /**
   * Load shortcuts from service
   */
  loadShortcuts(): void {
    this.categories = this.keyboardService.getShortcutsByCategory();
    this.filteredCategories = this.categories;
  }

  /**
   * Show help modal
   */
  show(): void {
    this.isVisible = true;
    this.releaseDialogContext ??= this.inputContexts.acquire('dialog.keyboardHelp');
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
  }

  /**
   * Filter shortcuts by search text
   */
  onSearchChange(): void {
    const search = this.searchText.toLowerCase().trim();

    if (!search) {
      this.filteredCategories = this.categories;
      return;
    }

    this.filteredCategories = this.categories
      .map(category => ({
        name: category.name,
        shortcuts: category.shortcuts.filter(shortcut => {
          const keyMatch = this.keyboardService.formatShortcut(shortcut)
            .toLowerCase()
            .includes(search);
          const descMatch = shortcut.description
            .toLowerCase()
            .includes(search);
          return keyMatch || descMatch;
        }),
      }))
      .filter(category => category.shortcuts.length > 0);
  }

  /**
   * Clear search
   */
  clearSearch(): void {
    this.searchText = '';
    this.onSearchChange();
  }

  /**
   * Format shortcut for display
   */
  formatShortcut(shortcut: any): string {
    return this.keyboardService.formatShortcut(shortcut);
  }

  /**
   * Toggle shortcut enabled/disabled
   */
  toggleShortcut(shortcut: any): void {
    const newState = !shortcut.enabled;
    this.keyboardService.toggleShortcut(shortcut.id, newState);
    this.loadShortcuts();
  }

  /**
   * Export shortcuts configuration
   */
  exportConfig(): void {
    const config = this.keyboardService.exportConfig();
    const blob = new Blob([JSON.stringify(config, null, 2)], { 
      type: 'application/json' 
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'keyboard-shortcuts.json';
    a.click();
    URL.revokeObjectURL(url);
  }

  /**
   * Import shortcuts configuration
   */
  importConfig(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (e) => {
      try {
        const config = JSON.parse(e.target?.result as string);
        this.keyboardService.importConfig(config);
        this.loadShortcuts();
        this.confirmDialog.alert('Import Successful\nShortcuts have been imported.');
      } catch (error) {
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
      // This would need to be implemented based on your default shortcuts
      this.confirmDialog.alert('Not Yet Implemented\nReset functionality is coming soon.');
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
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.close');
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.focusSearch');
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.exportBindings');
  }
}



