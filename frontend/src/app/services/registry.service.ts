import { Injectable } from '@angular/core';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from '../components/file-entry.component';

/* ─────────────────────────────────────────────────────────────
   TYPES
───────────────────────────────────────────────────────────── */

/** The VFS root the backend mounts the registry under. */
export const REGISTRY_ROOT = 'registry://';
export const REGISTRY_VFS_ROOT = '/registry';

/** Registry path separator (always forward slash in the VFS). */
const SEP = '/';

export interface RegistryFrame {
  /** Current public registry path, e.g. "registry://HKEY_LOCAL_MACHINE/SOFTWARE/Microsoft" */
  path: string;
  entries: FileInfo[];
}

/* ─────────────────────────────────────────────────────────────
   WAILS BRIDGE
───────────────────────────────────────────────────────────── */

declare const window: any;

function app(): any {
  const a = window.go?.app?.App;
  if (!a) throw new Error('Wails bridge not available');
  return a;
}

/* ─────────────────────────────────────────────────────────────
   PATH HELPERS
───────────────────────────────────────────────────────────── */

function normalizeRegistryPath(path: string): string {
  const normalized = (path || '').trim().replace(/\\/g, '/');
  const match = /^registry:\/\/(.*)?$/i.exec(normalized);
  if (!match) return normalized;
  const rest = (match[1] || '').replace(/^\/+/, '').replace(/\/+/g, '/');
  return rest ? `${REGISTRY_ROOT}${rest}` : REGISTRY_ROOT;
}

export function registryToVfsPath(path: string): string {
  const normalized = normalizeRegistryPath(path);
  if (!isRegistryPath(normalized)) return normalized;
  const rest = normalized.slice(REGISTRY_ROOT.length).replace(/^\/+/, '');
  return rest ? `${REGISTRY_VFS_ROOT}/${rest}` : REGISTRY_VFS_ROOT;
}

export function registryFromVfsPath(path: string): string {
  const normalized = (path || '').trim().replace(/\\/g, '/');
  if (normalized === REGISTRY_VFS_ROOT) return REGISTRY_ROOT;
  if (normalized.startsWith(REGISTRY_VFS_ROOT + SEP)) {
    return `${REGISTRY_ROOT}${normalized.slice(REGISTRY_VFS_ROOT.length + 1)}`;
  }
  return normalizeRegistryPath(normalized);
}

/** True if the given public path is inside the registry root. */
export function isRegistryPath(path: string): boolean {
  const normalized = normalizeRegistryPath(path);
  return normalized === REGISTRY_ROOT || normalized.startsWith(REGISTRY_ROOT);
}

/**
 * Returns the display path shown in the pane breadcrumb.
 * Strips the "/registry" prefix so users see "HKEY_LOCAL_MACHINE\SOFTWARE\…".
 */
export function registryDisplayPath(path: string): string {
  const normalized = normalizeRegistryPath(path);
  const rel = normalized.startsWith(REGISTRY_ROOT)
    ? normalized.slice(REGISTRY_ROOT.length)
    : normalized === REGISTRY_ROOT
    ? ''
    : normalized;
  return rel.replace(/\//g, '\\') || 'Registry';
}

/**
 * Builds a human-readable breadcrumb trail for a registry path.
 * e.g. ["Registry", "HKEY_LOCAL_MACHINE", "SOFTWARE", "Microsoft"]
 */
export function registryBreadcrumbs(path: string): string[] {
  const normalized = normalizeRegistryPath(path);
  if (normalized === REGISTRY_ROOT) return ['Registry'];
  const rel = normalized.slice(REGISTRY_ROOT.length);
  return ['Registry', ...rel.split(SEP).filter(Boolean)];
}

/** Parent public path, or null if already at the registry root. */
export function registryParentPath(path: string): string | null {
  const normalized = normalizeRegistryPath(path);
  if (normalized === REGISTRY_ROOT) return null;
  const rel = normalized.slice(REGISTRY_ROOT.length);
  const idx = rel.lastIndexOf(SEP);
  if (idx < 0) return REGISTRY_ROOT;
  return `${REGISTRY_ROOT}${rel.slice(0, idx)}`;
}

/* ─────────────────────────────────────────────────────────────
   ENTRY CONVERSION
───────────────────────────────────────────────────────────── */

/**
 * Converts a raw VFSListDir entry for display in the file pane.
 * Keys appear as directories; values appear as files.
 */
function toFileInfo(raw: any): FileInfo {
  return {
    name:        raw.name,
    path:        registryFromVfsPath(raw.path),
    type:        raw.type as FileType,
    size:        raw.size ?? 0,
    modified:    raw.modified ?? 0,
    created:     raw.modified ?? 0,
    accessed:    raw.modified ?? 0,
    permissions: raw.type === FileType.DIRECTORY ? 'drwxr-xr-x' : '-r--r--r--',
    isHidden:    false,
    registryKind: raw.registryKind,
    registryValueType: raw.registryValueType,
    registryPreview: raw.registryPreview,
  };
}

/** Prepends the ".." parent row to a list of entries. */
function withParent(entries: FileInfo[], hasParent: boolean): FileInfo[] {
  if (!hasParent) return entries;
  const dotdot: FileInfo = {
    name:        '..',
    path:        PARENT_ENTRY_PATH,
    type:        FileType.DIRECTORY,
    size:        0,
    modified:    0,
    created:     0,
    accessed:    0,
    permissions: 'drwxr-xr-x',
    isHidden:    false,
  };
  return [dotdot, ...entries];
}

/* ─────────────────────────────────────────────────────────────
   SERVICE
───────────────────────────────────────────────────────────── */

@Injectable({ providedIn: 'root' })
export class RegistryService {

  /* ─────────────── NAVIGATION ─────────────── */

  /** Load the registry root (list of hives). */
  async openRoot(): Promise<RegistryFrame> {
    return this.load(REGISTRY_ROOT);
  }

  /** Navigate into a subkey or stay at the frame root. */
  async navigateInto(path: string): Promise<RegistryFrame> {
    return this.load(path);
  }

  /** Navigate up one level; returns null if already at the registry root. */
  async navigateUp(current: RegistryFrame): Promise<RegistryFrame | null> {
    const parent = registryParentPath(current.path);
    if (!parent) return null;
    return this.load(parent);
  }

  /** Reload the current frame (after a write operation). */
  async refresh(frame: RegistryFrame): Promise<RegistryFrame> {
    return this.load(frame.path);
  }

  /* ─────────────── READ VALUE ─────────────── */

  /**
   * Reads the content of a registry value (a "file" in the VFS).
   * Returns the serialised string — the backend formats it based on value type:
   *   REG_SZ / REG_EXPAND_SZ  → the string
   *   REG_DWORD / REG_QWORD   → "4294967295 (0xFFFFFFFF)"
   *   REG_MULTI_SZ            → newline-separated strings
   *   REG_BINARY              → space-separated hex pairs
   */
  async readValue(path: string): Promise<string> {
    const result = await app().VFSReadFile(path);
    return typeof result === 'string' ? result : new TextDecoder().decode(result);
  }

  /* ─────────────── WRITE OPERATIONS ─────────────── */

  /** Create a new subkey (like mkdir). */
  async createKey(parentPath: string, keyName: string): Promise<void> {
    const newPath = `${normalizeRegistryPath(parentPath)}${SEP}${keyName}`;
    await app().VFSMkdir(newPath);
  }

  /**
   * Set a REG_SZ value.  Creates or overwrites.
   * For other types, write the serialised form — the backend stores as REG_SZ.
   */
  async setValue(keyPath: string, valueName: string, data: string): Promise<void> {
    const valuePath = `${normalizeRegistryPath(keyPath)}${SEP}${valueName || '(default)'}`;
    await app().VFSWriteFile(valuePath, data);
  }

  /** Delete a key or value at the given VFS path. */
  async delete(path: string): Promise<void> {
    await app().VFSRemove(normalizeRegistryPath(path));
  }

  /** Rename is not supported by the registry; always rejects. */
  rename(_oldPath: string, _newName: string): Promise<void> {
    return Promise.reject(new Error('Registry keys and values cannot be renamed.'));
  }

  /* ─────────────── INTERNAL ─────────────── */

  private async load(path: string): Promise<RegistryFrame> {
    const currentPath = normalizeRegistryPath(path);
    const listing = await app().VFSListDir(currentPath);
    const hasParent = currentPath !== REGISTRY_ROOT;
    const entries = withParent(
      (listing?.entries ?? []).map(toFileInfo),
      hasParent,
    );
    return { path: currentPath, entries };
  }
}