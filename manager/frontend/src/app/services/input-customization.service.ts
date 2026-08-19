import { Injectable, OnDestroy } from '@angular/core';

import { InputEngineService } from './input-engine.service';
import {
  KeybindingStorageService,
  type KeybindingOverride,
} from './keybinding-storage.service';
import type {
  InputBindingConflict,
  InputBindingDefinition,
  InputTrigger,
} from './input-system.types';

@Injectable({ providedIn: 'root' })
export class InputCustomizationService implements OnDestroy {
  private applying = false;
  private readonly subscription: { unsubscribe?: () => void } | (() => void) | void;

  constructor(
    private readonly engine: InputEngineService,
    private readonly storage: KeybindingStorageService,
  ) {
    this.subscription = this.engine.bindings$.subscribe(() => {
      this.reapplyPersistedOverrides();
    });
    this.reapplyPersistedOverrides();
  }

  ngOnDestroy(): void {
    if (typeof this.subscription === 'function') {
      this.subscription();
    } else {
      this.subscription?.unsubscribe?.();
    }
  }

  bindings(): InputBindingDefinition[] {
    return this.engine.getBindingSnapshot()
      .slice()
      .sort((left, right) =>
        (left.category ?? '').localeCompare(right.category ?? '') ||
        left.description.localeCompare(right.description) ||
        left.id.localeCompare(right.id)
      );
  }

  conflicts(): InputBindingConflict[] {
    return this.engine.inspectBindingConflicts();
  }

  conflictsFor(bindingId: string): InputBindingConflict[] {
    return this.conflicts().filter(conflict =>
      conflict.bindingId === bindingId ||
      conflict.conflictingBindingId === bindingId
    );
  }

  customize(
    bindingId: string,
    value: { trigger?: InputTrigger; enabled?: boolean },
  ): boolean {
    const current = this.engine.getBinding(bindingId);
    if (!current) return false;

    const override: KeybindingOverride = {
      id: bindingId,
      trigger: value.trigger,
      enabled: value.enabled,
    };

    this.storage.setOverride(override);
    return this.engine.applyUserBindingCustomization(override);
  }

  setEnabled(bindingId: string, enabled: boolean): boolean {
    const stored = this.storage.getOverride(bindingId);
    return this.customize(bindingId, {
      trigger: stored?.trigger,
      enabled,
    });
  }

  setTrigger(bindingId: string, trigger: InputTrigger): boolean {
    const stored = this.storage.getOverride(bindingId);
    return this.customize(bindingId, {
      trigger,
      enabled: stored?.enabled,
    });
  }

  reset(bindingId: string): boolean {
    this.storage.removeOverride(bindingId);
    const changed = this.engine.clearUserBindingCustomization(bindingId);
    this.reapplyPersistedOverrides();
    return changed;
  }

  resetAll(): void {
    this.storage.resetAll();
    for (const binding of this.engine.getBindingSnapshot()) {
      this.engine.clearUserBindingCustomization(binding.id);
    }
  }

  hasOverride(bindingId: string): boolean {
    return this.storage.hasOverride(bindingId);
  }

  exportConfig(): string {
    return JSON.stringify({
      version: 3,
      overrides: this.storage.getOverrides(),
    }, null, 2);
  }

  importConfig(raw: string): { imported: number; ignored: number } {
    const parsed = JSON.parse(raw) as {
      version?: unknown;
      overrides?: unknown;
      bindings?: unknown;
    };

    let values: KeybindingOverride[] = [];

    if (Array.isArray(parsed.overrides)) {
      values = parsed.overrides as KeybindingOverride[];
    } else if (Array.isArray(parsed.bindings)) {
      // Compatibility with the previous enable/disable-only export.
      values = (parsed.bindings as Array<{ id?: unknown; enabled?: unknown }>)
        .filter(item => typeof item.id === 'string')
        .map(item => ({
          id: item.id as string,
          enabled: typeof item.enabled === 'boolean'
            ? item.enabled
            : undefined,
        }));
    }

    let imported = 0;
    let ignored = 0;

    for (const override of values) {
      if (!override || typeof override.id !== 'string' || !override.id) {
        ignored += 1;
        continue;
      }

      this.storage.setOverride({
        id: override.id,
        trigger: override.trigger,
        enabled: override.enabled,
      });

      if (this.engine.getBinding(override.id)) {
        this.engine.applyUserBindingCustomization(override);
        imported += 1;
      } else {
        // Keep it persisted: this may belong to a plugin loaded later.
        imported += 1;
      }
    }

    return { imported, ignored };
  }

  private reapplyPersistedOverrides(): void {
    if (this.applying) return;
    this.applying = true;

    try {
      for (const override of this.storage.getOverrides()) {
        const current = this.engine.getBinding(override.id);
        if (!current || current.source === 'user') continue;
        this.engine.applyUserBindingCustomization(override);
      }
    } finally {
      this.applying = false;
    }
  }
}