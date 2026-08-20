import { FileInfo, FileType } from '../shared/protocol';
import { ApiService } from '../services/api.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { OperationHistoryService } from '../services/operation-history.service';

export interface PaneFileCommandsHost {
  readonly path: string;
  readonly selectedEntries: readonly FileInfo[];
  readonly cursorEntry: FileInfo | null;
  readonly canCreate: boolean;
  readonly isDirectoryTab: boolean;
  readonly isInArchive: boolean;
  refresh(): Promise<void>;
  selectPath(path: string): void;
}

/** Shared creation/manipulation commands for directory-capable panes. */
export class PaneFileCommandsController {
  constructor(
    private readonly api: ApiService,
    private readonly confirm: ConfirmDialogService,
    private readonly history: OperationHistoryService,
  ) {}

  async newFile(host: PaneFileCommandsHost): Promise<void> {
    await this.createDocument(host, {
      title: 'New File',
      defaultName: 'New File.txt',
      content: '',
      historyKey: 'reassembly.newFile',
      historyLabel: 'Create file',
    });
  }

  async newDocumentTemplate(
    host: PaneFileCommandsHost,
    kind: 'text' | 'markdown' | 'json',
  ): Promise<void> {
    const templates = {
      text: { title: 'New Text Document', defaultName: 'New Text Document.txt', content: '' },
      markdown: { title: 'New Markdown Document', defaultName: 'README.md', content: '# Title\n\n' },
      json: { title: 'New JSON Document', defaultName: 'data.json', content: '{\n  \n}\n' },
    } as const;
    const template = templates[kind];
    await this.createDocument(host, {
      ...template,
      historyKey: `reassembly.newTemplate.${kind}`,
      historyLabel: `Create ${template.title.toLowerCase()}`,
    });
  }

  async duplicateSelected(host: PaneFileCommandsHost): Promise<void> {
    if (!this.canMutate(host) || !host.selectedEntries.length) return;
    try {
      const result = await this.api.batchCopy(
        host.selectedEntries.map(entry => ({ source: entry.path, destination: host.path, exactDestination: false })),
        true,
      );
      const affected = result.affectedPaths ?? [];
      if (result.failed === 0 && result.skipped === 0 && affected.length === result.succeeded && affected.length > 0) {
        this.history.record({
          label: `Duplicate ${affected.length} item${affected.length === 1 ? '' : 's'}`,
          undo: async () => {
            const deletion = await this.api.batchDelete(affected.map(item => item.destination), true, false);
            if (deletion.failed > 0) throw new Error('Undo duplicate failed.');
          },
          redo: async () => {
            const redone = await this.api.batchCopy(affected.map(item => ({
              source: item.source,
              destination: item.destination,
              exactDestination: true,
            })), true);
            if (redone.failed > 0 || redone.skipped > 0) throw new Error('Redo duplicate failed.');
          },
        });
      }
      await host.refresh();
      if (affected.length === 1) host.selectPath(affected[0].destination);
    } catch (error) {
      await this.showError('Duplicate', error);
    }
  }

  async moveSelectionToNewFolder(host: PaneFileCommandsHost): Promise<void> {
    if (!this.canMutate(host) || !host.selectedEntries.length) return;
    const entries = [...host.selectedEntries];
    const requested = await this.confirm.prompt({
      title: 'Move to New Folder',
      message: `Create a folder and move ${entries.length} selected item${entries.length === 1 ? '' : 's'} into it:`,
      confirmText: 'Create & Move',
      defaultValue: 'New Folder',
      historyKey: 'reassembly.moveSelectionToNewFolder',
    });
    const folderName = await this.validateRequestedLeaf(requested, host.path);
    if (!folderName) return;

    const sourcePaths = entries.map(entry => entry.path);
    try {
      const folderPath = await this.api.joinPaths([host.path, folderName]);
      await this.api.createDirectory(folderPath, false);
      const result = await this.api.batchMove(sourcePaths.map(source => ({
        source,
        destination: folderPath,
        exactDestination: false,
      })), true);
      const affected = result.affectedPaths ?? [];
      if (result.failed > 0 || result.skipped > 0 || affected.length !== sourcePaths.length) {
        throw new Error(`Move completed only partially (${result.succeeded}/${result.total}).`);
      }
      this.history.record({
        label: `Move ${affected.length} item${affected.length === 1 ? '' : 's'} to ${folderName}`,
        undo: async () => {
          const back = await this.api.batchMove(affected.map(item => ({
            source: item.destination,
            destination: item.source,
            exactDestination: true,
          })), true);
          if (back.failed > 0 || back.skipped > 0) throw new Error('Undo move-to-folder failed.');
          await this.api.deleteFile(folderPath, true, false);
        },
        redo: async () => {
          await this.api.createDirectory(folderPath, false);
          const forward = await this.api.batchMove(affected.map(item => ({
            source: item.source,
            destination: folderPath,
            exactDestination: false,
          })), true);
          if (forward.failed > 0 || forward.skipped > 0) throw new Error('Redo move-to-folder failed.');
        },
      });
      await host.refresh();
      host.selectPath(folderPath);
    } catch (error) {
      await this.showError('Move Selection to New Folder', error);
    }
  }

  async createLinkForSelected(host: PaneFileCommandsHost, kind: 'symbolic' | 'hard'): Promise<void> {
    if (!this.canMutate(host)) return;
    const entry = host.cursorEntry ?? host.selectedEntries[0];
    if (!entry || entry.path === '__parent__') return;
    if (kind === 'hard' && entry.type === FileType.DIRECTORY) {
      await this.confirm.alert({ title: 'Hard Link', message: 'Hard links to directories are not supported.' });
      return;
    }
    const requested = await this.confirm.prompt({
      title: kind === 'symbolic' ? 'Create Symbolic Link' : 'Create Hard Link',
      message: `Link to "${entry.name}" as:`,
      confirmText: 'Create',
      defaultValue: `${entry.name}${kind === 'symbolic' ? ' link' : ' hardlink'}`,
      historyKey: `reassembly.${kind}Link.name`,
    });
    const cleanName = await this.validateRequestedLeaf(requested, host.path);
    if (!cleanName) return;
    try {
      const destination = await this.api.joinPaths([host.path, cleanName]);
      const create = () => kind === 'symbolic'
        ? this.api.createSymbolicLink(entry.path, destination)
        : this.api.createHardLink(entry.path, destination);
      await create();
      this.history.record({
        label: `${kind === 'symbolic' ? 'Create symbolic link' : 'Create hard link'} ${cleanName}`,
        undo: () => this.api.deleteFile(destination, true, false),
        redo: create,
      });
      await host.refresh();
      host.selectPath(destination);
    } catch (error) {
      await this.showError(kind === 'symbolic' ? 'Create Symbolic Link' : 'Create Hard Link', error);
    }
  }

  async createShortcutForSelected(host: PaneFileCommandsHost): Promise<void> {
    if (!this.canMutate(host)) return;
    const entry = host.cursorEntry ?? host.selectedEntries[0];
    if (!entry || entry.path === '__parent__') return;
    try {
      const osInfo = await this.api.getOSInfo();
      const windows = String(osInfo?.os).toLowerCase() === 'windows';
      let name = await this.confirm.prompt({
        title: 'Create Shortcut',
        message: `Create a shortcut to "${entry.name}" as:`,
        confirmText: 'Create',
        defaultValue: windows ? `${entry.name} - Shortcut.lnk` : `${entry.name} shortcut`,
        historyKey: 'reassembly.shortcut.name',
      });
      name = this.cleanLeafName(name);
      if (!name) {
        await this.confirm.alert({ title: 'Invalid Name', message: 'Choose a valid shortcut filename.' });
        return;
      }
      if (windows && !/\.lnk$/i.test(name)) name += '.lnk';
      if (!(await this.ensureLeafAvailable(host.path, name))) return;
      const destination = await this.api.joinPaths([host.path, name]);
      const create = () => this.api.createShortcut(entry.path, destination);
      await create();
      this.history.record({
        label: `Create shortcut ${name}`,
        undo: () => this.api.deleteFile(destination, true, false),
        redo: create,
      });
      await host.refresh();
      host.selectPath(destination);
    } catch (error) {
      await this.showError('Create Shortcut', error);
    }
  }

  private async createDocument(
    host: PaneFileCommandsHost,
    spec: { title: string; defaultName: string; content: string; historyKey: string; historyLabel: string },
  ): Promise<void> {
    if (!host.canCreate || host.isInArchive) return;
    const requested = await this.confirm.prompt({
      title: spec.title,
      message: 'Enter a name for the new document.',
      confirmText: 'Create',
      defaultValue: spec.defaultName,
      historyKey: spec.historyKey,
    });
    const cleanName = await this.validateRequestedLeaf(requested, host.path);
    if (!cleanName) return;
    try {
      const destination = await this.api.joinPaths([host.path, cleanName]);
      const create = () => this.api.writeFile(destination, spec.content, 'utf8');
      await create();
      this.history.record({
        label: `${spec.historyLabel} ${cleanName}`,
        undo: () => this.api.deleteFile(destination, true, false),
        redo: create,
      });
      await host.refresh();
      host.selectPath(destination);
    } catch (error) {
      await this.showError(spec.title, error);
    }
  }

  private canMutate(host: PaneFileCommandsHost): boolean {
    return host.isDirectoryTab && !host.isInArchive && host.canCreate;
  }

  private cleanLeafName(value: string | null): string {
    const clean = (value ?? '').trim();
    if (!clean || clean === '.' || clean === '..' || /[\/\\]/.test(clean) || /[<>:"|?*\u0000-\u001f]/.test(clean) || /[ .]$/.test(clean)) return '';
    const base = clean.split('.')[0].toUpperCase();
    const reserved = new Set(['CON','PRN','AUX','NUL','COM1','COM2','COM3','COM4','COM5','COM6','COM7','COM8','COM9','LPT1','LPT2','LPT3','LPT4','LPT5','LPT6','LPT7','LPT8','LPT9']);
    return reserved.has(base) ? '' : clean;
  }

  private async ensureLeafAvailable(directory: string, leafName: string): Promise<boolean> {
    const destination = await this.api.joinPaths([directory, leafName]);
    try {
      await this.api.getFileInfo(destination);
      await this.confirm.alert({ title: 'Name Already Exists', message: `An item named "${leafName}" already exists in this folder.` });
      return false;
    } catch {
      return true;
    }
  }

  private async validateRequestedLeaf(value: string | null, directory: string): Promise<string> {
    const clean = this.cleanLeafName(value);
    if (!clean) {
      await this.confirm.alert({
        title: 'Invalid Name',
        message: ['Choose a normal file or folder name.', 'Path separators, control characters, < > : " | ? *, trailing spaces/dots,', 'and Windows reserved device names such as CON, PRN, AUX, COM1 and LPT1 are not allowed.'].join('\n'),
      });
      return '';
    }
    return await this.ensureLeafAvailable(directory, clean) ? clean : '';
  }

  private async showError(title: string, error: unknown): Promise<void> {
    await this.confirm.alert({ title: `${title} Error`, message: error instanceof Error ? error.message : String(error) });
  }
}
