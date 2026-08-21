export interface PaneNavigation {
  refresh(): Promise<void>;
  focusContent(): void;
  focusLocation(): void;
  focusFilter(): void;
  goBack(): void;
  goForward(): void;
  navigateUp(): void;
  navigateTo(path: string): void | Promise<void>;
  navigateLinkedLocation(path: string, options?: unknown): Promise<unknown>;
  canNavigateToLocation(path: string): boolean | Promise<boolean>;
  currentLocationAvailable(): Promise<boolean>;
  jumpNavigationHistory(path: string): Promise<unknown>;
}

export interface PaneTabs {
  closeActiveTab(): void;
  newTab(): void;
  nextTab(): void;
  previousTab(): void;
  jumpToTab(index: number): void;
  closeOtherTabs(): void;
  closeTabsToRight(): void;
  toggleActiveTabPin(): void;
  exportActiveTabId(): string | null;
  exportTabCount(): number;
  restoreWorkspaceState(): Promise<void>;
  reloadWorkspaceTabs(preferredTabId?: string): Promise<unknown>;
}
