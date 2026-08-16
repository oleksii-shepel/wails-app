export interface ReassemblyPluginContributionRestoration {
  viewerRegistryShared: boolean;
  fileCategoriesVisible: boolean;
  relatedLocationsVisible: boolean;
  onCommandActivation: boolean;
  onSchemeActivation: boolean;
  onSurfaceActivation: boolean;
}

export function missingContributionRestoration(
  state: ReassemblyPluginContributionRestoration,
): string[] {
  return Object.entries(state)
    .filter(([, restored]) => !restored)
    .map(([feature]) => feature);
}
