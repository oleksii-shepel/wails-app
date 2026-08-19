import type { ConfirmDialogService } from '../services/confirm-dialog.service';
import type { WorkspaceService } from '../services/workspace.service';
import type { WorkspaceConfig } from '../shared/protocol-enhanced';

export interface FileBrowserWorkspaceHost {
  workspaceService: WorkspaceService;
  confirmDialog: ConfirmDialogService;
  getActiveWorkspace(): WorkspaceConfig | null;
  getShowHidden(): boolean;
  getShowWorkspaceMenu(): boolean;
  setShowHidden(showHidden: boolean): void;
  setShowWorkspaceMenu(show: boolean): void;
  refreshPanes(): Promise<void>;
}

export class FileBrowserWorkspaceController {
  async createNewWorkspace(host: FileBrowserWorkspaceHost): Promise<void> {
    const name = (await host.confirmDialog.prompt({
      title: 'New Workspace',
      message: 'Enter workspace name.',
      confirmText: 'Create',
      cancelText: 'Cancel',
      placeholder: 'Workspace name',
      defaultValue: '',
      historyKey: 'workspace.name',
    }))?.trim();
    if (!name) {
      return;
    }

    const workspace = await host.workspaceService.createWorkspace(name);
    host.workspaceService.switchWorkspace(workspace.id);
    host.setShowWorkspaceMenu(false);
  }

  switchWorkspace(host: FileBrowserWorkspaceHost, workspaceId: string): void {
    host.workspaceService.switchWorkspace(workspaceId);
    host.setShowWorkspaceMenu(false);
  }

  toggleWorkspaceMenu(host: FileBrowserWorkspaceHost, event?: Event): void {
    event?.stopPropagation();
    host.setShowWorkspaceMenu(!host.getShowWorkspaceMenu());
  }

  closeWorkspaceMenu(host: FileBrowserWorkspaceHost): void {
    host.setShowWorkspaceMenu(false);
  }

  async deleteCurrentWorkspace(host: FileBrowserWorkspaceHost): Promise<void> {
    const activeWorkspace = host.getActiveWorkspace();
    if (!activeWorkspace) {
      return;
    }

    const confirmed = await host.confirmDialog.confirm({
      title: 'Delete Workspace',
      message: `Delete workspace "${activeWorkspace.name}"?`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!confirmed) {
      return;
    }

    host.workspaceService.deleteWorkspace(activeWorkspace.id);
  }

  async renameCurrentWorkspace(host: FileBrowserWorkspaceHost): Promise<void> {
    const activeWorkspace = host.getActiveWorkspace();
    if (!activeWorkspace) {
      return;
    }

    const name = (await host.confirmDialog.prompt({
      title: 'Rename Workspace',
      message: 'Enter new workspace name.',
      confirmText: 'Rename',
      cancelText: 'Cancel',
      placeholder: 'Workspace name',
      defaultValue: activeWorkspace.name,
      historyKey: 'workspace.name',
    }))?.trim();
    if (!name) {
      return;
    }

    host.workspaceService.updateWorkspace(activeWorkspace.id, { name });
  }

  async toggleShowHidden(host: FileBrowserWorkspaceHost): Promise<void> {
    const nextShowHidden = !host.getShowHidden();
    host.setShowHidden(nextShowHidden);

    const activeWorkspace = host.getActiveWorkspace();
    if (activeWorkspace) {
      host.workspaceService.updateWorkspace(activeWorkspace.id, {
        showHidden: nextShowHidden,
      });
    }

    await host.refreshPanes();
  }
}