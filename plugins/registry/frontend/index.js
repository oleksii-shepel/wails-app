/**
 * Reassembly universal Registry frontend.
 *
 * Keep this entry intentionally simple while we rework pane architecture:
 * the plugin owns the `registry://` source and backend bridge, while Manager
 * continues to render the collection with its stable host-side pane pipeline.
 */

const ROOT = 'registry://';
const ROOT_HIVES = Object.freeze([
  'HKEY_CLASSES_ROOT',
  'HKEY_CURRENT_USER',
  'HKEY_LOCAL_MACHINE',
  'HKEY_USERS',
  'HKEY_CURRENT_CONFIG',
]);

function normalizePath(path) {
  const raw = String(path || ROOT).trim().replace(/\\/g, '/');
  if (!/^registry:\/\//i.test(raw)) return raw;
  const body = raw.slice(raw.indexOf('://') + 3).replace(/^\/+/, '');
  return body ? `${ROOT}${body}` : ROOT;
}

function joinPath(base, child) {
  const b = normalizePath(base).replace(/\/+$/, '');
  const c = String(child ?? '').replace(/^\/+|\/+$/g, '');
  if (!c) return normalizePath(base);
  if (b === 'registry:' || b === 'registry://') return `${ROOT}${c}`;
  return `${b}/${c}`;
}

function parentPath(path) {
  const normalized = normalizePath(path).replace(/\/+$/, '');
  if (normalized === 'registry:' || normalized === 'registry://') return ROOT;
  const body = normalized.slice(ROOT.length);
  const slash = body.lastIndexOf('/');
  return slash < 0 ? ROOT : `${ROOT}${body.slice(0, slash)}`;
}

function toFileInfo(entry, basePath) {
  const kind = String(entry?.kind ?? '').toLowerCase();
  const preview = String(entry?.preview ?? '');
  const name = String(entry?.name ?? '');
  const path = name
    ? joinPath(basePath, name)
    : normalizePath(entry?.path ?? basePath ?? ROOT);

  return {
    name,
    path,
    type: kind === 'key' ? 'DIRECTORY' : 'FILE',
    size: kind === 'value' ? new TextEncoder().encode(preview).byteLength : 0,
    created: 0,
    modified: 0,
    accessed: 0,
    permissions: '',
    isHidden: false,
    registryKind: kind === 'key' ? 'Key' : 'Value',
    registryValueType: entry?.valueType,
    registryPreview: preview,
  };
}

function rootListing() {
  return {
    path: ROOT,
    entries: ROOT_HIVES.map(name => toFileInfo({
      name,
      path: `${ROOT}${name}`,
      kind: 'key',
    }, ROOT)),
  };
}

async function backendCall(ctx, method, params, timeoutMs = 4000) {
  return await Promise.race([
    ctx.backend.call(method, params),
    new Promise((_, reject) => {
      const timeout = setTimeout(() => {
        clearTimeout(timeout);
        reject(new Error(`Registry backend ${method} timed out after ${timeoutMs} ms.`));
      }, timeoutMs);
    }),
  ]);
}

export async function activate(ctx) {
  if (!ctx?.api?.registerPanelSource) {
    throw new Error('Universal host does not expose the scoped plugin registry API.');
  }
  if (!ctx?.backend?.call) {
    throw new Error('Universal host does not expose backend.call().');
  }

  const source = {
    id: 'registry',
    kind: 'registry',
    title: 'Registry',

    canHandle(context) {
      return /^registry:\/\//i.test(String(context?.path ?? ''));
    },

    capabilities() {
      return {
        read: true,
        create: true,
        delete: true,
        rename: true,
        trash: false,
      };
    },

    async list(context) {
      const path = normalizePath(context?.path ?? ROOT);
      const result = path === ROOT
        ? rootListing()
        : await backendCall(ctx, 'registry.list', { path });
      const locationPath = normalizePath(result?.path ?? path);
      return {
        location: {
          path: locationPath,
          sourceId: 'registry',
          sourceKind: 'registry',
        },
        entries: Array.isArray(result?.entries)
          ? result.entries.map(entry => toFileInfo(entry, locationPath))
          : [],
      };
    },

    getRootPath() {
      return ROOT;
    },

    getParentPath(context) {
      return parentPath(context?.path ?? ROOT);
    },

    joinChildPath(context, childName) {
      return joinPath(context?.path ?? ROOT, childName);
    },

    supportsStreaming() {
      return false;
    },
  };

  ctx.api.registerPanelSource(source);

  return () => {
    // ScopedPluginRegistry owns cleanup and unregisters contributions.
  };
}
