import type { ChangeDetectorRef, SimpleChanges } from '@angular/core';
import type { ViewLayout } from './file-view-container.component';

export interface FileViewLifecycleHost {
  get currentView(): ViewLayout;
  sortEntries(): void;
  syncTreeToCurrentPath(): void;
  resortTreeNodes(): void;
  markTreeNodesChanged(): void;
  markForCheck(): void;
}

export class FileViewLifecycleController {
  onChanges(host: FileViewLifecycleHost, changes: SimpleChanges): void {
    host.sortEntries();

    const switchedToTree = !!changes['currentView'] && host.currentView === 'tree';
    const treeRelevantChange = switchedToTree || !!changes['currentPath'] || !!changes['showHidden'];
    const treeSortChanged = host.currentView === 'tree' &&
      (!!changes['sortField'] || !!changes['sortDirection'] || !!changes['directoriesFirst']);

    if (host.currentView === 'tree' && treeRelevantChange) {
      host.syncTreeToCurrentPath();
      return;
    }

    if (treeSortChanged) {
      host.resortTreeNodes();
      host.markTreeNodesChanged();
      host.markForCheck();
    }
  }
}