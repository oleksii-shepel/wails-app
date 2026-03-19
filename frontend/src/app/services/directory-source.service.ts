import { Injectable } from '@angular/core';

import type { ArchiveListing, FileInfo } from '../shared/protocol-enhanced';
import type { PathSegment } from '../components/address-bar.types';
import { buildSegments as buildBasicSegments, buildSegmentsFromParsedPath, normalizePath, parentPath, parsePathWithArchives } from '../components/address-bar.utils';
import { ArchiveService, type ArchiveBrowseTarget, type ArchiveFrame, isArchive } from './archive.service';
import { ApiService } from './api.service';
import { REGISTRY_ROOT, RegistryService, isRegistryPath, type RegistryFrame } from './registry.service';

export interface DirectorySourceListing {
  path: string;
  entries: FileInfo[];
}

export type DirectorySourceKind = 'filesystem' | 'registry';

interface DirectoryListingProvider {
  readonly kind: DirectorySourceKind;
  canHandlePath(path: string): boolean;
  canHandleEntries(entries: FileInfo[]): boolean;
  listDirectory(path: string, showHidden: boolean): Promise<DirectorySourceListing>;
  getRootPath(path: string): string | null;
  joinChildPath(basePath: string, childName: string): string;
  supportsStreaming(path: string): boolean;
}

export interface FileViewDirectorySource {
  listDirectory(path: string, showHidden?: boolean): Promise<DirectorySourceListing>;
  browseArchive(fsPath: string, nestedChain?: string[], innerPath?: string): Promise<ArchiveListing>;
  getArchiveFrame(fsPath: string, nestedChain?: string[], innerPath?: string): Promise<ArchiveFrame>;
  openNestedArchive(frame: ArchiveFrame, nestedChain: string[]): Promise<ArchiveFrame>;
  navigateArchive(frame: ArchiveFrame, folderInnerPath: string): Promise<ArchiveFrame>;
  navigateArchiveUp(frame: ArchiveFrame): Promise<ArchiveFrame | null>;
  refreshArchive(frame: ArchiveFrame): Promise<ArchiveFrame>;
  resolveArchiveBrowseTarget(seg: { name: string; fullPath: string }, stack: ArchiveFrame[]): ArchiveBrowseTarget | null;
  archiveListingToFileInfos(listing: ArchiveListing): FileInfo[];
  getPathKind(path: string, entries?: FileInfo[]): DirectorySourceKind;
  getRootPath(path: string, entries?: FileInfo[]): string | null;
  getParentPath(path: string): string;
  isRootPath(path: string): boolean;
  isPathInsideArchive(path: string): boolean;
  buildPathSegments(path: string): PathSegment[];
  joinChildPath(basePath: string, childName: string): string;
  supportsStreaming(path: string): boolean;
}

class FilesystemDirectoryProvider implements DirectoryListingProvider {
  readonly kind: DirectorySourceKind = 'filesystem';

  constructor(private readonly apiService: ApiService) {}

  canHandlePath(_path: string): boolean {
    return true;
  }

  canHandleEntries(_entries: FileInfo[]): boolean {
    return false;
  }

  listDirectory(path: string, showHidden: boolean): Promise<DirectorySourceListing> {
    return this.apiService.listDirectory(path, showHidden);
  }

  getRootPath(path: string): string | null {
    const normalized = (path || '/').replace(/\\/g, '/');
    const winDrive = normalized.match(/^([A-Za-z]:)\//);
    if (winDrive) return `${winDrive[1]}/`;
    return '/';
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmedBase = (basePath || '/').replace(/\\/g, '/').replace(/\/+$/, '');
    return `${trimmedBase}/${childName}`;
  }

  supportsStreaming(_path: string): boolean {
    return true;
  }
}

class RegistryDirectoryProvider implements DirectoryListingProvider {
  readonly kind: DirectorySourceKind = 'registry';

  constructor(private readonly registryService: RegistryService) {}

  canHandlePath(path: string): boolean {
    return isRegistryPath(path);
  }

  canHandleEntries(entries: FileInfo[]): boolean {
    return entries.some((entry) =>
      entry.path !== '__parent__' &&
      (isRegistryPath(entry.path) || !!entry.registryKind || !!entry.registryValueType || !!entry.registryPreview),
    );
  }

  async listDirectory(path: string, _showHidden: boolean): Promise<DirectorySourceListing> {
    const frame: RegistryFrame = path === REGISTRY_ROOT
      ? await this.registryService.openRoot()
      : await this.registryService.navigateInto(path);

    return {
      path: frame.path,
      entries: frame.entries,
    };
  }

  getRootPath(_path: string): string | null {
    return REGISTRY_ROOT;
  }

  joinChildPath(basePath: string, childName: string): string {
    if (basePath === REGISTRY_ROOT) {
      return `${REGISTRY_ROOT}${childName}`;
    }
    return `${basePath.replace(/\/+$/, '')}/${childName}`;
  }

  supportsStreaming(_path: string): boolean {
    return false;
  }
}

@Injectable({ providedIn: 'root' })
export class DirectorySourceService implements FileViewDirectorySource {
  private readonly providers: DirectoryListingProvider[];

  constructor(
    private readonly apiService: ApiService,
    private readonly archiveService: ArchiveService,
    private readonly registryService: RegistryService,
  ) {
    this.providers = [
      new RegistryDirectoryProvider(this.registryService),
      new FilesystemDirectoryProvider(this.apiService),
    ];
  }

  async listDirectory(path: string, showHidden: boolean = false): Promise<DirectorySourceListing> {
    return this.providerForPath(path).listDirectory(path, showHidden);
  }

  async browseArchive(fsPath: string, nestedChain: string[] = [], innerPath: string = ''): Promise<ArchiveListing> {
    return this.apiService.archiveBrowse(fsPath, nestedChain, innerPath);
  }

  async getArchiveFrame(fsPath: string, nestedChain: string[] = [], innerPath: string = ''): Promise<ArchiveFrame> {
    let frame = await this.archiveService.openRootArchive(fsPath);

    for (let index = 0; index < nestedChain.length; index += 1) {
      frame = await this.archiveService.openNestedArchive(frame, nestedChain.slice(0, index + 1));
    }

    if (innerPath) {
      frame = await this.archiveService.navigateInto(frame, innerPath);
    }

    return frame;
  }

  openNestedArchive(frame: ArchiveFrame, nestedChain: string[]): Promise<ArchiveFrame> {
    return this.archiveService.openNestedArchive(frame, nestedChain);
  }

  navigateArchive(frame: ArchiveFrame, folderInnerPath: string): Promise<ArchiveFrame> {
    return this.archiveService.navigateInto(frame, folderInnerPath);
  }

  navigateArchiveUp(frame: ArchiveFrame): Promise<ArchiveFrame | null> {
    return this.archiveService.navigateUp(frame);
  }

  refreshArchive(frame: ArchiveFrame): Promise<ArchiveFrame> {
    return this.archiveService.refreshFrame(frame);
  }

  resolveArchiveBrowseTarget(seg: { name: string; fullPath: string }, stack: ArchiveFrame[]): ArchiveBrowseTarget | null {
    return this.archiveService.resolveBrowseTarget(seg, stack);
  }

  archiveListingToFileInfos(listing: ArchiveListing): FileInfo[] {
    return this.archiveService.listingToFileInfos(listing);
  }

  getPathKind(path: string, entries: FileInfo[] = []): DirectorySourceKind {
    return this.providerFor(path, entries).kind;
  }

  getRootPath(path: string, entries: FileInfo[] = []): string | null {
    return this.providerFor(path, entries).getRootPath(path);
  }

  getParentPath(path: string): string {
    const normalized = normalizePath(path);
    const { fsDirPath, archiveInnerParts } = parsePathWithArchives(normalized);

    if (archiveInnerParts.length > 0) {
      const parentInnerParts = archiveInnerParts.slice(0, -1);
      return parentInnerParts.length > 0
        ? normalizePath(`${fsDirPath}/${parentInnerParts.join('/')}`)
        : fsDirPath;
    }

    return parentPath(normalized);
  }

  isRootPath(path: string): boolean {
    const normalized = normalizePath(path);
    if (this.isPathInsideArchive(normalized)) {
      return false;
    }

    const rootPath = this.getRootPath(normalized);
    return !!rootPath && normalized === normalizePath(rootPath);
  }

  isPathInsideArchive(path: string): boolean {
    const normalized = normalizePath(path);
    if (isRegistryPath(normalized)) {
      return false;
    }

    const { fsDirPath } = parsePathWithArchives(normalized);
    const leafName = fsDirPath.replace(/\\/g, '/').split('/').filter(Boolean).pop() ?? '';
    return isArchive(leafName);
  }

  buildPathSegments(path: string): PathSegment[] {
    const normalized = normalizePath(path);
    const { fsDirPath, archiveInnerParts } = parsePathWithArchives(normalized);
    const baseSegments = archiveInnerParts.length > 0
      ? buildSegmentsFromParsedPath(fsDirPath, archiveInnerParts)
      : buildBasicSegments(normalized);

    let insideArchive = false;
    return baseSegments.map((segment) => {
      const isArchiveRoot = isArchive(segment.label);
      const nextSegment: PathSegment = {
        ...segment,
        isArchiveRoot,
        isArchiveInner: insideArchive,
      };
      if (isArchiveRoot) {
        insideArchive = true;
      }
      return nextSegment;
    });
  }

  joinChildPath(basePath: string, childName: string): string {
    return this.providerForPath(basePath).joinChildPath(basePath, childName);
  }

  supportsStreaming(path: string): boolean {
    return this.providerForPath(path).supportsStreaming(path);
  }

  private providerFor(path: string, entries: FileInfo[] = []): DirectoryListingProvider {
    const entryMatch = this.providers.find((provider) => provider.canHandleEntries(entries));
    if (entryMatch) return entryMatch;
    return this.providerForPath(path);
  }

  private providerForPath(path: string): DirectoryListingProvider {
    return this.providers.find((provider) => provider.canHandlePath(path)) ?? this.providers[this.providers.length - 1];
  }
}