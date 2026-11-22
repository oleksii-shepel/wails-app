import crypto from 'node:crypto';
import fs from 'node:fs';
import fsp from 'node:fs/promises';
import path from 'node:path';
import {
  CliRuntimeError,
  CliUsageError,
  formatBytes,
  formatDate,
  parseSize,
  printJson,
  renderBar,
  renderTable,
} from './output.mjs';

const SKIP_DIRS = new Set(['.git', 'node_modules', '.angular', 'dist', 'wailsjs']);

function globToRegex(pattern, caseSensitive = false) {
  const escaped = pattern
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*/g, '.*')
    .replace(/\?/g, '.');
  return new RegExp(`^${escaped}$`, caseSensitive ? '' : 'i');
}

async function computeFileHashes(filePath, algos = ['sha256']) {
  const results = {};
  const hashes = algos.map((algo) => ({ algo, hash: crypto.createHash(algo) }));
  await new Promise((resolve, reject) => {
    const stream = fs.createReadStream(filePath);
    stream.on('data', (chunk) => {
      for (const item of hashes) item.hash.update(chunk);
    });
    stream.on('error', reject);
    stream.on('end', resolve);
  });
  for (const item of hashes) {
    results[item.algo] = item.hash.digest('hex');
  }
  return results;
}

export async function runLsCommand(args, ctx) {
  let showHidden = false;
  let longFormat = false;
  let sortField = 'name';
  let dirsFirst = true;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '-a' || a === '--all') showHidden = true;
    else if (a === '-l' || a === '--long') longFormat = true;
    else if (a === '-s' || a === '--sort') {
      const next = args[++i];
      if (!next || !['name', 'size', 'mtime'].includes(next)) {
        throw new CliUsageError('--sort must be one of: name, size, mtime.');
      }
      sortField = next;
    } else if (a === '--dirs-first') dirsFirst = true;
    else if (a === '--no-dirs-first') dirsFirst = false;
    else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for ls: ${a}`);
    else positional.push(a);
  }

  if (positional.length > 1) {
    throw new CliUsageError('Usage: aether ls [path] [-a] [-l] [-s name|size|mtime] [--no-dirs-first]');
  }

  const targetDir = path.resolve(ctx.cwd, positional[0] || '.');
  let entries;
  try {
    entries = await fsp.readdir(targetDir, { withFileTypes: true });
  } catch (err) {
    throw new CliRuntimeError(`Cannot list "${targetDir}": ${err.message}`);
  }

  const items = [];
  for (const entry of entries) {
    if (!showHidden && entry.name.startsWith('.')) continue;
    const full = path.join(targetDir, entry.name);
    let stat = null;
    try {
      stat = await fsp.lstat(full);
    } catch {
      // Ignore unreadable entries
    }
    const isDir = entry.isDirectory();
    const isSymlink = entry.isSymbolicLink();
    items.push({
      name: entry.name,
      path: full,
      type: isDir ? 'dir' : isSymlink ? 'symlink' : 'file',
      size: isDir ? 0 : (stat?.size ?? 0),
      mtime: stat ? new Date(stat.mtimeMs).toISOString() : null,
      mtimeMs: stat?.mtimeMs ?? 0,
      mode: stat ? (stat.mode & 0o777).toString(8).padStart(3, '0') : '---',
    });
  }

  items.sort((a, b) => {
    if (dirsFirst && a.type !== b.type) {
      if (a.type === 'dir') return -1;
      if (b.type === 'dir') return 1;
    }
    if (sortField === 'size') return b.size - a.size || a.name.localeCompare(b.name);
    if (sortField === 'mtime') return b.mtimeMs - a.mtimeMs || a.name.localeCompare(b.name);
    return a.name.localeCompare(b.name);
  });

  if (ctx.json) {
    printJson({ path: targetDir, count: items.length, entries: items });
    return 0;
  }

  if (items.length === 0) {
    console.log(ctx.style.gray('(empty directory)'));
    return 0;
  }

  if (!longFormat) {
    for (const item of items) {
      const label =
        item.type === 'dir'
          ? ctx.style.cyan(`${item.name}/`)
          : item.type === 'symlink'
            ? ctx.style.magenta(`${item.name}@`)
            : item.name;
      console.log(label);
    }
    return 0;
  }

  const rows = items.map((item) => [
    ctx.style.gray(item.mode),
    item.type === 'dir' ? ctx.style.cyan('DIR') : item.type === 'symlink' ? ctx.style.magenta('LNK') : 'FIL',
    item.type === 'dir' ? ctx.style.gray('-') : formatBytes(item.size),
    ctx.style.gray(formatDate(item.mtime)),
    item.type === 'dir' ? ctx.style.cyan(`${item.name}/`) : item.name,
  ]);
  console.log(
    renderTable(['MODE', 'TYPE', 'SIZE', 'MODIFIED', 'NAME'], rows, {
      alignRight: [false, false, true, false, false],
    }),
  );
  return 0;
}

export async function runDuCommand(args, ctx) {
  let maxDepth = 2;
  let top = 25;
  let showHidden = false;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '-a' || a === '--all') showHidden = true;
    else if (a === '-d' || a === '--depth') {
      const next = args[++i];
      if (!next || !/^\d+$/.test(next)) throw new CliUsageError('--depth requires a non-negative integer.');
      maxDepth = Number(next);
    } else if (a === '--top') {
      const next = args[++i];
      if (!next || !/^\d+$/.test(next) || Number(next) < 1) throw new CliUsageError('--top requires a positive integer.');
      top = Number(next);
    } else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for du: ${a}`);
    else positional.push(a);
  }

  if (positional.length > 1) {
    throw new CliUsageError('Usage: aether du [path] [--depth N] [--top N] [-a|--all]');
  }

  const rootDir = path.resolve(ctx.cwd, positional[0] || '.');
  let rootStat;
  try {
    rootStat = await fsp.lstat(rootDir);
  } catch (err) {
    throw new CliRuntimeError(`Cannot inspect "${rootDir}": ${err.message}`);
  }
  if (!rootStat.isDirectory()) {
    const payload = { root: rootDir, totalBytes: rootStat.size, fileCount: 1, dirCount: 0, entries: [] };
    if (ctx.json) printJson(payload);
    else console.log(`${formatBytes(rootStat.size)}  ${rootDir}`);
    return 0;
  }

  const topLevelBuckets = new Map();
  let totalBytes = 0;
  let fileCount = 0;
  let dirCount = 0;

  async function walk(dirPath, depth) {
    let entries;
    try {
      entries = await fsp.readdir(dirPath, { withFileTypes: true });
    } catch {
      return 0;
    }
    let subTotal = 0;
    for (const entry of entries) {
      if (!showHidden && entry.name.startsWith('.')) continue;
      const full = path.join(dirPath, entry.name);
      if (entry.isSymbolicLink()) continue;
      if (entry.isDirectory()) {
        dirCount++;
        const bytes = await walk(full, depth + 1);
        subTotal += bytes;
        if (depth < maxDepth) {
          topLevelBuckets.set(full, {
            path: full,
            relPath: path.relative(rootDir, full) || '.',
            type: 'dir',
            size: bytes,
            depth: depth + 1,
          });
        }
      } else if (entry.isFile()) {
        let size = 0;
        try {
          size = (await fsp.stat(full)).size;
        } catch {}
        fileCount++;
        subTotal += size;
        if (depth < maxDepth) {
          topLevelBuckets.set(full, {
            path: full,
            relPath: path.relative(rootDir, full) || entry.name,
            type: 'file',
            size,
            depth: depth + 1,
          });
        }
      }
    }
    return subTotal;
  }

  totalBytes = await walk(rootDir, 0);

  const ranked = [...topLevelBuckets.values()]
    .sort((a, b) => b.size - a.size || a.relPath.localeCompare(b.relPath))
    .slice(0, top);

  if (ctx.json) {
    printJson({
      root: rootDir,
      totalBytes,
      fileCount,
      dirCount,
      entries: ranked.map((e) => ({
        ...e,
        percent: totalBytes > 0 ? Number(((e.size / totalBytes) * 100).toFixed(2)) : 0,
      })),
    });
    return 0;
  }

  console.log(
    `${ctx.style.bold(rootDir)}  ${ctx.style.cyan(formatBytes(totalBytes))} ` +
      ctx.style.gray(`(${fileCount} files, ${dirCount} dirs)`),
  );
  if (ranked.length === 0) return 0;

  const rows = ranked.map((entry) => {
    const ratio = totalBytes > 0 ? entry.size / totalBytes : 0;
    const pct = `${(ratio * 100).toFixed(1)}%`;
    return [
      ctx.style.cyan(formatBytes(entry.size)),
      pct,
      ctx.style.blue(renderBar(ratio, 14)),
      entry.type === 'dir' ? ctx.style.bold(`${entry.relPath}/`) : entry.relPath,
    ];
  });
  console.log(renderTable(['SIZE', 'SHARE', 'BAR', 'PATH'], rows, { alignRight: [true, true, false, false] }));
  return 0;
}

export async function runFindCommand(args, ctx) {
  let pattern = '';
  let grep = '';
  let caseSensitive = false;
  let recursive = true;
  let maxDepth = Infinity;
  let showHidden = false;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '-p' || a === '--pattern') pattern = args[++i] ?? '';
    else if (a === '-g' || a === '--grep') grep = args[++i] ?? '';
    else if (a === '-c' || a === '--case-sensitive') caseSensitive = true;
    else if (a === '--no-recursive') recursive = false;
    else if (a === '-a' || a === '--all') showHidden = true;
    else if (a === '-d' || a === '--max-depth') {
      const next = args[++i];
      if (!next || !/^\d+$/.test(next)) throw new CliUsageError('--max-depth requires a non-negative integer.');
      maxDepth = Number(next);
    } else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for find: ${a}`);
    else positional.push(a);
  }

  if (positional.length > 1) {
    throw new CliUsageError('Usage: aether find [path] [-p glob] [-g text] [-c] [-d depth] [--no-recursive] [-a]');
  }

  const rootDir = path.resolve(ctx.cwd, positional[0] || '.');
  const nameRegex = pattern ? globToRegex(pattern, caseSensitive) : null;
  const grepNeedle = caseSensitive ? grep : grep.toLowerCase();
  const MAX_CONTENT_BYTES = 2 * 1024 * 1024;
  const matches = [];

  async function inspectContent(filePath) {
    const stat = await fsp.stat(filePath);
    if (!stat.isFile() || stat.size === 0 || stat.size > MAX_CONTENT_BYTES) return null;
    const buf = await fsp.readFile(filePath);
    const probe = Math.min(buf.length, 4096);
    for (let i = 0; i < probe; i++) {
      if (buf[i] === 0) return null;
    }
    const text = buf.toString('utf8');
    const lines = text.split(/\r?\n/);
    const hits = [];
    for (let idx = 0; idx < lines.length; idx++) {
      const hay = caseSensitive ? lines[idx] : lines[idx].toLowerCase();
      if (hay.includes(grepNeedle)) {
        hits.push({ line: idx + 1, snippet: lines[idx].trim().slice(0, 140) });
      }
    }
    return hits.length > 0 ? { count: hits.length, first: hits[0], hits: hits.slice(0, 5) } : null;
  }

  async function walk(dirPath, depth) {
    if (matches.length >= 500) return;
    let entries;
    try {
      entries = await fsp.readdir(dirPath, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      if (matches.length >= 500) break;
      if (!showHidden && entry.name.startsWith('.')) continue;
      if (entry.isDirectory() && SKIP_DIRS.has(entry.name)) continue;

      const full = path.join(dirPath, entry.name);
      const nameMatched = !nameRegex || nameRegex.test(entry.name);

      if (entry.isDirectory()) {
        if (!grep && nameMatched) {
          matches.push({ path: full, relPath: path.relative(rootDir, full), type: 'dir', size: 0 });
        }
        if (recursive && depth + 1 <= maxDepth) {
          await walk(full, depth + 1);
        }
      } else if (entry.isFile()) {
        if (!nameMatched) continue;
        let size = 0;
        try {
          size = (await fsp.stat(full)).size;
        } catch {}
        if (grep) {
          const contentMatch = await inspectContent(full);
          if (!contentMatch) continue;
          matches.push({
            path: full,
            relPath: path.relative(rootDir, full),
            type: 'file',
            size,
            contentMatches: contentMatch.count,
            line: contentMatch.first.line,
            snippet: contentMatch.first.snippet,
          });
        } else {
          matches.push({ path: full, relPath: path.relative(rootDir, full), type: 'file', size });
        }
      }
    }
  }

  await walk(rootDir, 0);

  if (ctx.json) {
    printJson({ root: rootDir, pattern: pattern || null, grep: grep || null, count: matches.length, matches });
    return 0;
  }

  if (matches.length === 0) {
    console.log(ctx.style.gray('No matches found.'));
    return 0;
  }

  for (const m of matches) {
    if (grep && m.line) {
      console.log(
        `${ctx.style.cyan(m.relPath)}:${ctx.style.yellow(`L${m.line}`)} ` +
          ctx.style.gray(`(${m.contentMatches}x) `) +
          m.snippet,
      );
    } else if (m.type === 'dir') {
      console.log(ctx.style.cyan(`${m.relPath}/`));
    } else {
      console.log(`${m.relPath} ${ctx.style.gray(`(${formatBytes(m.size)})`)}`);
    }
  }
  return 0;
}

export async function runDupCommand(args, ctx) {
  let minSize = 1;
  let showHidden = false;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '--min-size') minSize = Math.max(0, parseSize(args[++i], 1));
    else if (a === '-a' || a === '--all') showHidden = true;
    else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for dup: ${a}`);
    else positional.push(a);
  }

  if (positional.length > 1) {
    throw new CliUsageError('Usage: aether dup [path] [--min-size 1K] [-a|--all]');
  }

  const rootDir = path.resolve(ctx.cwd, positional[0] || '.');
  const sizeBuckets = new Map();
  let scannedFiles = 0;

  async function walk(dirPath) {
    let entries;
    try {
      entries = await fsp.readdir(dirPath, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      if (!showHidden && entry.name.startsWith('.')) continue;
      if (entry.isSymbolicLink()) continue;
      const full = path.join(dirPath, entry.name);
      if (entry.isDirectory()) {
        if (SKIP_DIRS.has(entry.name)) continue;
        await walk(full);
      } else if (entry.isFile()) {
        let stat;
        try {
          stat = await fsp.stat(full);
        } catch {
          continue;
        }
        scannedFiles++;
        if (stat.size < minSize) continue;
        const list = sizeBuckets.get(stat.size) || [];
        list.push({ path: full, relPath: path.relative(rootDir, full), size: stat.size });
        sizeBuckets.set(stat.size, list);
      }
    }
  }

  await walk(rootDir);

  const groups = [];
  let reclaimableBytes = 0;

  for (const [size, candidates] of sizeBuckets.entries()) {
    if (candidates.length < 2) continue;
    const hashMap = new Map();
    for (const item of candidates) {
      try {
        const { sha256 } = await computeFileHashes(item.path, ['sha256']);
        const bucket = hashMap.get(sha256) || [];
        bucket.push(item);
        hashMap.set(sha256, bucket);
      } catch {
        // Skip unreadable
      }
    }
    for (const [sha256, files] of hashMap.entries()) {
      if (files.length < 2) continue;
      const waste = size * (files.length - 1);
      reclaimableBytes += waste;
      groups.push({ sha256, size, reclaimableBytes: waste, files });
    }
  }

  groups.sort((a, b) => b.reclaimableBytes - a.reclaimableBytes || b.size - a.size);

  if (ctx.json) {
    printJson({ root: rootDir, scannedFiles, groupCount: groups.length, reclaimableBytes, groups });
    return 0;
  }

  if (groups.length === 0) {
    console.log(ctx.style.green(`No duplicates found across ${scannedFiles} files.`));
    return 0;
  }

  console.log(
    `${ctx.style.bold(`${groups.length} duplicate group(s)`)} across ${scannedFiles} files — ` +
      `${ctx.style.yellow(formatBytes(reclaimableBytes))} reclaimable\n`,
  );
  for (const g of groups) {
    console.log(
      `${ctx.style.cyan(`${g.files.length} copies`)} · ${formatBytes(g.size)} each · ` +
        `${ctx.style.gray(g.sha256.slice(0, 12) + '…')}`,
    );
    for (const f of g.files) {
      console.log(`  ${f.relPath}`);
    }
  }
  return 0;
}

export async function runHashCommand(args, ctx) {
  let algo = 'sha256';
  let manifest = false;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '--algo') {
      const next = (args[++i] || '').toLowerCase();
      if (!['sha256', 'sha512', 'sha1', 'md5', 'all'].includes(next)) {
        throw new CliUsageError('--algo must be one of: sha256, sha512, sha1, md5, all.');
      }
      algo = next;
    } else if (a === '--manifest') {
      manifest = true;
    } else if (a.startsWith('-')) {
      throw new CliUsageError(`Unknown option for hash: ${a}`);
    } else {
      positional.push(a);
    }
  }

  if (positional.length === 0) {
    throw new CliUsageError('Usage: aether hash <files...> [--algo sha256|sha512|sha1|md5|all] [--manifest]');
  }

  const algos = manifest ? ['sha256'] : algo === 'all' ? ['sha256', 'sha512', 'sha1', 'md5'] : [algo];
  const results = [];

  for (const raw of positional) {
    const target = path.resolve(ctx.cwd, raw);
    const stat = await fsp.stat(target).catch((err) => {
      throw new CliRuntimeError(`Cannot read "${raw}": ${err.message}`);
    });
    if (stat.isDirectory()) {
      throw new CliRuntimeError(`"${raw}" is a directory; hash expects file paths.`);
    }
    const digests = await computeFileHashes(target, algos);
    results.push({ path: target, name: raw, size: stat.size, hashes: digests });
  }

  if (ctx.json) {
    printJson({ algo: manifest ? 'sha256' : algo, files: results });
    return 0;
  }

  if (manifest) {
    for (const item of results) {
      console.log(`${item.hashes.sha256}  ${item.name}`);
    }
    return 0;
  }

  for (const item of results) {
    if (algos.length === 1) {
      console.log(`${ctx.style.cyan(item.hashes[algos[0]])}  ${item.name}`);
    } else {
      console.log(ctx.style.bold(item.name));
      for (const k of algos) {
        console.log(`  ${ctx.style.gray(k.toUpperCase().padEnd(6))} ${item.hashes[k]}`);
      }
    }
  }
  return 0;
}

export async function runVerifyCommand(args, ctx) {
  let rootOverride = null;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '--root') rootOverride = args[++i];
    else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for verify: ${a}`);
    else positional.push(a);
  }

  if (positional.length !== 1) {
    throw new CliUsageError('Usage: aether verify <manifest.sha256> [--root <dir>]');
  }

  const manifestPath = path.resolve(ctx.cwd, positional[0]);
  const baseDir = rootOverride ? path.resolve(ctx.cwd, rootOverride) : path.dirname(manifestPath);
  const raw = await fsp.readFile(manifestPath, 'utf8').catch((err) => {
    throw new CliRuntimeError(`Cannot read manifest "${manifestPath}": ${err.message}`);
  });

  const rows = [];
  let passed = 0;
  let failed = 0;
  let missing = 0;

  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const match = trimmed.match(/^([a-fA-F0-9]{32,128})\s+\*?(.+)$/);
    if (!match) {
      throw new CliRuntimeError(`Malformed manifest line: "${trimmed}"`);
    }
    const expected = match[1].toLowerCase();
    const relTarget = match[2].trim();
    const algo =
      expected.length === 32
        ? 'md5'
        : expected.length === 40
          ? 'sha1'
          : expected.length === 128
            ? 'sha512'
            : 'sha256';
    const fullTarget = path.resolve(baseDir, relTarget);
    if (!fs.existsSync(fullTarget)) {
      missing++;
      rows.push({ file: relTarget, status: 'missing', algo, expected, actual: null });
      continue;
    }
    const actual = (await computeFileHashes(fullTarget, [algo]))[algo].toLowerCase();
    if (actual === expected) {
      passed++;
      rows.push({ file: relTarget, status: 'ok', algo, expected, actual });
    } else {
      failed++;
      rows.push({ file: relTarget, status: 'mismatch', algo, expected, actual });
    }
  }

  const ok = failed === 0 && missing === 0;

  if (ctx.json) {
    printJson({ manifest: manifestPath, baseDir, ok, passed, failed, missing, entries: rows });
    return ok ? 0 : 1;
  }

  for (const row of rows) {
    const badge =
      row.status === 'ok'
        ? ctx.style.green('OK')
        : row.status === 'missing'
          ? ctx.style.yellow('MISSING')
          : ctx.style.red('FAILED');
    console.log(`${row.file}: ${badge}`);
  }
  if (!ok) {
    console.error(ctx.style.red(`Verification failed: ${passed} ok, ${failed} mismatch, ${missing} missing.`));
  }
  return ok ? 0 : 1;
}

export async function runSplitCommand(args, ctx) {
  let partSize = 1024 * 1024;
  let outDir = null;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '--size' || a === '-s') partSize = Math.max(1024, parseSize(args[++i]));
    else if (a === '--out-dir' || a === '-o') outDir = args[++i];
    else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for split: ${a}`);
    else positional.push(a);
  }

  if (positional.length !== 1) {
    throw new CliUsageError('Usage: aether split <file> [--size 1M] [--out-dir <dir>]');
  }

  const targetPath = path.resolve(ctx.cwd, positional[0]);
  const stat = await fsp.stat(targetPath).catch((err) => {
    throw new CliRuntimeError(`Cannot read "${targetPath}": ${err.message}`);
  });
  if (stat.isDirectory()) throw new CliRuntimeError('Cannot split a directory.');

  const destBaseDir = outDir ? path.resolve(ctx.cwd, outDir) : path.dirname(targetPath);
  await fsp.mkdir(destBaseDir, { recursive: true });
  const baseName = path.basename(targetPath);

  const parts = [];
  const fd = await fsp.open(targetPath, 'r');
  try {
    let index = 1;
    let offset = 0;
    const buf = Buffer.allocUnsafe(Math.min(partSize, 16 * 1024 * 1024));
    while (offset < stat.size || index === 1) {
      const partPath = path.join(destBaseDir, `${baseName}.part${String(index).padStart(3, '0')}`);
      const out = await fsp.open(partPath, 'w');
      let written = 0;
      try {
        while (written < partSize && offset < stat.size) {
          const toRead = Math.min(buf.length, partSize - written, stat.size - offset);
          const { bytesRead } = await fd.read(buf, 0, toRead, offset);
          if (!bytesRead) break;
          await out.write(buf, 0, bytesRead);
          offset += bytesRead;
          written += bytesRead;
        }
      } finally {
        await out.close();
      }
      parts.push({ path: partPath, bytes: written });
      index++;
      if (offset >= stat.size) break;
    }
  } finally {
    await fd.close();
  }

  if (ctx.json) {
    printJson({ source: targetPath, totalBytes: stat.size, partSize, parts });
    return 0;
  }

  console.log(ctx.style.green(`Split ${baseName} (${formatBytes(stat.size)}) into ${parts.length} part(s):`));
  for (const p of parts) {
    console.log(`  ${path.basename(p.path)} (${formatBytes(p.bytes)})`);
  }
  return 0;
}

export async function runJoinCommand(args, ctx) {
  let output = null;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '-o' || a === '--output') output = args[++i];
    else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for join: ${a}`);
    else positional.push(a);
  }

  if (positional.length === 0) {
    throw new CliUsageError('Usage: aether join <part001 | parts...> [-o <destination>]');
  }

  let parts = positional.map((p) => path.resolve(ctx.cwd, p));
  if (parts.length === 1 && /\.part\d{3,}$/i.test(parts[0])) {
    const first = parts[0];
    const dir = path.dirname(first);
    const stem = path.basename(first).replace(/\.part\d{3,}$/i, '');
    const siblings = await fsp.readdir(dir);
    parts = siblings
      .filter((name) => name.startsWith(`${stem}.part`) && /\.part\d{3,}$/i.test(name))
      .sort((a, b) => a.localeCompare(b, undefined, { numeric: true }))
      .map((name) => path.join(dir, name));
  }

  if (parts.length < 2) {
    throw new CliUsageError('Join requires at least two part files.');
  }

  const defaultDest = parts[0].replace(/\.part\d{3,}$/i, '.joined');
  const destination = output ? path.resolve(ctx.cwd, output) : defaultDest;
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

  const stat = await fsp.stat(destination);
  if (ctx.json) {
    printJson({ parts, destination, bytes: stat.size });
    return 0;
  }
  console.log(ctx.style.green(`Joined ${parts.length} parts -> ${destination} (${formatBytes(stat.size)})`));
  return 0;
}

export async function runRenameCommand(args, ctx) {
  let pattern = '';
  let find = '';
  let replace = '';
  let useRegex = false;
  let matchCase = false;
  let preserveExt = true;
  let start = 1;
  let step = 1;
  let apply = false;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '--pattern') pattern = args[++i] ?? '';
    else if (a === '--find') find = args[++i] ?? '';
    else if (a === '--replace') replace = args[++i] ?? '';
    else if (a === '--regex') useRegex = true;
    else if (a === '--match-case') matchCase = true;
    else if (a === '--no-preserve-ext') preserveExt = false;
    else if (a === '--preserve-ext') preserveExt = true;
    else if (a === '--start') start = Number(args[++i] ?? 1);
    else if (a === '--step') step = Number(args[++i] ?? 1);
    else if (a === '--apply') apply = true;
    else if (a === '--dry-run') apply = false;
    else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for rename: ${a}`);
    else positional.push(a);
  }

  if (positional.length === 0 || (!pattern && !find)) {
    throw new CliUsageError(
      'Usage: aether rename <paths...> (--pattern "{name}_{n}.{ext}" | --find X --replace Y) [--apply]',
    );
  }

  let searchRe = null;
  if (find) {
    try {
      const flags = matchCase ? 'g' : 'gi';
      searchRe = useRegex ? new RegExp(find, flags) : new RegExp(find.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), flags);
    } catch (err) {
      throw new CliUsageError(`Invalid --find regex: ${err.message}`);
    }
  }

  const plan = [];
  for (let idx = 0; idx < positional.length; idx++) {
    const src = path.resolve(ctx.cwd, positional[idx]);
    if (!fs.existsSync(src)) {
      throw new CliRuntimeError(`Path does not exist: ${src}`);
    }
    const dir = path.dirname(src);
    const oldName = path.basename(src);
    const ext = path.extname(oldName);
    const stem = ext ? oldName.slice(0, -ext.length) : oldName;
    const seq = String(start + idx * step);
    const padded = seq.padStart(3, '0');

    let newName = oldName;
    if (pattern) {
      newName = pattern
        .replace(/\{name\}/gi, stem)
        .replace(/\{ext\}/gi, ext.replace(/^\./, ''))
        .replace(/\{n\}/gi, seq)
        .replace(/\{nnn\}/gi, padded)
        .replace(/\{full\}/gi, oldName);
      if (preserveExt && ext && !pattern.toLowerCase().includes('{ext}') && !newName.endsWith(ext)) {
        newName = `${newName}${ext}`;
      }
    } else if (searchRe) {
      if (preserveExt && ext) {
        newName = `${stem.replace(searchRe, replace)}${ext}`;
      } else {
        newName = oldName.replace(searchRe, replace);
      }
    }

    const dest = path.join(dir, newName);
    plan.push({ src, dest, oldName, newName, changed: dest !== src });
  }

  // Collision check
  const targetCounts = new Map();
  for (const item of plan) {
    targetCounts.set(item.dest, (targetCounts.get(item.dest) || 0) + 1);
  }
  const sourceSet = new Set(plan.map((p) => p.src));
  for (const item of plan) {
    if ((targetCounts.get(item.dest) || 0) > 1) {
      throw new CliRuntimeError(`Collision detected: multiple items map to ${item.dest}`);
    }
    if (item.changed && fs.existsSync(item.dest) && !sourceSet.has(item.dest)) {
      throw new CliRuntimeError(`Destination already exists: ${item.dest}`);
    }
  }

  if (apply) {
    // Two-phase temp rename to safely handle cycles (A->B, B->A)
    const staged = [];
    for (let i = 0; i < plan.length; i++) {
      const item = plan[i];
      if (!item.changed) continue;
      const tmp = `${item.src}.aether_tmp_${process.pid}_${i}`;
      await fsp.rename(item.src, tmp);
      staged.push({ tmp, dest: item.dest });
    }
    for (const s of staged) {
      await fsp.rename(s.tmp, s.dest);
    }
  }

  if (ctx.json) {
    printJson({ applied: apply, count: plan.filter((p) => p.changed).length, items: plan });
    return 0;
  }

  for (const item of plan) {
    if (!item.changed) {
      console.log(ctx.style.gray(`= ${item.oldName} (unchanged)`));
    } else {
      const arrow = apply ? ctx.style.green('->') : ctx.style.yellow('->');
      console.log(`${item.oldName} ${arrow} ${ctx.style.bold(item.newName)}`);
    }
  }
  if (!apply) {
    console.log(ctx.style.gray('\nDry run only. Pass --apply to execute renames.'));
  }
  return 0;
}

export async function runCompareCommand(args, ctx) {
  let recursive = false;
  let useHash = false;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '-r' || a === '--recursive') recursive = true;
    else if (a === '--hash') useHash = true;
    else if (a.startsWith('-')) throw new CliUsageError(`Unknown option for compare: ${a}`);
    else positional.push(a);
  }

  if (positional.length !== 2) {
    throw new CliUsageError('Usage: aether compare <leftDir> <rightDir> [-r|--recursive] [--hash]');
  }

  const leftRoot = path.resolve(ctx.cwd, positional[0]);
  const rightRoot = path.resolve(ctx.cwd, positional[1]);

  async function collect(root) {
    const map = new Map();
    async function walk(dir) {
      const entries = await fsp.readdir(dir, { withFileTypes: true });
      for (const entry of entries) {
        if (entry.isSymbolicLink()) continue;
        const full = path.join(dir, entry.name);
        const rel = path.relative(root, full).replace(/\\/g, '/');
        if (entry.isDirectory()) {
          if (SKIP_DIRS.has(entry.name)) continue;
          map.set(rel, { path: full, rel, type: 'dir', size: 0, mtimeMs: 0 });
          if (recursive) await walk(full);
        } else if (entry.isFile()) {
          const stat = await fsp.stat(full);
          map.set(rel, { path: full, rel, type: 'file', size: stat.size, mtimeMs: stat.mtimeMs });
        }
      }
    }
    await walk(root);
    return map;
  }

  const [leftMap, rightMap] = await Promise.all([collect(leftRoot), collect(rightRoot)]);
  const allKeys = [...new Set([...leftMap.keys(), ...rightMap.keys()])].sort();

  const summary = { onlyLeft: [], onlyRight: [], modified: [], identical: [] };
  for (const key of allKeys) {
    const l = leftMap.get(key);
    const r = rightMap.get(key);
    if (l && !r) summary.onlyLeft.push(key);
    else if (!l && r) summary.onlyRight.push(key);
    else if (l && r) {
      if (l.type !== r.type) {
        summary.modified.push({ rel: key, reason: 'type-mismatch' });
      } else if (l.type === 'dir') {
        summary.identical.push(key);
      } else if (l.size !== r.size) {
        summary.modified.push({ rel: key, reason: 'size', leftSize: l.size, rightSize: r.size });
      } else if (useHash) {
        const [hl, hr] = await Promise.all([
          computeFileHashes(l.path, ['sha256']),
          computeFileHashes(r.path, ['sha256']),
        ]);
        if (hl.sha256 !== hr.sha256) {
          summary.modified.push({ rel: key, reason: 'content' });
        } else {
          summary.identical.push(key);
        }
      } else {
        summary.identical.push(key);
      }
    }
  }

  const same =
    summary.onlyLeft.length === 0 && summary.onlyRight.length === 0 && summary.modified.length === 0;

  if (ctx.json) {
    printJson({ left: leftRoot, right: rightRoot, identical: same, ...summary });
    return same ? 0 : 1;
  }

  console.log(
    `${ctx.style.green(`${summary.identical.length} identical`)}, ` +
      `${ctx.style.yellow(`${summary.modified.length} modified`)}, ` +
      `${ctx.style.cyan(`${summary.onlyLeft.length} only-left`)}, ` +
      `${ctx.style.magenta(`${summary.onlyRight.length} only-right`)}`,
  );
  for (const k of summary.onlyLeft) console.log(ctx.style.cyan(`  < ${k}`));
  for (const k of summary.onlyRight) console.log(ctx.style.magenta(`  > ${k}`));
  for (const m of summary.modified) console.log(ctx.style.yellow(`  ~ ${m.rel} (${m.reason})`));
  return same ? 0 : 1;
}
