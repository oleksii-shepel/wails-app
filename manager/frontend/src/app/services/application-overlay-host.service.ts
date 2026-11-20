import { Injectable } from '@angular/core';

export type AppOverlayToolsPage = 'operations' | 'disk' | 'duplicates';
export type AppOverlayHistoryMode = 'pane' | 'all' | 'frequent';
export interface KeyboardHelpRequest {
  contextIds?: readonly string[];
  includeGlobal?: boolean;
}

export interface ApplicationOverlayHost {
  openCommandPalette(commands: readonly unknown[]): void;
  openGlobalSearch(rootPath: string): void;
  closeGlobalSearch(): void;
  openTools(page: AppOverlayToolsPage, rootPath: string): void;
  showSelectionBasket(): void;
  openWorkspaceSwitcher(): void;
  createWorkspace(): void;
  openPathHistory(mode: AppOverlayHistoryMode): void;
  openQuickAccess(sectionId?: string): void;
  openRemoteConnections(): void;
  showKeyboardHelp(request?: KeyboardHelpRequest): void;
  toggleInputCenter(): void;
  showOperationJournal(): void;
  showClosedTabs(): void;
  closeClosedTabs(): void;
  openDiskUsage(rootPath: string): void;
  openAbout(): void;
}

@Injectable({ providedIn: 'root' })
export class ApplicationOverlayHostService {
  private host: ApplicationOverlayHost | null = null;

  bind(host: ApplicationOverlayHost): void {
    this.host = host;
  }

  unbind(host: ApplicationOverlayHost): void {
    if (this.host === host) this.host = null;
  }

  openCommandPalette(commands: readonly unknown[]): void { this.host?.openCommandPalette(commands); }
  openGlobalSearch(rootPath: string): void { this.host?.openGlobalSearch(rootPath); }
  closeGlobalSearch(): void { this.host?.closeGlobalSearch(); }
  openTools(page: AppOverlayToolsPage, rootPath: string): void { this.host?.openTools(page, rootPath); }
  showSelectionBasket(): void { this.host?.showSelectionBasket(); }
  openWorkspaceSwitcher(): void { this.host?.openWorkspaceSwitcher(); }
  createWorkspace(): void { this.host?.createWorkspace(); }
  openPathHistory(mode: AppOverlayHistoryMode): void { this.host?.openPathHistory(mode); }
  openQuickAccess(sectionId?: string): void { this.host?.openQuickAccess(sectionId); }
  openRemoteConnections(): void { this.host?.openRemoteConnections(); }
  showKeyboardHelp(request?: KeyboardHelpRequest): void { this.host?.showKeyboardHelp(request); }
  toggleInputCenter(): void { this.host?.toggleInputCenter(); }
  showOperationJournal(): void { this.host?.showOperationJournal(); }
  showClosedTabs(): void { this.host?.showClosedTabs(); }
  closeClosedTabs(): void { this.host?.closeClosedTabs(); }
  openDiskUsage(rootPath: string): void { this.host?.openDiskUsage(rootPath); }
  openAbout(): void { this.host?.openAbout(); }
}
