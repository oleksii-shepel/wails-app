import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

import type { WorkspaceConfig } from '../shared/protocol-enhanced';
import {
  fileWorkspacePresentation,
  type WorkspaceLayout,
  type WorkspacePresentation,
  type WorkspaceSurface,
} from './workspace-surface.model';

@Injectable({ providedIn: 'root' })
export class WorkspaceSurfaceService {
  private readonly presentations = new Map<string, WorkspacePresentation>();
  private readonly activeSubject =
    createBehaviorSubject<WorkspacePresentation | null>(null);

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
    if (this.activeSubject.value?.workspaceId === workspaceId) {
      this.activeSubject.next(pinned);
    }
    return pinned;
  }

  get(workspaceId: string): WorkspacePresentation | null {
    return this.presentations.get(workspaceId) ?? null;
  }
}
