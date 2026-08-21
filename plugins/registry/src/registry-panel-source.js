/**
 * Registry-owned PanelSource. Manager supplies only generic directory/VFS
 * capabilities through RegistryPanelSourceHost.
 */
export class RegistryPanelSource {
    constructor(host, activateValue) {
        this.host = host;
        this.activateValue = activateValue;
        this.id = 'registry';
        this.kind = 'registry';
        this.title = 'Registry';
    }
    canHandle(context) {
        return this.host.getPathKind(context.path, context.entries ?? []) === 'registry';
    }
    capabilities(context) {
        return this.host.capabilities(context.path);
    }
    async list(context, showHidden) {
        const listing = await this.host.listDirectory(context.path, showHidden);
        return {
            location: {
                path: listing.path || context.path,
                sourceId: this.id,
                sourceKind: this.kind,
            },
            entries: listing.entries,
        };
    }
    getRootPath(context) {
        return this.host.getRootPath(context.path, context.entries ?? []);
    }
    getParentPath(context) {
        return this.host.getParentPath(context.path);
    }
    joinChildPath(context, childName) {
        return this.host.joinChildPath(context.path, childName);
    }
    supportsStreaming(context) {
        return this.host.supportsStreaming(context.path);
    }
    activateEntry(context, entry) {
        if (entry.path === '__parent__')
            return false;
        const kind = String(entry.registryKind ?? '').toLowerCase();
        if (kind !== 'value')
            return false;
        return this.activateValue?.(context, entry) ?? false;
    }
    canPreviewEntry(_context, entry) {
        const kind = String(entry.registryKind ?? '').toLowerCase();
        return kind !== 'key' && entry.path !== '__parent__';
    }
}
//# sourceMappingURL=registry-panel-source.js.map