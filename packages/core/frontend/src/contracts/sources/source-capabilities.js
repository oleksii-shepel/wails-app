const READ_ONLY = Object.freeze({
    read: true, create: false, delete: false, rename: false, trash: false,
});
const MUTABLE = Object.freeze({
    read: true, create: true, delete: true, rename: true, trash: false,
});
const LOCAL = Object.freeze({ ...MUTABLE, trash: true });
export function getSourceCapabilities(location, options) {
    if (options?.inArchive)
        return MUTABLE;
    const value = (location ?? '').trim();
    const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec(value);
    if (!match)
        return LOCAL;
    switch (match[1].toLowerCase()) {
        case 'filesystem':
            return LOCAL;
        case 'memory':
        case 'smb':
        case 'registry':
            return MUTABLE;
        case 'gdrive':
        case 'dropbox':
        case 'onedrive':
        case 's3':
        case 'search':
        case 'http':
        case 'https':
            return READ_ONLY;
        default:
            return READ_ONLY;
    }
}
//# sourceMappingURL=source-capabilities.js.map