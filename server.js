const express = require('express');
const fs = require('fs');
const fsp = require('fs/promises');
const path = require('path');
const os = require('os');
const crypto = require('crypto');
const { execSync } = require('child_process');
const AdmZip = require('adm-zip');

const PORT = Number(process.env.PORT || 3000);
const HOST = process.env.HOST || '0.0.0.0';
const WORKSPACE_ROOT = process.cwd();
const FRONTEND_DIST = path.join(WORKSPACE_ROOT, 'manager', 'frontend', 'dist');
const TRASH_DIR = path.join(os.tmpdir(), 'reassembly-web-trash');

const ARCHIVE_EXTS = new Set(['.zip', '.jar', '.war', '.ear', '.apk', '.tar', '.gz', '.tgz']);

// Ensure frontend dist exists if started directly without prior build
if (!fs.existsSync(path.join(FRONTEND_DIST, 'index.html'))) {
  console.log('[server] Frontend dist missing, building Angular app...');
  execSync('npm --prefix manager/frontend run build', { stdio: 'inherit', cwd: WORKSPACE_ROOT });
}

fs.mkdirSync(TRASH_DIR, { recursive: true });

const app = express();
app.use(express.json({ limit: '25mb' }));

// In-memory state for browser preview session
let workspacesStore = {
  activeWorkspaceId: 'default',
  workspaces: [
    {
      id: 'default',
      name: 'Workspace',
      icon: 'folder',
      leftPane: {
        id: 'left',
        activeTabId: 'left-1',
        tabs: [
          {
            id: 'left-1',
            title: path.basename(WORKSPACE_ROOT) || 'Workspace',
            path: WORKSPACE_ROOT,
            kind: 'dir',
            isActive: true,
            isPinned: false,
            createdAt: Date.now(),
          },
        ],
      },
      rightPane: {
        id: 'right',
        activeTabId: 'right-1',
        tabs: [
          {
            id: 'right-1',
            title: 'docs',
            path: fs.existsSync(path.join(WORKSPACE_ROOT, 'docs')) ? path.join(WORKSPACE_ROOT, 'docs') : WORKSPACE_ROOT,
            kind: 'dir',
            isActive: true,
            isPinned: false,
            createdAt: Date.now(),
          },
        ],
      },
      showHidden: false,
      activePaneId: 'left',
      splitRatio: 0.5,
      createdAt: Date.now(),
      lastAccessedAt: Date.now(),
    },
  ],
};

let preferencesStore = {
  theme: 'dark',
  accentColor: '#0078d4',
  uiZoom: 100,
  compactMode: false,
  animationsEnabled: true,
  defaultShowHidden: false,
  defaultSortBy: 'name',
  defaultSortOrder: 'asc',
  enableKeyboardShortcuts: true,
  doubleClickAction: 'open',
  confirmDelete: true,
  confirmOnClose: false,
  reopenLastWorkspace: true,
};

const pluginStates = new Map();
const trashEntries = new Map();

function normalizeInputPath(rawPath) {
  let p = String(rawPath || '').trim();
  if (!p) return WORKSPACE_ROOT;
  if (p.startsWith('filesystem://')) {
    p = p.slice('filesystem://'.length);
    if (!p.startsWith('/')) p = '/' + p;
  }
  if (p.startsWith('~')) {
    p = path.join(os.homedir(), p.slice(1));
  }
  return path.resolve(p);
}

function guessMime(filePath, isDir) {
  if (isDir) return 'inode/directory';
  const ext = path.extname(filePath).toLowerCase();
  const map = {
    '.ts': 'text/typescript',
    '.tsx': 'text/typescript',
    '.js': 'text/javascript',
    '.mjs': 'text/javascript',
    '.json': 'application/json',
    '.md': 'text/markdown',
    '.html': 'text/html',
    '.css': 'text/css',
    '.scss': 'text/x-scss',
    '.go': 'text/x-go',
    '.txt': 'text/plain',
    '.yml': 'text/yaml',
    '.yaml': 'text/yaml',
    '.xml': 'application/xml',
    '.svg': 'image/svg+xml',
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.gif': 'image/gif',
    '.webp': 'image/webp',
    '.pdf': 'application/pdf',
    '.mp3': 'audio/mpeg',
    '.wav': 'audio/wav',
    '.mp4': 'video/mp4',
    '.webm': 'video/webm',
    '.zip': 'application/zip',
    '.tar': 'application/x-tar',
    '.gz': 'application/gzip',
  };
  return map[ext] || 'application/octet-stream';
}

function formatPermissions(mode) {
  const perms = (mode & 0o777).toString(8).padStart(3, '0');
  return perms;
}

async function statToFileInfo(fullPath, nameOverride) {
  const lstat = await fsp.lstat(fullPath);
  const isSymlink = lstat.isSymbolicLink();
  let stat = lstat;
  if (isSymlink) {
    try {
      stat = await fsp.stat(fullPath);
    } catch {
      stat = lstat;
    }
  }
  const name = nameOverride || path.basename(fullPath) || '/';
  const isDir = stat.isDirectory();
  const ext = path.extname(name).toLowerCase();
  let type = 'FILE';
  if (isDir) type = 'DIRECTORY';
  else if (isSymlink) type = 'SYMLINK';
  else if (ARCHIVE_EXTS.has(ext)) type = 'ARCHIVE';

  return {
    name,
    path: fullPath,
    type,
    mimeType: guessMime(fullPath, isDir),
    size: isDir ? 0 : stat.size,
    created: Math.floor((stat.birthtimeMs || stat.ctimeMs) / 1000),
    modified: Math.floor(stat.mtimeMs / 1000),
    accessed: Math.floor(stat.atimeMs / 1000),
    permissions: formatPermissions(stat.mode),
    isHidden: name.startsWith('.'),
    isSymlink,
  };
}

app.get('/api/os-info', (_req, res) => {
  res.json({
    os: process.platform === 'win32' ? 'windows' : process.platform === 'darwin' ? 'macos' : 'linux',
    version: os.release(),
    arch: process.arch,
    hostname: os.hostname(),
    systemDrive: null,
  });
});

app.get('/api/drives', (_req, res) => {
  res.json({
    schemes: [
      {
        name: 'Workspace',
        path: WORKSPACE_ROOT,
        schemeType: 'fixed',
        totalSpace: 0,
        freeSpace: 0,
        fileSystem: 'ext4',
      },
      {
        name: 'Root (/)',
        path: '/',
        schemeType: 'fixed',
        totalSpace: 0,
        freeSpace: 0,
        fileSystem: 'ext4',
      },
      {
        name: 'Trash Bin',
        path: 'bin://',
        schemeType: 'bin',
        totalSpace: 0,
        freeSpace: 0,
        fileSystem: null,
      },
    ],
  });
});

app.get('/api/special-folders', (_req, res) => {
  res.json([
    { name: 'Workspace', path: WORKSPACE_ROOT, icon: 'folder' },
    { name: 'Home', path: os.homedir(), icon: 'home' },
    { name: 'Temp', path: os.tmpdir(), icon: 'folder' },
    { name: 'Root', path: '/', icon: 'drive' },
  ]);
});

app.post('/api/fs/capacity', async (req, res) => {
  const target = normalizeInputPath(req.body?.path);
  try {
    const sf = await fsp.statfs(target);
    const totalSpace = sf.blocks * sf.bsize;
    const freeSpace = sf.bavail * sf.bsize;
    res.json({
      path: target,
      totalSpace,
      freeSpace,
      usedSpace: Math.max(0, totalSpace - freeSpace),
    });
  } catch {
    res.json({ path: target, totalSpace: 0, freeSpace: 0, usedSpace: 0 });
  }
});

app.post('/api/fs/list', async (req, res) => {
  try {
    const raw = String(req.body?.path || '').trim();
    if (raw.startsWith('bin://')) {
      const entries = Array.from(trashEntries.values()).map((t) => ({
        name: t.name,
        path: `bin://${t.id}`,
        type: t.isDir ? 'DIRECTORY' : 'FILE',
        mimeType: t.isDir ? 'inode/directory' : guessMime(t.name, false),
        size: t.size || 0,
        created: t.deletedAt,
        modified: t.deletedAt,
        accessed: t.deletedAt,
        permissions: '644',
        isHidden: false,
      }));
      return res.json({ path: 'bin://', entries, totalSize: entries.reduce((a, b) => a + b.size, 0) });
    }

    const dirPath = normalizeInputPath(raw);
    const showHidden = Boolean(req.body?.showHidden);
    const dirents = await fsp.readdir(dirPath, { withFileTypes: true });
    const entries = [];
    let totalSize = 0;

    for (const d of dirents) {
      if (!showHidden && d.name.startsWith('.')) continue;
      const fullPath = path.join(dirPath, d.name);
      try {
        const info = await statToFileInfo(fullPath, d.name);
        entries.push(info);
        totalSize += info.size;
      } catch {
        // skip unreadable entries
      }
    }

    res.json({ path: dirPath, entries, totalSize });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/info', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    const info = await statToFileInfo(target);
    res.json(info);
  } catch (err) {
    res.status(404).json({ error: err.message });
  }
});

app.post('/api/fs/exists', async (req, res) => {
  const target = normalizeInputPath(req.body?.path);
  try {
    await fsp.access(target);
    res.json({ exists: true });
  } catch {
    res.json({ exists: false });
  }
});

app.post('/api/fs/probe', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    const stat = await fsp.stat(target);
    const mimeType = guessMime(target, stat.isDirectory());
    const ext = path.extname(target).toLowerCase();
    const isArchive = ARCHIVE_EXTS.has(ext);
    let isText = mimeType.startsWith('text/') || mimeType === 'application/json' || mimeType === 'application/xml';
    if (!isText && !stat.isDirectory() && !isArchive && stat.size < 1024 * 1024) {
      const fd = await fsp.open(target, 'r');
      const buf = Buffer.alloc(Math.min(512, stat.size));
      await fd.read(buf, 0, buf.length, 0);
      await fd.close();
      isText = !buf.includes(0);
    }
    res.json({ mimeType, encoding: isText ? 'utf8' : 'binary', isText, isArchive });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/read', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    const encoding = req.body?.encoding === 'base64' ? 'base64' : 'utf8';
    const buf = await fsp.readFile(target);
    res.json({
      path: target,
      content: encoding === 'base64' ? buf.toString('base64') : buf.toString('utf8'),
      encoding,
      size: buf.length,
    });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/read-range', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    const offset = Math.max(0, Number(req.body?.offset || 0));
    const length = Math.max(0, Math.min(4 * 1024 * 1024, Number(req.body?.length || 65536)));
    const stat = await fsp.stat(target);
    const fd = await fsp.open(target, 'r');
    const toRead = Math.max(0, Math.min(length, stat.size - offset));
    const buf = Buffer.alloc(toRead);
    if (toRead > 0) {
      await fd.read(buf, 0, toRead, offset);
    }
    await fd.close();
    res.json({
      path: target,
      content: buf.toString('base64'),
      encoding: 'base64',
      size: stat.size,
      offset,
      length: toRead,
    });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/write', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    const encoding = req.body?.encoding === 'base64' ? 'base64' : 'utf8';
    const content = String(req.body?.content ?? '');
    await fsp.mkdir(path.dirname(target), { recursive: true });
    await fsp.writeFile(target, Buffer.from(content, encoding));
    res.json({ success: true, affectedPaths: [target] });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/mkdir', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    await fsp.mkdir(target, { recursive: req.body?.recursive !== false });
    res.json({ success: true, affectedPaths: [target] });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

async function moveToTrash(target) {
  const id = `trash-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const stat = await fsp.lstat(target);
  const trashPath = path.join(TRASH_DIR, id);
  await fsp.rename(target, trashPath);
  trashEntries.set(id, {
    id,
    name: path.basename(target),
    originalPath: target,
    trashPath,
    size: stat.isDirectory() ? 0 : stat.size,
    isDir: stat.isDirectory(),
    deletedAt: Math.floor(Date.now() / 1000),
  });
}

app.post('/api/fs/delete', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    if (req.body?.useTrash) {
      await moveToTrash(target);
    } else {
      await fsp.rm(target, { recursive: req.body?.recursive !== false, force: true });
    }
    res.json({ success: true, affectedPaths: [target] });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/batch-delete', async (req, res) => {
  const paths = Array.isArray(req.body?.paths) ? req.body.paths : [];
  const errors = [];
  const affectedPaths = [];
  let succeeded = 0;

  for (const p of paths) {
    try {
      const target = normalizeInputPath(p);
      if (req.body?.useTrash) {
        await moveToTrash(target);
      } else {
        await fsp.rm(target, { recursive: req.body?.recursive !== false, force: true });
      }
      succeeded++;
      affectedPaths.push(target);
    } catch (err) {
      errors.push({ path: p, error: err.message });
    }
  }

  res.json({
    operationId: `batch-${Date.now()}`,
    total: paths.length,
    succeeded,
    skipped: 0,
    failed: errors.length,
    errors,
    affectedPaths,
  });
});

app.post('/api/fs/move', async (req, res) => {
  try {
    const src = normalizeInputPath(req.body?.source);
    const dest = normalizeInputPath(req.body?.destination);
    await fsp.mkdir(path.dirname(dest), { recursive: true });
    await fsp.rename(src, dest);
    res.json({ success: true, affectedPaths: [dest] });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/copy', async (req, res) => {
  try {
    const src = normalizeInputPath(req.body?.source);
    const dest = normalizeInputPath(req.body?.destination);
    await fsp.mkdir(path.dirname(dest), { recursive: true });
    await fsp.cp(src, dest, { recursive: req.body?.recursive !== false });
    res.json({ success: true, affectedPaths: [dest] });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/search', async (req, res) => {
  try {
    const baseDir = normalizeInputPath(req.body?.path);
    const pattern = String(req.body?.pattern || '').trim().toLowerCase();
    const recursive = req.body?.recursive !== false;
    const matches = [];
    const maxMatches = 200;

    async function walk(dir) {
      if (matches.length >= maxMatches) return;
      let items;
      try {
        items = await fsp.readdir(dir, { withFileTypes: true });
      } catch {
        return;
      }
      for (const item of items) {
        if (matches.length >= maxMatches) break;
        if (item.name === 'node_modules' || item.name === '.git') continue;
        const full = path.join(dir, item.name);
        if (!pattern || item.name.toLowerCase().includes(pattern)) {
          try {
            matches.push(await statToFileInfo(full, item.name));
          } catch {}
        }
        if (recursive && item.isDirectory()) {
          await walk(full);
        }
      }
    }

    await walk(baseDir);
    res.json({ path: baseDir, matches, totalMatches: matches.length });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/hash', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    const buf = await fsp.readFile(target);
    const sha256 = crypto.createHash('sha256').update(buf).digest('hex');
    const md5 = crypto.createHash('md5').update(buf).digest('hex');
    const sha1 = crypto.createHash('sha1').update(buf).digest('hex');
    res.json({ hash: sha256, sha256, md5, sha1 });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/disk-usage', async (req, res) => {
  try {
    const baseDir = normalizeInputPath(req.body?.path);
    const items = await fsp.readdir(baseDir, { withFileTypes: true });
    const entries = [];
    let totalSize = 0;

    async function dirSize(p, depth = 0) {
      if (depth > 4) return 0;
      let sum = 0;
      try {
        const children = await fsp.readdir(p, { withFileTypes: true });
        for (const c of children) {
          if (c.name === 'node_modules' || c.name === '.git') continue;
          const cp = path.join(p, c.name);
          if (c.isDirectory()) sum += await dirSize(cp, depth + 1);
          else sum += (await fsp.stat(cp)).size;
        }
      } catch {}
      return sum;
    }

    for (const item of items) {
      const full = path.join(baseDir, item.name);
      let size = 0;
      try {
        size = item.isDirectory() ? await dirSize(full) : (await fsp.stat(full)).size;
      } catch {}
      totalSize += size;
      entries.push({
        name: item.name,
        path: full,
        isDir: item.isDirectory(),
        size,
        fileCount: item.isDirectory() ? 1 : 0,
      });
    }

    entries.sort((a, b) => b.size - a.size);
    res.json({ path: baseDir, totalSize, entries });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/attributes', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    const stat = await fsp.stat(target);
    res.json({
      readOnly: (stat.mode & 0o200) === 0,
      hidden: path.basename(target).startsWith('.'),
      system: false,
      archive: false,
      permissions: formatPermissions(stat.mode),
    });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/mtime', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    const ms = Number(req.body?.unixMilliseconds || Date.now());
    const dt = new Date(ms);
    await fsp.utimes(target, dt, dt);
    res.json({ success: true });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/resolve-link', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.path);
    const real = await fsp.realpath(target);
    res.json({ target: real, valid: true });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/link', async (req, res) => {
  try {
    const target = normalizeInputPath(req.body?.target);
    const linkPath = normalizeInputPath(req.body?.linkPath);
    if (req.body?.kind === 'hard') await fsp.link(target, linkPath);
    else await fsp.symlink(target, linkPath);
    res.json({ success: true, affectedPaths: [linkPath] });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/fs/split', async (_req, res) => {
  res.status(501).json({ error: 'Split not enabled in browser preview' });
});

app.post('/api/fs/join', async (_req, res) => {
  res.status(501).json({ error: 'Join not enabled in browser preview' });
});

app.get('/api/stream', async (req, res) => {
  try {
    const target = normalizeInputPath(req.query.path);
    res.sendFile(target);
  } catch (err) {
    res.status(404).send(err.message);
  }
});

app.get('/api/download', async (req, res) => {
  try {
    const target = normalizeInputPath(req.query.path);
    res.download(target);
  } catch (err) {
    res.status(404).send(err.message);
  }
});

app.get('/api/workspaces', (_req, res) => {
  res.json(workspacesStore);
});

app.post('/api/workspaces', (req, res) => {
  const ws = req.body;
  if (!ws || !ws.id) return res.status(400).json({ error: 'Invalid workspace' });
  const idx = workspacesStore.workspaces.findIndex((x) => x.id === ws.id);
  if (idx >= 0) workspacesStore.workspaces[idx] = ws;
  else workspacesStore.workspaces.push(ws);
  workspacesStore.activeWorkspaceId = ws.id;
  res.json({ success: true });
});

app.delete('/api/workspaces/:id', (req, res) => {
  const id = req.params.id;
  workspacesStore.workspaces = workspacesStore.workspaces.filter((w) => w.id !== id);
  if (workspacesStore.activeWorkspaceId === id) {
    workspacesStore.activeWorkspaceId = workspacesStore.workspaces[0]?.id || '';
  }
  res.json({ success: true });
});

app.get('/api/preferences', (_req, res) => {
  res.json(preferencesStore);
});

app.post('/api/preferences', (req, res) => {
  preferencesStore = { ...preferencesStore, ...(req.body || {}) };
  res.json({ success: true });
});

app.get('/api/plugins', (_req, res) => res.json([]));
app.get('/api/plugins/states', (_req, res) => res.json(Object.fromEntries(pluginStates)));
app.get('/api/plugins/:id/state', (req, res) => res.json(pluginStates.get(req.params.id) || { enabled: false, trusted: false }));
app.patch('/api/plugins/:id/state', (req, res) => {
  const next = { ...(pluginStates.get(req.params.id) || {}), ...(req.body || {}) };
  pluginStates.set(req.params.id, next);
  res.json(next);
});
app.post('/api/plugins/:id/quarantine', (req, res) => res.json({ quarantined: true, reason: req.body?.reason || '' }));
app.post('/api/plugins/:id/activation', (_req, res) => res.json({ recorded: true }));
app.get('/api/plugins/directory', (_req, res) => res.json({ directory: path.join(WORKSPACE_ROOT, 'plugins') }));
app.get('/api/plugins/:id/frontend-module', (_req, res) => res.status(404).json({ error: 'No plugin module' }));

app.get('/api/trash', (_req, res) => {
  res.json(Array.from(trashEntries.values()));
});

app.post('/api/trash/empty', async (_req, res) => {
  for (const t of trashEntries.values()) {
    await fsp.rm(t.trashPath, { recursive: true, force: true }).catch(() => {});
  }
  trashEntries.clear();
  res.json({ success: true });
});

app.delete('/api/trash/:id', async (req, res) => {
  const entry = trashEntries.get(req.params.id);
  if (entry) {
    await fsp.rm(entry.trashPath, { recursive: true, force: true }).catch(() => {});
    trashEntries.delete(req.params.id);
  }
  res.json({ success: true });
});

app.post('/api/trash/restore', async (req, res) => {
  const entry = trashEntries.get(req.body?.id);
  if (!entry) return res.status(404).json({ error: 'Trash entry not found' });
  await fsp.mkdir(path.dirname(entry.originalPath), { recursive: true });
  await fsp.rename(entry.trashPath, entry.originalPath);
  trashEntries.delete(entry.id);
  res.json({ destination: entry.originalPath });
});

app.post('/api/trash/restore-to', async (req, res) => {
  const entry = trashEntries.get(req.body?.id);
  if (!entry) return res.status(404).json({ error: 'Trash entry not found' });
  const dest = path.join(normalizeInputPath(req.body?.destinationDir), entry.name);
  await fsp.mkdir(path.dirname(dest), { recursive: true });
  await fsp.rename(entry.trashPath, dest);
  trashEntries.delete(entry.id);
  res.json({ destination: dest });
});

app.post('/api/trash/copy-to', async (req, res) => {
  const entry = trashEntries.get(req.body?.id);
  if (!entry) return res.status(404).json({ error: 'Trash entry not found' });
  const dest = path.join(normalizeInputPath(req.body?.destinationDir), entry.name);
  await fsp.mkdir(path.dirname(dest), { recursive: true });
  await fsp.cp(entry.trashPath, dest, { recursive: true });
  res.json({ destination: dest });
});

app.post('/api/trash/batch-transfer', async (_req, res) => {
  res.json({ success: true, succeeded: 0, failed: 0 });
});

app.post('/api/system-surface', (req, res) => {
  const kind = String(req.body?.kind || 'environment');
  if (kind === 'environment') {
    const entries = Object.entries(process.env).map(([name, value]) => ({
      name,
      path: `environment://${name}`,
      type: 'FILE',
      size: String(value || '').length,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '644',
      isHidden: false,
      registryPreview: String(value || ''),
    }));
    return res.json({ path: 'environment://', entries, totalSize: entries.length });
  }
  res.json({ path: `${kind}://`, entries: [], totalSize: 0 });
});

// Archive browsing via AdmZip
app.post('/api/archive/browse', (req, res) => {
  try {
    const fsPath = normalizeInputPath(req.body?.fsPath);
    const innerPath = String(req.body?.innerPath || '').replace(/^\/+|\/+$/g, '');
    const zip = new AdmZip(fsPath);
    const seen = new Map();
    const prefix = innerPath ? innerPath + '/' : '';

    for (const entry of zip.getEntries()) {
      const entryName = entry.entryName.replace(/^\/+|\/+$/g, '');
      if (!entryName || (prefix && !entryName.startsWith(prefix))) continue;
      const rest = entryName.slice(prefix.length);
      if (!rest) continue;
      const slashIdx = rest.indexOf('/');
      if (slashIdx >= 0) {
        const dirName = rest.slice(0, slashIdx);
        const dirPath = prefix + dirName;
        if (!seen.has(dirPath)) {
          seen.set(dirPath, {
            name: dirName,
            path: dirPath,
            type: 'DIRECTORY',
            size: 0,
            modified: Math.floor(entry.header.time.getTime() / 1000),
            isNested: false,
          });
        }
      } else {
        seen.set(entryName, {
          name: rest,
          path: entryName,
          type: entry.isDirectory ? 'DIRECTORY' : 'FILE',
          size: entry.header.size,
          modified: Math.floor(entry.header.time.getTime() / 1000),
          isNested: false,
        });
      }
    }

    res.json({
      archivePath: fsPath,
      innerPath,
      nestedChain: req.body?.nestedChain || [],
      entries: Array.from(seen.values()),
    });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/archive/read', (req, res) => {
  try {
    const fsPath = normalizeInputPath(req.body?.fsPath);
    const entryPath = String(req.body?.entryPath || '').replace(/^\/+/, '');
    const encoding = req.body?.encoding === 'base64' ? 'base64' : 'utf8';
    const zip = new AdmZip(fsPath);
    const entry = zip.getEntry(entryPath);
    if (!entry) return res.status(404).json({ error: 'Archive entry not found' });
    const buf = entry.getData();
    res.json({
      path: entryPath,
      content: encoding === 'base64' ? buf.toString('base64') : buf.toString('utf8'),
      encoding,
      size: buf.length,
    });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/archive/probe', (req, res) => {
  const entryPath = String(req.body?.entryPath || '');
  const mimeType = guessMime(entryPath, false);
  const isText = mimeType.startsWith('text/') || mimeType === 'application/json';
  res.json({ mimeType, encoding: isText ? 'utf8' : 'binary', isText, isArchive: false });
});

app.get('/api/archive/stream', (req, res) => {
  try {
    const fsPath = normalizeInputPath(req.query.fsPath);
    const entryPath = String(req.query.entryPath || '').replace(/^\/+/, '');
    const zip = new AdmZip(fsPath);
    const entry = zip.getEntry(entryPath);
    if (!entry) return res.status(404).send('Not found');
    res.type(guessMime(entryPath, false));
    res.send(entry.getData());
  } catch (err) {
    res.status(400).send(err.message);
  }
});

app.post('/api/archive/create-zip', async (req, res) => {
  try {
    const paths = Array.isArray(req.body?.paths) ? req.body.paths : [];
    const destination = normalizeInputPath(req.body?.destination);
    const zip = new AdmZip();
    for (const p of paths) {
      const target = normalizeInputPath(p);
      const stat = await fsp.stat(target);
      if (stat.isDirectory()) zip.addLocalFolder(target, path.basename(target));
      else zip.addLocalFile(target);
    }
    zip.writeZip(destination);
    res.json({ success: true, affectedPaths: [destination] });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/archive/extract', (req, res) => {
  try {
    const archivePath = normalizeInputPath(req.body?.archivePath || req.body?.fsPath);
    const destination = normalizeInputPath(req.body?.destination);
    const zip = new AdmZip(archivePath);
    zip.extractAllTo(destination, true);
    res.json({ success: true, affectedPaths: [destination] });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

app.post('/api/archive/copy-entries', (_req, res) => res.status(501).json({ error: 'Not implemented in web preview' }));
app.post('/api/archive/delete-entries', (_req, res) => res.status(501).json({ error: 'Not implemented in web preview' }));
app.post('/api/archive/import-entries', (_req, res) => res.status(501).json({ error: 'Not implemented in web preview' }));
app.post('/api/archive/move-entries', (_req, res) => res.status(501).json({ error: 'Not implemented in web preview' }));
app.post('/api/archive/selective-extract', (_req, res) => res.status(501).json({ error: 'Not implemented in web preview' }));

// Static frontend + SPA fallback
app.use(express.static(FRONTEND_DIST));
app.get('*', (_req, res) => {
  res.sendFile(path.join(FRONTEND_DIST, 'index.html'));
});

app.listen(PORT, HOST, () => {
  console.log(`Reassembly web preview server listening on http://${HOST}:${PORT}`);
});
