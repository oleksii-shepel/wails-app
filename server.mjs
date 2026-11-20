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
const PORT = Number(process.env.PORT || 3000);
const HOST = '0.0.0.0';

if (!fs.existsSync(path.join(DIST_DIR, 'index.html'))) {
  console.log('[server] Frontend dist not found, building manager/frontend...');
  execSync('npm --prefix manager/frontend run build', {
    cwd: WORKSPACE_ROOT,
    stdio: 'inherit',
  });
}

const app = express();
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

def main():
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

app.get('/api/drives', (_req, res) => {
  res.json({
    schemes: [
      {
        name: 'Workspace',
        path: WORKSPACE_ROOT,
        schemeType: 'fixed',
        totalSpace: 50 * 1024 * 1024 * 1024,
        freeSpace: 32 * 1024 * 1024 * 1024,
        fileSystem: 'ext4',
      },
      {
        name: 'Root (/)',
        path: '/',
        schemeType: 'fixed',
        totalSpace: 50 * 1024 * 1024 * 1024,
        freeSpace: 32 * 1024 * 1024 * 1024,
        fileSystem: 'ext4',
      },
      {
        name: 'Temp (/tmp)',
        path: '/tmp',
        schemeType: 'fixed',
        totalSpace: 50 * 1024 * 1024 * 1024,
        freeSpace: 32 * 1024 * 1024 * 1024,
        fileSystem: 'tmpfs',
      },
    ],
  });
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
    const hash = crypto.createHash('sha256');
    await new Promise((resolve, reject) => {
      const stream = fs.createReadStream(targetPath);
      stream.on('data', (chunk) => hash.update(chunk));
      stream.on('end', resolve);
      stream.on('error', reject);
    });
    res.json({ hash: hash.digest('hex') });
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
