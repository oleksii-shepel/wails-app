import { Injectable } from '@angular/core';
import { InputTrigger, InputBindingDefinition } from './input-system.types';
import { FILE_MANAGER_BINDINGS } from '../components/command-registry';
import { BUILTIN_INPUT_CATALOG } from './dialog-input-catalog';

export interface KeybindingOverride {
  id: string;
  trigger?: InputTrigger;
  enabled?: boolean;
}

const STORAGE_KEY = 'keybinding-overrides-v1';

function buildDefaultCatalog(): Map<string, InputBindingDefinition> {
  const map = new Map<string, InputBindingDefinition>();
  for (const binding of FILE_MANAGER_BINDINGS) {
    map.set(binding.id, binding);
  }
  for (const entry of BUILTIN_INPUT_CATALOG) {
    for (const binding of entry.bindings) {
      map.set(binding.id, binding);
    }
  }
  return map;
}

@Injectable({ providedIn: 'root' })
export class KeybindingStorageService {
  private readonly defaults = buildDefaultCatalog();
  private overrides: KeybindingOverride[] = [];

  constructor() {
    this.load();
  }

  getOverrides(): KeybindingOverride[] {
    return this.overrides.map(override => ({ ...override }));
  }

  getOverride(bindingId: string): KeybindingOverride | undefined {
    const override = this.overrides.find(item => item.id === bindingId);
    return override ? { ...override } : undefined;
  }

  hasOverride(bindingId: string): boolean {
    return this.overrides.some(item => item.id === bindingId);
  }

  getDefaultBinding(bindingId: string): InputBindingDefinition | undefined {
    return this.defaults.get(bindingId);
  }

  getDefaultBindings(): InputBindingDefinition[] {
    return Array.from(this.defaults.values()).map(binding => ({ ...binding }));
  }

  mergeWithDefaults(snapshot: InputBindingDefinition[]): InputBindingDefinition[] {
    const merged = new Map<string, InputBindingDefinition>();

    for (const binding of this.defaults.values()) {
      merged.set(binding.id, { ...binding, enabled: true });
    }

    for (const binding of snapshot) {
      merged.set(binding.id, binding);
    }

    return Array.from(merged.values());
  }

  setOverride(override: KeybindingOverride): void {
    const idx = this.overrides.findIndex(o => o.id === override.id);
    if (idx >= 0) {
      this.overrides[idx] = { ...this.overrides[idx], ...override };
    } else {
      this.overrides.push(override);
    }
    this.save();
  }

  removeOverride(bindingId: string): void {
    this.overrides = this.overrides.filter(o => o.id !== bindingId);
    this.save();
  }

  resetAll(): void {
    this.overrides = [];
    this.save();
  }

  applyOverrides(engine: {
    registerBinding(binding: InputBindingDefinition): void;
    updateBindingTrigger(bindingId: string, trigger: InputTrigger): boolean;
    setBindingEnabled(bindingId: string, enabled: boolean): void;
  }): void {
    for (const override of this.overrides) {
      if (override.trigger) {
        engine.updateBindingTrigger(override.id, override.trigger);
      }
      if (typeof override.enabled === 'boolean') {
        engine.setBindingEnabled(override.id, override.enabled);
      }
    }
  }

  private load(): void {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw) {
        const parsed = JSON.parse(raw) as KeybindingOverride[];
        if (Array.isArray(parsed)) {
          this.overrides = parsed;
        }
      }
    } catch {
      // ignore parse errors
    }
  }

  private save(): void {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.overrides));
    } catch {
      // ignore storage errors
    }
  }
}
