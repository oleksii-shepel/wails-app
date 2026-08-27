import type { ChangeDetectorRef, SimpleChanges } from '@angular/core';
import type { ViewLayout } from './pane-view-host.component';

export interface FileViewLifecycleHost {
  get currentView(): ViewLayout;
  sortEntries(): void;
  syncTreeToCurrentPath(): void;
  refreshCurrentTreePath(): void;
  resortTreeNodes(): void;
  markTreeNodesChanged(): void;
  markForCheck(): void;
}

export class FileViewLifecycleController {
  onChanges(host: FileViewLifecycleHost, changes: SimpleChanges): void {
    host.sortEntries();

    const switchedToTree = !!changes['currentView'] && host.currentView === 'tree';
    const treeRelevantChange = switchedToTree || !!changes['currentPath'] || !!changes['showHidden'];
    const treeEntriesChanged = host.currentView === 'tree' && !!changes['entries'];
    const treeSortChanged = host.currentView === 'tree' &&
      (!!changes['sortField'] || !!changes['sortDirection'] || !!changes['directoriesFirst']);

    if (host.currentView === 'tree' && treeRelevantChange) {
      host.syncTreeToCurrentPath();
      return;
    }

    if (treeEntriesChanged) {
      host.refreshCurrentTreePath();
      return;
    }

    if (treeSortChanged) {
      host.resortTreeNodes();
      host.markTreeNodesChanged();
      host.markForCheck();
    }
  }
}
