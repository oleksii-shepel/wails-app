export interface ReassemblyPluginRestorationState {
  readonly bootstrapConnected: boolean;
  readonly liveSelectionConnected: boolean;
  readonly commandPaletteConnected: boolean;
  readonly toolbarConnected: boolean;
  readonly contextMenuConnected: boolean;
}

export function auditReassemblyPluginRestoration(
  state: ReassemblyPluginRestorationState,
): string[] {
  return Object.entries(state)
    .filter(([, connected]) => !connected)
    .map(([name]) => name);
}
