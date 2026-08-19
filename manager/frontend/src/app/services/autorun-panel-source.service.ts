import { Injectable } from '@angular/core';

import { FileType, type FileInfo } from '../shared/protocol';
import type { PanelSource, PanelSourceContext, PanelSourceListing } from './panel-source';
import type { SourceCapabilities } from './source-capabilities';

export type AutorunScope = 'user' | 'machine';
export type AutorunOrigin = 'registry-run' | 'registry-run-once' | 'startup-folder';

export interface AutorunRecord {
  id: string;
  name: string;
  command: string;
  executable?: string;
  sourcePath: string;
  scope: AutorunScope;
  origin: AutorunOrigin;
  enabled: boolean;
}

type AutorunFileInfo = FileInfo & {
  autorunId?: string;
  autorunCommand?: string;
  autorunExecutable?: string;
  autorunSourcePath?: string;
  autorunScope?: AutorunScope;
  autorunOrigin?: AutorunOrigin;
  autorunEnabled?: boolean;
};

const READ_ONLY: SourceCapabilities = { read: true, create: false, delete: false, rename: false, trash: false };
const ROOT = 'autorun://';

function normalize(path: string): string {
  return (path || ROOT).replace(/\/+$/, '').toLowerCase() || 'autorun:';
}

function fileInfo(name: string, path: string, type: FileType): FileInfo {
  return {
    name, path, type, size: 0, created: 0, modified: 0, accessed: 0,
    permissions: 'r--', isHidden: false,
  };
}

@Injectable({ providedIn: 'root' })
export class AutorunPanelSource implements PanelSource {
  readonly id = 'builtin.autorun';
  readonly kind = 'virtual' as const;
  readonly title = 'Autorun';

  canHandle(context: PanelSourceContext): boolean {
    return /^autorun:\/\//i.test(context.path);
  }

  capabilities(): SourceCapabilities { return READ_ONLY; }
  supportsStreaming(): boolean { return false; }
  getRootPath(): string { return ROOT; }

  getParentPath(context: PanelSourceContext): string {
    const p = normalize(context.path);
    if (p === 'autorun:' || p === 'autorun://') return ROOT;
    const rest = context.path.replace(/^autorun:\/\//i, '').replace(/\/+$/, '');
    const cut = rest.lastIndexOf('/');
    return cut < 0 ? ROOT : `autorun://${rest.slice(0, cut)}`;
  }

  joinChildPath(context: PanelSourceContext, childName: string): string {
    return `${context.path.replace(/\/+$/, '')}/${encodeURIComponent(childName)}`;
  }

  async list(context: PanelSourceContext): Promise<PanelSourceListing> {
    const path = normalize(context.path);
    let entries: FileInfo[];

    if (path === 'autorun:' || path === 'autorun://') {
      entries = [
        fileInfo('Current User', 'autorun://user', FileType.DIRECTORY),
        fileInfo('All Users', 'autorun://machine', FileType.DIRECTORY),
      ];
    } else if (path === 'autorun://user' || path === 'autorun://machine') {
      const scope = path.endsWith('user') ? 'user' : 'machine';
      entries = [
        fileInfo('Registry Run', `autorun://${scope}/registry-run`, FileType.DIRECTORY),
        fileInfo('Registry RunOnce', `autorun://${scope}/registry-run-once`, FileType.DIRECTORY),
        fileInfo('Startup Folder', `autorun://${scope}/startup-folder`, FileType.DIRECTORY),
      ];
    } else {
      const records = await this.records();
      const [scope, origin] = this.parseLeaf(path);
      entries = records
        .filter(record => record.scope === scope && record.origin === origin)
        .map(record => this.toFileInfo(record));
    }

    return {
      location: { path: context.path || ROOT, sourceId: this.id, sourceKind: this.kind },
      entries,
    };
  }

  canPreviewEntry(_context: PanelSourceContext, entry: FileInfo): boolean {
    return !this.decodeEntryPath(entry.path);
  }

  activateEntry(_context: PanelSourceContext, entry: FileInfo): boolean {
    const id = this.decodeEntryPath(entry.path);
    if (!id) return false;

    // Autorun records are synthetic resources, not files at autorun://entry/....
    // Opening them through the generic viewer would make ApiService try to create/read
    // a Windows path with an invalid URI. Provider activation owns this leaf instead.
    void this.records().then(records => {
      const record = records.find(candidate => candidate.id === id);
      if (record?.executable) {
        return (window as any)?.go?.app?.App?.OpenFile?.(record.executable);
      }
      return undefined;
    }).catch(error => console.warn('Failed to activate autorun entry:', error));
    return true;
  }

  async records(): Promise<AutorunRecord[]> {
    const app = (window as any)?.go?.app?.App;
    if (!app?.AutorunList) return [];
    const result = await app.AutorunList();
    return Array.isArray(result) ? result : [];
  }

  async setEnabled(id: string, enabled: boolean): Promise<void> {
    const app = (window as any)?.go?.app?.App;
    if (!app?.AutorunSetEnabled) throw new Error('Autorun backend is not available on this platform.');
    await app.AutorunSetEnabled(id, enabled);
  }

  async delete(id: string): Promise<void> {
    const app = (window as any)?.go?.app?.App;
    if (!app?.AutorunDelete) throw new Error('Autorun backend is not available on this platform.');
    await app.AutorunDelete(id);
  }

  decodeEntryPath(path: string): string | null {
    const match = /^autorun:\/\/entry\/(.+)$/i.exec(path ?? '');
    if (!match?.[1]) return null;
    try { return decodeURIComponent(match[1]); } catch { return null; }
  }

  private parseLeaf(path: string): [AutorunScope, AutorunOrigin] {
    const match = /^autorun:\/\/(user|machine)\/(registry-run|registry-run-once|startup-folder)$/i.exec(path);
    if (!match) return ['user', 'registry-run'];
    return [match[1].toLowerCase() as AutorunScope, match[2].toLowerCase() as AutorunOrigin];
  }

  private toFileInfo(record: AutorunRecord): FileInfo {
    return {
      ...fileInfo(record.name, `autorun://entry/${encodeURIComponent(record.id)}`, FileType.FILE),
      size: record.command.length,
      permissions: record.enabled ? 'r-x' : 'r--',
      autorunId: record.id,
      autorunCommand: record.command,
      autorunExecutable: record.executable ?? '',
      autorunSourcePath: record.sourcePath,
      autorunScope: record.scope,
      autorunOrigin: record.origin,
      autorunEnabled: record.enabled,
    } as AutorunFileInfo;
  }
}