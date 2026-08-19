import { Injectable } from '@angular/core';

export interface PaneIntelligenceModes {
  readonly linkedNavigation: boolean;
  readonly mirrorPresentation: boolean;
  readonly mirrorFilter: boolean;
  readonly syncScroll: boolean;
  readonly followSelection: boolean;
}

export interface PanePairRoots {
  readonly left: string;
  readonly right: string;
}

export interface WorkspacePaneIntelligence
  extends PaneIntelligenceModes {
  readonly roots: PanePairRoots | null;
  readonly updatedAt: number;
}

export interface PanePairPreset {
  readonly id: string;
  readonly name: string;
  readonly roots: PanePairRoots;
  readonly modes: PaneIntelligenceModes;
  readonly createdAt: number;
  readonly updatedAt: number;
}

interface PersistedPaneIntelligence {
  version: 1;
  workspaces: Record<string, WorkspacePaneIntelligence>;
  presets: PanePairPreset[];
}

const STORAGE_KEY = 'pane-intelligence-v1';

const DEFAULT_MODES: PaneIntelligenceModes = {
  linkedNavigation: false,
  mirrorPresentation: false,
  mirrorFilter: false,
  syncScroll: false,
  followSelection: false,
};

@Injectable({ providedIn: 'root' })
export class PaneIntelligenceService {
  loadWorkspace(workspaceId: string): WorkspacePaneIntelligence {
    const stored = this.read();
    const value = stored.workspaces[workspaceId];

    if (!value) {
      return {
        ...DEFAULT_MODES,
        roots: null,
        updatedAt: 0,
      };
    }

    return this.normalizeWorkspaceState(value);
  }

  saveWorkspace(
    workspaceId: string,
    state: Omit<WorkspacePaneIntelligence, 'updatedAt'>,
  ): void {
    if (!workspaceId) return;

    const stored = this.read();
    stored.workspaces[workspaceId] = {
      ...this.normalizeWorkspaceState(state),
      updatedAt: Date.now(),
    };
    this.write(stored);
  }

  removeWorkspace(workspaceId: string): void {
    const stored = this.read();
    if (!(workspaceId in stored.workspaces)) return;
    delete stored.workspaces[workspaceId];
    this.write(stored);
  }

  listPresets(): readonly PanePairPreset[] {
    return this.read().presets
      .slice()
      .sort((a, b) => a.name.localeCompare(b.name, undefined, {
        numeric: true,
        sensitivity: 'base',
      }));
  }

  savePreset(input: {
    id?: string;
    name: string;
    roots: PanePairRoots;
    modes: PaneIntelligenceModes;
  }): PanePairPreset {
    const name = input.name.trim();
    if (!name) throw new Error('Preset name is required.');
    if (!input.roots.left || !input.roots.right) {
      throw new Error('Both pane roots are required.');
    }

    const stored = this.read();
    const now = Date.now();
    const existing = input.id
      ? stored.presets.find(item => item.id === input.id)
      : stored.presets.find(item =>
          item.name.localeCompare(name, undefined, { sensitivity: 'base' }) === 0
        );

    const preset: PanePairPreset = {
      id: existing?.id ?? this.generateId(),
      name,
      roots: {
        left: input.roots.left,
        right: input.roots.right,
      },
      modes: this.normalizeModes(input.modes),
      createdAt: existing?.createdAt ?? now,
      updatedAt: now,
    };

    stored.presets = [
      ...stored.presets.filter(item => item.id !== preset.id),
      preset,
    ];
    this.write(stored);
    return preset;
  }

  deletePreset(id: string): void {
    const stored = this.read();
    const next = stored.presets.filter(item => item.id !== id);
    if (next.length === stored.presets.length) return;
    stored.presets = next;
    this.write(stored);
  }

  preset(id: string): PanePairPreset | null {
    return this.read().presets.find(item => item.id === id) ?? null;
  }

  private read(): PersistedPaneIntelligence {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return this.empty();

      const parsed = JSON.parse(raw) as Partial<PersistedPaneIntelligence>;
      const presets = Array.isArray(parsed.presets)
        ? parsed.presets
            .filter((item): item is PanePairPreset =>
              !!item &&
              typeof item === 'object' &&
              typeof (item as PanePairPreset).id === 'string' &&
              typeof (item as PanePairPreset).name === 'string'
            )
            .map(item => ({
              ...item,
              roots: {
                left: item.roots?.left ?? '',
                right: item.roots?.right ?? '',
              },
              modes: this.normalizeModes(item.modes),
            }))
        : [];

      return {
        version: 1,
        workspaces:
          parsed.workspaces && typeof parsed.workspaces === 'object'
            ? parsed.workspaces as Record<string, WorkspacePaneIntelligence>
            : {},
        presets,
      };
    } catch {
      return this.empty();
    }
  }

  private write(value: PersistedPaneIntelligence): void {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
    } catch {
      // Storage failure must never break file-manager interaction.
    }
  }

  private normalizeWorkspaceState(
    value: Partial<WorkspacePaneIntelligence>,
  ): WorkspacePaneIntelligence {
    const modes = this.normalizeModes(value);
    const roots =
      value.roots?.left && value.roots?.right
        ? { left: value.roots.left, right: value.roots.right }
        : null;

    return {
      ...modes,
      roots,
      updatedAt:
        typeof value.updatedAt === 'number' && Number.isFinite(value.updatedAt)
          ? value.updatedAt
          : 0,
    };
  }

  private normalizeModes(
    value: Partial<PaneIntelligenceModes> | null | undefined,
  ): PaneIntelligenceModes {
    return {
      linkedNavigation: !!value?.linkedNavigation,
      mirrorPresentation: !!value?.mirrorPresentation,
      mirrorFilter: !!value?.mirrorPresentation && !!value?.mirrorFilter,
      syncScroll: !!value?.syncScroll,
      followSelection: !!value?.followSelection,
    };
  }

  private empty(): PersistedPaneIntelligence {
    return {
      version: 1,
      workspaces: {},
      presets: [],
    };
  }

  private generateId(): string {
    return `pane-pair-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }
}