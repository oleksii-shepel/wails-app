import { normalizePath } from './address-bar.utils';
import type { TabInfo } from '../shared/protocol-enhanced';

export interface FilePaneFocusHost {
  get currentTabId(): string;
  get paneId(): 'left' | 'right';
  get selectionCursorPath(): string | null;
  set selectionCursorPath(value: string | null);
  get selectionAnchorPath(): string | null;
  set selectionAnchorPath(value: string | null);
  get selectedFiles(): Set<string>;
  getSelectableEntries(): Array<{ path: string }>;
  scrollToCursorPath(path: string): void;
  emitPreviewTargetFromState(): void;
  getWorkspaceTabs(): TabInfo[];
  onTabChange(tab: TabInfo): Promise<void>;
  focusAddressBar(): void;
  openFindWidget(): void;
  toggleFindWidget(): void;
  closeFindWidget(): void;
}

export class FilePaneFocusController {
  ensureCursorVisibleOnActivation(host: FilePaneFocusHost): void {
    if (host.selectedFiles.size > 0 || host.selectionCursorPath) {
      return;
    }

    const selectable = host.getSelectableEntries();
    if (!selectable.length) {
      return;
    }

    host.selectionCursorPath = selectable[0].path;
    host.selectionAnchorPath = selectable[0].path;
    host.scrollToCursorPath(selectable[0].path);
  }

  focusCursorPath(host: FilePaneFocusHost, path: string): void {
    if (!path) return;

    const normalized = normalizePath(path);
    const selectable = host.getSelectableEntries();
    const index = selectable.findIndex(entry => normalizePath(entry.path) === normalized);
    if (index < 0) return;

    host.selectionCursorPath = selectable[index].path;
    host.selectionAnchorPath = host.selectionCursorPath;
    host.scrollToCursorPath(host.selectionCursorPath);
    host.emitPreviewTargetFromState();
  }

  async activateDirTabAndFocusPath(host: FilePaneFocusHost, tabId: string, focusPath: string): Promise<void> {
    if (!tabId) return;

    const tabs = host.getWorkspaceTabs();
    const tab = tabs.find(candidate => candidate.id === tabId);
    if (!tab || (tab.kind ?? 'dir') !== 'dir') {
      return;
    }

    await host.onTabChange(tab);
    this.focusCursorPath(host, focusPath);
  }

  focusAddressBar(host: FilePaneFocusHost): void {
    host.focusAddressBar();
  }

  focusFind(host: FilePaneFocusHost): void {
    host.openFindWidget();
  }

  focusFilter(host: FilePaneFocusHost): void {
    host.toggleFindWidget();
  }

  clearFilter(host: FilePaneFocusHost): void {
    host.closeFindWidget();
  }
}