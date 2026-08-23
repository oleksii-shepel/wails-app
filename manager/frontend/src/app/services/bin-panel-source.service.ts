import { Injectable } from '@angular/core';

import { FileType, type FileInfo } from '../shared/protocol';
import type { TrashEntry } from './api.service';
import { ApiService } from './api.service';
import { isArchive } from './archive.service';
import type { PanelSource, PanelSourceContext, PanelSourceListing } from './panel-source';
import type { SourceCapabilities } from './source-capabilities';

export type BinFileInfo = FileInfo & {
  binId: string;
  binOriginalPath: string;
  binDeletedAt: string;
  binCanRestore: boolean;
};

const ROOT = 'bin://';
const READ_ONLY: SourceCapabilities = Object.freeze({
  read: true,
  create: false,
  delete: false,
  rename: false,
  trash: false,
});

@Injectable({ providedIn: 'root' })
export class BinPanelSource implements PanelSource {
  readonly id = 'builtin.bin';
  readonly kind = 'virtual' as const;
  readonly title = 'Bin';

  constructor(private readonly api: ApiService) {}

  canHandle(context: PanelSourceContext): boolean {
    return /^bin:\/\//i.test(context.path);
  }

  capabilities(): SourceCapabilities { return READ_ONLY; }
  supportsStreaming(): boolean { return false; }
  getRootPath(): string { return ROOT; }
  getParentPath(): string { return ROOT; }
  joinChildPath(): string { return ROOT; }

  async list(_context: PanelSourceContext): Promise<PanelSourceListing> {
    const entries = (await this.api.listTrashEntries()).map(entry => this.toFileInfo(entry));
    return {
      location: { path: ROOT, sourceId: this.id, sourceKind: this.kind },
      entries,
    };
  }

  canPreviewEntry(): boolean { return false; }

  // Recycle-bin entries are synthetic shell items. They must never be sent to
  // the ordinary file viewer using their bin:// identity.
  activateEntry(): boolean { return true; }

  decodeEntryPath(path: string): string | null {
    const match = /^bin:\/\/entry\/(.+)$/i.exec(path ?? '');
    if (!match?.[1]) return null;
    try { return decodeURIComponent(match[1]); } catch { return null; }
  }

  async restore(id: string): Promise<string> {
    return await this.api.restoreTrashEntry(id);
  }

  async deletePermanently(id: string): Promise<void> {
    await this.api.deleteTrashEntry(id);
  }

  async empty(): Promise<void> {
    await this.api.emptyTrash();
  }

  private basename(path: string): string {
    const normalized = (path || '').trim().replace(/\\/g, '/').replace(/\/+$/, '');
    if (!normalized) return '';
    const index = normalized.lastIndexOf('/');
    return index >= 0 ? normalized.slice(index + 1) : normalized;
  }

  private toFileInfo(entry: TrashEntry): BinFileInfo {
    const deleted = Date.parse(entry.deletedAt || '');
    // Prefer the basename from the original path. On Windows the Recycle Bin
    // shell's display Name can omit extensions when Explorer hides them.
    const originalName = this.basename(entry.originalPath);
    const name = originalName || entry.name;
    const type = isArchive(name)
      ? FileType.ARCHIVE
      : entry.isDir
        ? FileType.DIRECTORY
        : FileType.FILE;
    return {
      name,
      path: `bin://entry/${encodeURIComponent(entry.id)}`,
      type,
      size: entry.size,
      created: 0,
      modified: Number.isFinite(deleted) ? Math.floor(deleted / 1000) : 0,
      accessed: 0,
      permissions: entry.canRestore ? 'r--' : '---',
      isHidden: false,
      binId: entry.id,
      binOriginalPath: entry.originalPath,
      binDeletedAt: entry.deletedAt,
      binCanRestore: entry.canRestore,
    };
  }
}