/**
 * Reassembly universal Registry frontend.
 *
 * This entry is deliberately self-contained: it does not import Angular or a
 * Manager-owned Registry module. It registers a PanelSource through the generic
 * scoped plugin API and delegates Registry I/O to the plugin backend process.
 */

const ROOT = 'registry://';

function normalizePath(path) {
  const raw = String(path || ROOT).trim().replace(/\\/g, '/');
  if (!/^registry:\/\//i.test(raw)) return raw;
  return `registry://${raw.slice(raw.indexOf('://') + 3).replace(/^\/+/, '')}`;
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

function toFileInfo(entry) {
  const kind = String(entry?.kind ?? '').toLowerCase();
  const preview = String(entry?.preview ?? '');
  return {
    name: String(entry?.name ?? ''),
    path: normalizePath(entry?.path ?? ROOT),
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

export async function activate(ctx) {
  if (!ctx?.api?.registerPanelSource) {
    throw new Error('Universal host does not expose the scoped plugin registry API.');
  }
  if (!ctx?.backend?.call) {
    throw new Error('Universal host does not expose backend.call().');
  }

  const hello = await ctx.backend.call('hello');
  if (hello?.protocol !== 1 || hello?.plugin !== 'reassembly.registry') {
    throw new Error(`Unexpected Registry backend handshake: ${JSON.stringify(hello)}`);
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
      const result = await ctx.backend.call('registry.list', { path });
      return {
        location: {
          path: normalizePath(result?.path ?? path),
          sourceId: 'registry',
          sourceKind: 'registry',
        },
        entries: Array.isArray(result?.entries)
          ? result.entries.map(toFileInfo)
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

  // ScopedPluginRegistry owns cleanup and removes this contribution when the
  // plugin is deactivated/uninstalled.
  ctx.api.registerPanelSource(source);

  return () => {
    // Intentionally empty. The host scope owns contribution cleanup.
  };
}
