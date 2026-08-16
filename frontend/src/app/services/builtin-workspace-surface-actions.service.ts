import { Injectable } from '@angular/core';

import { ApiService } from './api.service';
import { ConfirmDialogService } from './confirm-dialog.service';
import { WorkspaceSurfaceActionRegistryService } from './workspace-surface-action-registry.service';
import type {
  WorkspaceSurfaceActionContext,
  WorkspaceSurfaceActionContribution,
} from './workspace-surface-action.types';
import { FileType } from '../shared/protocol';

@Injectable({ providedIn: 'root' })
export class BuiltinWorkspaceSurfaceActionsService {
  private registered = false;

  constructor(
    private readonly registry: WorkspaceSurfaceActionRegistryService,
    private readonly api: ApiService,
    private readonly confirmDialog: ConfirmDialogService,
  ) {}

  register(): void {
    if (this.registered) return;
    this.registered = true;

    for (const contribution of this.contributions()) {
      this.registry.register(contribution);
    }
  }

  private contributions(): readonly WorkspaceSurfaceActionContribution[] {
    return [
      {
        id: 'builtin.workspace.search.open',
        surfaceId: 'search-workspace',
        action: 'open',
        execute: context => this.openSearchResult(context),
      },
      {
        id: 'builtin.workspace.search.reveal',
        surfaceId: 'search-workspace',
        action: 'reveal',
        execute: context => this.revealSearchResult(context),
      },
      {
        id: 'builtin.workspace.search.copyPath',
        surfaceId: 'search-workspace',
        action: 'copy-path',
        execute: context => this.copySearchPath(context),
      },
      {
        id: 'builtin.workspace.compare.copyLeftRight',
        surfaceId: 'compare-workspace',
        action: 'copy-left-right',
        execute: context => this.copyMissing(context, true),
      },
      {
        id: 'builtin.workspace.compare.copyRightLeft',
        surfaceId: 'compare-workspace',
        action: 'copy-right-left',
        execute: context => this.copyMissing(context, false),
      },
      {
        id: 'builtin.workspace.details.reveal',
        surfaceId: 'details-workspace',
        action: 'reveal',
        execute: context => this.revealDetails(context),
      },
      {
        id: 'builtin.workspace.details.rename',
        surfaceId: 'details-workspace',
        action: 'rename',
        execute: context => this.renameDetails(context),
      },
      {
        id: 'builtin.workspace.details.modified',
        surfaceId: 'details-workspace',
        action: 'modified-time',
        execute: context => this.modifiedTime(context),
      },
      {
        id: 'builtin.workspace.details.attributes',
        surfaceId: 'details-workspace',
        action: 'attributes',
        execute: context => this.attributes(context),
      },
    ];
  }

  private async openSearchResult(
    context: WorkspaceSurfaceActionContext,
  ): Promise<void> {
    const entry = context.event.payload as any;
    if (!entry?.path) return;

    try {
      if (entry.type === FileType.DIRECTORY) {
        context.services.closeToolWorkspace();
        const pane = context.services.getActivePane() as {
          navigateTo?(path: string): void | Promise<void>;
        } | null;

        await pane?.navigateTo?.(entry.path);
      } else {
        await this.api.openFile(entry.path);
      }
    } catch (error) {
      await this.showError(error);
    }
  }

  private async revealSearchResult(
    context: WorkspaceSurfaceActionContext,
  ): Promise<void> {
    const entry = context.event.payload as any;
    if (!entry?.path) return;

    try {
      await this.api.revealInFileManager(entry.path);
    } catch (error) {
      await this.showError(error);
    }
  }

  private async copySearchPath(
    context: WorkspaceSurfaceActionContext,
  ): Promise<void> {
    const entry = context.event.payload as any;
    if (!entry?.path) return;

    try {
      await navigator.clipboard.writeText(entry.path);
    } catch (error) {
      await this.showError(error);
    }
  }

  private async copyMissing(
    context: WorkspaceSurfaceActionContext,
    leftToRight: boolean,
  ): Promise<void> {
    const surface = context.event.surface;
    const leftPath = String(surface.state?.['leftPath'] ?? '');
    const rightPath = String(surface.state?.['rightPath'] ?? '');
    const rows = Array.isArray(surface.state?.['rows'])
      ? (surface.state?.['rows'] as any[])
      : [];

    if (!leftPath || !rightPath) return;

    const candidates = rows.filter(row =>
      leftToRight
        ? row.status === 'only-left' || row.status === 'onlyLeft'
        : row.status === 'only-right' || row.status === 'onlyRight',
    );

    if (candidates.length === 0) {
      await this.confirmDialog.alert({
        title: 'Compare Workspace',
        message: leftToRight
          ? 'There are no items that exist only on the left.'
          : 'There are no items that exist only on the right.',
      });
      return;
    }

    const confirmed = await this.confirmDialog.confirm({
      title: leftToRight ? 'Copy Missing →' : '← Copy Missing',
      message: `Copy ${candidates.length} missing item${
        candidates.length === 1 ? '' : 's'
      }?`,
      confirmText: 'Copy',
    });
    if (!confirmed) return;

    context.services.patchSurface(surface.id, {
      busy: true,
      error: '',
    });

    try {
      const destinationRoot = leftToRight ? rightPath : leftPath;
      const items = candidates.map(row => ({
        source: leftToRight ? row.leftPath : row.rightPath,
        destination: destinationRoot,
        exactDestination: false,
      }));

      const result = await this.api.batchCopy(items, true);
      if (result.failed > 0) {
        throw new Error(
          `Copy completed with ${result.failed} failure${
            result.failed === 1 ? '' : 's'
          }.`,
        );
      }

      await context.services.refreshSurface(surface);
    } catch (error) {
      context.services.patchSurface(surface.id, {
        busy: false,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async revealDetails(
    context: WorkspaceSurfaceActionContext,
  ): Promise<void> {
    const path = String(context.event.surface.state?.['path'] ?? '');
    if (!path) return;

    try {
      await this.api.revealInFileManager(path);
    } catch (error) {
      await this.showError(error);
    }
  }

  private async renameDetails(
    context: WorkspaceSurfaceActionContext,
  ): Promise<void> {
    const surface = context.event.surface;
    const path = String(surface.state?.['path'] ?? '');
    if (!path) return;

    try {
      const info = await this.api.getFileInfo(path);
      const name = await this.confirmDialog.prompt({
        title: `Rename — ${info.name}`,
        message: 'New name:',
        confirmText: 'Rename',
        defaultValue: info.name,
        historyKey: 'reassembly.workspace.details.rename',
      });

      if (!name || name === info.name) return;

      const parent = path.replace(/[\\/][^\\/]+$/, '');
      const destination = await this.api.joinPaths([
        parent,
        String(name).trim(),
      ]);

      const result = await this.api.batchMove(
        [{
          source: path,
          destination,
          exactDestination: true,
        }],
        true,
      );

      if (result.failed > 0 || result.skipped > 0) {
        throw new Error('Rename failed.');
      }

      context.services.patchSurface(surface.id, {
        path: destination,
      });

      const refreshed = {
        ...surface,
        state: {
          ...(surface.state ?? {}),
          path: destination,
        },
      };

      await context.services.refreshSurface(refreshed);
    } catch (error) {
      context.services.patchSurface(surface.id, {
        busy: false,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async modifiedTime(
    context: WorkspaceSurfaceActionContext,
  ): Promise<void> {
    const surface = context.event.surface;
    const path = String(surface.state?.['path'] ?? '');
    if (!path) return;

    try {
      const value = await this.confirmDialog.prompt({
        title: 'Modified Time',
        message: 'Enter YYYY-MM-DDTHH:mm:ss or "now".',
        confirmText: 'Apply',
        defaultValue: new Date().toISOString().slice(0, 19),
        historyKey: 'reassembly.workspace.details.modified',
      });

      if (!value) return;

      const raw = String(value).trim();
      const timestamp =
        raw.toLowerCase() === 'now'
          ? Date.now()
          : new Date(raw).getTime();

      if (!Number.isFinite(timestamp)) {
        throw new Error('Invalid date/time.');
      }

      await this.api.setModifiedTime(path, timestamp);
      await context.services.refreshSurface(surface);
    } catch (error) {
      context.services.patchSurface(surface.id, {
        busy: false,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async attributes(
    context: WorkspaceSurfaceActionContext,
  ): Promise<void> {
    const surface = context.event.surface;
    const path = String(surface.state?.['path'] ?? '');
    if (!path) return;

    try {
      const [osInfo, attributes] = await Promise.all([
        this.api.getOSInfo(),
        this.api.getPathAttributes(path),
      ]);

      if (String(osInfo?.os).toLowerCase() === 'windows') {
        const current = [
          attributes.readOnly ? 'readonly' : '',
          attributes.hidden ? 'hidden' : '',
          attributes.system ? 'system' : '',
          attributes.archive ? 'archive' : '',
        ].filter(Boolean).join(',');

        const value = await this.confirmDialog.prompt({
          title: 'Attributes',
          message: 'Comma-separated: readonly, hidden, system, archive',
          confirmText: 'Apply',
          defaultValue: current,
          historyKey: 'reassembly.workspace.details.attributes',
        });

        if (value === null || value === undefined) return;

        const tokens = new Set(
          String(value)
            .split(',')
            .map(token => token.trim().toLowerCase())
            .filter(Boolean),
        );

        await this.api.setPathAttributes(path, {
          readOnly: tokens.has('readonly'),
          hidden: tokens.has('hidden'),
          system: tokens.has('system'),
          archive: tokens.has('archive'),
        });
      } else {
        const currentMode = (attributes.unixMode || 0)
          .toString(8)
          .padStart(4, '0');

        const value = await this.confirmDialog.prompt({
          title: 'Permissions',
          message: 'Unix permission mode, e.g. 0644 or 0755:',
          confirmText: 'Apply',
          defaultValue: currentMode,
          historyKey: 'reassembly.workspace.details.permissions',
        });

        if (!value) return;

        const normalized = String(value).trim();
        if (!/^[0-7]{3,4}$/.test(normalized)) {
          throw new Error('Invalid Unix permission mode.');
        }

        await this.api.setPathAttributes(path, {
          unixMode: Number.parseInt(normalized, 8),
        });
      }

      await context.services.refreshSurface(surface);
    } catch (error) {
      context.services.patchSurface(surface.id, {
        busy: false,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async showError(error: unknown): Promise<void> {
    await this.confirmDialog.alert({
      title: 'Workspace Action Failed',
      message: error instanceof Error ? error.message : String(error),
    });
  }
}
