export interface PluginContributionRestoration {
  viewerRegistryShared: boolean;
  fileCategoriesVisible: boolean;
  relatedLocationsVisible: boolean;
  panelSourcesVisible: boolean;
  panelColumnsVisible: boolean;
  remoteProvidersPluginOwned: boolean;
  onCommandActivation: boolean;
  onSchemeActivation: boolean;
  onSurfaceActivation: boolean;
}

export function missingContributionRestoration(
  state: PluginContributionRestoration,
): string[] {
  return Object.entries(state)
    .filter(([, restored]) => !restored)
    .map(([feature]) => feature);
}