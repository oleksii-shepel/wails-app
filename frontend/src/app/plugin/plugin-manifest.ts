export const PLUGIN_API_VERSION = '1.0.0';
export const PLUGIN_MANIFEST_SCHEMA_VERSION = 1 as const;

export type PluginPermission =
  | 'commands'
  | 'keybindings'
  | 'viewers'
  | 'surfaces'
  | 'toolbar'
  | 'contextMenu'
  | 'fileCategories'
  | 'relatedLocations'
  | 'filesystem.read'
  | 'filesystem.write'
  | 'network'
  | 'process'
  | 'clipboard';

export type PluginActivationEvent =
  | 'onStartup'
  | 'onDemand'
  | `onCommand:${string}`
  | `onScheme:${string}`
  | `onSurface:${string}`;

export interface PluginManifest {
  schemaVersion: typeof PLUGIN_MANIFEST_SCHEMA_VERSION;
  id: string;
  name: string;
  version: string;
  description?: string;
  author?: string;
  homepage?: string;
  license?: string;
  /** Compatibility range for the host plugin API, e.g. ^1.0.0 or 1.x. */
  apiVersion?: string;
  permissions?: readonly PluginPermission[];
  activationEvents?: readonly PluginActivationEvent[];
  contributes?: {
    schemes?: readonly string[];
    extensions?: readonly string[];
    commands?: readonly string[];
    surfaces?: readonly string[];
    viewers?: readonly string[];
    relatedLocations?: readonly string[];
  };
}

export interface PluginManifestValidationResult {
  valid: boolean;
  errors: string[];
  warnings: string[];
}

const PLUGIN_ID = /^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$/;
const SEMVER = /^(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-[0-9A-Za-z.-]+)?$/;

export function validatePluginManifest(
  manifest: PluginManifest,
  hostApiVersion = PLUGIN_API_VERSION,
): PluginManifestValidationResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  if (manifest.schemaVersion !== PLUGIN_MANIFEST_SCHEMA_VERSION) {
    errors.push(`Unsupported plugin manifest schema ${manifest.schemaVersion}.`);
  }
  if (!manifest.id?.trim() || !PLUGIN_ID.test(manifest.id)) {
    errors.push('Plugin id must be a lowercase identifier containing letters, digits, dot, dash, or underscore.');
  }
  if (!manifest.name?.trim()) errors.push('Plugin name is required.');
  if (!manifest.version?.trim() || !SEMVER.test(manifest.version)) {
    errors.push(`Plugin ${manifest.id || '<unknown>'} has an invalid semantic version.`);
  }
  if (manifest.apiVersion && !isPluginApiCompatible(manifest.apiVersion, hostApiVersion)) {
    errors.push(
      `Plugin ${manifest.id || '<unknown>'} requires plugin API ${manifest.apiVersion}, host provides ${hostApiVersion}.`,
    );
  }

  const permissions = manifest.permissions ?? [];
  if (new Set(permissions).size !== permissions.length) {
    warnings.push('Plugin manifest contains duplicate permissions.');
  }

  return { valid: errors.length === 0, errors, warnings };
}

/**
 * Intentionally small compatibility matcher for the plugin API contract.
 * Supported forms: *, 1, 1.x, 1.2, 1.2.x, 1.2.3, ^1.2.3.
 * We can swap this for a full semver implementation later without changing manifests.
 */
export function isPluginApiCompatible(range: string, current: string): boolean {
  const wanted = range.trim();
  if (!wanted || wanted === '*') return true;

  const currentVersion = parseVersion(current);
  if (!currentVersion) return false;

  if (wanted.startsWith('^')) {
    const base = parseVersion(wanted.slice(1));
    if (!base) return false;
    if (base.major !== currentVersion.major) return false;
    return compareVersion(currentVersion, base) >= 0;
  }

  const wildcard = wanted.match(/^(\d+)(?:\.(\d+|x|\*))?(?:\.(\d+|x|\*))?$/i);
  if (wildcard) {
    const major = Number(wildcard[1]);
    if (currentVersion.major !== major) return false;
    const minor = wildcard[2];
    if (minor && minor !== 'x' && minor !== '*' && currentVersion.minor !== Number(minor)) return false;
    const patch = wildcard[3];
    if (patch && patch !== 'x' && patch !== '*' && currentVersion.patch !== Number(patch)) return false;
    return true;
  }

  const exact = parseVersion(wanted);
  return !!exact && compareVersion(currentVersion, exact) === 0;
}

interface ParsedVersion { major: number; minor: number; patch: number }

function parseVersion(value: string): ParsedVersion | null {
  const match = value.trim().match(SEMVER);
  if (!match) return null;
  return {
    major: Number(match[1]),
    minor: Number(match[2] ?? 0),
    patch: Number(match[3] ?? 0),
  };
}

function compareVersion(a: ParsedVersion, b: ParsedVersion): number {
  return a.major - b.major || a.minor - b.minor || a.patch - b.patch;
}