import { RegistryFileType as FileType, } from '../registry-model';
export class RegistryBackendError extends Error {
    constructor(message, method, causeValue) {
        super(message);
        this.method = method;
        this.causeValue = causeValue;
        this.name = 'RegistryBackendError';
    }
}
const ROOT = 'registry://';
export function normalizeRegistryPath(path) {
    const raw = (path || ROOT).trim().replace(/\\/g, '/');
    if (!/^registry:\/\//i.test(raw))
        return raw;
    return `registry://${raw.slice(raw.indexOf('://') + 3).replace(/^\/+/, '')}`;
}
export function joinRegistryPath(parts) {
    if (!parts.length)
        return ROOT;
    const [head, ...tail] = parts;
    let result = normalizeRegistryPath(head || ROOT);
    for (const part of tail) {
        const segment = String(part ?? '').replace(/^\/+|\/+$/g, '');
        if (!segment)
            continue;
        result = result === ROOT ? `${ROOT}${segment}` : `${result.replace(/\/+$/, '')}/${segment}`;
    }
    return normalizeRegistryPath(result);
}
function toRegistryFileInfo(entry) {
    const kind = String(entry.kind ?? '').toLowerCase();
    return {
        name: entry.name,
        path: normalizeRegistryPath(entry.path),
        type: kind === 'key' ? FileType.DIRECTORY : FileType.FILE,
        size: kind === 'value' ? new TextEncoder().encode(entry.preview ?? '').byteLength : 0,
        created: 0,
        modified: 0,
        accessed: 0,
        permissions: '',
        isHidden: false,
        registryKind: kind === 'key' ? 'Key' : 'Value',
        registryValueType: entry.valueType,
        registryPreview: entry.preview,
    };
}
export class RegistryBackendClient {
    constructor(transport) {
        this.transport = transport;
    }
    async initialize() {
        if (this.helloResult)
            return this.helloResult;
        const hello = await this.call('hello');
        if (hello.protocol !== 1) {
            throw new RegistryBackendError(`Unsupported Registry backend protocol ${hello.protocol}.`, 'hello');
        }
        if (hello.plugin !== 'reassembly.registry') {
            throw new RegistryBackendError(`Unexpected Registry backend identity: ${hello.plugin}.`, 'hello');
        }
        this.helloResult = hello;
        return hello;
    }
    async call(method, params) {
        try {
            return await this.transport.call(method, params);
        }
        catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            throw new RegistryBackendError(`Registry backend ${method} failed: ${message}`, method, error);
        }
    }
    async list(path) {
        const result = await this.call('registry.list', {
            path: normalizeRegistryPath(path),
        });
        return {
            path: normalizeRegistryPath(result.path),
            entries: (result.entries ?? []).map(toRegistryFileInfo),
        };
    }
    read(path) {
        return this.call('registry.read', {
            path: normalizeRegistryPath(path),
        });
    }
    async stat(path) {
        const normalized = normalizeRegistryPath(path).replace(/\/+$/, '');
        const body = normalized.slice(ROOT.length);
        const slash = body.lastIndexOf('/');
        const parent = slash < 0 ? ROOT : `${ROOT}${body.slice(0, slash)}`;
        const listing = await this.list(parent);
        const match = listing.entries.find(entry => normalizeRegistryPath(entry.path).replace(/\/+$/, '').toLowerCase() === normalized.toLowerCase());
        if (!match)
            throw new Error(`Registry entry not found: ${path}`);
        return match;
    }
    createKey(path) {
        return this.call('registry.createKey', { path: normalizeRegistryPath(path) });
    }
    write(path, type, data) {
        return this.call('registry.write', {
            path: normalizeRegistryPath(path),
            type,
            data,
        });
    }
    delete(path) {
        return this.call('registry.delete', { path: normalizeRegistryPath(path) });
    }
    rename(path, newName) {
        return this.call('registry.rename', {
            path: normalizeRegistryPath(path),
            newName,
        });
    }
}
export function createRegistryPanelSourceHost(backend) {
    return {
        getPathKind: path => /^registry:\/\//i.test(path) ? 'registry' : 'unknown',
        listDirectory: (path) => backend.list(path),
        getRootPath: path => /^registry:\/\//i.test(path) ? ROOT : null,
        getParentPath: path => {
            const normalized = normalizeRegistryPath(path).replace(/\/+$/, '');
            if (normalized === 'registry:' || normalized === ROOT.replace(/\/$/, ''))
                return ROOT;
            const body = normalized.slice(ROOT.length);
            const slash = body.lastIndexOf('/');
            return slash < 0 ? ROOT : `${ROOT}${body.slice(0, slash)}`;
        },
        joinChildPath: (path, childName) => joinRegistryPath([path, childName]),
        supportsStreaming: () => false,
        capabilities: () => ({
            read: true,
            create: true,
            delete: true,
            rename: true,
            trash: false,
        }),
    };
}
export function createRegistryOperations(backend, dialogs) {
    return {
        joinPaths: async (parts) => joinRegistryPath(parts),
        createDirectory: path => backend.createKey(path),
        writeFile: (path, content) => backend.write(path, 'REG_SZ', content),
        getFileInfo: path => backend.stat(path),
        moveFile: (source, destination) => {
            const name = destination.replace(/\/+$/, '').split('/').pop() ?? '';
            return backend.rename(source, name);
        },
        readFile: path => backend.read(path),
        writeRegistryValue: (path, type, data) => backend.write(path, type, data),
        deletePath: path => backend.delete(path),
        prompt: request => dialogs.prompt(request),
        confirm: request => dialogs.confirm(request),
    };
}
export const REGISTRY_HIVES = Object.freeze([
    'HKEY_CURRENT_USER',
    'HKEY_LOCAL_MACHINE',
    'HKEY_CLASSES_ROOT',
    'HKEY_USERS',
    'HKEY_CURRENT_CONFIG',
]);
export function registryHivePath(hive) {
    return `${ROOT}${hive}`;
}
export async function suggestRegistryChildren(backend, path, prefix = '') {
    const listing = await backend.list(path);
    const needle = prefix.trim().toLocaleLowerCase();
    return listing.entries
        .filter(entry => String(entry.registryKind ?? '').toLowerCase() === 'key')
        .filter(entry => !needle || entry.name.toLocaleLowerCase().startsWith(needle))
        .sort((a, b) => a.name.localeCompare(b.name));
}
//# sourceMappingURL=registry-backend.js.map