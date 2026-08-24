import { buildArchivePath, parseArchivePath } from '../services/archive.service';
import { normalizePath as normalizePathReference } from './address-bar.utils';
import type { FileViewDirectorySource } from '../services/directory-source.service';
import { FileType, type FileInfo } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import type { TreeNode } from './pane-tree.component';
import type { ViewLayout } from './file-view-container.component';

export interface FileViewTreeHost {
  get currentPath(): string;
  get currentView(): ViewLayout;
  get showHidden(): boolean;
  get entries(): FileInfo[];
  get treeNodes(): TreeNode[];
  set treeNodes(value: TreeNode[]);
  directorySource: FileViewDirectorySource;
  markForCheck(): void;
  sortTreeNodes(nodes: TreeNode[]): TreeNode[];
  expandTreeNode(path: string): void;
  centerTreeNode(path: string): void;
}

export class FileViewTreeController {
  private readonly nodeLoadPromises = new Map<string, Promise<void>>();
  private syncToken = 0;

  reset(): void {
    this.nodeLoadPromises.clear();
    this.syncToken += 1;
  }

  async syncToCurrentPath(host: FileViewTreeHost): Promise<void> {
    if (host.currentView !== 'tree') return;
    this.buildTreeNodes(host);

    const rootNode = host.treeNodes[0];
    if (!rootNode) return;

    const token = ++this.syncToken;
    const chain = this.buildPathChain(rootNode.path, host.currentPath);
    if (chain.length === 0) return;

    let current = rootNode;
    const expandedNodes: TreeNode[] = [rootNode];
    await this.ensureNodeChildrenLoaded(host, current);
    if (token !== this.syncToken) return;

    for (let index = 1; index < chain.length; index += 1) {
      const expected = this.normalizePath(chain[index]);
      const next = current.children?.find((child) => this.normalizePath(child.path) === expected);
      if (!next || (next.type !== FileType.DIRECTORY && next.type !== FileType.ARCHIVE)) break;
      current = next;
      expandedNodes.push(current);
      await this.ensureNodeChildrenLoaded(host, current);
      if (token !== this.syncToken) return;
    }

    setTimeout(() => {
      if (token !== this.syncToken || host.currentView !== 'tree') return;
      for (const node of expandedNodes) {
        host.expandTreeNode(node.path);
      }
      host.centerTreeNode(current.path);
    }, 0);
  }

  buildTreeNodes(host: FileViewTreeHost): void {
    const rootPath = this.getRootPath(host);
    if (!rootPath) {
      host.treeNodes = [];
      return;
    }

    const currentRoot = host.treeNodes[0];
    if (currentRoot && this.normalizePath(currentRoot.path) === this.normalizePath(rootPath)) {
      return;
    }

    this.nodeLoadPromises.clear();
    host.treeNodes = [this.createRootNode(rootPath)];
    host.markForCheck();
  }

  async ensureNodeChildrenLoaded(host: FileViewTreeHost, node: TreeNode): Promise<void> {
    if (node.loaded) return;

    const existingPromise = this.nodeLoadPromises.get(node.path);
    if (existingPromise) {
      await existingPromise;
      return;
    }

    const loadPromise = this.populateNodeChildren(host, node);
    this.nodeLoadPromises.set(node.path, loadPromise);
    try {
      await loadPromise;
    } finally {
      this.nodeLoadPromises.delete(node.path);
    }
  }

  refreshTreeNode(host: FileViewTreeHost, path: string): void {
    const target = this.normalizePath(path);
    const node = this.findTreeNodeByPath(host.treeNodes, target);
    if (!node || !node.loaded) return;
    node.loaded = false;
    host.treeNodes = [...host.treeNodes];
    host.markForCheck();
    void this.ensureNodeChildrenLoaded(host, node);
  }

  normalizePath(path: string): string {
    return normalizePathReference(path || '/');
  }

  private async populateNodeChildren(host: FileViewTreeHost, node: TreeNode): Promise<void> {
    node.loading = true;
    node.children = node.children?.length ? node.children : [];
    host.markForCheck();

    try {
      const context = node.archivePath ? parseArchivePath(node.archivePath) : null;
      const listing = context
        ? await host.directorySource.browseArchive(context.fsPath, context.nestedChain, context.innerPath)
        : await host.directorySource.listDirectory(node.path, host.showHidden);
      const childLevel = (node.level ?? 0) + 1;
      const existingChildren = new Map<string, TreeNode>();
      for (const child of node.children ?? []) {
        existingChildren.set(this.normalizePath(child.path), child);
      }

      const rawChildren: TreeNode[] = (listing.entries || [])
        .filter((entry) => entry.path !== PARENT_ENTRY_PATH)
        .map((entry) => this.toChildNode(host, entry as FileInfo, node, context, childLevel, existingChildren));

      node.children = host.sortTreeNodes(rawChildren);
      node.loaded = true;
      node.loading = false;
      host.treeNodes = [...host.treeNodes];
      host.markForCheck();
    } catch {
      node.children = [];
      node.loading = false;
      node.loaded = true;
      host.treeNodes = [...host.treeNodes];
      host.markForCheck();
    }
  }

  private toChildNode(
    host: FileViewTreeHost,
    entry: FileInfo,
    node: TreeNode,
    context: ReturnType<typeof parseArchivePath> | null,
    childLevel: number,
    existingChildren: Map<string, TreeNode>,
  ): TreeNode {
    const entryNestedChain = (entry as any).nestedChain as string[] | undefined;
    const isArchiveEntry = entry.type === FileType.ARCHIVE;

    let childArchivePath: string | undefined;
    if (context) {
      if (isArchiveEntry) {
        const newChain = entryNestedChain?.length ? entryNestedChain : [...context.nestedChain, entry.path];
        childArchivePath = buildArchivePath(context.fsPath, newChain, '');
      } else {
        childArchivePath = buildArchivePath(context.fsPath, context.nestedChain, entry.path);
      }
    } else {
      childArchivePath = isArchiveEntry ? entry.path : undefined;
    }

    const childPath = context
      ? host.directorySource.joinChildPath(node.path, entry.name)
      : entry.path;

    const existing = existingChildren.get(this.normalizePath(childPath));
    return {
      ...entry,
      path: childPath,
      level: childLevel,
      expandable: entry.type === FileType.DIRECTORY || entry.type === FileType.ARCHIVE,
      expanded: existing?.expanded ?? false,
      children: existing?.children ?? ((entry.type === FileType.DIRECTORY || entry.type === FileType.ARCHIVE) ? [] : undefined),
      loaded: existing?.loaded ?? false,
      loading: false,
      renaming: existing?.renaming ?? false,
      editingName: existing?.editingName,
      parent: node,
      archivePath: childArchivePath,
    } as TreeNode;
  }

  private createRootNode(rootPath: string, archivePath?: string): TreeNode {
    const isWindowsRoot = /^filesystem:\/\/[a-z]$/i.test(rootPath);
    const isRegistryRoot = rootPath === 'registry://';
    const rootName = archivePath
      ? rootPath.split('/').filter(Boolean).pop() || rootPath
      : (isRegistryRoot ? 'Registry' : (rootPath === '/' ? '/' : (isWindowsRoot ? `${rootPath.slice(-1).toUpperCase()}:` : rootPath)));
    return {
      name: rootName,
      path: rootPath,
      type: FileType.DIRECTORY,
      size: 0,
      modified: 0,
      created: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
      registryKind: isRegistryRoot ? 'Registry' : undefined,
      level: -1,
      syntheticRoot: rootPath,
      expandable: true,
      expanded: true,
      children: [],
      loaded: false,
      loading: false,
      parent: undefined,
      archivePath,
    } as TreeNode;
  }

  private getRootPath(host: FileViewTreeHost): string {
    return host.directorySource.getRootPath(this.normalizePath(host.currentPath || '/'), host.entries) ?? '/';
  }

  private buildPathChain(rootPath: string, targetPath: string): string[] {
    const root = this.normalizePath(rootPath);
    const target = this.normalizePath(targetPath || rootPath);

    const chain = [root];
    if (root === 'registry://') {
      if (target === root) return chain;
      const relative = target.slice(root.length).replace(/^\/+/, '');
      if (!relative) return chain;

      let current = root;
      for (const segment of relative.split('/').filter(Boolean)) {
        current = `${current}${segment}`;
        chain.push(this.normalizePath(current));
        current = `${current}/`;
      }
      return chain;
    }

    const rootForPrefix = root.endsWith('/') ? root : `${root}/`;
    const startsAtRoot = target === root || this.normalizePath(target).startsWith(rootForPrefix);
    if (!startsAtRoot) return chain;
    if (target === root) return chain;

    const relative = target.slice(root.length).replace(/^\/+/, '');
    if (!relative) return chain;

    let current = root;
    for (const segment of relative.split('/').filter(Boolean)) {
      current = current === '/' ? `/${segment}` : `${current.replace(/\/$/, '')}/${segment}`;
      chain.push(this.normalizePath(current));
    }
    return chain;
  }

  private findTreeNodeByPath(nodes: TreeNode[], target: string): TreeNode | undefined {
    for (const node of nodes) {
      if (this.normalizePath(node.path) === target) return node;
      if (node.children) {
        const found = this.findTreeNodeByPath(node.children, target);
        if (found) return found;
      }
    }
    return undefined;
  }
}
