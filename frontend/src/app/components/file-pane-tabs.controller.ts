import { samePath } from './address-bar.utils';

import type { ArchiveFrame } from '../services/archive.service';
import type { WorkspaceService } from '../services/workspace.service';
import type { TabInfo, WorkspaceConfig } from '../shared/protocol-enhanced';
import type { TabDropEvent } from './tab-bar.component';

export interface FilePaneTabsHost {
  get paneId(): 'left' | 'right';
  get currentTabId(): string;
  set currentTabId(value: string);
  get currentPath(): string;
  get tabs(): TabInfo[];
  set tabs(value: TabInfo[]);
  get activeDirTabId(): string | null;
  set activeDirTabId(value: string | null);
  get archiveStack(): ArchiveFrame[];
  get fsParentDir(): string;
  get lastSeenActiveTabId(): string | null;
  set lastSeenActiveTabId(value: string | null);
  get lastSeenActiveTabPath(): string | null;
  set lastSeenActiveTabPath(value: string | null);
  workspaceService: WorkspaceService;
  isInArchive(): boolean;
  deleteDirSession(tabId: string): void;
  deleteDirTabState(tabId: string): void;
  loadPaneState(): Promise<void>;
  onTabChange(tab: TabInfo): Promise<void>;
  onTabClose(tab: TabInfo): void;
  onTabPin(tab: TabInfo): void;
}

export class FilePaneTabsController {
  onWorkspaceUpdate(host: FilePaneTabsHost, ws: WorkspaceConfig): void {
    const pane = host.paneId === 'left' ? ws.leftPane : ws.rightPane;
    const nextActiveId = pane.activeTabId;
    const activeTab = pane.tabs.find(tab => tab.id === nextActiveId);
    const nextActivePath = activeTab?.location?.location ?? activeTab?.path ?? '';

    if (host.lastSeenActiveTabId === null) {
      host.lastSeenActiveTabId = nextActiveId;
      host.lastSeenActiveTabPath = nextActivePath;
      host.tabs = host.workspaceService.getTabs(host.paneId);
      host.currentTabId = nextActiveId;
      return;
    }

    const activeChanged = nextActiveId !== host.lastSeenActiveTabId || !samePath(nextActivePath, host.lastSeenActiveTabPath);
    host.lastSeenActiveTabId = nextActiveId;
    host.lastSeenActiveTabPath = nextActivePath;

    if (!activeChanged) {
      host.tabs = host.workspaceService.getTabs(host.paneId);
      return;
    }

    // Local tab switches already ran through onTabChange/loadPaneState.
    // When workspace state echoes back through the subscription, avoid a second
    // load cycle for the same active tab because it causes a visible redraw,
    // especially for viewer tabs whose tab path differs from the pane dir path.
    if (host.currentTabId === nextActiveId) {
      host.tabs = host.workspaceService.getTabs(host.paneId);
      return;
    }

    if (host.currentTabId === nextActiveId && samePath(host.currentPath, nextActivePath)) {
      host.tabs = host.workspaceService.getTabs(host.paneId);
      return;
    }

    void host.loadPaneState();
  }

  onTabDrop(host: FilePaneTabsHost, event: TabDropEvent): void {
    if (!event) return;

    host.workspaceService.moveTab(event.fromPaneId, event.toPaneId, event.tabId, event.toIndex);

    if (event.toPaneId !== host.paneId) {
      return;
    }

    host.tabs = host.workspaceService.getTabs(host.paneId);
    const active = host.workspaceService.getActiveTab(host.paneId);
    if (active?.id !== event.tabId) {
      return;
    }

    const droppedTab = host.tabs.find(tab => tab.id === event.tabId);
    if (droppedTab) {
      void host.onTabChange(droppedTab);
    }
  }

  onTabDragStart(host: FilePaneTabsHost, tab: TabInfo): void {
    if (!tab || tab.id !== host.currentTabId) return;
    if (!host.currentTabId || !host.currentPath) return;

    if (host.isInArchive()) {
      host.workspaceService.saveArchiveState(host.paneId, host.currentTabId, {
        path: host.currentPath,
        archiveStack: host.archiveStack,
        fsDirPath: host.archiveStack[0]?.fsPath ?? '',
        fsParentDir: host.fsParentDir,
      });
      return;
    }

    host.workspaceService.updateTabPath(host.paneId, host.currentTabId, host.currentPath);
  }

  onTabClose(host: FilePaneTabsHost, tab: TabInfo): void {
    host.deleteDirSession(tab.id);
    host.deleteDirTabState(tab.id);
    if (host.activeDirTabId === tab.id) {
      host.activeDirTabId = null;
    }
    host.workspaceService.clearArchiveState(host.paneId, tab.id);
    host.workspaceService.closeTab(host.paneId, tab.id);
    void host.loadPaneState();
  }

  onTabPin(host: FilePaneTabsHost, tab: TabInfo): void {
    host.workspaceService.toggleTabPin(host.paneId, tab.id);
    void host.loadPaneState();
  }

  onNewTab(host: FilePaneTabsHost): void {
    const newTab = host.workspaceService.addTab(host.paneId, host.currentPath, true);
    if (!newTab) return;
    host.currentTabId = newTab.id;
    host.tabs = host.workspaceService.getTabs(host.paneId);
    host.workspaceService.switchTab(host.paneId, newTab.id);
    host.tabs = host.workspaceService.getTabs(host.paneId);
  }

  async openPathInNewTab(host: FilePaneTabsHost, path: string): Promise<void> {
    const newTab = host.workspaceService.addTab(host.paneId, path, true);
    if (!newTab) return;
    host.tabs = host.workspaceService.getTabs(host.paneId);
    await host.onTabChange(newTab);
  }

  closeCurrentTab(host: FilePaneTabsHost): void {
    const tab = this.getActiveTab(host);
    if (tab) host.onTabClose(tab);
  }

  nextTab(host: FilePaneTabsHost): void {
    const index = host.tabs.findIndex(tab => tab.isActive);
    if (index >= 0) {
      void host.onTabChange(host.tabs[(index + 1) % host.tabs.length]);
    }
  }

  previousTab(host: FilePaneTabsHost): void {
    const index = host.tabs.findIndex(tab => tab.isActive);
    if (index >= 0) {
      void host.onTabChange(host.tabs[index === 0 ? host.tabs.length - 1 : index - 1]);
    }
  }

  jumpToTab(host: FilePaneTabsHost, position: number): void {
    if (position > 0 && position <= host.tabs.length) {
      void host.onTabChange(host.tabs[position - 1]);
    }
  }

  togglePinCurrentTab(host: FilePaneTabsHost): void {
    const tab = this.getActiveTab(host);
    if (tab) host.onTabPin(tab);
  }

  private getActiveTab(host: FilePaneTabsHost): TabInfo | undefined {
    return host.tabs.find(tab => tab.isActive);
  }
}