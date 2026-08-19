export interface PluginManagementRestoration {
  settingsMounted: boolean;
  policyControlsReachable: boolean;
  genericSurfaceTabsSupported: boolean;
  surfaceTabsPersisted: boolean;
  surfaceLaunchersDiscoverable: boolean;
}

export function missingPluginManagementFeatures(
  state: PluginManagementRestoration,
): string[] {
  return Object.entries(state)
    .filter(([, restored]) => !restored)
    .map(([feature]) => feature);
}