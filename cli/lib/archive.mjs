import { execFile } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import {
  CliRuntimeError,
  CliUsageError,
  formatBytes,
  formatDate,
  printJson,
  renderTable,
} from './output.mjs';

const PY_ARCHIVE_SCRIPT = `
import bz2, gzip, json, lzma, os, posixpath, sys, tarfile, zipfile
from datetime import datetime, timezone

SUFFIXES = ['.tar.gz', '.tgz', '.tar.bz2', '.tbz2', '.tar.xz', '.txz', '.zip', '.tar', '.gz', '.bz2', '.xz']

def norm_inner(p):
    parts = []
    for seg in (p or '').replace('\\\\', '/').split('/'):
        if not seg or seg == '.':
            continue
        if seg == '..':
            if parts:
                parts.pop()
            continue
        parts.append(seg)
    return '/'.join(parts)

def safe_join(root_dir, rel_path):
    real_root = os.path.realpath(root_dir)
    clean_rel = norm_inner(rel_path)
    if not clean_rel:
        return real_root
    candidate = os.path.realpath(os.path.join(real_root, clean_rel))
    if candidate != real_root and not candidate.startswith(real_root + os.sep):
        raise ValueError(f'Unsafe archive path rejected: {rel_path}')
    return candidate

def detect_kind(p):
    low = p.lower()
    if low.endswith('.zip'): return 'zip'
    if low.endswith(('.tar.gz', '.tgz')): return 'targz'
    if low.endswith(('.tar.bz2', '.tbz2')): return 'tarbz2'
    if low.endswith(('.tar.xz', '.txz')): return 'tarxz'
    if low.endswith('.tar'): return 'tar'
    if low.endswith('.gz'): return 'gz'
    if low.endswith('.bz2'): return 'bz2'
    if low.endswith('.xz'): return 'xz'
    raise ValueError(f'Unsupported archive format: {p}')

def archive_stem(p):
    base = os.path.basename(p)
    low = base.lower()
    for s in SUFFIXES:
        if low.endswith(s):
            return base[:-len(s)] or base
    return os.path.splitext(base)[0] or base

def list_entries(archive_path, inner):
    kind = detect_kind(archive_path)
    items = []
    if kind == 'zip':
        with zipfile.ZipFile(archive_path, 'r') as zf:
            for zi in zf.infolist():
                p = norm_inner(zi.filename)
                if not p: continue
                is_dir = zi.is_dir() or zi.filename.endswith('/')
                try:
                    dt = datetime(*zi.date_time, tzinfo=timezone.utc).isoformat()
                except Exception:
                    dt = None
                items.append({'path': p, 'isDir': is_dir, 'size': 0 if is_dir else zi.file_size, 'mtime': dt})
    elif kind.startswith('tar'):
        mode = {'tar': 'r:', 'targz': 'r:gz', 'tarbz2': 'r:bz2', 'tarxz': 'r:xz'}[kind]
        with tarfile.open(archive_path, mode) as tf:
            for ti in tf.getmembers():
                p = norm_inner(ti.name)
                if not p: continue
                is_dir = ti.isdir()
                dt = datetime.fromtimestamp(ti.mtime, tz=timezone.utc).isoformat() if ti.mtime else None
                items.append({'path': p, 'isDir': is_dir, 'size': 0 if is_dir else ti.size, 'mtime': dt})
    else:
        raw_open = {'gz': gzip.open, 'bz2': bz2.open, 'xz': lzma.open}[kind]
        with raw_open(archive_path, 'rb') as f:
            raw = f.read()
        items.append({'path': archive_stem(archive_path), 'isDir': False, 'size': len(raw), 'mtime': None})

    prefix = norm_inner(inner)
    if prefix:
        filtered = [x for x in items if x['path'] == prefix or x['path'].startswith(prefix + '/')]
    else:
        filtered = items
    return filtered

def create_zip(dest, sources):
    dest = os.path.abspath(dest)
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    added = 0
    with zipfile.ZipFile(dest, 'w', compression=zipfile.ZIP_DEFLATED) as zf:
        for src in sources:
            src = os.path.abspath(src)
            if not os.path.exists(src):
                raise FileNotFoundError(f'Source does not exist: {src}')
            parent = os.path.dirname(src)
            if os.path.isdir(src):
                zf.write(src, os.path.relpath(src, parent).replace(os.sep, '/'))
                for root, dirs, files in os.walk(src):
                    for d in dirs:
                        full = os.path.join(root, d)
                        zf.write(full, os.path.relpath(full, parent).replace(os.sep, '/'))
                    for fn in files:
                        full = os.path.join(root, fn)
                        zf.write(full, os.path.relpath(full, parent).replace(os.sep, '/'))
                        added += 1
            else:
                zf.write(src, os.path.basename(src))
                added += 1
    return {'destination': dest, 'filesAdded': added, 'bytes': os.path.getsize(dest)}

def extract_archive(archive_path, dest_dir, create_sub=False, flatten=False):
    target_dir = os.path.join(dest_dir, archive_stem(archive_path)) if create_sub else dest_dir
    os.makedirs(target_dir, exist_ok=True)
    kind = detect_kind(archive_path)
    files_written = 0
    bytes_written = 0

    def write_member(rel, raw):
        nonlocal files_written, bytes_written
        if flatten:
            rel = posixpath.basename(rel)
        if not rel:
            return
        t = safe_join(target_dir, rel)
        os.makedirs(os.path.dirname(t), exist_ok=True)
        with open(t, 'wb') as out:
            out.write(raw)
        files_written += 1
        bytes_written += len(raw)

    if kind == 'zip':
        with zipfile.ZipFile(archive_path, 'r') as zf:
            for zi in zf.infolist():
                rel = norm_inner(zi.filename)
                if not rel: continue
                if zi.is_dir() or zi.filename.endswith('/'):
                    if not flatten:
                        os.makedirs(safe_join(target_dir, rel), exist_ok=True)
                else:
                    write_member(rel, zf.read(zi))
    elif kind.startswith('tar'):
        mode = {'tar': 'r:', 'targz': 'r:gz', 'tarbz2': 'r:bz2', 'tarxz': 'r:xz'}[kind]
        with tarfile.open(archive_path, mode) as tf:
            for ti in tf.getmembers():
                rel = norm_inner(ti.name)
                if not rel: continue
                if ti.isdir():
                    if not flatten:
                        os.makedirs(safe_join(target_dir, rel), exist_ok=True)
                elif ti.isfile():
                    f = tf.extractfile(ti)
                    if f is not None:
                        write_member(rel, f.read())
    else:
        raw_open = {'gz': gzip.open, 'bz2': bz2.open, 'xz': lzma.open}[kind]
        with raw_open(archive_path, 'rb') as f:
            write_member(archive_stem(archive_path), f.read())

    return {'destination': target_dir, 'filesExtracted': files_written, 'bytesExtracted': bytes_written}

req = json.loads(sys.stdin.read())
op = req['op']
if op == 'list':
    res = list_entries(req['archive'], req.get('inner', ''))
elif op == 'zip':
    res = create_zip(req['dest'], req['sources'])
elif op == 'extract':
    res = extract_archive(req['archive'], req['dest'], req.get('sub', False), req.get('flatten', False))
else:
    raise ValueError(f'Unknown op: {op}')
sys.stdout.write(json.dumps({'ok': True, 'data': res}))
`;

function invokeArchiveWorker(payload) {
  return new Promise((resolve, reject) => {
    const child = execFile(process.platform === 'win32' ? 'python' : 'python3', ['-c', PY_ARCHIVE_SCRIPT], { maxBuffer: 8 * 1024 * 1024 }, (err, stdout, stderr) => {
      if (err) {
        reject(new CliRuntimeError(stderr?.trim() || err.message));
        return;
      }
      try {
        const parsed = JSON.parse(stdout);
        resolve(parsed.data);
      } catch (parseErr) {
        reject(new CliRuntimeError(`Archive worker returned invalid JSON: ${parseErr.message}`));
      }
    });
    child.stdin.write(JSON.stringify(payload));
    child.stdin.end();
  });
}

export async function runArchiveCommand(args, ctx) {
  const sub = args[0];
  const rest = args.slice(1);
  if (!sub || !['list', 'zip', 'extract'].includes(sub)) {
    throw new CliUsageError(
      'Usage: aether archive <list|zip|extract> ...\n' +
        '  list <archive> [--inner <path>]\n' +
        '  zip <dest.zip> <sources...>\n' +
        '  extract <archive> [destDir] [--sub] [--flatten]',
    );
  }

  if (sub === 'list') {
    let inner = '';
    const positional = [];
    for (let i = 0; i < rest.length; i++) {
      if (rest[i] === '--inner') inner = rest[++i] ?? '';
      else if (rest[i].startsWith('-')) throw new CliUsageError(`Unknown option for archive list: ${rest[i]}`);
      else positional.push(rest[i]);
    }
    if (positional.length !== 1) throw new CliUsageError('Usage: aether archive list <archive> [--inner <path>]');
    const archivePath = path.resolve(ctx.cwd, positional[0]);
    if (!fs.existsSync(archivePath)) throw new CliRuntimeError(`Archive not found: ${archivePath}`);

    const entries = await invokeArchiveWorker({ op: 'list', archive: archivePath, inner });
    if (ctx.json) {
      printJson({ archive: archivePath, count: entries.length, entries });
      return 0;
    }
    if (entries.length === 0) {
      console.log(ctx.style.gray('(empty archive)'));
      return 0;
    }
    const rows = entries.map((e) => [
      e.isDir ? ctx.style.cyan('DIR') : 'FIL',
      e.isDir ? ctx.style.gray('-') : formatBytes(e.size),
      ctx.style.gray(formatDate(e.mtime)),
      e.isDir ? ctx.style.cyan(`${e.path}/`) : e.path,
    ]);
    console.log(renderTable(['TYPE', 'SIZE', 'MODIFIED', 'PATH'], rows, { alignRight: [false, true, false, false] }));
    return 0;
  }

  if (sub === 'zip') {
    if (rest.length < 2) throw new CliUsageError('Usage: aether archive zip <dest.zip> <sources...>');
    const dest = path.resolve(ctx.cwd, rest[0]);
    const sources = rest.slice(1).map((s) => path.resolve(ctx.cwd, s));
    const data = await invokeArchiveWorker({ op: 'zip', dest, sources });
    if (ctx.json) {
      printJson(data);
      return 0;
    }
    console.log(ctx.style.green(`Created ${data.destination} (${data.filesAdded} files, ${formatBytes(data.bytes)})`));
    return 0;
  }

  // extract
  let subDir = false;
  let flatten = false;
  const positional = [];
  for (const a of rest) {
    if (a === '--sub') subDir = true;
    else if (a === '--flatten') flatten = true;
    else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for archive extract: ${a}`);
    else positional.push(a);
  }
  if (positional.length < 1 || positional.length > 2) {
    throw new CliUsageError('Usage: aether archive extract <archive> [destDir] [--sub] [--flatten]');
  }
  const archivePath = path.resolve(ctx.cwd, positional[0]);
  if (!fs.existsSync(archivePath)) throw new CliRuntimeError(`Archive not found: ${archivePath}`);
  const destDir = path.resolve(ctx.cwd, positional[1] || '.');

  const data = await invokeArchiveWorker({ op: 'extract', archive: archivePath, dest: destDir, sub: subDir, flatten });
  if (ctx.json) {
    printJson(data);
    return 0;
  }
  console.log(
    ctx.style.green(
      `Extracted ${data.filesExtracted} file(s) (${formatBytes(data.bytesExtracted)}) to ${data.destination}`,
    ),
  );
  return 0;
}
