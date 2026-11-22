import { execFile } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import fsp from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  CliRuntimeError,
  CliUsageError,
  formatBytes,
  formatDate,
  printJson,
  renderTable,
} from './output.mjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const REPO_ROOT = path.resolve(__dirname, '..', '..');

function getStatePaths() {
  const baseTmp = process.env.AETHER_STATE_DIR
    ? path.resolve(process.env.AETHER_STATE_DIR)
    : os.tmpdir();
  return {
    baseTmp,
    stateFile: path.join(baseTmp, 'reassembly-web-state.json'),
    trashRoot: path.join(baseTmp, 'reassembly-web-trash'),
    trashFilesDir: path.join(baseTmp, 'reassembly-web-trash', 'files'),
    trashManifest: path.join(baseTmp, 'reassembly-web-trash', 'index.json'),
    pluginsRoot: path.join(baseTmp, 'reassembly-web-plugins'),
  };
}

async function readTrashManifest(paths) {
  await fsp.mkdir(paths.trashFilesDir, { recursive: true });
  if (!fs.existsSync(paths.trashManifest)) return [];
  try {
    const raw = await fsp.readFile(paths.trashManifest, 'utf8');
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

async function writeTrashManifest(paths, entries) {
  await fsp.mkdir(paths.trashFilesDir, { recursive: true });
  await fsp.writeFile(paths.trashManifest, JSON.stringify(entries, null, 2), 'utf8');
}

async function moveSafely(src, dest) {
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

async function uniqueDestination(targetPath) {
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

export async function runTrashCommand(args, ctx) {
  const sub = args[0] || 'list';
  const rest = args.slice(1);
  const paths = getStatePaths();

  if (!['list', 'put', 'restore', 'empty'].includes(sub)) {
    throw new CliUsageError(
      'Usage: aether trash <list|put|restore|empty> ...\n' +
        '  list\n' +
        '  put <paths...>\n' +
        '  restore <id|name>\n' +
        '  empty --yes',
    );
  }

  if (sub === 'list') {
    const entries = await readTrashManifest(paths);
    if (ctx.json) {
      printJson({ trashRoot: paths.trashRoot, count: entries.length, entries });
      return 0;
    }
    if (entries.length === 0) {
      console.log(ctx.style.gray('Trash is empty.'));
      return 0;
    }
    const rows = entries.map((e) => [
      ctx.style.cyan(e.id.slice(-10)),
      e.isDir ? ctx.style.cyan('DIR') : 'FIL',
      e.isDir ? ctx.style.gray('-') : formatBytes(e.size),
      ctx.style.gray(formatDate(e.deletedAt)),
      e.name,
      ctx.style.gray(e.originalPath),
    ]);
    console.log(
      renderTable(['ID', 'TYPE', 'SIZE', 'DELETED', 'NAME', 'ORIGINAL PATH'], rows, {
        alignRight: [false, false, true, false, false, false],
      }),
    );
    return 0;
  }

  if (sub === 'put') {
    if (rest.length === 0) throw new CliUsageError('Usage: aether trash put <paths...>');
    const manifest = await readTrashManifest(paths);
    const moved = [];

    for (const raw of rest) {
      const target = path.resolve(ctx.cwd, raw);
      const lstat = await fsp.lstat(target).catch((err) => {
        throw new CliRuntimeError(`Cannot trash "${raw}": ${err.message}`);
      });
      const isDir = lstat.isDirectory();
      const size = isDir ? 0 : lstat.size;
      const name = path.basename(target);
      const id = `trash-${Date.now()}-${crypto.randomBytes(4).toString('hex')}`;
      const trashedPath = path.join(paths.trashFilesDir, `${id}_${name}`);
      await moveSafely(target, trashedPath);
      const entry = {
        id,
        name,
        originalPath: target,
        trashedPath,
        deletedAt: new Date().toISOString(),
        size,
        isDir,
        canRestore: true,
      };
      manifest.unshift(entry);
      moved.push(entry);
    }

    await writeTrashManifest(paths, manifest);
    if (ctx.json) {
      printJson({ trashed: moved });
      return 0;
    }
    for (const item of moved) {
      console.log(ctx.style.green(`Moved to trash: ${item.name} (${item.id})`));
    }
    return 0;
  }

  if (sub === 'restore') {
    if (rest.length !== 1) throw new CliUsageError('Usage: aether trash restore <id|name>');
    const query = rest[0];
    const manifest = await readTrashManifest(paths);
    const idx = manifest.findIndex(
      (e) => e.id === query || e.id.endsWith(query) || e.name === query,
    );
    if (idx < 0) {
      throw new CliRuntimeError(`Trash entry not found for "${query}".`);
    }
    const entry = manifest[idx];
    if (!fs.existsSync(entry.trashedPath)) {
      throw new CliRuntimeError(`Trashed payload is missing on disk: ${entry.trashedPath}`);
    }
    const destination = await uniqueDestination(entry.originalPath);
    await moveSafely(entry.trashedPath, destination);
    manifest.splice(idx, 1);
    await writeTrashManifest(paths, manifest);

    if (ctx.json) {
      printJson({ restored: entry, destination });
      return 0;
    }
    console.log(ctx.style.green(`Restored "${entry.name}" -> ${destination}`));
    return 0;
  }

  // empty
  const confirmed = rest.includes('--yes') || rest.includes('-y');
  if (!confirmed) {
    throw new CliUsageError('Refusing to empty trash without --yes confirmation.');
  }
  const existing = await readTrashManifest(paths);
  await fsp.rm(paths.trashFilesDir, { recursive: true, force: true });
  await fsp.mkdir(paths.trashFilesDir, { recursive: true });
  await writeTrashManifest(paths, []);

  if (ctx.json) {
    printJson({ emptiedCount: existing.length });
    return 0;
  }
  console.log(ctx.style.green(`Emptied ${existing.length} item(s) from trash.`));
  return 0;
}

export async function runWorkspaceCommand(args, ctx) {
  const sub = args[0] || 'list';
  if (!['list', 'active', 'paths'].includes(sub)) {
    throw new CliUsageError('Usage: aether ws <list|active|paths>');
  }

  const paths = getStatePaths();
  let state = { workspaces: [], activeWorkspaceId: null };
  if (fs.existsSync(paths.stateFile)) {
    try {
      state = JSON.parse(await fsp.readFile(paths.stateFile, 'utf8'));
    } catch (err) {
      throw new CliRuntimeError(`Failed to parse state file: ${err.message}`);
    }
  }

  const workspaces = Array.isArray(state?.workspaces) ? state.workspaces : [];
  const activeWs =
    workspaces.find((w) => w.id === state?.activeWorkspaceId) || workspaces[0] || null;

  if (sub === 'paths') {
    const extractPaneInfo = (pane) => {
      const tabs = Array.isArray(pane?.tabs) ? pane.tabs : [];
      const activeTab = tabs.find((t) => t.id === pane?.activeTabId || t.isActive) || tabs[0] || null;
      return {
        activePath: activeTab?.path || null,
        activeTitle: activeTab?.title || null,
        tabs: tabs.map((t) => ({ id: t.id, title: t.title, path: t.path, kind: t.kind, surfaceId: t.surfaceId })),
      };
    };
    const payload = activeWs
      ? {
          workspaceId: activeWs.id,
          workspaceName: activeWs.name,
          leftPane: extractPaneInfo(activeWs.leftPane),
          rightPane: extractPaneInfo(activeWs.rightPane),
        }
      : { workspaceId: null, leftPane: null, rightPane: null };

    if (ctx.json) {
      printJson(payload);
      return 0;
    }
    if (!activeWs) {
      console.log(ctx.style.gray('No saved workspaces found yet.'));
      return 0;
    }
    console.log(`${ctx.style.bold(activeWs.name || activeWs.id)}`);
    console.log(`  Left  : ${payload.leftPane.activePath || '-'}`);
    console.log(`  Right : ${payload.rightPane.activePath || '-'}`);
    return 0;
  }

  if (sub === 'active') {
    if (ctx.json) {
      printJson({ stateFile: paths.stateFile, activeWorkspace: activeWs });
      return 0;
    }
    if (!activeWs) {
      console.log(ctx.style.gray('No active workspace persisted yet.'));
      return 0;
    }
    console.log(`${ctx.style.bold(activeWs.name || activeWs.id)} (${activeWs.id})`);
    console.log(`Left tabs : ${activeWs.leftPane?.tabs?.length ?? 0}`);
    console.log(`Right tabs: ${activeWs.rightPane?.tabs?.length ?? 0}`);
    return 0;
  }

  if (ctx.json) {
    printJson({ stateFile: paths.stateFile, activeWorkspaceId: activeWs?.id ?? null, workspaces });
    return 0;
  }
  if (workspaces.length === 0) {
    console.log(ctx.style.gray(`No workspaces found in ${paths.stateFile}.`));
    return 0;
  }
  const rows = workspaces.map((w) => [
    w.id === activeWs?.id ? ctx.style.green('*') : '',
    w.id,
    w.name || '(unnamed)',
    String(w.leftPane?.tabs?.length ?? 0),
    String(w.rightPane?.tabs?.length ?? 0),
  ]);
  console.log(renderTable(['ACT', 'ID', 'NAME', 'L-TABS', 'R-TABS'], rows, { alignRight: [false, false, false, true, true] }));
  return 0;
}

function checkCommand(cmd, args) {
  return new Promise((resolve) => {
    execFile(cmd, args, { timeout: 3000 }, (err, stdout, stderr) => {
      if (err) resolve({ ok: false, output: err.message });
      else resolve({ ok: true, output: (stdout || stderr || '').trim().split(/\r?\n/)[0] });
    });
  });
}

export async function runDoctorCommand(_args, ctx) {
  const paths = getStatePaths();
  const distIndex = path.join(REPO_ROOT, 'manager', 'frontend', 'dist', 'index.html');
  const distBrowserIndex = path.join(REPO_ROOT, 'manager', 'frontend', 'dist', 'browser', 'index.html');
  const distBuilt = fs.existsSync(distIndex) || fs.existsSync(distBrowserIndex);

  const [pyCheck, goCheck] = await Promise.all([
    checkCommand('python3', ['--version']),
    checkCommand('go', ['version']),
  ]);

  let writableState = true;
  try {
    await fsp.mkdir(paths.trashFilesDir, { recursive: true });
    await fsp.access(paths.trashFilesDir, fs.constants.W_OK);
  } catch {
    writableState = false;
  }

  const checks = [
    { name: 'Node.js', status: 'ok', detail: process.version },
    { name: 'Python 3 (archives)', status: pyCheck.ok ? 'ok' : 'warn', detail: pyCheck.output },
    { name: 'Go (desktop Wails)', status: goCheck.ok ? 'ok' : 'info', detail: goCheck.ok ? goCheck.output : 'not installed (web/CLI mode active)' },
    { name: 'Frontend dist build', status: distBuilt ? 'ok' : 'warn', detail: distBuilt ? 'built' : 'not built (run npm run build)' },
    { name: 'State & Trash storage', status: writableState ? 'ok' : 'fail', detail: `${paths.baseTmp} (${writableState ? 'writable' : 'not writable'})` },
  ];

  const allOk = checks.every((c) => c.status !== 'fail');

  if (ctx.json) {
    printJson({ ok: allOk, repoRoot: REPO_ROOT, checks });
    return allOk ? 0 : 1;
  }

  console.log(ctx.style.bold('Aether / Reassembly Environment Doctor\n'));
  const rows = checks.map((c) => [
    c.status === 'ok'
      ? ctx.style.green('OK')
      : c.status === 'warn'
        ? ctx.style.yellow('WARN')
        : c.status === 'info'
          ? ctx.style.cyan('INFO')
          : ctx.style.red('FAIL'),
    c.name,
    c.detail,
  ]);
  console.log(renderTable(['STATUS', 'COMPONENT', 'DETAIL'], rows));
  return allOk ? 0 : 1;
}
