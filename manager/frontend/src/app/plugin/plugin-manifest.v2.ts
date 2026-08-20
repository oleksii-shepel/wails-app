// Merge these changes into plugin-manifest.ts.
// Kept as a standalone reference file so the patch remains reviewable.

export const PLUGIN_API_VERSION = '2.0.0';
export const PLUGIN_MANIFEST_SCHEMA_VERSION = 2 as const;

export interface PluginTarget {
  os: 'windows' | 'linux' | 'darwin';
  arch: 'x64' | 'arm64';
}

export interface PluginExecutableEntry {
  entry: string;
}

export interface PluginBackendEntry extends PluginExecutableEntry {
  protocol: number;
}

// Add to PluginPermission:
//   | 'schemes'
//   | 'dialogs'
//   | 'backend.execute'
//
// Add to PluginManifest:
//   target?: PluginTarget;
//   frontend?: PluginExecutableEntry;
//   backend?: PluginBackendEntry;
//
// For schemaVersion 2, target/frontend/backend are required.
// Keep schemaVersion 1 accepted during the transition if directory plugins remain supported.
