import type { WorkspaceService } from '../services/workspace.service';
import type { TabInfo } from '../shared/protocol-enhanced';

export interface FileBrowserPreviewPane {
  currentTabId: string;
  currentPath: string;
  getPreviewTargetPath(): string | null;
  openFrozenPreviewTab(
    targetPath: string,
    options: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; autoFollow: boolean },
  ): Promise<void>;
  activateDirTabAndFocusPath(tabId: string, path: string): Promise<void>;
  focusCursorPath(path: string): void;
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

    const target = fromPane.getPreviewTargetPath();
    if (!target) {
      return;
    }

    const toPaneId: 'left' | 'right' = host.getActivePaneId() === 'left' ? 'right' : 'left';
    const toPane = toPaneId === 'left' ? host.getLeftPane() : host.getRightPane();
    if (!toPane) {
      return;
    }

    await toPane.openFrozenPreviewTab(target, {
      sourcePane: host.getActivePaneId(),
      sourceTabId: fromPane.currentTabId,
      sourceDir: fromPane.currentPath,
      autoFollow: true,
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
      pane.activateDirTabAndFocusPath(sourceTabId, target).finally(() => {
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
    pane?.focusCursorPath(target);
  }
}