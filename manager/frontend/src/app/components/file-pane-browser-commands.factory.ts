import type { FilePaneFocusController, FilePaneFocusHost } from './file-pane-focus.controller';
import type { FilePaneNavigationController, FilePaneNavigationHost } from './file-pane-navigation.controller';
import type { FilePaneOpenController, FilePaneOpenHost } from './file-pane-open.controller';
import type { FilePaneOperationsController, FilePaneOperationsHost } from './file-pane-operations.controller';
import type { FilePanePreviewController, FilePanePreviewHost } from './file-pane-preview.controller';
import type { FilePanePreviewRequest } from './file-pane-preview.controller';
import type { FilePaneStateRuntimeAdapter } from './file-pane-state-runtime.factory';
import type { ViewerFileListItem } from '../shared/protocol-enhanced';
import type { SurfaceCapability } from '../surfaces/surface-contribution';
import type { SourceCapabilities } from '../services/source-capabilities';

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
    getTargetRequest(): FilePanePreviewRequest | null;
    focusCursorPath(path: string): void;
    activateDirTabAndFocusPath(tabId: string, focusPath: string): Promise<void>;
    openQuickView(): Promise<void>;
    zoomInActiveViewer(): void;
    zoomOutActiveViewer(): void;
    resetActiveViewerZoom(): void;
    openFrozenPreviewTab(
      targetPath: string,
      options?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; autoFollow?: boolean; fileList?: ViewerFileListItem[]; archiveContext?: import('../viewers/viewer-archive-context').ViewerArchiveContext | null },
    ): Promise<void>;
  };
}

export interface FilePaneBrowserCommandsSource {
  surfaceCapabilities: ReadonlySet<SurfaceCapability>;
  sourceCapabilities: SourceCapabilities;
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
    const supports = (capability: SurfaceCapability) => source.surfaceCapabilities.has(capability);
    const canMutate = (capability: 'create' | 'delete' | 'rename') => source.sourceCapabilities[capability];
    const navigateUp = () => supports('navigate')
      ? source.navigationController.onNavigateUp(source.navigationHost)
      : Promise.resolve();

    return {
      navigation: {
        navigateUp,
        refresh: () => { if (supports('navigate')) source.navigationController.onRefresh(source.navigationHost); },
        openSelectedItem: () => { if (supports('selection')) source.openController.onOpenSelectedItem(source.openHost, navigateUp); },
      },
      selection: {
        createFolder: () => supports('fileOperations') && canMutate('create') ? source.operationsController.onCreateFolder(source.operationsHost) : Promise.resolve(),
        deleteSelected: () => supports('fileOperations') && canMutate('delete') ? source.operationsController.onDeleteSelected(source.operationsHost) : Promise.resolve(),
        renameSelected: () => supports('fileOperations') && canMutate('rename') ? source.operationsController.onRenameSelected(source.operationsHost) : Promise.resolve(),
        copyToOther: () => { if (supports('fileOperations')) source.operationsController.onCopyToOther(source.operationsHost); },
        moveToOther: () => { if (supports('fileOperations') && canMutate('delete')) source.operationsController.onMoveToOther(source.operationsHost); },
      },
      preview: {
        getTargetPath: () => supports('preview') ? source.stateRuntime.getPreviewTargetPath() : '',
        getTargetRequest: () => supports('preview') ? source.previewController.getPreviewRequest(source.previewHost) : null,
        focusCursorPath: (path: string) => {
          if (!supports('preview')) {
            return;
          }
          source.focusController.focusCursorPath(source.focusHost, path);
        },
        activateDirTabAndFocusPath: (tabId: string, focusPath: string) =>
          source.focusController.activateDirTabAndFocusPath(source.focusHost, tabId, focusPath),
        openQuickView: () => supports('preview') ? source.previewController.openQuickView(source.previewHost) : Promise.resolve(),
        zoomInActiveViewer: () => { if (supports('viewerControls')) source.zoomInActiveViewer(); },
        zoomOutActiveViewer: () => { if (supports('viewerControls')) source.zoomOutActiveViewer(); },
        resetActiveViewerZoom: () => { if (supports('viewerControls')) source.resetActiveViewerZoom(); },
        openFrozenPreviewTab: (targetPath: string, options?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; autoFollow?: boolean }) =>
          source.previewController.openFrozenPreviewTab(source.previewHost, targetPath, options),
      },
    };
  }
}