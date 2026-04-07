import { Injectable } from '@angular/core';

import { FileType, type ArchiveListing, type FileInfo } from '../shared/protocol-enhanced';
import type { PathSegment } from '../components/address-bar.types';
import { buildSegments as buildBasicSegments, buildSegmentsFromParsedPath, isAliasPath, normalizePath, parentPath, parsePathWithArchives } from '../components/address-bar.utils';
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

function isWebHttpPath(path: string): boolean {
  const normalized = normalizePath(path);
  try {
    const parsed = new URL(normalized);
    return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && parsed.host.length > 0;
  } catch {
    return false;
  }
}

function looksLikeHtmlDocument(content: string): boolean {
  const sample = (content || '').trimStart().slice(0, 512).toLowerCase();
  return sample.startsWith('<!doctype html') ||
    sample.startsWith('<html') ||
    sample.includes('<head') ||
    sample.includes('<body');
}

function canonicalizeWebUrl(raw: string): string {
  const parsed = new URL(raw);
  parsed.protocol = parsed.protocol.toLowerCase();
  parsed.host = parsed.host.toLowerCase();
  parsed.hash = '';
  parsed.pathname = parsed.pathname.replace(/\\/g, '/');
  if (parsed.pathname === '/') {
    parsed.pathname = '';
  } else {
    parsed.pathname = parsed.pathname.replace(/\/+$/, '');
  }
  return parsed.toString();
}

function isLikelyWebPage(pathname: string): boolean {
  if (!pathname || pathname.endsWith('/')) {
    return true;
  }

  const match = /\.([^.\/]+)$/.exec(pathname);
  const ext = match?.[1]?.toLowerCase() || '';
  return ext === '' || [
    'html', 'htm', 'xhtml', 'shtml', 'php', 'asp', 'aspx', 'jsp', 'jspx', 'cfm',
  ].includes(ext);
}

function webEntryDisplayName(target: URL): string {
  const trimmedPath = target.pathname.replace(/\/+$/, '').replace(/^\/+/, '');
  if (!trimmedPath) {
    return target.search ? `${target.host}${target.search}` : target.host;
  }

  const segments = trimmedPath.split('/');
  const base = segments[segments.length - 1] || target.host;
  return target.search ? `${base}${target.search}` : base;
}

function parseSrcset(value: string | null): string[] {
  if (!value) return [];
  return value
    .split(',')
    .map((part) => part.trim().split(/\s+/)[0] || '')
    .filter(Boolean);
}

function resolveWebReference(baseUrl: URL, rawRef: string | null): URL | null {
  const ref = (rawRef || '').trim();
  if (!ref) return null;

  const lower = ref.toLowerCase();
  if (
    lower.startsWith('#') ||
    lower.startsWith('javascript:') ||
    lower.startsWith('mailto:') ||
    lower.startsWith('tel:') ||
    lower.startsWith('data:')
  ) {
    return null;
  }

  try {
    const resolved = new URL(ref, baseUrl);
    if (resolved.protocol !== 'http:' && resolved.protocol !== 'https:') {
      return null;
    }
    resolved.hash = '';
    return resolved;
  } catch {
    return null;
  }
}

function makeWebEntry(path: string, type: FileType): FileInfo {
  const target = new URL(path);
  return {
    name: webEntryDisplayName(target),
    path,
    type,
    size: 0,
    modified: 0,
    mimeType: '',
    isHidden: false,
  } as FileInfo;
}

function discoverWebEntriesFromHtml(content: string, pageUrl: string): FileInfo[] {
  let currentUrl: URL;
  try {
    currentUrl = new URL(canonicalizeWebUrl(pageUrl));
  } catch {
    return [];
  }

  const parser = new DOMParser();
  const document = parser.parseFromString(content, 'text/html');

  const baseHref = document.querySelector('base[href]')?.getAttribute('href') || null;
  const baseUrl = resolveWebReference(currentUrl, baseHref) || currentUrl;

  const entries = new Map<string, FileInfo>();
  const addEntry = (rawRef: string | null, navigable: boolean): void => {
    const resolved = resolveWebReference(baseUrl, rawRef);
    if (!resolved) return;

    if (resolved.search) {
      return;
    }

    const normalized = canonicalizeWebUrl(resolved.toString());
    if (normalized === currentUrl.toString() || entries.has(normalized)) {
      return;
    }

    const type = navigable && isLikelyWebPage(resolved.pathname)
      ? FileType.DIRECTORY
      : FileType.FILE;
    entries.set(normalized, makeWebEntry(normalized, type));
  };

  document.querySelectorAll('a[href]').forEach((node) => addEntry(node.getAttribute('href'), true));
  document.querySelectorAll('form[action]').forEach((node) => addEntry(node.getAttribute('action'), true));
  document.querySelectorAll('img[src],script[src],link[href],source[src],iframe[src],embed[src]').forEach((node) => {
    const attr = node.hasAttribute('src') ? 'src' : 'href';
    addEntry(node.getAttribute(attr), node.tagName.toLowerCase() === 'iframe' || node.tagName.toLowerCase() === 'embed');
  });
  document.querySelectorAll('object[data]').forEach((node) => addEntry(node.getAttribute('data'), false));
  document.querySelectorAll('video[src],audio[src]').forEach((node) => addEntry(node.getAttribute('src'), false));
  document.querySelectorAll('video[poster]').forEach((node) => addEntry(node.getAttribute('poster'), false));
  document.querySelectorAll('[srcset]').forEach((node) => {
    for (const candidate of parseSrcset(node.getAttribute('srcset'))) {
      addEntry(candidate, false);
    }
  });

  return Array.from(entries.values()).sort((left, right) => left.path.localeCompare(right.path));
}

function trimPathForChild(basePath: string): string {
  const normalized = normalizePath(basePath || '/');
  return /^[a-z][a-z0-9+.-]*:\/\/$/i.test(normalized)
    ? normalized
    : normalized.replace(/\/+$/, '');
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
    const normalized = normalizePath(path || '/');
    const driveMatch = /^(?:filesystem|drive):\/\/([a-z])(?:\/.*)?$/i.exec(normalized);
    if (driveMatch) return `filesystem://${driveMatch[1].toLowerCase()}`;
    const aliasMatch = /^([a-z][a-z0-9+.-]*):\/\//i.exec(normalized);
    if (aliasMatch) return `${aliasMatch[1].toLowerCase()}://`;
    return '/';
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmedBase = trimPathForChild(basePath);
    return `${trimmedBase}/${childName}`;
  }

  supportsStreaming(_path: string): boolean {
    return true;
  }
}

class AliasDirectoryProvider implements DirectoryListingProvider {
  readonly kind: DirectorySourceKind = 'filesystem';

  constructor(private readonly apiService: ApiService) {}

  canHandlePath(path: string): boolean {
    return isAliasPath(path) && !isRegistryPath(path);
  }

  canHandleEntries(_entries: FileInfo[]): boolean {
    return false;
  }

  listDirectory(path: string, showHidden: boolean): Promise<DirectorySourceListing> {
    if (isWebHttpPath(path)) {
      return this.listWebDirectory(path);
    }
    return this.apiService.listDirectory(path, showHidden);
  }

  getRootPath(path: string): string | null {
    const normalized = normalizePath(path);
    if (/^search:\/\//i.test(normalized) && normalized.toLowerCase() !== 'search://') {
      return normalized;
    }
    const aliasMatch = /^([a-z][a-z0-9+.-]*):\/\//i.exec(normalized);
    return aliasMatch ? `${aliasMatch[1].toLowerCase()}://` : null;
  }

  joinChildPath(basePath: string, childName: string): string {
    const normalizedBase = trimPathForChild(basePath);
    return `${normalizedBase}/${childName}`;
  }

  supportsStreaming(_path: string): boolean {
    return /^search:\/\//i.test(_path);
  }

  private async listWebDirectory(path: string): Promise<DirectorySourceListing> {
    const normalized = normalizePath(path);
    try {
      const content = await this.apiService.readFile(normalized, 'utf8');
      const entries = looksLikeHtmlDocument(content.content)
        ? discoverWebEntriesFromHtml(content.content, normalized)
        : [];
      return { path: normalized, entries };
    } catch (error) {
      const info = await this.apiService.getFileInfo(normalized).catch(() => null);
      if (info?.type === FileType.DIRECTORY) {
        return { path: normalized, entries: [] };
      }
      throw error;
    }
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
    return `${trimPathForChild(basePath)}/${childName}`;
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
      new AliasDirectoryProvider(this.apiService),
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
