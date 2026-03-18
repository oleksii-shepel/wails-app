import { Injectable } from '@angular/core';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from '../components/file-entry.component';

/* ─────────────────────────────────────────────────────────────
   TYPES
───────────────────────────────────────────────────────────── */

/** The VFS root the backend mounts the registry under. */
export const REGISTRY_ROOT = '/registry';

/** Registry path separator (always forward slash in the VFS). */
const SEP = '/';

export interface RegistryFrame {
  /** Current VFS path, e.g. "/registry/HKEY_LOCAL_MACHINE/SOFTWARE/Microsoft" */
  vfsPath: string;
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

/** True if the given VFS path is inside the registry root. */
export function isRegistryPath(path: string): boolean {
  return path === REGISTRY_ROOT || path.startsWith(REGISTRY_ROOT + SEP);
}

/**
 * Returns the display path shown in the pane breadcrumb.
 * Strips the "/registry" prefix so users see "HKEY_LOCAL_MACHINE\SOFTWARE\…".
 */
export function registryDisplayPath(vfsPath: string): string {
  const rel = vfsPath.startsWith(REGISTRY_ROOT + SEP)
    ? vfsPath.slice(REGISTRY_ROOT.length + 1)
    : vfsPath === REGISTRY_ROOT
    ? ''
    : vfsPath;
  return rel.replace(/\//g, '\\') || 'Registry';
}

/**
 * Builds a human-readable breadcrumb trail for a registry path.
 * e.g. ["Registry", "HKEY_LOCAL_MACHINE", "SOFTWARE", "Microsoft"]
 */
export function registryBreadcrumbs(vfsPath: string): string[] {
  if (vfsPath === REGISTRY_ROOT) return ['Registry'];
  const rel = vfsPath.slice(REGISTRY_ROOT.length + 1);
  return ['Registry', ...rel.split(SEP).filter(Boolean)];
}

/** Parent VFS path, or null if already at the registry root. */
export function registryParentPath(vfsPath: string): string | null {
  if (vfsPath === REGISTRY_ROOT) return null;
  const idx = vfsPath.lastIndexOf(SEP);
  if (idx <= REGISTRY_ROOT.length) return REGISTRY_ROOT;
  return vfsPath.slice(0, idx);
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
    path:        raw.path,
    type:        raw.type as FileType,
    size:        raw.size ?? 0,
    modified:    raw.modified ?? 0,
    created:     raw.modified ?? 0,
    accessed:    raw.modified ?? 0,
    permissions: raw.type === FileType.DIRECTORY ? 'drwxr-xr-x' : '-r--r--r--',
    isHidden:    false,
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
  async navigateInto(vfsPath: string): Promise<RegistryFrame> {
    return this.load(vfsPath);
  }

  /** Navigate up one level; returns null if already at the registry root. */
  async navigateUp(current: RegistryFrame): Promise<RegistryFrame | null> {
    const parent = registryParentPath(current.vfsPath);
    if (!parent) return null;
    return this.load(parent);
  }

  /** Reload the current frame (after a write operation). */
  async refresh(frame: RegistryFrame): Promise<RegistryFrame> {
    return this.load(frame.vfsPath);
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
  async readValue(vfsPath: string): Promise<string> {
    const result = await app().VFSReadFile(vfsPath);
    return typeof result === 'string' ? result : new TextDecoder().decode(result);
  }

  /* ─────────────── WRITE OPERATIONS ─────────────── */

  /** Create a new subkey (like mkdir). */
  async createKey(parentVfsPath: string, keyName: string): Promise<void> {
    const newPath = parentVfsPath + SEP + keyName;
    await app().VFSMkdir(newPath);
  }

  /**
   * Set a REG_SZ value.  Creates or overwrites.
   * For other types, write the serialised form — the backend stores as REG_SZ.
   */
  async setValue(keyVfsPath: string, valueName: string, data: string): Promise<void> {
    const valuePath = keyVfsPath + SEP + (valueName || '(default)');
    await app().VFSWriteFile(valuePath, data);
  }

  /** Delete a key or value at the given VFS path. */
  async delete(vfsPath: string): Promise<void> {
    await app().VFSRemove(vfsPath);
  }

  /** Rename is not supported by the registry; always rejects. */
  rename(_oldPath: string, _newName: string): Promise<void> {
    return Promise.reject(new Error('Registry keys and values cannot be renamed.'));
  }

  /* ─────────────── INTERNAL ─────────────── */

  private async load(vfsPath: string): Promise<RegistryFrame> {
    const listing = await app().VFSListDir(vfsPath);
    const hasParent = vfsPath !== REGISTRY_ROOT;
    const entries = withParent(
      (listing?.entries ?? []).map(toFileInfo),
      hasParent,
    );
    return { vfsPath, entries };
  }
}