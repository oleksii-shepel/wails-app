import type { WorkspaceService } from '../services/workspace.service';
import type { TabInfo } from '../shared/protocol-enhanced';
import type { FilePaneBrowserCommands } from './file-pane-browser-commands.factory';
import type { ViewerArchiveContext } from '../viewers/viewer-archive-context';

export interface FileBrowserPreviewPane {
  currentTabId: string;
  currentPath: string;
  browserCommands: Pick<FilePaneBrowserCommands, 'preview'>;
}

interface FileBrowserPreviewRequest {
	path: string;
	archiveContext: ViewerArchiveContext | null;
}

export interface FileBrowserPreviewHost {
  workspaceService: WorkspaceService;
  getActivePaneId(): 'left' | 'right';
  setActivePaneId(paneId: 'left' | 'right'): void;
  getActivePane(): FileBrowserPreviewPane | undefined;
  getLeftPane(): FileBrowserPreviewPane | undefined;
  getRightPane(): FileBrowserPreviewPane | undefined;
}

export class FileBrowserPreviewController {
  private lastCrossPreviewTarget: string | null = null;
  private lastPreviewTabFocus: string | null = null;
  private restoringFromPreview = false;

  async openFrozenPreviewInOtherPane(host: FileBrowserPreviewHost): Promise<void> {
    const fromPane = host.getActivePane();
    if (!fromPane) {
      return;
    }

    const request = fromPane.browserCommands.preview.getTargetRequest() as FileBrowserPreviewRequest | null;
    if (!request?.path) {
      return;
    }

    const toPaneId: 'left' | 'right' = host.getActivePaneId() === 'left' ? 'right' : 'left';
    const toPane = toPaneId === 'left' ? host.getLeftPane() : host.getRightPane();
    if (!toPane) {
      return;
    }

    await toPane.browserCommands.preview.openFrozenPreviewTab(request.path, {
      sourcePane: host.getActivePaneId(),
      sourceTabId: fromPane.currentTabId,
      sourceDir: fromPane.currentPath,
      autoFollow: true,
      archiveContext: request.archiveContext,
    });
  }

  switchActivePane(host: FileBrowserPreviewHost): void {
    host.setActivePaneId(host.getActivePaneId() === 'left' ? 'right' : 'left');
  }

  onPanePreviewTarget(host: FileBrowserPreviewHost, fromPane: 'left' | 'right', targetPath: string): void {
    if (fromPane !== host.getActivePaneId()) {
      return;
    }

    const otherPane: 'left' | 'right' = fromPane === 'left' ? 'right' : 'left';
    const next = targetPath || '';

    const sourcePane = fromPane === 'left' ? host.getLeftPane() : host.getRightPane();
    const sourceTabId = sourcePane?.currentTabId ?? '';
    const sourceDir = sourcePane?.currentPath ?? '';

    // If the active tab in the same pane is a viewer and set to auto-follow,
    // update it directly so the viewer reloads when the cursor moves.
    const sameActive = host.workspaceService.getActiveTab(fromPane);
    const sameSourceTabId = sourcePane?.currentTabId ?? '';
    const sameDedupeKey = `${fromPane}:${next}:${fromPane}:${sameSourceTabId}`;
    if (sameActive && (sameActive.kind ?? 'dir') === 'viewer' && sameActive.viewer?.autoFollow) {
      if (this.lastCrossPreviewTarget === sameDedupeKey) return;
      this.lastCrossPreviewTarget = sameDedupeKey;

      host.workspaceService.updateViewerTabTargetPath(fromPane, sameActive.id, next, {
        sourcePane: fromPane,
        sourceTabId: sameSourceTabId,
        sourceDir,
      });
      return;
    }

    const dedupeKey = `${otherPane}:${next}:${fromPane}:${sourceTabId}`;
    if (this.lastCrossPreviewTarget === dedupeKey) {
      return;
    }
    this.lastCrossPreviewTarget = dedupeKey;

    const active = host.workspaceService.getActiveTab(otherPane);
    if (!active || (active.kind ?? 'dir') !== 'viewer' || !active.viewer?.autoFollow) {
      return;
    }

    host.workspaceService.updateViewerTabTargetPath(otherPane, active.id, next, {
      sourcePane: fromPane,
      sourceTabId,
      sourceDir,
    });
  }

  onPaneActiveTabChanged(host: FileBrowserPreviewHost, fromPane: 'left' | 'right', tab: TabInfo): void {
    if (!tab || (tab.kind ?? 'dir') !== 'viewer') {
      return;
    }

    const target = tab.viewer?.targetPath ?? '';
    if (!target) {
      return;
    }

    const sourcePane = tab.viewer?.sourcePane;
    const sourceTabId = tab.viewer?.sourceTabId;

    if (sourcePane && sourceTabId) {
      const key = `${sourcePane}:${sourceTabId}:${target}`;
      if (this.lastPreviewTabFocus === key) {
        return;
      }
      this.lastPreviewTabFocus = key;

      const pane = sourcePane === 'left' ? host.getLeftPane() : host.getRightPane();
      if (!pane || this.restoringFromPreview) {
        return;
      }

      this.restoringFromPreview = true;
      pane.browserCommands.preview.activateDirTabAndFocusPath(sourceTabId, target).finally(() => {
        this.restoringFromPreview = false;
      });
      return;
    }

    const otherPane: 'left' | 'right' = fromPane === 'left' ? 'right' : 'left';
    const key = `${otherPane}:${target}`;
    if (this.lastPreviewTabFocus === key) {
      return;
    }
    this.lastPreviewTabFocus = key;

    const pane = otherPane === 'left' ? host.getLeftPane() : host.getRightPane();
    pane?.browserCommands.preview.focusCursorPath(target);
  }
}