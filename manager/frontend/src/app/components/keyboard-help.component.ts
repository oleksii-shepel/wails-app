import { ChangeDetectorRef, Component, OnInit, OnDestroy, ViewChild, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import {
  InputBindingDefinition,
  InputContextDefinition,
} from '../services/input-system.types';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';

interface BindingCategory {
  name: string;
  bindings: InputBindingDefinition[];
}

interface BindingSection {
  title: string;
  subtitle: string;
  categories: BindingCategory[];
  emptyMessage: string;
}

interface KeyboardHelpShowOptions {
  contextIds?: Iterable<string>;
  includeGlobal?: boolean;
}

@Component({
  selector: 'app-keyboard-help',
  standalone: true,
  imports: [CommonModule, FormsModule],
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
  sections: BindingSection[] = [];
  filteredSections: BindingSection[] = [];
  hasSearchResults = true;

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
  private preservedContextIds = new Set<string>();
  private includeGlobalSection = true;
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

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
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.overlayFreeze.release('dialog.keyboardHelp');
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.unregisterInputCommands();
  }

  loadShortcuts(): void {
    const snapshot = this.inputEngine.getBindingSnapshot();
    const contexts = new Map(
      this.inputEngine.getContextSnapshot().map(context => [context.id, context] as const),
    );

    const live = snapshot.filter(binding =>
      binding.enabled !== false &&
      binding.trigger.device === 'keyboard' &&
      !!binding.category
    );

    const dialogSection = this.buildDialogSection(live);
    const contextualSections = this.buildContextualSections(live, contexts);
    const globalCategories = this.groupBindings(
      live.filter(binding => binding.contextId === 'global'),
    );

    this.categories = globalCategories;
    this.sections = [
      ...(dialogSection ? [dialogSection] : []),
      ...contextualSections,
      ...(this.includeGlobalSection ? [{
        title: 'Global',
        subtitle: 'Available across the app when nothing more specific takes over.',
        categories: globalCategories,
        emptyMessage: 'No global shortcuts are currently available.',
      }] : []),
    ];

    this.applySearch();
  }

  /**
   * Show help modal
   */
  show(options: KeyboardHelpShowOptions = {}): void {
    const scopedContextIds = options.contextIds
      ? new Set(
          Array.from(options.contextIds)
            .map(contextId => String(contextId))
            .filter(contextId => contextId !== 'global' && contextId !== 'dialog.keyboardHelp'),
        )
      : null;
    this.preservedContextIds = scopedContextIds ?? new Set(
      Array.from(this.inputEngine.getActiveContextIds())
        .filter(contextId => contextId !== 'global' && contextId !== 'dialog.keyboardHelp'),
    );
    this.includeGlobalSection = options.includeGlobal ?? scopedContextIds === null;
    this.isVisible = true;
    this.renderVisible = true;
    this.isClosing = false;
    this.releaseDialogContext ??= this.inputContexts.acquire('dialog.keyboardHelp');
    this.overlayFreeze.acquire('dialog.keyboardHelp');
    if (this.modalStack && this.hostElement) {
      this.unregisterModal = this.modalStack.register({
        id: 'keyboard-help',
        element: this.hostElement.nativeElement,
        contextId: 'dialog.keyboardHelp',
        dismiss: () => this.hide(),
        dismissible: () => this.isVisible && !this.isClosing,
      });
    }
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
    this.unregisterModal?.();
    this.unregisterModal = null;
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
    this.applySearch();
  }

  /**
   * Clear search
   */
  clearSearch(): void {
    this.searchText = '';
    this.applySearch();
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

  private applySearch(): void {
    const search = this.searchText.toLowerCase().trim();
    if (!search) {
      this.filteredSections = this.sections;
      this.filteredCategories = this.categories;
      this.hasSearchResults = this.sections.some(section => section.categories.length > 0);
      return;
    }

    this.filteredSections = this.sections
      .map(section => ({
        ...section,
        categories: section.categories
          .map(category => ({
            name: category.name,
            bindings: category.bindings.filter(binding => this.bindingMatchesSearch(binding, search)),
          }))
          .filter(category => category.bindings.length > 0),
      }))
      .filter(section => section.categories.length > 0);

    this.filteredCategories = this.filteredSections
      .find(section => section.title === 'Global')
      ?.categories ?? [];
    this.hasSearchResults = this.filteredSections.length > 0;
  }

  private bindingMatchesSearch(binding: InputBindingDefinition, search: string): boolean {
    const keyMatch = this.formatBinding(binding).toLowerCase().includes(search);
    const descMatch = (binding.description ?? '').toLowerCase().includes(search);
    return keyMatch || descMatch;
  }

  private buildDialogSection(bindings: InputBindingDefinition[]): BindingSection | null {
    const helpBindings = bindings.filter(binding => binding.contextId === 'dialog.keyboardHelp');
    if (helpBindings.length === 0) {
      return null;
    }

    return {
      title: 'This Dialog',
      subtitle: 'Shortcuts for navigating the keyboard help window itself.',
      categories: this.groupBindings(helpBindings),
      emptyMessage: 'No keyboard-help shortcuts are registered.',
    };
  }

  private buildContextualSections(
    bindings: InputBindingDefinition[],
    contexts: Map<string, InputContextDefinition>,
  ): BindingSection[] {
    const sections: BindingSection[] = [];
    const liveContextIds = Array.from(this.preservedContextIds)
      .filter(contextId => contextId !== 'global');

    if (liveContextIds.length > 0) {
      for (const contextId of this.sortContextIds(liveContextIds, contexts)) {
        const contextBindings = bindings.filter(binding => binding.contextId === contextId);
        if (contextBindings.length === 0) continue;
        const context = contexts.get(contextId);
        sections.push({
          title: this.contextHeading(context, contextId),
          subtitle: this.contextSubtitle(context),
          categories: this.groupBindings(contextBindings),
          emptyMessage: 'No shortcuts are registered for this context.',
        });
      }
    }

    return sections;
  }

  private sortContextIds(
    contextIds: string[],
    contexts: Map<string, InputContextDefinition>,
  ): string[] {
    return contextIds
      .slice()
      .sort((left, right) => {
        const leftContext = contexts.get(left);
        const rightContext = contexts.get(right);
        const priorityDelta = (rightContext?.priority ?? 0) - (leftContext?.priority ?? 0);
        if (priorityDelta !== 0) return priorityDelta;
        return this.contextHeading(leftContext, left).localeCompare(this.contextHeading(rightContext, right));
      });
  }

  private contextHeading(context: InputContextDefinition | undefined, contextId: string): string {
    const description = context?.description?.trim();
    if (description) {
      return `Current Context: ${description}`;
    }
    return `Current Context: ${contextId}`;
  }

  private contextSubtitle(context: InputContextDefinition | undefined): string {
    const kind = context?.kind;
    switch (kind) {
      case 'dialog':
        return 'Actions available in the dialog you opened from.';
      case 'overlay':
        return 'Actions available in the overlay you opened from.';
      case 'widget':
        return 'Actions available in the focused widget you opened from.';
      case 'pane':
        return 'Actions available in the active pane you opened from.';
      default:
        return 'Actions that depend on where you were when you opened help.';
    }
  }

  private groupBindings(bindings: InputBindingDefinition[]): BindingCategory[] {
    const byCategory = new Map<string, InputBindingDefinition[]>();
    for (const binding of bindings) {
      const category = binding.category ?? 'Other';
      if (!byCategory.has(category)) {
        byCategory.set(category, []);
      }
      byCategory.get(category)!.push(binding);
    }

    return Array.from(byCategory.entries())
      .map(([name, categoryBindings]) => ({
        name,
        bindings: categoryBindings
          .slice()
          .sort((a, b) =>
            a.description.localeCompare(b.description) ||
            this.formatBinding(a).localeCompare(this.formatBinding(b)),
          ),
      }))
      .sort((a, b) => a.name.localeCompare(b.name));
  }
}
