import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

import type { WorkspaceConfig } from '../shared/protocol-enhanced';
import {
  fileWorkspacePresentation,
  type WorkspaceLayout,
  type WorkspacePresentation,
  type WorkspaceSurface,
} from './workspace-surface.model';

interface PersistedToolWorkspaceEnvelope {
  readonly version: 1;
  readonly workspaces: readonly WorkspacePresentation[];
}

const PERSISTED_TOOL_WORKSPACES_KEY =
  'reassembly.toolWorkspaces.v1';
const MAX_PERSISTED_BYTES = 3_500_000;

@Injectable({ providedIn: 'root' })
export class WorkspaceSurfaceService {
  private readonly presentations = new Map<string, WorkspacePresentation>();
  private readonly activeSubject =
    createBehaviorSubject<WorkspacePresentation | null>(null);

  constructor() {
    this.hydratePersistentTools();
  }

  readonly active$ = this.activeSubject;

  get active(): WorkspacePresentation | null {
    return this.activeSubject.value;
  }

  ensureFilesWorkspace(workspace: WorkspaceConfig): WorkspacePresentation {
    const existing = this.presentations.get(workspace.id);
    if (existing?.kind === 'files') {
      const next = fileWorkspacePresentation(
        workspace.id,
        workspace.splitRatio ?? 50,
      );
      this.presentations.set(workspace.id, next);
      return next;
    }

    const presentation = fileWorkspacePresentation(
      workspace.id,
      workspace.splitRatio ?? 50,
    );
    this.presentations.set(workspace.id, presentation);
    return presentation;
  }

  activateFilesWorkspace(workspace: WorkspaceConfig): WorkspacePresentation {
    const presentation = this.ensureFilesWorkspace(workspace);
    this.activeSubject.next(presentation);
    return presentation;
  }

  listPersistentTools(): readonly WorkspacePresentation[] {
    return [...this.presentations.values()]
      .filter(
        presentation =>
          presentation.kind === 'tool' &&
          presentation.lifetime === 'persistent',
      )
      .sort((left, right) =>
        left.title.localeCompare(right.title),
      );
  }

  openToolWorkspace(
    title: string,
    layout: WorkspaceLayout,
    options: {
      id?: string;
      lifetime?: 'persistent' | 'transient';
    } = {},
  ): WorkspacePresentation {
    const id =
      options.id ??
      `tool:${globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`}`;

    const presentation: WorkspacePresentation = {
      workspaceId: id,
      title: title.trim() || 'Tool',
      kind: 'tool',
      lifetime: options.lifetime ?? 'transient',
      layout,
    };

    this.presentations.set(id, presentation);
    this.activeSubject.next(presentation);
    return presentation;
  }

  promoteToWorkspace(
    surface: WorkspaceSurface,
    options: {
      title?: string;
      lifetime?: 'persistent' | 'transient';
    } = {},
  ): WorkspacePresentation {
    return this.openToolWorkspace(
      options.title ?? surface.title,
      { kind: 'surface', surface },
      { lifetime: options.lifetime },
    );
  }

  activate(workspaceId: string): boolean {
    const presentation = this.presentations.get(workspaceId);
    if (!presentation) return false;
    this.activeSubject.next(presentation);
    return true;
  }

  closeTransient(workspaceId: string): boolean {
    const presentation = this.presentations.get(workspaceId);
    if (!presentation || presentation.lifetime !== 'transient') return false;

    this.presentations.delete(workspaceId);
    if (this.activeSubject.value?.workspaceId === workspaceId) {
      this.activeSubject.next(null);
    }
    return true;
  }

  pin(workspaceId: string): WorkspacePresentation | null {
    const presentation = this.presentations.get(workspaceId);
    if (!presentation) return null;
    if (presentation.lifetime === 'persistent') return presentation;

    const pinned: WorkspacePresentation = {
      ...presentation,
      lifetime: 'persistent',
    };

    this.presentations.set(workspaceId, pinned);
    if (!this.persistPersistentTools()) {
      this.presentations.set(workspaceId, presentation);
      return null;
    }

    if (this.activeSubject.value?.workspaceId === workspaceId) {
      this.activeSubject.next(pinned);
    }
    return pinned;
  }

  rename(
    workspaceId: string,
    title: string,
  ): WorkspacePresentation | null {
    const presentation = this.presentations.get(workspaceId);
    const clean = title.trim();
    if (
      !presentation ||
      presentation.kind !== 'tool' ||
      presentation.lifetime !== 'persistent' ||
      !clean
    ) {
      return null;
    }

    const renamed: WorkspacePresentation = {
      ...presentation,
      title: clean,
    };

    this.presentations.set(workspaceId, renamed);
    if (!this.persistPersistentTools()) {
      this.presentations.set(workspaceId, presentation);
      return null;
    }

    if (this.activeSubject.value?.workspaceId === workspaceId) {
      this.activeSubject.next(renamed);
    }

    return renamed;
  }

  removePersistent(workspaceId: string): boolean {
    const presentation = this.presentations.get(workspaceId);
    if (
      !presentation ||
      presentation.kind !== 'tool' ||
      presentation.lifetime !== 'persistent'
    ) {
      return false;
    }

    this.presentations.delete(workspaceId);
    if (!this.persistPersistentTools()) {
      this.presentations.set(workspaceId, presentation);
      return false;
    }

    if (this.activeSubject.value?.workspaceId === workspaceId) {
      this.activeSubject.next(null);
    }
    return true;
  }

  get(workspaceId: string): WorkspacePresentation | null {
    return this.presentations.get(workspaceId) ?? null;
  }

  private hydratePersistentTools(): void {
    try {
      const raw = localStorage.getItem(PERSISTED_TOOL_WORKSPACES_KEY);
      if (!raw) return;

      const envelope = JSON.parse(raw) as PersistedToolWorkspaceEnvelope;
      if (envelope?.version !== 1 || !Array.isArray(envelope.workspaces)) {
        return;
      }

      for (const presentation of envelope.workspaces) {
        if (
          !presentation?.workspaceId ||
          presentation.kind !== 'tool' ||
          presentation.lifetime !== 'persistent' ||
          !presentation.layout
        ) {
          continue;
        }

        this.presentations.set(
          presentation.workspaceId,
          presentation,
        );
      }
    } catch {
      // Corrupt sidecar persistence must never prevent the file manager opening.
    }
  }

  private persistPersistentTools(): boolean {
    try {
      const envelope: PersistedToolWorkspaceEnvelope = {
        version: 1,
        workspaces: this.listPersistentTools(),
      };
      const serialized = JSON.stringify(envelope);

      if (serialized.length > MAX_PERSISTED_BYTES) {
        return false;
      }

      localStorage.setItem(
        PERSISTED_TOOL_WORKSPACES_KEY,
        serialized,
      );
      return true;
    } catch {
      return false;
    }
  }
}
