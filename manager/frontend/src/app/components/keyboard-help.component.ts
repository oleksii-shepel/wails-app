import { ChangeDetectorRef, Component, OnInit, OnDestroy, ViewChild, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { InputBindingDefinition } from '../services/input-system.types';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { VscodeScrollbarDirective } from '../directives/vscode-scrollbar.directive';

interface BindingCategory {
  name: string;
  bindings: InputBindingDefinition[];
}

@Component({
  selector: 'app-keyboard-help',
  standalone: true,
  imports: [CommonModule, FormsModule, VscodeScrollbarDirective],
  templateUrl: './keyboard-help.component.html',
  styleUrls: ['./keyboard-help.component.scss']
})
export class KeyboardHelpComponent implements OnInit, OnDestroy {
  @ViewChild('dialogPanel') dialogPanel?: ElementRef<HTMLElement>;
  @ViewChild('searchInput') searchInput?: ElementRef<HTMLInputElement>;

  isVisible = false;
  renderVisible = false;
  isClosing = false;
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

  private releaseDialogContext: (() => void) | null = null;

  constructor(
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
    private overlayFreeze: NativeOverlayFreezeService,
    private cdr: ChangeDetectorRef,
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
  }

  loadShortcuts(): void {
    const snapshot = this.inputEngine.getBindingSnapshot();

    const live = snapshot.filter(binding =>
      binding.enabled !== false &&
      binding.trigger.device === 'keyboard' &&
      !!binding.category
    );

    const byCategory = new Map<string, InputBindingDefinition[]>();
    for (const binding of live) {
      const category = binding.category!;
      if (!byCategory.has(category)) {
        byCategory.set(category, []);
      }
      byCategory.get(category)!.push(binding);
    }

    this.categories = Array.from(byCategory.entries())
      .map(([name, bindings]) => ({
        name,
        bindings: bindings
          .slice()
          .sort((a, b) =>
            a.description.localeCompare(b.description) ||
            this.formatBinding(a).localeCompare(this.formatBinding(b)),
          ),
      }))
      .sort((a, b) => a.name.localeCompare(b.name));

    this.filteredCategories = this.categories;
  }

  /**
   * Show help modal
   */
  show(): void {
    this.isVisible = true;
    this.renderVisible = true;
    this.isClosing = false;
    this.releaseDialogContext ??= this.inputContexts.acquire('dialog.keyboardHelp');
    this.overlayFreeze.acquire('dialog.keyboardHelp');
    this.searchText = '';
    this.loadShortcuts();
    this.cdr.detectChanges();
    queueMicrotask(() => this.dialogPanel?.nativeElement.focus());
  }

  /**
   * Hide help modal
   */
  hide(): void {
    if (!this.renderVisible || this.isClosing) {
      return;
    }
    this.isVisible = false;
    this.isClosing = true;
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.overlayFreeze.release('dialog.keyboardHelp');
    this.cdr.detectChanges();
  }

  onBackdropAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.renderVisible = false;
    this.isClosing = false;
    this.cdr.detectChanges();
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

  bindingKeySteps(binding: InputBindingDefinition): readonly (readonly string[])[] {
    if (binding.trigger.device !== 'keyboard') return [];
    return binding.trigger.chord.steps.map(step => [
      ...(step.modifiers ?? []).map(modifier => this.modNames[modifier] ?? modifier),
      this.keyNames[step.key] ?? step.key.toUpperCase(),
    ]);
  }

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('dialog.keyboardHelp.close', () => {
      if (this.isVisible) this.hide();
    });

    this.inputEngine.registerCommand('dialog.keyboardHelp.focusSearch', () => {
      if (!this.isVisible) return;
      this.searchInput?.nativeElement.focus();
      this.searchInput?.nativeElement.select();
    });
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.close');
    this.inputEngine.unregisterCommand('dialog.keyboardHelp.focusSearch');
  }
}
