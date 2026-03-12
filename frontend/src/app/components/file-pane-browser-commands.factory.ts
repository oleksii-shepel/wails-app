import type { FilePaneFocusController, FilePaneFocusHost } from './file-pane-focus.controller';
import type { FilePaneNavigationController, FilePaneNavigationHost } from './file-pane-navigation.controller';
import type { FilePaneOpenController, FilePaneOpenHost } from './file-pane-open.controller';
import type { FilePaneOperationsController, FilePaneOperationsHost } from './file-pane-operations.controller';
import type { FilePanePreviewController, FilePanePreviewHost } from './file-pane-preview.controller';
import type { FilePaneStateRuntimeAdapter } from './file-pane-state-runtime.factory';

export interface FilePaneBrowserCommands {
  navigation: {
    navigateUp(): Promise<void>;
    refresh(): void;
    openSelectedItem(): void;
  };
  selection: {
    createFolder(): Promise<void>;
    deleteSelected(): Promise<void>;
    renameSelected(): Promise<void>;
    copyToOther(): void;
    moveToOther(): void;
  };
  preview: {
    getTargetPath(): string;
    focusCursorPath(path: string): void;
    activateDirTabAndFocusPath(tabId: string, focusPath: string): Promise<void>;
    openQuickView(): Promise<void>;
    zoomInActiveViewer(): void;
    zoomOutActiveViewer(): void;
    resetActiveViewerZoom(): void;
    openFrozenPreviewTab(
      targetPath: string,
      options?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; autoFollow?: boolean },
    ): Promise<void>;
  };
}

export interface FilePaneBrowserCommandsSource {
  activeTabKind: 'dir' | 'viewer';
  navigationController: FilePaneNavigationController;
  navigationHost: FilePaneNavigationHost;
  openController: FilePaneOpenController;
  openHost: FilePaneOpenHost;
  operationsController: FilePaneOperationsController;
  operationsHost: FilePaneOperationsHost;
  previewController: FilePanePreviewController;
  previewHost: FilePanePreviewHost;
  focusController: FilePaneFocusController;
  focusHost: FilePaneFocusHost;
  stateRuntime: FilePaneStateRuntimeAdapter;
  zoomInActiveViewer(): void;
  zoomOutActiveViewer(): void;
  resetActiveViewerZoom(): void;
}

export class FilePaneBrowserCommandsFactory {
  create(source: FilePaneBrowserCommandsSource): FilePaneBrowserCommands {
    const navigateUp = () => source.navigationController.onNavigateUp(source.navigationHost);

    return {
      navigation: {
        navigateUp,
        refresh: () => source.navigationController.onRefresh(source.navigationHost),
        openSelectedItem: () => source.openController.onOpenSelectedItem(source.openHost, navigateUp),
      },
      selection: {
        createFolder: () => source.operationsController.onCreateFolder(source.operationsHost),
        deleteSelected: () => source.operationsController.onDeleteSelected(source.operationsHost),
        renameSelected: () => source.operationsController.onRenameSelected(source.operationsHost),
        copyToOther: () => source.operationsController.onCopyToOther(source.operationsHost),
        moveToOther: () => source.operationsController.onMoveToOther(source.operationsHost),
      },
      preview: {
        getTargetPath: () => source.stateRuntime.getPreviewTargetPath(),
        focusCursorPath: (path: string) => {
          if (source.activeTabKind !== 'dir') {
            return;
          }
          source.focusController.focusCursorPath(source.focusHost, path);
        },
        activateDirTabAndFocusPath: (tabId: string, focusPath: string) =>
          source.focusController.activateDirTabAndFocusPath(source.focusHost, tabId, focusPath),
        openQuickView: () => source.previewController.openQuickView(source.previewHost),
        zoomInActiveViewer: () => source.zoomInActiveViewer(),
        zoomOutActiveViewer: () => source.zoomOutActiveViewer(),
        resetActiveViewerZoom: () => source.resetActiveViewerZoom(),
        openFrozenPreviewTab: (targetPath: string, options?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; autoFollow?: boolean }) =>
          source.previewController.openFrozenPreviewTab(source.previewHost, targetPath, options),
      },
    };
  }
}
