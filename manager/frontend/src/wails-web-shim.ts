/* Web runtime shim providing window.runtime and window.go.app.App for browser execution. */

type ListenerEntry = {
  callback: (...args: any[]) => void;
  remaining: number;
};

const eventListeners = new Map<string, ListenerEntry[]>();

function installRuntime() {
  const w = window as any;
  if (w.runtime?.EventsOn) return;

  w.runtime = {
    LogPrint: (msg: string) => console.log(msg),
    LogTrace: (msg: string) => console.debug(msg),
    LogDebug: (msg: string) => console.debug(msg),
    LogInfo: (msg: string) => console.info(msg),
    LogWarning: (msg: string) => console.warn(msg),
    LogError: (msg: string) => console.error(msg),
    LogFatal: (msg: string) => console.error(msg),
    EventsOnMultiple: (eventName: string, callback: (...args: any[]) => void, maxCallbacks: number) => {
      const list = eventListeners.get(eventName) ?? [];
      const entry: ListenerEntry = { callback, remaining: maxCallbacks };
      list.push(entry);
      eventListeners.set(eventName, list);
      return () => {
        const current = eventListeners.get(eventName);
        if (!current) return;
        const idx = current.indexOf(entry);
        if (idx >= 0) current.splice(idx, 1);
      };
    },
    EventsOn: (eventName: string, callback: (...args: any[]) => void) => {
      return w.runtime.EventsOnMultiple(eventName, callback, -1);
    },
    EventsOnce: (eventName: string, callback: (...args: any[]) => void) => {
      return w.runtime.EventsOnMultiple(eventName, callback, 1);
    },
    EventsOff: (eventName: string, ...additional: string[]) => {
      for (const name of [eventName, ...additional]) {
        eventListeners.delete(name);
      }
    },
    EventsOffAll: () => {
      eventListeners.clear();
    },
    EventsEmit: (eventName: string, ...args: any[]) => {
      const list = eventListeners.get(eventName);
      if (!list?.length) return;
      for (const entry of [...list]) {
        try {
          entry.callback(...args);
        } catch (err) {
          console.error(`Event listener error for ${eventName}:`, err);
        }
        if (entry.remaining > 0) {
          entry.remaining -= 1;
          if (entry.remaining === 0) {
            const idx = list.indexOf(entry);
            if (idx >= 0) list.splice(idx, 1);
          }
        }
      }
    },
    WindowReload: () => window.location.reload(),
    WindowReloadApp: () => window.location.reload(),
    WindowSetAlwaysOnTop: () => {},
    WindowSetSystemDefaultTheme: () => {},
    WindowSetLightTheme: () => {},
    WindowSetDarkTheme: () => {},
    WindowCenter: () => {},
    WindowSetTitle: (title: string) => { document.title = title; },
    WindowFullscreen: () => { void document.documentElement.requestFullscreen?.().catch(() => {}); },
    WindowUnfullscreen: () => { void document.exitFullscreen?.().catch(() => {}); },
    WindowIsFullscreen: async () => !!document.fullscreenElement,
    WindowGetSize: async () => ({ w: window.innerWidth, h: window.innerHeight }),
    WindowSetSize: () => {},
    WindowSetMaxSize: () => {},
    WindowSetMinSize: () => {},
    WindowSetPosition: () => {},
    WindowGetPosition: async () => ({ x: window.screenX || 0, y: window.screenY || 0 }),
    WindowHide: () => {},
    WindowShow: () => {},
    WindowMaximise: () => {},
    WindowToggleMaximise: () => {},
    WindowUnmaximise: () => {},
    WindowIsMaximised: async () => false,
    WindowMinimise: () => {},
    WindowUnminimise: () => {},
    WindowIsMinimised: async () => false,
    WindowIsNormal: async () => true,
    WindowSetBackgroundColour: () => {},
    ScreenGetAll: async () => [{
      isCurrent: true,
      isPrimary: true,
      width: window.screen.width,
      height: window.screen.height,
      size: { width: window.screen.width, height: window.screen.height },
      physicalSize: { width: window.screen.width, height: window.screen.height },
    }],
    BrowserOpenURL: (url: string) => {
      try {
        const a = document.createElement('a');
        a.href = url;
        a.target = '_blank';
        a.rel = 'noopener noreferrer';
        a.click();
      } catch {}
    },
    Environment: async () => ({ buildType: 'web', platform: 'linux', arch: 'x64' }),
    Quit: () => {},
    Hide: () => {},
    Show: () => {},
    ClipboardGetText: async () => {
      try { return await navigator.clipboard.readText(); } catch { return ''; }
    },
    ClipboardSetText: async (text: string) => {
      try { await navigator.clipboard.writeText(text); return true; } catch { return false; }
    },
    OnFileDrop: () => {},
    OnFileDropOff: () => {},
    CanResolveFilePaths: () => false,
    ResolveFilePaths: () => {},
    InitializeNotifications: async () => {},
    CleanupNotifications: async () => {},
    IsNotificationAvailable: async () => false,
    RequestNotificationAuthorization: async () => false,
    CheckNotificationAuthorization: async () => false,
    SendNotification: async () => {},
    SendNotificationWithActions: async () => {},
    RegisterNotificationCategory: async () => {},
    RemoveNotificationCategory: async () => {},
    RemoveAllPendingNotifications: async () => {},
    RemovePendingNotification: async () => {},
    RemoveAllDeliveredNotifications: async () => {},
    RemoveDeliveredNotification: async () => {},
    RemoveNotification: async () => {},
  };
}

async function apiJson<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data?.error || data?.message || `HTTP ${res.status}`);
  }
  return data as T;
}

function triggerBrowserDownload(url: string, filename?: string): void {
  const a = document.createElement('a');
  a.href = url;
  a.download = filename || '';
  a.rel = 'noopener noreferrer';
  document.body.appendChild(a);
  a.click();
  a.remove();
}

const activeStreams = new Map<string, {
  streamId: string;
  paneId: string;
  path: string;
  entries: any[];
  sourceCount: number;
  cancelled: boolean;
}>();

const activeDiskScans = new Map<string, { cancelled: boolean }>();

function sortEntries(entries: any[], options?: any): any[] {
  const field = options?.sortField || 'name';
  const dir = options?.sortDirection === 'desc' ? -1 : 1;
  const dirsFirst = options?.directoriesFirst !== false;
  const filter = String(options?.filterText || '').trim().toLowerCase();

  const filtered = filter
    ? entries.filter((e) => String(e.name || '').toLowerCase().includes(filter))
    : [...entries];

  filtered.sort((a, b) => {
    if (dirsFirst) {
      const aDir = a.type === 'DIRECTORY' ? 0 : 1;
      const bDir = b.type === 'DIRECTORY' ? 0 : 1;
      if (aDir !== bDir) return aDir - bDir;
    }
    if (field === 'size') return (a.size - b.size) * dir;
    if (field === 'modified') return (a.modified - b.modified) * dir;
    if (field === 'type') return String(a.mimeType || '').localeCompare(String(b.mimeType || '')) * dir;
    return String(a.name || '').localeCompare(String(b.name || ''), undefined, { numeric: true, sensitivity: 'base' }) * dir;
  });
  return filtered;
}

function parentOf(p: string): string {
  const norm = (p || '/').replace(/\\/g, '/').replace(/\/+$/, '') || '/';
  if (norm === '/') return '/';
  const idx = norm.lastIndexOf('/');
  return idx <= 0 ? '/' : norm.slice(0, idx);
}

function installGoAppBridge() {
  const w = window as any;
  if (w.go?.app?.App) return;

  const App: Record<string, (...args: any[]) => any> = {
    Ping: async () => Date.now(),
    Shutdown: async () => {},
    InitVFS: async () => {},
    EmitEvent: async (name: string, data: any) => w.runtime.EventsEmit(name, data),

    GetOSInfo: () => apiJson('/api/os-info'),
    GetDrives: () => apiJson('/api/drives'),
    ListDrives: () => apiJson('/api/drives'),
    GetSpecialFolders: () => apiJson('/api/special-folders'),
    GetPathCapacity: (p: string) =>
      apiJson('/api/fs/capacity', { method: 'POST', body: JSON.stringify({ path: p || '/' }) }),

    ListDir: (p: string, showHidden = false) =>
      apiJson('/api/fs/list', { method: 'POST', body: JSON.stringify({ path: p, showHidden }) }),
    ListDirectory: (p: string, showHidden = false) =>
      apiJson('/api/fs/list', { method: 'POST', body: JSON.stringify({ path: p, showHidden }) }),
    VFSListDir: (p: string) =>
      apiJson('/api/fs/list', { method: 'POST', body: JSON.stringify({ path: p, showHidden: false }) }),
    ListDirFiltered: async (p: string, showHidden = false, filter?: any) => {
      const listing = await apiJson<any>('/api/fs/list', { method: 'POST', body: JSON.stringify({ path: p, showHidden }) });
      const query = String(filter?.query || filter?.pattern || '').toLowerCase();
      if (query) {
        listing.entries = listing.entries.filter((e: any) => String(e.name).toLowerCase().includes(query));
      }
      return listing;
    },

    StartListDirStream: (paneId: string, p: string, showHidden: boolean) =>
      App['StartListDirStreamWithOptions'](paneId, p, showHidden, undefined),
    StartListDirStreamWithOptions: async (paneId: string, p: string, showHidden: boolean, options?: any) => {
      const streamId = `stream-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
      const record = { streamId, paneId, path: p, entries: [] as any[], sourceCount: 0, cancelled: false };
      activeStreams.set(streamId, record);

      setTimeout(async () => {
        if (record.cancelled) return;
        try {
          const listing = await apiJson<any>('/api/fs/list', {
            method: 'POST',
            body: JSON.stringify({ path: p, showHidden }),
          });
          if (record.cancelled) return;
          record.path = listing.path || p;
          record.sourceCount = listing.entries.length;
          record.entries = sortEntries(listing.entries, options);

          w.runtime.EventsEmit('fs:listStream:meta', {
            streamId,
            paneId,
            path: record.path,
            showHidden,
            sourceCount: record.sourceCount,
            totalCount: record.entries.length,
            generation: 1,
            enumerateMs: 2,
          });
          w.runtime.EventsEmit('fs:listStream:batch', {
            streamId,
            generation: 1,
            paneId,
            path: record.path,
            entries: record.entries,
          });
          w.runtime.EventsEmit('fs:listStream:done', {
            streamId,
            paneId,
            path: record.path,
            entryCount: record.entries.length,
            sourceCount: record.sourceCount,
            totalSize: listing.totalSize || 0,
            durationMs: 4,
            enumerateMs: 2,
            indexMs: 1,
            sortMs: 1,
            generation: 1,
          });
        } catch (err) {
          if (record.cancelled) return;
          w.runtime.EventsEmit('fs:listStream:error', {
            streamId,
            generation: 1,
            paneId,
            path: p,
            error: err instanceof Error ? err.message : String(err),
          });
        }
      }, 0);

      return streamId;
    },
    ReadListDirStreamWindow: async (streamId: string, offset: number, limit: number) => {
      const record = activeStreams.get(streamId);
      if (!record) {
        return { streamId, paneId: 'left', path: '', offset, limit, totalCount: 0, sourceCount: 0, entries: [] };
      }
      return {
        streamId,
        paneId: record.paneId,
        path: record.path,
        offset,
        limit,
        totalCount: record.entries.length,
        sourceCount: record.sourceCount,
        entries: record.entries.slice(offset, offset + limit),
      };
    },
    CancelListDirStream: async (streamId: string) => {
      const record = activeStreams.get(streamId);
      if (record) record.cancelled = true;
      activeStreams.delete(streamId);
    },
    CancelDirStreams: async () => {
      for (const rec of activeStreams.values()) rec.cancelled = true;
      activeStreams.clear();
    },

    FileInfo: (p: string) => apiJson('/api/fs/info', { method: 'POST', body: JSON.stringify({ path: p }) }),
    GetFileInfo: (p: string) => apiJson('/api/fs/info', { method: 'POST', body: JSON.stringify({ path: p }) }),
    ProbeFile: (p: string) => apiJson('/api/fs/probe', { method: 'POST', body: JSON.stringify({ path: p }) }),
    PathExists: async (p: string) => {
      const res = await apiJson<{ exists: boolean }>('/api/fs/exists', { method: 'POST', body: JSON.stringify({ path: p }) });
      return res.exists;
    },
    IsDirectory: async (p: string) => {
      try {
        const info = await App['FileInfo'](p);
        return info.type === 'DIRECTORY';
      } catch {
        return false;
      }
    },
    IsRoot: async (p: string) => p === '/' || p === '',
    IsAbsPath: async (p: string) => p.startsWith('/') || /^[a-z]:/i.test(p),
    GetParentPath: async (p: string) => parentOf(p),
    JoinPaths: async (parts: string[]) => parts.join('/').replace(/\/+/g, '/'),
    SplitPath: async (p: string) => {
      const norm = (p || '/').replace(/\\/g, '/');
      const idx = norm.lastIndexOf('/');
      return { dir: idx <= 0 ? '/' : norm.slice(0, idx), base: norm.slice(idx + 1) };
    },
    Breadcrumbs: async (p: string) => {
      const parts = (p || '/').split('/').filter(Boolean);
      const crumbs = [{ name: '/', path: '/' }];
      let cur = '';
      for (const part of parts) {
        cur += '/' + part;
        crumbs.push({ name: part, path: cur });
      }
      return crumbs;
    },

    ReadFile: (p: string, encoding = 'utf8') =>
      apiJson('/api/fs/read', { method: 'POST', body: JSON.stringify({ path: p, encoding }) }),
    ReadFileContent: (p: string, encoding = 'utf8') =>
      apiJson('/api/fs/read', { method: 'POST', body: JSON.stringify({ path: p, encoding }) }),
    VFSReadFile: (p: string, encoding = 'utf8') =>
      apiJson('/api/fs/read', { method: 'POST', body: JSON.stringify({ path: p, encoding }) }),
    ReadFileRange: (p: string, offset: number, length: number) =>
      apiJson('/api/fs/read-range', { method: 'POST', body: JSON.stringify({ path: p, offset, length }) }),
    ReadFileStream: async () => '',
    CancelReadFileStream: async () => {},

    WriteFile: (p: string, content: string, encoding = 'utf8') =>
      apiJson('/api/fs/write', { method: 'POST', body: JSON.stringify({ path: p, content, encoding }) }),
    WriteFileContent: (p: string, content: string, encoding = 'utf8') =>
      apiJson('/api/fs/write', { method: 'POST', body: JSON.stringify({ path: p, content, encoding }) }),
    VFSWriteFile: (p: string, content: string, encoding = 'utf8') =>
      apiJson('/api/fs/write', { method: 'POST', body: JSON.stringify({ path: p, content, encoding }) }),

    CreateDir: (p: string, recursive = true) =>
      apiJson('/api/fs/mkdir', { method: 'POST', body: JSON.stringify({ path: p, recursive }) }),
    CreateDirectory: (p: string, recursive = true) =>
      apiJson('/api/fs/mkdir', { method: 'POST', body: JSON.stringify({ path: p, recursive }) }),
    VFSMkdir: (p: string) =>
      apiJson('/api/fs/mkdir', { method: 'POST', body: JSON.stringify({ path: p, recursive: true }) }),

    Delete: (p: string, recursive = true, useTrash = false) =>
      apiJson('/api/fs/delete', { method: 'POST', body: JSON.stringify({ path: p, recursive, useTrash }) }),
    DeleteFile: (p: string, recursive = true, useTrash = false) =>
      apiJson('/api/fs/delete', { method: 'POST', body: JSON.stringify({ path: p, recursive, useTrash }) }),
    VFSRemove: (p: string) =>
      apiJson('/api/fs/delete', { method: 'POST', body: JSON.stringify({ path: p, recursive: true, useTrash: false }) }),

    Move: (src: string, dest: string) =>
      apiJson('/api/fs/move', { method: 'POST', body: JSON.stringify({ source: src, destination: dest }) }),
    MoveFile: (src: string, dest: string) =>
      apiJson('/api/fs/move', { method: 'POST', body: JSON.stringify({ source: src, destination: dest }) }),
    Copy: (src: string, dest: string, recursive = true) =>
      apiJson('/api/fs/copy', { method: 'POST', body: JSON.stringify({ source: src, destination: dest, recursive }) }),
    CopyFile: (src: string, dest: string, recursive = true) =>
      apiJson('/api/fs/copy', { method: 'POST', body: JSON.stringify({ source: src, destination: dest, recursive }) }),

    BatchCopy: async (req: any) => {
      const items = req?.items || [];
      let succeeded = 0;
      const errors: any[] = [];
      const affectedPaths: any[] = [];
      for (const item of items) {
        try {
          const result: any = await App['CopyFile'](item.source, item.destination, true);
          succeeded++;
          affectedPaths.push({
            source: item.source,
            destination: result?.affectedPaths?.[0] || item.destination,
          });
        } catch (err: any) {
          errors.push({ source: item.source, destination: item.destination, error: err?.message || String(err) });
        }
      }
      return { operationId: `batch-${Date.now()}`, total: items.length, succeeded, skipped: 0, failed: errors.length, errors, affectedPaths };
    },
    BatchMove: async (req: any) => {
      const items = req?.items || [];
      let succeeded = 0;
      const errors: any[] = [];
      const affectedPaths: any[] = [];
      for (const item of items) {
        try {
          await App['MoveFile'](item.source, item.destination);
          succeeded++;
          affectedPaths.push({ source: item.source, destination: item.destination });
        } catch (err: any) {
          errors.push({ source: item.source, destination: item.destination, error: err?.message || String(err) });
        }
      }
      return { operationId: `batch-${Date.now()}`, total: items.length, succeeded, skipped: 0, failed: errors.length, errors, affectedPaths };
    },
    BatchDelete: (req: any) =>
      apiJson('/api/fs/batch-delete', {
        method: 'POST',
        body: JSON.stringify({
          paths: req?.paths || [],
          recursive: req?.recursive !== false,
          useTrash: req?.useTrash !== false,
        }),
      }),

    Search: (p: string, pattern: string, recursive = true) =>
      apiJson('/api/fs/search', { method: 'POST', body: JSON.stringify({ path: p, pattern, recursive }) }),
    SearchFiles: (p: string, pattern: string, recursive = true) =>
      apiJson('/api/fs/search', { method: 'POST', body: JSON.stringify({ path: p, pattern, recursive }) }),
    BuildSearchAlias: async (p: string, pattern: string, recursive = true) =>
      `search://?path=${encodeURIComponent(p)}&pattern=${encodeURIComponent(pattern)}&recursive=${recursive}`,

    HashFile: async (p: string) => {
      const res = await apiJson<{ hash: string }>('/api/fs/hash', { method: 'POST', body: JSON.stringify({ path: p }) });
      return res.hash;
    },
    GetFileChecksums: (p: string) =>
      apiJson('/api/fs/hash', { method: 'POST', body: JSON.stringify({ path: p }) }),

    AnalyzeDiskUsage: (p: string) =>
      apiJson('/api/fs/disk-usage', { method: 'POST', body: JSON.stringify({ path: p }) }),
    StartDiskUsageStream: async (p: string) => {
      const scanId = `du-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
      const scan = { cancelled: false };
      activeDiskScans.set(scanId, scan);
      setTimeout(async () => {
        if (scan.cancelled) return;
        try {
          const result: any = await apiJson('/api/fs/disk-usage', { method: 'POST', body: JSON.stringify({ path: p }) });
          if (scan.cancelled) return;
          for (const entry of result.entries || []) {
            w.runtime.EventsEmit('disk-usage:entry', { scanId, entry });
          }
          w.runtime.EventsEmit('disk-usage:done', { scanId, result });
        } catch (err) {
          w.runtime.EventsEmit('disk-usage:error', {
            scanId,
            error: err instanceof Error ? err.message : String(err),
          });
        }
      }, 0);
      return scanId;
    },
    CancelDiskUsageStream: async (scanId: string) => {
      const scan = activeDiskScans.get(scanId);
      if (scan) scan.cancelled = true;
      activeDiskScans.delete(scanId);
    },
    DirSizeRecursive: async (p: string) => {
      const res: any = await apiJson('/api/fs/disk-usage', { method: 'POST', body: JSON.stringify({ path: p }) });
      return res.totalSize || 0;
    },

    StreamBaseURL: async () => window.location.origin,
    SetStreamBaseURL: async () => {},
    GetStreamURL: async (p: string) => `/api/stream?path=${encodeURIComponent(p)}`,
    GetImageThumbnailURL: async (p: string) => `/api/stream?path=${encodeURIComponent(p)}`,
    GetVideoPreviewURL: async (p: string) => `/api/stream?path=${encodeURIComponent(p)}`,
    GetVideoPlaybackPlan: async (p: string) => ({
      size: 0,
      format: p.split('.').pop() || 'mp4',
      playbackMode: 'native',
      streamUrl: `/api/stream?path=${encodeURIComponent(p)}`,
      seekable: true,
      useNativePlayer: true,
    }),

    GetWorkspaces: () => apiJson('/api/workspaces'),
    SaveWorkspace: (ws: any) => apiJson('/api/workspaces', { method: 'POST', body: JSON.stringify(ws) }),
    LoadWorkspace: async (id: string) => {
      const list: any = await apiJson('/api/workspaces');
      return list.workspaces.find((x: any) => x.id === id) || list.workspaces[0];
    },
    DeleteWorkspace: (id: string) => apiJson(`/api/workspaces/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    GetPreferences: () => apiJson('/api/preferences'),
    SavePreferences: (prefs: any) => apiJson('/api/preferences', { method: 'POST', body: JSON.stringify(prefs) }),
    NormalizeWindowStateForSave: async (state: any) => state,
    GetMaxWindowViewport: async () => ({ width: window.innerWidth, height: window.innerHeight, x: 0, y: 0 }),

    NavNavigate: (_paneId: string, p: string) => App['ListDir'](p, false),
    NavNavigateShowHidden: (_paneId: string, p: string, showHidden: boolean) => App['ListDir'](p, showHidden),
    NavBack: async () => ({ path: '/', entries: [], totalSize: 0 }),
    NavForward: async () => ({ path: '/', entries: [], totalSize: 0 }),
    NavGoUp: async () => ({ path: '/', entries: [], totalSize: 0 }),

    GetActiveOperations: async () => [],
    CancelOperation: async () => false,
    DismissOperation: async () => {},
    ResolveConflict: async () => false,

    ListPlugins: () => apiJson('/api/plugins'),
    DiscoverPluginPackages: () => apiJson('/api/plugins'),
    GetPluginPackageStates: () => apiJson('/api/plugins/states'),
    GetPluginPackageState: (id: string) =>
      apiJson(`/api/plugins/${encodeURIComponent(id)}/state`),
    UpdatePluginPackageState: (id: string, patch: any) =>
      apiJson(`/api/plugins/${encodeURIComponent(id)}/state`, {
        method: 'PATCH',
        body: JSON.stringify(patch || {}),
      }),
    EnsureStartupPluginHostPrivilege: async () => false,
    EnsureUniversalPluginHostPrivilege: async () => true,
    QuarantinePluginPackage: (id: string, reason: string) =>
      apiJson(`/api/plugins/${encodeURIComponent(id)}/quarantine`, {
        method: 'POST',
        body: JSON.stringify({ reason }),
      }),
    RecordPluginActivation: (id: string, success: boolean, activationError = '') =>
      apiJson(`/api/plugins/${encodeURIComponent(id)}/activation`, {
        method: 'POST',
        body: JSON.stringify({ success, activationError }),
      }),
    InvokePluginCommand: async () => {},
    GetPluginDirectory: async () => {
      const res = await apiJson<{ directory: string }>('/api/plugins/directory');
      return res.directory;
    },
    ReadPluginFrontendModule: (id: string) =>
      apiJson(`/api/plugins/${encodeURIComponent(id)}/frontend-module`),
    ReadUniversalPluginFrontendEntry: async () => '',
    CallUniversalPluginBackend: async () => null,
    StopUniversalPluginBackend: async () => {},

    RemoteListConnections: async () => [],
    RemoteListProviders: async () => [],
    RemoteConnect: async () => { throw new Error('Remote mounts require desktop runtime'); },
    RemoteDisconnect: async () => {},
    RemoteConnectOAuth: async () => { throw new Error('Remote OAuth requires desktop runtime'); },
    RemoteOAuthBegin: async () => { throw new Error('Remote OAuth requires desktop runtime'); },
    RemoteOAuthStatus: async () => ({ status: 'cancelled' }),
    RemoteOAuthCancel: async () => {},

    ListTrashEntries: () => apiJson('/api/trash'),
    EmptyTrash: () => apiJson('/api/trash/empty', { method: 'POST' }),
    DeleteTrashEntry: (id: string) => apiJson(`/api/trash/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    RestoreTrashEntry: async (id: string) => {
      const res = await apiJson<{ destination: string }>('/api/trash/restore', {
        method: 'POST',
        body: JSON.stringify({ id }),
      });
      return res.destination;
    },
    RestoreTrashEntryTo: async (id: string, destinationDir: string) => {
      const res = await apiJson<{ destination: string }>('/api/trash/restore-to', {
        method: 'POST',
        body: JSON.stringify({ id, destinationDir }),
      });
      return res.destination;
    },
    CopyTrashEntryTo: async (id: string, destinationDir: string) => {
      const res = await apiJson<{ destination: string }>('/api/trash/copy-to', {
        method: 'POST',
        body: JSON.stringify({ id, destinationDir }),
      });
      return res.destination;
    },
    BatchTransferTrash: (request: any) =>
      apiJson('/api/trash/batch-transfer', {
        method: 'POST',
        body: JSON.stringify(request),
      }),

    AutorunList: async () => [],
    AutorunSetEnabled: async () => {},
    AutorunDelete: async () => {},
    SystemSurfaceList: (kind: string) =>
      apiJson('/api/system-surface', { method: 'POST', body: JSON.stringify({ kind }) }),
    NetworkListComputers: async () => [],
    NetworkListShares: async () => [],
    NetworkAdapterSetEnabled: async () => {},
    NetworkAdapterRenewDHCP: async () => {},
    NetworkAdapterDiagnose: async () => ({}),
    NetworkOpenSettings: async () => {},

    ArchiveBrowse: (fsPath: string, nestedChain: string[] = [], innerPath = '') =>
      apiJson('/api/archive/browse', {
        method: 'POST',
        body: JSON.stringify({ fsPath, nestedChain, innerPath }),
      }),
    ArchiveRead: (fsPath: string, nestedChain: string[] = [], entryPath = '', encoding = 'utf8') =>
      apiJson('/api/archive/read', {
        method: 'POST',
        body: JSON.stringify({ fsPath, nestedChain, entryPath, encoding }),
      }),
    ArchiveReadStream: async () => '',
    CancelArchiveReadStream: async () => {},
    ArchiveCopyEntries: (request: any) =>
      apiJson('/api/archive/copy-entries', { method: 'POST', body: JSON.stringify(request) }),
    ArchiveDeleteEntries: (request: any) =>
      apiJson('/api/archive/delete-entries', { method: 'POST', body: JSON.stringify(request) }),
    ArchiveImportEntries: (request: any) =>
      apiJson('/api/archive/import-entries', { method: 'POST', body: JSON.stringify(request) }),
    ArchiveMoveEntries: (request: any) =>
      apiJson('/api/archive/move-entries', { method: 'POST', body: JSON.stringify(request) }),
    BatchExtract: (request: any) =>
      apiJson('/api/archive/extract', { method: 'POST', body: JSON.stringify(request) }),
    BatchSelectiveExtract: (request: any) =>
      apiJson('/api/archive/selective-extract', { method: 'POST', body: JSON.stringify(request) }),
    CreateZipArchive: (paths: string[], destination: string) =>
      apiJson('/api/archive/create-zip', {
        method: 'POST',
        body: JSON.stringify({ paths, destination }),
      }),
    VFSOpenArchive: (fsPath: string) => App['ArchiveBrowse'](fsPath, [], ''),
    VFSMountArchive: async () => '',
    VFSUnmount: async () => {},
    CloseArchiveCache: async () => {},
    ExtractNestedArchive: async () => [],
    ProbeArchiveFile: (fsPath: string, nestedChain: string[] = [], entryPath = '') =>
      apiJson('/api/archive/probe', {
        method: 'POST',
        body: JSON.stringify({ fsPath, nestedChain, entryPath }),
      }),
    GetArchiveStreamURL: async (fsPath: string, nestedChain: string[] = [], entryPath = '') =>
      `/api/archive/stream?fsPath=${encodeURIComponent(fsPath)}&nestedChain=${encodeURIComponent(JSON.stringify(nestedChain || []))}&entryPath=${encodeURIComponent(entryPath)}`,
    GetArchiveImageThumbnailURL: async (fsPath: string, nestedChain: string[] = [], entryPath = '') =>
      `/api/archive/stream?fsPath=${encodeURIComponent(fsPath)}&nestedChain=${encodeURIComponent(JSON.stringify(nestedChain || []))}&entryPath=${encodeURIComponent(entryPath)}`,
    GetArchiveVideoPreviewURL: async (fsPath: string, nestedChain: string[] = [], entryPath = '') =>
      `/api/archive/stream?fsPath=${encodeURIComponent(fsPath)}&nestedChain=${encodeURIComponent(JSON.stringify(nestedChain || []))}&entryPath=${encodeURIComponent(entryPath)}`,
    GetArchiveVideoPlaybackPlan: async (fsPath: string, nestedChain: string[] = [], entryPath = '') => ({
      size: 0,
      format: entryPath.split('.').pop() || 'mp4',
      playbackMode: 'native',
      streamUrl: `/api/archive/stream?fsPath=${encodeURIComponent(fsPath)}&nestedChain=${encodeURIComponent(JSON.stringify(nestedChain || []))}&entryPath=${encodeURIComponent(entryPath)}`,
      seekable: true,
      useNativePlayer: true,
    }),

    CanConvertImage: async () => false,
    GetConvertedImageURL: async (p: string) => `/api/stream?path=${encodeURIComponent(p)}`,
    ConvertOfficeDocument: async () => ({ available: false }),
    IsOfficeDocumentAvailable: async () => false,
    GetConversionCapabilities: async () => ({}),

    GetPathAttributes: (p: string) =>
      apiJson('/api/fs/attributes', { method: 'POST', body: JSON.stringify({ path: p }) }),
    SetPathAttributes: (p: string, update: any) =>
      apiJson('/api/fs/attributes', { method: 'POST', body: JSON.stringify({ path: p, update }) }),
    SetModifiedTime: (p: string, unixMilliseconds: number) =>
      apiJson('/api/fs/mtime', { method: 'POST', body: JSON.stringify({ path: p, unixMilliseconds }) }),
    ResolveLink: (p: string) =>
      apiJson('/api/fs/resolve-link', { method: 'POST', body: JSON.stringify({ path: p }) }),
    CreateHardLink: (target: string, linkPath: string) =>
      apiJson('/api/fs/link', { method: 'POST', body: JSON.stringify({ kind: 'hard', target, linkPath }) }),
    CreateSymbolicLink: (target: string, linkPath: string) =>
      apiJson('/api/fs/link', { method: 'POST', body: JSON.stringify({ kind: 'symbolic', target, linkPath }) }),
    CreateShortcut: (target: string, linkPath: string) =>
      apiJson('/api/fs/link', { method: 'POST', body: JSON.stringify({ kind: 'symbolic', target, linkPath }) }),
    SplitFile: async (p: string, partSize: number) => {
      const res = await apiJson<{ parts: string[] }>('/api/fs/split', {
        method: 'POST',
        body: JSON.stringify({ path: p, partSize }),
      });
      return res.parts;
    },
    JoinFiles: async (parts: string[], destination: string) => {
      const res = await apiJson<{ destination: string }>('/api/fs/join', {
        method: 'POST',
        body: JSON.stringify({ parts, destination }),
      });
      return res.destination;
    },

    ChooseDirectory: async (defaultPath = '') => defaultPath || '/',
    OpenFile: async (p: string) => {
      const name = p.split('/').pop() || 'download';
      triggerBrowserDownload(`/api/download?path=${encodeURIComponent(p)}`, name);
    },
    OpenFileWithApplication: async (p: string) => {
      const name = p.split('/').pop() || 'download';
      triggerBrowserDownload(`/api/download?path=${encodeURIComponent(p)}`, name);
    },
    OpenFileWithChooser: async (p: string) => {
      const name = p.split('/').pop() || 'download';
      triggerBrowserDownload(`/api/download?path=${encodeURIComponent(p)}`, name);
    },
    OpenTerminalAt: async (p: string) => {
      try {
        await navigator.clipboard.writeText(p);
      } catch {}
    },
    RevealInFileManager: async (p: string) => {
      const name = p.split('/').pop() || 'download';
      triggerBrowserDownload(`/api/download?path=${encodeURIComponent(p)}`, name);
    },
    SetPaneDirectory: async () => {},
    CreatePaneWatch: async () => {},
    ClosePaneWatch: async () => {},
    RegisterDropListener: async () => {},
    SetupFileDropHandler: async () => {},
    HandleFileDrop: async () => {},
    JoinMountNamespace: async () => {},

    IsProcessElevated: async () => false,
    StartElevatedSession: async () => ({ active: false, pid: 0, startedAt: 0, port: 0, opCount: 0, error: '' }),
    StopElevatedSession: async () => {},
    StopElevationSession: async () => {},
    GetElevatedSessionStatus: async () => ({ active: false, pid: 0, startedAt: 0, port: 0, opCount: 0, error: '' }),
    ConfirmElevationRequest: async () => {},
    WaitForElevationApproval: async () => false,
    WriteRegistryValue: async () => ({ success: false, message: 'Registry requires Windows desktop' }),

    BrowserPaneSync: async () => {},
    BrowserPaneDispose: async () => {},
    BrowserPanePostMessage: async () => {},
    BrowserPaneCaptureScreenshot: async () => '',
    BrowserPaneGetDocumentBackground: async () => '#ffffff',
  };

  w.go = { app: { App } };
}

installRuntime();
installGoAppBridge();
