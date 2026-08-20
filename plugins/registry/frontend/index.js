/**
 * Reassembly universal Registry frontend.
 *
 * This entry is deliberately self-contained: it does not import Angular or a
 * Manager-owned Registry module. It registers a PanelSource through the generic
 * scoped plugin API and delegates Registry I/O to the plugin backend process.
 */

const ROOT = 'registry://';
let presentationModulePromise = null;

function loadPresentationModule() {
  if (!presentationModulePromise) {
    presentationModulePromise = import('../dist/fesm2022/reassembly-registry-plugin.mjs');
  }
  return presentationModulePromise;
}

function normalizePath(path) {
  const raw = String(path || ROOT).trim().replace(/\\/g, '/');
  if (!/^registry:\/\//i.test(raw)) return raw;

  // Registry locations are opaque hierarchical strings, not web URLs.
  // Preserve the spelling of every registry component exactly.
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

  // `name` is authoritative for Registry spelling because it comes directly
  // from Windows enumeration. Rebuild navigable child paths from the canonical
  // listing path + that exact name instead of propagating a stale/case-folded
  // entry.path from any transport/cache layer.
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

export async function activate(ctx) {
  if (!ctx?.api?.registerPanelSource || !ctx?.api?.registerPanelPresentation) {
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

  // ScopedPluginRegistry owns cleanup and removes this contribution when the
  // plugin is deactivated/uninstalled.
  ctx.api.registerPanelSource(source);
  ctx.api.registerPanelPresentation({
    id: 'reassembly.registry.presentation',
    score(context) {
      return context?.sourceKind === 'registry' &&
        (context?.viewMode === 'list' || context?.viewMode === 'details')
        ? 100
        : 0;
    },
    async load() {
      const module = await loadPresentationModule();
      return module.RegistryPanelPresentationComponent;
    },
  });

  return () => {
    // Intentionally empty. The host scope owns contribution cleanup.
  };
}
