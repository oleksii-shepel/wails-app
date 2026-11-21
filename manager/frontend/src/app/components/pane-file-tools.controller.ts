import { FileInfo, FileType } from '../shared/protocol';
import { ApiService } from '../services/api.service';
import { ArchiveFilesDialogService } from '../services/archive-files-dialog.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { OperationHistoryService } from '../services/operation-history.service';

export interface PaneFileToolsHost {
  readonly path: string;
  readonly selectedEntries: readonly FileInfo[];
  readonly cursorEntry: FileInfo | null;
  readonly canCreate: boolean;
  readonly isDirectoryTab: boolean;
  readonly isInArchive: boolean;
  refresh(): Promise<void>;
  selectPath(path: string): void;
}

/** Shared metadata, inspection and utility commands for directory-capable panes. */
export class PaneFileToolsController {
  constructor(
    private readonly api: ApiService,
    private readonly archiveFilesDialog: ArchiveFilesDialogService,
    private readonly confirm: ConfirmDialogService,
    private readonly history: OperationHistoryService,
  ) {}

  async revealSelectedInFileManager(host: PaneFileToolsHost): Promise<void> {
    const entry = host.cursorEntry ?? host.selectedEntries[0];
    if (!entry || host.isInArchive || !host.isDirectoryTab) return;
    await this.api.revealInFileManager(entry.path);
  }

  async touchSelectedModifiedTime(host: PaneFileToolsHost): Promise<void> {
    if (!host.isDirectoryTab || host.isInArchive) return;
    const entries = this.effectiveSelection(host);
    if (entries.length === 0) return;

    const value = await this.confirm.prompt({
      title: entries.length === 1 ? `Modified Time — ${entries[0].name}` : `Modified Time — ${entries.length} items`,
      message: 'Enter local date/time as YYYY-MM-DDTHH:mm:ss, or "now".',
      confirmText: 'Apply',
      defaultValue: new Date().toISOString().slice(0, 19),
      historyKey: 'reassembly.modifiedTime',
    });
    if (value == null) return;

    const raw = String(value).trim();
    const timestamp = raw.toLowerCase() === 'now' ? Date.now() : new Date(raw).getTime();
    if (!Number.isFinite(timestamp)) {
      await this.confirm.alert({ title: 'Invalid Date / Time', message: 'Use YYYY-MM-DDTHH:mm:ss (for example 2026-08-16T12:30:00) or "now".' });
      return;
    }

    const before = await Promise.all(entries.map(async entry => ({
      path: entry.path,
      modified: Number((await this.api.getFileInfo(entry.path)).modified),
    })));
    for (const entry of entries) await this.api.setModifiedTime(entry.path, timestamp);

    this.history.record({
      label: `Set modified time for ${entries.length} item${entries.length === 1 ? '' : 's'}`,
      undo: async () => {
        for (const item of before) {
          const milliseconds = item.modified < 10_000_000_000 ? item.modified * 1000 : item.modified;
          await this.api.setModifiedTime(item.path, milliseconds);
        }
      },
      redo: async () => { for (const entry of entries) await this.api.setModifiedTime(entry.path, timestamp); },
    });
    await host.refresh();
  }

  async showSelectedChecksum(host: PaneFileToolsHost): Promise<void> {
    if (!host.isDirectoryTab || host.isInArchive) return;
    const entries = this.effectiveSelection(host).filter(entry => entry.type !== FileType.DIRECTORY);
    if (entries.length === 0) return;
    const rows: string[] = [];
    for (const entry of entries) rows.push(`${await this.api.hashFile(entry.path)}  ${entry.name}`);
    const text = rows.join('\n');
    await this.confirm.alert({
      title: entries.length === 1 ? `SHA-256 — ${entries[0].name}` : `SHA-256 — ${entries.length} files`,
      message: entries.length === 1 ? `${text}\n\n${entries[0].path}` : text,
    });
  }

  async calculateSelectedSize(host: PaneFileToolsHost): Promise<void> {
    if (!host.isDirectoryTab || host.isInArchive) return;
    const entries = this.effectiveSelection(host);
    if (entries.length === 0) return;
    let total = 0;
    for (const entry of entries) total += entry.type === FileType.DIRECTORY ? await this.api.dirSizeRecursive(entry.path) : Number(entry.size) || 0;
    await this.confirm.alert({
      title: entries.length === 1 ? `Size — ${entries[0].name}` : `Selected Size — ${entries.length} items`,
      message: [`Size: ${this.formatSize(total)}`, `Bytes: ${total.toLocaleString()}`].join('\n'),
    });
  }

  async editSelectedAttributes(host: PaneFileToolsHost): Promise<void> {
    const entry = host.cursorEntry ?? host.selectedEntries[0];
    if (!entry || host.isInArchive || !host.isDirectoryTab) return;
    const [osInfo, attributes] = await Promise.all([this.api.getOSInfo(), this.api.getPathAttributes(entry.path)]);
    if (String(osInfo?.os).toLowerCase() === 'windows') {
      const current = [attributes.readOnly ? 'readonly' : '', attributes.hidden ? 'hidden' : '', attributes.system ? 'system' : '', attributes.archive ? 'archive' : ''].filter(Boolean).join(',');
      const value = await this.confirm.prompt({ title: `Attributes — ${entry.name}`, message: 'Comma-separated attributes: readonly, hidden, system, archive', confirmText: 'Apply', defaultValue: current, historyKey: 'reassembly.attributes.windows' });
      if (value == null) return;
      const tokens = new Set(String(value).split(',').map(token => token.trim().toLowerCase()).filter(Boolean));
      const valid = new Set(['readonly', 'hidden', 'system', 'archive']);
      const invalid = [...tokens].filter(token => !valid.has(token));
      if (invalid.length) { await this.confirm.alert({ title: 'Invalid Attributes', message: `Unknown attribute${invalid.length === 1 ? '' : 's'}: ${invalid.join(', ')}` }); return; }
      await this.api.setPathAttributes(entry.path, { readOnly: tokens.has('readonly'), hidden: tokens.has('hidden'), system: tokens.has('system'), archive: tokens.has('archive') });
    } else {
      const value = await this.confirm.prompt({ title: `Permissions — ${entry.name}`, message: 'Unix permission mode (octal), for example 0644 or 0755:', confirmText: 'Apply', defaultValue: (attributes.unixMode || 0).toString(8).padStart(4, '0'), historyKey: 'reassembly.attributes.unix' });
      if (value == null) return;
      const normalized = String(value).trim();
      if (!/^[0-7]{3,4}$/.test(normalized)) { await this.confirm.alert({ title: 'Invalid Permission Mode', message: 'Enter a 3- or 4-digit octal mode such as 644, 0644, or 0755.' }); return; }
      await this.api.setPathAttributes(entry.path, { unixMode: Number.parseInt(normalized, 8) });
    }
    await host.refresh();
    host.selectPath(entry.path);
  }

  async showSelectedProperties(host: PaneFileToolsHost): Promise<void> {
    const selected = this.effectiveSelection(host);
    if (selected.length === 0) return;
    if (selected.length > 1) {
      const infos = host.isInArchive ? selected : await Promise.all(selected.map(entry => this.api.getFileInfo(entry.path)));
      let totalSize = 0, files = 0, directories = 0, archives = 0, symlinks = 0;
      for (const info of infos) {
        if (info.type === FileType.DIRECTORY) { directories++; totalSize += host.isInArchive ? Number(info.size) || 0 : await this.api.dirSizeRecursive(info.path); }
        else { totalSize += Number(info.size) || 0; if (info.type === FileType.ARCHIVE) archives++; else if (info.type === FileType.SYMLINK) symlinks++; else files++; }
      }
      await this.confirm.alert({ title: `Properties — ${selected.length} items`, message: [`Items: ${selected.length}`, `Files: ${files}`, `Directories: ${directories}`, `Archives: ${archives}`, `Symbolic links: ${symlinks}`, `Total size: ${this.formatSize(totalSize) || '—'}`, `Bytes: ${totalSize.toLocaleString()}`, host.isInArchive ? 'Location: inside archive' : `Folder: ${host.path}`].join('\n') });
      return;
    }
    const entry = selected[0];
    const info = host.isInArchive ? entry : await this.api.getFileInfo(entry.path);
    const displaySize = !host.isInArchive && info.type === FileType.DIRECTORY ? await this.api.dirSizeRecursive(info.path) : Number(info.size) || 0;
    const type = info.type === FileType.DIRECTORY ? 'Directory' : info.type === FileType.ARCHIVE ? 'Archive' : info.type === FileType.SYMLINK ? 'Symbolic link' : 'File';
    const timestamp = (value: unknown) => { const n = Number(value); return Number.isFinite(n) && n > 0 ? new Date(n * (n < 10_000_000_000 ? 1000 : 1)).toLocaleString() : '—'; };
    await this.confirm.alert({ title: `Properties — ${info.name}`, message: [`Name: ${info.name}`, `Type: ${type}`, `Path: ${info.path}`, `Size: ${this.formatSize(displaySize) || '—'}`, `Bytes: ${displaySize.toLocaleString()}`, `Modified: ${timestamp(info.modified)}`, `Created: ${timestamp(info.created)}`, `Accessed: ${timestamp(info.accessed)}`, `Permissions: ${info.permissions || '—'}`, info.isSymlink ? 'Symbolic link: yes' : '', host.isInArchive ? 'Location: inside archive' : ''].filter(Boolean).join('\n') });
  }

  async createZipFromSelection(host: PaneFileToolsHost): Promise<void> {
    if (!host.isDirectoryTab || host.isInArchive || !host.canCreate || host.selectedEntries.length === 0) return;
    const entries = [...host.selectedEntries];
    const destinationFolder = host.path;
    const first = entries[0];
    const baseName = entries.length === 1 ? first.name.replace(/\.[^.]+$/, '') || first.name : 'Archive';
    const requested = await this.archiveFilesDialog.open({
      title: 'Archive Files',
      message: entries.length === 1 ? `Compress "${first.name}" to:` : `Compress ${entries.length} selected items to:`,
      confirmText: 'Create',
      defaultValue: `${baseName}.zip`,
    });
    if (requested == null) return;
    let cleanName = this.cleanLeafName(requested);
    if (!cleanName) { await this.confirm.alert({ title: 'Invalid Name', message: 'Choose a valid ZIP filename.' }); return; }
    if (!/\.zip$/i.test(cleanName)) cleanName += '.zip';
    const destination = await this.api.joinPaths([destinationFolder, cleanName]);
    try { await this.api.getFileInfo(destination); await this.confirm.alert({ title: 'Name Already Exists', message: `An item named "${cleanName}" already exists.` }); return; } catch { /* available */ }
    const sources = entries.map(entry => entry.path);
    await this.api.createZipArchive(sources, destination);
    this.history.record({ label: `Create archive ${cleanName}`, undo: async () => { await this.api.deleteFile(destination, true, false); }, redo: async () => { await this.api.createZipArchive(sources, destination); } });
    await host.refresh();
    host.selectPath(destination);
  }

  private effectiveSelection(host: PaneFileToolsHost): FileInfo[] {
    return host.selectedEntries.length > 0 ? [...host.selectedEntries] : host.cursorEntry ? [host.cursorEntry] : [];
  }

  private cleanLeafName(value: unknown): string | null {
    const name = String(value ?? '').trim();
    if (!name || /[\\/:*?"<>|]/.test(name) || /[ .]$/.test(name)) return null;
    return name;
  }

  private formatSize(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes < 0) return '—';
    if (bytes < 1024) return `${bytes} B`;
    const units = ['KiB', 'MiB', 'GiB', 'TiB'];
    let value = bytes / 1024, unit = units[0];
    for (let i = 1; i < units.length && value >= 1024; i++) { value /= 1024; unit = units[i]; }
    return `${value.toFixed(value >= 10 ? 1 : 2)} ${unit}`;
  }
}
