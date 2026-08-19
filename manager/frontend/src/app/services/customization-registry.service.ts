import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import type { WorkspaceLayout, WorkspacePresentation } from './workspace-surface.model';

export interface ThemeContribution {
  readonly id: string;
  readonly label: string;
  readonly kind: 'light' | 'dark';
  readonly variables: Readonly<Record<string, string>>;
}

export interface ColumnProfileContribution {
  readonly id: string;
  readonly label: string;
  readonly columnIds: readonly string[];
  readonly widths?: Readonly<Record<string, string>>;
}

export interface LayoutPresetContribution {
  readonly id: string;
  readonly label: string;
  readonly description?: string;
  createLayout(workspaceId: string): WorkspaceLayout;
}

export interface WorkspaceTemplateContext {
  readonly workspaceId: string;
  readonly title?: string;
}

export interface WorkspaceTemplateContribution {
  readonly id: string;
  readonly label: string;
  readonly description?: string;
  create(context: WorkspaceTemplateContext): WorkspacePresentation;
}

interface OwnedContribution<T> {
  readonly ownerId: string;
  readonly value: T;
  readonly order: number;
}

/** Runtime registry for user-facing customization assets and plugin contributions. */
@Injectable({ providedIn: 'root' })
export class CustomizationRegistryService {
  private readonly themes = new Map<string, Map<string, OwnedContribution<ThemeContribution>>>();
  private readonly columnProfiles = new Map<string, Map<string, OwnedContribution<ColumnProfileContribution>>>();
  private readonly layouts = new Map<string, Map<string, OwnedContribution<LayoutPresetContribution>>>();
  private readonly templates = new Map<string, Map<string, OwnedContribution<WorkspaceTemplateContribution>>>();
  private order = 0;

  readonly themes$ = createBehaviorSubject<ThemeContribution[]>([]);
  readonly columnProfiles$ = createBehaviorSubject<ColumnProfileContribution[]>([]);
  readonly layoutPresets$ = createBehaviorSubject<LayoutPresetContribution[]>([]);
  readonly workspaceTemplates$ = createBehaviorSubject<WorkspaceTemplateContribution[]>([]);

  registerTheme(value: ThemeContribution, ownerId = 'default'): void {
    this.register(this.themes, value.id, ownerId, value);
    this.emitThemes();
  }

  unregisterTheme(id: string, ownerId = 'default'): void {
    this.unregister(this.themes, id, ownerId);
    this.emitThemes();
  }

  registerColumnProfile(value: ColumnProfileContribution, ownerId = 'default'): void {
    this.register(this.columnProfiles, value.id, ownerId, value);
    this.emitColumnProfiles();
  }

  unregisterColumnProfile(id: string, ownerId = 'default'): void {
    this.unregister(this.columnProfiles, id, ownerId);
    this.emitColumnProfiles();
  }

  registerLayoutPreset(value: LayoutPresetContribution, ownerId = 'default'): void {
    this.register(this.layouts, value.id, ownerId, value);
    this.emitLayouts();
  }

  unregisterLayoutPreset(id: string, ownerId = 'default'): void {
    this.unregister(this.layouts, id, ownerId);
    this.emitLayouts();
  }

  registerWorkspaceTemplate(value: WorkspaceTemplateContribution, ownerId = 'default'): void {
    this.register(this.templates, value.id, ownerId, value);
    this.emitTemplates();
  }

  unregisterWorkspaceTemplate(id: string, ownerId = 'default'): void {
    this.unregister(this.templates, id, ownerId);
    this.emitTemplates();
  }

  getTheme(id: string): ThemeContribution | undefined { return this.active(this.themes.get(id))?.value; }
  getColumnProfile(id: string): ColumnProfileContribution | undefined { return this.active(this.columnProfiles.get(id))?.value; }
  getLayoutPreset(id: string): LayoutPresetContribution | undefined { return this.active(this.layouts.get(id))?.value; }
  getWorkspaceTemplate(id: string): WorkspaceTemplateContribution | undefined { return this.active(this.templates.get(id))?.value; }

  applyTheme(id: string, target: HTMLElement = document.documentElement): boolean {
    const theme = this.getTheme(id);
    if (!theme) return false;
    target.dataset['themeId'] = theme.id;
    target.dataset['themeKind'] = theme.kind;
    for (const [name, value] of Object.entries(theme.variables)) {
      target.style.setProperty(name.startsWith('--') ? name : `--${name}`, value);
    }
    return true;
  }

  unregisterOwner(ownerId: string): void {
    for (const registry of [this.themes, this.columnProfiles, this.layouts, this.templates]) {
      for (const [id, registrations] of registry) {
        registrations.delete(ownerId);
        if (registrations.size === 0) registry.delete(id);
      }
    }
    this.emitAll();
  }

  private register<T>(registry: Map<string, Map<string, OwnedContribution<T>>>, id: string, ownerId: string, value: T): void {
    const registrations = registry.get(id) ?? new Map<string, OwnedContribution<T>>();
    registrations.set(ownerId, { ownerId, value, order: ++this.order });
    registry.set(id, registrations);
  }

  private unregister<T>(registry: Map<string, Map<string, OwnedContribution<T>>>, id: string, ownerId: string): void {
    const registrations = registry.get(id);
    if (!registrations) return;
    registrations.delete(ownerId);
    if (registrations.size === 0) registry.delete(id);
  }

  private active<T>(registrations: Map<string, OwnedContribution<T>> | undefined): OwnedContribution<T> | undefined {
    if (!registrations?.size) return undefined;
    let active: OwnedContribution<T> | undefined;
    for (const registration of registrations.values()) {
      if (!active || registration.order > active.order) active = registration;
    }
    return active;
  }

  private values<T>(registry: Map<string, Map<string, OwnedContribution<T>>>): T[] {
    return Array.from(registry.values())
      .map(registrations => this.active(registrations))
      .filter((entry): entry is OwnedContribution<T> => !!entry)
      .sort((left, right) => left.order - right.order)
      .map(entry => entry.value);
  }

  private emitThemes(): void { this.themes$.next(this.values(this.themes)); }
  private emitColumnProfiles(): void { this.columnProfiles$.next(this.values(this.columnProfiles)); }
  private emitLayouts(): void { this.layoutPresets$.next(this.values(this.layouts)); }
  private emitTemplates(): void { this.workspaceTemplates$.next(this.values(this.templates)); }
  private emitAll(): void { this.emitThemes(); this.emitColumnProfiles(); this.emitLayouts(); this.emitTemplates(); }
}