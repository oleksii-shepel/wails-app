import express from 'express';
import fs from 'node:fs';
import fsp from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import crypto from 'node:crypto';
import { execSync, spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const WORKSPACE_ROOT = __dirname;
const PUBLIC_DIR = path.join(__dirname, 'public');
const DIST_DIR = path.join(__dirname, 'manager', 'frontend', 'dist');
const STATE_FILE = path.join(os.tmpdir(), 'reassembly-web-state.json');
const TRASH_ROOT = path.join(os.tmpdir(), 'reassembly-web-trash');
const TRASH_FILES_DIR = path.join(TRASH_ROOT, 'files');
const TRASH_MANIFEST = path.join(TRASH_ROOT, 'index.json');
const PLUGINS_ROOT = path.join(os.tmpdir(), 'reassembly-web-plugins');
const PLUGIN_STATE_FILE = path.join(os.tmpdir(), 'reassembly-web-plugin-states.json');
const PORT = Number(process.env.PORT || 3000);
const HOST = '127.0.0.1';

if (!fs.existsSync(path.join(DIST_DIR, 'index.html'))) {
  console.log('[server] Frontend dist not found, building manager/frontend...');
  execSync('npm --prefix manager/frontend run build', {
    cwd: WORKSPACE_ROOT,
    stdio: 'inherit',
  });
}

const app = express();
const previewHosts = new Set(
  ['127.0.0.1', 'localhost'].map(host => PORT === 80 ? host : `${host}:${PORT}`),
);
app.use((req, res, next) => {
  const host = req.headers.host?.toLowerCase();
  if (!previewHosts.has(host)) {
    return res.status(403).json({ error: 'Open the preview using its local address.' });
  }
  const origin = req.headers.origin;
  if (origin && origin !== `http://${host}`) {
    return res.status(403).json({ error: 'Cross-origin preview requests are disabled.' });
  }
  next();
});
app.use(express.json({ limit: '100mb' }));

const defaultWorkspace = {
  id: 'default-workspace',
  name: 'Workspace',
  icon: '📁',
  color: 'var(--vsc-accent-blue)',
  leftPane: {
    id: 'left',
    activeTabId: 'left-tab-1',
    tabs: [
      {
        id: 'left-tab-1',
        title: path.basename(WORKSPACE_ROOT) || 'Workspace',
        path: WORKSPACE_ROOT,
        kind: 'dir',
        surfaceId: 'dir',
        isActive: true,
        isPinned: false,
        isClosable: true,
        createdAt: Date.now(),
      },
    ],
  },
  rightPane: {
    id: 'right',
    activeTabId: 'right-tab-1',
    tabs: [
      {
        id: 'right-tab-1',
        title: 'docs',
        path: fs.existsSync(path.join(WORKSPACE_ROOT, 'docs'))
          ? path.join(WORKSPACE_ROOT, 'docs')
          : WORKSPACE_ROOT,
        kind: 'dir',
        surfaceId: 'dir',
        isActive: true,
        isPinned: false,
        isClosable: true,
        createdAt: Date.now(),
      },
    ],
  },
  showHidden: false,
  createdAt: Date.now(),
  lastAccessedAt: Date.now(),
};

let workspaces = [defaultWorkspace];
let activeWorkspaceId = defaultWorkspace.id;
let preferences = {};

function loadPersistedState() {
  try {
    if (!fs.existsSync(STATE_FILE)) return;
    const raw = fs.readFileSync(STATE_FILE, 'utf8');
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed?.workspaces) && parsed.workspaces.length > 0) {
      workspaces = parsed.workspaces;
    }
    if (typeof parsed?.activeWorkspaceId === 'string' && parsed.activeWorkspaceId) {
      activeWorkspaceId = parsed.activeWorkspaceId;
    }
    if (parsed?.preferences && typeof parsed.preferences === 'object') {
      preferences = parsed.preferences;
    }
  } catch (err) {
    console.warn('[server] Failed to load persisted state:', err?.message || err);
  }
}

async function savePersistedState() {
  try {
    await fsp.writeFile(
      STATE_FILE,
      JSON.stringify({ workspaces, activeWorkspaceId, preferences }, null, 2),
      'utf8',
    );
  } catch (err) {
    console.warn('[server] Failed to save persisted state:', err?.message || err);
  }
}

async function ensureTrashDir() {
  await fsp.mkdir(TRASH_FILES_DIR, { recursive: true });
}

async function readTrashManifest() {
  try {
    await ensureTrashDir();
    if (!fs.existsSync(TRASH_MANIFEST)) return [];
    const raw = await fsp.readFile(TRASH_MANIFEST, 'utf8');
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

async function writeTrashManifest(entries) {
  await ensureTrashDir();
  await fsp.writeFile(TRASH_MANIFEST, JSON.stringify(entries, null, 2), 'utf8');
}

async function uniqueDestinationPath(targetPath) {
  if (!fs.existsSync(targetPath)) return targetPath;
  const dir = path.dirname(targetPath);
  const ext = path.extname(targetPath);
  const stem = path.basename(targetPath, ext);
  for (let i = 1; i < 10000; i++) {
    const candidate = path.join(dir, `${stem} (${i})${ext}`);
    if (!fs.existsSync(candidate)) return candidate;
  }
  return path.join(dir, `${stem}-${Date.now()}${ext}`);
}

async function movePathSafely(src, dest) {
  await fsp.mkdir(path.dirname(dest), { recursive: true });
  try {
    await fsp.rename(src, dest);
  } catch (err) {
    if (err && err.code === 'EXDEV') {
      await fsp.cp(src, dest, { recursive: true });
      await fsp.rm(src, { recursive: true, force: true });
      return;
    }
    throw err;
  }
}

async function moveToTrash(targetPath) {
  const lstat = await fsp.lstat(targetPath);
  const isDir = lstat.isDirectory();
  const size = isDir ? 0 : lstat.size;
  const name = path.basename(targetPath);
  const id = `trash-${Date.now()}-${crypto.randomBytes(4).toString('hex')}`;
  await ensureTrashDir();
  const trashedPath = path.join(TRASH_FILES_DIR, `${id}_${name}`);
  await movePathSafely(targetPath, trashedPath);

  const manifest = await readTrashManifest();
  const entry = {
    id,
    name,
    originalPath: targetPath,
    trashedPath,
    deletedAt: new Date().toISOString(),
    size,
    isDir,
    canRestore: true,
  };
  manifest.unshift(entry);
  await writeTrashManifest(manifest);
  return entry;
}

loadPersistedState();

function resolveRealPath(inputPath) {
  let raw = String(inputPath || '').trim();
  if (!raw) return WORKSPACE_ROOT;
  if (raw.startsWith('filesystem://')) {
    raw = raw.slice('filesystem://'.length);
    if (/^[a-z](?:\/|$)/i.test(raw)) {
      raw = '/' + raw.slice(1).replace(/^\/+/, '');
    } else if (!raw.startsWith('/')) {
      raw = '/' + raw;
    }
  }
  return path.resolve(raw || '/');
}

const MIME_MAP = {
  '.txt': 'text/plain',
  '.md': 'text/markdown',
  '.ts': 'text/typescript',
  '.tsx': 'text/typescript',
  '.js': 'application/javascript',
  '.mjs': 'application/javascript',
  '.json': 'application/json',
  '.html': 'text/html',
  '.css': 'text/css',
  '.scss': 'text/x-scss',
  '.go': 'text/x-go',
  '.py': 'text/x-python',
  '.yml': 'text/yaml',
  '.yaml': 'text/yaml',
  '.xml': 'application/xml',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.webp': 'image/webp',
  '.ico': 'image/x-icon',
  '.pdf': 'application/pdf',
  '.mp3': 'audio/mpeg',
  '.wav': 'audio/wav',
  '.ogg': 'audio/ogg',
  '.mp4': 'video/mp4',
  '.webm': 'video/webm',
  '.zip': 'application/zip',
  '.tar': 'application/x-tar',
  '.gz': 'application/gzip',
  '.tgz': 'application/gzip',
  '.bz2': 'application/x-bzip2',
  '.tbz2': 'application/x-bzip2',
  '.xz': 'application/x-xz',
  '.txz': 'application/x-xz',
};

const ARCHIVE_SUFFIXES = [
  '.tar.gz',
  '.tar.bz2',
  '.tar.xz',
  '.zip',
  '.tar',
  '.tgz',
  '.tbz2',
  '.txz',
  '.gz',
  '.bz2',
  '.xz',
  '.rar',
  '.7z',
];

function isArchiveName(name) {
  const lower = String(name || '').toLowerCase();
  return ARCHIVE_SUFFIXES.some((ext) => lower.endsWith(ext));
}

function guessMime(name, isDir) {
  if (isDir) return 'inode/directory';
  const ext = path.extname(name).toLowerCase();
  return MIME_MAP[ext] || 'application/octet-stream';
}

function isTextMime(mime, name) {
  if (
    mime.startsWith('text/') ||
    mime === 'application/json' ||
    mime === 'application/javascript' ||
    mime === 'application/xml' ||
    mime === 'image/svg+xml'
  ) {
    return true;
  }
  const ext = path.extname(name).toLowerCase();
  return [
    '.ts', '.tsx', '.js', '.mjs', '.cjs', '.json', '.md', '.txt',
    '.go', '.mod', '.sum', '.py', '.rs', '.c', '.cpp', '.h', '.java',
    '.yml', '.yaml', '.toml', '.ini', '.conf', '.scss', '.css', '.html',
    '.sh', '.bash', '.zsh', '.gitignore', '.env',
  ].includes(ext);
}

function formatMode(mode) {
  const perms = [
    (mode & 0o400) ? 'r' : '-',
    (mode & 0o200) ? 'w' : '-',
    (mode & 0o100) ? 'x' : '-',
    (mode & 0o040) ? 'r' : '-',
    (mode & 0o020) ? 'w' : '-',
    (mode & 0o010) ? 'x' : '-',
    (mode & 0o004) ? 'r' : '-',
    (mode & 0o002) ? 'w' : '-',
    (mode & 0o001) ? 'x' : '-',
  ];
  return perms.join('');
}

async function toFileInfo(fullPath, direntName) {
  const name = direntName ?? (path.basename(fullPath) || '/');
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
  const isDir = stat.isDirectory();
  const type = isDir
    ? 'DIRECTORY'
    : isArchiveName(name)
      ? 'ARCHIVE'
      : isSymlink
        ? 'SYMLINK'
        : 'FILE';
  return {
    name,
    path: fullPath,
    type,
    mimeType: guessMime(name, isDir),
    size: isDir ? 0 : stat.size,
    created: Math.floor((stat.birthtimeMs || stat.ctimeMs) / 1000),
    modified: Math.floor(stat.mtimeMs / 1000),
    accessed: Math.floor(stat.atimeMs / 1000),
    permissions: (isDir ? 'd' : '-') + formatMode(stat.mode),
    isHidden: name.startsWith('.'),
    isSymlink,
  };
}

const ARCHIVE_PY_SCRIPT = String.raw`
import base64
import bz2
import gzip
import io
import json
import lzma
import mimetypes
import os
import posixpath
import shutil
import sys
import tarfile
import time
import zipfile

ARCHIVE_SUFFIXES = (
    '.tar.gz', '.tar.bz2', '.tar.xz',
    '.zip', '.tar', '.tgz', '.tbz2', '.txz',
    '.gz', '.bz2', '.xz',
)

def is_archive_name(name: str) -> bool:
    low = (name or '').lower()
    return any(low.endswith(ext) for ext in ARCHIVE_SUFFIXES)

def norm_inner(p: str) -> str:
    parts = []
    for seg in (p or '').replace('\\', '/').split('/'):
        if not seg or seg == '.':
            continue
        if seg == '..':
            if parts:
                parts.pop()
            continue
        parts.append(seg)
    return '/'.join(parts)

def open_archive_bytes(name: str, data: bytes):
    low = name.lower()
    bio = io.BytesIO(data)
    if low.endswith('.zip'):
        zf = zipfile.ZipFile(bio, 'r')
        return ('zip', zf)
    if low.endswith(('.tar', '.tar.gz', '.tgz', '.tar.bz2', '.tbz2', '.tar.xz', '.txz')):
        mode = 'r:*'
        tf = tarfile.open(fileobj=bio, mode=mode)
        return ('tar', tf)
    if low.endswith('.gz'):
        raw = gzip.decompress(data)
        base = name[:-3] or 'content'
        return ('single', (base, raw))
    if low.endswith('.bz2'):
        raw = bz2.decompress(data)
        base = name[:-4] or 'content'
        return ('single', (base, raw))
    if low.endswith('.xz'):
        raw = lzma.decompress(data)
        base = name[:-3] or 'content'
        return ('single', (base, raw))
    raise ValueError(f'Unsupported archive format: {name}')

def read_member_bytes(kind, handle, member_path: str) -> bytes:
    target = norm_inner(member_path)
    if kind == 'zip':
        for info in handle.infolist():
            if norm_inner(info.filename) == target and not info.is_dir():
                return handle.read(info.filename)
        raise FileNotFoundError(f'Entry not found: {member_path}')
    if kind == 'tar':
        for m in handle.getmembers():
            if norm_inner(m.name) == target and m.isfile():
                f = handle.extractfile(m)
                if f is None:
                    raise FileNotFoundError(f'Cannot read entry: {member_path}')
                return f.read()
        raise FileNotFoundError(f'Entry not found: {member_path}')
    if kind == 'single':
        base, raw = handle
        if norm_inner(base) == target:
            return raw
        raise FileNotFoundError(f'Entry not found: {member_path}')
    raise ValueError('Unknown archive kind')

def resolve_archive_chain(fs_path: str, nested_chain):
    with open(fs_path, 'rb') as f:
        data = f.read()
    cur_name = os.path.basename(fs_path)
    chain = [norm_inner(c) for c in (nested_chain or []) if norm_inner(c)]
    for seg in chain:
        kind, handle = open_archive_bytes(cur_name, data)
        try:
            data = read_member_bytes(kind, handle, seg)
        finally:
            if hasattr(handle, 'close'):
                handle.close()
        cur_name = posixpath.basename(seg)
    return cur_name, data, chain

def list_all_members(kind, handle):
    out = []
    if kind == 'zip':
        for info in handle.infolist():
            p = norm_inner(info.filename)
            if not p:
                continue
            dt = info.date_time
            try:
                mtime = int(time.mktime((dt[0], dt[1], dt[2], dt[3], dt[4], dt[5], -1, -1, -1)))
            except Exception:
                mtime = 0
            out.append((p, info.is_dir(), info.file_size, mtime))
    elif kind == 'tar':
        for m in handle.getmembers():
            p = norm_inner(m.name)
            if not p:
                continue
            out.append((p, m.isdir(), int(m.size or 0), int(m.mtime or 0)))
    elif kind == 'single':
        base, raw = handle
        out.append((norm_inner(base), False, len(raw), 0))
    return out

def op_browse(req):
    fs_path = req['fsPath']
    nested_chain = req.get('nestedChain') or []
    inner_path = norm_inner(req.get('innerPath') or '')
    name, data, chain = resolve_archive_chain(fs_path, nested_chain)
    kind, handle = open_archive_bytes(name, data)
    try:
        members = list_all_members(kind, handle)
    finally:
        if hasattr(handle, 'close'):
            handle.close()

    prefix = f'{inner_path}/' if inner_path else ''
    by_name = {}
    for member_path, is_dir, size, mtime in members:
        if prefix and not member_path.startswith(prefix):
            continue
        rel = member_path[len(prefix):] if prefix else member_path
        if not rel:
            continue
        parts = rel.split('/', 1)
        child_name = parts[0]
        child_path = f'{prefix}{child_name}'
        if len(parts) > 1 or is_dir:
            existing = by_name.get(child_name)
            if not existing or existing['type'] != 'DIRECTORY':
                by_name[child_name] = {
                    'name': child_name,
                    'path': child_path,
                    'type': 'DIRECTORY',
                    'size': 0,
                    'modified': mtime,
                    'isNested': False,
                }
        else:
            nested = is_archive_name(child_name)
            entry = {
                'name': child_name,
                'path': child_path,
                'type': 'ARCHIVE' if nested else 'FILE',
                'size': size,
                'modified': mtime,
                'isNested': nested,
            }
            if nested:
                entry['nestedChain'] = [*chain, child_path]
            by_name[child_name] = entry

    entries = sorted(by_name.values(), key=lambda e: (0 if e['type'] == 'DIRECTORY' else 1, e['name'].lower()))
    return {
        'archivePath': fs_path,
        'innerPath': inner_path,
        'nestedChain': chain,
        'entries': entries,
    }

def op_read(req):
    fs_path = req['fsPath']
    nested_chain = req.get('nestedChain') or []
    entry_path = norm_inner(req.get('entryPath') or '')
    encoding = req.get('encoding') or 'utf8'
    name, data, _ = resolve_archive_chain(fs_path, nested_chain)
    kind, handle = open_archive_bytes(name, data)
    try:
        raw = read_member_bytes(kind, handle, entry_path)
    finally:
        if hasattr(handle, 'close'):
            handle.close()
    if encoding == 'base64':
        content = base64.b64encode(raw).decode('ascii')
    else:
        content = raw.decode('utf-8', errors='replace')
    return {
        'path': entry_path,
        'content': content,
        'encoding': 'base64' if encoding == 'base64' else 'utf8',
        'size': len(raw),
    }

def op_probe(req):
    fs_path = req['fsPath']
    nested_chain = req.get('nestedChain') or []
    entry_path = norm_inner(req.get('entryPath') or '')
    name, data, _ = resolve_archive_chain(fs_path, nested_chain)
    kind, handle = open_archive_bytes(name, data)
    try:
        raw = read_member_bytes(kind, handle, entry_path)
    finally:
        if hasattr(handle, 'close'):
            handle.close()
    mime, _ = mimetypes.guess_type(entry_path)
    mime = mime or 'application/octet-stream'
    sample = raw[:8192]
    is_text = b'\x00' not in sample
    if is_text:
        try:
            sample.decode('utf-8')
        except UnicodeDecodeError:
            is_text = False
    return {
        'mimeType': mime,
        'encoding': 'utf8',
        'isText': is_text or mime.startswith('text/') or mime in ('application/json', 'application/javascript', 'image/svg+xml'),
        'isArchive': is_archive_name(entry_path),
    }

def safe_join(root_dir: str, rel_path: str) -> str:
    real_root = os.path.realpath(root_dir)
    clean_rel = norm_inner(rel_path)
    if not clean_rel:
        return ''
    candidate = os.path.realpath(os.path.join(real_root, clean_rel))
    if candidate != real_root and not candidate.startswith(real_root + os.sep):
        raise ValueError(f'Unsafe archive path rejected: {rel_path}')
    return candidate

def write_extracted_bytes(dest_dir: str, rel_path: str, raw: bytes) -> int:
    target = safe_join(dest_dir, rel_path)
    if not target:
        return 0
    os.makedirs(os.path.dirname(target), exist_ok=True)
    with open(target, 'wb') as out:
        out.write(raw)
    return len(raw)

def extract_all_bytes(name: str, data: bytes, dest_dir: str, flatten_single_root: bool):
    kind, handle = open_archive_bytes(name, data)
    files_written = 0
    bytes_written = 0
    try:
        members = list_all_members(kind, handle)
        strip_prefix = ''
        if flatten_single_root:
            tops = {m[0].split('/', 1)[0] for m in members if m[0]}
            if len(tops) == 1:
                only = next(iter(tops))
                has_nested = any(m[0].startswith(only + '/') for m in members)
                if has_nested:
                    strip_prefix = only + '/'

        if kind == 'zip':
            for info in handle.infolist():
                p = norm_inner(info.filename)
                if not p:
                    continue
                if strip_prefix and p.startswith(strip_prefix):
                    p = p[len(strip_prefix):]
                if not p:
                    continue
                if info.is_dir():
                    t = safe_join(dest_dir, p)
                    if t:
                        os.makedirs(t, exist_ok=True)
                else:
                    raw = handle.read(info.filename)
                    bytes_written += write_extracted_bytes(dest_dir, p, raw)
                    files_written += 1
        elif kind == 'tar':
            for m in handle.getmembers():
                p = norm_inner(m.name)
                if not p:
                    continue
                if strip_prefix and p.startswith(strip_prefix):
                    p = p[len(strip_prefix):]
                if not p:
                    continue
                if m.isdir():
                    t = safe_join(dest_dir, p)
                    if t:
                        os.makedirs(t, exist_ok=True)
                elif m.isfile():
                    f = handle.extractfile(m)
                    if f is not None:
                        raw = f.read()
                        bytes_written += write_extracted_bytes(dest_dir, p, raw)
                        files_written += 1
        elif kind == 'single':
            base, raw = handle
            bytes_written += write_extracted_bytes(dest_dir, norm_inner(base), raw)
            files_written += 1
    finally:
        if hasattr(handle, 'close'):
            handle.close()
    return files_written, bytes_written

def archive_stem(p: str) -> str:
    base = os.path.basename(p)
    low = base.lower()
    for ext in ARCHIVE_SUFFIXES:
        if low.endswith(ext):
            return base[:-len(ext)] or base
    return os.path.splitext(base)[0] or base

def op_extract(req):
    archives = req.get('archives') or []
    base_dest = req['destinationDir']
    create_sub = bool(req.get('createSubdirectory'))
    flatten_single = bool(req.get('flattenSingleRoot'))
    os.makedirs(base_dest, exist_ok=True)

    succeeded = 0
    errors = []
    total_files = 0
    total_bytes = 0
    for arch in archives:
        try:
            dest = os.path.join(base_dest, archive_stem(arch)) if create_sub else base_dest
            os.makedirs(dest, exist_ok=True)
            with open(arch, 'rb') as f:
                data = f.read()
            f_cnt, b_cnt = extract_all_bytes(os.path.basename(arch), data, dest, flatten_single)
            total_files += f_cnt
            total_bytes += b_cnt
            succeeded += 1
        except Exception as exc:
            errors.append({'source': arch, 'error': str(exc), 'code': 'unknown'})

    return {
        'operationId': f'extract-{int(time.time() * 1000)}',
        'total': len(archives),
        'succeeded': succeeded,
        'skipped': 0,
        'failed': len(errors),
        'extractedFiles': total_files,
        'extractedBytes': total_bytes,
        'errors': errors,
    }

def op_selective_extract(req):
    default_dest = req.get('destinationDir') or ''
    items = req.get('items') or []
    succeeded = 0
    errors = []
    total_files = 0
    total_bytes = 0

    for item in items:
        arch = item['archivePath']
        dest = item.get('destinationDir') or default_dest
        nested_chain = item.get('nestedChain') or []
        wanted = [norm_inner(e) for e in (item.get('entries') or []) if norm_inner(e)]
        strip_prefix = norm_inner(item.get('stripPrefix') or '')
        flatten_paths = bool(item.get('flattenPaths'))
        try:
            os.makedirs(dest, exist_ok=True)
            name, data, _ = resolve_archive_chain(arch, nested_chain)
            kind, handle = open_archive_bytes(name, data)
            try:
                members = list_all_members(kind, handle)
                for member_path, is_dir, _size, _mtime in members:
                    matched = not wanted or any(
                        member_path == w or member_path.startswith(w + '/')
                        for w in wanted
                    )
                    if not matched:
                        continue
                    rel = member_path
                    if strip_prefix and rel.startswith(strip_prefix + '/'):
                        rel = rel[len(strip_prefix) + 1:]
                    elif strip_prefix and rel == strip_prefix:
                        rel = posixpath.basename(rel)
                    if flatten_paths:
                        if is_dir:
                            continue
                        rel = posixpath.basename(rel)
                    if not rel:
                        continue
                    if is_dir:
                        t = safe_join(dest, rel)
                        if t:
                            os.makedirs(t, exist_ok=True)
                    else:
                        raw = read_member_bytes(kind, handle, member_path)
                        total_bytes += write_extracted_bytes(dest, rel, raw)
                        total_files += 1
            finally:
                if hasattr(handle, 'close'):
                    handle.close()
            succeeded += 1
        except Exception as exc:
            errors.append({'source': arch, 'error': str(exc), 'code': 'unknown'})

    return {
        'operationId': f'selective-{int(time.time() * 1000)}',
        'total': len(items),
        'succeeded': succeeded,
        'skipped': 0,
        'failed': len(errors),
        'extractedFiles': total_files,
        'extractedBytes': total_bytes,
        'errors': errors,
    }

def op_create_zip(req):
    sources = req.get('paths') or []
    dest = req['destination']
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    added = 0
    with zipfile.ZipFile(dest, 'w', compression=zipfile.ZIP_DEFLATED) as zf:
        for src in sources:
            src = os.path.abspath(src)
            if not os.path.exists(src):
                raise FileNotFoundError(f'Source does not exist: {src}')
            base_parent = os.path.dirname(src)
            if os.path.isdir(src):
                root_arc = os.path.relpath(src, base_parent).replace(os.sep, '/')
                zf.write(src, root_arc)
                for root, dirs, files in os.walk(src):
                    for d in dirs:
                        full_d = os.path.join(root, d)
                        arc_d = os.path.relpath(full_d, base_parent).replace(os.sep, '/')
                        zf.write(full_d, arc_d)
                    for fn in files:
                        full_f = os.path.join(root, fn)
                        arc_f = os.path.relpath(full_f, base_parent).replace(os.sep, '/')
                        zf.write(full_f, arc_f)
                        added += 1
            else:
                zf.write(src, os.path.basename(src))
                added += 1
    return {'success': True, 'affectedPaths': [dest], 'message': f'Archived {added} file(s)'}

def atomic_write_zip_from_builder(fs_path: str, build_fn):
    parent = os.path.dirname(os.path.abspath(fs_path))
    os.makedirs(parent, exist_ok=True)
    fd, tmp_path = tempfile.mkstemp(prefix='.archive-edit-', suffix='.zip', dir=parent)
    os.close(fd)
    try:
        with zipfile.ZipFile(tmp_path, 'w', compression=zipfile.ZIP_DEFLATED) as zout:
            build_fn(zout)
        os.replace(tmp_path, fs_path)
    except Exception:
        if os.path.exists(tmp_path):
            os.remove(tmp_path)
        raise

def op_import_entries(req):
    fs_path = req['fsPath']
    dest_prefix = norm_inner(req.get('destinationPath') or '')
    sources = req.get('sources') or []
    if not fs_path.lower().endswith('.zip'):
        raise ValueError('Only top-level .zip archives support importing entries.')
    if not os.path.exists(fs_path):
        raise FileNotFoundError(f'Archive not found: {fs_path}')

    new_items = []
    new_names = set()
    for src in sources:
        abs_src = os.path.abspath(src)
        if not os.path.exists(abs_src):
            raise FileNotFoundError(f'Source does not exist: {abs_src}')
        base_name = os.path.basename(abs_src)
        if os.path.isdir(abs_src):
            root_rel = f'{dest_prefix}/{base_name}' if dest_prefix else base_name
            root_rel = norm_inner(root_rel)
            for root, dirs, files in os.walk(abs_src):
                rel_dir = os.path.relpath(root, abs_src).replace(os.sep, '/')
                arc_dir = root_rel if rel_dir == '.' else norm_inner(f'{root_rel}/{rel_dir}')
                if arc_dir and arc_dir not in new_names:
                    new_items.append(('dir', abs_src if rel_dir == '.' else root, arc_dir))
                    new_names.add(arc_dir)
                for fn in files:
                    full_f = os.path.join(root, fn)
                    arc_f = norm_inner(f'{arc_dir}/{fn}')
                    new_items.append(('file', full_f, arc_f))
                    new_names.add(arc_f)
        else:
            arc_f = norm_inner(f'{dest_prefix}/{base_name}' if dest_prefix else base_name)
            new_items.append(('file', abs_src, arc_f))
            new_names.add(arc_f)

    def build(zout):
        with zipfile.ZipFile(fs_path, 'r') as zin:
            for info in zin.infolist():
                key = norm_inner(info.filename)
                if not key or key in new_names:
                    continue
                if info.is_dir():
                    zout.writestr(info, b'')
                else:
                    zout.writestr(info, zin.read(info.filename))
        for kind, disk_path, arc_path in new_items:
            if kind == 'dir':
                zout.write(disk_path, arc_path.rstrip('/') + '/')
            else:
                zout.write(disk_path, arc_path)

    atomic_write_zip_from_builder(fs_path, build)
    affected = [item[2] for item in new_items]
    return {'success': True, 'affectedPaths': affected, 'message': f'Imported {len(new_items)} entry(s) into archive'}

def op_delete_entries(req):
    fs_path = req['fsPath']
    targets = [norm_inner(e) for e in (req.get('entries') or []) if norm_inner(e)]
    if not fs_path.lower().endswith('.zip'):
        raise ValueError('Only top-level .zip archives support deleting entries.')
    if not targets:
        return {'success': True, 'affectedPaths': [], 'message': 'No archive entries selected'}

    removed = []
    def matches_target(entry_name: str) -> bool:
        for t in targets:
            if entry_name == t or entry_name.startswith(t + '/'):
                return True
        return False

    def build(zout):
        with zipfile.ZipFile(fs_path, 'r') as zin:
            for info in zin.infolist():
                key = norm_inner(info.filename)
                if key and matches_target(key):
                    removed.append(key)
                    continue
                if info.is_dir():
                    zout.writestr(info, b'')
                else:
                    zout.writestr(info, zin.read(info.filename))

    atomic_write_zip_from_builder(fs_path, build)
    return {'success': True, 'affectedPaths': removed, 'message': f'Removed {len(removed)} entry(s) from archive'}

def main():
    import tempfile
    globals()['tempfile'] = tempfile
    req = json.loads(sys.stdin.read())
    op = req['op']
    if op == 'browse':
        res = op_browse(req)
    elif op == 'read':
        res = op_read(req)
    elif op == 'probe':
        res = op_probe(req)
    elif op == 'extract':
        res = op_extract(req)
    elif op == 'selective_extract':
        res = op_selective_extract(req)
    elif op == 'create_zip':
        res = op_create_zip(req)
    elif op == 'import_entries':
        res = op_import_entries(req)
    elif op == 'delete_entries':
        res = op_delete_entries(req)
    else:
        raise ValueError(f'Unknown op: {op}')
    sys.stdout.write(json.dumps(res))

if __name__ == '__main__':
    main()
`;

function runArchivePython(payload) {
  return new Promise((resolve, reject) => {
    const child = spawn('python3', ['-c', ARCHIVE_PY_SCRIPT], {
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    const stdoutChunks = [];
    const stderrChunks = [];
    child.stdout.on('data', (chunk) => stdoutChunks.push(chunk));
    child.stderr.on('data', (chunk) => stderrChunks.push(chunk));
    child.on('error', (err) => reject(err));
    child.on('close', (code) => {
      const out = Buffer.concat(stdoutChunks).toString('utf8');
      const errOut = Buffer.concat(stderrChunks).toString('utf8');
      if (code !== 0) {
        reject(new Error(errOut.trim() || out.trim() || `Archive worker exited with ${code}`));
        return;
      }
      try {
        resolve(JSON.parse(out));
      } catch (parseErr) {
        reject(new Error(`Invalid archive worker output: ${parseErr?.message || parseErr}`));
      }
    });
    child.stdin.write(JSON.stringify(payload));
    child.stdin.end();
  });
}

app.get('/api/os-info', (_req, res) => {
  res.json({
    os: 'linux',
    version: os.release(),
    arch: process.arch,
    hostname: os.hostname(),
    systemDrive: null,
  });
});

async function readPathCapacity(targetPath) {
  const resolved = resolveRealPath(targetPath);
  let probePath = resolved;
  while (!fs.existsSync(probePath) && probePath !== path.dirname(probePath)) {
    probePath = path.dirname(probePath);
  }
  const st = await fsp.statfs(probePath);
  const totalSpace = Number(st.bsize) * Number(st.blocks);
  const freeSpace = Number(st.bsize) * Number(st.bavail);
  let fileSystem = 'ext4';
  try {
    const mounts = await fsp.readFile('/proc/mounts', 'utf8');
    let bestLen = 0;
    for (const line of mounts.split('\n')) {
      const parts = line.split(' ');
      if (parts.length >= 3) {
        const mountPoint = parts[1];
        if ((probePath === mountPoint || probePath.startsWith(mountPoint === '/' ? '/' : mountPoint + '/')) && mountPoint.length >= bestLen) {
          bestLen = mountPoint.length;
          fileSystem = parts[2];
        }
      }
    }
  } catch {}
  return { totalSpace, freeSpace, fileSystem };
}

app.get('/api/drives', async (_req, res) => {
  try {
    const candidates = [
      { name: 'Workspace', path: WORKSPACE_ROOT },
      { name: 'Root (/)', path: '/' },
      { name: 'Temp (/tmp)', path: '/tmp' },
    ];
    const schemes = await Promise.all(
      candidates.map(async (item) => {
        const cap = await readPathCapacity(item.path).catch(() => ({
          totalSpace: 0,
          freeSpace: 0,
          fileSystem: 'ext4',
        }));
        return {
          name: item.name,
          path: item.path,
          schemeType: 'fixed',
          totalSpace: cap.totalSpace,
          freeSpace: cap.freeSpace,
          fileSystem: cap.fileSystem,
        };
      }),
    );
    res.json({ schemes });
  } catch (err) {
    res.status(500).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/capacity', async (req, res) => {
  try {
    const cap = await readPathCapacity(req.body?.path || WORKSPACE_ROOT);
    res.json(cap);
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.get('/api/special-folders', (_req, res) => {
  const folders = [
    { name: 'Workspace', path: WORKSPACE_ROOT, icon: 'folder' },
    { name: 'Documentation', path: path.join(WORKSPACE_ROOT, 'docs'), icon: 'description' },
    { name: 'Manager', path: path.join(WORKSPACE_ROOT, 'manager'), icon: 'code' },
    { name: 'Frontend', path: path.join(WORKSPACE_ROOT, 'manager', 'frontend'), icon: 'web' },
    { name: 'Public Assets', path: PUBLIC_DIR, icon: 'image' },
    { name: 'Temp', path: '/tmp', icon: 'folder_open' },
  ].filter((f) => fs.existsSync(f.path));
  res.json({ folders });
});

app.post('/api/fs/list', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const showHidden = !!req.body?.showHidden;
    const dirents = await fsp.readdir(targetPath, { withFileTypes: true });
    const entries = [];
    let totalSize = 0;

    for (const d of dirents) {
      if (!showHidden && d.name.startsWith('.')) continue;
      const childPath = path.join(targetPath, d.name);
      try {
        const info = await toFileInfo(childPath, d.name);
        totalSize += info.size;
        entries.push(info);
      } catch {
        // Skip unreadable entries
      }
    }

    res.json({
      path: targetPath,
      entries,
      totalSize,
    });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/info', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const info = await toFileInfo(targetPath);
    res.json(info);
  } catch (err) {
    res.status(404).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/exists', (req, res) => {
  const targetPath = resolveRealPath(req.body?.path);
  res.json({ exists: fs.existsSync(targetPath) });
});

app.post('/api/fs/probe', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const name = path.basename(targetPath);
    const stat = await fsp.stat(targetPath);
    const mimeType = guessMime(name, stat.isDirectory());
    res.json({
      mimeType,
      encoding: 'utf8',
      isText: !stat.isDirectory() && isTextMime(mimeType, name),
      isArchive: !stat.isDirectory() && isArchiveName(name),
    });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/read', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const encoding = req.body?.encoding === 'base64' ? 'base64' : 'utf8';
    const buf = await fsp.readFile(targetPath);
    res.json({
      path: targetPath,
      content: encoding === 'base64' ? buf.toString('base64') : buf.toString('utf8'),
      encoding,
      size: buf.length,
    });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/read-range', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const offset = Math.max(0, Number(req.body?.offset || 0));
    const length = Math.max(0, Math.min(1024 * 1024, Number(req.body?.length || 65536)));
    const fd = await fsp.open(targetPath, 'r');
    try {
      const stat = await fd.stat();
      const buf = Buffer.alloc(length);
      const { bytesRead } = await fd.read(buf, 0, length, offset);
      res.json({
        path: targetPath,
        content: buf.subarray(0, bytesRead).toString('base64'),
        encoding: 'base64',
        size: stat.size,
      });
    } finally {
      await fd.close();
    }
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/write', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const encoding = req.body?.encoding === 'base64' ? 'base64' : 'utf8';
    const content = String(req.body?.content ?? '');
    await fsp.mkdir(path.dirname(targetPath), { recursive: true });
    await fsp.writeFile(targetPath, Buffer.from(content, encoding));
    res.json({ success: true, affectedPaths: [targetPath] });
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/upload', async (req, res) => {
  try {
    const targetDir = resolveRealPath(req.body?.directory);
    const files = Array.isArray(req.body?.files) ? req.body.files : [];
    if (!files.length) {
      res.status(400).json({ error: 'No files provided for upload.' });
      return;
    }
    await fsp.mkdir(targetDir, { recursive: true });
    const uploadedPaths = [];
    for (const item of files) {
      const cleanName = path.basename(String(item?.name || '').trim() || `upload-${Date.now()}`);
      if (!cleanName || cleanName === '.' || cleanName === '..') continue;
      const dest = await uniqueDestinationPath(path.join(targetDir, cleanName));
      const raw = Buffer.from(String(item?.contentBase64 || ''), 'base64');
      await fsp.writeFile(dest, raw);
      uploadedPaths.push(dest);
    }
    res.json({ success: true, uploadedPaths, count: uploadedPaths.length });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/mkdir', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    await fsp.mkdir(targetPath, { recursive: req.body?.recursive !== false });
    res.json({ success: true, affectedPaths: [targetPath] });
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/delete', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const useTrash = req.body?.useTrash === true;
    if (useTrash) {
      await moveToTrash(targetPath);
    } else {
      await fsp.rm(targetPath, { recursive: !!req.body?.recursive, force: true });
    }
    res.json({ success: true, affectedPaths: [targetPath] });
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/batch-delete', async (req, res) => {
  const paths = Array.isArray(req.body?.paths) ? req.body.paths : [];
  const recursive = req.body?.recursive !== false;
  const useTrash = req.body?.useTrash !== false;
  let succeeded = 0;
  const errors = [];
  for (const rawPath of paths) {
    const targetPath = resolveRealPath(rawPath);
    try {
      if (useTrash) {
        await moveToTrash(targetPath);
      } else {
        await fsp.rm(targetPath, { recursive, force: true });
      }
      succeeded++;
    } catch (err) {
      errors.push({
        source: targetPath,
        error: err instanceof Error ? err.message : String(err),
        code: 'unknown',
      });
    }
  }
  res.json({
    operationId: `delete-${Date.now()}`,
    total: paths.length,
    succeeded,
    skipped: 0,
    failed: errors.length,
    errors,
  });
});

app.post('/api/fs/move', async (req, res) => {
  try {
    const src = resolveRealPath(req.body?.source);
    const dest = resolveRealPath(req.body?.destination);
    await movePathSafely(src, dest);
    res.json({ success: true, affectedPaths: [src, dest] });
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/copy', async (req, res) => {
  try {
    const src = resolveRealPath(req.body?.source);
    let dest = resolveRealPath(req.body?.destination);
    if (fs.existsSync(dest) && path.resolve(src) === path.resolve(dest)) {
      dest = await uniqueDestinationPath(dest);
    }
    await fsp.mkdir(path.dirname(dest), { recursive: true });
    await fsp.cp(src, dest, { recursive: req.body?.recursive !== false });
    res.json({ success: true, affectedPaths: [dest] });
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/hash', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const stat = await fsp.stat(targetPath);
    if (stat.isDirectory()) {
      res.status(400).json({ error: 'Cannot compute file hash for a directory.' });
      return;
    }
    const sha256 = crypto.createHash('sha256');
    const sha512 = crypto.createHash('sha512');
    const sha1 = crypto.createHash('sha1');
    const md5 = crypto.createHash('md5');
    await new Promise((resolve, reject) => {
      const stream = fs.createReadStream(targetPath);
      stream.on('data', (chunk) => {
        sha256.update(chunk);
        sha512.update(chunk);
        sha1.update(chunk);
        md5.update(chunk);
      });
      stream.on('end', resolve);
      stream.on('error', reject);
    });
    const sha256Hex = sha256.digest('hex');
    res.json({
      hash: sha256Hex,
      size: stat.size,
      algorithms: {
        sha256: sha256Hex,
        sha512: sha512.digest('hex'),
        sha1: sha1.digest('hex'),
        md5: md5.digest('hex'),
      },
    });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/attributes', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    if (req.body?.update && typeof req.body.update === 'object') {
      const update = req.body.update;
      if (typeof update.unixMode === 'number' && Number.isFinite(update.unixMode)) {
        await fsp.chmod(targetPath, update.unixMode);
      } else if (typeof update.readOnly === 'boolean') {
        const st = await fsp.stat(targetPath);
        const nextMode = update.readOnly ? (st.mode & ~0o222) : (st.mode | 0o200);
        await fsp.chmod(targetPath, nextMode);
      }
    }
    const stat = await fsp.stat(targetPath);
    const unixMode = stat.mode & 0o7777;
    const isDir = stat.isDirectory();
    res.json({
      permissions: (isDir ? 'd' : '-') + formatMode(stat.mode),
      unixMode,
      readOnly: (stat.mode & 0o200) === 0,
      hidden: path.basename(targetPath).startsWith('.'),
      system: false,
      archive: false,
    });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/mtime', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const ms = Number(req.body?.unixMilliseconds);
    if (!Number.isFinite(ms)) {
      res.status(400).json({ error: 'Invalid timestamp' });
      return;
    }
    const mtime = new Date(ms);
    await fsp.utimes(targetPath, mtime, mtime);
    res.json({ ok: true });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/link', async (req, res) => {
  try {
    const target = resolveRealPath(req.body?.target);
    const linkPath = resolveRealPath(req.body?.linkPath);
    const kind = req.body?.kind === 'hard' ? 'hard' : 'symbolic';
    await fsp.mkdir(path.dirname(linkPath), { recursive: true });
    if (kind === 'hard') {
      await fsp.link(target, linkPath);
    } else {
      await fsp.symlink(target, linkPath);
    }
    res.json({ ok: true, linkPath });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/resolve-link', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const lstat = await fsp.lstat(targetPath);
    let resolvedTarget = targetPath;
    if (lstat.isSymbolicLink()) {
      const rawTarget = await fsp.readlink(targetPath);
      resolvedTarget = path.resolve(path.dirname(targetPath), rawTarget);
    }
    let isDir = false;
    let exists = false;
    try {
      const st = await fsp.stat(resolvedTarget);
      exists = true;
      isDir = st.isDirectory();
    } catch {
      exists = false;
    }
    res.json({ target: resolvedTarget, isDir, exists });
  } catch (err) {
    res.status(404).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/split', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.body?.path);
    const partSize = Math.max(1024, Math.floor(Number(req.body?.partSize || 1024 * 1024)));
    const fd = await fsp.open(targetPath, 'r');
    const parts = [];
    try {
      const stat = await fd.stat();
      if (stat.isDirectory()) throw new Error('Cannot split a directory.');
      let index = 1;
      let offset = 0;
      const buf = Buffer.allocUnsafe(Math.min(partSize, 16 * 1024 * 1024));
      while (offset < stat.size || index === 1) {
        const partPath = `${targetPath}.part${String(index).padStart(3, '0')}`;
        const out = await fsp.open(partPath, 'w');
        let writtenForPart = 0;
        try {
          while (writtenForPart < partSize && offset < stat.size) {
            const toRead = Math.min(buf.length, partSize - writtenForPart, stat.size - offset);
            const { bytesRead } = await fd.read(buf, 0, toRead, offset);
            if (!bytesRead) break;
            await out.write(buf, 0, bytesRead);
            offset += bytesRead;
            writtenForPart += bytesRead;
          }
        } finally {
          await out.close();
        }
        parts.push(partPath);
        index++;
        if (offset >= stat.size) break;
      }
    } finally {
      await fd.close();
    }
    res.json({ parts });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/join', async (req, res) => {
  try {
    const parts = Array.isArray(req.body?.parts) ? req.body.parts.map(resolveRealPath) : [];
    const destination = resolveRealPath(req.body?.destination);
    if (parts.length < 2) {
      res.status(400).json({ error: 'Select at least two parts to join.' });
      return;
    }
    await fsp.mkdir(path.dirname(destination), { recursive: true });
    const out = fs.createWriteStream(destination);
    for (const part of parts) {
      await new Promise((resolve, reject) => {
        const inp = fs.createReadStream(part);
        inp.on('error', reject);
        inp.on('end', resolve);
        inp.pipe(out, { end: false });
      });
    }
    await new Promise((resolve, reject) => {
      out.on('error', reject);
      out.on('finish', resolve);
      out.end();
    });
    res.json({ destination });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/search', async (req, res) => {
  try {
    const rootPath = resolveRealPath(req.body?.path);
    const pattern = String(req.body?.pattern || '').trim();
    const recursive = req.body?.recursive !== false;
    const regexStr = pattern
      .replace(/[.+^${}()|[\]\\]/g, '\\$&')
      .replace(/\*/g, '.*')
      .replace(/\?/g, '.');
    const regex = pattern ? new RegExp(regexStr, 'i') : /.*/;

    const matches = [];
    const MAX_MATCHES = 300;

    async function walk(dir, depth) {
      if (matches.length >= MAX_MATCHES) return;
      let dirents;
      try {
        dirents = await fsp.readdir(dir, { withFileTypes: true });
      } catch {
        return;
      }
      for (const d of dirents) {
        if (matches.length >= MAX_MATCHES) break;
        if (d.name === 'node_modules' || d.name === '.git') continue;
        const full = path.join(dir, d.name);
        if (regex.test(d.name)) {
          try {
            const info = await toFileInfo(full, d.name);
            info.searchBasePath = rootPath;
            info.searchPattern = pattern;
            info.searchParentPath = dir;
            matches.push(info);
          } catch {}
        }
        if (recursive && d.isDirectory() && depth < 6) {
          await walk(full, depth + 1);
        }
      }
    }

    await walk(rootPath, 0);
    res.json({
      path: rootPath,
      matches,
      totalMatches: matches.length,
    });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/fs/disk-usage', async (req, res) => {
  try {
    const rootPath = resolveRealPath(req.body?.path);
    const dirents = await fsp.readdir(rootPath, { withFileTypes: true });
    const entries = [];
    let totalSize = 0;
    let fileCount = 0;
    let dirCount = 0;

    async function dirSize(dir, depth) {
      let sum = 0;
      let files = 0;
      let dirs = 0;
      let items;
      try {
        items = await fsp.readdir(dir, { withFileTypes: true });
      } catch {
        return { sum, files, dirs };
      }
      for (const item of items) {
        if (item.name === 'node_modules' || item.name === '.git') continue;
        const p = path.join(dir, item.name);
        try {
          if (item.isDirectory()) {
            dirs++;
            if (depth < 5) {
              const sub = await dirSize(p, depth + 1);
              sum += sub.sum;
              files += sub.files;
              dirs += sub.dirs;
            }
          } else {
            files++;
            const st = await fsp.stat(p);
            sum += st.size;
          }
        } catch {}
      }
      return { sum, files, dirs };
    }

    for (const d of dirents) {
      if (d.name === 'node_modules' || d.name === '.git') continue;
      const full = path.join(rootPath, d.name);
      try {
        if (d.isDirectory()) {
          dirCount++;
          const sub = await dirSize(full, 1);
          totalSize += sub.sum;
          fileCount += sub.files;
          dirCount += sub.dirs;
          entries.push({
            name: d.name,
            path: full,
            isDir: true,
            size: sub.sum,
            fileCount: sub.files,
            dirCount: sub.dirs,
          });
        } else {
          fileCount++;
          const st = await fsp.stat(full);
          totalSize += st.size;
          entries.push({
            name: d.name,
            path: full,
            isDir: false,
            size: st.size,
            fileCount: 1,
            dirCount: 0,
          });
        }
      } catch {}
    }

    entries.sort((a, b) => b.size - a.size);
    res.json({
      rootPath,
      totalSize,
      fileCount,
      dirCount,
      entries,
    });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.get('/api/stream', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.query.path);
    const stat = await fsp.stat(targetPath);
    if (stat.isDirectory()) {
      res.status(400).send('Cannot stream directory');
      return;
    }
    const mime = guessMime(path.basename(targetPath), false);
    res.setHeader('Content-Type', mime);
    res.setHeader('Accept-Ranges', 'bytes');

    const range = req.headers.range;
    if (range) {
      const match = /bytes=(\d+)-(\d*)/.exec(range);
      if (match) {
        const start = Number(match[1]);
        const end = match[2] ? Math.min(stat.size - 1, Number(match[2])) : stat.size - 1;
        res.status(206);
        res.setHeader('Content-Range', `bytes ${start}-${end}/${stat.size}`);
        res.setHeader('Content-Length', String(end - start + 1));
        fs.createReadStream(targetPath, { start, end }).pipe(res);
        return;
      }
    }
    res.setHeader('Content-Length', String(stat.size));
    fs.createReadStream(targetPath).pipe(res);
  } catch (err) {
    res.status(404).send(err instanceof Error ? err.message : 'Not found');
  }
});

app.get('/api/download', async (req, res) => {
  try {
    const targetPath = resolveRealPath(req.query.path);
    const stat = await fsp.stat(targetPath);
    if (stat.isDirectory()) {
      res.status(400).send('Cannot download directory directly; create a ZIP archive first.');
      return;
    }
    const filename = path.basename(targetPath);
    res.download(targetPath, filename);
  } catch (err) {
    res.status(404).send(err instanceof Error ? err.message : 'Not found');
  }
});

// ── Archive endpoints ────────────────────────────────────────────────────────
app.post('/api/archive/browse', async (req, res) => {
  try {
    const result = await runArchivePython({
      op: 'browse',
      fsPath: resolveRealPath(req.body?.fsPath),
      nestedChain: Array.isArray(req.body?.nestedChain) ? req.body.nestedChain : [],
      innerPath: String(req.body?.innerPath || ''),
    });
    res.json(result);
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/archive/read', async (req, res) => {
  try {
    const result = await runArchivePython({
      op: 'read',
      fsPath: resolveRealPath(req.body?.fsPath),
      nestedChain: Array.isArray(req.body?.nestedChain) ? req.body.nestedChain : [],
      entryPath: String(req.body?.entryPath || ''),
      encoding: req.body?.encoding === 'base64' ? 'base64' : 'utf8',
    });
    res.json(result);
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/archive/probe', async (req, res) => {
  try {
    const result = await runArchivePython({
      op: 'probe',
      fsPath: resolveRealPath(req.body?.fsPath),
      nestedChain: Array.isArray(req.body?.nestedChain) ? req.body.nestedChain : [],
      entryPath: String(req.body?.entryPath || ''),
    });
    res.json(result);
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.get('/api/archive/stream', async (req, res) => {
  try {
    const fsPath = resolveRealPath(req.query.fsPath);
    let nestedChain = [];
    if (typeof req.query.nestedChain === 'string' && req.query.nestedChain) {
      try {
        const parsed = JSON.parse(req.query.nestedChain);
        if (Array.isArray(parsed)) nestedChain = parsed;
      } catch {}
    }
    const entryPath = String(req.query.entryPath || '');
    const result = await runArchivePython({
      op: 'read',
      fsPath,
      nestedChain,
      entryPath,
      encoding: 'base64',
    });
    const buf = Buffer.from(result.content || '', 'base64');
    res.setHeader('Content-Type', guessMime(path.basename(entryPath), false));
    res.setHeader('Content-Length', String(buf.length));
    res.send(buf);
  } catch (err) {
    res.status(404).send(err instanceof Error ? err.message : 'Not found');
  }
});

app.post('/api/archive/extract', async (req, res) => {
  try {
    const archives = Array.isArray(req.body?.archives)
      ? req.body.archives.map(resolveRealPath)
      : [];
    const result = await runArchivePython({
      op: 'extract',
      archives,
      destinationDir: resolveRealPath(req.body?.destinationDir),
      createSubdirectory: !!req.body?.createSubdirectory,
      flattenSingleRoot: !!req.body?.flattenSingleRoot,
    });
    res.json(result);
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/archive/selective-extract', async (req, res) => {
  try {
    const items = Array.isArray(req.body?.items)
      ? req.body.items.map((item) => ({
          ...item,
          archivePath: resolveRealPath(item.archivePath),
          destinationDir: item.destinationDir ? resolveRealPath(item.destinationDir) : undefined,
        }))
      : [];
    const result = await runArchivePython({
      op: 'selective_extract',
      destinationDir: resolveRealPath(req.body?.destinationDir),
      items,
    });
    res.json(result);
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/archive/create-zip', async (req, res) => {
  try {
    const rawPaths = Array.isArray(req.body?.paths)
      ? req.body.paths
      : (Array.isArray(req.body?.sources) ? req.body.sources : []);
    const paths = rawPaths.map(resolveRealPath);
    const destination = resolveRealPath(req.body?.destination);
    const result = await runArchivePython({
      op: 'create_zip',
      paths,
      destination,
    });
    res.json(result);
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/archive/import-entries', async (req, res) => {
  try {
    const result = await runArchivePython({
      op: 'import_entries',
      fsPath: resolveRealPath(req.body?.fsPath),
      destinationPath: String(req.body?.destinationPath || ''),
      sources: Array.isArray(req.body?.sources) ? req.body.sources.map(resolveRealPath) : [],
    });
    res.json(result);
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/archive/delete-entries', async (req, res) => {
  try {
    const result = await runArchivePython({
      op: 'delete_entries',
      fsPath: resolveRealPath(req.body?.fsPath),
      entries: Array.isArray(req.body?.entries) ? req.body.entries : [],
    });
    res.json(result);
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/archive/copy-entries', async (req, res) => {
  try {
    const fsPath = resolveRealPath(req.body?.fsPath);
    const destinationDir = resolveRealPath(req.body?.destinationDir);
    const extractResult = await runArchivePython({
      op: 'selective_extract',
      destinationDir,
      items: [{
        archivePath: fsPath,
        nestedChain: Array.isArray(req.body?.nestedChain) ? req.body.nestedChain : [],
        entries: Array.isArray(req.body?.entries) ? req.body.entries : [],
        stripPrefix: String(req.body?.stripPrefix || ''),
        flattenPaths: !!req.body?.flattenPaths,
        destinationDir,
      }],
    });
    const ok = extractResult.failed === 0;
    res.json({
      success: ok,
      affectedPaths: [destinationDir],
      message: ok ? `Extracted ${extractResult.extractedFiles} file(s)` : (extractResult.errors?.[0]?.error || 'Archive copy failed'),
    });
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/archive/move-entries', async (req, res) => {
  try {
    const fsPath = resolveRealPath(req.body?.fsPath);
    const nestedChain = Array.isArray(req.body?.nestedChain) ? req.body.nestedChain : [];
    if (nestedChain.length > 0) {
      res.status(400).json({ success: false, message: 'Moving entries out of nested archives is not supported.' });
      return;
    }
    const entries = Array.isArray(req.body?.entries) ? req.body.entries : [];
    const destinationDir = resolveRealPath(req.body?.destinationDir);
    const extractResult = await runArchivePython({
      op: 'selective_extract',
      destinationDir,
      items: [{
        archivePath: fsPath,
        nestedChain: [],
        entries,
        stripPrefix: String(req.body?.stripPrefix || ''),
        flattenPaths: !!req.body?.flattenPaths,
        destinationDir,
      }],
    });
    if (extractResult.failed > 0) {
      res.status(400).json({ success: false, message: extractResult.errors?.[0]?.error || 'Extract phase failed' });
      return;
    }
    const delResult = await runArchivePython({
      op: 'delete_entries',
      fsPath,
      entries,
    });
    res.json({
      success: !!delResult.success,
      affectedPaths: [fsPath, destinationDir],
      message: `Moved ${extractResult.extractedFiles} file(s) out of archive`,
    });
  } catch (err) {
    res.status(400).json({ success: false, message: err instanceof Error ? err.message : String(err) });
  }
});

// ── Trash / Recycle Bin endpoints ────────────────────────────────────────────
app.get('/api/trash', async (_req, res) => {
  const manifest = await readTrashManifest();
  const valid = manifest.filter((item) => item?.trashedPath && fs.existsSync(item.trashedPath));
  if (valid.length !== manifest.length) {
    await writeTrashManifest(valid);
  }
  res.json(valid);
});

app.post('/api/trash/restore', async (req, res) => {
  try {
    const id = String(req.body?.id || '');
    const manifest = await readTrashManifest();
    const idx = manifest.findIndex((item) => item.id === id);
    if (idx < 0) {
      res.status(404).json({ error: 'Trash item not found.' });
      return;
    }
    const entry = manifest[idx];
    const dest = await uniqueDestinationPath(entry.originalPath);
    await movePathSafely(entry.trashedPath, dest);
    manifest.splice(idx, 1);
    await writeTrashManifest(manifest);
    res.json({ destination: dest });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/trash/restore-to', async (req, res) => {
  try {
    const id = String(req.body?.id || '');
    const destinationDir = resolveRealPath(req.body?.destinationDir);
    const manifest = await readTrashManifest();
    const idx = manifest.findIndex((item) => item.id === id);
    if (idx < 0) {
      res.status(404).json({ error: 'Trash item not found.' });
      return;
    }
    const entry = manifest[idx];
    const dest = await uniqueDestinationPath(path.join(destinationDir, entry.name));
    await movePathSafely(entry.trashedPath, dest);
    manifest.splice(idx, 1);
    await writeTrashManifest(manifest);
    res.json({ destination: dest });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/trash/copy-to', async (req, res) => {
  try {
    const id = String(req.body?.id || '');
    const destinationDir = resolveRealPath(req.body?.destinationDir);
    const manifest = await readTrashManifest();
    const entry = manifest.find((item) => item.id === id);
    if (!entry) {
      res.status(404).json({ error: 'Trash item not found.' });
      return;
    }
    const dest = await uniqueDestinationPath(path.join(destinationDir, entry.name));
    await fsp.mkdir(path.dirname(dest), { recursive: true });
    await fsp.cp(entry.trashedPath, dest, { recursive: true });
    res.json({ destination: dest });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/trash/batch-transfer', async (req, res) => {
  const ids = Array.isArray(req.body?.ids) ? req.body.ids : [];
  const destinationDir = resolveRealPath(req.body?.destinationDir);
  const operation = req.body?.operation === 'copy' ? 'copy' : 'restore';
  const manifest = await readTrashManifest();
  let succeeded = 0;
  const destinations = [];
  const errors = [];

  for (const id of ids) {
    const idx = manifest.findIndex((item) => item.id === id);
    if (idx < 0) {
      errors.push({ id, error: 'Trash item not found' });
      continue;
    }
    const entry = manifest[idx];
    try {
      const dest = await uniqueDestinationPath(path.join(destinationDir, entry.name));
      if (operation === 'copy') {
        await fsp.mkdir(path.dirname(dest), { recursive: true });
        await fsp.cp(entry.trashedPath, dest, { recursive: true });
      } else {
        await movePathSafely(entry.trashedPath, dest);
        manifest.splice(idx, 1);
      }
      destinations.push(dest);
      succeeded++;
    } catch (err) {
      errors.push({ id, error: err instanceof Error ? err.message : String(err) });
    }
  }

  if (operation === 'restore' && succeeded > 0) {
    await writeTrashManifest(manifest);
  }
  res.json({
    operationId: `trash-transfer-${Date.now()}`,
    total: ids.length,
    succeeded,
    failed: errors.length,
    destinations,
    errors,
  });
});

app.delete('/api/trash/:id', async (req, res) => {
  try {
    const manifest = await readTrashManifest();
    const idx = manifest.findIndex((item) => item.id === req.params.id);
    if (idx >= 0) {
      const [removed] = manifest.splice(idx, 1);
      if (removed?.trashedPath) {
        await fsp.rm(removed.trashedPath, { recursive: true, force: true });
      }
      await writeTrashManifest(manifest);
    }
    res.json({ ok: true });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/trash/empty', async (_req, res) => {
  try {
    await fsp.rm(TRASH_FILES_DIR, { recursive: true, force: true });
    await writeTrashManifest([]);
    res.json({ ok: true });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

// ── Workspaces & Preferences ─────────────────────────────────────────────────
app.get('/api/workspaces', (_req, res) => {
  res.json({ workspaces, activeWorkspaceId });
});

app.post('/api/workspaces', async (req, res) => {
  const ws = req.body;
  if (ws && ws.id) {
    const idx = workspaces.findIndex((w) => w.id === ws.id);
    if (idx >= 0) workspaces[idx] = ws;
    else workspaces.push(ws);
    activeWorkspaceId = ws.id;
    await savePersistedState();
  }
  res.json({ ok: true });
});

app.delete('/api/workspaces/:id', async (req, res) => {
  workspaces = workspaces.filter((w) => w.id !== req.params.id);
  if (activeWorkspaceId === req.params.id) {
    activeWorkspaceId = workspaces[0]?.id || '';
  }
  await savePersistedState();
  res.json({ ok: true });
});

app.get('/api/preferences', (_req, res) => {
  res.json(preferences);
});

app.post('/api/preferences', async (req, res) => {
  preferences = { ...preferences, ...(req.body || {}) };
  await savePersistedState();
  res.json({ ok: true });
});

// ── System Surface Telemetry Providers (processes://, environment://, network://, services://)
function execCmd(command, args) {
  return new Promise((resolve) => {
    const child = spawn(command, args, { stdio: ['ignore', 'pipe', 'pipe'] });
    const chunks = [];
    child.stdout.on('data', (c) => chunks.push(c));
    child.on('error', () => resolve(''));
    child.on('close', () => resolve(Buffer.concat(chunks).toString('utf8')));
  });
}

function parseProcTcpAddr(hexAddrPort) {
  const [hexIp, hexPort] = String(hexAddrPort || '').split(':');
  if (!hexIp || hexIp.length !== 8 || !hexPort) return String(hexAddrPort || '');
  const octets = [
    parseInt(hexIp.slice(6, 8), 16),
    parseInt(hexIp.slice(4, 6), 16),
    parseInt(hexIp.slice(2, 4), 16),
    parseInt(hexIp.slice(0, 2), 16),
  ];
  return `${octets.join('.')}:${parseInt(hexPort, 16)}`;
}

const TCP_STATES = {
  '01': 'Established',
  '02': 'SynSent',
  '03': 'SynRecv',
  '04': 'FinWait1',
  '05': 'FinWait2',
  '06': 'TimeWait',
  '07': 'Close',
  '08': 'CloseWait',
  '09': 'LastAck',
  '0A': 'Listen',
  '0B': 'Closing',
};

app.post('/api/system-surface', async (req, res) => {
  const kind = String(req.body?.kind || '').toLowerCase();
  try {
    if (kind === 'environment') {
      const records = Object.entries(process.env)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([name, value]) => ({
          id: `process:${name}`,
          name,
          fields: {
            scope: 'Process',
            value: String(value ?? ''),
          },
        }));
      res.json(records);
      return;
    }

    if (kind === 'processes') {
      const raw = await execCmd('ps', ['-eo', 'pid=,pcpu=,rss=,comm=,args=']);
      const records = [];
      for (const line of raw.split('\n')) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        const m = /^(\d+)\s+([\d.]+)\s+(\d+)\s+(\S+)\s+(.*)$/.exec(trimmed);
        if (!m) continue;
        const pid = Number(m[1]);
        const cpu = `${m[2]}%`;
        const rssBytes = Number(m[3]) * 1024;
        const name = m[4];
        const cmdline = m[5];
        records.push({
          id: String(pid),
          name,
          fields: {
            pid,
            cpu,
            workingSet: rssBytes,
            path: cmdline,
          },
        });
      }
      res.json(records);
      return;
    }

    if (kind === 'network') {
      const records = [];
      const ifaces = os.networkInterfaces();
      for (const [iface, entries] of Object.entries(ifaces)) {
        for (let idx = 0; idx < (entries?.length || 0); idx++) {
          const info = entries[idx];
          records.push({
            id: `adapter:${iface}:${idx}`,
            name: `${iface} (${info.family})`,
            fields: {
              kind: 'Adapter',
              status: info.internal ? 'Loopback' : 'Up',
              local: `${info.address}/${info.netmask}`,
              remote: info.mac && info.mac !== '00:00:00:00:00:00' ? info.mac : info.cidr || '',
            },
          });
        }
      }
      for (const [proto, procPath] of [['TCP', '/proc/net/tcp'], ['UDP', '/proc/net/udp']]) {
        try {
          const raw = await fsp.readFile(procPath, 'utf8');
          const lines = raw.trim().split('\n').slice(1);
          for (const line of lines) {
            const cols = line.trim().split(/\s+/);
            if (cols.length < 4) continue;
            const slot = cols[0].replace(':', '');
            const local = parseProcTcpAddr(cols[1]);
            const remote = parseProcTcpAddr(cols[2]);
            const status = proto === 'UDP' ? 'Open' : (TCP_STATES[cols[3]] || 'Active');
            records.push({
              id: `${proto.toLowerCase()}:${slot}:${local}`,
              name: `${proto} ${local}`,
              fields: {
                kind: proto,
                status,
                local,
                remote,
              },
            });
          }
        } catch {}
      }
      res.json(records);
      return;
    }

    if (kind === 'services') {
      const records = [
        {
          id: 'service:reassembly-web-host',
          name: 'reassembly-web-host',
          fields: {
            status: 'Running',
            startType: 'Automatic',
            displayName: `Reassembly File Manager Web Host (PID ${process.pid})`,
          },
        },
      ];
      try {
        const initFiles = await fsp.readdir('/etc/init.d');
        for (const svc of initFiles.sort()) {
          if (svc.startsWith('.') || svc === 'README') continue;
          records.push({
            id: `service:${svc}`,
            name: svc,
            fields: {
              status: 'Installed',
              startType: 'System V',
              displayName: `/etc/init.d/${svc}`,
            },
          });
        }
      } catch {}
      res.json(records);
      return;
    }

    res.json([]);
  } catch (err) {
    res.status(500).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

// ── Declarative Directory Plugins Runtime & Storage ──────────────────────────
const PLUGIN_ID_REGEX = /^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$/;
const SEMVER_REGEX = /^(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-[0-9A-Za-z.-]+)?$/;
const SEEDED_PLUGINS = [
  {
    dirName: 'workspace-quick-tools',
    manifest: {
      schemaVersion: 1,
      id: 'reassembly.workspace-quick-tools',
      name: 'Workspace Quick Tools',
      version: '1.0.0',
      apiVersion: '^1.0.0',
      description: 'Adds quick workspace actions to the pane toolbar, context menu, and theme presets (Midnight Nord Slate).',
      author: 'Reassembly Extensions',
      permissions: ['commands', 'keybindings', 'toolbar', 'contextMenu', 'surfaces'],
      activationEvents: ['onStartup'],
      contributes: {
        commands: ['comparePanes', 'diskUsage', 'refreshPane'],
        surfaces: ['midnight-nord'],
      },
    },
    frontend: {
      schemaVersion: 1,
      commands: [
        {
          id: 'comparePanes',
          title: 'Plugin: Compare Dual Panes',
          name: 'comparePanes',
          targetCommandId: 'pane.compare.toggle',
        },
        {
          id: 'diskUsage',
          title: 'Plugin: Analyze Disk Usage',
          name: 'diskUsage',
          targetCommandId: 'pane.tools.diskUsage',
        },
        {
          id: 'refreshPane',
          title: 'Plugin: Quick Refresh',
          name: 'refreshPane',
          targetCommandId: 'pane.nav.refresh',
        },
      ],
      toolbar: [
        { id: 'compare-toolbar', commandId: 'comparePanes', when: 'always' },
        { id: 'disk-usage-toolbar', commandId: 'diskUsage', when: 'always' },
      ],
      contextMenu: [
        { id: 'compare-menu', commandId: 'comparePanes', when: 'selection' },
      ],
      themes: [
        {
          id: 'nordic-slate',
          label: 'Nordic Slate (Plugin)',
          kind: 'dark',
          variables: {},
        },
      ],
    },
  },
];

async function ensurePluginsInitialized() {
  const firstRun = !fs.existsSync(PLUGINS_ROOT);
  await fsp.mkdir(PLUGINS_ROOT, { recursive: true });
  if (firstRun) {
    for (const seed of SEEDED_PLUGINS) {
      const dir = path.join(PLUGINS_ROOT, seed.dirName);
      await fsp.mkdir(dir, { recursive: true });
      await fsp.writeFile(path.join(dir, 'plugin.json'), JSON.stringify(seed.manifest, null, 2), 'utf8');
      await fsp.writeFile(path.join(dir, 'frontend.json'), JSON.stringify(seed.frontend, null, 2), 'utf8');
    }
  }
}

async function readPluginStatesMap() {
  try {
    if (!fs.existsSync(PLUGIN_STATE_FILE)) return {};
    const parsed = JSON.parse(await fsp.readFile(PLUGIN_STATE_FILE, 'utf8'));
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

async function writePluginStatesMap(map) {
  await fsp.writeFile(PLUGIN_STATE_FILE, JSON.stringify(map, null, 2), 'utf8');
}

function defaultPluginState(pluginId, version) {
  return {
    pluginId,
    enabled: false,
    trusted: false,
    trustedVersion: undefined,
    grantedPermissions: [],
    quarantined: false,
    quarantineReason: undefined,
    lastActivationStatus: 'never',
    lastActivationError: undefined,
    lastActivatedAt: undefined,
    consecutiveActivationFailures: 0,
    lastSeenVersion: version,
    updatedAt: Date.now(),
  };
}

function validateDiscoveredPlugin(manifest, frontendText) {
  const errors = [];
  const warnings = [];
  if (!manifest || typeof manifest !== 'object') {
    errors.push('plugin.json must be a JSON object.');
    return { valid: false, errors, warnings };
  }
  if (manifest.schemaVersion !== 1 && manifest.schemaVersion !== 2) {
    errors.push(`Unsupported plugin manifest schema ${manifest.schemaVersion}.`);
  }
  if (!manifest.id || !PLUGIN_ID_REGEX.test(String(manifest.id))) {
    errors.push('Plugin id must be a lowercase identifier containing letters, digits, dot, dash, or underscore.');
  }
  if (!String(manifest.name || '').trim()) {
    errors.push('Plugin name is required.');
  }
  if (!manifest.version || !SEMVER_REGEX.test(String(manifest.version))) {
    errors.push(`Plugin ${manifest.id || '<unknown>'} has an invalid semantic version.`);
  }
  if (manifest.schemaVersion === 1) {
    try {
      const mod = JSON.parse(frontendText);
      if (mod?.schemaVersion !== 1) {
        errors.push(`frontend.json must declare schemaVersion: 1.`);
      }
    } catch (err) {
      errors.push(`Invalid frontend.json: ${err instanceof Error ? err.message : String(err)}`);
    }
  }
  return { valid: errors.length === 0, errors, warnings };
}

async function discoverPluginsFromDisk() {
  await ensurePluginsInitialized();
  const dirents = await fsp.readdir(PLUGINS_ROOT, { withFileTypes: true });
  const results = [];
  for (const d of dirents) {
    if (!d.isDirectory()) continue;
    const pkgDir = path.join(PLUGINS_ROOT, d.name);
    const manifestPath = path.join(pkgDir, 'plugin.json');
    if (!fs.existsSync(manifestPath)) continue;
    const manifestText = await fsp.readFile(manifestPath, 'utf8').catch(() => '');
    const frontendPath = path.join(pkgDir, 'frontend.json');
    const frontendText = fs.existsSync(frontendPath)
      ? await fsp.readFile(frontendPath, 'utf8').catch(() => '')
      : '';
    let manifest = {};
    let parseErr = '';
    try {
      manifest = JSON.parse(manifestText);
    } catch (err) {
      parseErr = err instanceof Error ? err.message : String(err);
    }
    const validation = parseErr
      ? { valid: false, errors: [`Invalid plugin.json: ${parseErr}`], warnings: [] }
      : validateDiscoveredPlugin(manifest, frontendText);
    const fingerprint = crypto
      .createHash('sha256')
      .update(manifestText)
      .update('\n---\n')
      .update(frontendText)
      .digest('hex')
      .slice(0, 16);

    results.push({
      manifest,
      kind: 'directory',
      path: pkgDir,
      manifestPath,
      fingerprint,
      valid: validation.valid,
      errors: validation.errors,
      warnings: validation.warnings,
    });
  }
  return results.sort((a, b) => String(a.manifest?.name || a.path).localeCompare(String(b.manifest?.name || b.path)));
}

app.get('/api/plugins/directory', async (_req, res) => {
  await ensurePluginsInitialized();
  res.json({ directory: PLUGINS_ROOT });
});

app.get('/api/plugins', async (_req, res) => {
  try {
    const discovered = await discoverPluginsFromDisk();
    res.json(discovered);
  } catch (err) {
    res.status(500).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.get('/api/plugins/states', async (_req, res) => {
  const discovered = await discoverPluginsFromDisk();
  const map = await readPluginStatesMap();
  const list = discovered.map((pkg) => {
    const id = String(pkg.manifest?.id || '');
    return map[id] || defaultPluginState(id, pkg.manifest?.version);
  });
  res.json(list);
});

app.get('/api/plugins/:id/state', async (req, res) => {
  const id = req.params.id;
  const map = await readPluginStatesMap();
  res.json(map[id] || defaultPluginState(id));
});

app.patch('/api/plugins/:id/state', async (req, res) => {
  try {
    const id = req.params.id;
    const discovered = await discoverPluginsFromDisk();
    const pkg = discovered.find((p) => p.manifest?.id === id);
    const map = await readPluginStatesMap();
    const current = map[id] || defaultPluginState(id, pkg?.manifest?.version);
    const patch = req.body || {};

    const next = {
      ...current,
      pluginId: id,
      lastSeenVersion: pkg?.manifest?.version || current.lastSeenVersion,
      updatedAt: Date.now(),
    };
    if (typeof patch.enabled === 'boolean') next.enabled = patch.enabled;
    if (typeof patch.trusted === 'boolean') {
      next.trusted = patch.trusted;
      next.trustedVersion = patch.trusted ? pkg?.manifest?.version : undefined;
    }
    if (Array.isArray(patch.grantedPermissions)) {
      next.grantedPermissions = [...new Set(patch.grantedPermissions.filter((p) => typeof p === 'string'))];
    }
    if (patch.clearQuarantine === true) {
      next.quarantined = false;
      next.quarantineReason = undefined;
      next.consecutiveActivationFailures = 0;
    }

    map[id] = next;
    await writePluginStatesMap(map);
    res.json(next);
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/plugins/:id/quarantine', async (req, res) => {
  const id = req.params.id;
  const map = await readPluginStatesMap();
  const current = map[id] || defaultPluginState(id);
  const next = {
    ...current,
    quarantined: true,
    quarantineReason: String(req.body?.reason || 'Quarantined due to activation error.'),
    enabled: false,
    updatedAt: Date.now(),
  };
  map[id] = next;
  await writePluginStatesMap(map);
  res.json(next);
});

app.post('/api/plugins/:id/activation', async (req, res) => {
  const id = req.params.id;
  const success = req.body?.success === true;
  const activationError = String(req.body?.activationError || '');
  const map = await readPluginStatesMap();
  const current = map[id] || defaultPluginState(id);
  const consecutiveFailures = success ? 0 : (current.consecutiveActivationFailures || 0) + 1;
  const quarantined = !success && consecutiveFailures >= 3;
  const next = {
    ...current,
    lastActivationStatus: success ? 'success' : 'failed',
    lastActivationError: success ? undefined : activationError,
    lastActivatedAt: Date.now(),
    consecutiveActivationFailures: consecutiveFailures,
    quarantined: current.quarantined || quarantined,
    quarantineReason: quarantined ? `Auto-quarantined after ${consecutiveFailures} consecutive activation failures.` : current.quarantineReason,
    updatedAt: Date.now(),
  };
  map[id] = next;
  await writePluginStatesMap(map);
  res.json(next);
});

app.get('/api/plugins/:id/frontend-module', async (req, res) => {
  try {
    const id = req.params.id;
    const discovered = await discoverPluginsFromDisk();
    const pkg = discovered.find((p) => p.manifest?.id === id);
    if (!pkg) {
      res.status(404).json({ error: `Plugin ${id} not found.` });
      return;
    }
    const frontendPath = path.join(pkg.path, 'frontend.json');
    if (!fs.existsSync(frontendPath)) {
      res.json({ schemaVersion: 1, commands: [], bindings: [], toolbar: [], contextMenu: [] });
      return;
    }
    const parsed = JSON.parse(await fsp.readFile(frontendPath, 'utf8'));
    res.json(parsed);
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.post('/api/plugins/create', async (req, res) => {
  try {
    await ensurePluginsInitialized();
    const rawId = String(req.body?.id || '').trim().toLowerCase();
    const name = String(req.body?.name || '').trim();
    const description = String(req.body?.description || '').trim();
    const commandsInput = Array.isArray(req.body?.commands) ? req.body.commands : [];
    if (!PLUGIN_ID_REGEX.test(rawId)) {
      res.status(400).json({ error: 'Invalid plugin ID. Use lowercase letters, numbers, dots, or hyphens (e.g. custom.quick-actions).' });
      return;
    }
    if (!name) {
      res.status(400).json({ error: 'Plugin name is required.' });
      return;
    }
    const safeDir = rawId.replace(/[^a-z0-9._-]/g, '-');
    const pkgDir = path.join(PLUGINS_ROOT, safeDir);
    if (fs.existsSync(pkgDir)) {
      res.status(409).json({ error: `A plugin directory already exists for "${rawId}".` });
      return;
    }

    const permissions = new Set(['commands']);
    const commands = [];
    const toolbar = [];
    const contextMenu = [];
    const bindings = [];

    for (let i = 0; i < commandsInput.length; i++) {
      const cmd = commandsInput[i];
      const cmdId = String(cmd?.id || `action-${i + 1}`).trim().replace(/[^a-z0-9._-]/gi, '-');
      const title = String(cmd?.title || cmdId).trim();
      const targetCommandId = String(cmd?.targetCommandId || 'pane.nav.refresh').trim();
      commands.push({ id: cmdId, title, name: cmdId, targetCommandId });
      if (cmd?.addToToolbar) {
        permissions.add('toolbar');
        toolbar.push({ id: `${cmdId}-toolbar`, commandId: cmdId, when: cmd?.when || 'always' });
      }
      if (cmd?.addToContextMenu) {
        permissions.add('contextMenu');
        contextMenu.push({ id: `${cmdId}-menu`, commandId: cmdId, when: cmd?.when || 'selection' });
      }
      if (cmd?.shortcutKey) {
        permissions.add('keybindings');
        bindings.push({
          id: `${cmdId}-key`,
          commandId: cmdId,
          trigger: {
            device: 'keyboard',
            chord: {
              steps: [{
                key: String(cmd.shortcutKey).toLowerCase(),
                modifiers: Array.isArray(cmd.shortcutModifiers) ? cmd.shortcutModifiers : ['ctrl', 'alt'],
              }],
            },
          },
        });
      }
    }

    const manifest = {
      schemaVersion: 1,
      id: rawId,
      name,
      version: '1.0.0',
      apiVersion: '^1.0.0',
      description: description || `Custom declarative extension: ${name}`,
      author: 'Workspace Extension Studio',
      permissions: [...permissions],
      activationEvents: ['onStartup'],
      contributes: {
        commands: commands.map((c) => c.id),
      },
    };
    const frontend = {
      schemaVersion: 1,
      commands,
      ...(bindings.length ? { bindings } : {}),
      ...(toolbar.length ? { toolbar } : {}),
      ...(contextMenu.length ? { contextMenu } : {}),
    };

    await fsp.mkdir(pkgDir, { recursive: true });
    await fsp.writeFile(path.join(pkgDir, 'plugin.json'), JSON.stringify(manifest, null, 2), 'utf8');
    await fsp.writeFile(path.join(pkgDir, 'frontend.json'), JSON.stringify(frontend, null, 2), 'utf8');

    const map = await readPluginStatesMap();
    map[rawId] = {
      ...defaultPluginState(rawId, manifest.version),
      enabled: req.body?.autoEnable === true,
      trusted: req.body?.autoTrust === true,
      trustedVersion: req.body?.autoTrust === true ? manifest.version : undefined,
      grantedPermissions: req.body?.autoTrust === true ? [...permissions] : [],
      updatedAt: Date.now(),
    };
    await writePluginStatesMap(map);

    res.status(201).json({ ok: true, id: rawId, path: pkgDir });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

app.delete('/api/plugins/:id', async (req, res) => {
  try {
    const id = req.params.id;
    const discovered = await discoverPluginsFromDisk();
    const pkg = discovered.find((p) => p.manifest?.id === id);
    if (pkg && pkg.path.startsWith(PLUGINS_ROOT + path.sep)) {
      await fsp.rm(pkg.path, { recursive: true, force: true });
    }
    const map = await readPluginStatesMap();
    if (map[id]) {
      delete map[id];
      await writePluginStatesMap(map);
    }
    res.json({ ok: true });
  } catch (err) {
    res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
  }
});

if (fs.existsSync(PUBLIC_DIR)) {
  app.use(express.static(PUBLIC_DIR));
}
app.use(express.static(DIST_DIR));
app.get('/{*path}', (_req, res) => {
  res.sendFile(path.join(DIST_DIR, 'index.html'));
});

app.listen(PORT, HOST, () => {
  console.log(`Reassembly File Manager web server listening on http://${HOST}:${PORT}`);
});
